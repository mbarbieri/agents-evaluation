package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all application configuration.
type Config struct {
	TelegramToken   string  `yaml:"telegram_token"`
	GeminiAPIKey    string  `yaml:"gemini_api_key"`
	ChatID          int64   `yaml:"chat_id"`
	GeminiModel     string  `yaml:"gemini_model"`
	DigestTime      string  `yaml:"digest_time"`
	Timezone        string  `yaml:"timezone"`
	ArticleCount    int     `yaml:"article_count"`
	FetchTimeoutSec int     `yaml:"fetch_timeout_secs"`
	TagDecayRate    float64 `yaml:"tag_decay_rate"`
	MinTagWeight    float64 `yaml:"min_tag_weight"`
	TagBoostOnLike  float64 `yaml:"tag_boost_on_like"`
	DBPath          string  `yaml:"db_path"`
	LogLevel        string  `yaml:"log_level"`
}

// Defaults returns a Config with all default values set.
func Defaults() Config {
	return Config{
		GeminiModel:     "gemini-2.0-flash-lite",
		DigestTime:      "09:00",
		Timezone:        "UTC",
		ArticleCount:    30,
		FetchTimeoutSec: 10,
		TagDecayRate:    0.02,
		MinTagWeight:    0.1,
		TagBoostOnLike:  0.2,
		DBPath:          "./hn-bot.db",
		LogLevel:        "info",
	}
}

// Load reads a YAML config file and returns a validated Config.
// Environment variables HN_BOT_CONFIG and HN_BOT_DB can override file path and db path.
func Load(path string) (Config, error) {
	if envPath := os.Getenv("HN_BOT_CONFIG"); envPath != "" {
		path = envPath
	}

	cfg := Defaults()

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("reading config file %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config file: %w", err)
	}

	if envDB := os.Getenv("HN_BOT_DB"); envDB != "" {
		cfg.DBPath = envDB
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// Validate checks that required fields are present and values are valid.
func (c *Config) Validate() error {
	if c.TelegramToken == "" {
		return fmt.Errorf("telegram_token is required")
	}
	if c.GeminiAPIKey == "" {
		return fmt.Errorf("gemini_api_key is required")
	}

	if err := ValidateTime(c.DigestTime); err != nil {
		return err
	}

	if _, err := time.LoadLocation(c.Timezone); err != nil {
		return fmt.Errorf("invalid timezone %q: %w", c.Timezone, err)
	}

	return nil
}

// ValidateTime checks that a time string is in valid HH:MM 24-hour format.
func ValidateTime(t string) error {
	if len(t) != 5 || t[2] != ':' {
		return fmt.Errorf("invalid time format %q: must be HH:MM", t)
	}

	hour := (int(t[0]-'0') * 10) + int(t[1]-'0')
	minute := (int(t[3]-'0') * 10) + int(t[4]-'0')

	if t[0] < '0' || t[0] > '9' || t[1] < '0' || t[1] > '9' ||
		t[3] < '0' || t[3] > '9' || t[4] < '0' || t[4] > '9' {
		return fmt.Errorf("invalid time format %q: must be HH:MM", t)
	}

	if hour < 0 || hour > 23 {
		return fmt.Errorf("invalid time %q: hour must be 0-23", t)
	}
	if minute < 0 || minute > 59 {
		return fmt.Errorf("invalid time %q: minute must be 0-59", t)
	}

	return nil
}
