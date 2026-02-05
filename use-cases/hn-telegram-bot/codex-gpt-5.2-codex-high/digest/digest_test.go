package digest

import (
	"context"
	"errors"
	"testing"
	"time"

	"hn-telegram-bot/hn"
	"hn-telegram-bot/model"
)

type mockHN struct {
	top   []int64
	items map[int64]hn.Item
}

func (m *mockHN) TopStories(ctx context.Context) ([]int64, error) {
	return m.top, nil
}

func (m *mockHN) Item(ctx context.Context, id int64) (hn.Item, error) {
	item, ok := m.items[id]
	if !ok {
		return hn.Item{}, errors.New("not found")
	}
	return item, nil
}

type mockScraper struct {
	results map[string]string
	errors  map[string]error
}

func (m *mockScraper) Scrape(ctx context.Context, url string) (string, error) {
	if err := m.errors[url]; err != nil {
		return "", err
	}
	return m.results[url], nil
}

type mockSummarizer struct {
	results map[string]model.SummaryResult
	err     error
	inputs  []string
}

func (m *mockSummarizer) Summarize(ctx context.Context, content string) (model.SummaryResult, error) {
	m.inputs = append(m.inputs, content)
	if m.err != nil {
		return model.SummaryResult{}, m.err
	}
	if res, ok := m.results[content]; ok {
		return res, nil
	}
	return model.SummaryResult{}, errors.New("missing result")
}

type mockStorage struct {
	applyDecayCalled bool
	decayRate        float64
	minWeight        float64
	sentIDs          []int64
	weights          map[string]model.TagWeight
	upserted         []model.Article
}

func (m *mockStorage) ApplyDecay(ctx context.Context, decayRate, minWeight float64) error {
	m.applyDecayCalled = true
	m.decayRate = decayRate
	m.minWeight = minWeight
	return nil
}

func (m *mockStorage) ListSentArticleIDsSince(ctx context.Context, since time.Time) ([]int64, error) {
	return m.sentIDs, nil
}

func (m *mockStorage) UpsertArticle(ctx context.Context, article model.Article) error {
	m.upserted = append(m.upserted, article)
	return nil
}

func (m *mockStorage) GetTagWeights(ctx context.Context) (map[string]model.TagWeight, error) {
	return m.weights, nil
}

type mockSender struct {
	sent []model.Article
	id   int
}

func (m *mockSender) SendArticle(ctx context.Context, article model.Article) (int, error) {
	m.sent = append(m.sent, article)
	m.id++
	return 100 + m.id, nil
}

func TestRunnerRun(t *testing.T) {
	now := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	hnClient := &mockHN{
		top: []int64{1, 2, 3},
		items: map[int64]hn.Item{
			1: {ID: 1, Type: "story", Title: "A", URL: "http://a", Score: 10, Descendants: 5},
			2: {ID: 2, Type: "story", Title: "Skip", URL: "http://skip", Score: 5, Descendants: 1},
			3: {ID: 3, Type: "story", Title: "B", URL: "http://b", Score: 1, Descendants: 0},
		},
	}
	scraper := &mockScraper{
		results: map[string]string{"http://a": "content A"},
		errors:  map[string]error{"http://b": errors.New("boom")},
	}
	summarizer := &mockSummarizer{results: map[string]model.SummaryResult{
		"content A": {Summary: "sumA", Tags: []string{"go"}},
		"B":         {Summary: "sumB", Tags: []string{"ai"}},
	}}
	storage := &mockStorage{
		sentIDs: []int64{2},
		weights: map[string]model.TagWeight{
			"go": {Tag: "go", Weight: 2.0},
			"ai": {Tag: "ai", Weight: 0.1},
		},
	}
	sender := &mockSender{}

	runner := &Runner{
		HN:           hnClient,
		Scraper:      scraper,
		Summarizer:   summarizer,
		Storage:      storage,
		Sender:       sender,
		ArticleCount: 2,
		TagDecayRate: 0.2,
		MinTagWeight: 0.1,
		Now: func() time.Time {
			return now
		},
	}

	if err := runner.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !storage.applyDecayCalled || storage.decayRate != 0.2 {
		t.Fatalf("expected decay to be applied")
	}
	if len(sender.sent) != 2 {
		t.Fatalf("expected 2 articles sent, got %d", len(sender.sent))
	}
	if sender.sent[0].ID != 1 {
		t.Fatalf("expected article 1 sent first, got %d", sender.sent[0].ID)
	}
	if len(storage.upserted) != 2 {
		t.Fatalf("expected 2 articles stored, got %d", len(storage.upserted))
	}
	if storage.upserted[0].TelegramMsgID == 0 || storage.upserted[0].SentAt == nil {
		t.Fatalf("expected stored article with message id and sent time")
	}
	if len(summarizer.inputs) < 2 || summarizer.inputs[1] != "B" {
		t.Fatalf("expected summarizer to receive title fallback, got %v", summarizer.inputs)
	}
}
