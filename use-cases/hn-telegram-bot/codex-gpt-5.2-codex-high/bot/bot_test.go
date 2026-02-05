package bot

import (
	"context"
	"strings"
	"testing"
	"time"

	"hn-telegram-bot/model"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type mockSender struct {
	messages []string
}

func (m *mockSender) SendText(ctx context.Context, chatID int64, text string) (int, error) {
	m.messages = append(m.messages, text)
	return 1, nil
}

func (m *mockSender) SendHTML(ctx context.Context, chatID int64, htmlText string) (int, error) {
	m.messages = append(m.messages, htmlText)
	return 1, nil
}

type mockStorage struct {
	settings map[string]string
	likes    int
	articles map[int]model.Article
	boosted  []string
	liked    []int64
}

func (m *mockStorage) SetSetting(ctx context.Context, key, value string) error {
	if m.settings == nil {
		m.settings = map[string]string{}
	}
	m.settings[key] = value
	return nil
}

func (m *mockStorage) GetSetting(ctx context.Context, key string) (string, bool, error) {
	val, ok := m.settings[key]
	return val, ok, nil
}

func (m *mockStorage) ListTopTags(ctx context.Context, limit int) ([]model.TagWeight, error) {
	return []model.TagWeight{{Tag: "go", Weight: 1.5, Count: 2}}, nil
}

func (m *mockStorage) CountLikes(ctx context.Context) (int, error) {
	return m.likes, nil
}

func (m *mockStorage) GetArticleByMessageID(ctx context.Context, msgID int) (model.Article, bool, error) {
	if m.articles == nil {
		return model.Article{}, false, nil
	}
	article, ok := m.articles[msgID]
	return article, ok, nil
}

func (m *mockStorage) IsLiked(ctx context.Context, articleID int64) (bool, error) {
	for _, id := range m.liked {
		if id == articleID {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockStorage) BoostTags(ctx context.Context, tags []string, boost float64) error {
	m.boosted = append(m.boosted, tags...)
	return nil
}

func (m *mockStorage) AddLike(ctx context.Context, articleID int64, likedAt time.Time) error {
	m.liked = append(m.liked, articleID)
	m.likes++
	return nil
}

var _ Storage = (*mockStorage)(nil)

type mockScheduler struct {
	updated string
}

func (m *mockScheduler) UpdateTime(digestTime string) error {
	m.updated = digestTime
	return nil
}

func commandMessage(chatID int64, text string) *tgbotapi.Message {
	command := text
	if idx := strings.Index(text, " "); idx != -1 {
		command = text[:idx]
	}
	return &tgbotapi.Message{
		Text: text,
		Chat: &tgbotapi.Chat{ID: chatID},
		Entities: []tgbotapi.MessageEntity{{
			Type:   "bot_command",
			Offset: 0,
			Length: len(command),
		}},
	}
}

func TestStartCommand(t *testing.T) {
	sender := &mockSender{}
	storage := &mockStorage{}
	settings := NewSettings(0, "09:00", 30)
	b := &Bot{Sender: sender, Storage: storage, Settings: settings}

	update := Update{Message: commandMessage(123, "/start")}
	b.ProcessUpdate(context.Background(), update)

	if settings.ChatID() != 123 {
		t.Fatalf("expected chat id set")
	}
	if storage.settings["chat_id"] != "123" {
		t.Fatalf("expected chat_id stored")
	}
	if len(sender.messages) == 0 {
		t.Fatalf("expected welcome message")
	}
}

func TestSettingsDisplay(t *testing.T) {
	sender := &mockSender{}
	storage := &mockStorage{}
	settings := NewSettings(123, "09:00", 30)
	b := &Bot{Sender: sender, Storage: storage, Settings: settings}

	update := Update{Message: commandMessage(123, "/settings")}
	b.ProcessUpdate(context.Background(), update)

	if len(sender.messages) == 0 || sender.messages[0] == "" {
		t.Fatalf("expected settings message")
	}
}

func TestSettingsUpdateTime(t *testing.T) {
	sender := &mockSender{}
	storage := &mockStorage{}
	scheduler := &mockScheduler{}
	settings := NewSettings(123, "09:00", 30)
	b := &Bot{Sender: sender, Storage: storage, Settings: settings, Scheduler: scheduler}

	update := Update{Message: commandMessage(123, "/settings time 10:15")}
	b.ProcessUpdate(context.Background(), update)

	if settings.DigestTime() != "10:15" {
		t.Fatalf("expected digest time updated")
	}
	if scheduler.updated != "10:15" {
		t.Fatalf("expected scheduler updated")
	}
}

func TestStatsNoLikes(t *testing.T) {
	sender := &mockSender{}
	storage := &mockStorage{likes: 0}
	settings := NewSettings(123, "09:00", 30)
	b := &Bot{Sender: sender, Storage: storage, Settings: settings}

	update := Update{Message: commandMessage(123, "/stats")}
	b.ProcessUpdate(context.Background(), update)

	if len(sender.messages) == 0 {
		t.Fatalf("expected stats message")
	}
}

func TestReactionThumbsUp(t *testing.T) {
	sender := &mockSender{}
	storage := &mockStorage{articles: map[int]model.Article{10: {ID: 1, Tags: []string{"go"}}}}
	settings := NewSettings(123, "09:00", 30)
	b := &Bot{Sender: sender, Storage: storage, Settings: settings, TagBoostOnLike: 0.2}

	update := Update{MessageReaction: &MessageReaction{Chat: tgbotapi.Chat{ID: 123}, MessageID: 10, NewReaction: []Reaction{{Emoji: "👍"}}}}
	b.ProcessUpdate(context.Background(), update)

	if len(storage.boosted) == 0 || storage.boosted[0] != "go" {
		t.Fatalf("expected tag boosted")
	}
	if len(storage.liked) != 1 {
		t.Fatalf("expected like recorded")
	}
}
