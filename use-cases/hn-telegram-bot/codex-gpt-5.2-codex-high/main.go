package main

import (
	"context"
	"database/sql"
	"log/slog"
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

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		logFatal("config load failed", err)
	}

	logger := setupLogger(cfg.LogLevel)
	slog.SetDefault(logger)
	logger.Info("config_loaded")

	db, err := sql.Open("sqlite", cfg.DBPath)
	if err != nil {
		logFatal("open database", err)
	}
	db.SetMaxOpenConns(1)

	store := storage.New(db)
	if err := store.Init(context.Background()); err != nil {
		logFatal("init database", err)
	}

	settings := bot.NewSettings(cfg.ChatID, cfg.DigestTime, cfg.ArticleCount)
	loadSettings(context.Background(), store, settings, logger)

	api, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		logFatal("create telegram bot", err)
	}

	sender := bot.NewTelegramSender(api)
	articleSender := &bot.SettingsArticleSender{Settings: settings, Sender: sender}

	hnClient := hn.NewClient(nil)
	scr := scraper.NewReadabilityScraper(time.Duration(cfg.FetchTimeoutSec) * time.Second)
	sum := summarizer.NewGeminiSummarizer(cfg.GeminiAPIKey, cfg.GeminiModel, nil)

	runner := &digest.Runner{
		HN:              hnClient,
		Scraper:         scr,
		Summarizer:      sum,
		Storage:         store,
		Sender:          articleSender,
		Logger:          logger,
		ArticleCount:    cfg.ArticleCount,
		ArticleCountFunc: settings.ArticleCount,
		TagDecayRate:    cfg.TagDecayRate,
		MinTagWeight:    cfg.MinTagWeight,
	}

	sched, err := scheduler.New(settings.DigestTime(), cfg.Timezone, func() {
		if err := runner.Run(context.Background()); err != nil {
			logger.Warn("scheduled_digest_failed", slog.String("error", err.Error()))
		}
	})
	if err != nil {
		logFatal("init scheduler", err)
	}
	sched.Start()
	logger.Info("scheduler_started")

	botHandler := &bot.Bot{
		Sender:         sender,
		Storage:        store,
		Digest:         runner,
		Scheduler:      sched,
		Settings:       settings,
		TagBoostOnLike: cfg.TagBoostOnLike,
		Logger:         logger,
	}

	poller := &bot.Poller{
		API:     api,
		Logger:  logger,
		Handler: func(ctx context.Context, update bot.Update) { botHandler.ProcessUpdate(ctx, update) },
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go poller.Run(ctx)

	<-ctx.Done()
	logger.Info("shutdown")

	sched.Stop()
	if err := db.Close(); err != nil {
		logger.Warn("db_close_failed", slog.String("error", err.Error()))
	}
}

func loadSettings(ctx context.Context, store *storage.Storage, settings *bot.Settings, logger *slog.Logger) {
	if val, ok, err := store.GetSetting(ctx, "chat_id"); err == nil && ok {
		if id, err := strconv.ParseInt(val, 10, 64); err == nil {
			settings.SetChatID(id)
		}
	}
	if val, ok, err := store.GetSetting(ctx, "digest_time"); err == nil && ok {
		settings.SetDigestTime(val)
	}
	if val, ok, err := store.GetSetting(ctx, "article_count"); err == nil && ok {
		if count, err := strconv.Atoi(val); err == nil {
			settings.SetArticleCount(count)
		}
	}
	logger.Info("settings_loaded", slog.Int64("chat_id", settings.ChatID()), slog.String("digest_time", settings.DigestTime()), slog.Int("article_count", settings.ArticleCount()))
}

func setupLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	return slog.New(h)
}

func logFatal(msg string, err error) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	logger.Error(msg, slog.String("error", err.Error()))
	os.Exit(1)
}
