package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	store, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store
}

func TestUpsertAndGetByMessageID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)

	now := time.Now().UTC()
	messageID := 42
	article := Article{
		ID:        1,
		Title:     "Title",
		URL:       "https://example.com",
		Summary:   "Summary",
		Tags:      []string{"go", "hn"},
		Score:     99,
		FetchedAt: now,
		SentAt:    &now,
		MessageID: &messageID,
	}

	if err := store.UpsertArticle(ctx, article); err != nil {
		t.Fatalf("UpsertArticle: %v", err)
	}

	fetched, ok, err := store.GetArticleByMessageID(ctx, messageID)
	if err != nil {
		t.Fatalf("GetArticleByMessageID: %v", err)
	}
	if !ok {
		t.Fatalf("expected article")
	}
	if fetched.ID != article.ID {
		t.Fatalf("expected id %d got %d", article.ID, fetched.ID)
	}
	if fetched.Title != article.Title {
		t.Fatalf("expected title %q got %q", article.Title, fetched.Title)
	}
	if fetched.MessageID == nil || *fetched.MessageID != messageID {
		t.Fatalf("expected message id %d", messageID)
	}
}

func TestGetArticleByMessageIDNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)

	_, ok, err := store.GetArticleByMessageID(ctx, 999)
	if err != nil {
		t.Fatalf("GetArticleByMessageID: %v", err)
	}
	if ok {
		t.Fatalf("expected not found")
	}
}

func TestMarkArticleSent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)

	now := time.Now().UTC()
	article := Article{
		ID:        2,
		Title:     "Title",
		URL:       "https://example.com",
		Summary:   "Summary",
		Tags:      []string{"go"},
		Score:     10,
		FetchedAt: now,
	}
	if err := store.UpsertArticle(ctx, article); err != nil {
		t.Fatalf("UpsertArticle: %v", err)
	}
	if err := store.MarkArticleSent(ctx, article.ID, now, 123); err != nil {
		t.Fatalf("MarkArticleSent: %v", err)
	}
	updated, ok, err := store.GetArticleByMessageID(ctx, 123)
	if err != nil {
		t.Fatalf("GetArticleByMessageID: %v", err)
	}
	if !ok {
		t.Fatalf("expected article")
	}
	if updated.SentAt == nil {
		t.Fatalf("expected sent_at")
	}
}

func TestRecentSentArticleIDs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)

	now := time.Now().UTC()
	old := now.Add(-10 * 24 * time.Hour)
	articleRecent := Article{
		ID:        3,
		Title:     "Recent",
		URL:       "https://example.com/1",
		Summary:   "Summary",
		Tags:      []string{"go"},
		Score:     1,
		FetchedAt: now,
		SentAt:    &now,
	}
	articleOld := Article{
		ID:        4,
		Title:     "Old",
		URL:       "https://example.com/2",
		Summary:   "Summary",
		Tags:      []string{"go"},
		Score:     1,
		FetchedAt: now,
		SentAt:    &old,
	}

	if err := store.UpsertArticle(ctx, articleRecent); err != nil {
		t.Fatalf("UpsertArticle: %v", err)
	}
	if err := store.UpsertArticle(ctx, articleOld); err != nil {
		t.Fatalf("UpsertArticle: %v", err)
	}

	ids, err := store.GetRecentlySentArticleIDs(ctx, now.Add(-7*24*time.Hour))
	if err != nil {
		t.Fatalf("GetRecentlySentArticleIDs: %v", err)
	}
	if len(ids) != 1 || ids[0] != articleRecent.ID {
		t.Fatalf("expected recent id %d got %v", articleRecent.ID, ids)
	}
}

func TestLikesAndCounts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)

	liked, err := store.IsLiked(ctx, 10)
	if err != nil {
		t.Fatalf("IsLiked: %v", err)
	}
	if liked {
		t.Fatalf("expected not liked")
	}

	if err := store.AddLike(ctx, 10, time.Now().UTC()); err != nil {
		t.Fatalf("AddLike: %v", err)
	}
	liked, err = store.IsLiked(ctx, 10)
	if err != nil {
		t.Fatalf("IsLiked: %v", err)
	}
	if !liked {
		t.Fatalf("expected liked")
	}

	count, err := store.CountLikes(ctx)
	if err != nil {
		t.Fatalf("CountLikes: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count 1 got %d", count)
	}
}

func TestBoostAndDecayTags(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)

	if err := store.BoostTags(ctx, []string{"go", "ai"}, 0.2); err != nil {
		t.Fatalf("BoostTags: %v", err)
	}

	if err := store.DecayTags(ctx, 0.5, 0.1); err != nil {
		t.Fatalf("DecayTags: %v", err)
	}

	tags, err := store.GetTopTags(ctx, 10)
	if err != nil {
		t.Fatalf("GetTopTags: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags got %d", len(tags))
	}
	for _, tag := range tags {
		if tag.Weight < 0.1 {
			t.Fatalf("expected weight >= 0.1 got %f", tag.Weight)
		}
		if tag.Count != 1 {
			t.Fatalf("expected count 1 got %d", tag.Count)
		}
	}

	weights, err := store.GetTagWeights(ctx)
	if err != nil {
		t.Fatalf("GetTagWeights: %v", err)
	}
	if len(weights) != 2 {
		t.Fatalf("expected 2 weights got %d", len(weights))
	}
}

func TestSettings(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)

	_, ok, err := store.GetSetting(ctx, "chat_id")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if ok {
		t.Fatalf("expected no setting")
	}

	if err := store.SetSetting(ctx, "chat_id", "123"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	value, ok, err := store.GetSetting(ctx, "chat_id")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if !ok {
		t.Fatalf("expected setting")
	}
	if value != "123" {
		t.Fatalf("expected value 123 got %q", value)
	}
}
