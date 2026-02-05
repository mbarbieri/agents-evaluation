package storage

import (
	"math"
	"path/filepath"
	"testing"
	"time"
)

// newTestStore creates a Store backed by a temporary SQLite database.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("New(%q): %v", dbPath, err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestNew(t *testing.T) {
	t.Run("creates database and tables", func(t *testing.T) {
		s := newTestStore(t)
		// Verify tables exist by running queries against them.
		if _, err := s.db.Exec("SELECT COUNT(*) FROM articles"); err != nil {
			t.Errorf("articles table missing: %v", err)
		}
		if _, err := s.db.Exec("SELECT COUNT(*) FROM likes"); err != nil {
			t.Errorf("likes table missing: %v", err)
		}
		if _, err := s.db.Exec("SELECT COUNT(*) FROM tag_weights"); err != nil {
			t.Errorf("tag_weights table missing: %v", err)
		}
		if _, err := s.db.Exec("SELECT COUNT(*) FROM settings"); err != nil {
			t.Errorf("settings table missing: %v", err)
		}
	})

	t.Run("invalid path returns error", func(t *testing.T) {
		_, err := New("/nonexistent/dir/db.sqlite")
		if err == nil {
			t.Fatal("expected error for invalid path, got nil")
		}
	})
}

func TestSaveArticle(t *testing.T) {
	s := newTestStore(t)

	article := &Article{
		ID:        12345,
		Title:     "Test Article",
		URL:       "https://example.com/test",
		Summary:   "A summary",
		Tags:      `["go","sqlite"]`,
		Score:     150,
		FetchedAt: time.Now().Unix(),
		SentAt:    0,
	}

	if err := s.SaveArticle(article); err != nil {
		t.Fatalf("SaveArticle: %v", err)
	}

	// Verify by querying directly.
	var title string
	err := s.db.QueryRow("SELECT title FROM articles WHERE id = ?", 12345).Scan(&title)
	if err != nil {
		t.Fatalf("query saved article: %v", err)
	}
	if title != "Test Article" {
		t.Errorf("title = %q, want %q", title, "Test Article")
	}

	// Test INSERT OR REPLACE behavior: update the same article.
	article.Title = "Updated Title"
	article.Score = 200
	if err := s.SaveArticle(article); err != nil {
		t.Fatalf("SaveArticle (replace): %v", err)
	}

	err = s.db.QueryRow("SELECT title FROM articles WHERE id = ?", 12345).Scan(&title)
	if err != nil {
		t.Fatalf("query replaced article: %v", err)
	}
	if title != "Updated Title" {
		t.Errorf("title after replace = %q, want %q", title, "Updated Title")
	}
}

func TestGetArticleBySentMsgID(t *testing.T) {
	s := newTestStore(t)

	article := &Article{
		ID:            1,
		Title:         "Sent Article",
		URL:           "https://example.com",
		Summary:       "summary",
		Tags:          `["test"]`,
		Score:         100,
		FetchedAt:     time.Now().Unix(),
		SentAt:        time.Now().Unix(),
		TelegramMsgID: 999,
	}
	if err := s.SaveArticle(article); err != nil {
		t.Fatalf("SaveArticle: %v", err)
	}

	t.Run("found", func(t *testing.T) {
		got, err := s.GetArticleBySentMsgID(999)
		if err != nil {
			t.Fatalf("GetArticleBySentMsgID: %v", err)
		}
		if got == nil {
			t.Fatal("expected article, got nil")
		}
		if got.ID != 1 {
			t.Errorf("ID = %d, want 1", got.ID)
		}
		if got.Title != "Sent Article" {
			t.Errorf("Title = %q, want %q", got.Title, "Sent Article")
		}
		if got.TelegramMsgID != 999 {
			t.Errorf("TelegramMsgID = %d, want 999", got.TelegramMsgID)
		}
	})

	t.Run("not found", func(t *testing.T) {
		got, err := s.GetArticleBySentMsgID(888)
		if err != nil {
			t.Fatalf("GetArticleBySentMsgID: %v", err)
		}
		if got != nil {
			t.Errorf("expected nil, got %+v", got)
		}
	})
}

func TestGetRecentSentArticleIDs(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().Unix()

	articles := []Article{
		{ID: 1, Title: "Recent", SentAt: now - 3600, FetchedAt: now},              // 1 hour ago
		{ID: 2, Title: "Yesterday", SentAt: now - 86400 + 100, FetchedAt: now},    // within 1 day
		{ID: 3, Title: "Old", SentAt: now - 86400*10, FetchedAt: now},             // 10 days ago
		{ID: 4, Title: "Unsent", SentAt: 0, FetchedAt: now},                       // not sent
		{ID: 5, Title: "Three Days Ago", SentAt: now - 86400*3 + 100, FetchedAt: now}, // within 3 days but not 1
	}
	for i := range articles {
		if err := s.SaveArticle(&articles[i]); err != nil {
			t.Fatalf("SaveArticle: %v", err)
		}
	}

	tests := []struct {
		name     string
		days     int
		wantIDs  map[int]bool
		wantLen  int
	}{
		{
			name:    "last 1 day",
			days:    1,
			wantIDs: map[int]bool{1: true, 2: true},
			wantLen: 2,
		},
		{
			name:    "last 7 days",
			days:    7,
			wantIDs: map[int]bool{1: true, 2: true, 5: true},
			wantLen: 3,
		},
		{
			name:    "last 30 days",
			days:    30,
			wantIDs: map[int]bool{1: true, 2: true, 3: true, 5: true},
			wantLen: 4,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ids, err := s.GetRecentSentArticleIDs(tc.days)
			if err != nil {
				t.Fatalf("GetRecentSentArticleIDs(%d): %v", tc.days, err)
			}
			if len(ids) != tc.wantLen {
				t.Errorf("got %d IDs, want %d (ids: %v)", len(ids), tc.wantLen, ids)
			}
			for _, id := range ids {
				if !tc.wantIDs[id] {
					t.Errorf("unexpected ID %d in result", id)
				}
			}
		})
	}
}

func TestMarkSent(t *testing.T) {
	s := newTestStore(t)

	article := &Article{
		ID:        42,
		Title:     "To Be Sent",
		FetchedAt: time.Now().Unix(),
	}
	if err := s.SaveArticle(article); err != nil {
		t.Fatalf("SaveArticle: %v", err)
	}

	beforeMark := time.Now().Unix()
	if err := s.MarkSent(42, 777); err != nil {
		t.Fatalf("MarkSent: %v", err)
	}

	got, err := s.GetArticleBySentMsgID(777)
	if err != nil {
		t.Fatalf("GetArticleBySentMsgID: %v", err)
	}
	if got == nil {
		t.Fatal("expected article after MarkSent, got nil")
	}
	if got.TelegramMsgID != 777 {
		t.Errorf("TelegramMsgID = %d, want 777", got.TelegramMsgID)
	}
	if got.SentAt < beforeMark {
		t.Errorf("SentAt = %d, expected >= %d", got.SentAt, beforeMark)
	}
}

func TestIsLiked(t *testing.T) {
	s := newTestStore(t)

	article := &Article{ID: 10, Title: "Likeable", FetchedAt: time.Now().Unix()}
	if err := s.SaveArticle(article); err != nil {
		t.Fatalf("SaveArticle: %v", err)
	}

	t.Run("before like", func(t *testing.T) {
		liked, err := s.IsLiked(10)
		if err != nil {
			t.Fatalf("IsLiked: %v", err)
		}
		if liked {
			t.Error("expected not liked before RecordLike")
		}
	})

	if err := s.RecordLike(10); err != nil {
		t.Fatalf("RecordLike: %v", err)
	}

	t.Run("after like", func(t *testing.T) {
		liked, err := s.IsLiked(10)
		if err != nil {
			t.Fatalf("IsLiked: %v", err)
		}
		if !liked {
			t.Error("expected liked after RecordLike")
		}
	})
}

func TestRecordLikeIdempotent(t *testing.T) {
	s := newTestStore(t)

	// RecordLike should not fail when called multiple times for the same article.
	if err := s.RecordLike(99); err != nil {
		t.Fatalf("RecordLike (first): %v", err)
	}
	if err := s.RecordLike(99); err != nil {
		t.Fatalf("RecordLike (second): %v", err)
	}

	count, err := s.GetLikeCount()
	if err != nil {
		t.Fatalf("GetLikeCount: %v", err)
	}
	if count != 1 {
		t.Errorf("like count = %d, want 1 (idempotent)", count)
	}
}

func TestTagWeightCRUD(t *testing.T) {
	s := newTestStore(t)

	t.Run("upsert and get single", func(t *testing.T) {
		if err := s.UpsertTagWeight("go", 2.5, 10); err != nil {
			t.Fatalf("UpsertTagWeight: %v", err)
		}
		tw, err := s.GetTagWeight("go")
		if err != nil {
			t.Fatalf("GetTagWeight: %v", err)
		}
		if tw == nil {
			t.Fatal("expected tag weight, got nil")
		}
		if tw.Tag != "go" || tw.Weight != 2.5 || tw.Count != 10 {
			t.Errorf("got %+v, want {Tag:go Weight:2.5 Count:10}", tw)
		}
	})

	t.Run("get not found", func(t *testing.T) {
		tw, err := s.GetTagWeight("nonexistent")
		if err != nil {
			t.Fatalf("GetTagWeight: %v", err)
		}
		if tw != nil {
			t.Errorf("expected nil for nonexistent tag, got %+v", tw)
		}
	})

	t.Run("upsert replaces existing", func(t *testing.T) {
		if err := s.UpsertTagWeight("go", 3.0, 15); err != nil {
			t.Fatalf("UpsertTagWeight (replace): %v", err)
		}
		tw, err := s.GetTagWeight("go")
		if err != nil {
			t.Fatalf("GetTagWeight: %v", err)
		}
		if tw.Weight != 3.0 || tw.Count != 15 {
			t.Errorf("after replace got %+v, want Weight=3.0, Count=15", tw)
		}
	})

	t.Run("get all", func(t *testing.T) {
		if err := s.UpsertTagWeight("rust", 1.5, 5); err != nil {
			t.Fatalf("UpsertTagWeight: %v", err)
		}
		if err := s.UpsertTagWeight("python", 2.0, 8); err != nil {
			t.Fatalf("UpsertTagWeight: %v", err)
		}

		all, err := s.GetTagWeights()
		if err != nil {
			t.Fatalf("GetTagWeights: %v", err)
		}
		if len(all) != 3 {
			t.Errorf("got %d tag weights, want 3", len(all))
		}
	})

	t.Run("get top", func(t *testing.T) {
		// Weights: go=3.0, python=2.0, rust=1.5
		top, err := s.GetTopTagWeights(2)
		if err != nil {
			t.Fatalf("GetTopTagWeights: %v", err)
		}
		if len(top) != 2 {
			t.Fatalf("got %d top weights, want 2", len(top))
		}
		if top[0].Tag != "go" {
			t.Errorf("top[0].Tag = %q, want %q", top[0].Tag, "go")
		}
		if top[1].Tag != "python" {
			t.Errorf("top[1].Tag = %q, want %q", top[1].Tag, "python")
		}
	})
}

func TestApplyDecay(t *testing.T) {
	s := newTestStore(t)

	if err := s.UpsertTagWeight("high", 10.0, 5); err != nil {
		t.Fatalf("UpsertTagWeight: %v", err)
	}
	if err := s.UpsertTagWeight("low", 0.5, 2); err != nil {
		t.Fatalf("UpsertTagWeight: %v", err)
	}

	decayRate := 0.1
	minWeight := 0.3

	if err := s.ApplyDecay(decayRate, minWeight); err != nil {
		t.Fatalf("ApplyDecay: %v", err)
	}

	t.Run("high weight decays", func(t *testing.T) {
		tw, err := s.GetTagWeight("high")
		if err != nil {
			t.Fatalf("GetTagWeight: %v", err)
		}
		// 10.0 * (1 - 0.1) = 9.0
		expected := 9.0
		if math.Abs(tw.Weight-expected) > 0.001 {
			t.Errorf("high weight = %f, want %f", tw.Weight, expected)
		}
	})

	t.Run("low weight respects minimum", func(t *testing.T) {
		tw, err := s.GetTagWeight("low")
		if err != nil {
			t.Fatalf("GetTagWeight: %v", err)
		}
		// 0.5 * (1 - 0.1) = 0.45, which is above minWeight 0.3, so 0.45.
		expected := 0.45
		if math.Abs(tw.Weight-expected) > 0.001 {
			t.Errorf("low weight = %f, want %f", tw.Weight, expected)
		}
	})

	t.Run("weight clamps to minimum", func(t *testing.T) {
		// Set weight to something that will decay below min.
		if err := s.UpsertTagWeight("tiny", 0.2, 1); err != nil {
			t.Fatalf("UpsertTagWeight: %v", err)
		}
		if err := s.ApplyDecay(0.9, 0.1); err != nil {
			t.Fatalf("ApplyDecay: %v", err)
		}
		tw, err := s.GetTagWeight("tiny")
		if err != nil {
			t.Fatalf("GetTagWeight: %v", err)
		}
		// 0.2 * (1 - 0.9) = 0.02, clamped to 0.1
		if math.Abs(tw.Weight-0.1) > 0.001 {
			t.Errorf("tiny weight = %f, want 0.1 (clamped)", tw.Weight)
		}
	})
}

func TestGetLikeCount(t *testing.T) {
	s := newTestStore(t)

	t.Run("empty", func(t *testing.T) {
		count, err := s.GetLikeCount()
		if err != nil {
			t.Fatalf("GetLikeCount: %v", err)
		}
		if count != 0 {
			t.Errorf("count = %d, want 0", count)
		}
	})

	for _, id := range []int{1, 2, 3} {
		if err := s.RecordLike(id); err != nil {
			t.Fatalf("RecordLike(%d): %v", id, err)
		}
	}

	t.Run("after likes", func(t *testing.T) {
		count, err := s.GetLikeCount()
		if err != nil {
			t.Fatalf("GetLikeCount: %v", err)
		}
		if count != 3 {
			t.Errorf("count = %d, want 3", count)
		}
	})
}

func TestSettings(t *testing.T) {
	s := newTestStore(t)

	t.Run("missing key returns empty string", func(t *testing.T) {
		val, err := s.GetSetting("nonexistent")
		if err != nil {
			t.Fatalf("GetSetting: %v", err)
		}
		if val != "" {
			t.Errorf("got %q, want empty string", val)
		}
	})

	t.Run("set and get", func(t *testing.T) {
		if err := s.SetSetting("theme", "dark"); err != nil {
			t.Fatalf("SetSetting: %v", err)
		}
		val, err := s.GetSetting("theme")
		if err != nil {
			t.Fatalf("GetSetting: %v", err)
		}
		if val != "dark" {
			t.Errorf("got %q, want %q", val, "dark")
		}
	})

	t.Run("overwrite existing", func(t *testing.T) {
		if err := s.SetSetting("theme", "light"); err != nil {
			t.Fatalf("SetSetting: %v", err)
		}
		val, err := s.GetSetting("theme")
		if err != nil {
			t.Fatalf("GetSetting: %v", err)
		}
		if val != "light" {
			t.Errorf("got %q, want %q", val, "light")
		}
	})

	t.Run("multiple keys", func(t *testing.T) {
		if err := s.SetSetting("lang", "en"); err != nil {
			t.Fatalf("SetSetting: %v", err)
		}
		lang, err := s.GetSetting("lang")
		if err != nil {
			t.Fatalf("GetSetting lang: %v", err)
		}
		theme, err := s.GetSetting("theme")
		if err != nil {
			t.Fatalf("GetSetting theme: %v", err)
		}
		if lang != "en" {
			t.Errorf("lang = %q, want %q", lang, "en")
		}
		if theme != "light" {
			t.Errorf("theme = %q, want %q", theme, "light")
		}
	})
}

func TestGetTagWeight(t *testing.T) {
	s := newTestStore(t)

	t.Run("not found", func(t *testing.T) {
		tw, err := s.GetTagWeight("missing")
		if err != nil {
			t.Fatalf("GetTagWeight: %v", err)
		}
		if tw != nil {
			t.Errorf("expected nil, got %+v", tw)
		}
	})

	t.Run("found", func(t *testing.T) {
		if err := s.UpsertTagWeight("ai", 5.0, 20); err != nil {
			t.Fatalf("UpsertTagWeight: %v", err)
		}
		tw, err := s.GetTagWeight("ai")
		if err != nil {
			t.Fatalf("GetTagWeight: %v", err)
		}
		if tw == nil {
			t.Fatal("expected tag weight, got nil")
		}
		if tw.Tag != "ai" {
			t.Errorf("Tag = %q, want %q", tw.Tag, "ai")
		}
		if tw.Weight != 5.0 {
			t.Errorf("Weight = %f, want 5.0", tw.Weight)
		}
		if tw.Count != 20 {
			t.Errorf("Count = %d, want 20", tw.Count)
		}
	})
}

func TestCloseAndReopen(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "reopen.db")

	// Create and populate.
	s1, err := New(dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := s1.SetSetting("persist", "yes"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reopen and verify data persists.
	s2, err := New(dbPath)
	if err != nil {
		t.Fatalf("New (reopen): %v", err)
	}
	defer s2.Close()

	val, err := s2.GetSetting("persist")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if val != "yes" {
		t.Errorf("got %q, want %q", val, "yes")
	}
}
