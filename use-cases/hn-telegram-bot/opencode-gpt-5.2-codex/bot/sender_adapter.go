package bot

import (
	"context"
	"errors"
)

type SenderAdapter struct {
	Bot *TelegramBot
}

func (a *SenderAdapter) Send(ctx context.Context, chatID int64, text string) error {
	if a == nil || a.Bot == nil {
		return errors.New("bot not initialized")
	}
	return a.Bot.SendMessage(ctx, chatID, text)
}
