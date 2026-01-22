package bot

import (
	"fmt"
	"strings"
	"testing"
)

// Mock storage
type mockStorage struct {
	chatID        string
	digestTime    string
	articleCount  string
	article       *Article
	liked         bool
	topTags       []TagWeight
	likeCount     int
	tagWeights    map[string]float64
	settings      map[string]string
}

func (m *mockStorage) GetSetting(key string) (string, error) {
	if m.settings != nil {
		return m.settings[key], nil
	}
	switch key {
	case "chat_id":
		return m.chatID, nil
	case "digest_time":
		return m.digestTime, nil
	case "article_count":
		return m.articleCount, nil
	}
	return "", nil
}

func (m *mockStorage) SetSetting(key, value string) error {
	if m.settings == nil {
		m.settings = make(map[string]string)
	}
	m.settings[key] = value
	return nil
}

func (m *mockStorage) GetArticleByTelegramMsgID(msgID int) (Article, error) {
	if m.article != nil {
		return *m.article, nil
	}
	return Article{}, fmt.Errorf("not found")
}

func (m *mockStorage) IsLiked(articleID int) (bool, error) {
	return m.liked, nil
}

func (m *mockStorage) RecordLike(articleID int) error {
	m.liked = true
	return nil
}

func (m *mockStorage) GetTagWeight(tag string) (float64, error) {
	if m.tagWeights == nil {
		return 1.0, nil
	}
	if weight, ok := m.tagWeights[tag]; ok {
		return weight, nil
	}
	return 1.0, nil
}

func (m *mockStorage) SetTagWeight(tag string, weight float64) error {
	if m.tagWeights == nil {
		m.tagWeights = make(map[string]float64)
	}
	m.tagWeights[tag] = weight
	return nil
}

func (m *mockStorage) IncrementTagOccurrence(tag string) error {
	return nil
}

func (m *mockStorage) GetTopTags(limit int) ([]TagWeight, error) {
	return m.topTags, nil
}

func (m *mockStorage) GetLikeCount() (int, error) {
	return m.likeCount, nil
}

// Mock scheduler
type mockScheduler struct {
	scheduled bool
	timeStr   string
}

func (m *mockScheduler) Schedule(timeStr string, callback func()) error {
	m.scheduled = true
	m.timeStr = timeStr
	return nil
}

func (m *mockScheduler) Start() {}
func (m *mockScheduler) Stop()  {}

// Mock digest runner
type mockDigestRunner struct {
	ran bool
}

func (m *mockDigestRunner) Run() {
	m.ran = true
}

func TestHandleStart(t *testing.T) {
	storage := &mockStorage{}
	handler := &CommandHandler{storage: storage}

	response := handler.HandleStart(12345)

	if !strings.Contains(response, "/fetch") {
		t.Error("Start response should mention /fetch command")
	}
	if !strings.Contains(response, "/settings") {
		t.Error("Start response should mention /settings command")
	}
	if !strings.Contains(response, "/stats") {
		t.Error("Start response should mention /stats command")
	}

	// Verify chat_id was saved
	saved, _ := storage.GetSetting("chat_id")
	if saved != "12345" {
		t.Errorf("chat_id = %v, want 12345", saved)
	}
}

func TestHandleSettings_Display(t *testing.T) {
	storage := &mockStorage{
		digestTime:   "09:00",
		articleCount: "30",
	}
	handler := &CommandHandler{storage: storage}

	response := handler.HandleSettings("")

	if !strings.Contains(response, "09:00") {
		t.Error("Settings display should show digest time")
	}
	if !strings.Contains(response, "30") {
		t.Error("Settings display should show article count")
	}
}

func TestHandleSettings_UpdateTime(t *testing.T) {
	storage := &mockStorage{}
	scheduler := &mockScheduler{}
	digestRunner := &mockDigestRunner{}
	handler := &CommandHandler{
		storage:      storage,
		scheduler:    scheduler,
		digestRunner: digestRunner,
	}

	response := handler.HandleSettings("time 14:30")

	if !strings.Contains(response, "14:30") {
		t.Error("Response should confirm time update")
	}

	saved, _ := storage.GetSetting("digest_time")
	if saved != "14:30" {
		t.Errorf("digest_time = %v, want 14:30", saved)
	}

	if !scheduler.scheduled {
		t.Error("Scheduler should have been updated")
	}
}

func TestHandleSettings_UpdateCount(t *testing.T) {
	storage := &mockStorage{}
	handler := &CommandHandler{storage: storage}

	response := handler.HandleSettings("count 50")

	if !strings.Contains(response, "50") {
		t.Error("Response should confirm count update")
	}

	saved, _ := storage.GetSetting("article_count")
	if saved != "50" {
		t.Errorf("article_count = %v, want 50", saved)
	}
}

func TestHandleSettings_InvalidTime(t *testing.T) {
	storage := &mockStorage{}
	handler := &CommandHandler{storage: storage}

	response := handler.HandleSettings("time 25:00")

	if !strings.Contains(response, "Usage") && !strings.Contains(response, "invalid") {
		t.Error("Response should indicate error for invalid time")
	}
}

func TestHandleSettings_InvalidCount(t *testing.T) {
	storage := &mockStorage{}
	handler := &CommandHandler{storage: storage}

	tests := []string{
		"count 0",
		"count 101",
		"count abc",
	}

	for _, arg := range tests {
		response := handler.HandleSettings(arg)
		if !strings.Contains(response, "Usage") && !strings.Contains(response, "invalid") && !strings.Contains(response, "between") {
			t.Errorf("Response should indicate error for %s", arg)
		}
	}
}

func TestHandleStats_WithData(t *testing.T) {
	storage := &mockStorage{
		topTags: []TagWeight{
			{Tag: "golang", Weight: 2.5},
			{Tag: "rust", Weight: 2.0},
		},
		likeCount: 15,
	}
	handler := &CommandHandler{storage: storage}

	response := handler.HandleStats()

	if !strings.Contains(response, "golang") {
		t.Error("Stats should show top tag")
	}
	if !strings.Contains(response, "2.5") {
		t.Error("Stats should show tag weight")
	}
	if !strings.Contains(response, "15") {
		t.Error("Stats should show like count")
	}
}

func TestHandleStats_NoData(t *testing.T) {
	storage := &mockStorage{
		topTags:   []TagWeight{},
		likeCount: 0,
	}
	handler := &CommandHandler{storage: storage}

	response := handler.HandleStats()

	if !strings.Contains(response, "👍") && !strings.Contains(response, "React") {
		t.Errorf("Stats with no data should encourage reactions, got: %s", response)
	}
}

func TestHandleReaction_ThumbsUp(t *testing.T) {
	storage := &mockStorage{
		article: &Article{
			ID:   12345,
			Tags: []string{"golang", "testing"},
		},
		liked:      false,
		tagWeights: make(map[string]float64),
	}
	handler := &CommandHandler{
		storage:        storage,
		tagBoostAmount: 0.2,
	}

	handler.HandleReaction(999, "👍")

	// Verify article was liked
	if !storage.liked {
		t.Error("Article should be marked as liked")
	}

	// Verify tags were boosted
	golangWeight, _ := storage.GetTagWeight("golang")
	if golangWeight != 1.2 { // 1.0 default + 0.2 boost
		t.Errorf("golang weight = %v, want 1.2", golangWeight)
	}
}

func TestHandleReaction_AlreadyLiked(t *testing.T) {
	storage := &mockStorage{
		article: &Article{
			ID:   12345,
			Tags: []string{"test"},
		},
		liked:      true,
		tagWeights: make(map[string]float64),
	}
	handler := &CommandHandler{
		storage:        storage,
		tagBoostAmount: 0.2,
	}

	handler.HandleReaction(999, "👍")

	// Tags should not be boosted again
	testWeight, _ := storage.GetTagWeight("test")
	if testWeight != 1.0 {
		t.Errorf("test weight = %v, want 1.0 (should not boost if already liked)", testWeight)
	}
}

func TestHandleReaction_NonThumbsUp(t *testing.T) {
	storage := &mockStorage{
		article: &Article{
			ID:   12345,
			Tags: []string{"test"},
		},
	}
	handler := &CommandHandler{storage: storage}

	// Should be ignored
	handler.HandleReaction(999, "❤️")
	handler.HandleReaction(999, "🔥")

	if storage.liked {
		t.Error("Non-thumbs-up reactions should be ignored")
	}
}

func TestFormatArticleMessage(t *testing.T) {
	message := formatArticleMessage("Test & Title", "https://example.com", "Summary with <html>", 100, 50, 12345)

	if !strings.Contains(message, "Test &amp; Title") {
		t.Error("Title should be HTML escaped")
	}
	if !strings.Contains(message, "Summary with &lt;html&gt;") {
		t.Error("Summary should be HTML escaped")
	}
	if !strings.Contains(message, "https://example.com") {
		t.Error("Article URL should be included")
	}
	if !strings.Contains(message, "https://news.ycombinator.com/item?id=12345") {
		t.Error("HN discussion URL should be included")
	}
	if !strings.Contains(message, "100") {
		t.Error("Score should be included")
	}
	if !strings.Contains(message, "50") {
		t.Error("Comment count should be included")
	}
}

func TestEscapeHTML(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"normal text", "normal text"},
		{"<tag>", "&lt;tag&gt;"},
		{"A & B", "A &amp; B"},
		{"A < B > C", "A &lt; B &gt; C"},
		{"Mixed <b>bold</b> & test", "Mixed &lt;b&gt;bold&lt;/b&gt; &amp; test"},
	}

	for _, tt := range tests {
		got := escapeHTML(tt.input)
		if got != tt.want {
			t.Errorf("escapeHTML(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
