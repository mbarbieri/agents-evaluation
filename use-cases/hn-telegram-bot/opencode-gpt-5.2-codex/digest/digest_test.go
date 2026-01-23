package digest

import (
	"context"
	"errors"
	"testing"
	"time"
)

type mockHN struct {
	ids  []int64
	item Item
}

func (m mockHN) TopStories(ctx context.Context) ([]int64, error) {
	return m.ids, nil
}

func (m mockHN) Item(ctx context.Context, id int64) (Item, error) {
	return m.item, nil
}

type mockScraper struct {
	content string
	err     error
}

func (m mockScraper) Extract(ctx context.Context, url string) (string, error) {
	return m.content, m.err
}

type mockSummarizer struct {
	result Summary
	err    error
}

func (m mockSummarizer) Summarize(ctx context.Context, content string) (Summary, error) {
	return m.result, m.err
}

type mockStore struct {
	saved       []Article
	recentIDs   []int64
	decayCalled bool
	decayErr    error
	weights     map[string]float64
}

func (m *mockStore) ApplyDecay(ctx context.Context, rate, min float64) error {
	m.decayCalled = true
	return m.decayErr
}

func (m *mockStore) RecentSentIDs(ctx context.Context, since time.Time) ([]int64, error) {
	return m.recentIDs, nil
}

func (m *mockStore) TagWeights(ctx context.Context) (map[string]float64, error) {
	if m.weights == nil {
		return map[string]float64{}, nil
	}
	return m.weights, nil
}

func (m *mockStore) SaveArticle(ctx context.Context, article Article) error {
	m.saved = append(m.saved, article)
	return nil
}

type mockSender struct {
	sent []Sent
	err  error
}

func (m *mockSender) Send(ctx context.Context, article Article) (Sent, error) {
	if m.err != nil {
		return Sent{}, m.err
	}
	sent := Sent{ArticleID: article.ID, MessageID: 10}
	m.sent = append(m.sent, sent)
	return sent, nil
}

func TestWorkflowSkipsSummarizerFailures(t *testing.T) {
	t.Parallel()

	workflow := NewWorkflow(
		WorkflowConfig{ArticleCount: 1, FetchLimit: 1, DecayRate: 0.1, MinTagWeight: 0.1},
		mockHN{ids: []int64{1}, item: Item{ID: 1, Title: "Title", URL: "https://example.com", Score: 10, Descendants: 1}},
		mockScraper{content: "content"},
		mockSummarizer{err: errors.New("boom")},
		&mockStore{},
		&mockSender{},
	)

	if err := workflow.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestWorkflowSendsTopArticles(t *testing.T) {
	t.Parallel()

	store := &mockStore{}
	sender := &mockSender{}
	workflow := NewWorkflow(
		WorkflowConfig{ArticleCount: 1, FetchLimit: 1, DecayRate: 0.1, MinTagWeight: 0.1},
		mockHN{ids: []int64{1}, item: Item{ID: 1, Title: "Title", URL: "https://example.com", Score: 10, Descendants: 1}},
		mockScraper{content: "content"},
		mockSummarizer{result: Summary{Summary: "ok", Tags: []string{"go"}}},
		store,
		sender,
	)

	if err := workflow.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(sender.sent) != 1 {
		t.Fatalf("expected 1 sent article")
	}
	if len(store.saved) != 1 {
		t.Fatalf("expected 1 saved article")
	}
	if !store.decayCalled {
		t.Fatalf("expected decay")
	}
}
