package bot

import (
	"context"
	"errors"
	"time"

	"hn-telegram-bot/storage"
)

type SettingsAdapter struct {
	Store *storage.Store
}

func (a *SettingsAdapter) Get(ctx context.Context, key string) (string, bool, error) {
	if a == nil || a.Store == nil {
		return "", false, errors.New("store not initialized")
	}
	return a.Store.GetSetting(ctx, key)
}

func (a *SettingsAdapter) Set(ctx context.Context, key, value string) error {
	if a == nil || a.Store == nil {
		return errors.New("store not initialized")
	}
	return a.Store.SetSetting(ctx, key, value)
}

type StatsAdapter struct {
	Store *storage.Store
}

func (a *StatsAdapter) TopTags(ctx context.Context, limit int) ([]TagWeight, error) {
	if a == nil || a.Store == nil {
		return nil, errors.New("store not initialized")
	}
	tags, err := a.Store.GetTopTags(ctx, limit)
	if err != nil {
		return nil, err
	}
	result := make([]TagWeight, 0, len(tags))
	for _, tag := range tags {
		result = append(result, TagWeight{Tag: tag.Tag, Weight: tag.Weight})
	}
	return result, nil
}

func (a *StatsAdapter) LikeCount(ctx context.Context) (int, error) {
	if a == nil || a.Store == nil {
		return 0, errors.New("store not initialized")
	}
	return a.Store.CountLikes(ctx)
}

type ReactionAdapter struct {
	Store *storage.Store
}

func (a *ReactionAdapter) ArticleByMessageID(ctx context.Context, messageID int) (Article, bool, error) {
	if a == nil || a.Store == nil {
		return Article{}, false, errors.New("store not initialized")
	}
	article, ok, err := a.Store.GetArticleByMessageID(ctx, messageID)
	if err != nil {
		return Article{}, false, err
	}
	if !ok {
		return Article{}, false, nil
	}
	return Article{ID: article.ID, Tags: article.Tags}, true, nil
}

func (a *ReactionAdapter) IsLiked(ctx context.Context, articleID int64) (bool, error) {
	if a == nil || a.Store == nil {
		return false, errors.New("store not initialized")
	}
	return a.Store.IsLiked(ctx, articleID)
}

func (a *ReactionAdapter) AddLike(ctx context.Context, articleID int64, likedAt time.Time) error {
	if a == nil || a.Store == nil {
		return errors.New("store not initialized")
	}
	return a.Store.AddLike(ctx, articleID, likedAt)
}

func (a *ReactionAdapter) BoostTags(ctx context.Context, tags []string, boost float64) error {
	if a == nil || a.Store == nil {
		return errors.New("store not initialized")
	}
	return a.Store.BoostTags(ctx, tags, boost)
}
