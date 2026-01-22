package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"hn-telegram-bot/bot"
	"hn-telegram-bot/config"
	"hn-telegram-bot/digest"
	"hn-telegram-bot/hn"
	"hn-telegram-bot/ranker"
	"hn-telegram-bot/scheduler"
	"hn-telegram-bot/scraper"
	"hn-telegram-bot/storage"
	"hn-telegram-bot/summarizer"
)

func main() {
	// Set up structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	slog.Info("starting HN Telegram Bot")

	// Load configuration
	configPath := config.GetConfigPath()
	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Error("failed to load config", "path", configPath, "error", err)
		os.Exit(1)
	}
	slog.Info("config loaded", "path", configPath)

	// Initialize database
	db, err := storage.NewDB(cfg.DBPath)
	if err != nil {
		slog.Error("failed to initialize database", "path", cfg.DBPath, "error", err)
		os.Exit(1)
	}
	defer db.Close()
	slog.Info("database initialized", "path", cfg.DBPath)

	// Initialize Telegram bot
	tgBot, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		slog.Error("failed to initialize Telegram bot", "error", err)
		os.Exit(1)
	}
	slog.Info("telegram bot initialized", "username", tgBot.Self.UserName)

	// Initialize components
	hnClient := hn.NewClient(
		hn.WithTimeout(time.Duration(cfg.FetchTimeoutSecs) * time.Second),
	)
	articleScraper := scraper.NewScraper(
		scraper.WithTimeout(time.Duration(cfg.FetchTimeoutSecs) * time.Second),
	)
	articleSummarizer := summarizer.NewSummarizer(
		cfg.GeminiAPIKey,
		summarizer.WithModel(cfg.GeminiModel),
	)

	// Initialize scheduler
	sched, err := scheduler.NewScheduler(cfg.Timezone)
	if err != nil {
		slog.Error("failed to initialize scheduler", "timezone", cfg.Timezone, "error", err)
		os.Exit(1)
	}

	// Create app instance
	app := &App{
		cfg:        cfg,
		db:         db,
		tgBot:      tgBot,
		hnClient:   hnClient,
		scraper:    articleScraper,
		summarizer: articleSummarizer,
		scheduler:  sched,
	}

	// Initialize chat ID from config or database
	if cfg.ChatID != 0 {
		app.chatID = cfg.ChatID
	} else if chatIDStr, err := db.GetSetting(context.Background(), "chat_id"); err == nil {
		if id, err := strconv.ParseInt(chatIDStr, 10, 64); err == nil {
			app.chatID = id
		}
	}

	// Set up context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		slog.Info("received shutdown signal", "signal", sig)
		cancel()
	}()

	// Schedule daily digest
	digestTime := cfg.DigestTime
	if storedTime, err := db.GetSetting(ctx, "digest_time"); err == nil {
		digestTime = storedTime
	}

	if err := sched.Schedule(digestTime, func() {
		app.runDigest(context.Background())
	}); err != nil {
		slog.Error("failed to schedule digest", "error", err)
		os.Exit(1)
	}
	sched.Start()
	defer sched.Stop()
	slog.Info("digest scheduled", "time", digestTime, "timezone", cfg.Timezone)

	// Run the bot
	slog.Info("starting bot polling")
	app.run(ctx)
	slog.Info("bot stopped")
}

// App holds all application dependencies.
type App struct {
	cfg        *config.Config
	db         *storage.DB
	tgBot      *tgbotapi.BotAPI
	hnClient   *hn.Client
	scraper    *scraper.Scraper
	summarizer *summarizer.Summarizer
	scheduler  *scheduler.Scheduler
	chatID     int64
	mu         sync.RWMutex
}

func (a *App) run(ctx context.Context) {
	// Use manual getUpdates to support message reactions
	offset := 0
	timeout := 30

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		updates, err := a.getUpdates(ctx, offset, timeout)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Warn("failed to get updates", "error", err)
			time.Sleep(time.Second)
			continue
		}

		for _, update := range updates {
			offset = update.UpdateID + 1
			a.handleUpdate(ctx, &update)
		}
	}
}

// Update represents a Telegram update with reaction support.
type Update struct {
	UpdateID        int              `json:"update_id"`
	Message         *tgbotapi.Message `json:"message"`
	MessageReaction *MessageReaction `json:"message_reaction"`
}

// MessageReaction represents a reaction update from Telegram.
type MessageReaction struct {
	Chat        Chat              `json:"chat"`
	MessageID   int               `json:"message_id"`
	Date        int               `json:"date"`
	OldReaction []ReactionType    `json:"old_reaction"`
	NewReaction []ReactionType    `json:"new_reaction"`
}

// Chat represents a Telegram chat.
type Chat struct {
	ID int64 `json:"id"`
}

// ReactionType represents a reaction emoji.
type ReactionType struct {
	Type  string `json:"type"`
	Emoji string `json:"emoji"`
}

func (a *App) getUpdates(ctx context.Context, offset, timeout int) ([]Update, error) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?offset=%d&timeout=%d&allowed_updates=%s",
		a.cfg.TelegramToken, offset, timeout, `["message","message_reaction"]`)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: time.Duration(timeout+10) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		OK     bool     `json:"ok"`
		Result []Update `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if !result.OK {
		return nil, fmt.Errorf("telegram API returned not OK")
	}

	return result.Result, nil
}

func (a *App) handleUpdate(ctx context.Context, update *Update) {
	if update.Message != nil {
		a.handleMessage(ctx, update.Message)
	}
	if update.MessageReaction != nil {
		a.handleReaction(ctx, update.MessageReaction)
	}
}

func (a *App) handleMessage(ctx context.Context, msg *tgbotapi.Message) {
	if msg.Text == "" {
		return
	}

	text := strings.TrimSpace(msg.Text)
	chatID := msg.Chat.ID

	slog.Info("received message", "chat_id", chatID, "text", text)

	switch {
	case text == "/start":
		a.handleStartCommand(ctx, chatID)
	case text == "/fetch":
		a.handleFetchCommand(ctx, chatID)
	case text == "/stats":
		a.handleStatsCommand(ctx, chatID)
	case strings.HasPrefix(text, "/settings"):
		args := strings.TrimPrefix(text, "/settings")
		a.handleSettingsCommand(ctx, chatID, strings.TrimSpace(args))
	}
}

func (a *App) handleStartCommand(ctx context.Context, chatID int64) {
	// Save chat ID
	a.mu.Lock()
	a.chatID = chatID
	a.mu.Unlock()

	if err := a.db.SetSetting(ctx, "chat_id", strconv.FormatInt(chatID, 10)); err != nil {
		slog.Warn("failed to save chat_id", "error", err)
	}

	msg := "Welcome to the HN Digest Bot! 🗞️\n\n" +
		"Commands:\n" +
		"/fetch - Get your personalized digest now\n" +
		"/settings - View or update digest settings\n" +
		"/stats - View your interests and stats\n\n" +
		"React with 👍 to articles you like to train your preferences!"

	a.sendMessage(ctx, chatID, msg, false)
}

func (a *App) handleFetchCommand(ctx context.Context, chatID int64) {
	a.mu.Lock()
	a.chatID = chatID
	a.mu.Unlock()

	go a.runDigest(ctx)
}

func (a *App) handleStatsCommand(ctx context.Context, chatID int64) {
	likeCount, err := a.db.GetLikeCount(ctx)
	if err != nil {
		slog.Warn("failed to get like count", "error", err)
		a.sendMessage(ctx, chatID, "Failed to retrieve stats.", false)
		return
	}

	if likeCount == 0 {
		a.sendMessage(ctx, chatID, "No likes yet! React with 👍 to articles to train your preferences.", false)
		return
	}

	topTags, err := a.db.GetTopTags(ctx, 10)
	if err != nil {
		slog.Warn("failed to get top tags", "error", err)
		a.sendMessage(ctx, chatID, "Failed to retrieve stats.", false)
		return
	}

	var sb strings.Builder
	sb.WriteString("📊 Your Interests:\n\n")
	for i, tag := range topTags {
		sb.WriteString(fmt.Sprintf("%d. %s (%.2f)\n", i+1, tag.Tag, tag.Weight))
	}
	sb.WriteString(fmt.Sprintf("\nTotal articles liked: %d", likeCount))

	a.sendMessage(ctx, chatID, sb.String(), false)
}

func (a *App) handleSettingsCommand(ctx context.Context, chatID int64, args string) {
	if args == "" {
		// Display current settings
		digestTime := a.cfg.DigestTime
		if storedTime, err := a.db.GetSetting(ctx, "digest_time"); err == nil {
			digestTime = storedTime
		}

		articleCount := a.cfg.ArticleCount
		if storedCount, err := a.db.GetSetting(ctx, "article_count"); err == nil {
			if n, err := strconv.Atoi(storedCount); err == nil {
				articleCount = n
			}
		}

		msg := fmt.Sprintf("Current Settings:\n\n"+
			"📅 Digest Time: %s\n"+
			"📰 Articles per Digest: %d\n\n"+
			"Update with:\n"+
			"/settings time HH:MM\n"+
			"/settings count N", digestTime, articleCount)

		a.sendMessage(ctx, chatID, msg, false)
		return
	}

	parts := strings.SplitN(args, " ", 2)
	if len(parts) < 2 {
		a.sendMessage(ctx, chatID, "Usage:\n/settings time HH:MM\n/settings count N", false)
		return
	}

	subCmd := strings.ToLower(parts[0])
	value := strings.TrimSpace(parts[1])

	switch subCmd {
	case "time":
		if !isValidTime(value) {
			a.sendMessage(ctx, chatID, "Invalid time format. Use HH:MM (e.g., 09:00, 18:30)", false)
			return
		}

		if err := a.db.SetSetting(ctx, "digest_time", value); err != nil {
			slog.Warn("failed to save digest_time", "error", err)
			a.sendMessage(ctx, chatID, "Failed to update settings.", false)
			return
		}

		// Update scheduler
		if err := a.scheduler.Schedule(value, func() {
			a.runDigest(context.Background())
		}); err != nil {
			slog.Warn("failed to reschedule digest", "error", err)
		}

		a.sendMessage(ctx, chatID, fmt.Sprintf("✅ Digest time updated to %s", value), false)

	case "count":
		count, err := strconv.Atoi(value)
		if err != nil || count < 1 || count > 100 {
			a.sendMessage(ctx, chatID, "Invalid count. Must be a number between 1 and 100.", false)
			return
		}

		if err := a.db.SetSetting(ctx, "article_count", value); err != nil {
			slog.Warn("failed to save article_count", "error", err)
			a.sendMessage(ctx, chatID, "Failed to update settings.", false)
			return
		}

		a.sendMessage(ctx, chatID, fmt.Sprintf("✅ Article count updated to %d", count), false)

	default:
		a.sendMessage(ctx, chatID, "Usage:\n/settings time HH:MM\n/settings count N", false)
	}
}

func (a *App) handleReaction(ctx context.Context, reaction *MessageReaction) {
	// Check for new thumbs-up reaction
	var hasThumbsUp bool
	for _, r := range reaction.NewReaction {
		if r.Emoji == "👍" {
			hasThumbsUp = true
			break
		}
	}

	// Check if it was already there
	for _, r := range reaction.OldReaction {
		if r.Emoji == "👍" {
			hasThumbsUp = false // It was already there, not a new reaction
			break
		}
	}

	if !hasThumbsUp {
		return
	}

	msgID := int64(reaction.MessageID)
	slog.Info("received thumbs-up reaction", "message_id", msgID)

	// Look up article
	article, err := a.db.GetArticleByMessageID(ctx, msgID)
	if err != nil {
		if err != storage.ErrNotFound {
			slog.Warn("failed to lookup article by message ID", "message_id", msgID, "error", err)
		}
		return // Not an article message or error
	}

	// Check if already liked
	liked, err := a.db.IsArticleLiked(ctx, article.ID)
	if err != nil {
		slog.Warn("failed to check if article liked", "article_id", article.ID, "error", err)
		return
	}
	if liked {
		return // Already liked, idempotent
	}

	// Record like
	if err := a.db.LikeArticle(ctx, article.ID); err != nil {
		slog.Warn("failed to record like", "article_id", article.ID, "error", err)
		return
	}

	// Boost tags
	for _, tag := range article.Tags {
		if err := a.db.BoostTagWeight(ctx, tag, a.cfg.TagBoostOnLike); err != nil {
			slog.Warn("failed to boost tag", "tag", tag, "error", err)
		}
	}

	slog.Info("processed like", "article_id", article.ID, "tags", article.Tags)
}

func (a *App) runDigest(ctx context.Context) {
	a.mu.RLock()
	chatID := a.chatID
	a.mu.RUnlock()

	if chatID == 0 {
		slog.Warn("cannot run digest: no chat_id set")
		return
	}

	// Get article count from settings
	articleCount := a.cfg.ArticleCount
	if storedCount, err := a.db.GetSetting(ctx, "article_count"); err == nil {
		if n, err := strconv.Atoi(storedCount); err == nil {
			articleCount = n
		}
	}

	// Create digest runner
	runner := digest.NewRunner(
		&hnClientAdapter{a.hnClient},
		&scraperAdapter{a.scraper},
		&summarizerAdapter{a.summarizer},
		&storageAdapter{a.db},
		&articleSenderAdapter{a},
		digest.WithChatID(chatID),
		digest.WithArticleCount(articleCount),
		digest.WithDecayRate(a.cfg.TagDecayRate),
		digest.WithMinTagWeight(a.cfg.MinTagWeight),
	)

	if err := runner.Run(ctx); err != nil {
		slog.Error("digest run failed", "error", err)
	}
}

func (a *App) sendMessage(ctx context.Context, chatID int64, text string, html bool) (int64, error) {
	msg := tgbotapi.NewMessage(chatID, text)
	if html {
		msg.ParseMode = tgbotapi.ModeHTML
	}

	sent, err := a.tgBot.Send(msg)
	if err != nil {
		slog.Warn("failed to send message", "chat_id", chatID, "error", err)
		return 0, err
	}
	return int64(sent.MessageID), nil
}

func isValidTime(s string) bool {
	if len(s) != 5 || s[2] != ':' {
		return false
	}
	hour, err1 := strconv.Atoi(s[:2])
	minute, err2 := strconv.Atoi(s[3:])
	return err1 == nil && err2 == nil && hour >= 0 && hour <= 23 && minute >= 0 && minute <= 59
}

// Adapter types to bridge between our interfaces and the digest package interfaces

type hnClientAdapter struct {
	client *hn.Client
}

func (h *hnClientAdapter) GetTopStories(ctx context.Context, limit int) ([]int64, error) {
	return h.client.GetTopStories(ctx, limit)
}

func (h *hnClientAdapter) GetItem(ctx context.Context, id int64) (*digest.HNItem, error) {
	item, err := h.client.GetItem(ctx, id)
	if err != nil {
		return nil, err
	}
	return &digest.HNItem{
		ID:          item.ID,
		Title:       item.Title,
		URL:         item.URL,
		Score:       item.Score,
		Descendants: item.Descendants,
	}, nil
}

type scraperAdapter struct {
	scraper *scraper.Scraper
}

func (s *scraperAdapter) Scrape(ctx context.Context, url string) (string, error) {
	return s.scraper.Scrape(ctx, url)
}

type summarizerAdapter struct {
	summarizer *summarizer.Summarizer
}

func (s *summarizerAdapter) Summarize(ctx context.Context, title, content string) (*digest.SummaryResult, error) {
	result, err := s.summarizer.Summarize(ctx, title, content)
	if err != nil {
		return nil, err
	}
	return &digest.SummaryResult{
		Summary: result.Summary,
		Tags:    result.Tags,
	}, nil
}

type storageAdapter struct {
	db *storage.DB
}

func (s *storageAdapter) GetRecentlySentArticleIDs(ctx context.Context, within time.Duration) ([]int64, error) {
	return s.db.GetRecentlySentArticleIDs(ctx, within)
}

func (s *storageAdapter) GetAllTagWeights(ctx context.Context) (map[string]float64, error) {
	return s.db.GetAllTagWeights(ctx)
}

func (s *storageAdapter) ApplyTagDecay(ctx context.Context, decayRate, minWeight float64) error {
	return s.db.ApplyTagDecay(ctx, decayRate, minWeight)
}

func (s *storageAdapter) SaveArticle(ctx context.Context, article *digest.StoredArticle) error {
	return s.db.SaveArticle(ctx, &storage.Article{
		ID:            article.ID,
		Title:         article.Title,
		URL:           article.URL,
		Summary:       article.Summary,
		Tags:          article.Tags,
		HNScore:       article.HNScore,
		FetchedAt:     article.FetchedAt,
		SentAt:        article.SentAt,
		TelegramMsgID: article.TelegramMsgID,
	})
}

func (s *storageAdapter) MarkArticleSent(ctx context.Context, articleID int64, telegramMsgID int64) error {
	return s.db.MarkArticleSent(ctx, articleID, telegramMsgID)
}

func (s *storageAdapter) GetSetting(ctx context.Context, key string) (string, error) {
	return s.db.GetSetting(ctx, key)
}

type articleSenderAdapter struct {
	app *App
}

func (a *articleSenderAdapter) SendArticle(ctx context.Context, chatID int64, article *digest.ArticleToSend) (int64, error) {
	msg := bot.FormatArticleMessage(&bot.ArticleForDisplay{
		ID:       article.ID,
		Title:    article.Title,
		Summary:  article.Summary,
		HNScore:  article.HNScore,
		Comments: article.Comments,
		URL:      article.URL,
	})
	return a.app.sendMessage(ctx, chatID, msg, true)
}

// Ensure ranker package is used (it's used internally by digest)
var _ = ranker.NewRanker
