package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramBot struct {
	api             *tgbotapi.BotAPI
	client          *http.Client
	baseURL         string
	updateOffset    int
	handler         *Handler
	reactionHandler *ReactionHandler
	fetchHandler    func(context.Context) error
	logger          *slog.Logger
}

func NewTelegramBot(token string, handler *Handler, reactionHandler *ReactionHandler, fetchHandler func(context.Context) error, logger *slog.Logger) (*TelegramBot, error) {
	if token == "" {
		return nil, errors.New("telegram token required")
	}
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	return &TelegramBot{
		api:             api,
		client:          &http.Client{Timeout: 30 * time.Second},
		baseURL:         fmt.Sprintf(tgbotapi.APIEndpoint, api.Token, ""),
		handler:         handler,
		reactionHandler: reactionHandler,
		fetchHandler:    fetchHandler,
		logger:          logger,
	}, nil
}

func (b *TelegramBot) SetHandler(handler *Handler) {
	if b == nil {
		return
	}
	b.handler = handler
}

func (b *TelegramBot) SetReactionHandler(handler *ReactionHandler) {
	if b == nil {
		return
	}
	b.reactionHandler = handler
}

func (b *TelegramBot) SetFetchHandler(handler func(context.Context) error) {
	if b == nil {
		return
	}
	b.fetchHandler = handler
}

func (b *TelegramBot) Run(ctx context.Context) error {
	if b == nil || b.client == nil || b.api == nil {
		return errors.New("bot not initialized")
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		updates, err := b.getUpdates(ctx)
		if err != nil {
			b.logError("get updates", err)
			time.Sleep(2 * time.Second)
			continue
		}
		for _, update := range updates {
			b.updateOffset = update.UpdateID + 1
			if update.Message != nil {
				b.handleMessage(ctx, update.Message)
				continue
			}
			if update.MessageReaction != nil {
				b.handleReaction(ctx, update.MessageReaction)
			}
		}
	}
}

func (b *TelegramBot) SendMessage(ctx context.Context, chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	_, err := b.api.Send(msg)
	if err != nil {
		return err
	}
	return nil
}

func (b *TelegramBot) SendArticle(ctx context.Context, chatID int64, message string) (int, error) {
	msg := tgbotapi.NewMessage(chatID, message)
	msg.ParseMode = "HTML"
	resp, err := b.api.Send(msg)
	if err != nil {
		return 0, err
	}
	return resp.MessageID, nil
}

func (b *TelegramBot) handleMessage(ctx context.Context, message *tgbotapi.Message) {
	if message == nil {
		return
	}
	if !message.IsCommand() {
		return
	}
	if b.handler == nil {
		return
	}
	command := message.Command()
	args := message.CommandArguments()
	chatID := message.Chat.ID
	switch command {
	case "start":
		if err := b.handler.HandleStart(ctx, chatID); err != nil {
			b.logError("start", err)
		}
	case "settings":
		if err := b.handler.HandleSettings(ctx, chatID, splitArgs(args)); err != nil {
			_ = b.SendMessage(ctx, chatID, "Usage: /settings time HH:MM | /settings count N")
			b.logError("settings", err)
		}
	case "stats":
		if err := b.handler.HandleStats(ctx, chatID); err != nil {
			b.logError("stats", err)
		}
	case "fetch":
		if b.fetchHandler != nil {
			if err := b.fetchHandler(ctx); err != nil {
				b.logError("fetch", err)
			}
		}
	}
}

func (b *TelegramBot) handleReaction(ctx context.Context, reaction *MessageReactionUpdate) {
	if reaction == nil || b.reactionHandler == nil {
		return
	}
	for _, r := range reaction.NewReaction {
		if err := b.reactionHandler.Handle(ctx, reaction.MessageID, r.Emoji); err != nil {
			b.logError("reaction", err)
		}
	}
}

func (b *TelegramBot) getUpdates(ctx context.Context) ([]Update, error) {
	url := b.baseURL + "getUpdates"
	body := map[string]any{
		"offset":          b.updateOffset,
		"timeout":         30,
		"allowed_updates": []string{"message", "message_reaction"},
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("getUpdates status %d", resp.StatusCode)
	}
	var payload UpdateResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if !payload.OK {
		return nil, errors.New("telegram api error")
	}
	return payload.Result, nil
}

func (b *TelegramBot) logError(action string, err error) {
	if b.logger == nil {
		return
	}
	b.logger.Error("telegram bot error", "action", action, "error", err)
}

func splitArgs(argString string) []string {
	if argString == "" {
		return nil
	}
	return strings.Fields(argString)
}

type UpdateResponse struct {
	OK     bool     `json:"ok"`
	Result []Update `json:"result"`
}

type Update struct {
	UpdateID        int                    `json:"update_id"`
	Message         *tgbotapi.Message      `json:"message"`
	MessageReaction *MessageReactionUpdate `json:"message_reaction"`
}

type MessageReactionUpdate struct {
	Chat        tgbotapi.Chat `json:"chat"`
	MessageID   int           `json:"message_id"`
	User        tgbotapi.User `json:"user"`
	Date        int           `json:"date"`
	OldReaction []Reaction    `json:"old_reaction"`
	NewReaction []Reaction    `json:"new_reaction"`
}

type Reaction struct {
	Type  string `json:"type"`
	Emoji string `json:"emoji"`
}

func parseChatID(value string) (int64, error) {
	if value == "" {
		return 0, errors.New("chat id required")
	}
	return strconv.ParseInt(value, 10, 64)
}
