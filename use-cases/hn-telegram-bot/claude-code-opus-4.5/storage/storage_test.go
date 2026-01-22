package storage

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestNewDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	// Verify tables exist by querying them
	ctx := context.Background()
	_, err = db.conn.ExecContext(ctx, "SELECT 1 FROM articles LIMIT 1")
	if err != nil {
		t.Errorf("articles table not created: %v", err)
	}
	_, err = db.conn.ExecContext(ctx, "SELECT 1 FROM likes LIMIT 1")
	if err != nil {
		t.Errorf("likes table not created: %v", err)
	}
	_, err = db.conn.ExecContext(ctx, "SELECT 1 FROM tag_weights LIMIT 1")
	if err != nil {
		t.Errorf("tag_weights table not created: %v", err)
	}
	_, err = db.conn.ExecContext(ctx, "SELECT 1 FROM settings LIMIT 1")
	if err != nil {
		t.Errorf("settings table not created: %v", err)
	}
}

func TestArticleCRUD(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	ctx := context.Background()

	// Create article
	article := &Article{
		ID:        12345,
		Title:     "Test Article",
		URL:       "https://example.com/test",
		Summary:   "This is a test summary",
		Tags:      []string{"go", "testing"},
		HNScore:   100,
		FetchedAt: time.Now(),
	}

	err := db.SaveArticle(ctx, article)
	if err != nil {
		t.Fatalf("SaveArticle failed: %v", err)
	}

	// Read article
	retrieved, err := db.GetArticle(ctx, 12345)
	if err != nil {
		t.Fatalf("GetArticle failed: %v", err)
	}
	if retrieved.Title != article.Title {
		t.Errorf("Title = %q, want %q", retrieved.Title, article.Title)
	}
	if retrieved.URL != article.URL {
		t.Errorf("URL = %q, want %q", retrieved.URL, article.URL)
	}
	if retrieved.Summary != article.Summary {
		t.Errorf("Summary = %q, want %q", retrieved.Summary, article.Summary)
	}
	if len(retrieved.Tags) != 2 || retrieved.Tags[0] != "go" {
		t.Errorf("Tags = %v, want %v", retrieved.Tags, article.Tags)
	}

	// Get non-existent article
	_, err = db.GetArticle(ctx, 99999)
	if err != ErrNotFound {
		t.Errorf("GetArticle for non-existent should return ErrNotFound, got: %v", err)
	}
}

func TestArticleByMessageID(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	ctx := context.Background()

	// Save article with message ID
	article := &Article{
		ID:              12345,
		Title:           "Test Article",
		URL:             "https://example.com/test",
		Tags:            []string{"go"},
		FetchedAt:       time.Now(),
		SentAt:          ptrTime(time.Now()),
		TelegramMsgID:   ptrInt64(789),
	}
	if err := db.SaveArticle(ctx, article); err != nil {
		t.Fatalf("SaveArticle failed: %v", err)
	}

	// Find by message ID
	retrieved, err := db.GetArticleByMessageID(ctx, 789)
	if err != nil {
		t.Fatalf("GetArticleByMessageID failed: %v", err)
	}
	if retrieved.ID != 12345 {
		t.Errorf("ID = %d, want %d", retrieved.ID, 12345)
	}

	// Non-existent message ID
	_, err = db.GetArticleByMessageID(ctx, 999)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestGetRecentlySentArticleIDs(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	ctx := context.Background()

	now := time.Now()

	// Article sent 3 days ago (within range)
	article1 := &Article{
		ID:        1,
		Title:     "Recent",
		URL:       "https://example.com/1",
		Tags:      []string{},
		FetchedAt: now,
		SentAt:    ptrTime(now.Add(-3 * 24 * time.Hour)),
	}
	// Article sent 10 days ago (outside range)
	article2 := &Article{
		ID:        2,
		Title:     "Old",
		URL:       "https://example.com/2",
		Tags:      []string{},
		FetchedAt: now,
		SentAt:    ptrTime(now.Add(-10 * 24 * time.Hour)),
	}
	// Article not sent yet
	article3 := &Article{
		ID:        3,
		Title:     "Not Sent",
		URL:       "https://example.com/3",
		Tags:      []string{},
		FetchedAt: now,
	}

	for _, a := range []*Article{article1, article2, article3} {
		if err := db.SaveArticle(ctx, a); err != nil {
			t.Fatalf("SaveArticle failed: %v", err)
		}
	}

	ids, err := db.GetRecentlySentArticleIDs(ctx, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("GetRecentlySentArticleIDs failed: %v", err)
	}

	if len(ids) != 1 || ids[0] != 1 {
		t.Errorf("got IDs %v, want [1]", ids)
	}
}

func TestMarkArticleSent(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	ctx := context.Background()

	article := &Article{
		ID:        12345,
		Title:     "Test",
		URL:       "https://example.com",
		Tags:      []string{},
		FetchedAt: time.Now(),
	}
	if err := db.SaveArticle(ctx, article); err != nil {
		t.Fatalf("SaveArticle failed: %v", err)
	}

	msgID := int64(456)
	if err := db.MarkArticleSent(ctx, 12345, msgID); err != nil {
		t.Fatalf("MarkArticleSent failed: %v", err)
	}

	retrieved, _ := db.GetArticle(ctx, 12345)
	if retrieved.SentAt == nil {
		t.Error("SentAt should be set")
	}
	if retrieved.TelegramMsgID == nil || *retrieved.TelegramMsgID != msgID {
		t.Errorf("TelegramMsgID = %v, want %d", retrieved.TelegramMsgID, msgID)
	}
}

func TestLikeOperations(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	ctx := context.Background()

	// Save article first
	article := &Article{
		ID:        12345,
		Title:     "Test",
		URL:       "https://example.com",
		Tags:      []string{"go"},
		FetchedAt: time.Now(),
	}
	if err := db.SaveArticle(ctx, article); err != nil {
		t.Fatalf("SaveArticle failed: %v", err)
	}

	// Check not liked
	liked, err := db.IsArticleLiked(ctx, 12345)
	if err != nil {
		t.Fatalf("IsArticleLiked failed: %v", err)
	}
	if liked {
		t.Error("article should not be liked initially")
	}

	// Like article
	if err := db.LikeArticle(ctx, 12345); err != nil {
		t.Fatalf("LikeArticle failed: %v", err)
	}

	// Check liked
	liked, err = db.IsArticleLiked(ctx, 12345)
	if err != nil {
		t.Fatalf("IsArticleLiked failed: %v", err)
	}
	if !liked {
		t.Error("article should be liked")
	}

	// Like again (idempotent)
	if err := db.LikeArticle(ctx, 12345); err != nil {
		t.Fatalf("LikeArticle (duplicate) failed: %v", err)
	}

	// Count likes
	count, err := db.GetLikeCount(ctx)
	if err != nil {
		t.Fatalf("GetLikeCount failed: %v", err)
	}
	if count != 1 {
		t.Errorf("like count = %d, want 1", count)
	}
}

func TestTagWeightOperations(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	ctx := context.Background()

	// Get non-existent tag (should return default)
	weight, err := db.GetTagWeight(ctx, "unknown")
	if err != nil {
		t.Fatalf("GetTagWeight failed: %v", err)
	}
	if weight != 1.0 {
		t.Errorf("weight = %f, want 1.0 (default)", weight)
	}

	// Boost a tag
	if err := db.BoostTagWeight(ctx, "go", 0.2); err != nil {
		t.Fatalf("BoostTagWeight failed: %v", err)
	}

	weight, err = db.GetTagWeight(ctx, "go")
	if err != nil {
		t.Fatalf("GetTagWeight failed: %v", err)
	}
	if weight != 1.2 {
		t.Errorf("weight = %f, want 1.2", weight)
	}

	// Boost again
	if err := db.BoostTagWeight(ctx, "go", 0.3); err != nil {
		t.Fatalf("BoostTagWeight failed: %v", err)
	}

	weight, _ = db.GetTagWeight(ctx, "go")
	if weight != 1.5 {
		t.Errorf("weight = %f, want 1.5", weight)
	}

	// Get all weights
	weights, err := db.GetAllTagWeights(ctx)
	if err != nil {
		t.Fatalf("GetAllTagWeights failed: %v", err)
	}
	if w, ok := weights["go"]; !ok || w != 1.5 {
		t.Errorf("weights[go] = %f, want 1.5", w)
	}
}

func TestApplyTagDecay(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	ctx := context.Background()

	// Set up initial weights
	db.BoostTagWeight(ctx, "go", 0.5)    // 1.5
	db.BoostTagWeight(ctx, "rust", 0.8)  // 1.8

	// Apply 10% decay with 0.1 minimum
	if err := db.ApplyTagDecay(ctx, 0.1, 0.1); err != nil {
		t.Fatalf("ApplyTagDecay failed: %v", err)
	}

	weights, _ := db.GetAllTagWeights(ctx)
	// 1.5 * 0.9 = 1.35
	if w := weights["go"]; w < 1.34 || w > 1.36 {
		t.Errorf("go weight = %f, want ~1.35", w)
	}
	// 1.8 * 0.9 = 1.62
	if w := weights["rust"]; w < 1.61 || w > 1.63 {
		t.Errorf("rust weight = %f, want ~1.62", w)
	}
}

func TestApplyTagDecayWithFloor(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	ctx := context.Background()

	// Insert a low weight tag directly
	_, err := db.conn.ExecContext(ctx,
		"INSERT INTO tag_weights (tag, weight, count) VALUES (?, ?, ?)",
		"lowweight", 0.15, 1)
	if err != nil {
		t.Fatal(err)
	}

	// Apply 50% decay with 0.1 floor
	// 0.15 * 0.5 = 0.075, but floor is 0.1
	if err := db.ApplyTagDecay(ctx, 0.5, 0.1); err != nil {
		t.Fatal(err)
	}

	weight, _ := db.GetTagWeight(ctx, "lowweight")
	if weight != 0.1 {
		t.Errorf("weight = %f, want 0.1 (floor)", weight)
	}
}

func TestGetTopTags(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	ctx := context.Background()

	// Create tags with different weights
	tags := []struct {
		name   string
		boost  float64
	}{
		{"first", 2.0},
		{"second", 1.5},
		{"third", 1.0},
		{"fourth", 0.5},
		{"fifth", 0.3},
	}

	for _, tag := range tags {
		db.BoostTagWeight(ctx, tag.name, tag.boost)
	}

	// Get top 3
	top, err := db.GetTopTags(ctx, 3)
	if err != nil {
		t.Fatalf("GetTopTags failed: %v", err)
	}

	if len(top) != 3 {
		t.Fatalf("got %d tags, want 3", len(top))
	}

	// Verify order (highest weight first)
	if top[0].Tag != "first" {
		t.Errorf("first tag = %q, want 'first'", top[0].Tag)
	}
	if top[1].Tag != "second" {
		t.Errorf("second tag = %q, want 'second'", top[1].Tag)
	}
}

func TestSettingsOperations(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	ctx := context.Background()

	// Get non-existent setting
	_, err := db.GetSetting(ctx, "unknown")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound for unknown setting, got: %v", err)
	}

	// Set setting
	if err := db.SetSetting(ctx, "chat_id", "12345"); err != nil {
		t.Fatalf("SetSetting failed: %v", err)
	}

	// Get setting
	val, err := db.GetSetting(ctx, "chat_id")
	if err != nil {
		t.Fatalf("GetSetting failed: %v", err)
	}
	if val != "12345" {
		t.Errorf("value = %q, want '12345'", val)
	}

	// Update setting
	if err := db.SetSetting(ctx, "chat_id", "67890"); err != nil {
		t.Fatalf("SetSetting (update) failed: %v", err)
	}

	val, _ = db.GetSetting(ctx, "chat_id")
	if val != "67890" {
		t.Errorf("value = %q, want '67890'", val)
	}
}

func TestTagJSONSerialization(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	ctx := context.Background()

	// Test with special characters in tags
	article := &Article{
		ID:        1,
		Title:     "Test",
		URL:       "https://example.com",
		Tags:      []string{"c++", "machine-learning", "web/api"},
		FetchedAt: time.Now(),
	}

	if err := db.SaveArticle(ctx, article); err != nil {
		t.Fatalf("SaveArticle failed: %v", err)
	}

	retrieved, _ := db.GetArticle(ctx, 1)
	if len(retrieved.Tags) != 3 {
		t.Errorf("got %d tags, want 3", len(retrieved.Tags))
	}
	expected := []string{"c++", "machine-learning", "web/api"}
	for i, tag := range expected {
		if retrieved.Tags[i] != tag {
			t.Errorf("tag[%d] = %q, want %q", i, retrieved.Tags[i], tag)
		}
	}
}

func TestEmptyTags(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	ctx := context.Background()

	article := &Article{
		ID:        1,
		Title:     "Test",
		URL:       "https://example.com",
		Tags:      []string{},
		FetchedAt: time.Now(),
	}

	if err := db.SaveArticle(ctx, article); err != nil {
		t.Fatalf("SaveArticle failed: %v", err)
	}

	retrieved, _ := db.GetArticle(ctx, 1)
	if retrieved.Tags == nil {
		t.Error("Tags should not be nil")
	}
	if len(retrieved.Tags) != 0 {
		t.Errorf("got %d tags, want 0", len(retrieved.Tags))
	}
}

// Helper functions

func newTestDB(t *testing.T) *DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	return db
}

func ptrTime(t time.Time) *time.Time {
	return &t
}

func ptrInt64(i int64) *int64 {
	return &i
}

// Ensure tags serialize correctly
func TestTagsJSONRoundTrip(t *testing.T) {
	original := []string{"go", "testing", "web-dev"}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}

	var restored []string
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}

	if len(restored) != len(original) {
		t.Fatalf("length mismatch: %d vs %d", len(restored), len(original))
	}
}
