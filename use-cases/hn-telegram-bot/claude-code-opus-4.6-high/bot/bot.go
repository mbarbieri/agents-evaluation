package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Bot handles Telegram updates via long polling with reaction support.
type Bot struct {
	token   string
	client  *http.Client
	baseURL string

	mu           sync.RWMutex
	chatID       int64
	digestTime   string
	articleCount int

	sender          MessageSender
	articleLookup   ArticleLookup
	likeTracker     LikeTracker
	tagBooster      TagBooster
	settingsStore   SettingsStore
	statsProvider   StatsProvider
	scheduleUpdater ScheduleUpdater
	digestFunc      func()

	tagBoostAmount float64
}

// Config holds Bot configuration.
type Config struct {
	Token          string
	ChatID         int64
	DigestTime     string
	ArticleCount   int
	TagBoostAmount float64
	BaseURL        string // Override for testing; empty means default
}

// Deps holds all injectable dependencies for the Bot.
type Deps struct {
	Sender          MessageSender
	ArticleLookup  ArticleLookup
	LikeTracker    LikeTracker
	TagBooster     TagBooster
	SettingsStore  SettingsStore
	StatsProvider  StatsProvider
	ScheduleUpdater ScheduleUpdater
	DigestFunc     func()
}

// New creates a Bot with the given configuration and dependencies.
func New(cfg Config, deps Deps) *Bot {
	base := "https://api.telegram.org"
	if cfg.BaseURL != "" {
		base = cfg.BaseURL
	}
	b := &Bot{
		token:           cfg.Token,
		client:          &http.Client{Timeout: 30 * time.Second},
		baseURL:         base,
		chatID:          cfg.ChatID,
		digestTime:      cfg.DigestTime,
		articleCount:    cfg.ArticleCount,
		tagBoostAmount:  cfg.TagBoostAmount,
		sender:          deps.Sender,
		articleLookup:   deps.ArticleLookup,
		likeTracker:     deps.LikeTracker,
		tagBooster:      deps.TagBooster,
		settingsStore:   deps.SettingsStore,
		statsProvider:   deps.StatsProvider,
		scheduleUpdater: deps.ScheduleUpdater,
		digestFunc:      deps.DigestFunc,
	}
	if b.sender == nil {
		b.sender = b
	}
	return b
}

// SendHTML sends an HTML-formatted message to a chat and returns the message ID.
func (b *Bot) SendHTML(chatID int64, text string) (int, error) {
	params := url.Values{}
	params.Set("chat_id", strconv.FormatInt(chatID, 10))
	params.Set("text", text)
	params.Set("parse_mode", "HTML")

	apiURL := fmt.Sprintf("%s/bot%s/sendMessage", b.baseURL, b.token)
	resp, err := b.client.PostForm(apiURL, params)
	if err != nil {
		return 0, fmt.Errorf("sending message: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("Telegram API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		OK     bool `json:"ok"`
		Result struct {
			MessageID int `json:"message_id"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("parsing send response: %w", err)
	}

	return result.Result.MessageID, nil
}

// GetChatID returns the current chat ID.
func (b *Bot) GetChatID() int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.chatID
}

// GetDigestTime returns the current digest time.
func (b *Bot) GetDigestTime() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.digestTime
}

// GetArticleCount returns the current article count.
func (b *Bot) GetArticleCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.articleCount
}

// Run starts the long polling loop. Blocks until context is canceled.
func (b *Bot) Run(ctx context.Context) error {
	offset := 0

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		updates, newOffset, err := b.getUpdates(ctx, offset)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			slog.Error("failed to get updates", "error", err)
			time.Sleep(5 * time.Second)
			continue
		}

		for _, update := range updates {
			b.handleUpdate(update)
		}

		if newOffset > offset {
			offset = newOffset
		}
	}
}

// telegramUpdate represents an incoming Telegram update.
type telegramUpdate struct {
	UpdateID int `json:"update_id"`
	Message  *struct {
		MessageID int `json:"message_id"`
		Chat      struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		Text string `json:"text"`
		From *struct {
			ID int64 `json:"id"`
		} `json:"from"`
	} `json:"message"`
	MessageReaction *struct {
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		MessageID    int `json:"message_id"`
		NewReactions []struct {
			Type  string `json:"type"`
			Emoji string `json:"emoji"`
		} `json:"new_reaction"`
	} `json:"message_reaction"`
}

func (b *Bot) getUpdates(ctx context.Context, offset int) ([]telegramUpdate, int, error) {
	params := url.Values{}
	params.Set("offset", strconv.Itoa(offset))
	params.Set("timeout", "30")
	params.Set("allowed_updates", `["message","message_reaction"]`)

	apiURL := fmt.Sprintf("%s/bot%s/getUpdates?%s", b.baseURL, b.token, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, offset, err
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, offset, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		OK     bool             `json:"ok"`
		Result []telegramUpdate `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, offset, fmt.Errorf("parsing updates: %w", err)
	}

	newOffset := offset
	for _, u := range result.Result {
		if u.UpdateID >= newOffset {
			newOffset = u.UpdateID + 1
		}
	}

	return result.Result, newOffset, nil
}

func (b *Bot) handleUpdate(update telegramUpdate) {
	if update.Message != nil {
		b.handleMessage(update.Message.Chat.ID, update.Message.Text)
	}
	if update.MessageReaction != nil {
		b.handleReaction(update.MessageReaction)
	}
}

func (b *Bot) handleMessage(chatID int64, text string) {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return
	}

	command := parts[0]
	args := parts[1:]

	switch command {
	case "/start":
		b.handleStart(chatID)
	case "/fetch":
		b.handleFetch(chatID)
	case "/settings":
		b.handleSettings(chatID, args)
	case "/stats":
		b.handleStats(chatID)
	}
}

func (b *Bot) handleStart(chatID int64) {
	b.mu.Lock()
	b.chatID = chatID
	b.mu.Unlock()

	if b.settingsStore != nil {
		if err := b.settingsStore.SetSetting("chat_id", strconv.FormatInt(chatID, 10)); err != nil {
			slog.Error("failed to save chat_id", "error", err)
		}
	}

	msg := "Welcome to HN Digest Bot!\n\n" +
		"Available commands:\n" +
		"/fetch - Get your personalized digest now\n" +
		"/settings - View or update digest settings\n" +
		"/stats - View your interest statistics"

	if _, err := b.sender.SendHTML(chatID, msg); err != nil {
		slog.Error("failed to send welcome message", "error", err)
	}
	slog.Info("user registered", "chat_id", chatID)
}

func (b *Bot) handleFetch(chatID int64) {
	if b.digestFunc != nil {
		b.digestFunc()
	}
}

func (b *Bot) handleSettings(chatID int64, args []string) {
	if len(args) == 0 {
		b.mu.RLock()
		digestTime := b.digestTime
		articleCount := b.articleCount
		b.mu.RUnlock()

		msg := fmt.Sprintf("Current settings:\n\nDigest time: %s\nArticle count: %d", digestTime, articleCount)
		if _, err := b.sender.SendHTML(chatID, msg); err != nil {
			slog.Error("failed to send settings", "error", err)
		}
		return
	}

	if len(args) < 2 {
		b.sendSettingsUsage(chatID)
		return
	}

	switch args[0] {
	case "time":
		b.handleSettingsTime(chatID, args[1])
	case "count":
		b.handleSettingsCount(chatID, args[1])
	default:
		b.sendSettingsUsage(chatID)
	}
}

func (b *Bot) sendSettingsUsage(chatID int64) {
	msg := "Usage:\n/settings time HH:MM\n/settings count N (1-100)"
	if _, err := b.sender.SendHTML(chatID, msg); err != nil {
		slog.Error("failed to send settings usage", "error", err)
	}
}

func (b *Bot) handleSettingsTime(chatID int64, timeStr string) {
	if len(timeStr) != 5 || timeStr[2] != ':' {
		b.sendSettingsUsage(chatID)
		return
	}

	hour, err1 := strconv.Atoi(timeStr[:2])
	minute, err2 := strconv.Atoi(timeStr[3:])
	if err1 != nil || err2 != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		b.sendSettingsUsage(chatID)
		return
	}

	b.mu.Lock()
	b.digestTime = timeStr
	b.mu.Unlock()

	if b.settingsStore != nil {
		if err := b.settingsStore.SetSetting("digest_time", timeStr); err != nil {
			slog.Error("failed to save digest_time", "error", err)
		}
	}

	if b.scheduleUpdater != nil && b.digestFunc != nil {
		if err := b.scheduleUpdater.Schedule(timeStr, b.digestFunc); err != nil {
			slog.Error("failed to update schedule", "error", err)
		}
	}

	msg := fmt.Sprintf("Digest time updated to %s", timeStr)
	if _, err := b.sender.SendHTML(chatID, msg); err != nil {
		slog.Error("failed to send time update confirmation", "error", err)
	}
	slog.Info("digest time updated", "time", timeStr)
}

func (b *Bot) handleSettingsCount(chatID int64, countStr string) {
	count, err := strconv.Atoi(countStr)
	if err != nil || count < 1 || count > 100 {
		b.sendSettingsUsage(chatID)
		return
	}

	b.mu.Lock()
	b.articleCount = count
	b.mu.Unlock()

	if b.settingsStore != nil {
		if err := b.settingsStore.SetSetting("article_count", strconv.Itoa(count)); err != nil {
			slog.Error("failed to save article_count", "error", err)
		}
	}

	msg := fmt.Sprintf("Article count updated to %d", count)
	if _, err := b.sender.SendHTML(chatID, msg); err != nil {
		slog.Error("failed to send count update confirmation", "error", err)
	}
	slog.Info("article count updated", "count", count)
}

func (b *Bot) handleStats(chatID int64) {
	if b.statsProvider == nil {
		return
	}

	likeCount, err := b.statsProvider.GetLikeCount()
	if err != nil {
		slog.Error("failed to get like count", "error", err)
		return
	}

	if likeCount == 0 {
		msg := "No preferences learned yet. React with 👍 to articles you like to train your digest!"
		if _, err := b.sender.SendHTML(chatID, msg); err != nil {
			slog.Error("failed to send stats message", "error", err)
		}
		return
	}

	topTags, err := b.statsProvider.GetTopTagWeights(10)
	if err != nil {
		slog.Error("failed to get top tags", "error", err)
		return
	}

	var sb strings.Builder
	sb.WriteString("Your interests:\n\n")
	for _, tw := range topTags {
		sb.WriteString(fmt.Sprintf("• %s (%.2f)\n", tw.Tag, tw.Weight))
	}
	sb.WriteString(fmt.Sprintf("\nTotal likes: %d", likeCount))

	if _, err := b.sender.SendHTML(chatID, sb.String()); err != nil {
		slog.Error("failed to send stats", "error", err)
	}
}

func (b *Bot) handleReaction(reaction *struct {
	Chat struct {
		ID int64 `json:"id"`
	} `json:"chat"`
	MessageID    int `json:"message_id"`
	NewReactions []struct {
		Type  string `json:"type"`
		Emoji string `json:"emoji"`
	} `json:"new_reaction"`
}) {
	// Only process thumbs-up reactions
	hasThumbsUp := false
	for _, r := range reaction.NewReactions {
		if r.Emoji == "👍" {
			hasThumbsUp = true
			break
		}
	}
	if !hasThumbsUp {
		return
	}

	if b.articleLookup == nil || b.likeTracker == nil || b.tagBooster == nil {
		return
	}

	article, err := b.articleLookup.GetArticleBySentMsgID(reaction.MessageID)
	if err != nil {
		slog.Error("failed to look up article", "msg_id", reaction.MessageID, "error", err)
		return
	}
	if article == nil {
		return
	}

	liked, err := b.likeTracker.IsLiked(article.ID)
	if err != nil {
		slog.Error("failed to check like status", "article_id", article.ID, "error", err)
		return
	}
	if liked {
		return
	}

	if err := b.likeTracker.RecordLike(article.ID); err != nil {
		slog.Error("failed to record like", "article_id", article.ID, "error", err)
		return
	}

	// Parse tags and boost weights
	var tags []string
	if article.Tags != "" {
		if err := json.Unmarshal([]byte(article.Tags), &tags); err != nil {
			slog.Error("failed to parse article tags", "article_id", article.ID, "error", err)
			return
		}
	}

	for _, tag := range tags {
		existing, err := b.tagBooster.GetTagWeight(tag)
		if err != nil {
			slog.Error("failed to get tag weight", "tag", tag, "error", err)
			continue
		}

		if existing == nil {
			if err := b.tagBooster.UpsertTagWeight(tag, 1.0+b.tagBoostAmount, 1); err != nil {
				slog.Error("failed to create tag weight", "tag", tag, "error", err)
			}
		} else {
			if err := b.tagBooster.UpsertTagWeight(tag, existing.Weight+b.tagBoostAmount, existing.Count+1); err != nil {
				slog.Error("failed to boost tag weight", "tag", tag, "error", err)
			}
		}
	}

	slog.Info("article liked", "article_id", article.ID, "msg_id", reaction.MessageID, "tags_boosted", tags)
}
