package storage

import (
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	t.Run("creates database with schema", func(t *testing.T) {
		dbPath := ":memory:"
		store, err := New(dbPath)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer store.Close()

		// Verify tables exist by querying them
		tables := []string{"articles", "likes", "tag_weights", "settings"}
		for _, table := range tables {
			var count int
			err := store.db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count)
			if err != nil {
				t.Errorf("Failed to check table %s: %v", table, err)
				continue
			}
			if count != 1 {
				t.Errorf("Table %s does not exist", table)
			}
		}
	})

	t.Run("fails on invalid path", func(t *testing.T) {
		_, err := New("/invalid/path/that/does/not/exist/db.sqlite")
		if err == nil {
			t.Error("New() should error on invalid path")
		}
	})
}

func TestSaveArticle(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	article := &Article{
		ID:        12345,
		Title:     "Test Article",
		URL:       "https://example.com/article",
		Summary:   "A test summary",
		Tags:      []string{"go", "testing"},
		HNScore:   100,
		FetchedAt: time.Now(),
	}

	t.Run("saves new article", func(t *testing.T) {
		err := store.SaveArticle(article)
		if err != nil {
			t.Errorf("SaveArticle() error = %v", err)
		}

		// Verify article was saved
		saved, err := store.GetArticle(article.ID)
		if err != nil {
			t.Errorf("GetArticle() error = %v", err)
		}
		if saved.Title != article.Title {
			t.Errorf("Saved title = %v, want %v", saved.Title, article.Title)
		}
	})

	t.Run("updates existing article", func(t *testing.T) {
		article.Summary = "Updated summary"
		err := store.SaveArticle(article)
		if err != nil {
			t.Errorf("SaveArticle() update error = %v", err)
		}

		saved, err := store.GetArticle(article.ID)
		if err != nil {
			t.Errorf("GetArticle() error = %v", err)
		}
		if saved.Summary != "Updated summary" {
			t.Errorf("Updated summary = %v, want 'Updated summary'", saved.Summary)
		}
	})
}

func TestMarkArticleSent(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	article := &Article{
		ID:        12345,
		Title:     "Test Article",
		URL:       "https://example.com/article",
		Summary:   "A test summary",
		Tags:      []string{"go"},
		HNScore:   100,
		FetchedAt: time.Now(),
	}

	if err := store.SaveArticle(article); err != nil {
		t.Fatalf("SaveArticle() error = %v", err)
	}

	sentAt := time.Now()
	msgID := int64(98765)

	t.Run("marks article as sent", func(t *testing.T) {
		err := store.MarkArticleSent(article.ID, msgID, sentAt)
		if err != nil {
			t.Errorf("MarkArticleSent() error = %v", err)
		}

		saved, err := store.GetArticle(article.ID)
		if err != nil {
			t.Errorf("GetArticle() error = %v", err)
		}
		if !saved.SentAt.Equal(sentAt) {
			t.Errorf("SentAt = %v, want %v", saved.SentAt, sentAt)
		}
		if saved.TelegramMsgID != msgID {
			t.Errorf("TelegramMsgID = %v, want %v", saved.TelegramMsgID, msgID)
		}
	})

	t.Run("returns error for non-existent article", func(t *testing.T) {
		err := store.MarkArticleSent(99999, msgID, sentAt)
		if err == nil {
			t.Error("MarkArticleSent() should error for non-existent article")
		}
	})
}

func TestGetRecentArticleIDs(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Create articles with different sent times
	now := time.Now()

	articles := []*Article{
		{ID: 1, Title: "Recent", URL: "https://example.com/1", FetchedAt: now, SentAt: &now},
		{ID: 2, Title: "Old", URL: "https://example.com/2", FetchedAt: now.Add(-8 * 24 * time.Hour), SentAt: func() *time.Time { t := now.Add(-8 * 24 * time.Hour); return &t }()},
		{ID: 3, Title: "Not Sent", URL: "https://example.com/3", FetchedAt: now},
	}

	for _, a := range articles {
		if err := store.SaveArticle(a); err != nil {
			t.Fatalf("SaveArticle() error = %v", err)
		}
		if a.SentAt != nil {
			if err := store.MarkArticleSent(a.ID, int64(a.ID), *a.SentAt); err != nil {
				t.Fatalf("MarkArticleSent() error = %v", err)
			}
		}
	}

	t.Run("returns IDs of articles sent in last 7 days", func(t *testing.T) {
		ids, err := store.GetRecentArticleIDs(7)
		if err != nil {
			t.Errorf("GetRecentArticleIDs() error = %v", err)
		}

		if len(ids) != 1 || ids[0] != 1 {
			t.Errorf("GetRecentArticleIDs() = %v, want [1]", ids)
		}
	})
}

func TestFindArticleByMessageID(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	article := &Article{
		ID:        12345,
		Title:     "Test Article",
		URL:       "https://example.com/article",
		Summary:   "A test summary",
		Tags:      []string{"go"},
		HNScore:   100,
		FetchedAt: time.Now(),
	}

	if err := store.SaveArticle(article); err != nil {
		t.Fatalf("SaveArticle() error = %v", err)
	}

	sentAt := time.Now()
	msgID := int64(98765)
	if err := store.MarkArticleSent(article.ID, msgID, sentAt); err != nil {
		t.Fatalf("MarkArticleSent() error = %v", err)
	}

	t.Run("finds article by message ID", func(t *testing.T) {
		found, err := store.FindArticleByMessageID(msgID)
		if err != nil {
			t.Errorf("FindArticleByMessageID() error = %v", err)
		}
		if found == nil {
			t.Error("FindArticleByMessageID() returned nil, expected article")
			return
		}
		if found.ID != article.ID {
			t.Errorf("Found article ID = %v, want %v", found.ID, article.ID)
		}
	})

	t.Run("returns nil for unknown message ID", func(t *testing.T) {
		found, err := store.FindArticleByMessageID(99999)
		if err != nil {
			t.Errorf("FindArticleByMessageID() error = %v", err)
		}
		if found != nil {
			t.Error("FindArticleByMessageID() should return nil for unknown ID")
		}
	})
}

func TestRecordLike(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	article := &Article{
		ID:        12345,
		Title:     "Test Article",
		URL:       "https://example.com/article",
		Summary:   "A test summary",
		Tags:      []string{"go"},
		HNScore:   100,
		FetchedAt: time.Now(),
	}

	if err := store.SaveArticle(article); err != nil {
		t.Fatalf("SaveArticle() error = %v", err)
	}

	t.Run("records new like", func(t *testing.T) {
		err := store.RecordLike(article.ID)
		if err != nil {
			t.Errorf("RecordLike() error = %v", err)
		}

		// Verify like was recorded
		count, err := store.GetLikeCount()
		if err != nil {
			t.Errorf("GetLikeCount() error = %v", err)
		}
		if count != 1 {
			t.Errorf("Like count = %v, want 1", count)
		}
	})

	t.Run("returns false for already liked article", func(t *testing.T) {
		isNew, err := store.RecordLikeWithCheck(article.ID)
		if err != nil {
			t.Errorf("RecordLikeWithCheck() error = %v", err)
		}
		if isNew {
			t.Error("RecordLikeWithCheck() should return false for already liked article")
		}
	})
}

func TestTagWeights(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	t.Run("upserts tag weight", func(t *testing.T) {
		err := store.UpsertTagWeight("golang", 1.5, 1)
		if err != nil {
			t.Errorf("UpsertTagWeight() error = %v", err)
		}

		weight, count, err := store.GetTagWeight("golang")
		if err != nil {
			t.Errorf("GetTagWeight() error = %v", err)
		}
		if weight != 1.5 {
			t.Errorf("Weight = %v, want 1.5", weight)
		}
		if count != 1 {
			t.Errorf("Count = %v, want 1", count)
		}
	})

	t.Run("updates existing tag weight", func(t *testing.T) {
		err := store.UpsertTagWeight("golang", 2.0, 2)
		if err != nil {
			t.Errorf("UpsertTagWeight() update error = %v", err)
		}

		weight, count, err := store.GetTagWeight("golang")
		if err != nil {
			t.Errorf("GetTagWeight() error = %v", err)
		}
		if weight != 2.0 {
			t.Errorf("Weight = %v, want 2.0", weight)
		}
		if count != 2 {
			t.Errorf("Count = %v, want 2", count)
		}
	})

	t.Run("gets all tag weights", func(t *testing.T) {
		// Add more tags
		store.UpsertTagWeight("rust", 1.2, 1)
		store.UpsertTagWeight("python", 0.8, 3)

		weights, err := store.GetAllTagWeights()
		if err != nil {
			t.Errorf("GetAllTagWeights() error = %v", err)
		}

		if len(weights) != 3 {
			t.Errorf("Got %d weights, want 3", len(weights))
		}
	})

	t.Run("gets top tags", func(t *testing.T) {
		tags, err := store.GetTopTags(2)
		if err != nil {
			t.Errorf("GetTopTags() error = %v", err)
		}

		if len(tags) != 2 {
			t.Errorf("Got %d tags, want 2", len(tags))
		}

		// Should be sorted by weight descending
		if tags[0].Name != "golang" || tags[0].Weight != 2.0 {
			t.Errorf("First tag = %v, want golang with weight 2.0", tags[0])
		}
	})
}

func TestSettings(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	t.Run("sets and gets setting", func(t *testing.T) {
		err := store.SetSetting("chat_id", "123456")
		if err != nil {
			t.Errorf("SetSetting() error = %v", err)
		}

		value, err := store.GetSetting("chat_id")
		if err != nil {
			t.Errorf("GetSetting() error = %v", err)
		}
		if value != "123456" {
			t.Errorf("Setting value = %v, want 123456", value)
		}
	})

	t.Run("returns empty string for missing setting", func(t *testing.T) {
		value, err := store.GetSetting("non_existent")
		if err != nil {
			t.Errorf("GetSetting() error = %v", err)
		}
		if value != "" {
			t.Errorf("Setting value = %v, want empty string", value)
		}
	})

	t.Run("updates existing setting", func(t *testing.T) {
		err := store.SetSetting("digest_time", "09:00")
		if err != nil {
			t.Errorf("SetSetting() error = %v", err)
		}

		err = store.SetSetting("digest_time", "14:00")
		if err != nil {
			t.Errorf("SetSetting() update error = %v", err)
		}

		value, err := store.GetSetting("digest_time")
		if err != nil {
			t.Errorf("GetSetting() error = %v", err)
		}
		if value != "14:00" {
			t.Errorf("Updated value = %v, want 14:00", value)
		}
	})
}

func TestGetSentArticleCount(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	now := time.Now()

	// Create and mark some articles as sent
	for i := 1; i <= 3; i++ {
		article := &Article{
			ID:        int64(i),
			Title:     "Article",
			URL:       "https://example.com",
			FetchedAt: now,
		}
		if err := store.SaveArticle(article); err != nil {
			t.Fatalf("SaveArticle() error = %v", err)
		}
		if err := store.MarkArticleSent(int64(i), int64(i), now); err != nil {
			t.Fatalf("MarkArticleSent() error = %v", err)
		}
	}

	count, err := store.GetSentArticleCount()
	if err != nil {
		t.Errorf("GetSentArticleCount() error = %v", err)
	}
	if count != 3 {
		t.Errorf("Sent article count = %v, want 3", count)
	}
}

// Helper functions

func newTestStore(t *testing.T) *Storage {
	t.Helper()
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}
	return store
}
