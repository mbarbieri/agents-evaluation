package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"hn-telegram-bot/bot"
	"hn-telegram-bot/config"
	"hn-telegram-bot/digest"
	"hn-telegram-bot/hn"
	"hn-telegram-bot/scheduler"
	"hn-telegram-bot/scraper"
	"hn-telegram-bot/storage"
	"hn-telegram-bot/summarizer"
)

func main() {
	// Structured JSON logging to stdout
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Load configuration
	cfgPath := "./config.yaml"
	cfg, err := config.Load(cfgPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	slog.Info("config loaded", "digest_time", cfg.DigestTime, "timezone", cfg.Timezone, "article_count", cfg.ArticleCount)

	// Set log level
	switch cfg.LogLevel {
	case "debug":
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))
	case "warn":
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn})))
	case "error":
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))
	}

	// Initialize storage
	store, err := storage.New(cfg.DBPath)
	if err != nil {
		slog.Error("failed to initialize storage", "error", err)
		os.Exit(1)
	}
	defer store.Close()
	slog.Info("storage initialized", "db_path", cfg.DBPath)

	// Load settings from DB (override config defaults)
	if savedChatID, _ := store.GetSetting("chat_id"); savedChatID != "" {
		if id, err := strconv.ParseInt(savedChatID, 10, 64); err == nil {
			cfg.ChatID = id
		}
	}
	if savedTime, _ := store.GetSetting("digest_time"); savedTime != "" {
		cfg.DigestTime = savedTime
	}
	if savedCount, _ := store.GetSetting("article_count"); savedCount != "" {
		if n, err := strconv.Atoi(savedCount); err == nil {
			cfg.ArticleCount = n
		}
	}

	// Initialize components
	httpClient := &http.Client{Timeout: time.Duration(cfg.FetchTimeoutSec) * time.Second}
	hnClient := hn.NewClient(httpClient)
	articleScraper := scraper.NewScraper(time.Duration(cfg.FetchTimeoutSec) * time.Second)
	articleSummarizer := summarizer.NewSummarizer(cfg.GeminiAPIKey, cfg.GeminiModel, httpClient)

	// Create storage adapter for the digest package
	storageAdapter := &digestStorageAdapter{store: store}

	// Initialize scheduler
	sched, err := scheduler.New(cfg.Timezone)
	if err != nil {
		slog.Error("failed to create scheduler", "error", err)
		os.Exit(1)
	}
	slog.Info("scheduler initialized", "timezone", cfg.Timezone)

	// Create bot
	telegramBot := bot.New(bot.Config{
		Token:          cfg.TelegramToken,
		ChatID:         cfg.ChatID,
		DigestTime:     cfg.DigestTime,
		ArticleCount:   cfg.ArticleCount,
		TagBoostAmount: cfg.TagBoostOnLike,
	}, bot.Deps{
		ArticleLookup:   &articleLookupAdapter{store: store},
		LikeTracker:     &likeTrackerAdapter{store: store},
		TagBooster:      &tagBoosterAdapter{store: store},
		SettingsStore:   &settingsAdapter{store: store},
		StatsProvider:   &statsAdapter{store: store},
		ScheduleUpdater: sched,
	})

	// Create digest runner
	digestRunner := digest.NewRunner(
		&hnClientAdapter{client: hnClient},
		&scraperAdapter{scraper: articleScraper},
		&summarizerAdapter{summarizer: articleSummarizer},
		telegramBot,
		storageAdapter,
		digest.Config{
			ChatID:       cfg.ChatID,
			ArticleCount: cfg.ArticleCount,
			DecayRate:    cfg.TagDecayRate,
			MinWeight:    cfg.MinTagWeight,
		},
	)

	// Wire digest function into bot
	digestFunc := func() {
		ctx := context.Background()
		// Update config from bot's current settings
		digestRunner.UpdateConfig(digest.Config{
			ChatID:       telegramBot.GetChatID(),
			ArticleCount: telegramBot.GetArticleCount(),
			DecayRate:    cfg.TagDecayRate,
			MinWeight:    cfg.MinTagWeight,
		})
		if err := digestRunner.Run(ctx); err != nil {
			slog.Error("digest run failed", "error", err)
		}
	}

	// Update bot with digest function
	telegramBot = bot.New(bot.Config{
		Token:          cfg.TelegramToken,
		ChatID:         cfg.ChatID,
		DigestTime:     cfg.DigestTime,
		ArticleCount:   cfg.ArticleCount,
		TagBoostAmount: cfg.TagBoostOnLike,
	}, bot.Deps{
		ArticleLookup:   &articleLookupAdapter{store: store},
		LikeTracker:     &likeTrackerAdapter{store: store},
		TagBooster:      &tagBoosterAdapter{store: store},
		SettingsStore:   &settingsAdapter{store: store},
		StatsProvider:   &statsAdapter{store: store},
		ScheduleUpdater: sched,
		DigestFunc:      digestFunc,
	})

	// Schedule daily digest
	if err := sched.Schedule(cfg.DigestTime, digestFunc); err != nil {
		slog.Error("failed to schedule digest", "error", err)
		os.Exit(1)
	}
	sched.Start()
	slog.Info("scheduler started", "digest_time", cfg.DigestTime)

	// Graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		slog.Info("received signal, shutting down", "signal", sig)
		cancel()
	}()

	// Start bot polling
	slog.Info("bot starting", "chat_id", cfg.ChatID)
	if err := telegramBot.Run(ctx); err != nil {
		slog.Error("bot stopped with error", "error", err)
	}

	sched.Stop()
	slog.Info("shutdown complete")
}

// --- Adapters to bridge package types ---

// hnClientAdapter bridges hn.Client to digest.HNClient
type hnClientAdapter struct {
	client hn.Client
}

func (a *hnClientAdapter) TopStories(ctx context.Context, limit int) ([]int, error) {
	return a.client.TopStories(ctx, limit)
}

func (a *hnClientAdapter) GetItem(ctx context.Context, id int) (*digest.HNItem, error) {
	item, err := a.client.GetItem(ctx, id)
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

// scraperAdapter bridges scraper.Scraper to digest.ContentScraper
type scraperAdapter struct {
	scraper scraper.Scraper
}

func (a *scraperAdapter) Scrape(ctx context.Context, url string) (string, error) {
	return a.scraper.Scrape(ctx, url)
}

// summarizerAdapter bridges summarizer.Summarizer to digest.ArticleSummarizer
type summarizerAdapter struct {
	summarizer summarizer.Summarizer
}

func (a *summarizerAdapter) Summarize(ctx context.Context, title, content string) (*digest.SummaryResult, error) {
	result, err := a.summarizer.Summarize(ctx, title, content)
	if err != nil {
		return nil, err
	}
	return &digest.SummaryResult{
		Summary: result.Summary,
		Tags:    result.Tags,
	}, nil
}

// digestStorageAdapter bridges storage.Store to digest.Storage
type digestStorageAdapter struct {
	store *storage.Store
}

func (a *digestStorageAdapter) GetRecentSentArticleIDs(days int) ([]int, error) {
	return a.store.GetRecentSentArticleIDs(days)
}

func (a *digestStorageAdapter) SaveArticle(article *digest.StoredArticle) error {
	return a.store.SaveArticle(&storage.Article{
		ID:        article.ID,
		Title:     article.Title,
		URL:       article.URL,
		Summary:   article.Summary,
		Tags:      article.Tags,
		Score:     article.Score,
		FetchedAt: article.FetchedAt,
	})
}

func (a *digestStorageAdapter) MarkSent(articleID, telegramMsgID int) error {
	return a.store.MarkSent(articleID, telegramMsgID)
}

func (a *digestStorageAdapter) ApplyDecay(decayRate, minWeight float64) error {
	return a.store.ApplyDecay(decayRate, minWeight)
}

func (a *digestStorageAdapter) GetTagWeights() ([]digest.TagWeightEntry, error) {
	weights, err := a.store.GetTagWeights()
	if err != nil {
		return nil, err
	}
	result := make([]digest.TagWeightEntry, len(weights))
	for i, w := range weights {
		result[i] = digest.TagWeightEntry{Tag: w.Tag, Weight: w.Weight}
	}
	return result, nil
}

// articleLookupAdapter bridges storage.Store to bot.ArticleLookup
type articleLookupAdapter struct {
	store *storage.Store
}

func (a *articleLookupAdapter) GetArticleBySentMsgID(msgID int) (*bot.StoredArticle, error) {
	article, err := a.store.GetArticleBySentMsgID(msgID)
	if err != nil {
		return nil, err
	}
	if article == nil {
		return nil, nil
	}
	return &bot.StoredArticle{ID: article.ID, Tags: article.Tags}, nil
}

// likeTrackerAdapter bridges storage.Store to bot.LikeTracker
type likeTrackerAdapter struct {
	store *storage.Store
}

func (a *likeTrackerAdapter) IsLiked(articleID int) (bool, error) {
	return a.store.IsLiked(articleID)
}

func (a *likeTrackerAdapter) RecordLike(articleID int) error {
	return a.store.RecordLike(articleID)
}

// tagBoosterAdapter bridges storage.Store to bot.TagBooster
type tagBoosterAdapter struct {
	store *storage.Store
}

func (a *tagBoosterAdapter) GetTagWeight(tag string) (*bot.TagWeightInfo, error) {
	tw, err := a.store.GetTagWeight(tag)
	if err != nil {
		return nil, err
	}
	if tw == nil {
		return nil, nil
	}
	return &bot.TagWeightInfo{Tag: tw.Tag, Weight: tw.Weight, Count: tw.Count}, nil
}

func (a *tagBoosterAdapter) UpsertTagWeight(tag string, weight float64, count int) error {
	return a.store.UpsertTagWeight(tag, weight, count)
}

// settingsAdapter bridges storage.Store to bot.SettingsStore
type settingsAdapter struct {
	store *storage.Store
}

func (a *settingsAdapter) GetSetting(key string) (string, error) {
	return a.store.GetSetting(key)
}

func (a *settingsAdapter) SetSetting(key, value string) error {
	return a.store.SetSetting(key, value)
}

// statsAdapter bridges storage.Store to bot.StatsProvider
type statsAdapter struct {
	store *storage.Store
}

func (a *statsAdapter) GetTopTagWeights(limit int) ([]bot.TagWeightInfo, error) {
	weights, err := a.store.GetTopTagWeights(limit)
	if err != nil {
		return nil, err
	}
	result := make([]bot.TagWeightInfo, len(weights))
	for i, w := range weights {
		result[i] = bot.TagWeightInfo{Tag: w.Tag, Weight: w.Weight, Count: w.Count}
	}
	return result, nil
}

func (a *statsAdapter) GetLikeCount() (int, error) {
	return a.store.GetLikeCount()
}
