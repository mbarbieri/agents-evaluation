package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/opencode/hn-telegram-bot/config"
	"github.com/opencode/hn-telegram-bot/storage"
)

type Bot struct {
	api            *tgbotapi.BotAPI
	storage        Storage
	cfg            *config.Config
	articleSender  ArticleSender
	onConfigChange func()
}

func NewBot(cfg *config.Config, s Storage, sender ArticleSender, onChange func()) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create telegram bot: %w", err)
	}

	return &Bot{
		api:            api,
		storage:        s,
		cfg:            cfg,
		articleSender:  sender,
		onConfigChange: onChange,
	}, nil
}

func (b *Bot) SetArticleSender(sender ArticleSender) {
	b.articleSender = sender
}

func (b *Bot) Start(ctx context.Context) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	u.AllowedUpdates = []string{"message", "message_reaction"}

	// We use manual long polling to handle message_reaction which is not natively supported by the lib's wrappers
	for {
		select {
		case <-ctx.Done():
			return
		default:
			updates, err := b.getUpdates(u)
			if err != nil {
				slog.Error("failed to get updates", "error", err)
				continue
			}

			for _, update := range updates {
				if update.UpdateID >= u.Offset {
					u.Offset = update.UpdateID + 1
				}

				if update.MessageReaction != nil {
					b.handleReaction(ctx, update.MessageReaction)
					continue
				}

				if update.Message != nil {
					b.handleMessage(ctx, update.Message)
				}
			}
		}
	}
}

func (b *Bot) handleMessage(ctx context.Context, msg *tgbotapi.Message) {
	if !msg.IsCommand() {
		return
	}

	switch msg.Command() {
	case "start":
		b.handleStart(ctx, msg)
	case "fetch":
		b.handleFetch(ctx, msg)
	case "settings":
		b.handleSettings(ctx, msg)
	case "stats":
		b.handleStats(ctx, msg)
	default:
		// Unknown command
	}
}

func (b *Bot) handleStart(ctx context.Context, msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	err := b.storage.SetSetting(ctx, "chat_id", strconv.FormatInt(chatID, 10))
	if err != nil {
		slog.Error("failed to save chat_id", "error", err)
	}
	b.cfg.ChatID = chatID

	welcome := `Welcome to HN Telegram Bot! 🚀

I will deliver a daily digest of Hacker News stories personalized for you.

Commands:
/fetch - Get a digest immediately
/settings - View or change your preferences
/stats - See what topics I've learned you like

React with 👍 to any article I send to help me learn your interests!`

	reply := tgbotapi.NewMessage(chatID, welcome)
	b.api.Send(reply)
}

func (b *Bot) handleFetch(ctx context.Context, msg *tgbotapi.Message) {
	if b.cfg.ChatID == 0 {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "Please run /start first to register your chat.")
		b.api.Send(reply)
		return
	}

	reply := tgbotapi.NewMessage(msg.Chat.ID, "Fetching your personalized digest... ⏳")
	b.api.Send(reply)

	go func() {
		if err := b.articleSender.SendDigest(ctx); err != nil {
			slog.Error("failed to send manual digest", "error", err)
			errorReply := tgbotapi.NewMessage(msg.Chat.ID, "Sorry, I encountered an error while fetching the digest.")
			b.api.Send(errorReply)
		}
	}()
}

func (b *Bot) handleSettings(ctx context.Context, msg *tgbotapi.Message) {
	args := strings.Fields(msg.CommandArguments())

	if len(args) == 0 {
		// Display current settings
		settingsMsg := fmt.Sprintf("Current Settings:\n- Digest Time: %s\n- Article Count: %d\n\nTo change: /settings time HH:MM or /settings count N", b.cfg.DigestTime, b.cfg.ArticleCount)
		reply := tgbotapi.NewMessage(msg.Chat.ID, settingsMsg)
		b.api.Send(reply)
		return
	}

	if len(args) < 2 {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "Usage: /settings time HH:MM or /settings count N")
		b.api.Send(reply)
		return
	}

	switch args[0] {
	case "time":
		timeStr := args[1]
		var h, m int
		if _, err := fmt.Sscanf(timeStr, "%d:%d", &h, &m); err != nil || h < 0 || h > 23 || m < 0 || m > 59 {
			reply := tgbotapi.NewMessage(msg.Chat.ID, "Invalid time format. Use HH:MM (e.g., 09:30).")
			b.api.Send(reply)
			return
		}

		b.cfg.DigestTime = timeStr
		if err := b.storage.SetSetting(ctx, "digest_time", timeStr); err != nil {
			slog.Error("failed to save digest_time", "error", err)
		}

		if b.onConfigChange != nil {
			b.onConfigChange()
		}
		reply := tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("Digest time updated to %s", timeStr))
		b.api.Send(reply)

	case "count":
		count, err := strconv.Atoi(args[1])
		if err != nil || count < 1 || count > 100 {
			reply := tgbotapi.NewMessage(msg.Chat.ID, "Invalid count. Use a number between 1 and 100.")
			b.api.Send(reply)
			return
		}

		b.cfg.ArticleCount = count
		if err := b.storage.SetSetting(ctx, "article_count", strconv.Itoa(count)); err != nil {
			slog.Error("failed to save article_count", "error", err)
		}
		reply := tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("Article count updated to %d", count))
		b.api.Send(reply)

	default:
		reply := tgbotapi.NewMessage(msg.Chat.ID, "Unknown setting. Use 'time' or 'count'.")
		b.api.Send(reply)
	}
}

func (b *Bot) handleStats(ctx context.Context, msg *tgbotapi.Message) {
	tags, err := b.storage.GetTopTags(ctx, 10)
	if err != nil {
		slog.Error("failed to get top tags", "error", err)
		return
	}

	likes, err := b.storage.GetTotalLikes(ctx)
	if err != nil {
		slog.Error("failed to get total likes", "error", err)
	}

	if len(tags) == 0 {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "I haven't learned your preferences yet. React with 👍 to articles you like!")
		b.api.Send(reply)
		return
	}

	var sb strings.Builder
	sb.WriteString("<b>Your Top Interests:</b>\n\n")
	for i, t := range tags {
		sb.WriteString(fmt.Sprintf("%d. %s: %.2f\n", i+1, t.Name, t.Weight))
	}
	sb.WriteString(fmt.Sprintf("\nTotal Likes: %d", likes))

	reply := tgbotapi.NewMessage(msg.Chat.ID, sb.String())
	reply.ParseMode = tgbotapi.ModeHTML
	b.api.Send(reply)
}

func (b *Bot) handleReaction(ctx context.Context, reaction *MessageReactionUpdated) {
	// Only care about thumbs up
	hasThumbsUp := false
	for _, r := range reaction.NewReaction {
		if r.Emoji == "👍" {
			hasThumbsUp = true
			break
		}
	}

	if !hasThumbsUp {
		return
	}

	art, err := b.storage.GetArticleByMessageID(ctx, reaction.MessageID)
	if err != nil {
		slog.Error("failed to lookup article by message id", "error", err, "msgID", reaction.MessageID)
		return
	}

	if art == nil {
		return // Not an article message or not tracked
	}

	liked, err := b.storage.IsArticleLiked(ctx, art.ID)
	if err != nil {
		slog.Error("failed to check if article liked", "error", err, "artID", art.ID)
		return
	}

	if liked {
		return // Already liked
	}

	if err := b.storage.LikeArticle(ctx, art.ID); err != nil {
		slog.Error("failed to record like", "error", err, "artID", art.ID)
	}

	// Boost tags
	weights, err := b.storage.GetAllTagWeights(ctx)
	if err != nil {
		slog.Error("failed to get tag weights", "error", err)
		return
	}

	for _, tag := range art.Tags {
		currentWeight := 1.0
		if w, ok := weights[tag]; ok {
			currentWeight = w
		}
		newWeight := currentWeight + b.cfg.TagBoostOnLike
		if err := b.storage.UpdateTagWeight(ctx, tag, newWeight, 1); err != nil {
			slog.Error("failed to update tag weight", "tag", tag, "error", err)
		}
	}

	slog.Info("Processed like for article", "artID", art.ID, "title", art.Title)
}

func (b *Bot) SendArticle(chatID int64, art *storage.Article) (int, error) {
	text := fmt.Sprintf("<b>🚀 %s</b>\n\n<i>%s</i>\n\n⭐ %d points | 💬 %d comments\n\n<a href=\"%s\">Read Article</a> | <a href=\"https://news.ycombinator.com/item?id=%d\">HN Discussion</a>",
		escapeHTML(art.Title),
		escapeHTML(art.Summary),
		art.Score,
		0, // We need to store comment count in storage.Article or fetch it
		art.URL,
		art.ID,
	)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.DisableWebPagePreview = false

	sent, err := b.api.Send(msg)
	if err != nil {
		return 0, err
	}
	return sent.MessageID, nil
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func (b *Bot) getUpdates(config tgbotapi.UpdateConfig) ([]CustomUpdate, error) {
	params := make(map[string]string)
	if config.Offset != 0 {
		params["offset"] = strconv.Itoa(config.Offset)
	}
	if config.Timeout != 0 {
		params["timeout"] = strconv.Itoa(config.Timeout)
	}
	if len(config.AllowedUpdates) > 0 {
		data, _ := json.Marshal(config.AllowedUpdates)
		params["allowed_updates"] = string(data)
	}

	resp, err := b.api.MakeRequest("getUpdates", params)
	if err != nil {
		return nil, err
	}

	var result []CustomUpdate
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, err
	}

	return result, nil
}
