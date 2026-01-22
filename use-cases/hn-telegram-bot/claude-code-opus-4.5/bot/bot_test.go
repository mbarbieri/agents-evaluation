package bot

import (
	"context"
	"strings"
	"testing"
)

// Mock implementations for testing

type mockMessageSender struct {
	sentMessages []sentMessage
}

type sentMessage struct {
	chatID int64
	text   string
	html   bool
}

func (m *mockMessageSender) SendMessage(ctx context.Context, chatID int64, text string, html bool) (int64, error) {
	m.sentMessages = append(m.sentMessages, sentMessage{chatID, text, html})
	return int64(len(m.sentMessages)), nil
}

type mockSettingsStore struct {
	settings map[string]string
}

func newMockSettingsStore() *mockSettingsStore {
	return &mockSettingsStore{settings: make(map[string]string)}
}

func (m *mockSettingsStore) GetSetting(ctx context.Context, key string) (string, error) {
	if v, ok := m.settings[key]; ok {
		return v, nil
	}
	return "", ErrSettingNotFound
}

func (m *mockSettingsStore) SetSetting(ctx context.Context, key, value string) error {
	m.settings[key] = value
	return nil
}

type mockArticleLookup struct {
	articles map[int64]*ArticleInfo
}

func newMockArticleLookup() *mockArticleLookup {
	return &mockArticleLookup{articles: make(map[int64]*ArticleInfo)}
}

func (m *mockArticleLookup) GetArticleByMessageID(ctx context.Context, msgID int64) (*ArticleInfo, error) {
	if a, ok := m.articles[msgID]; ok {
		return a, nil
	}
	return nil, ErrArticleNotFound
}

type mockLikeTracker struct {
	liked map[int64]bool
}

func newMockLikeTracker() *mockLikeTracker {
	return &mockLikeTracker{liked: make(map[int64]bool)}
}

func (m *mockLikeTracker) IsArticleLiked(ctx context.Context, articleID int64) (bool, error) {
	return m.liked[articleID], nil
}

func (m *mockLikeTracker) LikeArticle(ctx context.Context, articleID int64) error {
	m.liked[articleID] = true
	return nil
}

func (m *mockLikeTracker) GetLikeCount(ctx context.Context) (int, error) {
	return len(m.liked), nil
}

type mockTagBooster struct {
	boosted map[string]float64
}

func newMockTagBooster() *mockTagBooster {
	return &mockTagBooster{boosted: make(map[string]float64)}
}

func (m *mockTagBooster) BoostTagWeight(ctx context.Context, tag string, boost float64) error {
	m.boosted[tag] += boost
	return nil
}

type mockTagStats struct {
	topTags []TagStat
}

func (m *mockTagStats) GetTopTags(ctx context.Context, limit int) ([]TagStat, error) {
	if limit > len(m.topTags) {
		return m.topTags, nil
	}
	return m.topTags[:limit], nil
}

type mockScheduleUpdater struct {
	scheduledTime string
}

func (m *mockScheduleUpdater) Schedule(timeStr string, fn func()) error {
	m.scheduledTime = timeStr
	return nil
}

type mockDigestTrigger struct {
	triggered bool
}

func (m *mockDigestTrigger) TriggerDigest(ctx context.Context) error {
	m.triggered = true
	return nil
}

// Tests

func TestHandleStartCommand(t *testing.T) {
	sender := &mockMessageSender{}
	settings := newMockSettingsStore()

	handler := NewCommandHandler(sender, settings, nil, nil, nil)
	ctx := context.Background()

	err := handler.HandleStart(ctx, 12345)
	if err != nil {
		t.Fatalf("HandleStart failed: %v", err)
	}

	// Should save chat_id
	chatID, err := settings.GetSetting(ctx, "chat_id")
	if err != nil {
		t.Fatalf("chat_id not saved: %v", err)
	}
	if chatID != "12345" {
		t.Errorf("chat_id = %q, want '12345'", chatID)
	}

	// Should send welcome message
	if len(sender.sentMessages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(sender.sentMessages))
	}
	if sender.sentMessages[0].chatID != 12345 {
		t.Errorf("message sent to wrong chat: %d", sender.sentMessages[0].chatID)
	}
}

func TestHandleSettingsCommandDisplay(t *testing.T) {
	sender := &mockMessageSender{}
	settings := newMockSettingsStore()
	settings.settings["digest_time"] = "09:00"
	settings.settings["article_count"] = "30"

	handler := NewCommandHandler(sender, settings, nil, nil, nil)
	ctx := context.Background()

	err := handler.HandleSettings(ctx, 12345, "")
	if err != nil {
		t.Fatalf("HandleSettings failed: %v", err)
	}

	if len(sender.sentMessages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(sender.sentMessages))
	}

	msg := sender.sentMessages[0].text
	if !contains(msg, "09:00") || !contains(msg, "30") {
		t.Errorf("settings message should contain current values, got: %s", msg)
	}
}

func TestHandleSettingsCommandUpdateTime(t *testing.T) {
	sender := &mockMessageSender{}
	settings := newMockSettingsStore()
	schedUpdater := &mockScheduleUpdater{}

	handler := NewCommandHandler(sender, settings, schedUpdater, nil, nil)
	ctx := context.Background()

	err := handler.HandleSettings(ctx, 12345, "time 18:30")
	if err != nil {
		t.Fatalf("HandleSettings failed: %v", err)
	}

	// Should update setting
	newTime, _ := settings.GetSetting(ctx, "digest_time")
	if newTime != "18:30" {
		t.Errorf("digest_time = %q, want '18:30'", newTime)
	}

	// Should update scheduler
	if schedUpdater.scheduledTime != "18:30" {
		t.Errorf("scheduler not updated with new time")
	}
}

func TestHandleSettingsCommandUpdateCount(t *testing.T) {
	sender := &mockMessageSender{}
	settings := newMockSettingsStore()

	handler := NewCommandHandler(sender, settings, nil, nil, nil)
	ctx := context.Background()

	err := handler.HandleSettings(ctx, 12345, "count 50")
	if err != nil {
		t.Fatalf("HandleSettings failed: %v", err)
	}

	newCount, _ := settings.GetSetting(ctx, "article_count")
	if newCount != "50" {
		t.Errorf("article_count = %q, want '50'", newCount)
	}
}

func TestHandleSettingsCommandInvalidCount(t *testing.T) {
	sender := &mockMessageSender{}
	settings := newMockSettingsStore()

	handler := NewCommandHandler(sender, settings, nil, nil, nil)
	ctx := context.Background()

	// Count out of range
	handler.HandleSettings(ctx, 12345, "count 200")

	// Should send error message, not crash
	if len(sender.sentMessages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(sender.sentMessages))
	}
}

func TestHandleSettingsCommandInvalidTime(t *testing.T) {
	sender := &mockMessageSender{}
	settings := newMockSettingsStore()

	handler := NewCommandHandler(sender, settings, nil, nil, nil)
	ctx := context.Background()

	handler.HandleSettings(ctx, 12345, "time 25:00")

	if len(sender.sentMessages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(sender.sentMessages))
	}
}

func TestHandleStatsCommand(t *testing.T) {
	sender := &mockMessageSender{}
	likeTracker := newMockLikeTracker()
	likeTracker.liked[1] = true
	likeTracker.liked[2] = true

	tagStats := &mockTagStats{
		topTags: []TagStat{
			{Tag: "go", Weight: 2.5},
			{Tag: "rust", Weight: 1.8},
		},
	}

	handler := NewCommandHandler(sender, nil, nil, likeTracker, tagStats)
	ctx := context.Background()

	err := handler.HandleStats(ctx, 12345)
	if err != nil {
		t.Fatalf("HandleStats failed: %v", err)
	}

	if len(sender.sentMessages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(sender.sentMessages))
	}

	msg := sender.sentMessages[0].text
	if !contains(msg, "go") || !contains(msg, "rust") {
		t.Errorf("stats should contain tag names, got: %s", msg)
	}
}

func TestHandleStatsCommandNoLikes(t *testing.T) {
	sender := &mockMessageSender{}
	likeTracker := newMockLikeTracker()
	tagStats := &mockTagStats{topTags: nil}

	handler := NewCommandHandler(sender, nil, nil, likeTracker, tagStats)
	ctx := context.Background()

	handler.HandleStats(ctx, 12345)

	msg := sender.sentMessages[0].text
	// Message should mention likes or thumbs-up emoji
	if !strings.Contains(msg, "👍") && !strings.Contains(msg, "like") {
		t.Errorf("no-likes message should mention liking, got: %s", msg)
	}
}

func TestHandleFetchCommand(t *testing.T) {
	sender := &mockMessageSender{}
	digestTrigger := &mockDigestTrigger{}

	handler := NewCommandHandler(sender, nil, nil, nil, nil)
	handler.digestTrigger = digestTrigger
	ctx := context.Background()

	err := handler.HandleFetch(ctx, 12345)
	if err != nil {
		t.Fatalf("HandleFetch failed: %v", err)
	}

	if !digestTrigger.triggered {
		t.Error("digest was not triggered")
	}
}

func TestHandleReaction(t *testing.T) {
	articleLookup := newMockArticleLookup()
	articleLookup.articles[100] = &ArticleInfo{
		ID:   12345,
		Tags: []string{"go", "testing"},
	}

	likeTracker := newMockLikeTracker()
	tagBooster := newMockTagBooster()

	handler := NewReactionHandler(articleLookup, likeTracker, tagBooster, 0.2)
	ctx := context.Background()

	err := handler.HandleReaction(ctx, 100, "👍")
	if err != nil {
		t.Fatalf("HandleReaction failed: %v", err)
	}

	// Should record like
	if !likeTracker.liked[12345] {
		t.Error("article should be liked")
	}

	// Should boost tags
	if tagBooster.boosted["go"] != 0.2 {
		t.Errorf("go boost = %f, want 0.2", tagBooster.boosted["go"])
	}
	if tagBooster.boosted["testing"] != 0.2 {
		t.Errorf("testing boost = %f, want 0.2", tagBooster.boosted["testing"])
	}
}

func TestHandleReactionNonThumbsUp(t *testing.T) {
	handler := NewReactionHandler(nil, nil, nil, 0.2)
	ctx := context.Background()

	// Non-thumbs-up reactions should be ignored (no error)
	err := handler.HandleReaction(ctx, 100, "❤️")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = handler.HandleReaction(ctx, 100, "🎉")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleReactionUnknownMessage(t *testing.T) {
	articleLookup := newMockArticleLookup() // Empty

	handler := NewReactionHandler(articleLookup, nil, nil, 0.2)
	ctx := context.Background()

	// Unknown message ID should be silently ignored
	err := handler.HandleReaction(ctx, 999, "👍")
	if err != nil {
		t.Fatalf("unexpected error for unknown message: %v", err)
	}
}

func TestHandleReactionAlreadyLiked(t *testing.T) {
	articleLookup := newMockArticleLookup()
	articleLookup.articles[100] = &ArticleInfo{
		ID:   12345,
		Tags: []string{"go"},
	}

	likeTracker := newMockLikeTracker()
	likeTracker.liked[12345] = true // Already liked

	tagBooster := newMockTagBooster()

	handler := NewReactionHandler(articleLookup, likeTracker, tagBooster, 0.2)
	ctx := context.Background()

	err := handler.HandleReaction(ctx, 100, "👍")
	if err != nil {
		t.Fatalf("HandleReaction failed: %v", err)
	}

	// Tags should NOT be boosted again (idempotent)
	if tagBooster.boosted["go"] != 0 {
		t.Errorf("tags should not be boosted for already-liked article")
	}
}

func TestFormatArticleMessage(t *testing.T) {
	article := &ArticleForDisplay{
		ID:       12345,
		Title:    "Test <Article> & More",
		Summary:  "This is a test summary with <html> & entities",
		HNScore:  100,
		Comments: 50,
		URL:      "https://example.com/article",
	}

	msg := FormatArticleMessage(article)

	// Should escape HTML
	if contains(msg, "<Article>") {
		t.Error("HTML in title should be escaped")
	}
	if !contains(msg, "&lt;Article&gt;") {
		t.Error("title should have escaped HTML entities")
	}

	// Should contain key elements
	if !contains(msg, "12345") {
		t.Error("message should contain HN item ID in link")
	}
	if !contains(msg, "100") {
		t.Error("message should contain score")
	}
	if !contains(msg, "50") {
		t.Error("message should contain comment count")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
