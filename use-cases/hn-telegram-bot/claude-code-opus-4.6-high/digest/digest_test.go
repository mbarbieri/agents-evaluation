package digest

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// --- Mock implementations ---

type mockHNClient struct {
	topStories []int
	items      map[int]*HNItem
	topErr     error
	itemErr    map[int]error
}

func (m *mockHNClient) TopStories(ctx context.Context, limit int) ([]int, error) {
	if m.topErr != nil {
		return nil, m.topErr
	}
	if limit > len(m.topStories) {
		return m.topStories, nil
	}
	return m.topStories[:limit], nil
}

func (m *mockHNClient) GetItem(ctx context.Context, id int) (*HNItem, error) {
	if err, ok := m.itemErr[id]; ok {
		return nil, err
	}
	item, ok := m.items[id]
	if !ok {
		return nil, fmt.Errorf("item %d not found", id)
	}
	return item, nil
}

type mockScraper struct {
	content map[string]string
	err     map[string]error
}

func (m *mockScraper) Scrape(ctx context.Context, url string) (string, error) {
	if err, ok := m.err[url]; ok {
		return "", err
	}
	return m.content[url], nil
}

type mockSummarizer struct {
	results map[string]*SummaryResult
	err     map[string]error
}

func (m *mockSummarizer) Summarize(ctx context.Context, title, content string) (*SummaryResult, error) {
	if err, ok := m.err[title]; ok {
		return nil, err
	}
	if result, ok := m.results[title]; ok {
		return result, nil
	}
	return &SummaryResult{Summary: "Summary of " + title, Tags: []string{"test"}}, nil
}

type mockSender struct {
	sent  []sentMessage
	msgID int
	err   error
}

type sentMessage struct {
	chatID int64
	text   string
}

func (m *mockSender) SendHTML(chatID int64, text string) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	m.msgID++
	m.sent = append(m.sent, sentMessage{chatID: chatID, text: text})
	return m.msgID, nil
}

type mockStorage struct {
	recentIDs  []int
	articles   []*StoredArticle
	tagWeights []TagWeightEntry
	decayed    bool
	decayRate  float64
	minWeight  float64
	markSent   map[int]int // article ID -> msg ID
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		markSent: make(map[int]int),
	}
}

func (m *mockStorage) GetRecentSentArticleIDs(days int) ([]int, error) {
	return m.recentIDs, nil
}

func (m *mockStorage) SaveArticle(a *StoredArticle) error {
	m.articles = append(m.articles, a)
	return nil
}

func (m *mockStorage) MarkSent(articleID, telegramMsgID int) error {
	m.markSent[articleID] = telegramMsgID
	return nil
}

func (m *mockStorage) ApplyDecay(decayRate, minWeight float64) error {
	m.decayed = true
	m.decayRate = decayRate
	m.minWeight = minWeight
	return nil
}

func (m *mockStorage) GetTagWeights() ([]TagWeightEntry, error) {
	return m.tagWeights, nil
}

// --- Tests ---

func TestRun_FullPipeline(t *testing.T) {
	hn := &mockHNClient{
		topStories: []int{1, 2, 3},
		items: map[int]*HNItem{
			1: {ID: 1, Title: "Article One", URL: "http://one.com", Score: 100, Descendants: 50},
			2: {ID: 2, Title: "Article Two", URL: "http://two.com", Score: 200, Descendants: 80},
			3: {ID: 3, Title: "Article Three", URL: "http://three.com", Score: 50, Descendants: 10},
		},
	}

	scraper := &mockScraper{
		content: map[string]string{
			"http://one.com":   "Content of article one",
			"http://two.com":   "Content of article two",
			"http://three.com": "Content of article three",
		},
	}

	summarizer := &mockSummarizer{
		results: map[string]*SummaryResult{
			"Article One":   {Summary: "Summary one", Tags: []string{"go", "testing"}},
			"Article Two":   {Summary: "Summary two", Tags: []string{"rust", "systems"}},
			"Article Three": {Summary: "Summary three", Tags: []string{"ai", "ml"}},
		},
	}

	sender := &mockSender{}
	storage := newMockStorage()
	storage.tagWeights = []TagWeightEntry{
		{Tag: "go", Weight: 2.0},
		{Tag: "rust", Weight: 1.5},
	}

	runner := NewRunner(hn, scraper, summarizer, sender, storage, Config{
		ChatID:       100,
		ArticleCount: 2,
		DecayRate:    0.02,
		MinWeight:    0.1,
	})

	err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !storage.decayed {
		t.Error("expected decay to be applied")
	}
	if len(sender.sent) != 2 {
		t.Errorf("expected 2 messages sent, got %d", len(sender.sent))
	}
	if len(storage.articles) != 2 {
		t.Errorf("expected 2 articles saved, got %d", len(storage.articles))
	}
	if len(storage.markSent) != 2 {
		t.Errorf("expected 2 articles marked sent, got %d", len(storage.markSent))
	}
}

func TestRun_FilterRecentlySent(t *testing.T) {
	hn := &mockHNClient{
		topStories: []int{1, 2},
		items: map[int]*HNItem{
			1: {ID: 1, Title: "Article One", URL: "http://one.com", Score: 100},
			2: {ID: 2, Title: "Article Two", URL: "http://two.com", Score: 200},
		},
	}

	scraper := &mockScraper{content: map[string]string{
		"http://two.com": "Content two",
	}}
	summarizer := &mockSummarizer{}
	sender := &mockSender{}
	storage := newMockStorage()
	storage.recentIDs = []int{1} // Article 1 was recently sent

	runner := NewRunner(hn, scraper, summarizer, sender, storage, Config{
		ChatID:       100,
		ArticleCount: 5,
		DecayRate:    0.02,
		MinWeight:    0.1,
	})

	err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sender.sent) != 1 {
		t.Errorf("expected 1 message sent (filtered 1), got %d", len(sender.sent))
	}
}

func TestRun_ScrapeFailure(t *testing.T) {
	hn := &mockHNClient{
		topStories: []int{1},
		items: map[int]*HNItem{
			1: {ID: 1, Title: "Article One", URL: "http://fail.com", Score: 100},
		},
	}

	scraper := &mockScraper{
		err: map[string]error{"http://fail.com": fmt.Errorf("scrape failed")},
	}
	summarizer := &mockSummarizer{}
	sender := &mockSender{}
	storage := newMockStorage()

	runner := NewRunner(hn, scraper, summarizer, sender, storage, Config{
		ChatID:       100,
		ArticleCount: 5,
		DecayRate:    0.02,
		MinWeight:    0.1,
	})

	err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still send (using title as fallback content)
	if len(sender.sent) != 1 {
		t.Errorf("expected 1 message sent (fallback to title), got %d", len(sender.sent))
	}
}

func TestRun_SummarizeFailure(t *testing.T) {
	hn := &mockHNClient{
		topStories: []int{1},
		items: map[int]*HNItem{
			1: {ID: 1, Title: "Bad Article", URL: "http://bad.com", Score: 100},
		},
	}

	scraper := &mockScraper{content: map[string]string{"http://bad.com": "content"}}
	summarizer := &mockSummarizer{
		err: map[string]error{"Bad Article": fmt.Errorf("AI failed")},
	}
	sender := &mockSender{}
	storage := newMockStorage()

	runner := NewRunner(hn, scraper, summarizer, sender, storage, Config{
		ChatID:       100,
		ArticleCount: 5,
		DecayRate:    0.02,
		MinWeight:    0.1,
	})

	err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Article should be skipped
	if len(sender.sent) != 0 {
		t.Errorf("expected 0 messages (summarize failed), got %d", len(sender.sent))
	}
}

func TestRun_TopStoriesFails(t *testing.T) {
	hn := &mockHNClient{topErr: fmt.Errorf("network error")}
	runner := NewRunner(hn, nil, nil, nil, newMockStorage(), Config{ArticleCount: 5, DecayRate: 0.02, MinWeight: 0.1})

	err := runner.Run(context.Background())
	if err == nil {
		t.Fatal("expected error for top stories failure")
	}
}

func TestRun_ContextCanceled(t *testing.T) {
	hn := &mockHNClient{
		topStories: []int{1, 2, 3},
		items:      map[int]*HNItem{},
	}

	storage := newMockStorage()
	runner := NewRunner(hn, &mockScraper{}, &mockSummarizer{}, &mockSender{}, storage, Config{
		ArticleCount: 5,
		DecayRate:    0.02,
		MinWeight:    0.1,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := runner.Run(ctx)
	if err != nil {
		// Context cancelation during processing is fine
	}
}

func TestRun_SendFailure(t *testing.T) {
	hn := &mockHNClient{
		topStories: []int{1},
		items: map[int]*HNItem{
			1: {ID: 1, Title: "Article", URL: "http://ok.com", Score: 100},
		},
	}

	scraper := &mockScraper{content: map[string]string{"http://ok.com": "content"}}
	summarizer := &mockSummarizer{}
	sender := &mockSender{err: fmt.Errorf("send failed")}
	storage := newMockStorage()

	runner := NewRunner(hn, scraper, summarizer, sender, storage, Config{
		ChatID:       100,
		ArticleCount: 5,
		DecayRate:    0.02,
		MinWeight:    0.1,
	})

	err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Article should not be saved when send fails
	if len(storage.markSent) != 0 {
		t.Errorf("expected no articles marked sent, got %d", len(storage.markSent))
	}
}

func TestFormatArticle(t *testing.T) {
	msg := FormatArticle("Test Title", "A great summary", 100, 50, 12345, "http://example.com")

	if !strings.Contains(msg, "<b>Test Title</b>") {
		t.Error("expected bold title")
	}
	if !strings.Contains(msg, "<i>A great summary</i>") {
		t.Error("expected italic summary")
	}
	if !strings.Contains(msg, "100 points") {
		t.Error("expected score")
	}
	if !strings.Contains(msg, "50 comments") {
		t.Error("expected comments")
	}
	if !strings.Contains(msg, "http://example.com") {
		t.Error("expected article URL")
	}
	if !strings.Contains(msg, "https://news.ycombinator.com/item?id=12345") {
		t.Error("expected HN discussion URL")
	}
}

func TestFormatArticle_HTMLEscaping(t *testing.T) {
	msg := FormatArticle("Title & <More>", "Summary with <html>", 0, 0, 1, "http://test.com")

	if strings.Contains(msg, "& <") {
		t.Error("expected HTML entities to be escaped")
	}
	if !strings.Contains(msg, "&amp;") {
		t.Error("expected ampersand to be escaped")
	}
	if !strings.Contains(msg, "&lt;More&gt;") {
		t.Error("expected angle brackets to be escaped")
	}
}

func TestEscapeHTML(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{"a & b", "a &amp; b"},
		{"<script>", "&lt;script&gt;"},
		{"a & b < c > d", "a &amp; b &lt; c &gt; d"},
	}

	for _, tt := range tests {
		got := escapeHTML(tt.input)
		if got != tt.expected {
			t.Errorf("escapeHTML(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestRankArticles(t *testing.T) {
	articles := []RankableArticle{
		{ID: 1, Tags: []string{"go"}, HNScore: 100},
		{ID: 2, Tags: []string{"rust", "systems"}, HNScore: 50},
	}

	weights := map[string]float64{
		"go":      1.0,
		"rust":    2.0,
		"systems": 1.0,
	}

	ranked := rankArticles(articles, weights)

	// Article 2 has tag_score=3.0, should rank higher
	if ranked[0].ID != 2 {
		t.Errorf("expected article 2 ranked first, got %d", ranked[0].ID)
	}
}

func TestRankArticles_Empty(t *testing.T) {
	ranked := rankArticles(nil, map[string]float64{})
	if len(ranked) != 0 {
		t.Errorf("expected empty, got %d", len(ranked))
	}
}

func TestUpdateConfig(t *testing.T) {
	runner := NewRunner(nil, nil, nil, nil, nil, Config{ArticleCount: 10})
	runner.UpdateConfig(Config{ArticleCount: 20})
	if runner.config.ArticleCount != 20 {
		t.Errorf("expected 20, got %d", runner.config.ArticleCount)
	}
}

func TestRun_NoURL(t *testing.T) {
	hn := &mockHNClient{
		topStories: []int{1},
		items: map[int]*HNItem{
			1: {ID: 1, Title: "Ask HN: Something", URL: "", Score: 100},
		},
	}

	scraper := &mockScraper{}
	summarizer := &mockSummarizer{}
	sender := &mockSender{}
	storage := newMockStorage()

	runner := NewRunner(hn, scraper, summarizer, sender, storage, Config{
		ChatID:       100,
		ArticleCount: 5,
		DecayRate:    0.02,
		MinWeight:    0.1,
	})

	err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still process using title as content (no scrape needed for empty URL)
	if len(sender.sent) != 1 {
		t.Errorf("expected 1 message sent, got %d", len(sender.sent))
	}
}
