package digest

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"hn-telegram-bot/ranker"
)

const defaultRecencyWindow = 7 * 24 * time.Hour

// HNItem represents a Hacker News item.
type HNItem struct {
	ID          int64
	Title       string
	URL         string
	Score       int
	Descendants int
}

// SummaryResult contains summarization output.
type SummaryResult struct {
	Summary string
	Tags    []string
}

// StoredArticle represents an article in storage.
type StoredArticle struct {
	ID            int64
	Title         string
	URL           string
	Summary       string
	Tags          []string
	HNScore       int
	FetchedAt     time.Time
	SentAt        *time.Time
	TelegramMsgID *int64
}

// ProcessedArticle is an article ready for ranking.
type ProcessedArticle struct {
	ID       int64
	Title    string
	URL      string
	Summary  string
	Tags     []string
	HNScore  int
	Comments int
}

// ArticleToSend contains data for sending an article to Telegram.
type ArticleToSend struct {
	ID       int64
	Title    string
	URL      string
	Summary  string
	HNScore  int
	Comments int
}

// HNClient fetches data from Hacker News.
type HNClient interface {
	GetTopStories(ctx context.Context, limit int) ([]int64, error)
	GetItem(ctx context.Context, id int64) (*HNItem, error)
}

// Scraper extracts content from URLs.
type Scraper interface {
	Scrape(ctx context.Context, url string) (string, error)
}

// Summarizer generates summaries.
type Summarizer interface {
	Summarize(ctx context.Context, title, content string) (*SummaryResult, error)
}

// Storage provides persistence operations.
type Storage interface {
	GetRecentlySentArticleIDs(ctx context.Context, within time.Duration) ([]int64, error)
	GetAllTagWeights(ctx context.Context) (map[string]float64, error)
	ApplyTagDecay(ctx context.Context, decayRate, minWeight float64) error
	SaveArticle(ctx context.Context, article *StoredArticle) error
	MarkArticleSent(ctx context.Context, articleID int64, telegramMsgID int64) error
	GetSetting(ctx context.Context, key string) (string, error)
}

// ArticleSender sends articles to Telegram.
type ArticleSender interface {
	SendArticle(ctx context.Context, chatID int64, article *ArticleToSend) (int64, error)
}

// Runner orchestrates the digest workflow.
type Runner struct {
	hnClient     HNClient
	scraper      Scraper
	summarizer   Summarizer
	storage      Storage
	sender       ArticleSender
	chatID       int64
	articleCount int
	decayRate    float64
	minTagWeight float64
}

// Option configures a Runner.
type Option func(*Runner)

// WithChatID sets the Telegram chat ID.
func WithChatID(chatID int64) Option {
	return func(r *Runner) {
		r.chatID = chatID
	}
}

// WithArticleCount sets the number of articles per digest.
func WithArticleCount(count int) Option {
	return func(r *Runner) {
		r.articleCount = count
	}
}

// WithDecayRate sets the tag decay rate.
func WithDecayRate(rate float64) Option {
	return func(r *Runner) {
		r.decayRate = rate
	}
}

// WithMinTagWeight sets the minimum tag weight floor.
func WithMinTagWeight(weight float64) Option {
	return func(r *Runner) {
		r.minTagWeight = weight
	}
}

// NewRunner creates a new digest runner.
func NewRunner(
	hnClient HNClient,
	scraper Scraper,
	summarizer Summarizer,
	storage Storage,
	sender ArticleSender,
	opts ...Option,
) *Runner {
	r := &Runner{
		hnClient:     hnClient,
		scraper:      scraper,
		summarizer:   summarizer,
		storage:      storage,
		sender:       sender,
		articleCount: 30,
		decayRate:    0.02,
		minTagWeight: 0.1,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Run executes the digest workflow.
func (r *Runner) Run(ctx context.Context) error {
	if r.chatID == 0 {
		return fmt.Errorf("chat_id not set")
	}

	slog.Info("starting digest run", "chat_id", r.chatID, "article_count", r.articleCount)

	// Step 1: Apply tag decay
	if err := r.storage.ApplyTagDecay(ctx, r.decayRate, r.minTagWeight); err != nil {
		slog.Warn("failed to apply tag decay", "error", err)
	}

	// Step 2: Fetch top stories (2x buffer for filtering)
	fetchCount := r.articleCount * 2
	storyIDs, err := r.hnClient.GetTopStories(ctx, fetchCount)
	if err != nil {
		return fmt.Errorf("fetch top stories: %w", err)
	}
	slog.Info("fetched story IDs", "count", len(storyIDs))

	// Step 3: Filter recently sent
	recentIDs, err := r.storage.GetRecentlySentArticleIDs(ctx, defaultRecencyWindow)
	if err != nil {
		slog.Warn("failed to get recently sent IDs", "error", err)
	}
	recentSet := make(map[int64]bool)
	for _, id := range recentIDs {
		recentSet[id] = true
	}

	var filteredIDs []int64
	for _, id := range storyIDs {
		if !recentSet[id] {
			filteredIDs = append(filteredIDs, id)
		}
	}
	slog.Info("filtered stories", "before", len(storyIDs), "after", len(filteredIDs))

	// Step 4: Process each story
	var processed []*ProcessedArticle
	for _, id := range filteredIDs {
		article, err := r.processStory(ctx, id)
		if err != nil {
			slog.Warn("failed to process story", "id", id, "error", err)
			continue
		}
		processed = append(processed, article)
	}
	slog.Info("processed articles", "count", len(processed))

	if len(processed) == 0 {
		slog.Info("no articles to send")
		return nil
	}

	// Step 5: Rank articles
	tagWeights, err := r.storage.GetAllTagWeights(ctx)
	if err != nil {
		slog.Warn("failed to get tag weights", "error", err)
		tagWeights = make(map[string]float64)
	}

	rankableArticles := make([]ranker.RankableArticle, len(processed))
	for i, a := range processed {
		rankableArticles[i] = ranker.RankableArticle{
			ID:      a.ID,
			Tags:    a.Tags,
			HNScore: a.HNScore,
		}
	}

	articleRanker := ranker.NewRanker(0.7, 0.3)
	ranked := articleRanker.Rank(rankableArticles, tagWeights)

	// Map ranked back to processed articles
	processedByID := make(map[int64]*ProcessedArticle)
	for _, a := range processed {
		processedByID[a.ID] = a
	}

	// Step 6: Send top N articles
	sendCount := r.articleCount
	if sendCount > len(ranked) {
		sendCount = len(ranked)
	}

	for i := 0; i < sendCount; i++ {
		rankedArticle := ranked[i]
		article := processedByID[rankedArticle.ID]

		toSend := &ArticleToSend{
			ID:       article.ID,
			Title:    article.Title,
			URL:      article.URL,
			Summary:  article.Summary,
			HNScore:  article.HNScore,
			Comments: article.Comments,
		}

		msgID, err := r.sender.SendArticle(ctx, r.chatID, toSend)
		if err != nil {
			slog.Warn("failed to send article", "id", article.ID, "error", err)
			continue
		}

		// Save and mark as sent
		stored := &StoredArticle{
			ID:        article.ID,
			Title:     article.Title,
			URL:       article.URL,
			Summary:   article.Summary,
			Tags:      article.Tags,
			HNScore:   article.HNScore,
			FetchedAt: time.Now(),
		}
		if err := r.storage.SaveArticle(ctx, stored); err != nil {
			slog.Warn("failed to save article", "id", article.ID, "error", err)
		}
		if err := r.storage.MarkArticleSent(ctx, article.ID, msgID); err != nil {
			slog.Warn("failed to mark article sent", "id", article.ID, "error", err)
		}

		slog.Info("sent article", "id", article.ID, "title", article.Title, "score", rankedArticle.FinalScore)
	}

	slog.Info("digest run complete", "sent", sendCount)
	return nil
}

func (r *Runner) processStory(ctx context.Context, id int64) (*ProcessedArticle, error) {
	// Fetch item details
	item, err := r.hnClient.GetItem(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("fetch item: %w", err)
	}

	// Scrape content (use title as fallback)
	content := item.Title
	if item.URL != "" {
		scraped, err := r.scraper.Scrape(ctx, item.URL)
		if err != nil {
			slog.Warn("scrape failed, using title as content", "url", item.URL, "error", err)
		} else if scraped != "" {
			content = scraped
		}
	}

	// Summarize
	result, err := r.summarizer.Summarize(ctx, item.Title, content)
	if err != nil {
		return nil, fmt.Errorf("summarize: %w", err)
	}

	url := item.URL
	if url == "" {
		// For Ask HN, Show HN, etc. - use the HN discussion page
		url = fmt.Sprintf("https://news.ycombinator.com/item?id=%d", item.ID)
	}

	return &ProcessedArticle{
		ID:       item.ID,
		Title:    item.Title,
		URL:      url,
		Summary:  result.Summary,
		Tags:     result.Tags,
		HNScore:  item.Score,
		Comments: item.Descendants,
	}, nil
}
