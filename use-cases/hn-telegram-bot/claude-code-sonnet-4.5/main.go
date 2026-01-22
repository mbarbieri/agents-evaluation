package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"

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

// Adapter types to bridge between package-specific interfaces

type hnItemAdapter struct {
	item *hn.Item
}

func (a *hnItemAdapter) GetID() int          { return a.item.ID }
func (a *hnItemAdapter) GetTitle() string    { return a.item.Title }
func (a *hnItemAdapter) GetURL() string      { return a.item.URL }
func (a *hnItemAdapter) GetScore() int       { return a.item.Score }
func (a *hnItemAdapter) GetDescendants() int { return a.item.Descendants }

type hnClientAdapter struct {
	client *hn.HTTPClient
}

func (a *hnClientAdapter) GetTopStories() ([]int, error) {
	return a.client.GetTopStories()
}

func (a *hnClientAdapter) GetItem(id int) (digest.HNItem, error) {
	item, err := a.client.GetItem(id)
	if err != nil {
		return nil, err
	}
	return &hnItemAdapter{item: item}, nil
}

type summaryAdapter struct {
	result *summarizer.SummaryResult
}

func (a *summaryAdapter) GetSummary() string { return a.result.Summary }
func (a *summaryAdapter) GetTags() []string  { return a.result.Tags }

type summarizerAdapter struct {
	summarizer *summarizer.GeminiSummarizer
}

func (a *summarizerAdapter) Summarize(title, content string) (digest.Summary, error) {
	result, err := a.summarizer.Summarize(title, content)
	if err != nil {
		return nil, err
	}
	return &summaryAdapter{result: result}, nil
}

type rankerAdapter struct {
	ranker *ranker.WeightedRanker
}

func (a *rankerAdapter) Rank(articles []digest.RankerArticle, tagWeights map[string]float64) []digest.RankedArticle {
	// Convert digest.RankerArticle to ranker.Article
	var rankerArticles []ranker.Article
	for _, art := range articles {
		rankerArticles = append(rankerArticles, ranker.Article{
			ID:      art.ID,
			Tags:    art.Tags,
			HNScore: art.HNScore,
		})
	}

	ranked := a.ranker.Rank(rankerArticles, tagWeights)

	// Convert back to digest.RankedArticle
	var result []digest.RankedArticle
	for _, r := range ranked {
		result = append(result, digest.RankedArticle{
			Article: digest.RankerArticle{
				ID:      r.Article.ID,
				Tags:    r.Article.Tags,
				HNScore: r.Article.HNScore,
			},
			Score: r.Score,
		})
	}

	return result
}

type storageAdapter struct {
	storage *storage.SQLiteStorage
}

func (a *storageAdapter) ApplyDecay(decayRate, minWeight float64) error {
	return a.storage.ApplyDecay(decayRate, minWeight)
}

func (a *storageAdapter) GetRecentlySentArticles(days int) ([]int, error) {
	return a.storage.GetRecentlySentArticles(days)
}

func (a *storageAdapter) GetAllTagWeights() ([]digest.TagWeight, error) {
	weights, err := a.storage.GetAllTagWeights()
	if err != nil {
		return nil, err
	}

	var result []digest.TagWeight
	for _, w := range weights {
		result = append(result, digest.TagWeight{
			Tag:    w.Tag,
			Weight: w.Weight,
		})
	}

	return result, nil
}

func (a *storageAdapter) SaveArticle(article *digest.StorageArticle) error {
	storageArticle := &storage.Article{
		ID:            article.ID,
		Title:         article.Title,
		URL:           article.URL,
		Summary:       article.Summary,
		Tags:          article.Tags,
		HNScore:       article.HNScore,
		FetchedAt:     article.FetchedAt,
		SentAt:        article.SentAt,
		TelegramMsgID: article.TelegramMsgID,
	}
	return a.storage.SaveArticle(storageArticle)
}

func (a *storageAdapter) GetSetting(key string) (string, error) {
	return a.storage.GetSetting(key)
}

func (a *storageAdapter) GetArticleByTelegramMsgID(msgID int) (bot.Article, error) {
	article, err := a.storage.GetArticleByTelegramMsgID(msgID)
	if err != nil {
		return bot.Article{}, err
	}

	return bot.Article{
		ID:   article.ID,
		Tags: article.Tags,
	}, nil
}

func (a *storageAdapter) IsLiked(articleID int) (bool, error) {
	return a.storage.IsLiked(articleID)
}

func (a *storageAdapter) RecordLike(articleID int) error {
	return a.storage.RecordLike(articleID)
}

func (a *storageAdapter) GetTagWeight(tag string) (float64, error) {
	return a.storage.GetTagWeight(tag)
}

func (a *storageAdapter) SetTagWeight(tag string, weight float64) error {
	return a.storage.SetTagWeight(tag, weight)
}

func (a *storageAdapter) IncrementTagOccurrence(tag string) error {
	return a.storage.IncrementTagOccurrence(tag)
}

func (a *storageAdapter) SetSetting(key, value string) error {
	return a.storage.SetSetting(key, value)
}

func (a *storageAdapter) GetTopTags(limit int) ([]bot.TagWeight, error) {
	tags, err := a.storage.GetTopTags(limit)
	if err != nil {
		return nil, err
	}

	var result []bot.TagWeight
	for _, t := range tags {
		result = append(result, bot.TagWeight{
			Tag:    t.Tag,
			Weight: t.Weight,
		})
	}

	return result, nil
}

func (a *storageAdapter) GetLikeCount() (int, error) {
	return a.storage.GetLikeCount()
}

func main() {
	// Load configuration
	configPath := os.Getenv("HN_BOT_CONFIG")
	if configPath == "" {
		configPath = "./config.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	// Initialize logging
	logLevel := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	slog.Info("Starting HN Telegram Bot")

	// Initialize storage
	db, err := storage.New(cfg.DBPath)
	if err != nil {
		slog.Error("Failed to initialize storage", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	slog.Info("Storage initialized")

	storageAdapt := &storageAdapter{storage: db}

	// Initialize scheduler
	sched, err := scheduler.New(cfg.Timezone)
	if err != nil {
		slog.Error("Failed to initialize scheduler", "error", err)
		os.Exit(1)
	}
	slog.Info("Scheduler initialized")

	// Initialize HN client
	hnClient := &hnClientAdapter{client: hn.NewClient("")}
	slog.Info("HN client initialized")

	// Initialize scraper
	scrap := scraper.New(cfg.FetchTimeoutSecs)
	slog.Info("Scraper initialized")

	// Initialize summarizer
	summ := &summarizerAdapter{
		summarizer: summarizer.New(cfg.GeminiAPIKey, cfg.GeminiModel, ""),
	}
	slog.Info("Summarizer initialized")

	// Initialize ranker
	rank := &rankerAdapter{ranker: ranker.New()}
	slog.Info("Ranker initialized")

	// Initialize bot
	telegramBot, err := bot.New(cfg.TelegramToken, nil)
	if err != nil {
		slog.Error("Failed to initialize bot", "error", err)
		os.Exit(1)
	}
	slog.Info("Bot initialized")

	// Initialize digest workflow
	workflow := digest.New(
		hnClient,
		scrap,
		summ,
		rank,
		storageAdapt,
		telegramBot,
		cfg.TagDecayRate,
		cfg.MinTagWeight,
		2.0, // buffer factor
	)

	// Initialize command handler
	handler := bot.NewCommandHandler(
		storageAdapt,
		sched,
		workflow,
		cfg.TagBoostOnLike,
	)

	// Update bot with handler
	telegramBot, err = bot.New(cfg.TelegramToken, handler)
	if err != nil {
		slog.Error("Failed to reinitialize bot with handler", "error", err)
		os.Exit(1)
	}

	// Schedule digest if chat_id is configured
	chatIDStr, _ := db.GetSetting("chat_id")
	if chatIDStr != "" {
		chatID, _ := strconv.ParseInt(chatIDStr, 10, 64)
		if chatID != 0 {
			digestTime, _ := db.GetSetting("digest_time")
			if digestTime == "" {
				digestTime = cfg.DigestTime
			}

			sched.Schedule(digestTime, workflow.Run)
			slog.Info("Digest scheduled", "time", digestTime)
		}
	} else if cfg.ChatID != 0 {
		// Use chat_id from config if available
		db.SetSetting("chat_id", strconv.FormatInt(cfg.ChatID, 10))
		sched.Schedule(cfg.DigestTime, workflow.Run)
		slog.Info("Digest scheduled from config", "time", cfg.DigestTime)
	}

	sched.Start()
	slog.Info("Scheduler started")

	// Graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		slog.Info("Received signal, shutting down gracefully", "signal", sig)
		cancel()
	}()

	// Start bot (blocking)
	go func() {
		if err := telegramBot.Start(); err != nil {
			slog.Error("Bot error", "error", err)
			cancel()
		}
	}()

	<-ctx.Done()

	slog.Info("Stopping scheduler")
	sched.Stop()

	slog.Info("Closing database")
	db.Close()

	slog.Info("Shutdown complete")
}
