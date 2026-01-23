package bot

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeReactionStore struct {
	article  ReactionArticle
	found    bool
	liked    bool
	boosted  bool
	recorded bool
}

func (f *fakeReactionStore) ArticleByTelegramMessageID(ctx context.Context, messageID int) (ReactionArticle, error) {
	if !f.found {
		return ReactionArticle{}, errors.New("no")
	}
	return f.article, nil
}

func (f *fakeReactionStore) IsLiked(ctx context.Context, articleID int) (bool, error) {
	return f.liked, nil
}

func (f *fakeReactionStore) BoostTagsOnLike(ctx context.Context, tags []string, boost float64) error {
	f.boosted = true
	return nil
}

func (f *fakeReactionStore) RecordLike(ctx context.Context, articleID int, likedAt time.Time) error {
	f.recorded = true
	return nil
}

func TestHandleThumbsUpReaction_Idempotent(t *testing.T) {
	t.Parallel()
	st := &fakeReactionStore{found: true, liked: true, article: ReactionArticle{ID: 1, Tags: []string{"go"}}}
	if err := HandleThumbsUpReaction(context.Background(), st, 10, ReactionConfig{Boost: 0.2}); err != nil {
		t.Fatalf("HandleThumbsUpReaction: %v", err)
	}
	if st.boosted || st.recorded {
		t.Fatalf("expected no changes")
	}
}

func TestHandleThumbsUpReaction_BoostsAndRecords(t *testing.T) {
	t.Parallel()
	st := &fakeReactionStore{found: true, liked: false, article: ReactionArticle{ID: 1, Tags: []string{"go"}}}
	if err := HandleThumbsUpReaction(context.Background(), st, 10, ReactionConfig{Boost: 0.2}); err != nil {
		t.Fatalf("HandleThumbsUpReaction: %v", err)
	}
	if !st.boosted || !st.recorded {
		t.Fatalf("expected boosted and recorded")
	}
}
