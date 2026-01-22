package config

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration
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

// Load reads configuration from file and environment variables
func Load() (*Config, error) {
	configPath := os.Getenv("HN_BOT_CONFIG")
	if configPath == "" {
		configPath = "./config.yaml"
	}

	cfg := &Config{
		// Defaults
		ChatID:          0,
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

	// Read file
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// If config file is missing but we are testing or have env vars, maybe ok?
			// Spec says: "The bot reads configuration from a YAML file."
			// And "Fail startup if telegram_token or gemini_api_key is missing"
			// So if file is missing, we proceed to check if fields are populated (maybe by magic?),
			// but practically we probably return error if file missing unless we support pure env var config.
			// Given instructions focus on YAML, we should expect file to exist.
			return nil, fmt.Errorf("config file not found at %s: %w", configPath, err)
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Environment overrides
	if dbPath := os.Getenv("HN_BOT_DB"); dbPath != "" {
		cfg.DBPath = dbPath
	}

	// Validation
	if cfg.TelegramToken == "" {
		return nil, fmt.Errorf("telegram_token is required")
	}
	if cfg.GeminiAPIKey == "" {
		return nil, fmt.Errorf("gemini_api_key is required")
	}

	// Time format validation HH:MM
	matched, _ := regexp.MatchString(`^([0-1][0-9]|2[0-3]):[0-5][0-9]$`, cfg.DigestTime)
	if !matched {
		return nil, fmt.Errorf("digest_time must be in HH:MM format (24h), got: %s", cfg.DigestTime)
	}

	// Timezone validation
	if _, err := time.LoadLocation(cfg.Timezone); err != nil {
		return nil, fmt.Errorf("invalid timezone '%s': %w", cfg.Timezone, err)
	}

	return cfg, nil
}
