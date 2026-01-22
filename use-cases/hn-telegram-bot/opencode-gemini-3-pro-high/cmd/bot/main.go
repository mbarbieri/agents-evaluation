package main

import (
	"log"
	"log/slog"
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
)

// Ensure dependencies satisfy interfaces
var _ digest.HNClient = (*hn.Client)(nil)

// ranker is a package with functions, not interface.

func main() {
	// 1. Config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Logging
	opts := &slog.HandlerOptions{}
	if cfg.LogLevel == "debug" {
		opts.Level = slog.LevelDebug
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, opts))
	slog.SetDefault(logger)

	slog.Info("Starting HN Telegram Bot", "config", cfg)

	// 2. Storage
	store, err := storage.New(cfg.DBPath)
	if err != nil {
		slog.Error("Failed to init storage", "err", err)
		os.Exit(1)
	}
	defer store.Close()

	// 3. Scheduler
	sched, err := scheduler.New(cfg.Timezone)
	if err != nil {
		slog.Error("Failed to init scheduler", "err", err)
		os.Exit(1)
	}

	// 4. Components
	hnClient := hn.NewClient()

	summ := summarizer.New(cfg.GeminiAPIKey, cfg.GeminiModel)

	// 5. Bot & Digest
	// We need a trigger func that runs the digest.
	// But Digest needs Bot (Sender).
	// Bot needs Trigger.
	// Cyclic dependency?
	// Trigger calls digest.Run. Digest uses Sender (Bot).
	// Bot struct calls Trigger.

	// We can create Bot first, then Digest, then set Trigger on Bot?
	// Bot.digestTrigger is private.
	// `bot.New` takes trigger.

	// Solution: Create a wrapper or use a closure variable that is assigned later.

	var runDigest func()

	b, err := bot.New(cfg, store, sched, func() {
		if runDigest != nil {
			runDigest()
		}
	})
	if err != nil {
		slog.Error("Failed to init bot", "err", err)
		os.Exit(1)
	}

	// Initialize Digest
	// Scraper new:
	timeout := time.Duration(cfg.FetchTimeoutSec) * time.Second
	scrPtr := scraper.New(timeout)

	dig := digest.New(store, hnClient, scrPtr, summ, b)
	dig.ArticleCount = cfg.ArticleCount
	dig.TagDecayRate = cfg.TagDecayRate
	dig.MinTagWeight = cfg.MinTagWeight

	runDigest = func() {
		slog.Info("Running digest workflow")
		if err := dig.Run(); err != nil {
			slog.Error("Digest workflow failed", "err", err)
		} else {
			slog.Info("Digest workflow completed")
		}
	}

	// 6. Schedule
	// Initial schedule from config/db
	// DB settings override config.
	dbTime, _ := store.GetSetting("digest_time")
	scheduleTime := cfg.DigestTime
	if dbTime != "" {
		scheduleTime = dbTime
	}

	if err := sched.UpdateSchedule(scheduleTime, runDigest); err != nil {
		slog.Error("Failed to schedule digest", "err", err)
		os.Exit(1)
	}
	sched.Start()
	slog.Info("Scheduler started", "time", scheduleTime)

	// 7. Start Bot
	go b.Start()
	slog.Info("Bot started")

	// 8. Wait for Signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	slog.Info("Shutting down...")
	sched.Stop()
	// store closed by defer
}
