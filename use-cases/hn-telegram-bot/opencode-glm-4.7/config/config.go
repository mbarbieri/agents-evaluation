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
	if path == "" {
		path = os.Getenv("HN_BOT_CONFIG")
		if path == "" {
			path = "./config.yaml"
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if cfg.TelegramToken == "" {
		return nil, fmt.Errorf("telegram_token is required")
	}

	if cfg.GeminiAPIKey == "" {
		return nil, fmt.Errorf("gemini_api_key is required")
	}

	if cfg.ChatID == 0 {
		cfg.ChatID = 0
	}

	if cfg.GeminiModel == "" {
		cfg.GeminiModel = "gemini-2.0-flash-lite"
	}

	if cfg.DigestTime == "" {
		cfg.DigestTime = "09:00"
	}

	if err := validateTimeFormat(cfg.DigestTime); err != nil {
		return nil, err
	}

	if cfg.Timezone == "" {
		cfg.Timezone = "UTC"
	}

	if _, err := time.LoadLocation(cfg.Timezone); err != nil {
		return nil, fmt.Errorf("invalid timezone: %w", err)
	}

	if cfg.ArticleCount == 0 {
		cfg.ArticleCount = 30
	}

	if cfg.FetchTimeoutSecs == 0 {
		cfg.FetchTimeoutSecs = 10
	}

	if cfg.TagDecayRate == 0 {
		cfg.TagDecayRate = 0.02
	}

	if cfg.MinTagWeight == 0 {
		cfg.MinTagWeight = 0.1
	}

	if cfg.TagBoostOnLike == 0 {
		cfg.TagBoostOnLike = 0.2
	}

	if cfg.DBPath == "" {
		cfg.DBPath = "./hn-bot.db"
	}

	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}

	if dbPath := os.Getenv("HN_BOT_DB"); dbPath != "" {
		cfg.DBPath = dbPath
	}

	return &cfg, nil
}

func validateTimeFormat(timeStr string) error {
	var hour, minute int
	_, err := fmt.Sscanf(timeStr, "%d:%d", &hour, &minute)
	if err != nil {
		return fmt.Errorf("digest_time must be in HH:MM format")
	}

	if hour < 0 || hour > 23 {
		return fmt.Errorf("digest_time must be in HH:MM format")
	}

	if minute < 0 || minute > 59 {
		return fmt.Errorf("digest_time must be in HH:MM format")
	}

	return nil
}
