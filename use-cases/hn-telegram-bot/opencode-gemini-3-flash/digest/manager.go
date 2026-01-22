package digest

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/opencode/hn-telegram-bot/config"
	"github.com/opencode/hn-telegram-bot/hn"
	"github.com/opencode/hn-telegram-bot/ranker"
	"github.com/opencode/hn-telegram-bot/storage"
	"github.com/opencode/hn-telegram-bot/summarizer"
)

type HNClient interface {
	GetTopStories(ctx context.Context) ([]int64, error)
	GetItem(ctx context.Context, id int64) (*hn.Item, error)
}

type Scraper interface {
	Scrape(url string) (string, error)
}

type Summarizer interface {
	Summarize(ctx context.Context, title, content string) (*summarizer.Summary, error)
}

type Bot interface {
	SendArticle(chatID int64, art *storage.Article) (int, error)
}

type Storage interface {
	GetAllTagWeights(ctx context.Context) (map[string]float64, error)
	UpdateTagWeight(ctx context.Context, name string, weight float64, countIncr int) error
	GetRecentArticleIDs(ctx context.Context, days int) ([]int64, error)
	SaveArticle(ctx context.Context, a *storage.Article) error
	MarkArticleSent(ctx context.Context, id int64, msgID int) error
}

type Manager struct {
	cfg        *config.Config
	hn         HNClient
	scraper    Scraper
	summarizer Summarizer
	bot        Bot
	storage    Storage
}

func NewManager(cfg *config.Config, hn HNClient, s Scraper, sum Summarizer, bot Bot, storage Storage) *Manager {
	return &Manager{
		cfg:        cfg,
		hn:         hn,
		scraper:    s,
		summarizer: sum,
		bot:        bot,
		storage:    storage,
	}
}

func (m *Manager) SendDigest(ctx context.Context) error {
	slog.Info("Starting digest cycle")

	if m.cfg.ChatID == 0 {
		return fmt.Errorf("chat_id not set in config")
	}

	// 1. Apply Decay
	if err := m.applyDecay(ctx); err != nil {
		slog.Error("failed to apply decay", "error", err)
	}

	// 2. Fetch Stories
	ids, err := m.hn.GetTopStories(ctx)
	if err != nil {
		return fmt.Errorf("failed to get top stories: %w", err)
	}

	// 3. Filter Recent
	recentIDs, err := m.storage.GetRecentArticleIDs(ctx, 7)
	if err != nil {
		slog.Error("failed to get recent IDs", "error", err)
	}
	recentMap := make(map[int64]bool)
	for _, id := range recentIDs {
		recentMap[id] = true
	}

	var articles []*storage.Article
	count := 0
	targetCount := m.cfg.ArticleCount
	fetchCount := targetCount * 2
	if fetchCount > len(ids) {
		fetchCount = len(ids)
	}

	// 4. Process Each Story
	for i := 0; i < fetchCount && count < fetchCount; i++ {
		id := ids[i]
		if recentMap[id] {
			continue
		}

		item, err := m.hn.GetItem(ctx, id)
		if err != nil {
			slog.Warn("failed to fetch item", "id", id, "error", err)
			continue
		}

		if item.Type != "story" || item.URL == "" {
			continue
		}

		// Scrape
		content, err := m.scraper.Scrape(item.URL)
		if err != nil {
			slog.Warn("scraping failed, using title as fallback", "url", item.URL, "error", err)
			content = item.Title
		}

		// Summarize
		summary, err := m.summarizer.Summarize(ctx, item.Title, content)
		if err != nil {
			slog.Warn("summarization failed, skipping article", "id", id, "error", err)
			continue
		}

		art := &storage.Article{
			ID:        item.ID,
			Title:     item.Title,
			URL:       item.URL,
			Summary:   summary.Summary,
			Tags:      summary.Tags,
			Score:     item.Score,
			FetchedAt: time.Now(),
		}
		articles = append(articles, art)
		count++
	}

	// 5. Rank Articles
	weights, err := m.storage.GetAllTagWeights(ctx)
	if err != nil {
		slog.Error("failed to get tag weights for ranking", "error", err)
	}
	ranked := ranker.Rank(articles, weights)

	// 6. Send Top N
	toSend := targetCount
	if len(ranked) < toSend {
		toSend = len(ranked)
	}

	for i := 0; i < toSend; i++ {
		art := ranked[i]
		msgID, err := m.bot.SendArticle(m.cfg.ChatID, art)
		if err != nil {
			slog.Error("failed to send article", "id", art.ID, "error", err)
			continue
		}

		// 7. Persist
		if err := m.storage.SaveArticle(ctx, art); err != nil {
			slog.Error("failed to save article", "id", art.ID, "error", err)
		}
		if err := m.storage.MarkArticleSent(ctx, art.ID, msgID); err != nil {
			slog.Error("failed to mark article as sent", "id", art.ID, "error", err)
		}

		// Small delay to avoid rate limits
		time.Sleep(500 * time.Millisecond)
	}

	slog.Info("Digest cycle completed", "sent_count", toSend)
	return nil
}

func (m *Manager) applyDecay(ctx context.Context) error {
	weights, err := m.storage.GetAllTagWeights(ctx)
	if err != nil {
		return err
	}

	for tag, weight := range weights {
		newWeight := weight * (1.0 - m.cfg.TagDecayRate)
		if newWeight < m.cfg.MinTagWeight {
			newWeight = m.cfg.MinTagWeight
		}
		if err := m.storage.UpdateTagWeight(ctx, tag, newWeight, 0); err != nil {
			slog.Error("failed to update decayed tag weight", "tag", tag, "error", err)
		}
	}
	return nil
}
