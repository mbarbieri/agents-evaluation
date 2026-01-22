package config

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all application configuration.
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

// digestTimeRegex validates HH:MM format with proper ranges.
var digestTimeRegex = regexp.MustCompile(`^([01][0-9]|2[0-3]):([0-5][0-9])$`)

// Load reads configuration from a YAML file and applies defaults.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config yaml: %w", err)
	}

	applyDefaults(cfg)
	applyEnvironmentOverrides(cfg)

	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return cfg, nil
}

// GetConfigPath returns the config file path from environment or default.
func GetConfigPath() string {
	if path := os.Getenv("HN_BOT_CONFIG"); path != "" {
		return path
	}
	return "./config.yaml"
}

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

func applyEnvironmentOverrides(cfg *Config) {
	if dbPath := os.Getenv("HN_BOT_DB"); dbPath != "" {
		cfg.DBPath = dbPath
	}
}

func validate(cfg *Config) error {
	if cfg.TelegramToken == "" {
		return fmt.Errorf("telegram_token is required")
	}
	if cfg.GeminiAPIKey == "" {
		return fmt.Errorf("gemini_api_key is required")
	}
	if !digestTimeRegex.MatchString(cfg.DigestTime) {
		return fmt.Errorf("digest_time must be in HH:MM format (00:00-23:59), got %q", cfg.DigestTime)
	}
	if _, err := time.LoadLocation(cfg.Timezone); err != nil {
		return fmt.Errorf("invalid timezone %q: %w", cfg.Timezone, err)
	}
	return nil
}
