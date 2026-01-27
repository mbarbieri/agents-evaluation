package bot

import (
	"log/slog"
	"os"
	"testing"

	"hn-telegram-bot/storage"
)

// MockArticle represents an article in mock storage
type MockArticle struct {
	ID            int64
	Title         string
	Tags          []string
	TelegramMsgID int64
}

// MockStorage implements Storage interface for testing
type MockStorage struct {
	settings   map[string]string
	articles   map[int64]*MockArticle
	likes      map[int64]bool
	tagWeights map[string]float64
	tagCounts  map[string]int
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		settings:   make(map[string]string),
		articles:   make(map[int64]*MockArticle),
		likes:      make(map[int64]bool),
		tagWeights: make(map[string]float64),
		tagCounts:  make(map[string]int),
	}
}

func (m *MockStorage) SetSetting(key, value string) error {
	m.settings[key] = value
	return nil
}

func (m *MockStorage) GetSetting(key string) (string, error) {
	return m.settings[key], nil
}

func (m *MockStorage) FindArticleByMessageID(msgID int64) (*storage.Article, error) {
	for _, article := range m.articles {
		if article.TelegramMsgID == msgID {
			return &storage.Article{
				ID:            article.ID,
				Title:         article.Title,
				Tags:          article.Tags,
				TelegramMsgID: article.TelegramMsgID,
			}, nil
		}
	}
	return nil, nil
}

func (m *MockStorage) RecordLikeWithCheck(articleID int64) (bool, error) {
	if m.likes[articleID] {
		return false, nil
	}
	m.likes[articleID] = true
	return true, nil
}

func (m *MockStorage) GetLikeCount() (int, error) {
	return len(m.likes), nil
}

func (m *MockStorage) GetTopTags(n int) ([]storage.TagWeight, error) {
	var result []storage.TagWeight
	for tag, weight := range m.tagWeights {
		result = append(result, storage.TagWeight{Name: tag, Weight: weight, Count: m.tagCounts[tag]})
		if len(result) >= n {
			break
		}
	}
	return result, nil
}

func (m *MockStorage) UpsertTagWeight(tag string, weight float64, count int) error {
	m.tagWeights[tag] = weight
	m.tagCounts[tag] = count
	return nil
}

func (m *MockStorage) GetAllTagWeights() (map[string]float64, error) {
	return m.tagWeights, nil
}

func TestNew(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("fails with invalid token", func(t *testing.T) {
		config := Config{Token: "invalid-token"}
		storage := NewMockStorage()

		_, err := New(config, storage, logger)
		if err == nil {
			t.Error("New() should error with invalid token")
		}
	})
}

func TestEscapeHTML(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World", "Hello World"},
		{"<script>", "&lt;script&gt;"},
		{"A & B", "A &amp; B"},
		{"<b>Bold</b>", "&lt;b&gt;Bold&lt;/b&gt;"},
	}

	for _, tt := range tests {
		result := escapeHTML(tt.input)
		if result != tt.expected {
			t.Errorf("escapeHTML(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestIsValidTime(t *testing.T) {
	tests := []struct {
		timeStr string
		valid   bool
	}{
		{"00:00", true},
		{"23:59", true},
		{"12:30", true},
		{"09:00", true},
		{"9:00", false},
		{"24:00", false},
		{"12:60", false},
		{"abc", false},
		{"12-30", false},
	}

	for _, tt := range tests {
		result := isValidTime(tt.timeStr)
		if result != tt.valid {
			t.Errorf("isValidTime(%q) = %v, want %v", tt.timeStr, result, tt.valid)
		}
	}
}

func TestMockStorage(t *testing.T) {
	storage := NewMockStorage()

	t.Run("SetSetting and GetSetting", func(t *testing.T) {
		err := storage.SetSetting("test_key", "test_value")
		if err != nil {
			t.Errorf("SetSetting error = %v", err)
		}

		value, err := storage.GetSetting("test_key")
		if err != nil {
			t.Errorf("GetSetting error = %v", err)
		}
		if value != "test_value" {
			t.Errorf("GetSetting = %v, want test_value", value)
		}
	})

	t.Run("RecordLikeWithCheck", func(t *testing.T) {
		// First like should succeed
		isNew, err := storage.RecordLikeWithCheck(123)
		if err != nil {
			t.Errorf("RecordLikeWithCheck error = %v", err)
		}
		if !isNew {
			t.Error("First like should be new")
		}

		// Second like should not be new
		isNew, err = storage.RecordLikeWithCheck(123)
		if err != nil {
			t.Errorf("RecordLikeWithCheck error = %v", err)
		}
		if isNew {
			t.Error("Second like should not be new")
		}
	})

	t.Run("GetLikeCount", func(t *testing.T) {
		count, err := storage.GetLikeCount()
		if err != nil {
			t.Errorf("GetLikeCount error = %v", err)
		}
		if count != 1 {
			t.Errorf("GetLikeCount = %d, want 1", count)
		}
	})

	t.Run("UpsertTagWeight and GetAllTagWeights", func(t *testing.T) {
		err := storage.UpsertTagWeight("go", 1.5, 3)
		if err != nil {
			t.Errorf("UpsertTagWeight error = %v", err)
		}

		weights, err := storage.GetAllTagWeights()
		if err != nil {
			t.Errorf("GetAllTagWeights error = %v", err)
		}

		if weights["go"] != 1.5 {
			t.Errorf("Weight = %v, want 1.5", weights["go"])
		}
	})

	t.Run("FindArticleByMessageID", func(t *testing.T) {
		// Add an article
		storage.articles[1] = &MockArticle{ID: 1, Title: "Test", TelegramMsgID: 100}

		// Find it
		article, err := storage.FindArticleByMessageID(100)
		if err != nil {
			t.Errorf("FindArticleByMessageID error = %v", err)
		}
		if article == nil {
			t.Error("Expected to find article")
			return
		}
		if article.ID != 1 {
			t.Errorf("Article ID = %d, want 1", article.ID)
		}

		// Not found
		article, err = storage.FindArticleByMessageID(999)
		if err != nil {
			t.Errorf("FindArticleByMessageID error = %v", err)
		}
		if article != nil {
			t.Error("Expected nil for non-existent message")
		}
	})
}
