package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
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

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	// Setup structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Set log level from config
	if cfg.LogLevel == "debug" {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
	}

	logger.Info("Configuration loaded",
		"timezone", cfg.Timezone,
		"digest_time", cfg.DigestTime,
		"article_count", cfg.ArticleCount,
	)

	// Initialize storage
	store, err := storage.New(cfg.DBPath)
	if err != nil {
		logger.Error("Failed to initialize storage", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	logger.Info("Storage initialized", "db_path", cfg.DBPath)

	// Initialize HN client
	hnClient := hn.NewClient(time.Duration(cfg.FetchTimeoutSecs) * time.Second)

	// Initialize scraper
	scraperClient := scraper.New(&http.Client{
		Timeout: time.Duration(cfg.FetchTimeoutSecs) * time.Second,
	})

	// Initialize summarizer
	summarizerClient := summarizer.NewClient(
		cfg.GeminiAPIKey,
		cfg.GeminiModel,
		30*time.Second,
	)

	// Initialize bot
	botConfig := bot.Config{
		Token:         cfg.TelegramToken,
		InitialChatID: cfg.ChatID,
		DigestTime:    cfg.DigestTime,
		ArticleCount:  cfg.ArticleCount,
	}

	botInstance, err := bot.New(botConfig, store, logger)
	if err != nil {
		logger.Error("Failed to initialize bot", "error", err)
		os.Exit(1)
	}

	logger.Info("Bot initialized", "bot_name", botInstance.GetChatID())

	// Initialize digest service
	digestDeps := &digest.Dependencies{
		Storage:    store,
		HNClient:   hnClient,
		Scraper:    scraperClient,
		Summarizer: summarizerClient,
		Bot:        botInstance,
		Logger:     logger,
	}

	digestService := digest.NewService(digestDeps, cfg)

	// Load settings from storage
	if err := digestService.LoadSettingsFromStorage(); err != nil {
		logger.Error("Failed to load settings from storage", "error", err)
	}

	// Set up bot callbacks
	botInstance.SetDigestTrigger(func(ctx context.Context) error {
		return digestService.Run(ctx)
	})

	botInstance.SetSettingsUpdater(func(digestTime string, articleCount int) error {
		return digestService.UpdateSettings(digestTime, articleCount)
	})

	// Initialize scheduler
	sched, err := scheduler.New(cfg.Timezone)
	if err != nil {
		logger.Error("Failed to initialize scheduler", "error", err)
		os.Exit(1)
	}

	// Schedule digest job
	if err := sched.ScheduleDigest(cfg.DigestTime, func() {
		if err := digestService.Run(context.Background()); err != nil {
			logger.Error("Scheduled digest failed", "error", err)
		}
	}); err != nil {
		logger.Error("Failed to schedule digest", "error", err)
		os.Exit(1)
	}

	logger.Info("Digest scheduled", "time", cfg.DigestTime, "timezone", cfg.Timezone)

	// Start scheduler
	sched.Start()
	defer sched.Stop()

	// Setup update config for long polling
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60
	updateConfig.AllowedUpdates = []string{"message"} // Only receive message updates for now

	// Get updates channel
	updates := botInstance.GetUpdatesChan(updateConfig)

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("Bot started", "digest_time", cfg.DigestTime)

	// Main loop
	for {
		select {
		case update := <-updates:
			botInstance.HandleUpdate(update)

		case sig := <-sigChan:
			logger.Info("Received shutdown signal", "signal", sig)
			logger.Info("Shutting down gracefully...")
			return
		}
	}
}
