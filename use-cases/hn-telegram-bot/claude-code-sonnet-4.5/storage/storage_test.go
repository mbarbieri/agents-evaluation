package storage

import (
	"os"
	"testing"
	"time"
)

func setupTestDB(t *testing.T) *SQLiteStorage {
	tmpfile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	storage, err := New(tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		storage.Close()
		os.Remove(tmpfile.Name())
	})

	return storage
}

func TestNew_CreatesSchema(t *testing.T) {
	storage := setupTestDB(t)

	// Verify tables exist by attempting to query them
	rows, err := storage.db.Query("SELECT name FROM sqlite_master WHERE type='table'")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	tables := make(map[string]bool)
	for rows.Next() {
		var name string
		rows.Scan(&name)
		tables[name] = true
	}

	required := []string{"articles", "likes", "tag_weights", "settings"}
	for _, table := range required {
		if !tables[table] {
			t.Errorf("Table %s not created", table)
		}
	}
}

func TestSaveArticle(t *testing.T) {
	storage := setupTestDB(t)

	article := &Article{
		ID:          12345,
		Title:       "Test Article",
		URL:         "https://example.com",
		Summary:     "Test summary",
		Tags:        []string{"golang", "testing"},
		HNScore:     100,
		FetchedAt:   time.Now(),
		TelegramMsgID: 0,
	}

	err := storage.SaveArticle(article)
	if err != nil {
		t.Fatalf("SaveArticle() error = %v", err)
	}

	// Retrieve and verify
	retrieved, err := storage.GetArticleByID(12345)
	if err != nil {
		t.Fatalf("GetArticleByID() error = %v", err)
	}

	if retrieved.Title != article.Title {
		t.Errorf("Title = %v, want %v", retrieved.Title, article.Title)
	}
	if retrieved.URL != article.URL {
		t.Errorf("URL = %v, want %v", retrieved.URL, article.URL)
	}
	if len(retrieved.Tags) != 2 {
		t.Errorf("Tags length = %v, want 2", len(retrieved.Tags))
	}
}

func TestSaveArticle_Upsert(t *testing.T) {
	storage := setupTestDB(t)

	article := &Article{
		ID:      12345,
		Title:   "Original Title",
		URL:     "https://example.com",
		Summary: "Original summary",
		Tags:    []string{"golang"},
		HNScore: 100,
		FetchedAt: time.Now(),
	}

	storage.SaveArticle(article)

	// Update with same ID
	article.Title = "Updated Title"
	article.TelegramMsgID = 999

	err := storage.SaveArticle(article)
	if err != nil {
		t.Fatalf("SaveArticle() upsert error = %v", err)
	}

	retrieved, _ := storage.GetArticleByID(12345)
	if retrieved.Title != "Updated Title" {
		t.Errorf("Title = %v, want Updated Title", retrieved.Title)
	}
	if retrieved.TelegramMsgID != 999 {
		t.Errorf("TelegramMsgID = %v, want 999", retrieved.TelegramMsgID)
	}
}

func TestGetArticleByTelegramMsgID(t *testing.T) {
	storage := setupTestDB(t)

	article := &Article{
		ID:            12345,
		Title:         "Test",
		URL:           "https://example.com",
		Summary:       "Summary",
		Tags:          []string{"test"},
		HNScore:       50,
		FetchedAt:     time.Now(),
		TelegramMsgID: 888,
	}

	storage.SaveArticle(article)

	retrieved, err := storage.GetArticleByTelegramMsgID(888)
	if err != nil {
		t.Fatalf("GetArticleByTelegramMsgID() error = %v", err)
	}

	if retrieved.ID != 12345 {
		t.Errorf("ID = %v, want 12345", retrieved.ID)
	}
}

func TestGetArticleByTelegramMsgID_NotFound(t *testing.T) {
	storage := setupTestDB(t)

	_, err := storage.GetArticleByTelegramMsgID(999)
	if err == nil {
		t.Error("GetArticleByTelegramMsgID() expected error for nonexistent msg ID, got nil")
	}
}

func TestGetRecentlySentArticles(t *testing.T) {
	storage := setupTestDB(t)

	now := time.Now()

	// Article sent 3 days ago
	recent := &Article{
		ID:       1,
		Title:    "Recent",
		URL:      "https://example.com/1",
		Summary:  "Recent article",
		Tags:     []string{"test"},
		HNScore:  50,
		FetchedAt: now.Add(-3 * 24 * time.Hour),
		SentAt:   &now,
	}
	storage.SaveArticle(recent)

	// Article sent 10 days ago
	old := &Article{
		ID:       2,
		Title:    "Old",
		URL:      "https://example.com/2",
		Summary:  "Old article",
		Tags:     []string{"test"},
		HNScore:  50,
		FetchedAt: now.Add(-10 * 24 * time.Hour),
	}
	oldSentAt := now.Add(-10 * 24 * time.Hour)
	old.SentAt = &oldSentAt
	storage.SaveArticle(old)

	ids, err := storage.GetRecentlySentArticles(7)
	if err != nil {
		t.Fatalf("GetRecentlySentArticles() error = %v", err)
	}

	if len(ids) != 1 {
		t.Fatalf("GetRecentlySentArticles() returned %d articles, want 1", len(ids))
	}

	if ids[0] != 1 {
		t.Errorf("Article ID = %v, want 1", ids[0])
	}
}

func TestIsLiked(t *testing.T) {
	storage := setupTestDB(t)

	// Not liked initially
	liked, err := storage.IsLiked(12345)
	if err != nil {
		t.Fatalf("IsLiked() error = %v", err)
	}
	if liked {
		t.Error("IsLiked() = true, want false")
	}

	// Record like
	err = storage.RecordLike(12345)
	if err != nil {
		t.Fatalf("RecordLike() error = %v", err)
	}

	// Now should be liked
	liked, err = storage.IsLiked(12345)
	if err != nil {
		t.Fatalf("IsLiked() error = %v", err)
	}
	if !liked {
		t.Error("IsLiked() = false, want true")
	}
}

func TestRecordLike_Idempotent(t *testing.T) {
	storage := setupTestDB(t)

	// Record like twice
	storage.RecordLike(12345)
	err := storage.RecordLike(12345)
	if err != nil {
		t.Fatalf("RecordLike() second call error = %v", err)
	}

	// Should only have one record
	var count int
	storage.db.QueryRow("SELECT COUNT(*) FROM likes WHERE article_id = ?", 12345).Scan(&count)
	if count != 1 {
		t.Errorf("Like count = %v, want 1", count)
	}
}

func TestGetLikeCount(t *testing.T) {
	storage := setupTestDB(t)

	count, err := storage.GetLikeCount()
	if err != nil {
		t.Fatalf("GetLikeCount() error = %v", err)
	}
	if count != 0 {
		t.Errorf("Initial count = %v, want 0", count)
	}

	storage.RecordLike(1)
	storage.RecordLike(2)
	storage.RecordLike(3)

	count, err = storage.GetLikeCount()
	if err != nil {
		t.Fatalf("GetLikeCount() error = %v", err)
	}
	if count != 3 {
		t.Errorf("Count after 3 likes = %v, want 3", count)
	}
}

func TestGetTagWeight(t *testing.T) {
	storage := setupTestDB(t)

	// Non-existent tag should return 1.0
	weight, err := storage.GetTagWeight("golang")
	if err != nil {
		t.Fatalf("GetTagWeight() error = %v", err)
	}
	if weight != 1.0 {
		t.Errorf("Weight = %v, want 1.0", weight)
	}
}

func TestSetTagWeight(t *testing.T) {
	storage := setupTestDB(t)

	err := storage.SetTagWeight("golang", 1.5)
	if err != nil {
		t.Fatalf("SetTagWeight() error = %v", err)
	}

	weight, _ := storage.GetTagWeight("golang")
	if weight != 1.5 {
		t.Errorf("Weight = %v, want 1.5", weight)
	}

	// Update existing
	storage.SetTagWeight("golang", 2.0)
	weight, _ = storage.GetTagWeight("golang")
	if weight != 2.0 {
		t.Errorf("Weight = %v, want 2.0", weight)
	}
}

func TestIncrementTagOccurrence(t *testing.T) {
	storage := setupTestDB(t)

	// First increment creates the tag
	err := storage.IncrementTagOccurrence("golang")
	if err != nil {
		t.Fatalf("IncrementTagOccurrence() error = %v", err)
	}

	var count int
	storage.db.QueryRow("SELECT occurrence_count FROM tag_weights WHERE tag = ?", "golang").Scan(&count)
	if count != 1 {
		t.Errorf("Occurrence count = %v, want 1", count)
	}

	// Second increment
	storage.IncrementTagOccurrence("golang")
	storage.db.QueryRow("SELECT occurrence_count FROM tag_weights WHERE tag = ?", "golang").Scan(&count)
	if count != 2 {
		t.Errorf("Occurrence count = %v, want 2", count)
	}
}

func TestGetTopTags(t *testing.T) {
	storage := setupTestDB(t)

	storage.SetTagWeight("golang", 2.0)
	storage.SetTagWeight("rust", 1.8)
	storage.SetTagWeight("python", 1.5)

	tags, err := storage.GetTopTags(2)
	if err != nil {
		t.Fatalf("GetTopTags() error = %v", err)
	}

	if len(tags) != 2 {
		t.Fatalf("GetTopTags() returned %d tags, want 2", len(tags))
	}

	if tags[0].Tag != "golang" || tags[0].Weight != 2.0 {
		t.Errorf("First tag = %v with weight %v, want golang with 2.0", tags[0].Tag, tags[0].Weight)
	}

	if tags[1].Tag != "rust" || tags[1].Weight != 1.8 {
		t.Errorf("Second tag = %v with weight %v, want rust with 1.8", tags[1].Tag, tags[1].Weight)
	}
}

func TestGetAllTagWeights(t *testing.T) {
	storage := setupTestDB(t)

	storage.SetTagWeight("golang", 2.0)
	storage.SetTagWeight("rust", 1.5)

	tags, err := storage.GetAllTagWeights()
	if err != nil {
		t.Fatalf("GetAllTagWeights() error = %v", err)
	}

	if len(tags) != 2 {
		t.Fatalf("GetAllTagWeights() returned %d tags, want 2", len(tags))
	}
}

func TestApplyDecay(t *testing.T) {
	storage := setupTestDB(t)

	storage.SetTagWeight("golang", 2.0)
	storage.SetTagWeight("rust", 0.16)

	err := storage.ApplyDecay(0.1, 0.15) // 10% decay, 0.15 minimum
	if err != nil {
		t.Fatalf("ApplyDecay() error = %v", err)
	}

	golangWeight, _ := storage.GetTagWeight("golang")
	rustWeight, _ := storage.GetTagWeight("rust")

	expectedGolang := 2.0 * 0.9 // 1.8
	if golangWeight != expectedGolang {
		t.Errorf("Golang weight = %v, want %v", golangWeight, expectedGolang)
	}

	// Rust should be at minimum (0.16 * 0.9 = 0.144, clamped to 0.15)
	if rustWeight != 0.15 {
		t.Errorf("Rust weight = %v, want 0.15 (minimum)", rustWeight)
	}
}

func TestGetSetting(t *testing.T) {
	storage := setupTestDB(t)

	// Non-existent setting
	val, err := storage.GetSetting("test_key")
	if err != nil {
		t.Fatalf("GetSetting() error = %v", err)
	}
	if val != "" {
		t.Errorf("GetSetting() = %v, want empty string", val)
	}
}

func TestSetSetting(t *testing.T) {
	storage := setupTestDB(t)

	err := storage.SetSetting("digest_time", "10:30")
	if err != nil {
		t.Fatalf("SetSetting() error = %v", err)
	}

	val, _ := storage.GetSetting("digest_time")
	if val != "10:30" {
		t.Errorf("GetSetting() = %v, want 10:30", val)
	}

	// Update
	storage.SetSetting("digest_time", "11:00")
	val, _ = storage.GetSetting("digest_time")
	if val != "11:00" {
		t.Errorf("GetSetting() = %v, want 11:00", val)
	}
}

func TestClose(t *testing.T) {
	tmpfile, _ := os.CreateTemp("", "test-*.db")
	tmpfile.Close()
	defer os.Remove(tmpfile.Name())

	storage, _ := New(tmpfile.Name())
	err := storage.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}
