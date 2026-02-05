package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Poller performs long polling for Telegram updates.
type Poller struct {
	API     *tgbotapi.BotAPI
	Logger  *slog.Logger
	Handler func(ctx context.Context, update Update)
}

// Run starts polling until context is canceled.
func (p *Poller) Run(ctx context.Context) {
	logger := p.Logger
	if logger == nil {
		logger = slog.Default()
	}
	offset := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		updates, err := p.getUpdates(ctx, offset)
		if err != nil {
			logger.Warn("poll_updates_failed", slog.String("error", err.Error()))
			time.Sleep(2 * time.Second)
			continue
		}
		for _, update := range updates {
			if update.UpdateID >= offset {
				offset = update.UpdateID + 1
			}
			if p.Handler != nil {
				p.Handler(ctx, update)
			}
		}
	}
}

func (p *Poller) getUpdates(ctx context.Context, offset int) ([]Update, error) {
	params := tgbotapi.Params{
		"offset":         strconv.Itoa(offset),
		"timeout":        "30",
		"allowed_updates": `["message","message_reaction"]`,
	}
	resp, err := p.API.MakeRequest("getUpdates", params)
	if err != nil {
		return nil, err
	}
	if !resp.Ok {
		return nil, fmt.Errorf("telegram response not ok: %s", resp.Description)
	}
	var updates []Update
	if err := json.Unmarshal(resp.Result, &updates); err != nil {
		return nil, err
	}
	return updates, nil
}
