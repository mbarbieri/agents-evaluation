package digest

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"hn-telegram-bot/ranker"
)

type Item struct {
	ID          int64
	Title       string
	URL         string
	Score       int
	Descendants int
}

type Summary struct {
	Summary string
	Tags    []string
}

type Article struct {
	ID        int64
	Title     string
	URL       string
	Summary   string
	Tags      []string
	Score     int
	Comments  int
	FetchedAt time.Time
	SentAt    *time.Time
	MessageID *int
}

type Sent struct {
	ArticleID int64
	MessageID int
}

type HNClient interface {
	TopStories(ctx context.Context) ([]int64, error)
	Item(ctx context.Context, id int64) (Item, error)
}

type Scraper interface {
	Extract(ctx context.Context, url string) (string, error)
}

type Summarizer interface {
	Summarize(ctx context.Context, content string) (Summary, error)
}

type Store interface {
	ApplyDecay(ctx context.Context, rate, min float64) error
	RecentSentIDs(ctx context.Context, since time.Time) ([]int64, error)
	TagWeights(ctx context.Context) (map[string]float64, error)
	SaveArticle(ctx context.Context, article Article) error
}

type Sender interface {
	Send(ctx context.Context, article Article) (Sent, error)
}

type WorkflowConfig struct {
	ArticleCount int
	FetchLimit   int
	DecayRate    float64
	MinTagWeight float64
}

type Workflow struct {
	config    WorkflowConfig
	hn        HNClient
	scraper   Scraper
	summarize Summarizer
	store     Store
	sender    Sender
}

func NewWorkflow(cfg WorkflowConfig, hn HNClient, scraper Scraper, summarizer Summarizer, store Store, sender Sender) *Workflow {
	return &Workflow{config: cfg, hn: hn, scraper: scraper, summarize: summarizer, store: store, sender: sender}
}

func (w *Workflow) Run(ctx context.Context) error {
	if w == nil {
		return errors.New("workflow not initialized")
	}
	if err := w.store.ApplyDecay(ctx, w.config.DecayRate, w.config.MinTagWeight); err != nil {
		slog.Error("decay failed", "error", err)
	}
	ids, err := w.hn.TopStories(ctx)
	if err != nil {
		return err
	}
	if w.config.FetchLimit > 0 && len(ids) > w.config.FetchLimit {
		ids = ids[:w.config.FetchLimit]
	}
	recentIDs, err := w.store.RecentSentIDs(ctx, time.Now().UTC().Add(-7*24*time.Hour))
	if err != nil {
		return err
	}
	filtered := filterIDs(ids, recentIDs)
	articles := w.processItems(ctx, filtered)
	weights, err := w.store.TagWeights(ctx)
	if err != nil {
		return err
	}
	articles = w.rank(articles, weights)
	if w.config.ArticleCount > 0 && len(articles) > w.config.ArticleCount {
		articles = articles[:w.config.ArticleCount]
	}
	for _, article := range articles {
		sent, err := w.sender.Send(ctx, article)
		if err != nil {
			slog.Error("send failed", "error", err, "id", article.ID)
			continue
		}
		article.SentAt = timePtr(time.Now().UTC())
		article.MessageID = &sent.MessageID
		if err := w.store.SaveArticle(ctx, article); err != nil {
			slog.Error("save article failed", "error", err, "id", article.ID)
		}
	}
	return nil
}

func (w *Workflow) processItems(ctx context.Context, ids []int64) []Article {
	var articles []Article
	for _, id := range ids {
		item, err := w.hn.Item(ctx, id)
		if err != nil {
			slog.Error("hn item failed", "error", err, "id", id)
			continue
		}
		content, err := w.scraper.Extract(ctx, item.URL)
		if err != nil || content == "" {
			content = item.Title
		}
		summary, err := w.summarize.Summarize(ctx, content)
		if err != nil {
			slog.Error("summarize failed", "error", err, "id", id)
			continue
		}
		articles = append(articles, Article{
			ID:        item.ID,
			Title:     item.Title,
			URL:       item.URL,
			Summary:   summary.Summary,
			Tags:      summary.Tags,
			Score:     item.Score,
			Comments:  item.Descendants,
			FetchedAt: time.Now().UTC(),
		})
	}
	return articles
}

func (w *Workflow) rank(articles []Article, weights map[string]float64) []Article {
	if len(articles) == 0 {
		return articles
	}
	entries := make([]ranker.Article, 0, len(articles))
	byID := make(map[int64]Article, len(articles))
	for _, article := range articles {
		entries = append(entries, ranker.Article{ID: article.ID, Tags: article.Tags, Score: article.Score})
		byID[article.ID] = article
	}
	ranker.Rank(entries, weights)
	sorted := make([]Article, 0, len(entries))
	for _, entry := range entries {
		sorted = append(sorted, byID[entry.ID])
	}
	return sorted
}

func filterIDs(ids []int64, recent []int64) []int64 {
	if len(recent) == 0 {
		return ids
	}
	seen := make(map[int64]struct{}, len(recent))
	for _, id := range recent {
		seen[id] = struct{}{}
	}
	var filtered []int64
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		filtered = append(filtered, id)
	}
	return filtered
}

func timePtr(value time.Time) *time.Time {
	return &value
}
