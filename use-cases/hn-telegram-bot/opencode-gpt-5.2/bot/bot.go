package bot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"hn-telegram-bot/storage"
)

type Bot struct {
	Log         *slog.Logger
	Token       string
	API         *tgbotapi.BotAPI
	Client      *http.Client
	Settings    *Settings
	Store       *storage.Store
	Digest      DigestRunner
	Scheduler   SchedulerUpdater
	BoostOnLike float64
}

func (b *Bot) Run(ctx context.Context) error {
	log := b.Log
	if log == nil {
		log = slog.Default()
	}
	if b.Client == nil {
		b.Client = &http.Client{Timeout: 60 * time.Second}
	}
	if b.API == nil {
		api, err := tgbotapi.NewBotAPI(b.Token)
		if err != nil {
			return fmt.Errorf("create telegram api: %w", err)
		}
		b.API = api
	}
	if b.Settings == nil {
		return fmt.Errorf("missing settings")
	}
	if b.Store == nil {
		return fmt.Errorf("missing store")
	}

	chatID := b.Settings.ChatID()
	if chatID == 0 {
		log.Info("chat_id not set; waiting for /start")
	}

	var offset int
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		updates, err := b.getUpdates(ctx, offset)
		if err != nil {
			log.Warn("getUpdates failed", "err", err)
			time.Sleep(1 * time.Second)
			continue
		}
		for _, u := range updates {
			if u.UpdateID >= offset {
				offset = u.UpdateID + 1
			}
			// Standard messages (commands)
			if u.Message != nil {
				b.handleMessage(ctx, log, u.Message)
				continue
			}
			// Reactions
			if u.MessageReaction != nil {
				b.handleReaction(ctx, log, u.MessageReaction)
				continue
			}
		}
	}
}

type tgUpdate struct {
	UpdateID        int               `json:"update_id"`
	Message         *tgbotapi.Message `json:"message,omitempty"`
	MessageReaction *MessageReaction  `json:"message_reaction,omitempty"`
}

type tgGetUpdatesResponse struct {
	OK     bool       `json:"ok"`
	Result []tgUpdate `json:"result"`
}

func (b *Bot) getUpdates(ctx context.Context, offset int) ([]tgUpdate, error) {
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates", b.Token)
	u, _ := url.Parse(endpoint)
	q := u.Query()
	q.Set("timeout", "30")
	q.Set("offset", strconv.Itoa(offset))
	allowed := `["message","message_reaction"]`
	q.Set("allowed_updates", allowed)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := b.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("telegram status %d: %s", resp.StatusCode, string(body))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var out tgGetUpdatesResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	if !out.OK {
		return nil, errors.New("telegram response not ok")
	}
	return out.Result, nil
}

type telegramSender struct{ api *tgbotapi.BotAPI }

func (s telegramSender) SendText(ctx context.Context, chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	_, err := s.api.Send(msg)
	return err
}

func (s telegramSender) SendHTML(ctx context.Context, chatID int64, html string) (int, error) {
	msg := tgbotapi.NewMessage(chatID, html)
	msg.ParseMode = "HTML"
	sent, err := s.api.Send(msg)
	if err != nil {
		return 0, err
	}
	return sent.MessageID, nil
}

func (b *Bot) handleMessage(ctx context.Context, log *slog.Logger, m *tgbotapi.Message) {
	if m == nil || m.Text == "" {
		return
	}
	if !strings.HasPrefix(m.Text, "/") {
		return
	}
	cmd := strings.Fields(m.Text)
	name := strings.TrimPrefix(cmd[0], "/")
	args := ""
	if len(cmd) > 1 {
		args = strings.Join(cmd[1:], " ")
	}

	sender := telegramSender{api: b.API}

	chatID := m.Chat.ID
	if b.Settings.ChatID() == 0 && name != "start" {
		_ = sender.SendText(ctx, chatID, "Please run /start first.")
		return
	}
	switch name {
	case "start":
		if err := b.Settings.HandleStart(ctx, sender, b.Store, chatID); err != nil {
			log.Warn("start handler failed", "err", err)
		}
	case "fetch":
		if b.Settings.ChatID() == 0 {
			_ = sender.SendText(ctx, chatID, "Please run /start first.")
			return
		}
		HandleFetch(ctx, sender, chatID, b.Digest)
	case "settings":
		if b.Settings.ChatID() == 0 {
			_ = sender.SendText(ctx, chatID, "Please run /start first.")
			return
		}
		if err := b.Settings.HandleSettings(ctx, sender, b.Store, b.Scheduler, chatID, args); err != nil {
			log.Warn("settings handler failed", "err", err)
		}
	case "stats":
		if b.Settings.ChatID() == 0 {
			_ = sender.SendText(ctx, chatID, "Please run /start first.")
			return
		}
		// Adapt storage.TagWeight to bot.TagWeight
		tw, err := b.Store.TopTags(ctx, 10)
		if err != nil {
			log.Warn("top tags failed", "err", err)
			return
		}
		likes, err := b.Store.LikeCount(ctx)
		if err != nil {
			log.Warn("like count failed", "err", err)
			return
		}
		var tags []TagWeight
		for _, t := range tw {
			tags = append(tags, TagWeight{Tag: t.Tag, Weight: t.Weight})
		}
		_ = HandleStats(ctx, sender, chatID, statsStoreAdapter{tags: tags, likes: likes})
	default:
		_ = sender.SendText(ctx, chatID, "Unknown command")
	}
}

type statsStoreAdapter struct {
	tags  []TagWeight
	likes int
}

func (s statsStoreAdapter) TopTags(ctx context.Context, limit int) ([]TagWeight, error) {
	return s.tags, nil
}
func (s statsStoreAdapter) LikeCount(ctx context.Context) (int, error) { return s.likes, nil }

func (b *Bot) handleReaction(ctx context.Context, log *slog.Logger, r *MessageReaction) {
	if r == nil {
		return
	}
	thumbsUp := false
	for _, rt := range r.NewReaction {
		if rt.Type == "emoji" && rt.Emoji == "👍" {
			thumbsUp = true
			break
		}
	}
	if !thumbsUp {
		return
	}
	err := HandleThumbsUpReaction(ctx, reactionStoreAdapter{store: b.Store}, r.MessageID, ReactionConfig{Boost: b.BoostOnLike})
	if err != nil {
		log.Warn("reaction handler failed", "message_id", r.MessageID, "err", err)
	}
}

type reactionStoreAdapter struct{ store *storage.Store }

func (r reactionStoreAdapter) ArticleByTelegramMessageID(ctx context.Context, messageID int) (ReactionArticle, error) {
	a, err := r.store.ArticleByTelegramMessageID(ctx, messageID)
	if err != nil {
		return ReactionArticle{}, err
	}
	return ReactionArticle{ID: a.ID, Tags: a.Tags}, nil
}

func (r reactionStoreAdapter) IsLiked(ctx context.Context, articleID int) (bool, error) {
	return r.store.IsLiked(ctx, articleID)
}

func (r reactionStoreAdapter) BoostTagsOnLike(ctx context.Context, tags []string, boost float64) error {
	return r.store.BoostTagsOnLike(ctx, tags, boost)
}

func (r reactionStoreAdapter) RecordLike(ctx context.Context, articleID int, likedAt time.Time) error {
	return r.store.RecordLike(ctx, articleID, likedAt)
}
