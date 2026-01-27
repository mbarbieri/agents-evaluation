package config

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration for the application
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

// timeRegex matches HH:MM format where HH is 00-23 and MM is 00-59
var timeRegex = regexp.MustCompile(`^([01]?[0-9]|2[0-3]):([0-5][0-9])$`)

// Load reads configuration from file and environment variables
func Load() (*Config, error) {
	configPath := os.Getenv("HN_BOT_CONFIG")
	if configPath == "" {
		configPath = "./config.yaml"
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

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

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply environment variable overrides
	if envDB := os.Getenv("HN_BOT_DB"); envDB != "" {
		cfg.DBPath = envDB
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks that all configuration values are valid
func (c *Config) Validate() error {
	if c.TelegramToken == "" {
		return fmt.Errorf("telegram_token is required")
	}

	if c.GeminiAPIKey == "" {
		return fmt.Errorf("gemini_api_key is required")
	}

	if !timeRegex.MatchString(c.DigestTime) {
		return fmt.Errorf("digest_time must be in HH:MM format with hours 0-23 and minutes 0-59")
	}

	if _, err := time.LoadLocation(c.Timezone); err != nil {
		return fmt.Errorf("timezone must be a valid IANA identifier: %w", err)
	}

	if c.ArticleCount < 1 || c.ArticleCount > 100 {
		return fmt.Errorf("article_count must be between 1 and 100")
	}

	if c.FetchTimeoutSecs < 1 {
		return fmt.Errorf("fetch_timeout_secs must be at least 1")
	}

	if c.TagDecayRate < 0 || c.TagDecayRate > 1 {
		return fmt.Errorf("tag_decay_rate must be between 0 and 1")
	}

	if c.MinTagWeight < 0 {
		return fmt.Errorf("min_tag_weight must be non-negative")
	}

	if c.TagBoostOnLike < 0 {
		return fmt.Errorf("tag_boost_on_like must be non-negative")
	}

	return nil
}
