package main

import (
	"context"
	"errors"
	"fmt"
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
	"hn-telegram-bot/settings"
	"hn-telegram-bot/storage"
	"hn-telegram-bot/summarizer"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config load failed", "error", err)
		os.Exit(1)
	}

	store, err := storage.New(cfg.DBPath)
	if err != nil {
		logger.Error("storage init failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		_ = store.Close()
	}()

	defaults := map[string]string{
		"chat_id":       fmt.Sprintf("%d", cfg.ChatID),
		"digest_time":   cfg.DigestTime,
		"article_count": fmt.Sprintf("%d", cfg.ArticleCount),
	}
	settingsManager, err := settings.NewManager(ctx, store, defaults)
	if err != nil {
		logger.Error("settings init failed", "error", err)
		os.Exit(1)
	}
	_ = settingsManager

	hnClient := hn.NewClient("", nil)
	scraperClient := scraper.New(nil, time.Duration(cfg.FetchTimeoutSec)*time.Second)
	summarizerClient := summarizer.NewClient("", cfg.GeminiModel, cfg.GeminiAPIKey, nil)

	settingsAdapter := &bot.SettingsAdapter{Store: store}
	statsAdapter := &bot.StatsAdapter{Store: store}
	reactionAdapter := &bot.ReactionAdapter{Store: store}

	var cronScheduler *scheduler.Scheduler

	reactionHandler := bot.NewReactionHandler(reactionAdapter, cfg.TagBoostOnLike)

	telegramBot, err := bot.NewTelegramBot(cfg.TelegramToken, nil, reactionHandler, nil, logger)
	if err != nil {
		logger.Error("telegram init failed", "error", err)
		os.Exit(1)
	}

	schedulerJob := func() {
		workflow := newWorkflow(cfg, hnClient, scraperClient, summarizerClient, store, telegramBot)
		if err := workflow.Run(context.Background()); err != nil {
			logger.Error("digest run failed", "error", err)
		}
	}
	cronScheduler, err = scheduler.New(cfg.Timezone, cfg.DigestTime, schedulerJob)
	if err != nil {
		logger.Error("scheduler init failed", "error", err)
		os.Exit(1)
	}
	if err := cronScheduler.Start(); err != nil {
		logger.Error("scheduler start failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		_ = cronScheduler.Stop()
	}()

	commandHandler := bot.NewHandler(&bot.SenderAdapter{Bot: telegramBot}, settingsAdapter, cronScheduler, statsAdapter)
	telegramBot.SetHandler(commandHandler)
	telegramBot.SetReactionHandler(reactionHandler)
	telegramBot.SetFetchHandler(func(ctx context.Context) error {
		workflow := newWorkflow(cfg, hnClient, scraperClient, summarizerClient, store, telegramBot)
		return workflow.Run(ctx)
	})

	if err := telegramBot.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("bot stopped", "error", err)
	}

	logger.Info("shutdown complete")
}

func newWorkflow(cfg config.Config, hnClient *hn.Client, scraperClient *scraper.Scraper, summarizerClient *summarizer.Client, store *storage.Store, telegramBot *bot.TelegramBot) *digest.Workflow {
	adapter := &digest.StorageAdapter{Store: store}
	chatID := cfg.ChatID
	if chatID == 0 {
		value, ok, err := store.GetSetting(context.Background(), "chat_id")
		if err == nil && ok {
			if parsed, parseErr := strconv.ParseInt(value, 10, 64); parseErr == nil {
				chatID = parsed
			}
		}
	}
	sender := &digest.TelegramSender{Bot: telegramBot, ChatID: chatID}
	hnAdapter := &digest.HNAdapter{Client: hnClient}
	summarizerAdapter := &digest.SummarizerAdapter{Client: summarizerClient}
	return digest.NewWorkflow(
		digest.WorkflowConfig{
			ArticleCount: cfg.ArticleCount,
			FetchLimit:   cfg.ArticleCount * 2,
			DecayRate:    cfg.TagDecayRate,
			MinTagWeight: cfg.MinTagWeight,
		},
		hnAdapter,
		scraperClient,
		summarizerAdapter,
		adapter,
		sender,
	)
}
