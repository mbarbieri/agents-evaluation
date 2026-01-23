package bot

import (
	"context"
	"errors"
	"testing"
)

type mockSender struct {
	lastChat int64
	lastText string
	err      error
}

func (m *mockSender) Send(ctx context.Context, chatID int64, text string) error {
	if m.err != nil {
		return m.err
	}
	m.lastChat = chatID
	m.lastText = text
	return nil
}

type mockSettings struct {
	values map[string]string
}

func (m *mockSettings) Get(ctx context.Context, key string) (string, bool, error) {
	value, ok := m.values[key]
	return value, ok, nil
}

func (m *mockSettings) Set(ctx context.Context, key, value string) error {
	if m.values == nil {
		m.values = map[string]string{}
	}
	m.values[key] = value
	return nil
}

type mockScheduler struct {
	updated string
	called  bool
	err     error
}

func (m *mockScheduler) Update(digestTime string) error {
	m.updated = digestTime
	m.called = true
	return m.err
}

type mockStats struct {
	tags  []TagWeight
	likes int
}

func (m *mockStats) TopTags(ctx context.Context, limit int) ([]TagWeight, error) {
	return m.tags, nil
}

func (m *mockStats) LikeCount(ctx context.Context) (int, error) {
	return m.likes, nil
}

func TestStartCommandStoresChatID(t *testing.T) {
	t.Parallel()
	sender := &mockSender{}
	settings := &mockSettings{}
	h := NewHandler(sender, settings, nil, nil)

	if err := h.HandleStart(context.Background(), 123); err != nil {
		t.Fatalf("HandleStart: %v", err)
	}
	if settings.values["chat_id"] != "123" {
		t.Fatalf("expected chat_id to be stored")
	}
	if sender.lastChat != 123 {
		t.Fatalf("expected message to be sent")
	}
}

func TestSettingsCommandUpdatesTime(t *testing.T) {
	t.Parallel()
	sender := &mockSender{}
	settings := &mockSettings{values: map[string]string{"digest_time": "09:00", "article_count": "30"}}
	scheduler := &mockScheduler{}
	h := NewHandler(sender, settings, scheduler, nil)

	if err := h.HandleSettings(context.Background(), 1, []string{"time", "10:00"}); err != nil {
		t.Fatalf("HandleSettings: %v", err)
	}
	if settings.values["digest_time"] != "10:00" {
		t.Fatalf("expected updated digest time")
	}
	if !scheduler.called || scheduler.updated != "10:00" {
		t.Fatalf("expected scheduler update")
	}
}

func TestSettingsCommandInvalidInput(t *testing.T) {
	t.Parallel()
	sender := &mockSender{}
	settings := &mockSettings{values: map[string]string{"digest_time": "09:00", "article_count": "30"}}
	h := NewHandler(sender, settings, nil, nil)

	if err := h.HandleSettings(context.Background(), 1, []string{"count", "500"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestStatsCommandNoLikes(t *testing.T) {
	t.Parallel()
	sender := &mockSender{}
	stats := &mockStats{likes: 0}
	h := NewHandler(sender, &mockSettings{}, nil, stats)

	if err := h.HandleStats(context.Background(), 1); err != nil {
		t.Fatalf("HandleStats: %v", err)
	}
	if sender.lastText == "" {
		t.Fatalf("expected response")
	}
}

func TestSettingsCommandReportsErrors(t *testing.T) {
	t.Parallel()
	sender := &mockSender{}
	settings := &mockSettings{values: map[string]string{"digest_time": "09:00", "article_count": "30"}}
	scheduler := &mockScheduler{err: errors.New("boom")}
	h := NewHandler(sender, settings, scheduler, nil)

	if err := h.HandleSettings(context.Background(), 1, []string{"time", "10:00"}); err == nil {
		t.Fatalf("expected error")
	}
}
