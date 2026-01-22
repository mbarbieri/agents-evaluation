package bot

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/opencode/hn-telegram-bot/storage"
)

// Reaction types as Telegram API doesn't have them in the library yet
type MessageReactionUpdated struct {
	Chat        tgbotapi.Chat  `json:"chat"`
	MessageID   int            `json:"message_id"`
	User        *tgbotapi.User `json:"user"`
	ActorChat   *tgbotapi.Chat `json:"actor_chat"`
	Date        int            `json:"date"`
	OldReaction []ReactionType `json:"old_reaction"`
	NewReaction []ReactionType `json:"new_reaction"`
}

type ReactionType struct {
	Type  string `json:"type"`
	Emoji string `json:"emoji"`
}

// CustomUpdate wraps tgbotapi.Update to include MessageReaction
type CustomUpdate struct {
	tgbotapi.Update
	MessageReaction *MessageReactionUpdated `json:"message_reaction"`
}

// Interfaces for dependency injection
type Storage interface {
	SetSetting(ctx context.Context, key, value string) error
	GetSetting(ctx context.Context, key string) (string, error)
	GetTopTags(ctx context.Context, limit int) ([]storage.TagWeight, error)
	GetTotalLikes(ctx context.Context) (int, error)
	GetArticleByMessageID(ctx context.Context, msgID int) (*storage.Article, error)
	IsArticleLiked(ctx context.Context, id int64) (bool, error)
	LikeArticle(ctx context.Context, id int64) error
	UpdateTagWeight(ctx context.Context, name string, weight float64, countIncr int) error
	GetAllTagWeights(ctx context.Context) (map[string]float64, error)
}

type ArticleSender interface {
	SendDigest(ctx context.Context) error
}
