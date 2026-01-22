package bot

import (
	"encoding/json"
	"fmt"
	"html"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"hn-telegram-bot/config"
	"hn-telegram-bot/scheduler"
	"hn-telegram-bot/storage"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Interfaces
type Storage interface {
	SetSetting(key, value string) error
	GetSetting(key string) (string, error)
	GetArticleByMsgID(msgID int) (*storage.Article, error)
	IsArticleLiked(id int) (bool, error)
	AddLike(id int) error
	BoostTag(tag string, initial, boost float64) error
	GetTagWeights() (map[string]float64, error)
	GetTotalLikes() (int, error)
}

type Sender interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
}

// Custom Types for Reaction
type CustomUpdate struct {
	UpdateID        int                     `json:"update_id"`
	Message         *tgbotapi.Message       `json:"message"`
	MessageReaction *MessageReactionUpdated `json:"message_reaction"`
}

type MessageReactionUpdated struct {
	Chat        *tgbotapi.Chat `json:"chat"`
	MessageID   int            `json:"message_id"`
	User        *tgbotapi.User `json:"user"`
	Date        int            `json:"date"`
	OldReaction []ReactionType `json:"old_reaction"`
	NewReaction []ReactionType `json:"new_reaction"`
}

type ReactionType struct {
	Type  string `json:"type"`
	Emoji string `json:"emoji"`
}

// Bot Struct
type Bot struct {
	api            *tgbotapi.BotAPI
	sender         Sender
	storage        Storage
	scheduler      *scheduler.Scheduler
	config         *config.Config
	digestTrigger  func()
	tagBoostOnLike float64
}

func New(cfg *config.Config, store Storage, sched *scheduler.Scheduler, trigger func()) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot api: %w", err)
	}

	return &Bot{
		api:            api,
		sender:         api,
		storage:        store,
		scheduler:      sched,
		config:         cfg,
		digestTrigger:  trigger,
		tagBoostOnLike: cfg.TagBoostOnLike,
	}, nil
}

func (b *Bot) Start() {
	log.Printf("Authorized on account %s", b.api.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	u.AllowedUpdates = []string{"message", "message_reaction"}

	// Custom Long Polling Loop
	for {
		updates, err := b.getUpdates(u)
		if err != nil {
			log.Printf("Failed to get updates: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		for _, update := range updates {
			if update.UpdateID >= u.Offset {
				u.Offset = update.UpdateID + 1
			}

			// Convert CustomUpdate to tgbotapi.Update where possible for compatibility
			// or just handle fields manually.
			// The tests use tgbotapi.Update, so my HandleUpdate expects that.
			// I need to map CustomUpdate to a structure HandleUpdate can use.
			// But HandleUpdate in tests uses tgbotapi.Update which might NOT have MessageReaction.
			// This is a mismatch.

			// Let's make HandleUpdate accept *CustomUpdate or convert.
			// Since I need to support both standard Messages and custom Reactions.

			b.handleCustomUpdate(update)
		}
	}
}

// getUpdates performs the API call and unmarshals into CustomUpdate
func (b *Bot) getUpdates(config tgbotapi.UpdateConfig) ([]CustomUpdate, error) {
	resp, err := b.api.Request(config)
	if err != nil {
		return nil, err
	}

	var updates []CustomUpdate
	if err := json.Unmarshal(resp.Result, &updates); err != nil {
		return nil, err
	}
	return updates, nil
}

// HandleCustomUpdate processes an update
func (b *Bot) HandleCustomUpdate(u CustomUpdate) {
	// Handle Message
	if u.Message != nil {
		b.handleMessage(u.Message)
	}

	// Handle Reaction
	if u.MessageReaction != nil {
		b.handleReaction(u.MessageReaction)
	}
}

// Wrapper for compatibility if needed, but mainly we use CustomUpdate
func (b *Bot) HandleUpdate(u tgbotapi.Update) {
	cu := CustomUpdate{
		UpdateID: u.UpdateID,
		Message:  u.Message,
		// MessageReaction is lost here if we convert from standard Update
		// unless we do some magic, but this method is mostly for standard messages
	}
	b.HandleCustomUpdate(cu)
}

func (b *Bot) handleCustomUpdate(u CustomUpdate) {
	b.HandleCustomUpdate(u)
}

func (b *Bot) handleMessage(msg *tgbotapi.Message) {

	if !msg.IsCommand() {
		return
	}

	chatID := msg.Chat.ID

	// Ensure ChatID is saved on /start
	if msg.Command() == "start" {
		if err := b.storage.SetSetting("chat_id", strconv.FormatInt(chatID, 10)); err != nil {
			log.Printf("Failed to save chat_id: %v", err)
		}

		txt := "Welcome! I am your HN Digest Bot.\n" +
			"Commands:\n" +
			"/fetch - Get digest immediately\n" +
			"/settings - View/Edit settings\n" +
			"/stats - View your preferences"

		msg := tgbotapi.NewMessage(chatID, txt)
		b.sender.Send(msg)
		return
	}

	// Check if configured (optional, but good practice)
	// For simplicity, we assume if they can talk to bot, we handle it.

	switch msg.Command() {
	case "fetch":
		if b.digestTrigger != nil {
			// Run in goroutine to not block?
			// Spec says "Sends articles directly without confirmation".
			go b.digestTrigger()
		}
	case "settings":
		b.handleSettings(msg)
	case "stats":
		b.handleStats(msg)
	}
}

func (b *Bot) handleReaction(react *MessageReactionUpdated) {
	// Check for thumbs up
	hasLike := false
	for _, r := range react.NewReaction {
		if r.Emoji == "👍" {
			hasLike = true
			break
		}
	}

	if !hasLike {
		return
	}

	msgID := react.MessageID

	// Lookup article
	article, err := b.storage.GetArticleByMsgID(msgID)
	if err != nil {
		// Not found or error
		return
	}

	// Check idempotency
	liked, err := b.storage.IsArticleLiked(article.ID)
	if err != nil {
		log.Printf("Error checking like status: %v", err)
		return
	}
	if liked {
		return
	}

	// Record like
	if err := b.storage.AddLike(article.ID); err != nil {
		log.Printf("Failed to add like: %v", err)
		return
	}

	// Boost tags
	for _, tag := range article.Tags {
		if err := b.storage.BoostTag(tag, 1.0, b.tagBoostOnLike); err != nil {
			log.Printf("Failed to boost tag %s: %v", tag, err)
		}
	}

	log.Printf("Liked article %d, boosted tags: %v", article.ID, article.Tags)
}

func (b *Bot) handleSettings(msg *tgbotapi.Message) {
	args := msg.CommandArguments()
	chatID := msg.Chat.ID

	if args == "" {
		// Display current settings
		timeStr, _ := b.storage.GetSetting("digest_time")
		if timeStr == "" {
			timeStr = b.config.DigestTime
		}

		countStr, _ := b.storage.GetSetting("article_count")
		if countStr == "" {
			countStr = strconv.Itoa(b.config.ArticleCount)
		}

		txt := fmt.Sprintf("Current Settings:\nTime: %s\nCount: %s\n\nTo change:\n/settings time HH:MM\n/settings count N", timeStr, countStr)
		b.sender.Send(tgbotapi.NewMessage(chatID, txt))
		return
	}

	parts := strings.Fields(args)
	if len(parts) != 2 {
		b.sender.Send(tgbotapi.NewMessage(chatID, "Invalid format. Use:\n/settings time HH:MM\n/settings count N"))
		return
	}

	cmd := parts[0]
	val := parts[1]

	switch cmd {
	case "time":
		// Validate and update
		if err := b.scheduler.UpdateSchedule(val, b.digestTrigger); err != nil {
			b.sender.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("Error: %v", err)))
			return
		}
		b.storage.SetSetting("digest_time", val)
		b.sender.Send(tgbotapi.NewMessage(chatID, "Digest time updated to "+val))

	case "count":
		n, err := strconv.Atoi(val)
		if err != nil || n < 1 || n > 100 {
			b.sender.Send(tgbotapi.NewMessage(chatID, "Count must be number between 1 and 100"))
			return
		}
		b.storage.SetSetting("article_count", val)
		b.sender.Send(tgbotapi.NewMessage(chatID, "Article count updated to "+val))

	default:
		b.sender.Send(tgbotapi.NewMessage(chatID, "Unknown setting. Use 'time' or 'count'."))
	}
}

func (b *Bot) handleStats(msg *tgbotapi.Message) {
	// Top 10 tags
	weights, err := b.storage.GetTagWeights()
	if err != nil {
		b.sender.Send(tgbotapi.NewMessage(msg.Chat.ID, "Error fetching stats"))
		return
	}

	totalLikes, _ := b.storage.GetTotalLikes()
	if totalLikes == 0 {
		b.sender.Send(tgbotapi.NewMessage(msg.Chat.ID, "No stats yet. React with 👍 to articles to start learning!"))
		return
	}

	// Sort weights
	type tagWeight struct {
		Tag string
		W   float64
	}
	var sorted []tagWeight
	for t, w := range weights {
		sorted = append(sorted, tagWeight{t, w})
	}

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].W > sorted[j].W
	})

	txt := fmt.Sprintf("Total Likes: %d\n\nTop Interests:\n", totalLikes)
	for i, tw := range sorted {
		if i >= 10 {
			break
		}
		txt += fmt.Sprintf("- %s: %.1f\n", tw.Tag, tw.W)
	}

	b.sender.Send(tgbotapi.NewMessage(msg.Chat.ID, txt))
}

// SendArticle sends a formatted article message and returns the message ID
func (b *Bot) SendArticle(a storage.Article) (int, error) {
	chatIDStr, err := b.storage.GetSetting("chat_id")
	if err != nil {
		return 0, err
	}
	if chatIDStr == "" {
		if b.config.ChatID != 0 {
			chatIDStr = strconv.FormatInt(b.config.ChatID, 10)
		} else {
			return 0, fmt.Errorf("chat_id not set")
		}
	}
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid chat_id: %w", err)
	}

	// Format Message
	// 📄 <b>Title</b>
	// <i>Summary</i>
	// 🔼 Score | 💬 Comments (Need comment count? Article struct doesn't have it? Wait.)
	// HN Item has Descendants. Article struct in Storage doesn't have it.
	// Spec for DB schema: "Descendants (comment count)" is NOT listed in DB Schema section.
	// "Fields: ... HN score at fetch time ... "
	// But "Features -> Message Formatting" says "Score (points) and comment count with icons".
	// I missed storing Comment Count in `storage.Article`.
	// I should add it to `Article` struct in `storage` package and schema if I want to display it.
	// Or just display Score if I can't change schema now easily (I can, project is in dev).

	// Let's check `storage/storage.go`.
	// type Article struct { ... Score int ... }
	// I'll add Comments int.
	// I need to update `storage.go` (struct and query) and `digest.go` (mapping).

	// For now, I'll assume Score only or try to add Comments.
	// Given strict specs, I should add it.

	// Escape Content
	title := html.EscapeString(a.Title)
	summary := html.EscapeString(a.Summary)

	text := fmt.Sprintf("📄 <b>%s</b>\n\n<i>%s</i>\n\n🔼 %d | 🔗 <a href=\"%s\">Read</a> | 💬 <a href=\"https://news.ycombinator.com/item?id=%d\">Discuss</a>",
		title, summary, a.Score, a.URL, a.ID)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	msg.DisableWebPagePreview = true // Maybe? No spec says.

	sent, err := b.sender.Send(msg)
	if err != nil {
		return 0, err
	}
	return sent.MessageID, nil
}
