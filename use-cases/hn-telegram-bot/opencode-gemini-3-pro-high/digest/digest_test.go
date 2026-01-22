package digest

import (
	"hn-telegram-bot/hn"
	"hn-telegram-bot/storage"
	"testing"
	"time"
)

// Mocks

type MockStorage struct {
	DecayCalled bool
	Articles    []storage.Article
}

func (m *MockStorage) ApplyTagDecay(rate, min float64) error {
	m.DecayCalled = true
	return nil
}
func (m *MockStorage) GetRecentSentArticleIDs(d time.Duration) ([]int, error) {
	return []int{999}, nil // 999 is recently sent
}
func (m *MockStorage) GetTagWeights() (map[string]float64, error) {
	return map[string]float64{"go": 1.0}, nil
}
func (m *MockStorage) SaveArticle(a storage.Article) error {
	m.Articles = append(m.Articles, a)
	return nil
}
func (m *MockStorage) MarkArticleSent(id, msgID int) error {
	return nil
}

type MockHN struct{}

func (m *MockHN) GetTopStories() ([]int, error) {
	return []int{100, 200, 999}, nil // 999 should be filtered
}
func (m *MockHN) GetItem(id int) (*hn.Item, error) {
	return &hn.Item{
		ID:    id,
		Type:  "story",
		Title: "Test Article",
		URL:   "http://test.com",
		Score: 100,
		Time:  time.Now().Unix(),
	}, nil
}

type MockScraper struct{}

func (m *MockScraper) Scrape(url string) (string, error) {
	return "Scraped content", nil
}

type MockSummarizer struct{}

func (m *MockSummarizer) Summarize(text string) (string, []string, error) {
	return "Summary", []string{"go"}, nil
}

type MockSender struct {
	SentCount int
}

func (m *MockSender) SendArticle(a storage.Article) (int, error) {
	m.SentCount++
	return 12345, nil // msgID
}

func TestRunDigest(t *testing.T) {
	store := &MockStorage{}
	hnClient := &MockHN{}
	scraper := &MockScraper{}
	summarizer := &MockSummarizer{}
	sender := &MockSender{}

	d := New(store, hnClient, scraper, summarizer, sender)
	d.ArticleCount = 2
	d.TagDecayRate = 0.1
	d.MinTagWeight = 0.1

	err := d.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if !store.DecayCalled {
		t.Error("Tag decay not applied")
	}

	// We have 3 stories: 100, 200, 999.
	// 999 is filtered (recent).
	// 100 and 200 processed.
	// Both sent? Count=2.

	if sender.SentCount != 2 {
		t.Errorf("Expected 2 articles sent, got %d", sender.SentCount)
	}

	if len(store.Articles) != 2 {
		t.Errorf("Expected 2 articles saved, got %d", len(store.Articles))
	}
}
