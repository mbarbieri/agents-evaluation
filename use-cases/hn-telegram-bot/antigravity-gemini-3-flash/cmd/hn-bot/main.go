package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/antigravity/hn-telegram-bot/bot"
	"github.com/antigravity/hn-telegram-bot/config"
	"github.com/antigravity/hn-telegram-bot/digest"
	"github.com/antigravity/hn-telegram-bot/hn"
	"github.com/antigravity/hn-telegram-bot/scheduler"
	"github.com/antigravity/hn-telegram-bot/scraper"
	"github.com/antigravity/hn-telegram-bot/storage"
	"github.com/antigravity/hn-telegram-bot/summarizer"
)

func main() {
	// Configure logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	slog.Info("Starting HN Telegram Bot")

	// 1. Load Configuration
	configPath := os.Getenv("HN_BOT_CONFIG")
	if configPath == "" {
		configPath = "./config.yaml"
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// 2. Initialize Storage
	dbPath := os.Getenv("HN_BOT_DB")
	if dbPath == "" {
		dbPath = cfg.DBPath
	}
	store, err := storage.NewStorage(dbPath)
	if err != nil {
		slog.Error("Failed to initialize storage", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	// 3. Initialize Components
	hnClient := hn.NewClient("https://hacker-news.firebaseio.com", nil)
	scrapeClient := scraper.NewScraper(time.Duration(cfg.FetchTimeoutSecs) * time.Second)
	sumClient := summarizer.NewSummarizer(cfg.GeminiAPIKey, cfg.GeminiModel, "")

	workflowCfg := &digest.WorkflowConfig{
		ArticleCount: cfg.ArticleCount,
		TagDecayRate: cfg.TagDecayRate,
		MinTagWeight: cfg.MinTagWeight,
		RecentDays:   7,
	}
	workflow := digest.NewWorkflow(store, hnClient, scrapeClient, sumClient, workflowCfg)

	tgBot, err := bot.NewBot(cfg.TelegramToken, store, workflow, cfg.TagBoostOnLike)
	if err != nil {
		slog.Error("Failed to initialize Telegram bot", "error", err)
		os.Exit(1)
	}

	// 4. Setup Scheduler
	sched := scheduler.NewScheduler(cfg.Timezone)
	sched.Start()
	defer sched.Stop()

	// Trigger digest job
	digestJob := func() {
		slog.Info("Scheduled digest trigger fired")
		if err := workflow.Run(context.Background(), tgBot); err != nil {
			slog.Error("Digest workflow failed", "error", err)
		}
	}

	// Convert HH:MM to cron expression "0 MM HH * * *" (assuming seconds enabled in our scheduler)
	cronExpr := fmt.Sprintf("0 %s %s * * *", cfg.DigestTime[3:], cfg.DigestTime[:2])
	if err := sched.UpdateSchedule(cronExpr, digestJob); err != nil {
		slog.Error("Failed to set schedule", "error", err)
		os.Exit(1)
	}

	// 5. Handle Graceful Shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		slog.Info("Received signal, shutting down", "signal", sig)
		cancel()
	}()

	// 6. Start Bot Polling
	if err := tgBot.Start(ctx); err != nil {
		slog.Error("Bot polling stopped with error", "error", err)
	}

	slog.Info("HN Telegram Bot stopped")
}
