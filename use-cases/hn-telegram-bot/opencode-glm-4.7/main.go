package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"hn-bot/bot"
	"hn-bot/config"
	"hn-bot/digest"
	"hn-bot/hn"
	"hn-bot/ranker"
	"hn-bot/scheduler"
	"hn-bot/scraper"
	"hn-bot/storage"
	"hn-bot/summarizer"
)

func main() {
	configPath := os.Getenv("HN_BOT_CONFIG")
	if configPath == "" {
		configPath = "./config.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	slog.Info("Loaded configuration", "config_path", configPath)

	store, err := storage.New(cfg.DBPath)
	if err != nil {
		slog.Error("Failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	slog.Info("Database initialized", "db_path", cfg.DBPath)

	hnClient := hn.NewClient("https://hacker-news.firebaseio.com")
	scraper := scraper.NewScraper(cfg.FetchTimeoutSecs)
	summarizer := summarizer.NewSummarizer(cfg.GeminiAPIKey, cfg.GeminiModel, "https://generativelanguage.googleapis.com")
	ranker := ranker.NewRanker(0.3, 0.7)

	digestWorkflow := digest.NewDigest(
		hnClient,
		scraper,
		summarizer,
		ranker,
		store,
		nil,
		0,
		float64(cfg.ArticleCount),
		cfg.TagDecayRate,
		cfg.MinTagWeight,
		cfg.TagBoostOnLike,
	)

	botHandler, err := bot.NewHandler(
		cfg.TelegramToken,
		nil,
		store,
		store,
		store,
		store,
		store,
		cfg.TagBoostOnLike,
	)
	if err != nil {
		slog.Error("Failed to create bot handler", "error", err)
		os.Exit(1)
	}

	telegramBot, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		slog.Error("Failed to create Telegram bot", "error", err)
		os.Exit(1)
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := telegramBot.GetUpdatesChan(u)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	chatID, err := store.GetSetting("chat_id")
	if err != nil {
		slog.Info("No chat ID set, user needs to run /start")
	} else {
		botHandler.SetChatID(parseChatID(chatID))
	}

	scheduler, err := scheduler.NewScheduler(cfg.Timezone)
	if err != nil {
		slog.Error("Failed to create scheduler", "error", err)
		os.Exit(1)
	}

	digestTime, _ := store.GetSetting("digest_time")
	if digestTime == "" {
		digestTime = cfg.DigestTime
	}

	articleCountStr, _ := store.GetSetting("article_count")
	if articleCountStr != "" {
	}

	if chatID != "" {
		err = scheduler.Schedule(digestTime, func() {
			slog.Info("Running scheduled digest")
			digestWorkflow.Run()
		})
		if err != nil {
			slog.Error("Failed to schedule digest", "error", err)
		} else {
			slog.Info("Scheduled digest", "time", digestTime, "timezone", cfg.Timezone)
		}
	}

	slog.Info("Bot started, waiting for updates...")

	go func() {
		for update := range updates {
			if update.Message == nil {
				continue
			}

			if !update.Message.IsCommand() {
				continue
			}

			switch update.Message.Command() {
			case "start":
				if err := botHandler.HandleStart(update.Message.Chat.ID); err != nil {
					slog.Error("Failed to handle /start", "error", err)
				}
				store.SetSetting("chat_id", formatChatID(update.Message.Chat.ID))

			case "fetch":
				if err := botHandler.HandleFetch(update.Message.Chat.ID); err == nil {
					slog.Info("Running manual digest")
					digestWorkflow.Run()
				} else {
					slog.Error("Failed to handle /fetch", "error", err)
				}

			case "settings":
				args := update.Message.CommandArguments()
				if err := botHandler.HandleSettings(update.Message.Chat.ID, args); err != nil {
					slog.Error("Failed to handle /settings", "error", err)
				}

			case "stats":
				if err := botHandler.HandleStats(update.Message.Chat.ID); err != nil {
					slog.Error("Failed to handle /stats", "error", err)
				}
			}
		}
	}()

	<-sigChan
	slog.Info("Shutting down...")

	if err := scheduler.Stop(); err != nil {
		slog.Error("Failed to stop scheduler", "error", err)
	}

	slog.Info("Shutdown complete")
}

func parseChatID(s string) int64 {
	id := int64(0)
	for _, c := range s {
		id = id*10 + int64(c-'0')
	}
	return id
}

func formatChatID(id int64) string {
	return fmt.Sprintf("%d", id)
}

func parseInt(s string) int {
	i := 0
	for _, c := range s {
		i = i*10 + int(c-'0')
	}
	return i
}
