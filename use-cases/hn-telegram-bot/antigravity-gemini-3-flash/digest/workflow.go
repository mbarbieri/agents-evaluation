package digest

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/antigravity/hn-telegram-bot/hn"
	"github.com/antigravity/hn-telegram-bot/ranker"
	"github.com/antigravity/hn-telegram-bot/storage"
)

type Storage interface {
	GetTagWeights() (map[string]storage.TagWeight, error)
	UpdateTagWeight(tag string, weight float64, occurrences int) error
	GetRecentHNIDs(days int) ([]int, error)
	SaveArticle(a *storage.Article) error
}

type HNClient interface {
	GetTopStories() ([]int, error)
	GetItem(id int) (*hn.Item, error)
}

type Scraper interface {
	Scrape(url string) (string, error)
}

type Summarizer interface {
	Summarize(title, content string) (string, []string, error)
}

type Sender interface {
	SendArticle(a *storage.Article) (int, error)
}

type WorkflowConfig struct {
	ArticleCount int
	TagDecayRate float64
	MinTagWeight float64
	RecentDays   int
}

type Workflow struct {
	storage    Storage
	hn         HNClient
	scraper    Scraper
	summarizer Summarizer
	config     *WorkflowConfig
}

func NewWorkflow(s Storage, h HNClient, sc Scraper, su Summarizer, cfg *WorkflowConfig) *Workflow {
	return &Workflow{
		storage:    s,
		hn:         h,
		scraper:    sc,
		summarizer: su,
		config:     cfg,
	}
}

func (w *Workflow) Run(ctx context.Context, sender Sender) error {
	slog.Info("Starting digest workflow cycle")

	// 1. Apply Decay
	if err := w.applyDecay(); err != nil {
		slog.Error("Failed to apply decay", "error", err)
	}

	// 2. Fetch Stories (2x count)
	topIDs, err := w.hn.GetTopStories()
	if err != nil {
		return fmt.Errorf("failed to fetch top stories: %w", err)
	}

	// 3. Filter Recent
	recentIDs, err := w.storage.GetRecentHNIDs(w.config.RecentDays)
	if err != nil {
		slog.Warn("Failed to fetch recent IDs, continuing without filtering", "error", err)
	}
	recentMap := make(map[int]bool)
	for _, id := range recentIDs {
		recentMap[id] = true
	}

	targetCount := w.config.ArticleCount
	maxToProcess := targetCount * 2
	processedCount := 0
	var articles []ranker.Article
	articleMap := make(map[int]*storage.Article)

	for _, id := range topIDs {
		if recentMap[id] {
			continue
		}
		if processedCount >= maxToProcess {
			break
		}

		item, err := w.hn.GetItem(id)
		if err != nil {
			slog.Warn("Failed to fetch item details", "id", id, "error", err)
			continue
		}

		// Scrape
		content, err := w.scraper.Scrape(item.URL)
		if err != nil {
			slog.Warn("Scraper failed, using title as fallback", "id", id, "url", item.URL, "error", err)
			content = item.Title
		}

		// Summarize
		summary, tags, err := w.summarizer.Summarize(item.Title, content)
		if err != nil {
			slog.Warn("Summarization failed, skipping article", "id", id, "error", err)
			continue
		}

		art := &storage.Article{
			HNID:      item.ID,
			Title:     item.Title,
			URL:       item.URL,
			Summary:   summary,
			Tags:      tags,
			HNScore:   item.Score,
			FetchedAt: time.Now(),
		}
		articleMap[id] = art
		articles = append(articles, ranker.Article{
			ID:      art.HNID,
			Tags:    art.Tags,
			HNScore: art.HNScore,
		})
		processedCount++
	}

	// 5. Rank Articles
	weights, err := w.storage.GetTagWeights()
	if err != nil {
		return fmt.Errorf("failed to get tag weights for ranking: %w", err)
	}
	weightMap := make(map[string]float64)
	for k, v := range weights {
		weightMap[k] = v.Weight
	}

	r := ranker.NewRanker(weightMap)
	ranked := r.Rank(articles)

	// 6. Send Top N
	toSend := targetCount
	if len(ranked) < toSend {
		toSend = len(ranked)
	}

	slog.Info("Sending top articles", "count", toSend)
	for i := 0; i < toSend; i++ {
		artID := ranked[i].ID
		art := articleMap[artID]

		msgID, err := sender.SendArticle(art)
		if err != nil {
			slog.Error("Failed to send article", "id", artID, "error", err)
			continue
		}
		art.SentAt = time.Now()
		art.TelegramMessageID = msgID

		// 7. Persist
		if err := w.storage.SaveArticle(art); err != nil {
			slog.Error("Failed to save article to storage", "id", artID, "error", err)
		}
	}

	slog.Info("Digest workflow cycle completed")
	return nil
}

func (w *Workflow) applyDecay() error {
	weights, err := w.storage.GetTagWeights()
	if err != nil {
		return err
	}

	for _, tw := range weights {
		newWeight := tw.Weight * (1 - w.config.TagDecayRate)
		if newWeight < w.config.MinTagWeight {
			newWeight = w.config.MinTagWeight
		}
		if err := w.storage.UpdateTagWeight(tw.Tag, newWeight, tw.Occurrences); err != nil {
			slog.Warn("Failed to update tag weight during decay", "tag", tw.Tag, "error", err)
		}
	}
	return nil
}
