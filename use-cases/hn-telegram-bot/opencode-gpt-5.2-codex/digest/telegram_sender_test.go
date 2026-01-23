package digest

import (
	"context"
	"testing"
)

type mockTelegramBot struct {
	message string
	chatID  int64
}

func (m *mockTelegramBot) SendArticle(ctx context.Context, chatID int64, message string) (int, error) {
	m.chatID = chatID
	m.message = message
	return 99, nil
}

func TestTelegramSender(t *testing.T) {
	t.Parallel()
	bot := &mockTelegramBot{}
	sender := &TelegramSender{Bot: bot, ChatID: 1}
	article := Article{ID: 1, Title: "Title", Summary: "Summary", URL: "https://example.com", Score: 1, Comments: 1}
	sent, err := sender.Send(context.Background(), article)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if sent.MessageID != 99 {
		t.Fatalf("expected message id")
	}
	if bot.chatID != 1 {
		t.Fatalf("expected chat id")
	}
}
