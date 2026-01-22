package digest

import (
	"log/slog"
	"strconv"
	"time"
)

// Interfaces for dependencies
type HNClient interface {
	GetTopStories() ([]int, error)
	GetItem(id int) (HNItem, error)
}

type HNItem interface {
	GetID() int
	GetTitle() string
	GetURL() string
	GetScore() int
	GetDescendants() int
}

type Scraper interface {
	Scrape(url string) (string, error)
}

type Summarizer interface {
	Summarize(title, content string) (Summary, error)
}

type Summary interface {
	GetSummary() string
	GetTags() []string
}

type Ranker interface {
	Rank(articles []RankerArticle, tagWeights map[string]float64) []RankedArticle
}

type RankerArticle struct {
	ID      int
	Tags    []string
	HNScore int
}

type RankedArticle struct {
	Article RankerArticle
	Score   float64
}

type WorkflowStorage interface {
	ApplyDecay(decayRate, minWeight float64) error
	GetRecentlySentArticles(days int) ([]int, error)
	GetAllTagWeights() ([]TagWeight, error)
	SaveArticle(article *StorageArticle) error
	GetSetting(key string) (string, error)
}

type TagWeight struct {
	Tag    string
	Weight float64
}

type StorageArticle struct {
	ID            int
	Title         string
	URL           string
	Summary       string
	Tags          []string
	HNScore       int
	FetchedAt     time.Time
	SentAt        *time.Time
	TelegramMsgID int
}

type Bot interface {
	SendArticle(chatID int64, title, url, summary string, score, comments, hnID int) (int, error)
}

type Workflow struct {
	hnClient      HNClient
	scraper       Scraper
	summarizer    Summarizer
	ranker        Ranker
	storage       WorkflowStorage
	bot           Bot
	decayRate     float64
	minWeight     float64
	bufferFactor  float64
}

func New(
	hnClient HNClient,
	scraper Scraper,
	summarizer Summarizer,
	ranker Ranker,
	storage WorkflowStorage,
	bot Bot,
	decayRate, minWeight, bufferFactor float64,
) *Workflow {
	return &Workflow{
		hnClient:      hnClient,
		scraper:       scraper,
		summarizer:    summarizer,
		ranker:        ranker,
		storage:       storage,
		bot:           bot,
		decayRate:     decayRate,
		minWeight:     minWeight,
		bufferFactor:  bufferFactor,
	}
}

func (w *Workflow) Run() {
	slog.Info("Starting digest workflow")

	// Step 1: Apply decay
	if err := w.storage.ApplyDecay(w.decayRate, w.minWeight); err != nil {
		slog.Error("Failed to apply decay", "error", err)
	}

	// Get configuration
	chatIDStr, _ := w.storage.GetSetting("chat_id")
	articleCountStr, _ := w.storage.GetSetting("article_count")

	if chatIDStr == "" {
		slog.Warn("No chat_id configured, skipping digest")
		return
	}

	chatID, _ := strconv.ParseInt(chatIDStr, 10, 64)
	articleCount := 30
	if articleCountStr != "" {
		if count, err := strconv.Atoi(articleCountStr); err == nil {
			articleCount = count
		}
	}

	// Step 2: Fetch stories
	fetchCount := int(float64(articleCount) * w.bufferFactor)
	topStories, err := w.hnClient.GetTopStories()
	if err != nil {
		slog.Error("Failed to fetch top stories", "error", err)
		return
	}

	if len(topStories) > fetchCount {
		topStories = topStories[:fetchCount]
	}

	slog.Info("Fetched top stories", "count", len(topStories))

	// Step 3: Filter recent articles
	recentArticleIDs, _ := w.storage.GetRecentlySentArticles(7)
	recentMap := make(map[int]bool)
	for _, id := range recentArticleIDs {
		recentMap[id] = true
	}

	var filteredStories []int
	for _, id := range topStories {
		if !recentMap[id] {
			filteredStories = append(filteredStories, id)
		}
	}

	slog.Info("Filtered recent articles", "before", len(topStories), "after", len(filteredStories))

	// Step 4: Process each story
	type processedArticle struct {
		ID          int
		Title       string
		URL         string
		Summary     string
		Tags        []string
		HNScore     int
		Descendants int
	}

	var processed []processedArticle

	for _, id := range filteredStories {
		item, err := w.hnClient.GetItem(id)
		if err != nil {
			slog.Warn("Failed to fetch item", "id", id, "error", err)
			continue
		}

		// Scrape content
		content, err := w.scraper.Scrape(item.GetURL())
		if err != nil {
			slog.Warn("Failed to scrape, using title fallback", "id", id, "error", err)
			content = item.GetTitle()
		}

		// Summarize
		summary, err := w.summarizer.Summarize(item.GetTitle(), content)
		if err != nil {
			slog.Warn("Failed to summarize, skipping article", "id", id, "error", err)
			continue
		}

		processed = append(processed, processedArticle{
			ID:          item.GetID(),
			Title:       item.GetTitle(),
			URL:         item.GetURL(),
			Summary:     summary.GetSummary(),
			Tags:        summary.GetTags(),
			HNScore:     item.GetScore(),
			Descendants: item.GetDescendants(),
		})
	}

	slog.Info("Processed articles", "count", len(processed))

	if len(processed) == 0 {
		slog.Warn("No articles to send")
		return
	}

	// Step 5: Rank articles
	tagWeights, _ := w.storage.GetAllTagWeights()
	weightMap := make(map[string]float64)
	for _, tw := range tagWeights {
		weightMap[tw.Tag] = tw.Weight
	}

	var toRank []RankerArticle
	for _, p := range processed {
		toRank = append(toRank, RankerArticle{
			ID:      p.ID,
			Tags:    p.Tags,
			HNScore: p.HNScore,
		})
	}

	ranked := w.ranker.Rank(toRank, weightMap)

	// Create lookup map for processed articles
	processedMap := make(map[int]processedArticle)
	for _, p := range processed {
		processedMap[p.ID] = p
	}

	// Step 6: Send top N articles
	sendCount := articleCount
	if len(ranked) < sendCount {
		sendCount = len(ranked)
	}

	for i := 0; i < sendCount; i++ {
		article := processedMap[ranked[i].Article.ID]

		msgID, err := w.bot.SendArticle(
			chatID,
			article.Title,
			article.URL,
			article.Summary,
			article.HNScore,
			article.Descendants,
			article.ID,
		)

		if err != nil {
			slog.Error("Failed to send article", "id", article.ID, "error", err)
			continue
		}

		// Save to storage
		now := time.Now()
		storageArticle := &StorageArticle{
			ID:            article.ID,
			Title:         article.Title,
			URL:           article.URL,
			Summary:       article.Summary,
			Tags:          article.Tags,
			HNScore:       article.HNScore,
			FetchedAt:     now,
			SentAt:        &now,
			TelegramMsgID: msgID,
		}

		if err := w.storage.SaveArticle(storageArticle); err != nil {
			slog.Warn("Failed to save article", "id", article.ID, "error", err)
		}
	}

	slog.Info("Digest workflow completed", "sent", sendCount)
}
