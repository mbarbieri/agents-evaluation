package config

import (
	"fmt"
	"os"
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

	data, err := os.ReadFile(path)
	if err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	// Environment variable overrides
	if envToken := os.Getenv("HN_BOT_TELEGRAM_TOKEN"); envToken != "" {
		cfg.TelegramToken = envToken
	}
	if envKey := os.Getenv("HN_BOT_GEMINI_KEY"); envKey != "" {
		cfg.GeminiAPIKey = envKey
	}
	if envDB := os.Getenv("HN_BOT_DB"); envDB != "" {
		cfg.DBPath = envDB
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	if c.TelegramToken == "" {
		return fmt.Errorf("telegram_token is required")
	}
	if c.GeminiAPIKey == "" {
		return fmt.Errorf("gemini_api_key is required")
	}

	t, err := time.Parse("15:04", c.DigestTime)
	if err != nil {
		return fmt.Errorf("invalid digest_time: %w", err)
	}
	if t.Hour() < 0 || t.Hour() > 23 || t.Minute() < 0 || t.Minute() > 59 {
		return fmt.Errorf("digest_time out of range")
	}

	_, err = time.LoadLocation(c.Timezone)
	if err != nil {
		return fmt.Errorf("invalid timezone: %w", err)
	}

	return nil
}
