package storage

import (
	"database/sql"
	"os"
	"testing"
	"time"
)

func TestNewStorage_FileCreation(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	_, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file was not created")
	}
}

func TestStorage_InitializeSchema(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	s, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer s.Close()

	articles, err := s.GetRecentArticles(7)
	if err != nil {
		t.Errorf("Unexpected error querying articles: %v", err)
	}
	if len(articles) != 0 {
		t.Errorf("Expected empty database, got %d articles", len(articles))
	}
}

func TestStorage_SaveAndGetArticle(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	s, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer s.Close()

	now := time.Now()
	article := Article{
		ID:        12345,
		Title:     "Test Article",
		URL:       "https://example.com/test",
		Summary:   "Test summary",
		Tags:      `["go", "programming"]`,
		Score:     100,
		FetchedAt: now,
		SentAt:    sql.NullTime{Time: now, Valid: true},
		MessageID: sql.NullInt64{Int64: 54321, Valid: true},
	}

	err = s.SaveArticle(&article)
	if err != nil {
		t.Fatalf("Failed to save article: %v", err)
	}

	retrieved, err := s.GetArticle(12345)
	if err != nil {
		t.Fatalf("Failed to get article: %v", err)
	}

	if retrieved.ID != article.ID {
		t.Errorf("Expected article ID %d, got %d", article.ID, retrieved.ID)
	}
	if retrieved.Title != article.Title {
		t.Errorf("Expected title '%s', got '%s'", article.Title, retrieved.Title)
	}
	if retrieved.Summary != article.Summary {
		t.Errorf("Expected summary '%s', got '%s'", article.Summary, retrieved.Summary)
	}
	if retrieved.Tags != article.Tags {
		t.Errorf("Expected tags '%s', got '%s'", article.Tags, retrieved.Tags)
	}
	if retrieved.MessageID.Int64 != 54321 {
		t.Errorf("Expected message ID 54321, got %d", retrieved.MessageID.Int64)
	}
}

func TestStorage_SaveArticle_DuplicateUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	s, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer s.Close()

	now := time.Now()
	article := Article{
		ID:        12345,
		Title:     "Original Title",
		URL:       "https://example.com/test",
		Summary:   "Original summary",
		Tags:      `["go"]`,
		Score:     100,
		FetchedAt: now,
		SentAt:    sql.NullTime{Time: now, Valid: true},
		MessageID: sql.NullInt64{Int64: 100, Valid: true},
	}

	err = s.SaveArticle(&article)
	if err != nil {
		t.Fatalf("Failed to save article: %v", err)
	}

	article.Title = "Updated Title"
	article.MessageID = sql.NullInt64{Int64: 200, Valid: true}

	err = s.SaveArticle(&article)
	if err != nil {
		t.Fatalf("Failed to update article: %v", err)
	}

	retrieved, _ := s.GetArticle(12345)
	if retrieved.Title != "Updated Title" {
		t.Errorf("Expected updated title, got '%s'", retrieved.Title)
	}
	if retrieved.MessageID.Int64 != 200 {
		t.Errorf("Expected updated message ID, got %d", retrieved.MessageID.Int64)
	}
}

func TestStorage_GetRecentArticles(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	s, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer s.Close()

	now := time.Now()

	for i := 0; i < 5; i++ {
		article := Article{
			ID:        int64(1000 + i),
			Title:     "Test Article",
			URL:       "https://example.com/test",
			Summary:   "Test summary",
			Tags:      `["test"]`,
			Score:     50 + i*10,
			FetchedAt: now,
			SentAt:    sql.NullTime{Time: now.Add(-time.Duration(i) * time.Hour), Valid: true},
			MessageID: sql.NullInt64{Int64: int64(100 + i), Valid: true},
		}
		s.SaveArticle(&article)
	}

	recent, err := s.GetRecentArticles(7)
	if err != nil {
		t.Fatalf("Failed to get recent articles: %v", err)
	}

	if len(recent) != 5 {
		t.Errorf("Expected 5 recent articles, got %d", len(recent))
	}
}

func TestStorage_GetRecentArticles_ExcludesOld(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	s, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer s.Close()

	now := time.Now()

	oldArticle := Article{
		ID:        9999,
		Title:     "Old Article",
		URL:       "https://example.com/old",
		Summary:   "Old summary",
		Tags:      `["old"]`,
		Score:     100,
		FetchedAt: now.Add(-8 * 24 * time.Hour),
		SentAt:    sql.NullTime{Time: now.Add(-8 * 24 * time.Hour), Valid: true},
		MessageID: sql.NullInt64{Int64: 9999, Valid: true},
	}
	s.SaveArticle(&oldArticle)

	recent, err := s.GetRecentArticles(7)
	if err != nil {
		t.Fatalf("Failed to get recent articles: %v", err)
	}

	for _, article := range recent {
		if article.ID == 9999 {
			t.Error("Old article should be excluded from recent articles")
		}
	}
}

func TestStorage_GetArticle_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	s, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer s.Close()

	_, err = s.GetArticle(99999)
	if err == nil {
		t.Error("Expected error for non-existent article")
	}
}

func TestStorage_LikeArticle(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	s, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer s.Close()

	now := time.Now()
	article := Article{
		ID:        12345,
		Title:     "Test Article",
		URL:       "https://example.com/test",
		Summary:   "Test summary",
		Tags:      `["go", "programming"]`,
		Score:     100,
		FetchedAt: now,
		SentAt:    sql.NullTime{Time: now, Valid: true},
		MessageID: sql.NullInt64{Int64: 54321, Valid: true},
	}
	s.SaveArticle(&article)

	err = s.LikeArticle(12345)
	if err != nil {
		t.Fatalf("Failed to like article: %v", err)
	}

	isLiked, err := s.IsArticleLiked(12345)
	if err != nil {
		t.Fatalf("Failed to check if liked: %v", err)
	}
	if !isLiked {
		t.Error("Article should be marked as liked")
	}

	like, _ := s.GetLike(12345)
	if like.ArticleID != 12345 {
		t.Errorf("Expected like for article 12345, got %d", like.ArticleID)
	}
}

func TestStorage_LikeArticle_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	s, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer s.Close()

	now := time.Now()
	article := Article{
		ID:        12345,
		Title:     "Test Article",
		URL:       "https://example.com/test",
		Summary:   "Test summary",
		Tags:      `["go"]`,
		Score:     100,
		FetchedAt: now,
		SentAt:    sql.NullTime{Time: now, Valid: true},
		MessageID: sql.NullInt64{Int64: 54321, Valid: true},
	}
	s.SaveArticle(&article)

	err = s.LikeArticle(12345)
	if err != nil {
		t.Fatalf("Failed to like article: %v", err)
	}

	err = s.LikeArticle(12345)
	if err != nil {
		t.Fatalf("Second like should not fail (idempotent): %v", err)
	}
}

func TestStorage_IsArticleLiked_NotLiked(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	s, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer s.Close()

	isLiked, err := s.IsArticleLiked(99999)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if isLiked {
		t.Error("Non-existent article should not be liked")
	}
}

func TestStorage_GetLikeCount(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	s, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer s.Close()

	count, err := s.GetLikeCount()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 likes, got %d", count)
	}

	now := time.Now()
	article := Article{
		ID:        12345,
		Title:     "Test",
		URL:       "https://example.com",
		Summary:   "Summary",
		Tags:      `["test"]`,
		Score:     100,
		FetchedAt: now,
		SentAt:    sql.NullTime{Time: now, Valid: true},
		MessageID: sql.NullInt64{Int64: 100, Valid: true},
	}
	s.SaveArticle(&article)
	s.LikeArticle(12345)

	count, err = s.GetLikeCount()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 like, got %d", count)
	}
}

func TestStorage_GetTagsByWeight(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	s, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer s.Close()

	tags, err := s.GetTagsByWeight()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("Expected empty tags, got %d", len(tags))
	}

	s.UpsertTagWeight("go", 2.5, 5)
	s.UpsertTagWeight("rust", 1.8, 3)
	s.UpsertTagWeight("python", 1.2, 2)

	tags, err = s.GetTagsByWeight()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(tags) != 3 {
		t.Errorf("Expected 3 tags, got %d", len(tags))
	}

	if tags[0].Name != "go" || tags[0].Weight != 2.5 {
		t.Errorf("Expected first tag to be 'go' with weight 2.5, got '%s' with weight %f", tags[0].Name, tags[0].Weight)
	}
	if tags[1].Name != "rust" || tags[1].Weight != 1.8 {
		t.Errorf("Expected second tag to be 'rust' with weight 1.8, got '%s' with weight %f", tags[1].Name, tags[1].Weight)
	}
}

func TestStorage_UpsertTagWeight(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	s, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer s.Close()

	s.UpsertTagWeight("go", 1.5, 3)

	tag, _ := s.GetTagWeight("go")
	if tag.Weight != 1.5 {
		t.Errorf("Expected weight 1.5, got %f", tag.Weight)
	}
	if tag.Count != 3 {
		t.Errorf("Expected count 3, got %d", tag.Count)
	}

	s.UpsertTagWeight("go", 2.0, 5)

	tag, _ = s.GetTagWeight("go")
	if tag.Weight != 2.0 {
		t.Errorf("Expected weight 2.0, got %f", tag.Weight)
	}
	if tag.Count != 5 {
		t.Errorf("Expected count 5, got %d", tag.Count)
	}
}

func TestStorage_DecayAllTags(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	s, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer s.Close()

	s.UpsertTagWeight("go", 1.0, 5)
	s.UpsertTagWeight("rust", 1.0, 3)
	s.UpsertTagWeight("python", 0.2, 2)

	s.DecayAllTags(0.1, 0.1)

	goTag, _ := s.GetTagWeight("go")
	if goTag.Weight != 0.9 {
		t.Errorf("Expected weight 0.9 after 10%% decay, got %f", goTag.Weight)
	}

	pythonTag, _ := s.GetTagWeight("python")
	expectedPython := 0.2 * 0.9
	if pythonTag.Weight < expectedPython-0.001 || pythonTag.Weight > expectedPython+0.001 {
		t.Errorf("Expected weight %f after decay, got %f", expectedPython, pythonTag.Weight)
	}
}

func TestStorage_GetTagWeight_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	s, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer s.Close()

	_, err = s.GetTagWeight("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent tag")
	}
}

func TestStorage_GetSetting(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	s, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer s.Close()

	value, err := s.GetSetting("chat_id")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if value != "" {
		t.Errorf("Expected empty setting, got '%s'", value)
	}

	s.SetSetting("chat_id", "12345")

	value, err = s.GetSetting("chat_id")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if value != "12345" {
		t.Errorf("Expected setting '12345', got '%s'", value)
	}
}

func TestStorage_SetSetting(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	s, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer s.Close()

	err = s.SetSetting("test_key", "test_value")
	if err != nil {
		t.Fatalf("Failed to set setting: %v", err)
	}

	value, _ := s.GetSetting("test_key")
	if value != "test_value" {
		t.Errorf("Expected 'test_value', got '%s'", value)
	}

	s.SetSetting("test_key", "updated_value")

	value, _ = s.GetSetting("test_key")
	if value != "updated_value" {
		t.Errorf("Expected 'updated_value', got '%s'", value)
	}
}

func TestStorage_GetArticleByMessageID(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	s, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer s.Close()

	now := time.Now()
	article := Article{
		ID:        12345,
		Title:     "Test Article",
		URL:       "https://example.com/test",
		Summary:   "Test summary",
		Tags:      `["go"]`,
		Score:     100,
		FetchedAt: now,
		SentAt:    sql.NullTime{Time: now, Valid: true},
		MessageID: sql.NullInt64{Int64: 54321, Valid: true},
	}
	s.SaveArticle(&article)

	retrieved, err := s.GetArticleByMessageID(54321)
	if err != nil {
		t.Fatalf("Failed to get article by message ID: %v", err)
	}

	if retrieved.ID != 12345 {
		t.Errorf("Expected article ID 12345, got %d", retrieved.ID)
	}
}

func TestStorage_GetArticleByMessageID_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	s, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer s.Close()

	_, err = s.GetArticleByMessageID(99999)
	if err == nil {
		t.Error("Expected error for non-existent message ID")
	}
}
