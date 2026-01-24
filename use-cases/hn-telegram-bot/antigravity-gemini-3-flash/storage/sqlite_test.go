package storage

import (
	"os"
	"testing"
	"time"
)

func TestStorage(t *testing.T) {
	dbPath := "test.db"
	defer os.Remove(dbPath)

	s, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer s.Close()

	t.Run("Articles", func(t *testing.T) {
		art := &Article{
			HNID:      12345,
			Title:     "Test Article",
			URL:       "https://example.com/test",
			Summary:   "Summary of the test article",
			Tags:      []string{"go", "sqlite"},
			HNScore:   100,
			FetchedAt: time.Now(),
		}

		if err := s.SaveArticle(art); err != nil {
			t.Fatalf("failed to save article: %v", err)
		}

		// Save again to test upsert
		art.Title = "Updated Title"
		if err := s.SaveArticle(art); err != nil {
			t.Fatalf("failed to update article: %v", err)
		}

		got, err := s.GetArticle(12345)
		if err != nil {
			t.Fatalf("failed to get article: %v", err)
		}
		if got.Title != "Updated Title" {
			t.Errorf("expected Title Updated Title, got %s", got.Title)
		}
		if len(got.Tags) != 2 || got.Tags[0] != "go" {
			t.Errorf("tags mismatch: %v", got.Tags)
		}
	})

	t.Run("Likes", func(t *testing.T) {
		if err := s.MarkLiked(12345); err != nil {
			t.Fatalf("failed to mark liked: %v", err)
		}

		liked, err := s.IsLiked(12345)
		if err != nil {
			t.Fatalf("failed to check like status: %v", err)
		}
		if !liked {
			t.Error("expected article to be liked")
		}

		liked, _ = s.IsLiked(67890)
		if liked {
			t.Error("expected article to NOT be liked")
		}
	})

	t.Run("TagWeights", func(t *testing.T) {
		if err := s.UpdateTagWeight("rust", 1.5, 5); err != nil {
			t.Fatalf("failed to update tag weight: %v", err)
		}

		weights, err := s.GetTagWeights()
		if err != nil {
			t.Fatalf("failed to get tag weights: %v", err)
		}
		if w, ok := weights["rust"]; !ok || w.Weight != 1.5 {
			t.Errorf("expected weight 1.5 for rust, got %v", w)
		}
	})

	t.Run("Settings", func(t *testing.T) {
		if err := s.SetSetting("chat_id", "987654321"); err != nil {
			t.Fatalf("failed to set setting: %v", err)
		}

		val, err := s.GetSetting("chat_id")
		if err != nil {
			t.Fatalf("failed to get setting: %v", err)
		}
		if val != "987654321" {
			t.Errorf("expected setting 987654321, got %s", val)
		}
	})

	t.Run("GetArticleByMessageID", func(t *testing.T) {
		art := &Article{
			HNID:              999,
			TelegramMessageID: 888,
			Title:             "ByMsgID",
			FetchedAt:         time.Now(),
		}
		s.SaveArticle(art)

		got, err := s.GetArticleByMessageID(888)
		if err != nil {
			t.Fatalf("failed to get article: %v", err)
		}
		if got.HNID != 999 {
			t.Errorf("expected HNID 999, got %d", got.HNID)
		}
	})

	t.Run("GetRecentHNIDs", func(t *testing.T) {
		art := &Article{
			HNID:   111,
			SentAt: time.Now(),
		}
		s.SaveArticle(art)

		ids, err := s.GetRecentHNIDs(7)
		if err != nil {
			t.Fatalf("failed to get recent IDs: %v", err)
		}
		found := false
		for _, id := range ids {
			if id == 111 {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected ID 111 to be found in recent")
		}
	})
}
