package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration
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

// Load reads the configuration from file and environment variables
func Load() (*Config, error) {
	configPath := os.Getenv("HN_BOT_CONFIG")
	if configPath == "" {
		configPath = "./config.yaml"
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
	}

	// Apply defaults
	applyDefaults(&cfg)

	// Validate
	if err := validate(&cfg); err != nil {
		return nil, err
	}

	// Override db_path from env
	if dbPath := os.Getenv("HN_BOT_DB"); dbPath != "" {
		cfg.DBPath = dbPath
	}

	return &cfg, nil
}

// applyDefaults sets default values for missing fields
func applyDefaults(cfg *Config) {
	if cfg.GeminiModel == "" {
		cfg.GeminiModel = "gemini-2.0-flash-lite"
	}
	if cfg.DigestTime == "" {
		cfg.DigestTime = "09:00"
	}
	if cfg.Timezone == "" {
		cfg.Timezone = "UTC"
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
}

// validate checks the configuration for correctness
func validate(cfg *Config) error {
	if cfg.TelegramToken == "" {
		return fmt.Errorf("telegram_token is required")
	}
	if cfg.GeminiAPIKey == "" {
		return fmt.Errorf("gemini_api_key is required")
	}

	// Validate digest_time format
	if _, err := time.Parse("15:04", cfg.DigestTime); err != nil {
		return fmt.Errorf("invalid digest_time format, expected HH:MM: %w", err)
	}

	// Validate timezone
	if _, err := time.LoadLocation(cfg.Timezone); err != nil {
		return fmt.Errorf("invalid timezone: %w", err)
	}

	return nil
}
