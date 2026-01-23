package digest

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

type HNClient interface {
	TopStories(ctx context.Context) ([]int, error)
	Item(ctx context.Context, id int) (HNItem, error)
}

type HNItem struct {
	ID          int
	Title       string
	URL         string
	Score       int
	Descendants int
}

type Scraper interface {
	Extract(ctx context.Context, url string) (string, error)
}

type Summarizer interface {
	Summarize(ctx context.Context, content string) (SummaryResult, error)
}

type SummaryResult struct {
	Summary string
	Tags    []string
}

type Storage interface {
	ApplyDecay(ctx context.Context, decayRate float64, minWeight float64) error
	SentArticleIDsSince(ctx context.Context, since time.Time) (map[int]struct{}, error)
	UpsertArticle(ctx context.Context, a StoredArticle) error
	MarkArticleSent(ctx context.Context, articleID int, sentAt time.Time, telegramMessageID int) error
}

type StoredArticle struct {
	ID             int
	Title          string
	URL            string
	Summary        string
	Tags           []string
	HNScore        int
	HNCommentCount int
	FetchedAt      time.Time
	SentAt         *time.Time
	MessageID      *int
}

type Ranker interface {
	Rank(ctx context.Context, articles []RankArticle) ([]RankArticle, error)
}

type RankArticle struct {
	ID         int
	Tags       []string
	HNScore    int
	FinalScore float64
	Title      string
	URL        string
	Summary    string
	Comments   int
}

type Sender interface {
	SendArticle(ctx context.Context, a RankArticle) (telegramMessageID int, err error)
}

type Config struct {
	ArticleCount   int
	DecayRate      float64
	MinTagWeight   float64
	RecentWindow   time.Duration
	ScrapeFallback bool
}

type Service struct {
	Log        *slog.Logger
	HN         HNClient
	Scraper    Scraper
	Summarizer Summarizer
	Ranker     Ranker
	Store      Storage
	Sender     Sender
	Cfg        Config
}

func (s Service) Run(ctx context.Context) error {
	log := s.Log
	if log == nil {
		log = slog.Default()
	}

	cfg := s.Cfg
	if cfg.ArticleCount <= 0 {
		cfg.ArticleCount = 30
	}
	if cfg.RecentWindow == 0 {
		cfg.RecentWindow = 7 * 24 * time.Hour
	}

	log.Info("digest start", "article_count", cfg.ArticleCount)

	if err := s.Store.ApplyDecay(ctx, cfg.DecayRate, cfg.MinTagWeight); err != nil {
		log.Warn("apply decay failed", "err", err)
	}

	ids, err := s.HN.TopStories(ctx)
	if err != nil {
		return err
	}
	limit := cfg.ArticleCount * 2
	if limit > len(ids) {
		limit = len(ids)
	}
	ids = ids[:limit]

	sentSince := time.Now().UTC().Add(-cfg.RecentWindow)
	recent, err := s.Store.SentArticleIDsSince(ctx, sentSince)
	if err != nil {
		return err
	}

	var candidates []RankArticle
	for _, id := range ids {
		if _, ok := recent[id]; ok {
			continue
		}
		item, err := s.HN.Item(ctx, id)
		if err != nil {
			log.Warn("hn item failed", "id", id, "err", err)
			continue
		}
		if item.URL == "" {
			continue
		}

		content, err := s.Scraper.Extract(ctx, item.URL)
		if err != nil {
			log.Warn("scrape failed", "id", id, "url", item.URL, "err", err)
			content = item.Title
		}

		sum, err := s.Summarizer.Summarize(ctx, content)
		if err != nil {
			log.Warn("summarize failed", "id", id, "err", err)
			continue
		}

		sa := StoredArticle{
			ID:             item.ID,
			Title:          item.Title,
			URL:            item.URL,
			Summary:        sum.Summary,
			Tags:           sum.Tags,
			HNScore:        item.Score,
			HNCommentCount: item.Descendants,
			FetchedAt:      time.Now().UTC(),
		}
		if err := s.Store.UpsertArticle(ctx, sa); err != nil {
			log.Warn("persist article failed", "id", id, "err", err)
		}

		candidates = append(candidates, RankArticle{
			ID:       item.ID,
			Tags:     sum.Tags,
			HNScore:  item.Score,
			Title:    item.Title,
			URL:      item.URL,
			Summary:  sum.Summary,
			Comments: item.Descendants,
		})
	}

	ranked, err := s.Ranker.Rank(ctx, candidates)
	if err != nil {
		return err
	}
	if len(ranked) > cfg.ArticleCount {
		ranked = ranked[:cfg.ArticleCount]
	}

	for _, a := range ranked {
		msgID, err := s.Sender.SendArticle(ctx, a)
		if err != nil {
			log.Warn("send article failed", "id", a.ID, "err", err)
			continue
		}
		sentAt := time.Now().UTC()
		if err := s.Store.MarkArticleSent(ctx, a.ID, sentAt, msgID); err != nil {
			log.Warn("mark sent failed", "id", a.ID, "err", err)
		}
	}

	log.Info("digest done", "candidates", len(candidates), "sent", len(ranked))
	return nil
}

func (s Service) Validate() error {
	if s.HN == nil || s.Scraper == nil || s.Summarizer == nil || s.Ranker == nil || s.Store == nil || s.Sender == nil {
		return fmt.Errorf("missing dependency")
	}
	return nil
}
