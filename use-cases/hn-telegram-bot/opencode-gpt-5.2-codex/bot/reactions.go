package bot

import (
	"context"
	"errors"
	"time"
)

type Article struct {
	ID   int64
	Tags []string
}

type ReactionStore interface {
	ArticleByMessageID(ctx context.Context, messageID int) (Article, bool, error)
	IsLiked(ctx context.Context, articleID int64) (bool, error)
	AddLike(ctx context.Context, articleID int64, likedAt time.Time) error
	BoostTags(ctx context.Context, tags []string, boost float64) error
}

type ReactionHandler struct {
	store ReactionStore
	boost float64
}

func NewReactionHandler(store ReactionStore, boost float64) *ReactionHandler {
	return &ReactionHandler{store: store, boost: boost}
}

func (h *ReactionHandler) Handle(ctx context.Context, messageID int, emoji string) error {
	if h == nil || h.store == nil {
		return errors.New("reaction handler not initialized")
	}
	if emoji != "👍" {
		return nil
	}
	article, found, err := h.store.ArticleByMessageID(ctx, messageID)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	liked, err := h.store.IsLiked(ctx, article.ID)
	if err != nil {
		return err
	}
	if liked {
		return nil
	}
	if err := h.store.BoostTags(ctx, article.Tags, h.boost); err != nil {
		return err
	}
	return h.store.AddLike(ctx, article.ID, time.Now().UTC())
}
