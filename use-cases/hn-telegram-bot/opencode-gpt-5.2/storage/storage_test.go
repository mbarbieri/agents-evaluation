package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	ctx := context.Background()
	st, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestUpsertArticleAndLookupByTelegramMessageID(t *testing.T) {
	t.Parallel()
	st := openTestStore(t)
	ctx := context.Background()

	fetched := time.Unix(100, 0).UTC()
	sent := time.Unix(200, 0).UTC()
	msgID := 123
	a := Article{
		ID:                42,
		Title:             "Hello",
		URL:               "https://example.com",
		Summary:           "Sum",
		Tags:              []string{"go", "hn"},
		HNScore:           10,
		HNCommentCount:    5,
		FetchedAt:         fetched,
		SentAt:            &sent,
		TelegramMessageID: &msgID,
	}
	if err := st.UpsertArticle(ctx, a); err != nil {
		t.Fatalf("UpsertArticle: %v", err)
	}

	got, err := st.ArticleByTelegramMessageID(ctx, msgID)
	if err != nil {
		t.Fatalf("ArticleByTelegramMessageID: %v", err)
	}
	if got.ID != 42 || got.Title != "Hello" {
		t.Fatalf("unexpected article: %+v", got)
	}
	if got.TelegramMessageID == nil || *got.TelegramMessageID != msgID {
		t.Fatalf("message id not set: %+v", got.TelegramMessageID)
	}
	if got.SentAt == nil || !got.SentAt.Equal(sent) {
		t.Fatalf("sent_at mismatch: %+v", got.SentAt)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "go" {
		t.Fatalf("tags mismatch: %+v", got.Tags)
	}
}

func TestArticleByTelegramMessageID_NotFound(t *testing.T) {
	t.Parallel()
	st := openTestStore(t)
	ctx := context.Background()

	_, err := st.ArticleByTelegramMessageID(ctx, 999)
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMarkArticleSent(t *testing.T) {
	t.Parallel()
	st := openTestStore(t)
	ctx := context.Background()

	fetched := time.Unix(100, 0).UTC()
	a := Article{
		ID:             1,
		Title:          "T",
		URL:            "U",
		Summary:        "S",
		Tags:           []string{"x"},
		HNScore:        1,
		HNCommentCount: 0,
		FetchedAt:      fetched,
	}
	if err := st.UpsertArticle(ctx, a); err != nil {
		t.Fatalf("UpsertArticle: %v", err)
	}

	sent := time.Unix(200, 0).UTC()
	if err := st.MarkArticleSent(ctx, 1, sent, 777); err != nil {
		t.Fatalf("MarkArticleSent: %v", err)
	}

	got, err := st.ArticleByTelegramMessageID(ctx, 777)
	if err != nil {
		t.Fatalf("ArticleByTelegramMessageID: %v", err)
	}
	if got.SentAt == nil || !got.SentAt.Equal(sent) {
		t.Fatalf("sent_at mismatch: %+v", got.SentAt)
	}
}

func TestSentArticleIDsSince(t *testing.T) {
	t.Parallel()
	st := openTestStore(t)
	ctx := context.Background()

	base := time.Unix(1000, 0).UTC()
	for i := 1; i <= 3; i++ {
		a := Article{
			ID:             i,
			Title:          "T",
			URL:            "U",
			Summary:        "S",
			Tags:           []string{"x"},
			HNScore:        1,
			HNCommentCount: 0,
			FetchedAt:      base,
		}
		if err := st.UpsertArticle(ctx, a); err != nil {
			t.Fatalf("UpsertArticle: %v", err)
		}
	}
	if err := st.MarkArticleSent(ctx, 1, base.Add(-2*time.Hour), 11); err != nil {
		t.Fatalf("MarkArticleSent: %v", err)
	}
	if err := st.MarkArticleSent(ctx, 2, base.Add(-30*time.Minute), 22); err != nil {
		t.Fatalf("MarkArticleSent: %v", err)
	}

	ids, err := st.SentArticleIDsSince(ctx, base.Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("SentArticleIDsSince: %v", err)
	}
	if _, ok := ids[2]; !ok {
		t.Fatalf("expected 2 in ids")
	}
	if _, ok := ids[1]; ok {
		t.Fatalf("did not expect 1 in ids")
	}
}

func TestLikeIdempotency(t *testing.T) {
	t.Parallel()
	st := openTestStore(t)
	ctx := context.Background()

	liked, err := st.IsLiked(ctx, 1)
	if err != nil {
		t.Fatalf("IsLiked: %v", err)
	}
	if liked {
		t.Fatalf("expected not liked")
	}

	now := time.Unix(100, 0).UTC()
	if err := st.RecordLike(ctx, 1, now); err != nil {
		t.Fatalf("RecordLike: %v", err)
	}
	if err := st.RecordLike(ctx, 1, now.Add(time.Hour)); err != nil {
		t.Fatalf("RecordLike second: %v", err)
	}

	liked, err = st.IsLiked(ctx, 1)
	if err != nil {
		t.Fatalf("IsLiked: %v", err)
	}
	if !liked {
		t.Fatalf("expected liked")
	}

	c, err := st.LikeCount(ctx)
	if err != nil {
		t.Fatalf("LikeCount: %v", err)
	}
	if c != 1 {
		t.Fatalf("expected 1 like, got %d", c)
	}
}

func TestBoostTagsOnLikeAndTopTags(t *testing.T) {
	t.Parallel()
	st := openTestStore(t)
	ctx := context.Background()

	if err := st.BoostTagsOnLike(ctx, []string{"go", "db"}, 0.2); err != nil {
		t.Fatalf("BoostTagsOnLike: %v", err)
	}
	if err := st.BoostTagsOnLike(ctx, []string{"go"}, 0.2); err != nil {
		t.Fatalf("BoostTagsOnLike: %v", err)
	}

	top, err := st.TopTags(ctx, 10)
	if err != nil {
		t.Fatalf("TopTags: %v", err)
	}
	if len(top) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(top))
	}
	if top[0].Tag != "go" {
		t.Fatalf("expected go first, got %q", top[0].Tag)
	}
	if top[0].Weight <= top[1].Weight {
		t.Fatalf("expected go weight > db weight")
	}
	if top[0].Occurrences != 2 {
		t.Fatalf("expected go occurrences 2, got %d", top[0].Occurrences)
	}
}

func TestApplyDecay_RespectsFloor(t *testing.T) {
	t.Parallel()
	st := openTestStore(t)
	ctx := context.Background()

	if err := st.BoostTagsOnLike(ctx, []string{"go"}, 0.2); err != nil {
		t.Fatalf("BoostTagsOnLike: %v", err)
	}
	if err := st.ApplyDecay(ctx, 0.5, 0.1); err != nil {
		t.Fatalf("ApplyDecay: %v", err)
	}
	weights, err := st.GetTagWeights(ctx, []string{"go"})
	if err != nil {
		t.Fatalf("GetTagWeights: %v", err)
	}
	if weights["go"] < 0.1 {
		t.Fatalf("expected floor respected")
	}
}

func TestSettingsCRUD(t *testing.T) {
	t.Parallel()
	st := openTestStore(t)
	ctx := context.Background()

	_, err := st.GetSetting(ctx, "chat_id")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if err := st.SetSetting(ctx, "chat_id", "123"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	if err := st.SetSetting(ctx, "chat_id", "456"); err != nil {
		t.Fatalf("SetSetting update: %v", err)
	}
	v, err := st.GetSetting(ctx, "chat_id")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if v != "456" {
		t.Fatalf("expected 456, got %q", v)
	}
}
