package storage

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"hn-telegram-bot/model"
)

func newTestStorage(t *testing.T) *Storage {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.SetMaxOpenConns(1)
	storage := New(db)
	if err := storage.Init(context.Background()); err != nil {
		t.Fatalf("init schema: %v", err)
	}
	return storage
}

func TestInitCreatesTables(t *testing.T) {
	storage := newTestStorage(t)
	rows, err := storage.db.Query(`SELECT name FROM sqlite_master WHERE type='table'`)
	if err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	defer rows.Close()

	found := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		found[name] = true
	}
	for _, tbl := range []string{"articles", "likes", "tag_weights", "settings"} {
		if !found[tbl] {
			t.Fatalf("expected table %s", tbl)
		}
	}
}

func TestUpsertAndGetArticleByMessageID(t *testing.T) {
	storage := newTestStorage(t)
	ctx := context.Background()

	sentAt := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	article := model.Article{
		ID:            123,
		Title:         "Title",
		URL:           "https://example.com",
		Summary:       "Summary",
		Tags:          []string{"go", "news"},
		HNScore:       55,
		Comments:      10,
		FetchedAt:     time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		SentAt:        &sentAt,
		TelegramMsgID: 777,
	}
	if err := storage.UpsertArticle(ctx, article); err != nil {
		t.Fatalf("UpsertArticle: %v", err)
	}

	got, ok, err := storage.GetArticleByMessageID(ctx, 777)
	if err != nil {
		t.Fatalf("GetArticleByMessageID: %v", err)
	}
	if !ok {
		t.Fatalf("expected article")
	}
	if got.ID != article.ID || got.Title != article.Title || got.URL != article.URL {
		t.Fatalf("unexpected article: %+v", got)
	}
	if got.SentAt == nil || !got.SentAt.Equal(sentAt) {
		t.Fatalf("expected sentAt %v, got %v", sentAt, got.SentAt)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "go" {
		t.Fatalf("unexpected tags: %v", got.Tags)
	}
}

func TestListSentArticleIDsSince(t *testing.T) {
	storage := newTestStorage(t)
	ctx := context.Background()

	now := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
	past := now.AddDate(0, 0, -10)
	recent := now.AddDate(0, 0, -2)

	article1 := model.Article{ID: 1, Title: "a", URL: "u", Summary: "s", Tags: []string{}, HNScore: 1, Comments: 0, FetchedAt: past, SentAt: &past, TelegramMsgID: 1}
	article2 := model.Article{ID: 2, Title: "b", URL: "u", Summary: "s", Tags: []string{}, HNScore: 1, Comments: 0, FetchedAt: recent, SentAt: &recent, TelegramMsgID: 2}

	if err := storage.UpsertArticle(ctx, article1); err != nil {
		t.Fatalf("UpsertArticle 1: %v", err)
	}
	if err := storage.UpsertArticle(ctx, article2); err != nil {
		t.Fatalf("UpsertArticle 2: %v", err)
	}

	ids, err := storage.ListSentArticleIDsSince(ctx, now.AddDate(0, 0, -7))
	if err != nil {
		t.Fatalf("ListSentArticleIDsSince: %v", err)
	}
	if len(ids) != 1 || ids[0] != 2 {
		t.Fatalf("expected [2], got %v", ids)
	}
}

func TestLikesIdempotent(t *testing.T) {
	storage := newTestStorage(t)
	ctx := context.Background()

	liked, err := storage.IsLiked(ctx, 10)
	if err != nil {
		t.Fatalf("IsLiked: %v", err)
	}
	if liked {
		t.Fatalf("expected not liked")
	}

	if err := storage.AddLike(ctx, 10, time.Now().UTC()); err != nil {
		t.Fatalf("AddLike: %v", err)
	}
	if err := storage.AddLike(ctx, 10, time.Now().UTC()); err != nil {
		t.Fatalf("AddLike duplicate: %v", err)
	}

	liked, err = storage.IsLiked(ctx, 10)
	if err != nil {
		t.Fatalf("IsLiked after: %v", err)
	}
	if !liked {
		t.Fatalf("expected liked")
	}
}

func TestBoostTagsAndListTop(t *testing.T) {
	storage := newTestStorage(t)
	ctx := context.Background()

	if err := storage.BoostTags(ctx, []string{"go", "ai"}, 0.2); err != nil {
		t.Fatalf("BoostTags: %v", err)
	}
	if err := storage.BoostTags(ctx, []string{"go"}, 0.2); err != nil {
		t.Fatalf("BoostTags again: %v", err)
	}

	tags, err := storage.ListTopTags(ctx, 2)
	if err != nil {
		t.Fatalf("ListTopTags: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(tags))
	}
	if tags[0].Tag != "go" {
		t.Fatalf("expected go to be top, got %s", tags[0].Tag)
	}
	if tags[0].Count != 2 {
		t.Fatalf("expected count 2, got %d", tags[0].Count)
	}
}

func TestApplyDecay(t *testing.T) {
	storage := newTestStorage(t)
	ctx := context.Background()

	if err := storage.BoostTags(ctx, []string{"go"}, 0.2); err != nil {
		t.Fatalf("BoostTags: %v", err)
	}
	if err := storage.ApplyDecay(ctx, 0.5, 0.5); err != nil {
		t.Fatalf("ApplyDecay: %v", err)
	}

	weights, err := storage.GetTagWeights(ctx)
	if err != nil {
		t.Fatalf("GetTagWeights: %v", err)
	}
	w := weights["go"].Weight
	if w < 0.5 {
		t.Fatalf("expected min floor 0.5, got %v", w)
	}
}

func TestSettings(t *testing.T) {
	storage := newTestStorage(t)
	ctx := context.Background()

	_, ok, err := storage.GetSetting(ctx, "chat_id")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if ok {
		t.Fatalf("expected no setting")
	}

	if err := storage.SetSetting(ctx, "chat_id", "123"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	val, ok, err := storage.GetSetting(ctx, "chat_id")
	if err != nil {
		t.Fatalf("GetSetting after set: %v", err)
	}
	if !ok || val != "123" {
		t.Fatalf("unexpected setting %v %v", ok, val)
	}
}

func TestCountLikes(t *testing.T) {
	storage := newTestStorage(t)
	ctx := context.Background()

	if err := storage.AddLike(ctx, 1, time.Now().UTC()); err != nil {
		t.Fatalf("AddLike: %v", err)
	}
	if err := storage.AddLike(ctx, 2, time.Now().UTC()); err != nil {
		t.Fatalf("AddLike: %v", err)
	}
	count, err := storage.CountLikes(ctx)
	if err != nil {
		t.Fatalf("CountLikes: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 likes, got %d", count)
	}
}
