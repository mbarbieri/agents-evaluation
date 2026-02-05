package bot

import (
	"context"
	"fmt"
	"html"
	"strings"

	"hn-telegram-bot/model"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Sender sends messages to Telegram.
type Sender interface {
	SendText(ctx context.Context, chatID int64, text string) (int, error)
	SendHTML(ctx context.Context, chatID int64, htmlText string) (int, error)
}

// ArticleSender sends formatted article messages.
type ArticleSender interface {
	SendArticle(ctx context.Context, article model.Article) (int, error)
}

// TelegramSender implements Sender using tgbotapi.
type TelegramSender struct {
	api *tgbotapi.BotAPI
}

// NewTelegramSender creates a new sender.
func NewTelegramSender(api *tgbotapi.BotAPI) *TelegramSender {
	return &TelegramSender{api: api}
}

// SendText sends a plain text message.
func (s *TelegramSender) SendText(ctx context.Context, chatID int64, text string) (int, error) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = ""
	resp, err := s.api.Send(msg)
	if err != nil {
		return 0, err
	}
	return resp.MessageID, nil
}

// SendHTML sends an HTML-formatted message.
func (s *TelegramSender) SendHTML(ctx context.Context, chatID int64, htmlText string) (int, error) {
	msg := tgbotapi.NewMessage(chatID, htmlText)
	msg.ParseMode = tgbotapi.ModeHTML
	resp, err := s.api.Send(msg)
	if err != nil {
		return 0, err
	}
	return resp.MessageID, nil
}

// SettingsArticleSender sends article messages using the current chat ID from settings.
type SettingsArticleSender struct {
	Settings *Settings
	Sender   Sender
}

// SendArticle sends the article to the configured chat.
func (s *SettingsArticleSender) SendArticle(ctx context.Context, article model.Article) (int, error) {
	chatID := s.Settings.ChatID()
	if chatID == 0 {
		return 0, fmt.Errorf("chat id not configured")
	}
	return s.Sender.SendHTML(ctx, chatID, FormatArticleMessage(article))
}

// FormatArticleMessage renders the article message with HTML formatting.
func FormatArticleMessage(article model.Article) string {
	title := html.EscapeString(article.Title)
	summary := html.EscapeString(article.Summary)
	hnURL := fmt.Sprintf("https://news.ycombinator.com/item?id=%d", article.ID)

	var b strings.Builder
	b.WriteString("📰 <b>")
	b.WriteString(title)
	b.WriteString("</b>\n")
	if summary != "" {
		b.WriteString("<i>")
		b.WriteString(summary)
		b.WriteString("</i>\n\n")
	} else {
		b.WriteString("\n")
	}
	b.WriteString(fmt.Sprintf("⭐ %d  💬 %d\n", article.HNScore, article.Comments))
	if article.URL != "" {
		b.WriteString(fmt.Sprintf("<a href=\"%s\">Read</a> | ", article.URL))
	}
	b.WriteString(fmt.Sprintf("<a href=\"%s\">HN</a>", hnURL))
	return b.String()
}
