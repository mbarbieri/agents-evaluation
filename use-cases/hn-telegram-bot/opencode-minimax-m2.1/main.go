package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"hn-bot/bot"
	"hn-bot/config"
	"hn-bot/digest"
	"hn-bot/scheduler"
	"hn-bot/storage"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	configPath := os.Getenv("HN_BOT_CONFIG")
	if configPath == "" {
		configPath = "./config.yaml"
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		logger.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	store, err := storage.NewStorage(cfg.DBPath)
	if err != nil {
		logger.Error("Failed to open storage", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	botAPI, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		logger.Error("Failed to create bot", "error", err)
		os.Exit(1)
	}
	botAPI.Debug = cfg.LogLevel == "debug"

	logger.Info("Bot started", "token_hash", cfg.TelegramToken[:10]+"...")

	d := digest.NewDigest(cfg, store, logger)

	sched, err := scheduler.NewScheduler(cfg.Timezone)
	if err != nil {
		logger.Error("Failed to create scheduler", "error", err)
		os.Exit(1)
	}

	digestTime := cfg.DigestTime
	if storedTime, err := store.GetSetting("digest_time"); err == nil && storedTime != "" {
		digestTime = storedTime
	}

	err = sched.Schedule(digestTime, func() {
		if err := runDigest(ctx, botAPI, store, d, cfg.ChatID, logger); err != nil {
			logger.Error("Failed to run digest", "error", err)
		}
	})
	if err != nil {
		logger.Error("Failed to schedule digest", "error", err)
		os.Exit(1)
	}

	sched.Start()
	logger.Info("Scheduler started", "time", digestTime, "timezone", cfg.Timezone)

	go handleUpdates(ctx, botAPI, store, d, cfg, logger)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down...")
	cancel()
	sched.Stop()
	logger.Info("Shutdown complete")
}

func runDigest(ctx context.Context, botAPI *tgbotapi.BotAPI, store *storage.Storage, d *digest.Digest, chatID int64, logger *slog.Logger) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if chatID == 0 {
		storedChatID, err := store.GetSetting("chat_id")
		if err == nil && storedChatID != "" {
			chatID, _ = strconv.ParseInt(storedChatID, 10, 64)
		}
	}

	if chatID == 0 {
		logger.Warn("No chat ID configured, skipping digest")
		return nil
	}

	articles, err := d.Run()
	if err != nil {
		return fmt.Errorf("failed to generate digest: %w", err)
	}

	if len(articles) == 0 {
		logger.Info("No articles to send")
		return nil
	}

	for _, article := range articles {
		msg := d.FormatArticleMessage(&article, 0)
		tgMsg := tgbotapi.NewMessage(chatID, msg)
		tgMsg.ParseMode = tgbotapi.ModeHTML
		tgMsg.DisableWebPagePreview = false

		sent, err := botAPI.Send(tgMsg)
		if err != nil {
			logger.Error("Failed to send message", "error", err, "article_id", article.ID)
			continue
		}

		if err := d.SaveArticle(&article, int64(sent.MessageID)); err != nil {
			logger.Error("Failed to save article", "error", err, "article_id", article.ID)
		}

		time.Sleep(500 * time.Millisecond)
	}

	logger.Info("Digest sent", "articles_count", len(articles))
	return nil
}

func handleUpdates(ctx context.Context, botAPI *tgbotapi.BotAPI, store *storage.Storage, d *digest.Digest, cfg *config.Config, logger *slog.Logger) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := botAPI.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			return
		case update := <-updates:
			if update.Message != nil {
				handleMessage(ctx, update.Message, botAPI, store, d, cfg, logger)
			}
		}
	}
}

func handleMessage(ctx context.Context, msg *tgbotapi.Message, botAPI *tgbotapi.BotAPI, store *storage.Storage, d *digest.Digest, cfg *config.Config, logger *slog.Logger) {
	if !msg.IsCommand() {
		return
	}

	chatID := msg.Chat.ID
	command, args := bot.ParseCommand(msg.Text)

	response := ""

	switch command {
	case "start":
		if err := store.SetSetting("chat_id", strconv.FormatInt(chatID, 10)); err != nil {
			logger.Error("Failed to save chat_id", "error", err)
			response = "Error saving settings. Please try again."
		} else {
			response = bot.FormatWelcomeMessage()
		}

	case "fetch":
		go func() {
			if err := runDigest(ctx, botAPI, store, d, chatID, logger); err != nil {
				logger.Error("Failed to run fetch", "error", err)
			}
		}()
		return

	case "settings":
		if args == "" {
			digestTime, articleCount, _ := d.GetSettings()
			response = bot.FormatSettingsDisplay(digestTime, articleCount)
		} else {
			key, value, err := bot.ParseSettingsArgs(args)
			if err != nil {
				response = fmt.Sprintf("Invalid format. Use:\n/settings time HH:MM\n/settings count N")
			} else {
				if err := d.UpdateSetting(key, value); err != nil {
					response = "Error updating setting. Please try again."
				} else {
					response = bot.FormatSettingsUpdate(key, value)
				}
			}
		}

	case "stats":
		tags, likeCount, _ := d.GetStats()
		response = bot.FormatStats(tags, likeCount)

	default:
		response = "Unknown command. Use /start, /fetch, /settings, or /stats."
	}

	if response != "" {
		tgMsg := tgbotapi.NewMessage(chatID, response)
		tgMsg.ParseMode = tgbotapi.ModeHTML
		botAPI.Send(tgMsg)
	}
}
