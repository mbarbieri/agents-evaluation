package bot

import (
	"strconv"
)

// MessageSender sends messages to Telegram
type MessageSender interface {
	SendMessage(chatID int64, text, parseMode string) error
}

// BotStorage handles storage operations for bot
type BotStorage interface {
	GetSetting(key string) (string, error)
	SetSetting(key, value string) error
}

// CommandHandlers handles bot commands
type CommandHandlers struct {
	Storage BotStorage
	Sender  MessageSender
}

// Start handles the /start command
func (h *CommandHandlers) Start(chatID int64) error {
	// Set chat_id
	err := h.Storage.SetSetting("chat_id", strconv.FormatInt(chatID, 10))
	if err != nil {
		return err
	}

	// Send welcome message
	message := `Welcome to HN Telegram Bot!

Available commands:
/fetch - Manually trigger digest
/settings - View or update settings
/stats - View preference statistics

React with 👍 to articles to learn your preferences.`
	return h.Sender.SendMessage(chatID, message, "")
}
