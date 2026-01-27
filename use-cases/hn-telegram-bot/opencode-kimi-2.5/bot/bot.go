package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"hn-telegram-bot/storage"
)

// Storage defines the interface for storage operations needed by the bot
type Storage interface {
	SetSetting(key, value string) error
	GetSetting(key string) (string, error)
	FindArticleByMessageID(msgID int64) (*storage.Article, error)
	RecordLikeWithCheck(articleID int64) (bool, error)
	GetLikeCount() (int, error)
	GetTopTags(n int) ([]storage.TagWeight, error)
	UpsertTagWeight(tag string, weight float64, count int) error
	GetAllTagWeights() (map[string]float64, error)
}

// Article represents an article for internal bot use
type Article struct {
	ID            int64
	Title         string
	Tags          []string
	TelegramMsgID int64
}

// DigestTrigger is a function that triggers the digest workflow
type DigestTrigger func(ctx context.Context) error

// SettingsUpdater is a function that updates settings
type SettingsUpdater func(digestTime string, articleCount int) error

// Bot handles Telegram bot commands and reactions
type Bot struct {
	api             *tgbotapi.BotAPI
	storage         Storage
	digestTrigger   DigestTrigger
	settingsUpdater SettingsUpdater
	config          Config
	chatID          int64
	mu              sync.RWMutex
	logger          *slog.Logger
}

// Config holds bot configuration
type Config struct {
	Token         string
	InitialChatID int64
	DigestTime    string
	ArticleCount  int
}

// New creates a new Bot instance
func New(config Config, storage Storage, logger *slog.Logger) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(config.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot API: %w", err)
	}

	bot := &Bot{
		api:     api,
		storage: storage,
		config:  config,
		chatID:  config.InitialChatID,
		logger:  logger,
	}

	// Load chat ID from storage if available
	if chatIDStr, err := storage.GetSetting("chat_id"); err == nil && chatIDStr != "" {
		if chatID, err := strconv.ParseInt(chatIDStr, 10, 64); err == nil {
			bot.chatID = chatID
		}
	}

	return bot, nil
}

// SetDigestTrigger sets the function to trigger digest
func (b *Bot) SetDigestTrigger(trigger DigestTrigger) {
	b.digestTrigger = trigger
}

// SetSettingsUpdater sets the function to update settings
func (b *Bot) SetSettingsUpdater(updater SettingsUpdater) {
	b.settingsUpdater = updater
}

// GetChatID returns the configured chat ID
func (b *Bot) GetChatID() int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.chatID
}

// SetChatID sets the chat ID
func (b *Bot) SetChatID(chatID int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.chatID = chatID
}

// SendMessage sends a text message to the configured chat
func (b *Bot) SendMessage(text string) error {
	chatID := b.GetChatID()
	if chatID == 0 {
		return fmt.Errorf("chat ID not configured")
	}

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	_, err := b.api.Send(msg)
	return err
}

// SendArticle sends an article message and returns the message ID
func (b *Bot) SendArticle(title, summary string, hnScore, commentCount int, articleURL, hnURL string) (int64, error) {
	chatID := b.GetChatID()
	if chatID == 0 {
		return 0, fmt.Errorf("chat ID not configured")
	}

	// Escape HTML special characters in title and summary
	title = escapeHTML(title)
	summary = escapeHTML(summary)

	text := fmt.Sprintf(
		"📰 <b>%s</b>\n\n"+
			"<i>%s</i>\n\n"+
			"📊 %d points · 💬 %d comments\n"+
			"🔗 <a href=\"%s\">Article</a> · <a href=\"%s\">Comments</a>",
		title, summary, hnScore, commentCount, articleURL, hnURL,
	)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.DisableWebPagePreview = false

	sent, err := b.api.Send(msg)
	if err != nil {
		return 0, fmt.Errorf("failed to send article: %w", err)
	}

	return int64(sent.MessageID), nil
}

// HandleUpdate processes a Telegram update
func (b *Bot) HandleUpdate(update tgbotapi.Update) {
	if update.Message != nil && update.Message.IsCommand() {
		b.handleCommand(update.Message)
	}
}

// handleCommand processes bot commands
func (b *Bot) handleCommand(msg *tgbotapi.Message) {
	command := msg.Command()
	args := msg.CommandArguments()

	switch command {
	case "start":
		b.handleStart(msg)
	case "fetch":
		b.handleFetch(msg)
	case "settings":
		b.handleSettings(msg, args)
	case "stats":
		b.handleStats(msg)
	default:
		b.sendResponse(msg.Chat.ID, "Unknown command. Use /start to see available commands.")
	}
}

// handleStart processes the /start command
func (b *Bot) handleStart(msg *tgbotapi.Message) {
	chatID := msg.Chat.ID

	// Save chat ID
	if err := b.storage.SetSetting("chat_id", strconv.FormatInt(chatID, 10)); err != nil {
		b.logger.Error("Failed to save chat ID", "error", err)
	}

	b.SetChatID(chatID)

	response := "Welcome to HN Digest Bot! 🤖\n\n" +
		"Available commands:\n" +
		"/fetch - Get articles now\n" +
		"/settings - View or update settings\n" +
		"/stats - View your preferences\n\n" +
		"React with 👍 on articles to train my recommendations!"

	b.sendResponse(chatID, response)
}

// handleFetch processes the /fetch command
func (b *Bot) handleFetch(msg *tgbotapi.Message) {
	if b.digestTrigger == nil {
		b.sendResponse(msg.Chat.ID, "Digest trigger not configured.")
		return
	}

	b.sendResponse(msg.Chat.ID, "Fetching articles...")

	go func() {
		if err := b.digestTrigger(context.Background()); err != nil {
			b.logger.Error("Digest trigger failed", "error", err)
			b.sendResponse(msg.Chat.ID, "Failed to fetch articles. Please try again later.")
		}
	}()
}

// handleSettings processes the /settings command
func (b *Bot) handleSettings(msg *tgbotapi.Message, args string) {
	chatID := msg.Chat.ID

	if args == "" {
		// Display current settings
		digestTime := b.config.DigestTime
		if dt, err := b.storage.GetSetting("digest_time"); err == nil && dt != "" {
			digestTime = dt
		}

		articleCount := b.config.ArticleCount
		if ac, err := b.storage.GetSetting("article_count"); err == nil && ac != "" {
			if count, err := strconv.Atoi(ac); err == nil {
				articleCount = count
			}
		}

		response := fmt.Sprintf(
			"Current settings:\n"+
				"🕐 Digest time: %s\n"+
				"📰 Article count: %d\n\n"+
				"To update:\n"+
				"/settings time HH:MM\n"+
				"/settings count N",
			digestTime, articleCount,
		)
		b.sendResponse(chatID, response)
		return
	}

	// Parse setting update
	parts := strings.Fields(args)
	if len(parts) != 2 {
		b.sendResponse(chatID, "Invalid format. Use:\n/settings time HH:MM\n/settings count N")
		return
	}

	settingType := parts[0]
	value := parts[1]

	switch settingType {
	case "time":
		if !isValidTime(value) {
			b.sendResponse(chatID, "Invalid time format. Use HH:MM (24-hour format).")
			return
		}
		if err := b.storage.SetSetting("digest_time", value); err != nil {
			b.logger.Error("Failed to save digest time", "error", err)
			b.sendResponse(chatID, "Failed to update setting.")
			return
		}
		if b.settingsUpdater != nil {
			b.settingsUpdater(value, 0) // Only update time
		}
		b.sendResponse(chatID, fmt.Sprintf("Digest time updated to %s", value))

	case "count":
		count, err := strconv.Atoi(value)
		if err != nil || count < 1 || count > 100 {
			b.sendResponse(chatID, "Invalid count. Must be between 1 and 100.")
			return
		}
		if err := b.storage.SetSetting("article_count", value); err != nil {
			b.logger.Error("Failed to save article count", "error", err)
			b.sendResponse(chatID, "Failed to update setting.")
			return
		}
		if b.settingsUpdater != nil {
			b.settingsUpdater("", count) // Only update count
		}
		b.sendResponse(chatID, fmt.Sprintf("Article count updated to %d", count))

	default:
		b.sendResponse(chatID, "Unknown setting. Use 'time' or 'count'.")
	}
}

// handleStats processes the /stats command
func (b *Bot) handleStats(msg *tgbotapi.Message) {
	likeCount, err := b.storage.GetLikeCount()
	if err != nil {
		b.logger.Error("Failed to get like count", "error", err)
		b.sendResponse(msg.Chat.ID, "Failed to retrieve statistics.")
		return
	}

	if likeCount == 0 {
		b.sendResponse(msg.Chat.ID, "No likes yet! 👍\n\nReact with 👍 on articles to help me learn your preferences.")
		return
	}

	topTags, err := b.storage.GetTopTags(10)
	if err != nil {
		b.logger.Error("Failed to get top tags", "error", err)
		b.sendResponse(msg.Chat.ID, "Failed to retrieve statistics.")
		return
	}

	var response strings.Builder
	response.WriteString(fmt.Sprintf("📊 Statistics\n\n"))
	response.WriteString(fmt.Sprintf("Total likes: %d\n\n", likeCount))

	if len(topTags) > 0 {
		response.WriteString("Top interests:\n")
		for _, tag := range topTags {
			response.WriteString(fmt.Sprintf("• %s (%.2f)\n", tag.Name, tag.Weight))
		}
	}

	b.sendResponse(msg.Chat.ID, response.String())
}

// HandleReaction processes a thumbs-up reaction on an article
func (b *Bot) HandleReaction(msgID int64) error {
	// Find the article by message ID
	article, err := b.storage.FindArticleByMessageID(msgID)
	if err != nil {
		return fmt.Errorf("failed to find article: %w", err)
	}

	if article == nil {
		// Not an article message, silently ignore
		return nil
	}

	// Record the like (idempotent)
	isNew, err := b.storage.RecordLikeWithCheck(article.ID)
	if err != nil {
		return fmt.Errorf("failed to record like: %w", err)
	}

	if !isNew {
		// Already liked, skip tag boosting
		return nil
	}

	// Boost tag weights
	tagWeights, err := b.storage.GetAllTagWeights()
	if err != nil {
		return fmt.Errorf("failed to get tag weights: %w", err)
	}

	for _, tag := range article.Tags {
		currentWeight, _ := tagWeights[tag]
		if currentWeight == 0 {
			currentWeight = 1.0
		}
		newWeight := currentWeight + 0.2 // boost amount from config
		if err := b.storage.UpsertTagWeight(tag, newWeight, 1); err != nil {
			b.logger.Error("Failed to update tag weight", "error", err, "tag", tag)
		}
	}

	b.logger.Info("Recorded like and boosted tags",
		"article_id", article.ID,
		"tags", article.Tags,
	)

	return nil
}

// sendResponse sends a response message
func (b *Bot) sendResponse(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	if _, err := b.api.Send(msg); err != nil {
		b.logger.Error("Failed to send response", "error", err)
	}
}

// escapeHTML escapes HTML special characters
func escapeHTML(text string) string {
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")
	return text
}

// isValidTime checks if a string is in HH:MM format
func isValidTime(timeStr string) bool {
	if len(timeStr) != 5 || timeStr[2] != ':' {
		return false
	}

	hour := (timeStr[0]-'0')*10 + (timeStr[1] - '0')
	minute := (timeStr[3]-'0')*10 + (timeStr[4] - '0')

	return hour >= 0 && hour <= 23 && minute >= 0 && minute <= 59
}

// GetUpdatesChan returns a channel for receiving updates
func (b *Bot) GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	return b.api.GetUpdatesChan(config)
}
