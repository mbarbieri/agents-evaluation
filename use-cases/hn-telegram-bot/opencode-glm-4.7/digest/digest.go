package digest

import (
	"fmt"
	"log/slog"
	"time"

	"hn-bot/bot"
	"hn-bot/hn"
	"hn-bot/ranker"
	"hn-bot/scraper"
	"hn-bot/storage"
	"hn-bot/summarizer"
)

type Digest struct {
	hnClient      *hn.Client
	scraper       *scraper.Scraper
	summarizer    *summarizer.Summarizer
	ranker        *ranker.Ranker
	storage       StorageInterface
	messageSender bot.MessageSender
	chatID        int64
	articleCount  int
	decayRate     float64
	minTagWeight  float64
	tagBoost      float64
}

type StorageInterface interface {
	DecayTagWeights(decayRate, minWeight float64) error
	GetRecentArticles(cutoff time.Time) ([]int64, error)
	GetAllTagWeights() (map[string]storage.TagWeight, error)
}

func NewDigest(hnClient *hn.Client, scraper *scraper.Scraper, summarizer *summarizer.Summarizer, ranker *ranker.Ranker, storage StorageInterface, messageSender bot.MessageSender, chatID int64, articleCount, decayRate, minTagWeight, tagBoost float64) *Digest {
	return &Digest{
		hnClient:      hnClient,
		scraper:       scraper,
		summarizer:    summarizer,
		ranker:        ranker,
		storage:       storage,
		messageSender: messageSender,
		chatID:        chatID,
		articleCount:  int(articleCount),
		decayRate:     decayRate,
		minTagWeight:  minTagWeight,
		tagBoost:      tagBoost,
	}
}

func (d *Digest) Run() error {
	slog.Info("Starting digest cycle")

	if err := d.storage.DecayTagWeights(d.decayRate, d.minTagWeight); err != nil {
		slog.Warn("Failed to decay tag weights", "error", err)
	}

	storyIDs, err := d.hnClient.GetTopStories()
	if err != nil {
		return fmt.Errorf("failed to get top stories: %w", err)
	}

	slog.Info("Fetched top stories", "count", len(storyIDs))

	fetchCount := d.articleCount * 2
	if fetchCount > len(storyIDs) {
		fetchCount = len(storyIDs)
	}

	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	recentArticles, err := d.storage.GetRecentArticles(cutoff)
	if err != nil {
		slog.Warn("Failed to get recent articles", "error", err)
	}

	recentSet := make(map[int64]bool)
	for _, id := range recentArticles {
		recentSet[id] = true
	}

	var processedArticles []ranker.Article
	for i := 0; i < fetchCount && i < len(storyIDs); i++ {
		storyID := storyIDs[i]
		if recentSet[int64(storyID)] {
			continue
		}

		item, err := d.hnClient.GetItem(storyID)
		if err != nil {
			slog.Warn("Failed to get item details", "id", storyID, "error", err)
			continue
		}

		if item.Type != "story" || item.URL == "" {
			continue
		}

		var content string
		if item.URL != "" {
			content, err = d.scraper.Scrape(item.URL)
			if err != nil {
				slog.Warn("Failed to scrape article", "url", item.URL, "error", err)
				content = item.Title
			}
		} else {
			content = item.Title
		}

		summary, tags, err := d.summarizer.Summarize(content)
		if err != nil {
			slog.Warn("Failed to summarize article", "id", storyID, "error", err)
			continue
		}

		article := ranker.Article{
			ID:      int64(item.ID),
			Title:   item.Title,
			Tags:    tags,
			HNScore: item.Score,
		}

		processedArticles = append(processedArticles, article)
		_ = summary
	}

	slog.Info("Processed articles", "count", len(processedArticles))

	tagWeightsMap := make(map[string]float64)
	tagWeights, err := d.storage.GetAllTagWeights()
	if err != nil {
		slog.Warn("Failed to get tag weights", "error", err)
	} else {
		for tag, tw := range tagWeights {
			tagWeightsMap[tag] = tw.Weight
		}
	}

	rankedArticles := d.ranker.RankArticles(processedArticles, tagWeightsMap)
	slog.Info("Ranked articles", "count", len(rankedArticles))

	topArticles := d.ranker.GetTopArticles(rankedArticles, tagWeightsMap, d.articleCount)
	slog.Info("Sending top articles", "count", len(topArticles))

	for _, article := range topArticles {
		message := bot.FormatArticleMessage(article.Title, "AI-generated summary", "https://example.com", int(article.ID), article.HNScore, 0)
		if err := d.messageSender.Send(d.chatID, message); err != nil {
			slog.Warn("Failed to send article", "id", article.ID, "error", err)
		} else {
			slog.Info("Sent article", "id", article.ID)
		}
	}

	slog.Info("Digest cycle completed")
	return nil
}
