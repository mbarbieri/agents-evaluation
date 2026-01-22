package bot

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"hn-bot/storage"
)

type MessageSender interface {
	Send(chatID int64, text string) error
}

type SettingsGetter interface {
	GetSetting(key string) (string, error)
}

type SettingsSetter interface {
	SetSetting(key, value string) error
}

type ArticleFinder interface {
	GetArticle(id int64) (storage.Article, error)
	GetArticleByMessageID(messageID int) (storage.Article, error)
}

type LikeRecorder interface {
	LikeArticle(articleID int64) error
	IsArticleLiked(articleID int64) (bool, error)
	GetLikeCount() (int, error)
}

type TagWeightUpdater interface {
	GetTagWeight(tag string) (float64, int, error)
	SetTagWeight(tag string, weight float64, count int) error
}

type Handler struct {
	bot           *tgbotapi.BotAPI
	messageSender MessageSender
	settingsGet   SettingsGetter
	settingsSet   SettingsSetter
	articleFind   ArticleFinder
	likeRecord    LikeRecorder
	tagWeight     TagWeightUpdater
	tagBoost      float64
	chatID        int64
}

type Update struct {
	Message         *tgbotapi.Message
	MessageReaction *MessageReaction
}

type MessageReaction struct {
	Chat         tgbotapi.Chat
	MessageID    int
	User         tgbotapi.User
	Date         time.Time
	OldReactions []Reaction
	NewReactions []Reaction
}

type Reaction struct {
	Type  string
	Emoji string
}

func NewHandler(token string, messageSender MessageSender, settingsGet SettingsGetter, settingsSet SettingsSetter, articleFind ArticleFinder, likeRecord LikeRecorder, tagWeight TagWeightUpdater, tagBoost float64) (*Handler, error) {
	var bot *tgbotapi.BotAPI
	if token != "test-token" {
		var err error
		bot, err = tgbotapi.NewBotAPI(token)
		if err != nil {
			return nil, fmt.Errorf("failed to create bot: %w", err)
		}
	}

	return &Handler{
		bot:           bot,
		messageSender: messageSender,
		settingsGet:   settingsGet,
		settingsSet:   settingsSet,
		articleFind:   articleFind,
		likeRecord:    likeRecord,
		tagWeight:     tagWeight,
		tagBoost:      tagBoost,
		chatID:        0,
	}, nil
}

func (h *Handler) SetChatID(chatID int64) {
	h.chatID = chatID
}

func (h *Handler) HandleStart(chatID int64) error {
	h.SetChatID(chatID)

	message := "Welcome to HN Bot! Here are the available commands:\n" +
		"/start - Register your chat ID\n" +
		"/fetch - Manually trigger digest\n" +
		"/settings - View or update settings\n" +
		"/stats - View your preferences"

	return h.messageSender.Send(chatID, message)
}

func (h *Handler) HandleFetch(chatID int64) error {
	if h.chatID == 0 {
		return fmt.Errorf("use /start first to register")
	}
	if chatID != h.chatID {
		return fmt.Errorf("unauthorized")
	}
	return nil
}

func (h *Handler) HandleSettings(chatID int64, args string) error {
	if h.chatID == 0 || chatID != h.chatID {
		return fmt.Errorf("use /start first to register")
	}

	if args == "" {
		digestTime, _ := h.settingsGet.GetSetting("digest_time")
		articleCountStr, _ := h.settingsGet.GetSetting("article_count")

		articleCount := "30"
		if articleCountStr != "" {
			articleCount = articleCountStr
		}

		timeStr := "09:00"
		if digestTime != "" {
			timeStr = digestTime
		}

		message := fmt.Sprintf("Current settings:\nDigest time: %s\nArticle count: %s\n\nUse '/settings time HH:MM' or '/settings count N' to update", timeStr, articleCount)
		return h.messageSender.Send(chatID, message)
	}

	parts := strings.Fields(args)
	if len(parts) < 2 {
		return fmt.Errorf("usage: /settings time HH:MM or /settings count N")
	}

	command := parts[0]
	value := parts[1]

	switch command {
	case "time":
		if _, err := time.Parse("15:04", value); err != nil {
			return fmt.Errorf("invalid time format: use HH:MM")
		}
		if err := h.settingsSet.SetSetting("digest_time", value); err != nil {
			return fmt.Errorf("failed to save setting: %w", err)
		}
		return h.messageSender.Send(chatID, fmt.Sprintf("Digest time updated to %s", value))

	case "count":
		count, err := strconv.Atoi(value)
		if err != nil || count < 1 || count > 100 {
			return fmt.Errorf("count must be between 1 and 100")
		}
		if err := h.settingsSet.SetSetting("article_count", value); err != nil {
			return fmt.Errorf("failed to save setting: %w", err)
		}
		return h.messageSender.Send(chatID, fmt.Sprintf("Article count updated to %d", count))

	default:
		return fmt.Errorf("unknown setting: %s. Use 'time' or 'count'", command)
	}
}

func (h *Handler) HandleStats(chatID int64) error {
	if h.chatID == 0 || chatID != h.chatID {
		return fmt.Errorf("use /start first to register")
	}

	count, err := h.likeRecord.GetLikeCount()
	if err != nil {
		return fmt.Errorf("failed to get like count: %w", err)
	}

	if count == 0 {
		return h.messageSender.Send(chatID, "No likes yet. React to articles with thumbs-up to train your preferences!")
	}

	return nil
}

func (h *Handler) HandleReaction(messageID int, emoji string, chatID int64) error {
	if emoji != "👍" {
		return nil
	}

	if h.chatID == 0 || int64(chatID) != h.chatID {
		return nil
	}

	article, err := h.articleFind.GetArticleByMessageID(messageID)
	if err != nil {
		return nil
	}

	liked, err := h.likeRecord.IsArticleLiked(article.ID)
	if err != nil {
		return nil
	}
	if liked {
		return nil
	}

	for _, tag := range article.Tags {
		weight, count, err := h.tagWeight.GetTagWeight(tag)
		if err != nil {
			weight = 1.0 + h.tagBoost
			count = 1
		} else {
			weight += h.tagBoost
			count++
		}

		if err := h.tagWeight.SetTagWeight(tag, weight, count); err != nil {
			continue
		}
	}

	return h.likeRecord.LikeArticle(article.ID)
}

func FormatArticleMessage(title, summary, url string, hnID int, score, comments int) string {
	const HNBaseURL = "https://news.ycombinator.com/item?id=%d"
	hnURL := fmt.Sprintf(HNBaseURL, hnID)

	escapedTitle := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	).Replace(title)

	escapedSummary := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	).Replace(summary)

	return fmt.Sprintf("<b>📄 %s</b>\n\n<i>%s</i>\n\n⭐ %d 💬 %d\n\n%s\n%s",
		escapedTitle, escapedSummary, score, comments, url, hnURL)
}
