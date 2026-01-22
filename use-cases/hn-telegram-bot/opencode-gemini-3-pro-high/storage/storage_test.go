package storage

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func newTestDB(t *testing.T) (*DB, func()) {
	tmpDir, err := os.MkdirTemp("", "storage_test")
	if err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to init db: %v", err)
	}

	return db, func() {
		db.Close()
		os.RemoveAll(tmpDir)
	}
}

func TestSettings(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	// Test default/missing
	val, err := db.GetSetting("foo")
	if err != nil {
		t.Fatalf("GetSetting failed: %v", err)
	}
	if val != "" {
		t.Errorf("Expected empty string for missing setting, got %s", val)
	}

	// Test Set/Get
	if err := db.SetSetting("foo", "bar"); err != nil {
		t.Fatalf("SetSetting failed: %v", err)
	}

	val, err = db.GetSetting("foo")
	if err != nil {
		t.Fatalf("GetSetting failed: %v", err)
	}
	if val != "bar" {
		t.Errorf("Expected 'bar', got '%s'", val)
	}

	// Test Update
	if err := db.SetSetting("foo", "baz"); err != nil {
		t.Fatalf("SetSetting update failed: %v", err)
	}
	val, err = db.GetSetting("foo")
	if val != "baz" {
		t.Errorf("Expected 'baz', got '%s'", val)
	}
}

func TestArticles(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	article := Article{
		ID:        12345,
		Title:     "Test Article",
		URL:       "http://example.com",
		Summary:   "Summary here",
		Tags:      []string{"tech", "go"},
		Score:     100,
		FetchedAt: time.Now(),
	}

	// Test Save
	if err := db.SaveArticle(article); err != nil {
		t.Fatalf("SaveArticle failed: %v", err)
	}

	// Test Save duplicate (should act as upsert or ignore, preferably upsert for updates)
	article.Score = 150
	if err := db.SaveArticle(article); err != nil {
		t.Fatalf("SaveArticle update failed: %v", err)
	}

	// Verify data
	// (We haven't implemented GetArticleByID but strictly speaking we might not need it for the bot flow,
	// checking via generic SQL to confirm storage)
	var score int
	var tagsJSON string
	row := db.sqlDB.QueryRow("SELECT score, tags FROM articles WHERE id = ?", 12345)
	if err := row.Scan(&score, &tagsJSON); err != nil {
		t.Fatalf("Failed to query article: %v", err)
	}
	if score != 150 {
		t.Errorf("Expected score 150, got %d", score)
	}
	var tags []string
	json.Unmarshal([]byte(tagsJSON), &tags)
	if len(tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(tags))
	}

	// Test MarkSent
	msgID := 999
	if err := db.MarkArticleSent(12345, msgID); err != nil {
		t.Fatalf("MarkArticleSent failed: %v", err)
	}

	// Verify Sent
	var sentAt sql.NullTime
	var mID int
	row = db.sqlDB.QueryRow("SELECT sent_at, telegram_msg_id FROM articles WHERE id = ?", 12345)
	if err := row.Scan(&sentAt, &mID); err != nil {
		t.Fatalf("Failed to verify sent status: %v", err)
	}
	if !sentAt.Valid {
		t.Error("sent_at should be valid")
	}
	if mID != msgID {
		t.Errorf("Expected msgID %d, got %d", msgID, mID)
	}

	// Test Recency Filter (GetRecentSentArticleIDs)
	// We just marked 12345 as sent. It should appear in the list.
	ids, err := db.GetRecentSentArticleIDs(7 * 24 * time.Hour)
	if err != nil {
		t.Fatalf("GetRecentSentArticleIDs failed: %v", err)
	}
	if len(ids) != 1 || ids[0] != 12345 {
		t.Errorf("Expected [12345], got %v", ids)
	}
}

func TestLikesAndTags(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	articleID := 111

	// Check IsLiked (false)
	liked, err := db.IsArticleLiked(articleID)
	if err != nil {
		t.Fatalf("IsArticleLiked failed: %v", err)
	}
	if liked {
		t.Error("Expected not liked")
	}

	// Add Like
	if err := db.AddLike(articleID); err != nil {
		t.Fatalf("AddLike failed: %v", err)
	}

	// Check IsLiked (true)
	liked, err = db.IsArticleLiked(articleID)
	if err != nil {
		t.Fatalf("IsArticleLiked failed: %v", err)
	}
	if !liked {
		t.Error("Expected liked")
	}

	// Test Tag Weights
	// "rust" doesn't exist, should default to 0 if we fetch? No, we fetch only top.

	// Boost Tag "rust"
	if err := db.BoostTag("rust", 1.0, 0.2); err != nil { // init 1.0 + 0.2
		t.Fatalf("BoostTag failed: %v", err)
	}

	weights, err := db.GetTagWeights()
	if err != nil {
		t.Fatalf("GetTagWeights failed: %v", err)
	}
	if w, ok := weights["rust"]; !ok || w != 1.2 { // 1.0 (base) + 0.2 (boost)
		t.Errorf("Expected rust weight 1.2, got %v", w)
	}

	// Boost again
	if err := db.BoostTag("rust", 1.0, 0.2); err != nil {
		t.Fatalf("BoostTag 2 failed: %v", err)
	}
	weights, _ = db.GetTagWeights()
	if w := weights["rust"]; w != 1.4 {
		t.Errorf("Expected rust weight 1.4, got %f", w)
	}

	// Decay
	// Decay by 50%, min 0.1
	if err := db.ApplyTagDecay(0.5, 0.1); err != nil {
		t.Fatalf("ApplyTagDecay failed: %v", err)
	}
	weights, _ = db.GetTagWeights()
	if w := weights["rust"]; w != 0.7 { // 1.4 * 0.5 = 0.7
		t.Errorf("Expected rust weight 0.7, got %f", w)
	}
}

func TestGetArticleByMsgID(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	article := Article{
		ID:      100,
		Title:   "Msg Article",
		URL:     "url",
		Summary: "sum",
		Tags:    []string{"a"},
	}
	db.SaveArticle(article)
	db.MarkArticleSent(100, 555)

	art, err := db.GetArticleByMsgID(555)
	if err != nil {
		t.Fatalf("GetArticleByMsgID failed: %v", err)
	}
	if art.ID != 100 {
		t.Errorf("Expected article ID 100, got %d", art.ID)
	}
}
