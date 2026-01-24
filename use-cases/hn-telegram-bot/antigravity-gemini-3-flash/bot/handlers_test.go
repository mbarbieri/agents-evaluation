package bot

import (
	"context"
	"testing"

	"github.com/antigravity/hn-telegram-bot/digest"
	"github.com/antigravity/hn-telegram-bot/storage"
)

type mockStorage struct {
	settings map[string]string
	weights  map[string]storage.TagWeight
	articles map[int]*storage.Article
}

func (m *mockStorage) SetSetting(k, v string) error                         { m.settings[k] = v; return nil }
func (m *mockStorage) GetSetting(k string) (string, error)                  { return m.settings[k], nil }
func (m *mockStorage) GetTagWeights() (map[string]storage.TagWeight, error) { return m.weights, nil }
func (m *mockStorage) UpdateTagWeight(t string, w float64, o int) error {
	m.weights[t] = storage.TagWeight{Tag: t, Weight: w, Occurrences: o}
	return nil
}
func (m *mockStorage) IsLiked(id int) (bool, error) { return false, nil }
func (m *mockStorage) MarkLiked(id int) error       { return nil }
func (m *mockStorage) SaveArticle(a *storage.Article) error {
	m.articles[a.HNID] = a
	return nil
}
func (m *mockStorage) GetArticleByMessageID(id int) (*storage.Article, error) {
	for _, a := range m.articles {
		if a.TelegramMessageID == id {
			return a, nil
		}
	}
	return nil, nil
}

type mockWorkflow struct {
	called bool
}

func (m *mockWorkflow) Run(ctx context.Context, sender digest.Sender) error {
	m.called = true
	return nil
}

func TestHandlers(t *testing.T) {
	ms := &mockStorage{
		settings: make(map[string]string),
		weights:  make(map[string]storage.TagWeight),
		articles: make(map[int]*storage.Article),
	}
	mw := &mockWorkflow{}

	h := NewHandler(ms, mw, nil) // Sender is nil for now

	t.Run("StartCommand", func(t *testing.T) {
		msg := h.HandleStart(12345)
		if msg == "" {
			t.Error("expected welcome message, got empty")
		}
		if ms.settings["chat_id"] != "12345" {
			t.Errorf("expected chat_id 12345, got %s", ms.settings["chat_id"])
		}
	})

	t.Run("SettingsCommand", func(t *testing.T) {
		// Test display
		ms.settings["digest_time"] = "09:00"
		ms.settings["article_count"] = "30"
		msg := h.HandleSettings("", 12345)
		if msg == "" {
			t.Error("expected settings message, got empty")
		}

		// Test update time
		msg = h.HandleSettings("time 10:00", 12345)
		if ms.settings["digest_time"] != "10:00" {
			t.Errorf("expected digest_time 10:00, got %s", ms.settings["digest_time"])
		}

		// Test update count
		msg = h.HandleSettings("count 50", 12345)
		if ms.settings["article_count"] != "50" {
			t.Errorf("expected article_count 50, got %s", ms.settings["article_count"])
		}
	})

	t.Run("StatsCommand", func(t *testing.T) {
		ms.weights["go"] = storage.TagWeight{Tag: "go", Weight: 2.0, Occurrences: 1}
		msg := h.HandleStats()
		if msg == "" {
			t.Error("expected stats message, got empty")
		}
	})

	t.Run("HandleReaction", func(t *testing.T) {
		ms.articles[1] = &storage.Article{HNID: 1, TelegramMessageID: 555, Tags: []string{"rust"}}
		ms.weights["rust"] = storage.TagWeight{Tag: "rust", Weight: 1.0, Occurrences: 0}

		err := h.HandleReaction(555, "👍", 0.2)
		if err != nil {
			t.Fatalf("failed to handle reaction: %v", err)
		}

		if ms.weights["rust"].Weight != 1.2 {
			t.Errorf("expected weight 1.2, got %f", ms.weights["rust"].Weight)
		}
		if ms.weights["rust"].Occurrences != 1 {
			t.Errorf("expected 1 occurrence, got %d", ms.weights["rust"].Occurrences)
		}
	})
}
