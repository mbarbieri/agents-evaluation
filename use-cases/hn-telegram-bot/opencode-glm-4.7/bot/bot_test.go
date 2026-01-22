package bot

import (
	"errors"
	"testing"

	"hn-bot/storage"
)

type mockMessageSender struct {
	messages []message
}

type message struct {
	chatID int64
	text   string
}

func (m *mockMessageSender) Send(chatID int64, text string) error {
	m.messages = append(m.messages, message{chatID: chatID, text: text})
	return nil
}

type mockSettingsGetter struct {
	settings map[string]string
}

func (m *mockSettingsGetter) GetSetting(key string) (string, error) {
	if val, ok := m.settings[key]; ok {
		return val, nil
	}
	return "", errors.New("not found")
}

type mockSettingsSetter struct {
	settings map[string]string
}

func (m *mockSettingsSetter) SetSetting(key, value string) error {
	if m.settings == nil {
		m.settings = make(map[string]string)
	}
	m.settings[key] = value
	return nil
}

type mockArticleFinder struct {
	articles         map[int64]storage.Article
	messageToArticle map[int]storage.Article
}

func (m *mockArticleFinder) GetArticle(id int64) (storage.Article, error) {
	if article, ok := m.articles[id]; ok {
		return article, nil
	}
	return storage.Article{}, errors.New("not found")
}

func (m *mockArticleFinder) GetArticleByMessageID(messageID int) (storage.Article, error) {
	if article, ok := m.messageToArticle[messageID]; ok {
		return article, nil
	}
	return storage.Article{}, errors.New("not found")
}

type mockLikeRecorder struct {
	likes     map[int64]bool
	likeCount int
}

func (m *mockLikeRecorder) LikeArticle(articleID int64) error {
	if m.likes == nil {
		m.likes = make(map[int64]bool)
	}
	if m.likes[articleID] {
		return nil
	}
	m.likes[articleID] = true
	m.likeCount++
	return nil
}

func (m *mockLikeRecorder) IsArticleLiked(articleID int64) (bool, error) {
	if m.likes == nil {
		return false, nil
	}
	return m.likes[articleID], nil
}

func (m *mockLikeRecorder) GetLikeCount() (int, error) {
	return m.likeCount, nil
}

type mockTagWeightUpdater struct {
	weights map[string]tagWeightInfo
}

type tagWeightInfo struct {
	weight float64
	count  int
}

func (m *mockTagWeightUpdater) GetTagWeight(tag string) (float64, int, error) {
	if info, ok := m.weights[tag]; ok {
		return info.weight, info.count, nil
	}
	return 0, 0, errors.New("not found")
}

func (m *mockTagWeightUpdater) SetTagWeight(tag string, weight float64, count int) error {
	if m.weights == nil {
		m.weights = make(map[string]tagWeightInfo)
	}
	m.weights[tag] = tagWeightInfo{weight: weight, count: count}
	return nil
}

func TestHandleStart(t *testing.T) {
	msgSender := &mockMessageSender{}
	handler, err := NewHandler("test-token", msgSender, &mockSettingsGetter{}, &mockSettingsSetter{}, &mockArticleFinder{}, &mockLikeRecorder{}, &mockTagWeightUpdater{}, 0.2)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	err = handler.HandleStart(12345)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(msgSender.messages) != 1 {
		t.Errorf("got %d messages, want 1", len(msgSender.messages))
	}

	if msgSender.messages[0].chatID != 12345 {
		t.Errorf("chatID = %d, want 12345", msgSender.messages[0].chatID)
	}

	if handler.chatID != 12345 {
		t.Errorf("handler chatID = %d, want 12345", handler.chatID)
	}
}

func TestHandleSettingsView(t *testing.T) {
	settingsGet := &mockSettingsGetter{settings: map[string]string{
		"digest_time":   "10:00",
		"article_count": "25",
	}}
	msgSender := &mockMessageSender{}

	handler, err := NewHandler("test-token", msgSender, settingsGet, &mockSettingsSetter{}, &mockArticleFinder{}, &mockLikeRecorder{}, &mockTagWeightUpdater{}, 0.2)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}
	handler.SetChatID(12345)

	err = handler.HandleSettings(12345, "")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(msgSender.messages) != 1 {
		t.Errorf("got %d messages, want 1", len(msgSender.messages))
	}

	text := msgSender.messages[0].text
	if text == "" {
		t.Error("expected non-empty message")
	}
}

func TestHandleSettingsUpdateTime(t *testing.T) {
	settingsSet := &mockSettingsSetter{}
	msgSender := &mockMessageSender{}

	handler, err := NewHandler("test-token", msgSender, &mockSettingsGetter{}, settingsSet, &mockArticleFinder{}, &mockLikeRecorder{}, &mockTagWeightUpdater{}, 0.2)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}
	handler.SetChatID(12345)

	err = handler.HandleSettings(12345, "time 10:30")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if settingsSet.settings["digest_time"] != "10:30" {
		t.Errorf("digest_time = %s, want 10:30", settingsSet.settings["digest_time"])
	}
}

func TestHandleSettingsUpdateCount(t *testing.T) {
	settingsSet := &mockSettingsSetter{}
	msgSender := &mockMessageSender{}

	handler, err := NewHandler("test-token", msgSender, &mockSettingsGetter{}, settingsSet, &mockArticleFinder{}, &mockLikeRecorder{}, &mockTagWeightUpdater{}, 0.2)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}
	handler.SetChatID(12345)

	err = handler.HandleSettings(12345, "count 50")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if settingsSet.settings["article_count"] != "50" {
		t.Errorf("article_count = %s, want 50", settingsSet.settings["article_count"])
	}
}

func TestHandleSettingsInvalid(t *testing.T) {
	msgSender := &mockMessageSender{}

	handler, err := NewHandler("test-token", msgSender, &mockSettingsGetter{}, &mockSettingsSetter{}, &mockArticleFinder{}, &mockLikeRecorder{}, &mockTagWeightUpdater{}, 0.2)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}
	handler.SetChatID(12345)

	err = handler.HandleSettings(12345, "invalid format")
	if err == nil {
		t.Error("expected error for invalid format, got nil")
	}
}

func TestHandleReactionThumbsUp(t *testing.T) {
	article := storage.Article{
		ID:        123,
		Tags:      []string{"rust", "programming"},
		MessageID: 456,
	}

	articleFind := &mockArticleFinder{
		messageToArticle: map[int]storage.Article{456: article},
	}
	likeRecord := &mockLikeRecorder{}
	tagWeight := &mockTagWeightUpdater{
		weights: map[string]tagWeightInfo{
			"rust":        {weight: 2.0, count: 5},
			"programming": {weight: 1.5, count: 3},
		},
	}

	handler, _ := NewHandler("test-token", &mockMessageSender{}, &mockSettingsGetter{}, &mockSettingsSetter{}, articleFind, likeRecord, tagWeight, 0.2)
	handler.SetChatID(12345)

	err := handler.HandleReaction(456, "👍", 12345)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if likeRecord.likeCount != 1 {
		t.Errorf("like count = %d, want 1", likeRecord.likeCount)
	}

	if tagWeight.weights["rust"].weight != 2.2 {
		t.Errorf("rust weight = %v, want 2.2", tagWeight.weights["rust"].weight)
	}
	if tagWeight.weights["rust"].count != 6 {
		t.Errorf("rust count = %d, want 6", tagWeight.weights["rust"].count)
	}
}

func TestHandleReactionOtherEmoji(t *testing.T) {
	article := storage.Article{
		ID:        123,
		Tags:      []string{"rust"},
		MessageID: 456,
	}

	articleFind := &mockArticleFinder{
		messageToArticle: map[int]storage.Article{456: article},
	}
	likeRecord := &mockLikeRecorder{}

	handler, _ := NewHandler("test-token", &mockMessageSender{}, &mockSettingsGetter{}, &mockSettingsSetter{}, articleFind, likeRecord, &mockTagWeightUpdater{}, 0.2)
	handler.SetChatID(12345)

	err := handler.HandleReaction(456, "❤️", 12345)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if likeRecord.likeCount != 0 {
		t.Errorf("like count = %d, want 0", likeRecord.likeCount)
	}
}

func TestHandleReactionIdempotent(t *testing.T) {
	article := storage.Article{
		ID:        123,
		Tags:      []string{"rust"},
		MessageID: 456,
	}

	articleFind := &mockArticleFinder{
		messageToArticle: map[int]storage.Article{456: article},
	}
	likeRecord := &mockLikeRecorder{likes: map[int64]bool{123: true}}

	handler, _ := NewHandler("test-token", &mockMessageSender{}, &mockSettingsGetter{}, &mockSettingsSetter{}, articleFind, likeRecord, &mockTagWeightUpdater{}, 0.2)
	handler.SetChatID(12345)

	err := handler.HandleReaction(456, "👍", 12345)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if likeRecord.likeCount != 0 {
		t.Errorf("like count = %d, want 0 (already liked)", likeRecord.likeCount)
	}
}

func TestFormatArticleMessage(t *testing.T) {
	message := FormatArticleMessage("Test & Title", "Test <summary>", "https://example.com", 12345, 100, 50)

	if message == "" {
		t.Error("expected non-empty message")
	}

	if !contains(message, "Test &amp; Title") {
		t.Error("title should be HTML escaped")
	}

	if !contains(message, "Test &lt;summary&gt;") {
		t.Error("summary should be HTML escaped")
	}

	if !contains(message, "https://example.com") {
		t.Error("message should contain article URL")
	}

	if !contains(message, "https://news.ycombinator.com/item?id=12345") {
		t.Error("message should contain HN discussion URL")
	}

	if !contains(message, "100") {
		t.Error("message should contain HN score")
	}

	if !contains(message, "50") {
		t.Error("message should contain comment count")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
