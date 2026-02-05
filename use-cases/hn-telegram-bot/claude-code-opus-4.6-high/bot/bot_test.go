package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// --- Mock implementations ---

type mockSender struct {
	lastChatID int64
	lastText   string
	msgID      int
	err        error
}

func (m *mockSender) SendHTML(chatID int64, text string) (int, error) {
	m.lastChatID = chatID
	m.lastText = text
	m.msgID++
	return m.msgID, m.err
}

type mockArticleLookup struct {
	articles map[int]*StoredArticle // keyed by telegram msg id
}

func (m *mockArticleLookup) GetArticleBySentMsgID(msgID int) (*StoredArticle, error) {
	a, ok := m.articles[msgID]
	if !ok {
		return nil, nil
	}
	return a, nil
}

type mockLikeTracker struct {
	liked map[int]bool
}

func (m *mockLikeTracker) IsLiked(articleID int) (bool, error) {
	return m.liked[articleID], nil
}

func (m *mockLikeTracker) RecordLike(articleID int) error {
	m.liked[articleID] = true
	return nil
}

type mockTagBooster struct {
	weights map[string]*TagWeightInfo
}

func (m *mockTagBooster) GetTagWeight(tag string) (*TagWeightInfo, error) {
	tw, ok := m.weights[tag]
	if !ok {
		return nil, nil
	}
	return tw, nil
}

func (m *mockTagBooster) UpsertTagWeight(tag string, weight float64, count int) error {
	m.weights[tag] = &TagWeightInfo{Tag: tag, Weight: weight, Count: count}
	return nil
}

type mockSettingsStore struct {
	settings map[string]string
}

func (m *mockSettingsStore) GetSetting(key string) (string, error) {
	return m.settings[key], nil
}

func (m *mockSettingsStore) SetSetting(key, value string) error {
	m.settings[key] = value
	return nil
}

type mockStatsProvider struct {
	likeCount int
	topTags   []TagWeightInfo
}

func (m *mockStatsProvider) GetTopTagWeights(limit int) ([]TagWeightInfo, error) {
	if limit > len(m.topTags) {
		return m.topTags, nil
	}
	return m.topTags[:limit], nil
}

func (m *mockStatsProvider) GetLikeCount() (int, error) {
	return m.likeCount, nil
}

type mockScheduleUpdater struct {
	lastTime string
}

func (m *mockScheduleUpdater) Schedule(digestTime string, task func()) error {
	m.lastTime = digestTime
	return nil
}

// --- Tests ---

func newTestBot(sender *mockSender) *Bot {
	return New(Config{
		Token:          "test-token",
		ChatID:         0,
		DigestTime:     "09:00",
		ArticleCount:   30,
		TagBoostAmount: 0.2,
	}, Deps{
		Sender:        sender,
		SettingsStore: &mockSettingsStore{settings: map[string]string{}},
	})
}

func TestHandleStart(t *testing.T) {
	sender := &mockSender{}
	settings := &mockSettingsStore{settings: map[string]string{}}
	b := New(Config{
		Token:        "test",
		DigestTime:   "09:00",
		ArticleCount: 30,
	}, Deps{
		Sender:        sender,
		SettingsStore: settings,
	})

	b.handleStart(12345)

	if b.GetChatID() != 12345 {
		t.Errorf("expected chat ID 12345, got %d", b.GetChatID())
	}
	if sender.lastChatID != 12345 {
		t.Errorf("expected message sent to 12345, got %d", sender.lastChatID)
	}
	if settings.settings["chat_id"] != "12345" {
		t.Errorf("expected chat_id saved, got %s", settings.settings["chat_id"])
	}
}

func TestHandleSettings_Display(t *testing.T) {
	sender := &mockSender{}
	b := New(Config{
		Token:        "test",
		DigestTime:   "14:30",
		ArticleCount: 20,
	}, Deps{Sender: sender})

	b.handleSettings(100, nil)

	if sender.lastText == "" {
		t.Fatal("expected settings message")
	}
	if sender.lastChatID != 100 {
		t.Errorf("expected chat ID 100, got %d", sender.lastChatID)
	}
}

func TestHandleSettings_UpdateTime(t *testing.T) {
	sender := &mockSender{}
	settings := &mockSettingsStore{settings: map[string]string{}}
	sched := &mockScheduleUpdater{}
	digestCalled := false

	b := New(Config{
		Token:        "test",
		DigestTime:   "09:00",
		ArticleCount: 30,
	}, Deps{
		Sender:          sender,
		SettingsStore:   settings,
		ScheduleUpdater: sched,
		DigestFunc:      func() { digestCalled = true },
	})

	b.handleSettings(100, []string{"time", "18:30"})

	if b.GetDigestTime() != "18:30" {
		t.Errorf("expected digest time 18:30, got %s", b.GetDigestTime())
	}
	if settings.settings["digest_time"] != "18:30" {
		t.Errorf("expected digest_time saved")
	}
	if sched.lastTime != "18:30" {
		t.Errorf("expected schedule updated to 18:30")
	}
	_ = digestCalled
}

func TestHandleSettings_UpdateCount(t *testing.T) {
	sender := &mockSender{}
	settings := &mockSettingsStore{settings: map[string]string{}}

	b := New(Config{
		Token:        "test",
		DigestTime:   "09:00",
		ArticleCount: 30,
	}, Deps{
		Sender:        sender,
		SettingsStore: settings,
	})

	b.handleSettings(100, []string{"count", "15"})

	if b.GetArticleCount() != 15 {
		t.Errorf("expected article count 15, got %d", b.GetArticleCount())
	}
	if settings.settings["article_count"] != "15" {
		t.Errorf("expected article_count saved")
	}
}

func TestHandleSettings_InvalidCount(t *testing.T) {
	sender := &mockSender{}
	b := newTestBot(sender)

	b.handleSettings(100, []string{"count", "0"})
	if sender.lastText == "" {
		t.Fatal("expected usage message for count=0")
	}

	b.handleSettings(100, []string{"count", "101"})
	b.handleSettings(100, []string{"count", "abc"})
}

func TestHandleSettings_InvalidTime(t *testing.T) {
	sender := &mockSender{}
	b := newTestBot(sender)

	b.handleSettings(100, []string{"time", "25:00"})
	b.handleSettings(100, []string{"time", "abc"})
}

func TestHandleSettings_Usage(t *testing.T) {
	sender := &mockSender{}
	b := newTestBot(sender)

	b.handleSettings(100, []string{"invalid"})
	if sender.lastText == "" {
		t.Fatal("expected usage message")
	}
}

func TestHandleStats_NoLikes(t *testing.T) {
	sender := &mockSender{}
	stats := &mockStatsProvider{likeCount: 0}

	b := New(Config{Token: "test", DigestTime: "09:00", ArticleCount: 30}, Deps{
		Sender:        sender,
		StatsProvider: stats,
	})

	b.handleStats(100)

	if sender.lastText == "" {
		t.Fatal("expected no-likes message")
	}
}

func TestHandleStats_WithLikes(t *testing.T) {
	sender := &mockSender{}
	stats := &mockStatsProvider{
		likeCount: 5,
		topTags: []TagWeightInfo{
			{Tag: "go", Weight: 2.5, Count: 3},
			{Tag: "rust", Weight: 1.8, Count: 2},
		},
	}

	b := New(Config{Token: "test", DigestTime: "09:00", ArticleCount: 30}, Deps{
		Sender:        sender,
		StatsProvider: stats,
	})

	b.handleStats(100)

	if sender.lastText == "" {
		t.Fatal("expected stats message")
	}
}

func TestHandleFetch(t *testing.T) {
	sender := &mockSender{}
	called := false
	b := New(Config{Token: "test", DigestTime: "09:00", ArticleCount: 30}, Deps{
		Sender:     sender,
		DigestFunc: func() { called = true },
	})

	b.handleFetch(100)

	if !called {
		t.Error("expected digest function to be called")
	}
}

func TestHandleReaction_ThumbsUp(t *testing.T) {
	tagsJSON, _ := json.Marshal([]string{"go", "testing"})
	lookup := &mockArticleLookup{
		articles: map[int]*StoredArticle{
			42: {ID: 100, Tags: string(tagsJSON)},
		},
	}
	likeTracker := &mockLikeTracker{liked: map[int]bool{}}
	tagBooster := &mockTagBooster{weights: map[string]*TagWeightInfo{
		"go": {Tag: "go", Weight: 1.5, Count: 2},
	}}

	b := New(Config{Token: "test", DigestTime: "09:00", ArticleCount: 30, TagBoostAmount: 0.2}, Deps{
		Sender:        &mockSender{},
		ArticleLookup: lookup,
		LikeTracker:   likeTracker,
		TagBooster:    tagBooster,
	})

	b.handleReaction(&struct {
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		MessageID    int `json:"message_id"`
		NewReactions []struct {
			Type  string `json:"type"`
			Emoji string `json:"emoji"`
		} `json:"new_reaction"`
	}{
		MessageID: 42,
		NewReactions: []struct {
			Type  string `json:"type"`
			Emoji string `json:"emoji"`
		}{{Type: "emoji", Emoji: "👍"}},
	})

	if !likeTracker.liked[100] {
		t.Error("expected article 100 to be liked")
	}
	if tagBooster.weights["go"].Weight != 1.7 {
		t.Errorf("expected go weight 1.7, got %f", tagBooster.weights["go"].Weight)
	}
	if tagBooster.weights["testing"] == nil {
		t.Fatal("expected testing tag to be created")
	}
	if tagBooster.weights["testing"].Weight != 1.2 {
		t.Errorf("expected testing weight 1.2, got %f", tagBooster.weights["testing"].Weight)
	}
}

func TestHandleReaction_NotThumbsUp(t *testing.T) {
	likeTracker := &mockLikeTracker{liked: map[int]bool{}}

	b := New(Config{Token: "test", DigestTime: "09:00", ArticleCount: 30}, Deps{
		Sender:        &mockSender{},
		ArticleLookup: &mockArticleLookup{articles: map[int]*StoredArticle{}},
		LikeTracker:   likeTracker,
		TagBooster:    &mockTagBooster{weights: map[string]*TagWeightInfo{}},
	})

	b.handleReaction(&struct {
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		MessageID    int `json:"message_id"`
		NewReactions []struct {
			Type  string `json:"type"`
			Emoji string `json:"emoji"`
		} `json:"new_reaction"`
	}{
		MessageID: 42,
		NewReactions: []struct {
			Type  string `json:"type"`
			Emoji string `json:"emoji"`
		}{{Type: "emoji", Emoji: "❤️"}},
	})

	if len(likeTracker.liked) != 0 {
		t.Error("expected no likes for non-thumbsup reaction")
	}
}

func TestHandleReaction_AlreadyLiked(t *testing.T) {
	tagsJSON, _ := json.Marshal([]string{"go"})
	lookup := &mockArticleLookup{
		articles: map[int]*StoredArticle{42: {ID: 100, Tags: string(tagsJSON)}},
	}
	tagBooster := &mockTagBooster{weights: map[string]*TagWeightInfo{
		"go": {Tag: "go", Weight: 1.5, Count: 1},
	}}

	b := New(Config{Token: "test", DigestTime: "09:00", ArticleCount: 30, TagBoostAmount: 0.2}, Deps{
		Sender:        &mockSender{},
		ArticleLookup: lookup,
		LikeTracker:   &mockLikeTracker{liked: map[int]bool{100: true}},
		TagBooster:    tagBooster,
	})

	b.handleReaction(&struct {
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		MessageID    int `json:"message_id"`
		NewReactions []struct {
			Type  string `json:"type"`
			Emoji string `json:"emoji"`
		} `json:"new_reaction"`
	}{
		MessageID: 42,
		NewReactions: []struct {
			Type  string `json:"type"`
			Emoji string `json:"emoji"`
		}{{Type: "emoji", Emoji: "👍"}},
	})

	// Weight should NOT have changed
	if tagBooster.weights["go"].Weight != 1.5 {
		t.Errorf("expected weight unchanged at 1.5, got %f", tagBooster.weights["go"].Weight)
	}
}

func TestHandleReaction_ArticleNotFound(t *testing.T) {
	lookup := &mockArticleLookup{articles: map[int]*StoredArticle{}}
	likeTracker := &mockLikeTracker{liked: map[int]bool{}}

	b := New(Config{Token: "test", DigestTime: "09:00", ArticleCount: 30}, Deps{
		Sender:        &mockSender{},
		ArticleLookup: lookup,
		LikeTracker:   likeTracker,
		TagBooster:    &mockTagBooster{weights: map[string]*TagWeightInfo{}},
	})

	b.handleReaction(&struct {
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		MessageID    int `json:"message_id"`
		NewReactions []struct {
			Type  string `json:"type"`
			Emoji string `json:"emoji"`
		} `json:"new_reaction"`
	}{
		MessageID: 999,
		NewReactions: []struct {
			Type  string `json:"type"`
			Emoji string `json:"emoji"`
		}{{Type: "emoji", Emoji: "👍"}},
	})

	if len(likeTracker.liked) != 0 {
		t.Error("should not record like for unknown article")
	}
}

func TestSendHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := `{"ok":true,"result":{"message_id":42}}`
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, resp)
	}))
	defer srv.Close()

	b := New(Config{Token: "test", BaseURL: srv.URL, DigestTime: "09:00", ArticleCount: 30}, Deps{})

	msgID, err := b.SendHTML(100, "test message")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msgID != 42 {
		t.Errorf("expected message ID 42, got %d", msgID)
	}
}

func TestSendHTML_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"ok":false}`)
	}))
	defer srv.Close()

	b := New(Config{Token: "test", BaseURL: srv.URL, DigestTime: "09:00", ArticleCount: 30}, Deps{})

	_, err := b.SendHTML(100, "test")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetUpdates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := `{
			"ok": true,
			"result": [
				{"update_id": 1, "message": {"message_id": 10, "chat": {"id": 100}, "text": "/start"}},
				{"update_id": 2, "message_reaction": {"chat": {"id": 100}, "message_id": 5, "new_reaction": [{"type": "emoji", "emoji": "👍"}]}}
			]
		}`
		fmt.Fprint(w, resp)
	}))
	defer srv.Close()

	b := New(Config{Token: "test", BaseURL: srv.URL, DigestTime: "09:00", ArticleCount: 30}, Deps{})

	updates, offset, err := b.getUpdates(context.Background(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updates) != 2 {
		t.Errorf("expected 2 updates, got %d", len(updates))
	}
	if offset != 3 {
		t.Errorf("expected offset 3, got %d", offset)
	}
}

func TestHandleMessage_EmptyText(t *testing.T) {
	sender := &mockSender{}
	b := newTestBot(sender)
	// Should not panic
	b.handleMessage(100, "")
	b.handleMessage(100, "   ")
}

func TestHandleMessage_UnknownCommand(t *testing.T) {
	sender := &mockSender{}
	b := newTestBot(sender)
	b.handleMessage(100, "/unknown")
	// Should not send anything or panic
}

func TestHandleUpdate_Message(t *testing.T) {
	sender := &mockSender{}
	settings := &mockSettingsStore{settings: map[string]string{}}
	b := New(Config{Token: "test", DigestTime: "09:00", ArticleCount: 30}, Deps{
		Sender:        sender,
		SettingsStore: settings,
	})

	b.handleUpdate(telegramUpdate{
		UpdateID: 1,
		Message: &struct {
			MessageID int `json:"message_id"`
			Chat      struct {
				ID int64 `json:"id"`
			} `json:"chat"`
			Text string `json:"text"`
			From *struct {
				ID int64 `json:"id"`
			} `json:"from"`
		}{
			MessageID: 1,
			Chat:      struct{ ID int64 `json:"id"` }{ID: 100},
			Text:      "/start",
		},
	})

	if b.GetChatID() != 100 {
		t.Errorf("expected chat ID 100, got %d", b.GetChatID())
	}
}

func TestHandleUpdate_Reaction(t *testing.T) {
	tagsJSON, _ := json.Marshal([]string{"go"})
	lookup := &mockArticleLookup{
		articles: map[int]*StoredArticle{42: {ID: 100, Tags: string(tagsJSON)}},
	}
	likeTracker := &mockLikeTracker{liked: map[int]bool{}}
	tagBooster := &mockTagBooster{weights: map[string]*TagWeightInfo{}}

	b := New(Config{Token: "test", DigestTime: "09:00", ArticleCount: 30, TagBoostAmount: 0.2}, Deps{
		Sender:        &mockSender{},
		ArticleLookup: lookup,
		LikeTracker:   likeTracker,
		TagBooster:    tagBooster,
	})

	b.handleUpdate(telegramUpdate{
		UpdateID: 2,
		MessageReaction: &struct {
			Chat struct {
				ID int64 `json:"id"`
			} `json:"chat"`
			MessageID    int `json:"message_id"`
			NewReactions []struct {
				Type  string `json:"type"`
				Emoji string `json:"emoji"`
			} `json:"new_reaction"`
		}{
			Chat:      struct{ ID int64 `json:"id"` }{ID: 100},
			MessageID: 42,
			NewReactions: []struct {
				Type  string `json:"type"`
				Emoji string `json:"emoji"`
			}{{Type: "emoji", Emoji: "👍"}},
		},
	})

	if !likeTracker.liked[100] {
		t.Error("expected article 100 to be liked")
	}
}

func TestRun_ContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"ok":true,"result":[]}`)
	}))
	defer srv.Close()

	b := New(Config{Token: "test", BaseURL: srv.URL, DigestTime: "09:00", ArticleCount: 30}, Deps{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := b.Run(ctx)
	if err != nil {
		t.Errorf("expected nil error on context cancel, got %v", err)
	}
}
