package digest

import (
	"context"
	"log/slog"
	"time"

	"hn-telegram-bot/hn"
	"hn-telegram-bot/model"
	"hn-telegram-bot/ranker"
)

// HNClient defines HN API access for the digest.
type HNClient interface {
	TopStories(ctx context.Context) ([]int64, error)
	Item(ctx context.Context, id int64) (hn.Item, error)
}

// Scraper extracts article content.
type Scraper interface {
	Scrape(ctx context.Context, url string) (string, error)
}

// Summarizer generates summaries and tags.
type Summarizer interface {
	Summarize(ctx context.Context, content string) (model.SummaryResult, error)
}

// Storage provides persistence for digest operations.
type Storage interface {
	ApplyDecay(ctx context.Context, decayRate, minWeight float64) error
	ListSentArticleIDsSince(ctx context.Context, since time.Time) ([]int64, error)
	UpsertArticle(ctx context.Context, article model.Article) error
	GetTagWeights(ctx context.Context) (map[string]model.TagWeight, error)
}

// Sender delivers article messages.
type Sender interface {
	SendArticle(ctx context.Context, article model.Article) (int, error)
}

// Runner orchestrates the daily digest workflow.
type Runner struct {
	HN            HNClient
	Scraper       Scraper
	Summarizer    Summarizer
	Storage       Storage
	Sender        Sender
	Logger        *slog.Logger
	ArticleCount  int
	ArticleCountFunc func() int
	TagDecayRate  float64
	MinTagWeight  float64
	Now           func() time.Time
}

// Run executes the digest pipeline.
func (r *Runner) Run(ctx context.Context) error {
	logger := r.Logger
	if logger == nil {
		logger = slog.Default()
	}
	now := time.Now
	if r.Now != nil {
		now = r.Now
	}
	start := now()
	logger.Info("digest_start", slog.Time("time", start))

	if err := r.Storage.ApplyDecay(ctx, r.TagDecayRate, r.MinTagWeight); err != nil {
		logger.Warn("digest_decay_failed", slog.String("error", err.Error()))
	}

	ids, err := r.HN.TopStories(ctx)
	if err != nil {
		return err
	}
	logger.Info("digest_topstories", slog.Int("count", len(ids)))

	count := r.ArticleCount
	if r.ArticleCountFunc != nil {
		count = r.ArticleCountFunc()
	}
	limit := count * 2
	if limit <= 0 {
		return nil
	}
	if len(ids) > limit {
		ids = ids[:limit]
	}

	sentSince := start.AddDate(0, 0, -7)
	sentIDs, err := r.Storage.ListSentArticleIDsSince(ctx, sentSince)
	if err != nil {
		logger.Warn("digest_sent_ids_failed", slog.String("error", err.Error()))
	}
	sentMap := make(map[int64]bool)
	for _, id := range sentIDs {
		sentMap[id] = true
	}

	var articles []model.Article
	for _, id := range ids {
		if sentMap[id] {
			continue
		}
		item, err := r.HN.Item(ctx, id)
		if err != nil {
			logger.Warn("digest_item_failed", slog.Int64("id", id), slog.String("error", err.Error()))
			continue
		}
		if item.Type != "story" {
			continue
		}
		content := item.Title
		if item.URL != "" {
			text, err := r.Scraper.Scrape(ctx, item.URL)
			if err != nil {
				logger.Warn("digest_scrape_failed", slog.Int64("id", id), slog.String("error", err.Error()))
			} else if text != "" {
				content = text
			}
		}

		res, err := r.Summarizer.Summarize(ctx, content)
		if err != nil {
			logger.Warn("digest_summarize_failed", slog.Int64("id", id), slog.String("error", err.Error()))
			continue
		}
		article := model.Article{
			ID:        item.ID,
			Title:     item.Title,
			URL:       item.URL,
			Summary:   res.Summary,
			Tags:      res.Tags,
			HNScore:   item.Score,
			Comments:  item.Descendants,
			FetchedAt: start,
		}
		articles = append(articles, article)
	}

	weights, err := r.Storage.GetTagWeights(ctx)
	if err != nil {
		logger.Warn("digest_get_weights_failed", slog.String("error", err.Error()))
	}

	scored := ranker.Rank(articles, weights)
	if len(scored) == 0 {
		logger.Info("digest_no_articles")
		return nil
	}

	if len(scored) > count {
		scored = scored[:count]
	}

	for _, item := range scored {
		msgID, err := r.Sender.SendArticle(ctx, item.Article)
		if err != nil {
			logger.Warn("digest_send_failed", slog.Int64("id", item.Article.ID), slog.String("error", err.Error()))
			continue
		}
		sentAt := now()
		article := item.Article
		article.SentAt = &sentAt
		article.TelegramMsgID = msgID
		if err := r.Storage.UpsertArticle(ctx, article); err != nil {
			logger.Warn("digest_store_failed", slog.Int64("id", item.Article.ID), slog.String("error", err.Error()))
		}
	}

	logger.Info("digest_complete", slog.Int("sent", len(scored)))
	return nil
}
