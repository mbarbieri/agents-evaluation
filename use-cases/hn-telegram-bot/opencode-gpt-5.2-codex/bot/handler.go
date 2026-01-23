package bot

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Sender interface {
	Send(ctx context.Context, chatID int64, text string) error
}

type SettingsStore interface {
	Get(ctx context.Context, key string) (string, bool, error)
	Set(ctx context.Context, key, value string) error
}

type Scheduler interface {
	Update(digestTime string) error
}

type StatsStore interface {
	TopTags(ctx context.Context, limit int) ([]TagWeight, error)
	LikeCount(ctx context.Context) (int, error)
}

type TagWeight struct {
	Tag    string
	Weight float64
}

type Handler struct {
	sender    Sender
	settings  SettingsStore
	scheduler Scheduler
	stats     StatsStore
}

func NewHandler(sender Sender, settings SettingsStore, scheduler Scheduler, stats StatsStore) *Handler {
	return &Handler{sender: sender, settings: settings, scheduler: scheduler, stats: stats}
}

func (h *Handler) HandleStart(ctx context.Context, chatID int64) error {
	if h == nil || h.settings == nil || h.sender == nil {
		return errors.New("handler not initialized")
	}
	if chatID == 0 {
		return errors.New("chat id required")
	}
	if err := h.settings.Set(ctx, "chat_id", strconv.FormatInt(chatID, 10)); err != nil {
		return err
	}
	msg := "Welcome! Commands: /fetch, /settings, /stats"
	return h.sender.Send(ctx, chatID, msg)
}

func (h *Handler) HandleSettings(ctx context.Context, chatID int64, args []string) error {
	if h == nil || h.settings == nil || h.sender == nil {
		return errors.New("handler not initialized")
	}
	if chatID == 0 {
		return errors.New("chat id required")
	}
	if len(args) == 0 {
		timeVal, _, _ := h.settings.Get(ctx, "digest_time")
		countVal, _, _ := h.settings.Get(ctx, "article_count")
		msg := fmt.Sprintf("Digest time: %s\nArticle count: %s", timeVal, countVal)
		return h.sender.Send(ctx, chatID, msg)
	}
	if len(args) < 2 {
		return errors.New("usage: /settings time HH:MM | /settings count N")
	}
	key := strings.ToLower(args[0])
	value := args[1]
	if key == "time" {
		if _, err := time.Parse("15:04", value); err != nil {
			return errors.New("invalid time format")
		}
		if err := h.settings.Set(ctx, "digest_time", value); err != nil {
			return err
		}
		if h.scheduler != nil {
			if err := h.scheduler.Update(value); err != nil {
				return err
			}
		}
		return h.sender.Send(ctx, chatID, "Digest time updated")
	}
	if key == "count" {
		count, err := strconv.Atoi(value)
		if err != nil || count < 1 || count > 100 {
			return errors.New("count must be 1-100")
		}
		if err := h.settings.Set(ctx, "article_count", strconv.Itoa(count)); err != nil {
			return err
		}
		return h.sender.Send(ctx, chatID, "Article count updated")
	}
	return errors.New("usage: /settings time HH:MM | /settings count N")
}

func (h *Handler) HandleStats(ctx context.Context, chatID int64) error {
	if h == nil || h.stats == nil || h.sender == nil {
		return errors.New("handler not initialized")
	}
	likes, err := h.stats.LikeCount(ctx)
	if err != nil {
		return err
	}
	if likes == 0 {
		return h.sender.Send(ctx, chatID, "No likes yet. React with 👍 to train preferences.")
	}
	tags, err := h.stats.TopTags(ctx, 10)
	if err != nil {
		return err
	}
	lines := []string{"Top interests:"}
	for _, tag := range tags {
		lines = append(lines, fmt.Sprintf("- %s (%.2f)", tag.Tag, tag.Weight))
	}
	lines = append(lines, fmt.Sprintf("Total likes: %d", likes))
	return h.sender.Send(ctx, chatID, strings.Join(lines, "\n"))
}
