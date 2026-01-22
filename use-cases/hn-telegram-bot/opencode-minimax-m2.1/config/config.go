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

func (c *Config) setDefaults() {
	if c.GeminiModel == "" {
		c.GeminiModel = "gemini-2.0-flash-lite"
	}
	if c.DigestTime == "" {
		c.DigestTime = "09:00"
	}
	if c.Timezone == "" {
		c.Timezone = "UTC"
	}
	if c.ArticleCount == 0 {
		c.ArticleCount = 30
	}
	if c.FetchTimeoutSecs == 0 {
		c.FetchTimeoutSecs = 10
	}
	if c.TagDecayRate == 0 {
		c.TagDecayRate = 0.02
	}
	if c.MinTagWeight == 0 {
		c.MinTagWeight = 0.1
	}
	if c.TagBoostOnLike == 0 {
		c.TagBoostOnLike = 0.2
	}
	if c.DBPath == "" {
		c.DBPath = "./hn-bot.db"
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	cfg.setDefaults()

	if dbPath := os.Getenv("HN_BOT_DB"); dbPath != "" {
		cfg.DBPath = dbPath
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	if c.TelegramToken == "" {
		return fmt.Errorf("telegram_token is required")
	}
	if c.GeminiAPIKey == "" {
		return fmt.Errorf("gemini_api_key is required")
	}

	if err := c.validateTimeFormat(); err != nil {
		return fmt.Errorf("digest_time validation failed: %w", err)
	}

	if err := c.validateTimezone(); err != nil {
		return fmt.Errorf("timezone validation failed: %w", err)
	}

	if c.ArticleCount <= 0 {
		return fmt.Errorf("article_count must be positive, got %d", c.ArticleCount)
	}

	return nil
}

func (c *Config) validateTimeFormat() error {
	if len(c.DigestTime) != 5 {
		return fmt.Errorf("invalid format, expected HH:MM, got '%s'", c.DigestTime)
	}

	var hour, minute int
	_, err := fmt.Sscanf(c.DigestTime, "%d:%d", &hour, &minute)
	if err != nil {
		return fmt.Errorf("failed to parse time: %w", err)
	}

	if hour < 0 || hour > 23 {
		return fmt.Errorf("hour must be 0-23, got %d", hour)
	}
	if minute < 0 || minute > 59 {
		return fmt.Errorf("minute must be 0-59, got %d", minute)
	}

	return nil
}

func (c *Config) validateTimezone() error {
	_, err := time.LoadLocation(c.Timezone)
	if err != nil {
		return fmt.Errorf("failed to load timezone '%s': %w", c.Timezone, err)
	}
	return nil
}
