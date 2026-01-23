package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hn-telegram-bot/bot"
	"hn-telegram-bot/config"
	"hn-telegram-bot/digest"
	"hn-telegram-bot/hn"
	"hn-telegram-bot/ranker"
	"hn-telegram-bot/scheduler"
	"hn-telegram-bot/scraper"
	"hn-telegram-bot/storage"
	"hn-telegram-bot/summarizer"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	os.Exit(run())
}

func run() int {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config load failed", "err", err)
		return 1
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	st, err := storage.Open(ctx, cfg.DBPath)
	if err != nil {
		logger.Error("db open failed", "err", err)
		return 1
	}
	defer func() { _ = st.Close() }()

	settings := bot.NewSettings(cfg)
	settings.LoadFromStore(ctx, st)

	// Build digest pipeline
	hnClient := hn.NewClient(nil)
	scr := scraper.New(nil, time.Duration(cfg.FetchTimeoutSec)*time.Second)
	sum := summarizer.New(cfg.GeminiAPIKey, cfg.GeminiModel, nil)
	rk := ranker.Ranker{Weights: st}
	api, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		logger.Error("create telegram api failed", "err", err)
		return 1
	}

	// Sender is implemented by the Telegram bot layer.
	// Digest runner uses a lightweight adapter set by bot.
	var digestRunner digest.Service
	digestRunner = digest.Service{
		Log:        logger,
		HN:         hnAdapter{c: hnClient},
		Scraper:    scr,
		Summarizer: summarizerAdapter{c: sum},
		Ranker:     rankerAdapter{r: rk},
		Store:      storageAdapter{s: st},
		Sender:     nil,
		Cfg: digest.Config{
			ArticleCount: cfg.ArticleCount,
			DecayRate:    cfg.TagDecayRate,
			MinTagWeight: cfg.MinTagWeight,
			RecentWindow: 7 * 24 * time.Hour,
		},
	}

	// Scheduler wraps digest runner.
	sched := scheduler.New(runnerFunc(func(ctx context.Context) error { return digestRunner.Run(ctx) }), schedulerLogger{log: logger})
	if err := sched.Start(ctx, settings); err != nil {
		logger.Error("scheduler start failed", "err", err)
		return 1
	}
	defer sched.Stop()

	tg := &bot.Bot{
		Log:         logger,
		Token:       cfg.TelegramToken,
		API:         api,
		Client:      nil,
		Settings:    settings,
		Store:       st,
		Digest:      runnerFunc(func(ctx context.Context) error { return digestRunner.Run(ctx) }),
		Scheduler:   schedulerAdapterForBot{sched: sched},
		BoostOnLike: cfg.TagBoostOnLike,
	}

	// Configure sender for digest pipeline.
	digestRunner.Sender = botSender{api: api, chatID: settings}

	logger.Info("started", "timezone", cfg.Timezone, "digest_time", settings.DigestTime())
	if err := tg.Run(ctx); err != nil {
		logger.Error("bot run failed", "err", err)
		return 1
	}
	logger.Info("shutdown")
	return 0
}

type runnerFunc func(context.Context) error

func (f runnerFunc) Run(ctx context.Context) error { return f(ctx) }

type schedulerLogger struct{ log *slog.Logger }

func (s schedulerLogger) Info(msg string, keysAndValues ...any) { s.log.Info(msg, keysAndValues...) }
func (s schedulerLogger) Error(err error, msg string, keysAndValues ...any) {
	args := append([]any{"err", err}, keysAndValues...)
	s.log.Error(msg, args...)
}

// --- Adapters to keep packages narrow ---

type hnAdapter struct{ c hn.Client }

func (h hnAdapter) TopStories(ctx context.Context) ([]int, error) { return h.c.TopStories(ctx) }
func (h hnAdapter) Item(ctx context.Context, id int) (digest.HNItem, error) {
	it, err := h.c.Item(ctx, id)
	if err != nil {
		return digest.HNItem{}, err
	}
	return digest.HNItem{ID: it.ID, Title: it.Title, URL: it.URL, Score: it.Score, Descendants: it.Descendants}, nil
}

type summarizerAdapter struct{ c summarizer.Client }

func (s summarizerAdapter) Summarize(ctx context.Context, content string) (digest.SummaryResult, error) {
	r, err := s.c.Summarize(ctx, content)
	if err != nil {
		return digest.SummaryResult{}, err
	}
	return digest.SummaryResult{Summary: r.Summary, Tags: r.Tags}, nil
}

type rankerAdapter struct{ r ranker.Ranker }

func (r rankerAdapter) Rank(ctx context.Context, arts []digest.RankArticle) ([]digest.RankArticle, error) {
	var in []ranker.Article
	for _, a := range arts {
		in = append(in, ranker.Article{ID: a.ID, Tags: a.Tags, HNScore: a.HNScore})
	}
	ranked, err := r.r.Rank(ctx, in)
	if err != nil {
		return nil, err
	}
	idx := map[int]digest.RankArticle{}
	for _, a := range arts {
		idx[a.ID] = a
	}
	var out []digest.RankArticle
	for _, ra := range ranked {
		a := idx[ra.ID]
		a.FinalScore = ra.FinalScore
		out = append(out, a)
	}
	return out, nil
}

type storageAdapter struct{ s *storage.Store }

func (s storageAdapter) ApplyDecay(ctx context.Context, decayRate float64, minWeight float64) error {
	return s.s.ApplyDecay(ctx, decayRate, minWeight)
}
func (s storageAdapter) SentArticleIDsSince(ctx context.Context, since time.Time) (map[int]struct{}, error) {
	return s.s.SentArticleIDsSince(ctx, since)
}
func (s storageAdapter) UpsertArticle(ctx context.Context, a digest.StoredArticle) error {
	return s.s.UpsertArticle(ctx, storage.Article{
		ID:             a.ID,
		Title:          a.Title,
		URL:            a.URL,
		Summary:        a.Summary,
		Tags:           a.Tags,
		HNScore:        a.HNScore,
		HNCommentCount: a.HNCommentCount,
		FetchedAt:      a.FetchedAt,
	})
}
func (s storageAdapter) MarkArticleSent(ctx context.Context, articleID int, sentAt time.Time, telegramMessageID int) error {
	return s.s.MarkArticleSent(ctx, articleID, sentAt, telegramMessageID)
}

// botSender is used by digest pipeline; it creates a BotAPI per send.
// This keeps the digest package free of telegram dependency.
type schedulerAdapterForBot struct{ sched *scheduler.Scheduler }

func (s schedulerAdapterForBot) Update(settings interface {
	DigestTime() string
	Timezone() string
}) error {
	return s.sched.Update(settings)
}

type botSender struct {
	api    *tgbotapi.BotAPI
	chatID interface{ ChatID() int64 }
}

func (b botSender) SendArticle(ctx context.Context, a digest.RankArticle) (int, error) {
	chatID := b.chatID.ChatID()
	if chatID == 0 {
		return 0, errors.New("chat_id not set")
	}
	html := bot.FormatArticleHTML(bot.ArticleMessage{ID: a.ID, Title: a.Title, URL: a.URL, Summary: a.Summary, Score: a.HNScore, Comments: a.Comments})
	msg := tgbotapi.NewMessage(chatID, html)
	msg.ParseMode = "HTML"
	sent, err := b.api.Send(msg)
	if err != nil {
		return 0, err
	}
	return sent.MessageID, nil
}
