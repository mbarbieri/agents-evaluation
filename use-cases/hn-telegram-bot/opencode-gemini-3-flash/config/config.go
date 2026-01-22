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
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
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

	// Validate DigestTime format HH:MM
	var h, m int
	if _, err := fmt.Sscanf(c.DigestTime, "%d:%d", &h, &m); err != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return fmt.Errorf("invalid digest_time format, expected HH:MM (00-23:00-59), got %s", c.DigestTime)
	}

	// Validate Timezone
	if _, err := time.LoadLocation(c.Timezone); err != nil {
		return fmt.Errorf("invalid timezone: %w", err)
	}

	if c.ArticleCount < 1 || c.ArticleCount > 100 {
		return fmt.Errorf("article_count must be between 1 and 100")
	}

	return nil
}
