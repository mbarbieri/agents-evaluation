package digest

import (
	"context"
	"testing"

	"github.com/antigravity/hn-telegram-bot/hn"
	"github.com/antigravity/hn-telegram-bot/storage"
)

type mockStorage struct {
	weights map[string]storage.TagWeight
	ids     []int
}

func (m *mockStorage) GetTagWeights() (map[string]storage.TagWeight, error) { return m.weights, nil }
func (m *mockStorage) UpdateTagWeight(t string, w float64, o int) error     { return nil }
func (m *mockStorage) GetRecentHNIDs(d int) ([]int, error)                  { return m.ids, nil }
func (m *mockStorage) SaveArticle(a *storage.Article) error                 { return nil }

type mockHN struct{}

func (m *mockHN) GetTopStories() ([]int, error) { return []int{1, 2}, nil }
func (m *mockHN) GetItem(id int) (*hn.Item, error) {
	return &hn.Item{ID: id, Title: "Title", URL: "URL", Score: 100}, nil
}

type mockScraper struct{}

func (m *mockScraper) Scrape(url string) (string, error) { return "content", nil }

type mockSummarizer struct{}

func (m *mockSummarizer) Summarize(t, c string) (string, []string, error) {
	return "summary", []string{"tag1"}, nil
}

type mockSender struct {
	sentCount int
}

func (m *mockSender) SendArticle(a *storage.Article) (int, error) {
	m.sentCount++
	return 123, nil
}

func TestWorkflow(t *testing.T) {
	ms := &mockStorage{
		weights: map[string]storage.TagWeight{"tag1": {Tag: "tag1", Weight: 1.5, Occurrences: 10}},
		ids:     []int{},
	}
	mh := &mockHN{}
	msc := &mockScraper{}
	msu := &mockSummarizer{}
	mrs := &mockSender{}

	config := &WorkflowConfig{
		ArticleCount: 1,
		TagDecayRate: 0.02,
		MinTagWeight: 0.1,
		RecentDays:   7,
	}

	w := NewWorkflow(ms, mh, msc, msu, config)

	t.Run("RunCycle", func(t *testing.T) {
		err := w.Run(context.Background(), mrs)
		if err != nil {
			t.Fatalf("workflow failed: %v", err)
		}
		if mrs.sentCount != 1 {
			t.Errorf("expected 1 sent article, got %d", mrs.sentCount)
		}
	})
}
