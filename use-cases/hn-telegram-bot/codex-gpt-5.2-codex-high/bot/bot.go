package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"regexp"

	"hn-telegram-bot/model"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Storage defines persistence used by bot handlers.
type Storage interface {
	SetSetting(ctx context.Context, key, value string) error
	GetSetting(ctx context.Context, key string) (string, bool, error)
	ListTopTags(ctx context.Context, limit int) ([]model.TagWeight, error)
	CountLikes(ctx context.Context) (int, error)
	GetArticleByMessageID(ctx context.Context, msgID int) (model.Article, bool, error)
	IsLiked(ctx context.Context, articleID int64) (bool, error)
	BoostTags(ctx context.Context, tags []string, boost float64) error
	AddLike(ctx context.Context, articleID int64, likedAt time.Time) error
}

// DigestRunner triggers a digest run.
type DigestRunner interface {
	Run(ctx context.Context) error
}

// SchedulerUpdater updates the digest schedule.
type SchedulerUpdater interface {
	UpdateTime(digestTime string) error
}

// Bot handles Telegram updates.
type Bot struct {
	Sender          Sender
	Storage         Storage
	Digest          DigestRunner
	Scheduler       SchedulerUpdater
	Settings        *Settings
	TagBoostOnLike  float64
	Logger          *slog.Logger
}

// ProcessUpdate dispatches a Telegram update.
func (b *Bot) ProcessUpdate(ctx context.Context, update Update) {
	logger := b.Logger
	if logger == nil {
		logger = slog.Default()
	}

	if update.Message != nil {
		b.handleMessage(ctx, logger, update.Message)
		return
	}
	if update.MessageReaction != nil {
		b.handleReaction(ctx, logger, update.MessageReaction)
	}
}

func (b *Bot) handleMessage(ctx context.Context, logger *slog.Logger, msg *tgbotapi.Message) {
	if msg == nil || msg.Chat == nil {
		return
	}
	if b.Settings.ChatID() != 0 && msg.Chat.ID != b.Settings.ChatID() {
		return
	}

	if !msg.IsCommand() {
		return
	}
	command := msg.Command()
	args := strings.TrimSpace(msg.CommandArguments())

	if b.Settings.ChatID() == 0 && command != "start" {
		_, _ = b.Sender.SendText(ctx, msg.Chat.ID, "Please run /start first to register this chat.")
		return
	}

	switch command {
	case "start":
		b.handleStart(ctx, msg.Chat.ID)
	case "fetch":
		b.handleFetch(ctx, logger, msg.Chat.ID)
	case "settings":
		b.handleSettings(ctx, logger, msg.Chat.ID, args)
	case "stats":
		b.handleStats(ctx, logger, msg.Chat.ID)
	default:
		_, _ = b.Sender.SendText(ctx, msg.Chat.ID, "Unknown command. Try /start, /fetch, /settings, /stats.")
	}
}

func (b *Bot) handleStart(ctx context.Context, chatID int64) {
	b.Settings.SetChatID(chatID)
	_ = b.Storage.SetSetting(ctx, "chat_id", strconv.FormatInt(chatID, 10))
	message := "Welcome!\nCommands: /fetch, /settings, /stats"
	_, _ = b.Sender.SendText(ctx, chatID, message)
}

func (b *Bot) handleFetch(ctx context.Context, logger *slog.Logger, chatID int64) {
	if b.Digest == nil {
		return
	}
	if err := b.Digest.Run(ctx); err != nil {
		logger.Warn("manual_fetch_failed", slog.String("error", err.Error()))
	}
}

func (b *Bot) handleSettings(ctx context.Context, logger *slog.Logger, chatID int64, args string) {
	if args == "" {
		text := fmt.Sprintf("Digest time: %s\nArticle count: %d", b.Settings.DigestTime(), b.Settings.ArticleCount())
		_, _ = b.Sender.SendText(ctx, chatID, text)
		return
	}
	parts := strings.Fields(args)
	if len(parts) != 2 {
		b.sendSettingsUsage(ctx, chatID)
		return
	}
	switch parts[0] {
	case "time":
		if !validTime(parts[1]) {
			b.sendSettingsUsage(ctx, chatID)
			return
		}
		if b.Scheduler != nil {
			if err := b.Scheduler.UpdateTime(parts[1]); err != nil {
				logger.Warn("schedule_update_failed", slog.String("error", err.Error()))
				_, _ = b.Sender.SendText(ctx, chatID, "Failed to update digest time.")
				return
			}
		}
		b.Settings.SetDigestTime(parts[1])
		_ = b.Storage.SetSetting(ctx, "digest_time", parts[1])
		_, _ = b.Sender.SendText(ctx, chatID, fmt.Sprintf("Digest time updated to %s", parts[1]))
	case "count":
		count, err := strconv.Atoi(parts[1])
		if err != nil || count < 1 || count > 100 {
			b.sendSettingsUsage(ctx, chatID)
			return
		}
		b.Settings.SetArticleCount(count)
		_ = b.Storage.SetSetting(ctx, "article_count", strconv.Itoa(count))
		_, _ = b.Sender.SendText(ctx, chatID, fmt.Sprintf("Article count updated to %d", count))
	default:
		b.sendSettingsUsage(ctx, chatID)
	}
}

func (b *Bot) sendSettingsUsage(ctx context.Context, chatID int64) {
	_, _ = b.Sender.SendText(ctx, chatID, "Usage: /settings time HH:MM | /settings count N")
}

func (b *Bot) handleStats(ctx context.Context, logger *slog.Logger, chatID int64) {
	likes, err := b.Storage.CountLikes(ctx)
	if err != nil {
		logger.Warn("stats_count_likes_failed", slog.String("error", err.Error()))
		return
	}
	if likes == 0 {
		_, _ = b.Sender.SendText(ctx, chatID, "No likes yet. React with 👍 on articles to train your preferences.")
		return
	}
	tags, err := b.Storage.ListTopTags(ctx, 10)
	if err != nil {
		logger.Warn("stats_top_tags_failed", slog.String("error", err.Error()))
		return
	}
	var bld strings.Builder
	bld.WriteString("Top interests:\n")
	for _, tag := range tags {
		bld.WriteString(fmt.Sprintf("- %s: %.2f (%d)\n", tag.Tag, tag.Weight, tag.Count))
	}
	bld.WriteString(fmt.Sprintf("Total likes: %d", likes))
	_, _ = b.Sender.SendText(ctx, chatID, strings.TrimSpace(bld.String()))
}

func (b *Bot) handleReaction(ctx context.Context, logger *slog.Logger, reaction *MessageReaction) {
	if reaction == nil {
		return
	}
	if b.Settings.ChatID() != 0 && reaction.Chat.ID != b.Settings.ChatID() {
		return
	}
	if !hasThumbsUp(reaction.NewReaction) {
		return
	}
	article, ok, err := b.Storage.GetArticleByMessageID(ctx, reaction.MessageID)
	if err != nil {
		logger.Warn("reaction_article_lookup_failed", slog.String("error", err.Error()))
		return
	}
	if !ok {
		return
	}
	liked, err := b.Storage.IsLiked(ctx, article.ID)
	if err != nil {
		logger.Warn("reaction_like_check_failed", slog.String("error", err.Error()))
		return
	}
	if liked {
		return
	}
	if err := b.Storage.BoostTags(ctx, article.Tags, b.TagBoostOnLike); err != nil {
		logger.Warn("reaction_boost_failed", slog.String("error", err.Error()))
		return
	}
	if err := b.Storage.AddLike(ctx, article.ID, time.Now().UTC()); err != nil {
		logger.Warn("reaction_like_failed", slog.String("error", err.Error()))
		return
	}
	logger.Info("reaction_processed", slog.Int64("article_id", article.ID), slog.Int("message_id", reaction.MessageID))
}

func hasThumbsUp(reactions []Reaction) bool {
	for _, reaction := range reactions {
		if reaction.Emoji == "👍" {
			return true
		}
	}
	return false
}

func validTime(value string) bool {
	return timeHHMM.MatchString(value)
}

var timeHHMM = regexp.MustCompile(`^(?:[01]\d|2[0-3]):[0-5]\d$`)
