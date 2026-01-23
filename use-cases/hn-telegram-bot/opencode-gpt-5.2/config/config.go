package config

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
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
}

func Load() (Config, error) {
	return LoadFromEnv(os.Getenv)
}

func LoadFromEnv(getenv func(string) string) (Config, error) {
	path := getenv("HN_BOT_CONFIG")
	if path == "" {
		path = "./config.yaml"
	}

	cfg, err := LoadFile(path)
	if err != nil {
		return Config{}, err
	}

	if override := getenv("HN_BOT_DB"); override != "" {
		cfg.DBPath = override
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func LoadFile(path string) (Config, error) {
	cfg := Defaults()

	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config yaml: %w", err)
	}
	return cfg, nil
}

func (c Config) Validate() error {
	var errs []error
	if c.TelegramToken == "" {
		errs = append(errs, errors.New("telegram_token is required"))
	}
	if c.GeminiAPIKey == "" {
		errs = append(errs, errors.New("gemini_api_key is required"))
	}
	if _, _, err := ParseHHMM(c.DigestTime); err != nil {
		errs = append(errs, fmt.Errorf("digest_time: %w", err))
	}
	if c.Timezone == "" {
		errs = append(errs, errors.New("timezone is required"))
	} else if _, err := time.LoadLocation(c.Timezone); err != nil {
		errs = append(errs, fmt.Errorf("timezone: %w", err))
	}
	if c.ArticleCount <= 0 || c.ArticleCount > 100 {
		errs = append(errs, errors.New("article_count must be between 1 and 100"))
	}
	if c.FetchTimeoutSec <= 0 {
		errs = append(errs, errors.New("fetch_timeout_secs must be > 0"))
	}
	if c.TagDecayRate < 0 || c.TagDecayRate >= 1 {
		errs = append(errs, errors.New("tag_decay_rate must be in [0,1)"))
	}
	if c.MinTagWeight <= 0 {
		errs = append(errs, errors.New("min_tag_weight must be > 0"))
	}
	if c.TagBoostOnLike <= 0 {
		errs = append(errs, errors.New("tag_boost_on_like must be > 0"))
	}
	if c.DBPath == "" {
		errs = append(errs, errors.New("db_path is required"))
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

var hhmmRe = regexp.MustCompile(`^(\d\d):(\d\d)$`)

func ParseHHMM(s string) (hour int, minute int, err error) {
	m := hhmmRe.FindStringSubmatch(s)
	if m == nil {
		return 0, 0, fmt.Errorf("invalid format (expected HH:MM)")
	}
	h, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid hour")
	}
	min, err := strconv.Atoi(m[2])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid minute")
	}
	if h < 0 || h > 23 {
		return 0, 0, fmt.Errorf("hour out of range")
	}
	if min < 0 || min > 59 {
		return 0, 0, fmt.Errorf("minute out of range")
	}
	return h, min, nil
}
