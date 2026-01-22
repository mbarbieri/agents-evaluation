package bot

import (
	"errors"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"hn-telegram-bot/storage"
)

// MockStorage implements storage interface for testing
type MockStorage struct {
	ChatID      int64
	Settings    map[string]string
	Articles    map[int]storage.Article
	Likes       map[int]bool
	TagWeights  map[string]float64
	TotalLikes  int
	BoostCalled bool
}

func (m *MockStorage) SetSetting(key, value string) error {
	m.Settings[key] = value
	if key == "chat_id" {
		// parse int64
	}
	return nil
}

func (m *MockStorage) GetSetting(key string) (string, error) {
	return m.Settings[key], nil
}

func (m *MockStorage) SaveArticle(a storage.Article) error {
	m.Articles[a.ID] = a
	return nil
}

func (m *MockStorage) MarkArticleSent(id, msgID int) error {
	if a, ok := m.Articles[id]; ok {
		a.MsgID = msgID
		a.SentAt = time.Now()
		m.Articles[id] = a
	}
	return nil
}

func (m *MockStorage) GetRecentSentArticleIDs(d time.Duration) ([]int, error) {
	return []int{}, nil
}

func (m *MockStorage) GetArticleByMsgID(msgID int) (*storage.Article, error) {
	for _, a := range m.Articles {
		if a.MsgID == msgID {
			return &a, nil
		}
	}
	return nil, errors.New("not found")
}

func (m *MockStorage) IsArticleLiked(id int) (bool, error) {
	return m.Likes[id], nil
}

func (m *MockStorage) AddLike(id int) error {
	m.Likes[id] = true
	return nil
}

func (m *MockStorage) BoostTag(tag string, initial, boost float64) error {
	m.TagWeights[tag] += boost // simplified
	m.BoostCalled = true
	return nil
}

func (m *MockStorage) GetTagWeights() (map[string]float64, error) {
	return m.TagWeights, nil
}

func (m *MockStorage) ApplyTagDecay(decay, min float64) error {
	return nil
}

func (m *MockStorage) GetTotalLikes() (int, error) {
	return m.TotalLikes, nil
}

// Mock Sender
type MockSender struct {
	SentMessages []tgbotapi.MessageConfig
}

func (s *MockSender) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	if msg, ok := c.(tgbotapi.MessageConfig); ok {
		s.SentMessages = append(s.SentMessages, msg)
		return tgbotapi.Message{MessageID: 1}, nil
	}
	return tgbotapi.Message{}, nil
}

func TestStartCommand(t *testing.T) {
	store := &MockStorage{Settings: make(map[string]string)}
	sender := &MockSender{}
	b := &Bot{
		storage: store,
		sender:  sender,
	}

	update := CustomUpdate{
		Message: &tgbotapi.Message{
			Text: "/start",
			Entities: []tgbotapi.MessageEntity{
				{Type: "bot_command", Offset: 0, Length: 6},
			},
			Chat: &tgbotapi.Chat{ID: 12345},
		},
	}

	b.HandleCustomUpdate(update)

	if store.Settings["chat_id"] != "12345" {
		t.Errorf("Chat ID not saved. Got: %s", store.Settings["chat_id"])
	}
	if len(sender.SentMessages) == 0 {
		t.Error("Welcome message not sent")
	}
}

func TestReactionHandling(t *testing.T) {
	store := &MockStorage{
		Articles: map[int]storage.Article{
			100: {ID: 100, MsgID: 555, Tags: []string{"go"}},
		},
		Likes:      make(map[int]bool),
		TagWeights: make(map[string]float64),
	}
	sender := &MockSender{}
	b := &Bot{
		storage:        store,
		sender:         sender,
		tagBoostOnLike: 0.2,
	}

	update := CustomUpdate{
		MessageReaction: &MessageReactionUpdated{
			Chat:      &tgbotapi.Chat{ID: 12345},
			MessageID: 555,
			User:      &tgbotapi.User{ID: 111},
			NewReaction: []ReactionType{
				{Type: "emoji", Emoji: "👍"},
			},
		},
	}

	b.HandleCustomUpdate(update)

	if !store.Likes[100] {
		t.Error("Article 100 should be liked")
	}
	if !store.BoostCalled {
		t.Error("BoostTag should be called")
	}
}

func TestFetchCommand(t *testing.T) {
	done := make(chan bool)
	digestFunc := func() { done <- true }

	b := &Bot{
		digestTrigger: digestFunc,
		sender:        &MockSender{},
	}

	update := CustomUpdate{
		Message: &tgbotapi.Message{
			Text: "/fetch",
			Entities: []tgbotapi.MessageEntity{
				{Type: "bot_command", Offset: 0, Length: 6},
			},
			Chat: &tgbotapi.Chat{ID: 123},
		},
	}
	b.HandleCustomUpdate(update)

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Error("Digest trigger not called")
	}
}
