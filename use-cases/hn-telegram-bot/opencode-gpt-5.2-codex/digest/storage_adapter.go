package digest

import (
	"context"
	"errors"
	"time"

	"hn-telegram-bot/storage"
)

type StorageAdapter struct {
	Store *storage.Store
}

func (a *StorageAdapter) ApplyDecay(ctx context.Context, rate, min float64) error {
	if a == nil || a.Store == nil {
		return errors.New("store not initialized")
	}
	return a.Store.DecayTags(ctx, rate, min)
}

func (a *StorageAdapter) RecentSentIDs(ctx context.Context, since time.Time) ([]int64, error) {
	if a == nil || a.Store == nil {
		return nil, errors.New("store not initialized")
	}
	return a.Store.GetRecentlySentArticleIDs(ctx, since)
}

func (a *StorageAdapter) TagWeights(ctx context.Context) (map[string]float64, error) {
	if a == nil || a.Store == nil {
		return nil, errors.New("store not initialized")
	}
	return a.Store.GetTagWeights(ctx)
}

func (a *StorageAdapter) SaveArticle(ctx context.Context, article Article) error {
	if a == nil || a.Store == nil {
		return errors.New("store not initialized")
	}
	return a.Store.UpsertArticle(ctx, storage.Article{
		ID:        article.ID,
		Title:     article.Title,
		URL:       article.URL,
		Summary:   article.Summary,
		Tags:      article.Tags,
		Score:     article.Score,
		FetchedAt: article.FetchedAt,
		SentAt:    article.SentAt,
		MessageID: article.MessageID,
	})
}
