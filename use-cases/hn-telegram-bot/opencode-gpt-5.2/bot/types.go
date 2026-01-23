package bot

// Minimal structures to support Telegram "message_reaction" updates.

type ReactionUpdate struct {
	UpdateID        int              `json:"update_id"`
	MessageReaction *MessageReaction `json:"message_reaction,omitempty"`
}

type MessageReaction struct {
	Chat        Chat           `json:"chat"`
	MessageID   int            `json:"message_id"`
	User        User           `json:"user"`
	Date        int64          `json:"date"`
	OldReaction []ReactionType `json:"old_reaction"`
	NewReaction []ReactionType `json:"new_reaction"`
}

type Chat struct {
	ID int64 `json:"id"`
}

type User struct {
	ID int64 `json:"id"`
}

type ReactionType struct {
	Type  string `json:"type"`
	Emoji string `json:"emoji"`
}
