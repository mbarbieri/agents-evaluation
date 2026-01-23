package digest

import (
	"context"
	"errors"
)

type ArticleSender interface {
	SendArticle(ctx context.Context, chatID int64, message string) (int, error)
}

type TelegramSender struct {
	Bot    ArticleSender
	ChatID int64
}

func (s *TelegramSender) Send(ctx context.Context, article Article) (Sent, error) {
	if s == nil || s.Bot == nil {
		return Sent{}, errors.New("sender not initialized")
	}
	if s.ChatID == 0 {
		return Sent{}, errors.New("chat id required")
	}
	message := FormatArticleMessage(article)
	messageID, err := s.Bot.SendArticle(ctx, s.ChatID, message)
	if err != nil {
		return Sent{}, err
	}
	return Sent{ArticleID: article.ID, MessageID: messageID}, nil
}
