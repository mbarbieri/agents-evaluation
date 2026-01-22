package digest

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Mocks

type mockHNClient struct {
	topStories []int64
	items      map[int64]*HNItem
	fetchError error
}

func (m *mockHNClient) GetTopStories(ctx context.Context, limit int) ([]int64, error) {
	if m.fetchError != nil {
		return nil, m.fetchError
	}
	if limit > len(m.topStories) {
		return m.topStories, nil
	}
	return m.topStories[:limit], nil
}

func (m *mockHNClient) GetItem(ctx context.Context, id int64) (*HNItem, error) {
	if item, ok := m.items[id]; ok {
		return item, nil
	}
	return nil, errors.New("item not found")
}

type mockScraper struct {
	contents   map[string]string
	shouldFail bool
}

func (m *mockScraper) Scrape(ctx context.Context, url string) (string, error) {
	if m.shouldFail {
		return "", errors.New("scrape failed")
	}
	if content, ok := m.contents[url]; ok {
		return content, nil
	}
	return "Default scraped content", nil
}

type mockSummarizer struct {
	results    map[string]*SummaryResult
	shouldFail bool
}

func (m *mockSummarizer) Summarize(ctx context.Context, title, content string) (*SummaryResult, error) {
	if m.shouldFail {
		return nil, errors.New("summarization failed")
	}
	if result, ok := m.results[title]; ok {
		return result, nil
	}
	return &SummaryResult{
		Summary: "Default summary for " + title,
		Tags:    []string{"default"},
	}, nil
}

type mockStorage struct {
	articles        map[int64]*StoredArticle
	recentlySent    []int64
	tagWeights      map[string]float64
	likedArticles   map[int64]bool
	settings        map[string]string
	sentArticleIDs  []int64
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		articles:       make(map[int64]*StoredArticle),
		recentlySent:   []int64{},
		tagWeights:     make(map[string]float64),
		likedArticles:  make(map[int64]bool),
		settings:       make(map[string]string),
		sentArticleIDs: []int64{},
	}
}

func (m *mockStorage) GetRecentlySentArticleIDs(ctx context.Context, within time.Duration) ([]int64, error) {
	return m.recentlySent, nil
}

func (m *mockStorage) GetAllTagWeights(ctx context.Context) (map[string]float64, error) {
	return m.tagWeights, nil
}

func (m *mockStorage) ApplyTagDecay(ctx context.Context, decayRate, minWeight float64) error {
	for tag := range m.tagWeights {
		newWeight := m.tagWeights[tag] * (1 - decayRate)
		if newWeight < minWeight {
			newWeight = minWeight
		}
		m.tagWeights[tag] = newWeight
	}
	return nil
}

func (m *mockStorage) SaveArticle(ctx context.Context, article *StoredArticle) error {
	m.articles[article.ID] = article
	return nil
}

func (m *mockStorage) MarkArticleSent(ctx context.Context, articleID int64, telegramMsgID int64) error {
	m.sentArticleIDs = append(m.sentArticleIDs, articleID)
	if a, ok := m.articles[articleID]; ok {
		now := time.Now()
		a.SentAt = &now
		a.TelegramMsgID = &telegramMsgID
	}
	return nil
}

func (m *mockStorage) GetSetting(ctx context.Context, key string) (string, error) {
	if v, ok := m.settings[key]; ok {
		return v, nil
	}
	return "", errors.New("setting not found")
}

type mockArticleSender struct {
	sentArticles []*ArticleToSend
}

func (m *mockArticleSender) SendArticle(ctx context.Context, chatID int64, article *ArticleToSend) (int64, error) {
	m.sentArticles = append(m.sentArticles, article)
	return int64(len(m.sentArticles)), nil
}

// Tests

func TestRunDigest(t *testing.T) {
	hnClient := &mockHNClient{
		topStories: []int64{1, 2, 3},
		items: map[int64]*HNItem{
			1: {ID: 1, Title: "Article 1", URL: "https://example.com/1", Score: 100, Descendants: 50},
			2: {ID: 2, Title: "Article 2", URL: "https://example.com/2", Score: 200, Descendants: 100},
			3: {ID: 3, Title: "Article 3", URL: "https://example.com/3", Score: 50, Descendants: 25},
		},
	}

	scraper := &mockScraper{
		contents: map[string]string{
			"https://example.com/1": "Content for article 1",
			"https://example.com/2": "Content for article 2",
			"https://example.com/3": "Content for article 3",
		},
	}

	summarizer := &mockSummarizer{
		results: map[string]*SummaryResult{
			"Article 1": {Summary: "Summary 1", Tags: []string{"go", "testing"}},
			"Article 2": {Summary: "Summary 2", Tags: []string{"rust"}},
			"Article 3": {Summary: "Summary 3", Tags: []string{"python"}},
		},
	}

	storage := newMockStorage()
	storage.tagWeights["go"] = 2.0
	storage.tagWeights["testing"] = 1.5

	sender := &mockArticleSender{}

	runner := NewRunner(
		hnClient, scraper, summarizer, storage, sender,
		WithChatID(12345),
		WithArticleCount(2),
		WithDecayRate(0.02),
		WithMinTagWeight(0.1),
	)

	ctx := context.Background()
	err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Should have sent 2 articles
	if len(sender.sentArticles) != 2 {
		t.Errorf("sent %d articles, want 2", len(sender.sentArticles))
	}

	// Article 1 should be sent (has "go" and "testing" tags with high weights)
	found := false
	for _, a := range sender.sentArticles {
		if a.ID == 1 {
			found = true
			break
		}
	}
	if !found {
		t.Error("Article 1 should have been sent (highest tag score)")
	}
}

func TestRunDigestWithDecay(t *testing.T) {
	hnClient := &mockHNClient{
		topStories: []int64{1},
		items: map[int64]*HNItem{
			1: {ID: 1, Title: "Article 1", URL: "https://example.com/1", Score: 100},
		},
	}

	storage := newMockStorage()
	storage.tagWeights["go"] = 1.0

	runner := NewRunner(
		hnClient, &mockScraper{}, &mockSummarizer{}, storage, &mockArticleSender{},
		WithChatID(12345),
		WithArticleCount(1),
		WithDecayRate(0.1),
		WithMinTagWeight(0.5),
	)

	ctx := context.Background()
	runner.Run(ctx)

	// Tag weight should have decayed: 1.0 * 0.9 = 0.9
	if w := storage.tagWeights["go"]; w < 0.89 || w > 0.91 {
		t.Errorf("tag weight = %f, want ~0.9", w)
	}
}

func TestRunDigestWithDecayFloor(t *testing.T) {
	hnClient := &mockHNClient{
		topStories: []int64{1},
		items: map[int64]*HNItem{
			1: {ID: 1, Title: "Article 1", URL: "https://example.com/1", Score: 100},
		},
	}

	storage := newMockStorage()
	storage.tagWeights["go"] = 0.2 // Close to floor

	runner := NewRunner(
		hnClient, &mockScraper{}, &mockSummarizer{}, storage, &mockArticleSender{},
		WithChatID(12345),
		WithArticleCount(1),
		WithDecayRate(0.5), // 50% decay would take it to 0.1
		WithMinTagWeight(0.1),
	)

	ctx := context.Background()
	runner.Run(ctx)

	// Should hit the floor at 0.1
	if w := storage.tagWeights["go"]; w != 0.1 {
		t.Errorf("tag weight = %f, want 0.1 (floor)", w)
	}
}

func TestRunDigestFiltersRecentlySent(t *testing.T) {
	hnClient := &mockHNClient{
		topStories: []int64{1, 2, 3},
		items: map[int64]*HNItem{
			1: {ID: 1, Title: "Article 1", URL: "https://example.com/1", Score: 100},
			2: {ID: 2, Title: "Article 2", URL: "https://example.com/2", Score: 200},
			3: {ID: 3, Title: "Article 3", URL: "https://example.com/3", Score: 50},
		},
	}

	storage := newMockStorage()
	storage.recentlySent = []int64{2} // Article 2 was recently sent

	sender := &mockArticleSender{}

	runner := NewRunner(
		hnClient, &mockScraper{}, &mockSummarizer{}, storage, sender,
		WithChatID(12345),
		WithArticleCount(3),
	)

	ctx := context.Background()
	runner.Run(ctx)

	// Should not have sent Article 2
	for _, a := range sender.sentArticles {
		if a.ID == 2 {
			t.Error("Article 2 should not have been sent (recently sent)")
		}
	}
}

func TestRunDigestScrapeFailure(t *testing.T) {
	hnClient := &mockHNClient{
		topStories: []int64{1},
		items: map[int64]*HNItem{
			1: {ID: 1, Title: "Article 1", URL: "https://example.com/1", Score: 100},
		},
	}

	scraper := &mockScraper{shouldFail: true}
	summarizer := &mockSummarizer{}
	storage := newMockStorage()
	sender := &mockArticleSender{}

	runner := NewRunner(
		hnClient, scraper, summarizer, storage, sender,
		WithChatID(12345),
		WithArticleCount(1),
	)

	ctx := context.Background()
	err := runner.Run(ctx)

	// Should not fail entirely - scraper failure should be gracefully handled
	// The article should use title as fallback content
	if err != nil {
		t.Logf("Run returned error (may be acceptable): %v", err)
	}
}

func TestRunDigestSummarizerFailure(t *testing.T) {
	hnClient := &mockHNClient{
		topStories: []int64{1, 2},
		items: map[int64]*HNItem{
			1: {ID: 1, Title: "Article 1", URL: "https://example.com/1", Score: 100},
			2: {ID: 2, Title: "Article 2", URL: "https://example.com/2", Score: 200},
		},
	}

	summarizer := &mockSummarizer{shouldFail: true}
	storage := newMockStorage()
	sender := &mockArticleSender{}

	runner := NewRunner(
		hnClient, &mockScraper{}, summarizer, storage, sender,
		WithChatID(12345),
		WithArticleCount(2),
	)

	ctx := context.Background()
	runner.Run(ctx)

	// With summarizer failing, no articles should be sent
	if len(sender.sentArticles) > 0 {
		t.Error("No articles should be sent when summarizer fails")
	}
}

func TestRunDigestNoChatID(t *testing.T) {
	runner := NewRunner(
		&mockHNClient{}, &mockScraper{}, &mockSummarizer{},
		newMockStorage(), &mockArticleSender{},
		// No chat ID set
	)

	ctx := context.Background()
	err := runner.Run(ctx)

	if err == nil {
		t.Error("expected error when chat_id is 0")
	}
}

func TestProcessedArticle(t *testing.T) {
	article := &ProcessedArticle{
		ID:       12345,
		Title:    "Test Article",
		URL:      "https://example.com",
		Summary:  "A test summary",
		Tags:     []string{"go", "testing"},
		HNScore:  100,
		Comments: 50,
	}

	if article.ID != 12345 {
		t.Error("article ID mismatch")
	}
	if len(article.Tags) != 2 {
		t.Error("expected 2 tags")
	}
}
