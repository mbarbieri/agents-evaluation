package bot

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/antigravity/hn-telegram-bot/digest"
	"github.com/antigravity/hn-telegram-bot/storage"
)

type Storage interface {
	SetSetting(key, value string) error
	GetSetting(key string) (string, error)
	GetTagWeights() (map[string]storage.TagWeight, error)
	UpdateTagWeight(tag string, weight float64, occurrences int) error
	IsLiked(articleID int) (bool, error)
	MarkLiked(articleID int) error
	SaveArticle(a *storage.Article) error
	GetArticleByMessageID(msgID int) (*storage.Article, error)
}

type Workflow interface {
	Run(ctx context.Context, sender digest.Sender) error
}

type Sender interface {
	SendArticle(a *storage.Article) (int, error)
}

type Handler struct {
	storage  Storage
	workflow Workflow
	sender   Sender
}

func NewHandler(s Storage, w Workflow, sn Sender) *Handler {
	return &Handler{
		storage:  s,
		workflow: w,
		sender:   sn,
	}
}

func (h *Handler) HandleStart(chatID int64) string {
	h.storage.SetSetting("chat_id", strconv.FormatInt(chatID, 10))
	return "Welcome to HN Telegram Bot!\n\nI'll send you a daily digest of Hacker News articles based on your interests.\n\nCommands:\n/fetch - Get a digest now\n/settings - View or change settings\n/stats - View your interests\n\nReact with 👍 to articles to help me learn what you like!"
}

func (h *Handler) HandleFetch() string {
	go func() {
		h.workflow.Run(context.Background(), h.sender)
	}()
	return "Fetching latest articles for you..."
}

func (h *Handler) HandleSettings(args string, chatID int64) string {
	if args == "" {
		time, _ := h.storage.GetSetting("digest_time")
		count, _ := h.storage.GetSetting("article_count")
		return fmt.Sprintf("Current Settings:\nDigest Time: %s\nArticle Count: %s\n\nTo change, use:\n/settings time HH:MM\n/settings count 1-100", time, count)
	}

	parts := strings.Fields(args)
	if len(parts) < 2 {
		return "Usage:\n/settings time HH:MM\n/settings count N"
	}

	key := parts[0]
	val := parts[1]

	switch key {
	case "time":
		if len(val) != 5 || val[2] != ':' {
			return "Invalid time format. Use HH:MM"
		}
		h.storage.SetSetting("digest_time", val)
		return fmt.Sprintf("Digest time updated to %s", val)
	case "count":
		n, err := strconv.Atoi(val)
		if err != nil || n < 1 || n > 100 {
			return "Invalid count. Use a number between 1 and 100"
		}
		h.storage.SetSetting("article_count", val)
		return fmt.Sprintf("Article count updated to %d", n)
	}

	return "Unknown setting. Use 'time' or 'count'."
}

func (h *Handler) HandleStats() string {
	weights, err := h.storage.GetTagWeights()
	if err != nil {
		return "Failed to retrieve statistics."
	}

	if len(weights) == 0 {
		return "I haven't learned your interests yet. React with 👍 to articles!"
	}

	var tws []storage.TagWeight
	for _, tw := range weights {
		tws = append(tws, tw)
	}

	sort.Slice(tws, func(i, j int) bool {
		return tws[i].Weight > tws[j].Weight
	})

	limit := 10
	if len(tws) < limit {
		limit = len(tws)
	}

	var sb strings.Builder
	sb.WriteString("<b>Top Interests:</b>\n")
	for i := 0; i < limit; i++ {
		sb.WriteString(fmt.Sprintf("%s: %.2f (liked %d times)\n", tws[i].Tag, tws[i].Weight, tws[i].Occurrences))
	}

	return sb.String()
}

func (h *Handler) HandleReaction(msgID int, emoji string, boostAmount float64) error {
	if emoji != "👍" {
		return nil
	}

	art, err := h.storage.GetArticleByMessageID(msgID)
	if err != nil {
		return fmt.Errorf("failed to get article for reaction: %w", err)
	}
	if art == nil {
		return nil // Not an article message
	}

	liked, err := h.storage.IsLiked(art.HNID)
	if err != nil {
		return err
	}
	if liked {
		return nil // Idempotent
	}

	weights, err := h.storage.GetTagWeights()
	if err != nil {
		return err
	}

	for _, tag := range art.Tags {
		tw, ok := weights[tag]
		if !ok {
			tw = storage.TagWeight{Tag: tag, Weight: 1.0, Occurrences: 0}
		}
		tw.Weight += boostAmount
		tw.Occurrences++
		if err := h.storage.UpdateTagWeight(tw.Tag, tw.Weight, tw.Occurrences); err != nil {
			return err
		}
	}

	return h.storage.MarkLiked(art.HNID)
}
