package bot

import (
	"encoding/json"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type ReactionType struct {
	Type  string `json:"type"`
	Emoji string `json:"emoji"`
}

type MessageReactionUpdated struct {
	Chat        tgbotapi.Chat  `json:"chat"`
	MessageID   int            `json:"message_id"`
	User        *tgbotapi.User `json:"user"`
	Date        int            `json:"date"`
	OldReaction []ReactionType `json:"old_reaction"`
	NewReaction []ReactionType `json:"new_reaction"`
}

type CustomUpdate struct {
	tgbotapi.Update
	MessageReaction *MessageReactionUpdated `json:"message_reaction"`
}

func UnmarshalUpdate(data []byte) (*CustomUpdate, error) {
	var update CustomUpdate
	if err := json.Unmarshal(data, &update); err != nil {
		return nil, err
	}
	return &update, nil
}
