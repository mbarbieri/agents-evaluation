package storage

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *Storage {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	storage := &Storage{db: db}
	if err := storage.initTables(); err != nil {
		t.Fatal(err)
	}
	return storage
}

func TestInitTables(t *testing.T) {
	storage := setupTestDB(t)
	defer storage.db.Close()

	// Check if tables exist
	var count int
	err := storage.db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name IN ('articles', 'likes', 'tag_weights', 'settings')").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 4 {
		t.Errorf("expected 4 tables, got %d", count)
	}
}

func TestSaveAndGetArticle(t *testing.T) {
	storage := setupTestDB(t)
	defer storage.db.Close()

	tags := []string{"go", "programming"}
	article := Article{
		ID:        1,
		Title:     "Test Article",
		URL:       "https://example.com",
		Summary:   "Test summary",
		Tags:      tags,
		HNScore:   100,
		FetchedAt: time.Now(),
	}

	err := storage.SaveArticle(article)
	if err != nil {
		t.Fatal(err)
	}

	got, err := storage.GetArticle(1)
	if err != nil {
		t.Fatal(err)
	}

	if got.ID != 1 {
		t.Errorf("expected ID 1, got %d", got.ID)
	}
	if got.Title != "Test Article" {
		t.Errorf("expected title 'Test Article', got %s", got.Title)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "go" {
		t.Errorf("expected tags ['go', 'programming'], got %v", got.Tags)
	}
	if got.HNScore != 100 {
		t.Errorf("expected HNScore 100, got %d", got.HNScore)
	}
	if got.SentAt != nil {
		t.Error("expected SentAt nil")
	}
}

func TestUpdateArticleSent(t *testing.T) {
	storage := setupTestDB(t)
	defer storage.db.Close()

	article := Article{
		ID:        1,
		Title:     "Test",
		URL:       "https://example.com",
		Summary:   "Summary",
		Tags:      []string{},
		HNScore:   50,
		FetchedAt: time.Now(),
	}
	storage.SaveArticle(article)

	sentAt := time.Now()
	messageID := 123
	err := storage.UpdateArticleSent(1, sentAt, messageID)
	if err != nil {
		t.Fatal(err)
	}

	got, err := storage.GetArticle(1)
	if err != nil {
		t.Fatal(err)
	}
	if got.SentAt == nil || !got.SentAt.Equal(sentAt) {
		t.Errorf("expected SentAt %v, got %v", sentAt, got.SentAt)
	}
	if got.MessageID == nil || *got.MessageID != 123 {
		t.Errorf("expected MessageID 123, got %v", got.MessageID)
	}
}

func TestGetRecentArticles(t *testing.T) {
	storage := setupTestDB(t)
	defer storage.db.Close()

	now := time.Now()
	old := now.AddDate(0, 0, -10)

	// Article sent recently
	recent := Article{
		ID:        1,
		Title:     "Recent",
		URL:       "https://example.com/1",
		Summary:   "Summary",
		Tags:      []string{},
		HNScore:   50,
		FetchedAt: now,
		SentAt:    &now,
	}
	storage.SaveArticle(recent)

	// Article sent long ago
	past := Article{
		ID:        2,
		Title:     "Past",
		URL:       "https://example.com/2",
		Summary:   "Summary",
		Tags:      []string{},
		HNScore:   50,
		FetchedAt: old,
		SentAt:    &old,
	}
	storage.SaveArticle(past)

	// Article not sent
	unsent := Article{
		ID:        3,
		Title:     "Unsent",
		URL:       "https://example.com/3",
		Summary:   "Summary",
		Tags:      []string{},
		HNScore:   50,
		FetchedAt: now,
	}
	storage.SaveArticle(unsent)

	recentArticles, err := storage.GetRecentArticles(7)
	if err != nil {
		t.Fatal(err)
	}

	if len(recentArticles) != 1 {
		t.Errorf("expected 1 recent article, got %d", len(recentArticles))
	}
	if recentArticles[0].ID != 1 {
		t.Errorf("expected recent article ID 1, got %d", recentArticles[0].ID)
	}
}

func TestAddLikeAndIsLiked(t *testing.T) {
	storage := setupTestDB(t)
	defer storage.db.Close()

	// Initially not liked
	liked, err := storage.IsLiked(1)
	if err != nil {
		t.Fatal(err)
	}
	if liked {
		t.Error("expected not liked initially")
	}

	// Add like
	err = storage.AddLike(1)
	if err != nil {
		t.Fatal(err)
	}

	// Now liked
	liked, err = storage.IsLiked(1)
	if err != nil {
		t.Fatal(err)
	}
	if !liked {
		t.Error("expected liked after adding")
	}

	// Add again, should be idempotent
	err = storage.AddLike(1)
	if err != nil {
		t.Fatal(err)
	}
	liked, err = storage.IsLiked(1)
	if err != nil {
		t.Fatal(err)
	}
	if !liked {
		t.Error("expected still liked")
	}
}

func TestGetAndUpdateTagWeights(t *testing.T) {
	storage := setupTestDB(t)
	defer storage.db.Close()

	// Initially empty
	weights, err := storage.GetTagWeights()
	if err != nil {
		t.Fatal(err)
	}
	if len(weights) != 0 {
		t.Errorf("expected no tag weights, got %d", len(weights))
	}

	// Update a tag
	err = storage.UpdateTagWeight("go", 1.5, 2)
	if err != nil {
		t.Fatal(err)
	}

	weights, err = storage.GetTagWeights()
	if err != nil {
		t.Fatal(err)
	}
	if len(weights) != 1 {
		t.Errorf("expected 1 tag weight, got %d", len(weights))
	}
	tw, ok := weights["go"]
	if !ok {
		t.Error("expected 'go' tag")
	}
	if tw.Weight != 1.5 {
		t.Errorf("expected weight 1.5, got %f", tw.Weight)
	}
	if tw.Count != 2 {
		t.Errorf("expected count 2, got %d", tw.Count)
	}

	// Update again
	err = storage.UpdateTagWeight("go", 2.0, 3)
	if err != nil {
		t.Fatal(err)
	}

	weights, err = storage.GetTagWeights()
	if err != nil {
		t.Fatal(err)
	}
	tw = weights["go"]
	if tw.Weight != 2.0 {
		t.Errorf("expected weight 2.0, got %f", tw.Weight)
	}
	if tw.Count != 3 {
		t.Errorf("expected count 3, got %d", tw.Count)
	}
}

func TestDecayTagWeights(t *testing.T) {
	storage := setupTestDB(t)
	defer storage.db.Close()

	storage.UpdateTagWeight("go", 1.0, 1)
	storage.UpdateTagWeight("rust", 2.0, 2)

	err := storage.DecayTagWeights(0.1, 0.5)
	if err != nil {
		t.Fatal(err)
	}

	weights, err := storage.GetTagWeights()
	if err != nil {
		t.Fatal(err)
	}

	if weights["go"].Weight != 0.9 {
		t.Errorf("expected go weight 0.9, got %f", weights["go"].Weight)
	}
	if weights["rust"].Weight != 1.8 {
		t.Errorf("expected rust weight 1.8, got %f", weights["rust"].Weight)
	}
}

func TestGetAndSetSetting(t *testing.T) {
	storage := setupTestDB(t)
	defer storage.db.Close()

	// Get non-existent
	_, err := storage.GetSetting("chat_id")
	if err != sql.ErrNoRows {
		t.Errorf("expected ErrNoRows, got %v", err)
	}

	// Set
	err = storage.SetSetting("chat_id", "123")
	if err != nil {
		t.Fatal(err)
	}

	// Get
	value, err := storage.GetSetting("chat_id")
	if err != nil {
		t.Fatal(err)
	}
	if value != "123" {
		t.Errorf("expected '123', got %s", value)
	}

	// Update
	err = storage.SetSetting("chat_id", "456")
	if err != nil {
		t.Fatal(err)
	}
	value, err = storage.GetSetting("chat_id")
	if err != nil {
		t.Fatal(err)
	}
	if value != "456" {
		t.Errorf("expected '456', got %s", value)
	}
}
