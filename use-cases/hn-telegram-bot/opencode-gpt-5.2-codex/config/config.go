package config

import (
	"errors"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultGeminiModel    = "gemini-2.0-flash-lite"
	defaultDigestTime     = "09:00"
	defaultTimezone       = "UTC"
	defaultArticleCount   = 30
	defaultFetchTimeout   = 10
	defaultTagDecayRate   = 0.02
	defaultMinTagWeight   = 0.1
	defaultTagBoostOnLike = 0.2
	defaultDBPath         = "./hn-bot.db"
	defaultLogLevel       = "info"
)

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

func Defaults() Config {
	return Config{
		GeminiModel:     defaultGeminiModel,
		DigestTime:      defaultDigestTime,
		Timezone:        defaultTimezone,
		ArticleCount:    defaultArticleCount,
		FetchTimeoutSec: defaultFetchTimeout,
		TagDecayRate:    defaultTagDecayRate,
		MinTagWeight:    defaultMinTagWeight,
		TagBoostOnLike:  defaultTagBoostOnLike,
		DBPath:          defaultDBPath,
		LogLevel:        defaultLogLevel,
	}
}

func Load() (Config, error) {
	path := os.Getenv("HN_BOT_CONFIG")
	if path == "" {
		path = "./config.yaml"
	}
	return LoadFrom(path)
}

func LoadFrom(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	cfg := Defaults()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	applyEnvOverrides(&cfg)

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if cfg == nil {
		return
	}
	if db := os.Getenv("HN_BOT_DB"); db != "" {
		cfg.DBPath = db
	}
}

func (c Config) Validate() error {
	if c.TelegramToken == "" {
		return errors.New("telegram_token is required")
	}
	if c.GeminiAPIKey == "" {
		return errors.New("gemini_api_key is required")
	}
	if _, err := time.Parse("15:04", c.DigestTime); err != nil {
		return fmt.Errorf("digest_time must be HH:MM: %w", err)
	}
	if _, err := time.LoadLocation(c.Timezone); err != nil {
		return fmt.Errorf("timezone invalid: %w", err)
	}
	if c.ArticleCount <= 0 {
		return errors.New("article_count must be positive")
	}
	if c.FetchTimeoutSec <= 0 {
		return errors.New("fetch_timeout_secs must be positive")
	}
	if c.TagDecayRate < 0 || c.TagDecayRate > 1 {
		return errors.New("tag_decay_rate must be between 0 and 1")
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
	if c.GeminiModel == "" {
		return errors.New("gemini_model must not be empty")
	}
	if c.LogLevel == "" {
		return errors.New("log_level must not be empty")
	}
	return nil
}
