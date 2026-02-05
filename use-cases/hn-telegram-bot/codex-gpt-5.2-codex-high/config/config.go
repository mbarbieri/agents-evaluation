package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultConfigPath   = "./config.yaml"
	defaultGeminiModel  = "gemini-2.0-flash-lite"
	defaultDigestTime   = "09:00"
	defaultTimezone     = "UTC"
	defaultArticleCount = 30
	defaultTimeoutSecs  = 10
	defaultTagDecayRate = 0.02
	defaultMinTagWeight = 0.1
	defaultTagBoost     = 0.2
	defaultDBPath       = "./hn-bot.db"
	defaultLogLevel     = "info"
)

var timeHHMM = regexp.MustCompile(`^(?:[01]\d|2[0-3]):[0-5]\d$`)

// Config defines all runtime configuration.
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

// Default returns a Config populated with default values.
func Default() Config {
	return Config{
		ChatID:          0,
		GeminiModel:     defaultGeminiModel,
		DigestTime:      defaultDigestTime,
		Timezone:        defaultTimezone,
		ArticleCount:    defaultArticleCount,
		FetchTimeoutSec: defaultTimeoutSecs,
		TagDecayRate:    defaultTagDecayRate,
		MinTagWeight:    defaultMinTagWeight,
		TagBoostOnLike:  defaultTagBoost,
		DBPath:          defaultDBPath,
		LogLevel:        defaultLogLevel,
	}
}

// Load reads configuration from the path in HN_BOT_CONFIG or the default path.
func Load() (Config, error) {
	path := os.Getenv("HN_BOT_CONFIG")
	if path == "" {
		path = defaultConfigPath
	}
	return LoadFrom(path)
}

// LoadFrom reads configuration from a specific file path.
func LoadFrom(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config yaml: %w", err)
	}

	if override := os.Getenv("HN_BOT_DB"); override != "" {
		cfg.DBPath = override
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate ensures configuration is complete and valid.
func (c Config) Validate() error {
	if c.TelegramToken == "" {
		return errors.New("telegram_token is required")
	}
	if c.GeminiAPIKey == "" {
		return errors.New("gemini_api_key is required")
	}
	if !timeHHMM.MatchString(c.DigestTime) {
		return fmt.Errorf("digest_time must be HH:MM in 24-hour format: %s", c.DigestTime)
	}
	if _, err := time.LoadLocation(c.Timezone); err != nil {
		return fmt.Errorf("timezone must be a valid IANA identifier: %w", err)
	}
	if c.ArticleCount <= 0 {
		return errors.New("article_count must be positive")
	}
	if c.FetchTimeoutSec <= 0 {
		return errors.New("fetch_timeout_secs must be positive")
	}
	if c.TagDecayRate < 0 || c.TagDecayRate >= 1 {
		return errors.New("tag_decay_rate must be in [0,1)")
	}
	if c.MinTagWeight <= 0 {
		return errors.New("min_tag_weight must be positive")
	}
	if c.TagBoostOnLike < 0 {
		return errors.New("tag_boost_on_like must be non-negative")
	}
	if c.DBPath == "" {
		return errors.New("db_path must not be empty")
	}
	return nil
}
