package bot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type ArticleStorage interface {
	GetArticleByTelegramMsgID(msgID int) (Article, error)
	IsLiked(articleID int) (bool, error)
	RecordLike(articleID int) error
	GetTagWeight(tag string) (float64, error)
	SetTagWeight(tag string, weight float64) error
	IncrementTagOccurrence(tag string) error
}

type SettingsStorage interface {
	GetSetting(key string) (string, error)
	SetSetting(key string, value string) error
}

type StatsStorage interface {
	GetTopTags(limit int) ([]TagWeight, error)
	GetLikeCount() (int, error)
}

type Article struct {
	ID   int
	Tags []string
}

type TagWeight struct {
	Tag    string
	Weight float64
}

type Scheduler interface {
	Schedule(timeStr string, callback func()) error
	Start()
	Stop()
}

type DigestRunner interface {
	Run()
}

type CommandHandler struct {
	storage        interface {
		ArticleStorage
		SettingsStorage
		StatsStorage
	}
	scheduler       Scheduler
	digestRunner    DigestRunner
	tagBoostAmount  float64
	settingsMutex   sync.Mutex
}

func NewCommandHandler(storage interface {
	ArticleStorage
	SettingsStorage
	StatsStorage
}, scheduler Scheduler, digestRunner DigestRunner, tagBoostAmount float64) *CommandHandler {
	return &CommandHandler{
		storage:        storage,
		scheduler:      scheduler,
		digestRunner:   digestRunner,
		tagBoostAmount: tagBoostAmount,
	}
}

func (h *CommandHandler) HandleStart(chatID int64) string {
	h.storage.SetSetting("chat_id", strconv.FormatInt(chatID, 10))
	return `Welcome to the HN Digest Bot!

Available commands:
/fetch - Manually trigger digest delivery
/settings - View or update digest settings
/stats - View your learned preferences`
}

func (h *CommandHandler) HandleSettings(args string) string {
	h.settingsMutex.Lock()
	defer h.settingsMutex.Unlock()

	args = strings.TrimSpace(args)

	if args == "" {
		return h.displaySettings()
	}

	parts := strings.SplitN(args, " ", 2)
	if len(parts) != 2 {
		return h.settingsUsage()
	}

	command := parts[0]
	value := parts[1]

	switch command {
	case "time":
		return h.updateDigestTime(value)
	case "count":
		return h.updateArticleCount(value)
	default:
		return h.settingsUsage()
	}
}

func (h *CommandHandler) displaySettings() string {
	digestTime, _ := h.storage.GetSetting("digest_time")
	articleCount, _ := h.storage.GetSetting("article_count")

	if digestTime == "" {
		digestTime = "09:00"
	}
	if articleCount == "" {
		articleCount = "30"
	}

	return fmt.Sprintf(`Current settings:
Digest time: %s
Article count: %s

To update:
/settings time HH:MM
/settings count N`, digestTime, articleCount)
}

func (h *CommandHandler) updateDigestTime(timeStr string) string {
	if err := validateTimeFormat(timeStr); err != nil {
		return fmt.Sprintf("Invalid time format: %s\n\n%s", err, h.settingsUsage())
	}

	h.storage.SetSetting("digest_time", timeStr)

	// Update scheduler if available
	if h.scheduler != nil && h.digestRunner != nil {
		h.scheduler.Schedule(timeStr, h.digestRunner.Run)
	}

	return fmt.Sprintf("Digest time updated to %s", timeStr)
}

func (h *CommandHandler) updateArticleCount(countStr string) string {
	count, err := strconv.Atoi(countStr)
	if err != nil || count < 1 || count > 100 {
		return fmt.Sprintf("Invalid count. Must be between 1 and 100.\n\n%s", h.settingsUsage())
	}

	h.storage.SetSetting("article_count", countStr)
	return fmt.Sprintf("Article count updated to %s", countStr)
}

func (h *CommandHandler) settingsUsage() string {
	return `Usage:
/settings time HH:MM (00:00 - 23:59)
/settings count N (1-100)`
}

func (h *CommandHandler) HandleStats() string {
	tags, _ := h.storage.GetTopTags(10)
	likeCount, _ := h.storage.GetLikeCount()

	if likeCount == 0 {
		return "No likes yet! React with 👍 to articles you enjoy to train your preferences."
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Your interests (based on %d likes):\n\n", likeCount))

	for _, tag := range tags {
		sb.WriteString(fmt.Sprintf("• %s (%.2f)\n", tag.Tag, tag.Weight))
	}

	return sb.String()
}

func (h *CommandHandler) HandleReaction(msgID int, emoji string) {
	if emoji != "👍" {
		return // Ignore non-thumbs-up reactions
	}

	article, err := h.storage.GetArticleByTelegramMsgID(msgID)
	if err != nil {
		slog.Warn("Reaction to unknown message", "msg_id", msgID)
		return
	}

	// Check if already liked
	liked, _ := h.storage.IsLiked(article.ID)
	if liked {
		return // Already liked, don't boost again
	}

	// Record like
	h.storage.RecordLike(article.ID)

	// Boost each tag
	for _, tag := range article.Tags {
		currentWeight, _ := h.storage.GetTagWeight(tag)
		newWeight := currentWeight + h.tagBoostAmount
		h.storage.SetTagWeight(tag, newWeight)
		h.storage.IncrementTagOccurrence(tag)
	}

	slog.Info("Boosted tags from like", "article_id", article.ID, "tags", article.Tags)
}

func validateTimeFormat(timeStr string) error {
	re := regexp.MustCompile(`^([0-1][0-9]|2[0-3]):([0-5][0-9])$`)
	if !re.MatchString(timeStr) {
		return fmt.Errorf("expected HH:MM format with hours 0-23 and minutes 0-59")
	}
	return nil
}

func formatArticleMessage(title, url, summary string, score, comments, hnID int) string {
	title = escapeHTML(title)
	summary = escapeHTML(summary)

	hnURL := fmt.Sprintf("https://news.ycombinator.com/item?id=%d", hnID)

	return fmt.Sprintf(`📰 <b>%s</b>

<i>%s</i>

🔗 <a href="%s">Read article</a>
💬 <a href="%s">%d points, %d comments</a>`,
		title, summary, url, hnURL, score, comments)
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// Bot integrates with Telegram
type Bot struct {
	api     *tgbotapi.BotAPI
	handler *CommandHandler
}

func New(token string, handler *CommandHandler) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	return &Bot{
		api:     api,
		handler: handler,
	}, nil
}

func (b *Bot) Start() error {
	slog.Info("Bot started", "username", b.api.Self.UserName)

	// Use manual getUpdates to support message_reaction
	offset := 0
	for {
		updates, err := b.getUpdates(offset)
		if err != nil {
			slog.Error("Failed to get updates", "error", err)
			continue
		}

		for _, update := range updates {
			offset = update.UpdateID + 1
			b.handleUpdate(update)
		}
	}
}

func (b *Bot) getUpdates(offset int) ([]Update, error) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates", b.api.Token)

	reqBody := map[string]interface{}{
		"offset":          offset,
		"timeout":         30,
		"allowed_updates": []string{"message", "message_reaction"},
	}

	jsonData, _ := json.Marshal(reqBody)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Ok     bool     `json:"ok"`
		Result []Update `json:"result"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result.Result, nil
}

type Update struct {
	UpdateID        int              `json:"update_id"`
	Message         *tgbotapi.Message `json:"message"`
	MessageReaction *MessageReaction `json:"message_reaction"`
}

type MessageReaction struct {
	Chat       tgbotapi.Chat  `json:"chat"`
	MessageID  int            `json:"message_id"`
	User       tgbotapi.User  `json:"user"`
	Date       int            `json:"date"`
	OldReaction []Reaction    `json:"old_reaction"`
	NewReaction []Reaction    `json:"new_reaction"`
}

type Reaction struct {
	Type  string `json:"type"`
	Emoji string `json:"emoji"`
}

func (b *Bot) handleUpdate(update Update) {
	if update.Message != nil {
		b.handleMessage(update.Message)
	} else if update.MessageReaction != nil {
		b.handleReaction(update.MessageReaction)
	}
}

func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	if !msg.IsCommand() {
		return
	}

	var response string
	switch msg.Command() {
	case "start":
		response = b.handler.HandleStart(msg.Chat.ID)
	case "settings":
		response = b.handler.HandleSettings(msg.CommandArguments())
	case "stats":
		response = b.handler.HandleStats()
	case "fetch":
		if b.handler.digestRunner != nil {
			go b.handler.digestRunner.Run()
			response = "Fetching digest..."
		} else {
			response = "Digest runner not configured"
		}
	default:
		return
	}

	b.sendMessage(msg.Chat.ID, response)
}

func (b *Bot) handleReaction(reaction *MessageReaction) {
	// Only process new reactions (additions)
	for _, newR := range reaction.NewReaction {
		if newR.Type == "emoji" {
			b.handler.HandleReaction(reaction.MessageID, newR.Emoji)
		}
	}
}

func (b *Bot) sendMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	b.api.Send(msg)
}

func (b *Bot) SendArticle(chatID int64, title, url, summary string, score, comments, hnID int) (int, error) {
	text := formatArticleMessage(title, url, summary, score, comments, hnID)
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"

	sent, err := b.api.Send(msg)
	if err != nil {
		return 0, err
	}

	return sent.MessageID, nil
}
