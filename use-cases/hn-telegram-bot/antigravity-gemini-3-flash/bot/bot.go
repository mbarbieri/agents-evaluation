package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/antigravity/hn-telegram-bot/storage"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	api            *tgbotapi.BotAPI
	handler        *Handler
	token          string
	tagBoostAmount float64
}

func NewBot(token string, storage Storage, workflow Workflow, tagBoost float64) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	b := &Bot{
		api:            api,
		token:          token,
		tagBoostAmount: tagBoost,
	}

	b.handler = NewHandler(storage, workflow, b)
	return b, nil
}

func (b *Bot) Start(ctx context.Context) error {
	slog.Info("Starting bot long polling", "user", b.api.Self.UserName)

	offset := 0
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			updates, err := b.getUpdates(offset)
			if err != nil {
				slog.Error("Failed to get updates", "error", err)
				time.Sleep(5 * time.Second)
				continue
			}

			for _, update := range updates {
				if update.UpdateID >= offset {
					offset = update.UpdateID + 1
				}

				if update.Message != nil {
					b.handleMessage(update.Message)
				} else if update.MessageReaction != nil {
					b.handleReaction(update.MessageReaction)
				}
			}
		}
	}
}

func (b *Bot) getUpdates(offset int) ([]CustomUpdate, error) {
	urlStr := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates", b.token)
	u, _ := url.Parse(urlStr)
	q := u.Query()
	q.Set("offset", strconv.Itoa(offset))
	q.Set("timeout", "30")
	q.Set("allowed_updates", `["message", "message_reaction"]`)
	u.RawQuery = q.Encode()

	resp, err := http.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var apiResp struct {
		OK     bool           `json:"ok"`
		Result []CustomUpdate `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	if !apiResp.OK {
		return nil, fmt.Errorf("api response not ok")
	}

	return apiResp.Result, nil
}

func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	if !msg.IsCommand() {
		return
	}

	var response string
	switch msg.Command() {
	case "start":
		response = b.handler.HandleStart(msg.Chat.ID)
	case "fetch":
		response = b.handler.HandleFetch()
	case "settings":
		response = b.handler.HandleSettings(msg.CommandArguments(), msg.Chat.ID)
	case "stats":
		response = b.handler.HandleStats()
	default:
		return
	}

	if response != "" {
		newMsg := tgbotapi.NewMessage(msg.Chat.ID, response)
		newMsg.ParseMode = tgbotapi.ModeHTML
		b.api.Send(newMsg)
	}
}

func (b *Bot) handleReaction(mr *MessageReactionUpdated) {
	for _, r := range mr.NewReaction {
		if r.Emoji == "👍" {
			if err := b.handler.HandleReaction(mr.MessageID, r.Emoji, b.tagBoostAmount); err != nil {
				slog.Error("Failed to handle reaction", "error", err)
			}
		}
	}
}

func (b *Bot) SendArticle(a *storage.Article) (int, error) {
	// Format article message
	text := fmt.Sprintf("<b>🔥 %s</b>\n\n<i>%s</i>\n\n⭐ %d points\n\n<a href=\"%s\">Read Article</a> | <a href=\"https://news.ycombinator.com/item?id=%d\">HN Discussion</a>",
		escapeHTML(a.Title),
		escapeHTML(a.Summary),
		a.HNScore,
		a.URL,
		a.HNID,
	)

	chatIDStr, err := b.handler.storage.GetSetting("chat_id")
	if err != nil || chatIDStr == "" {
		return 0, fmt.Errorf("chat_id not set")
	}
	chatID, _ := strconv.ParseInt(chatIDStr, 10, 64)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	sent, err := b.api.Send(msg)
	if err != nil {
		return 0, err
	}
	return sent.MessageID, nil
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
