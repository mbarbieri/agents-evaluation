package bot

import (
	"context"
	"testing"
	"time"
)

type mockReactionStore struct {
	article Article
	found   bool
	liked   bool
	boosted []string
}

func (m *mockReactionStore) ArticleByMessageID(ctx context.Context, messageID int) (Article, bool, error) {
	return m.article, m.found, nil
}

func (m *mockReactionStore) IsLiked(ctx context.Context, articleID int64) (bool, error) {
	return m.liked, nil
}

func (m *mockReactionStore) AddLike(ctx context.Context, articleID int64, likedAt time.Time) error {
	m.liked = true
	return nil
}

func (m *mockReactionStore) BoostTags(ctx context.Context, tags []string, boost float64) error {
	m.boosted = append(m.boosted, tags...)
	return nil
}

func TestHandleReactionIgnoresNonThumbsUp(t *testing.T) {
	t.Parallel()
	store := &mockReactionStore{}
	h := NewReactionHandler(store, 0.2)

	if err := h.Handle(context.Background(), 1, "❤️"); err != nil {
		t.Fatalf("Handle: %v", err)
	}
}

func TestHandleReactionBoostsTags(t *testing.T) {
	t.Parallel()
	store := &mockReactionStore{article: Article{ID: 1, Tags: []string{"go"}}, found: true}
	h := NewReactionHandler(store, 0.2)

	if err := h.Handle(context.Background(), 2, "👍"); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(store.boosted) != 1 {
		t.Fatalf("expected boost")
	}
}
