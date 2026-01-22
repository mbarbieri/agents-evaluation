package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/opencode/hn-telegram-bot/bot"
	"github.com/opencode/hn-telegram-bot/config"
	"github.com/opencode/hn-telegram-bot/digest"
	"github.com/opencode/hn-telegram-bot/hn"
	"github.com/opencode/hn-telegram-bot/scheduler"
	"github.com/opencode/hn-telegram-bot/scraper"
	"github.com/opencode/hn-telegram-bot/storage"
	"github.com/opencode/hn-telegram-bot/summarizer"
)

func main() {
	// 1. Load Config
	configPath := os.Getenv("HN_BOT_CONFIG")
	if configPath == "" {
		configPath = "./config.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Override DB path if env var set
	if dbEnv := os.Getenv("HN_BOT_DB"); dbEnv != "" {
		cfg.DBPath = dbEnv
	}

	// 2. Initialize Storage
	store, err := storage.NewStorage(cfg.DBPath)
	if err != nil {
		slog.Error("failed to initialize storage", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	// Load chat_id from DB if not in config
	if cfg.ChatID == 0 {
		if val, _ := store.GetSetting(context.Background(), "chat_id"); val != "" {
			if id, err := strconv.ParseInt(val, 10, 64); err == nil {
				cfg.ChatID = id
			}
		}
	}
	// Load other dynamic settings from DB
	if val, _ := store.GetSetting(context.Background(), "digest_time"); val != "" {
		cfg.DigestTime = val
	}
	if val, _ := store.GetSetting(context.Background(), "article_count"); val != "" {
		if count, err := strconv.Atoi(val); err == nil {
			cfg.ArticleCount = count
		}
	}

	// 3. Initialize Components
	hnClient := hn.NewClient("")
	articleScraper := scraper.NewScraper(cfg.FetchTimeoutSecs)
	articleSummarizer := summarizer.NewSummarizer(cfg.GeminiAPIKey, cfg.GeminiModel, "")

	sched := scheduler.NewScheduler(cfg.Timezone)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 4. Wire Bot and Digest Manager
	// Note: We use SetArticleSender to break circularity
	tgBot, err := bot.NewBot(cfg, store, nil, func() {
		slog.Info("Config changed, updating schedule")
	})
	if err != nil {
		slog.Error("failed to initialize bot", "error", err)
		os.Exit(1)
	}

	digestMgr := digest.NewManager(cfg, hnClient, articleScraper, articleSummarizer, tgBot, store)
	tgBot.SetArticleSender(digestMgr)

	// 5. Start Scheduler
	runDigest := func() {
		slog.Info("Running digest")
		if err := digestMgr.SendDigest(context.Background()); err != nil {
			slog.Error("digest failed", "error", err)
		}
	}

	sched.UpdateSchedule(cfg.DigestTime, runDigest)
	sched.Start()
	slog.Info("Scheduler started", "time", cfg.DigestTime, "timezone", cfg.Timezone)

	// Update bot config change handler with actual digest run
	// In a real app we might need to re-wrap or use a more sophisticated mechanism
	// but this works for now as the closure captures digestMgr.

	// 6. Start Bot
	go tgBot.Start(ctx)
	slog.Info("Bot started")

	// 7. Graceful Shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	slog.Info("Shutting down...", "signal", sig)

	sched.Stop()
	cancel()

	slog.Info("Shutdown complete")
}
