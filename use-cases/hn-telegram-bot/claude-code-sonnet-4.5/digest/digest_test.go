package digest

import (
	"errors"
	"testing"
)

// Mock types
type mockHNClient struct {
	topStories []int
	items      map[int]*mockItem
	err        error
}

type mockItem struct {
	ID          int
	Title       string
	URL         string
	Score       int
	Descendants int
}

func (m *mockItem) GetID() int          { return m.ID }
func (m *mockItem) GetTitle() string    { return m.Title }
func (m *mockItem) GetURL() string      { return m.URL }
func (m *mockItem) GetScore() int       { return m.Score }
func (m *mockItem) GetDescendants() int { return m.Descendants }

func (m *mockHNClient) GetTopStories() ([]int, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.topStories, nil
}

func (m *mockHNClient) GetItem(id int) (HNItem, error) {
	if m.err != nil {
		return nil, m.err
	}
	if item, ok := m.items[id]; ok {
		return item, nil
	}
	return nil, errors.New("not found")
}

type mockScraper struct {
	content string
	err     error
}

func (m *mockScraper) Scrape(url string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.content, nil
}

type mockSummarizer struct {
	summary *mockSummary
	err     error
}

type mockSummary struct {
	Summary string
	Tags    []string
}

func (m *mockSummary) GetSummary() string  { return m.Summary }
func (m *mockSummary) GetTags() []string   { return m.Tags }

func (m *mockSummarizer) Summarize(title, content string) (Summary, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.summary, nil
}

type mockRanker struct {
	ranked []RankedArticle
}

func (m *mockRanker) Rank(articles []RankerArticle, tagWeights map[string]float64) []RankedArticle {
	return m.ranked
}

type mockStorage struct {
	recentArticleIDs []int
	tagWeights       map[string]float64
	savedArticles    []*StorageArticle
	decayApplied     bool
	decayRate        float64
	minWeight        float64
	chatID           string
	articleCount     string
}

func (m *mockStorage) ApplyDecay(decayRate, minWeight float64) error {
	m.decayApplied = true
	m.decayRate = decayRate
	m.minWeight = minWeight
	return nil
}

func (m *mockStorage) GetRecentlySentArticles(days int) ([]int, error) {
	return m.recentArticleIDs, nil
}

func (m *mockStorage) GetAllTagWeights() ([]TagWeight, error) {
	var weights []TagWeight
	for tag, weight := range m.tagWeights {
		weights = append(weights, TagWeight{Tag: tag, Weight: weight})
	}
	return weights, nil
}

func (m *mockStorage) SaveArticle(article *StorageArticle) error {
	m.savedArticles = append(m.savedArticles, article)
	return nil
}

func (m *mockStorage) GetSetting(key string) (string, error) {
	switch key {
	case "chat_id":
		return m.chatID, nil
	case "article_count":
		return m.articleCount, nil
	}
	return "", nil
}

type mockBot struct {
	sentArticles []sentArticle
}

type sentArticle struct {
	chatID   int64
	title    string
	url      string
	summary  string
	score    int
	comments int
	hnID     int
}

func (m *mockBot) SendArticle(chatID int64, title, url, summary string, score, comments, hnID int) (int, error) {
	m.sentArticles = append(m.sentArticles, sentArticle{
		chatID:   chatID,
		title:    title,
		url:      url,
		summary:  summary,
		score:    score,
		comments: comments,
		hnID:     hnID,
	})
	return len(m.sentArticles), nil
}

func TestRun_Success(t *testing.T) {
	hnClient := &mockHNClient{
		topStories: []int{1, 2, 3},
		items: map[int]*mockItem{
			1: {ID: 1, Title: "Article 1", URL: "https://example.com/1", Score: 100, Descendants: 10},
			2: {ID: 2, Title: "Article 2", URL: "https://example.com/2", Score: 200, Descendants: 20},
			3: {ID: 3, Title: "Article 3", URL: "https://example.com/3", Score: 150, Descendants: 15},
		},
	}

	scraper := &mockScraper{content: "Article content"}
	summarizer := &mockSummarizer{
		summary: &mockSummary{
			Summary: "Test summary",
			Tags:    []string{"test", "golang"},
		},
	}

	ranker := &mockRanker{
		ranked: []RankedArticle{
			{Article: RankerArticle{ID: 2}},
			{Article: RankerArticle{ID: 1}},
			{Article: RankerArticle{ID: 3}},
		},
	}

	storage := &mockStorage{
		recentArticleIDs: []int{},
		tagWeights:       map[string]float64{"test": 1.5},
		chatID:           "12345",
		articleCount:     "2",
	}

	bot := &mockBot{}

	workflow := &Workflow{
		hnClient:      hnClient,
		scraper:       scraper,
		summarizer:    summarizer,
		ranker:        ranker,
		storage:       storage,
		bot:           bot,
		decayRate:     0.02,
		minWeight:     0.1,
		bufferFactor:  2.0,
	}

	workflow.Run()

	// Verify decay was applied
	if !storage.decayApplied {
		t.Error("Decay should have been applied")
	}

	// Verify articles were sent (should send top 2 based on articleCount)
	if len(bot.sentArticles) != 2 {
		t.Errorf("Sent %d articles, want 2", len(bot.sentArticles))
	}

	// Verify correct articles were sent (2 and 1 based on ranking)
	if len(bot.sentArticles) >= 2 {
		if bot.sentArticles[0].hnID != 2 {
			t.Errorf("First article ID = %d, want 2", bot.sentArticles[0].hnID)
		}
		if bot.sentArticles[1].hnID != 1 {
			t.Errorf("Second article ID = %d, want 1", bot.sentArticles[1].hnID)
		}
	}

	// Verify articles were saved
	if len(storage.savedArticles) != 2 {
		t.Errorf("Saved %d articles, want 2", len(storage.savedArticles))
	}
}

func TestRun_FiltersRecentArticles(t *testing.T) {
	hnClient := &mockHNClient{
		topStories: []int{1, 2, 3},
		items: map[int]*mockItem{
			1: {ID: 1, Title: "Article 1", URL: "https://example.com/1", Score: 100, Descendants: 10},
			2: {ID: 2, Title: "Article 2", URL: "https://example.com/2", Score: 200, Descendants: 20},
			3: {ID: 3, Title: "Article 3", URL: "https://example.com/3", Score: 150, Descendants: 15},
		},
	}

	scraper := &mockScraper{content: "Content"}
	summarizer := &mockSummarizer{summary: &mockSummary{Summary: "Summary", Tags: []string{"test"}}}
	ranker := &mockRanker{
		ranked: []RankedArticle{
			{Article: RankerArticle{ID: 3}},
		},
	}

	storage := &mockStorage{
		recentArticleIDs: []int{1, 2}, // Articles 1 and 2 were recently sent
		tagWeights:       map[string]float64{},
		chatID:           "12345",
		articleCount:     "10",
	}

	bot := &mockBot{}

	workflow := &Workflow{
		hnClient:      hnClient,
		scraper:       scraper,
		summarizer:    summarizer,
		ranker:        ranker,
		storage:       storage,
		bot:           bot,
		decayRate:     0.02,
		minWeight:     0.1,
		bufferFactor:  2.0,
	}

	workflow.Run()

	// Should only send article 3 (1 and 2 were filtered out)
	if len(bot.sentArticles) != 1 {
		t.Errorf("Sent %d articles, want 1 (others filtered as recent)", len(bot.sentArticles))
	}

	if len(bot.sentArticles) > 0 && bot.sentArticles[0].hnID != 3 {
		t.Errorf("Article ID = %d, want 3", bot.sentArticles[0].hnID)
	}
}

func TestRun_HandlesScrapeFailure(t *testing.T) {
	hnClient := &mockHNClient{
		topStories: []int{1},
		items: map[int]*mockItem{
			1: {ID: 1, Title: "Article 1", URL: "https://example.com/1", Score: 100, Descendants: 10},
		},
	}

	// Scraper fails
	scraper := &mockScraper{err: errors.New("scrape failed")}

	// Should still process with title as fallback
	summarizer := &mockSummarizer{summary: &mockSummary{Summary: "Summary", Tags: []string{"test"}}}
	ranker := &mockRanker{ranked: []RankedArticle{{Article: RankerArticle{ID: 1}}}}
	storage := &mockStorage{
		recentArticleIDs: []int{},
		tagWeights:       map[string]float64{},
		chatID:           "12345",
		articleCount:     "10",
	}
	bot := &mockBot{}

	workflow := &Workflow{
		hnClient:      hnClient,
		scraper:       scraper,
		summarizer:    summarizer,
		ranker:        ranker,
		storage:       storage,
		bot:           bot,
		decayRate:     0.02,
		minWeight:     0.1,
		bufferFactor:  2.0,
	}

	workflow.Run()

	// Should still send the article
	if len(bot.sentArticles) != 1 {
		t.Errorf("Sent %d articles, want 1 (should use title fallback)", len(bot.sentArticles))
	}
}

func TestRun_SkipsSummarizeFailure(t *testing.T) {
	hnClient := &mockHNClient{
		topStories: []int{1, 2},
		items: map[int]*mockItem{
			1: {ID: 1, Title: "Article 1", URL: "https://example.com/1", Score: 100, Descendants: 10},
			2: {ID: 2, Title: "Article 2", URL: "https://example.com/2", Score: 200, Descendants: 20},
		},
	}

	scraper := &mockScraper{content: "Content"}
	// Summarizer fails
	summarizer := &mockSummarizer{err: errors.New("summarize failed")}
	ranker := &mockRanker{ranked: []RankedArticle{}} // No articles to rank since summarization failed
	storage := &mockStorage{
		recentArticleIDs: []int{},
		tagWeights:       map[string]float64{},
		chatID:           "12345",
		articleCount:     "10",
	}
	bot := &mockBot{}

	workflow := &Workflow{
		hnClient:      hnClient,
		scraper:       scraper,
		summarizer:    summarizer,
		ranker:        ranker,
		storage:       storage,
		bot:           bot,
		decayRate:     0.02,
		minWeight:     0.1,
		bufferFactor:  2.0,
	}

	workflow.Run()

	// Should not send any articles since summarization failed
	if len(bot.sentArticles) != 0 {
		t.Errorf("Sent %d articles, want 0 (summarization failed)", len(bot.sentArticles))
	}
}
