package bot

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

// Update represents a Telegram update with reaction support.
type Update struct {
	UpdateID        int              `json:"update_id"`
	Message         *tgbotapi.Message `json:"message,omitempty"`
	MessageReaction *MessageReaction `json:"message_reaction,omitempty"`
}

// MessageReaction represents a reaction update.
type MessageReaction struct {
	Chat        tgbotapi.Chat `json:"chat"`
	MessageID   int          `json:"message_id"`
	User        *tgbotapi.User `json:"user,omitempty"`
	Date        int          `json:"date"`
	OldReaction []Reaction   `json:"old_reaction"`
	NewReaction []Reaction   `json:"new_reaction"`
}

// Reaction represents a single reaction entry.
type Reaction struct {
	Type  string `json:"type"`
	Emoji string `json:"emoji"`
}
