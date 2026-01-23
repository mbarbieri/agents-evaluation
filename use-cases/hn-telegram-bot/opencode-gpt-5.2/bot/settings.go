package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"hn-telegram-bot/config"
)

type SettingsStore interface {
	GetSetting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key string, value string) error
}

type SchedulerUpdater interface {
	Update(settings interface {
		DigestTime() string
		Timezone() string
	}) error
}

type Settings struct {
	mu           sync.RWMutex
	chatID       int64
	digestTime   string
	timezone     string
	articleCount int
}

func NewSettings(cfg config.Config) *Settings {
	return &Settings{
		chatID:       cfg.ChatID,
		digestTime:   cfg.DigestTime,
		timezone:     cfg.Timezone,
		articleCount: cfg.ArticleCount,
	}
}

func (s *Settings) ChatID() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.chatID
}

func (s *Settings) DigestTime() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.digestTime
}

func (s *Settings) Timezone() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.timezone
}

func (s *Settings) ArticleCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.articleCount
}

func (s *Settings) LoadFromStore(ctx context.Context, st SettingsStore) {
	// Best-effort; missing settings are fine.
	if v, err := st.GetSetting(ctx, "chat_id"); err == nil {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			s.mu.Lock()
			s.chatID = id
			s.mu.Unlock()
		}
	}
	if v, err := st.GetSetting(ctx, "digest_time"); err == nil {
		s.mu.Lock()
		s.digestTime = v
		s.mu.Unlock()
	}
	if v, err := st.GetSetting(ctx, "article_count"); err == nil {
		if n, err := strconv.Atoi(v); err == nil {
			s.mu.Lock()
			s.articleCount = n
			s.mu.Unlock()
		}
	}
}

func (s *Settings) HandleStart(ctx context.Context, sender Sender, st SettingsStore, chatID int64) error {
	s.mu.Lock()
	s.chatID = chatID
	s.mu.Unlock()
	if err := st.SetSetting(ctx, "chat_id", strconv.FormatInt(chatID, 10)); err != nil {
		// caller logs
	}
	msg := "Welcome!\n\nCommands:\n/fetch - fetch digest now\n/settings - view or update settings\n/stats - view learned interests\n\nReact with \U0001F44D on articles to train preferences."
	return sender.SendText(ctx, chatID, msg)
}

func (s *Settings) HandleSettings(ctx context.Context, sender Sender, st SettingsStore, sched SchedulerUpdater, chatID int64, args string) error {
	args = strings.TrimSpace(args)
	if args == "" {
		return sender.SendText(ctx, chatID, fmt.Sprintf("Digest time: %s\nArticle count: %d", s.DigestTime(), s.ArticleCount()))
	}

	fields := strings.Fields(args)
	if len(fields) != 2 {
		return sender.SendText(ctx, chatID, "Usage: /settings time HH:MM | /settings count N")
	}
	switch fields[0] {
	case "time":
		if _, _, err := config.ParseHHMM(fields[1]); err != nil {
			return sender.SendText(ctx, chatID, "Usage: /settings time HH:MM")
		}
		s.mu.Lock()
		s.digestTime = fields[1]
		s.mu.Unlock()
		_ = st.SetSetting(ctx, "digest_time", fields[1])
		if sched != nil {
			_ = sched.Update(s)
		}
		return sender.SendText(ctx, chatID, "Updated digest time.")
	case "count":
		n, err := strconv.Atoi(fields[1])
		if err != nil || n < 1 || n > 100 {
			return sender.SendText(ctx, chatID, "Usage: /settings count N (1-100)")
		}
		s.mu.Lock()
		s.articleCount = n
		s.mu.Unlock()
		_ = st.SetSetting(ctx, "article_count", strconv.Itoa(n))
		return sender.SendText(ctx, chatID, "Updated article count.")
	default:
		return sender.SendText(ctx, chatID, "Usage: /settings time HH:MM | /settings count N")
	}
}
