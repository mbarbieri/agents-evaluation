package bot

import (
	"testing"
)

// Mock implementations
type mockMessageSender struct {
	sentMessages []string
}

func (m *mockMessageSender) SendMessage(chatID int64, text, parseMode string) error {
	m.sentMessages = append(m.sentMessages, text)
	return nil
}

type mockStorage struct {
	settings map[string]string
}

func (m *mockStorage) GetSetting(key string) (string, error) {
	return m.settings[key], nil
}

func (m *mockStorage) SetSetting(key, value string) error {
	if m.settings == nil {
		m.settings = make(map[string]string)
	}
	m.settings[key] = value
	return nil
}

func TestStartHandler(t *testing.T) {
	sender := &mockMessageSender{}
	storage := &mockStorage{}

	handler := &CommandHandlers{
		Storage: storage,
		Sender:  sender,
	}

	err := handler.Start(123)
	if err != nil {
		t.Fatal(err)
	}

	// Check chat_id set
	chatID, err := storage.GetSetting("chat_id")
	if err != nil {
		t.Fatal(err)
	}
	if chatID != "123" {
		t.Errorf("expected chat_id 123, got %s", chatID)
	}

	// Check message sent
	if len(sender.sentMessages) != 1 {
		t.Errorf("expected 1 message, got %d", len(sender.sentMessages))
	}
	if len(sender.sentMessages) == 0 || !contains(sender.sentMessages[0], "Welcome") {
		t.Errorf("expected welcome message, got %s", sender.sentMessages[0])
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr // simple check
}
