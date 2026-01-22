package config

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	TelegramToken    string  `yaml:"telegram_token"`
	GeminiAPIKey     string  `yaml:"gemini_api_key"`
	ChatID           int64   `yaml:"chat_id"`
	GeminiModel      string  `yaml:"gemini_model"`
	DigestTime       string  `yaml:"digest_time"`
	Timezone         string  `yaml:"timezone"`
	ArticleCount     int     `yaml:"article_count"`
	FetchTimeoutSecs int     `yaml:"fetch_timeout_secs"`
	TagDecayRate     float64 `yaml:"tag_decay_rate"`
	MinTagWeight     float64 `yaml:"min_tag_weight"`
	TagBoostOnLike   float64 `yaml:"tag_boost_on_like"`
	DBPath           string  `yaml:"db_path"`
	LogLevel         string  `yaml:"log_level"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := &Config{
		ChatID:           0,
		GeminiModel:      "gemini-2.0-flash-lite",
		DigestTime:       "09:00",
		Timezone:         "UTC",
		ArticleCount:     30,
		FetchTimeoutSecs: 10,
		TagDecayRate:     0.02,
		MinTagWeight:     0.1,
		TagBoostOnLike:   0.2,
		DBPath:           "./hn-bot.db",
		LogLevel:         "info",
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	// Environment variable overrides
	if dbPath := os.Getenv("HN_BOT_DB"); dbPath != "" {
		cfg.DBPath = dbPath
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.TelegramToken == "" {
		return fmt.Errorf("telegram_token is required")
	}

	if c.GeminiAPIKey == "" {
		return fmt.Errorf("gemini_api_key is required")
	}

	if err := validateTimeFormat(c.DigestTime); err != nil {
		return err
	}

	if err := validateTimezone(c.Timezone); err != nil {
		return err
	}

	return nil
}

func validateTimeFormat(timeStr string) error {
	// Must be HH:MM format with hours 0-23 and minutes 0-59
	re := regexp.MustCompile(`^([0-1][0-9]|2[0-3]):([0-5][0-9])$`)
	if !re.MatchString(timeStr) {
		return fmt.Errorf("invalid time format: %s (expected HH:MM with hours 0-23 and minutes 0-59)", timeStr)
	}
	return nil
}

func validateTimezone(tz string) error {
	_, err := time.LoadLocation(tz)
	if err != nil {
		return fmt.Errorf("invalid timezone: %s", tz)
	}
	return nil
}
