package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestLoadFromDefaults(t *testing.T) {
	path := writeTempConfig(t, "telegram_token: t\n"+"gemini_api_key: g\n")
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom error: %v", err)
	}
	if cfg.GeminiModel != defaultGeminiModel {
		t.Fatalf("expected default gemini_model %q, got %q", defaultGeminiModel, cfg.GeminiModel)
	}
	if cfg.DigestTime != defaultDigestTime {
		t.Fatalf("expected default digest_time %q, got %q", defaultDigestTime, cfg.DigestTime)
	}
	if cfg.Timezone != defaultTimezone {
		t.Fatalf("expected default timezone %q, got %q", defaultTimezone, cfg.Timezone)
	}
	if cfg.ArticleCount != defaultArticleCount {
		t.Fatalf("expected default article_count %d, got %d", defaultArticleCount, cfg.ArticleCount)
	}
	if cfg.FetchTimeoutSec != defaultTimeoutSecs {
		t.Fatalf("expected default fetch_timeout_secs %d, got %d", defaultTimeoutSecs, cfg.FetchTimeoutSec)
	}
	if cfg.TagDecayRate != defaultTagDecayRate {
		t.Fatalf("expected default tag_decay_rate %v, got %v", defaultTagDecayRate, cfg.TagDecayRate)
	}
	if cfg.MinTagWeight != defaultMinTagWeight {
		t.Fatalf("expected default min_tag_weight %v, got %v", defaultMinTagWeight, cfg.MinTagWeight)
	}
	if cfg.TagBoostOnLike != defaultTagBoost {
		t.Fatalf("expected default tag_boost_on_like %v, got %v", defaultTagBoost, cfg.TagBoostOnLike)
	}
	if cfg.DBPath != defaultDBPath {
		t.Fatalf("expected default db_path %q, got %q", defaultDBPath, cfg.DBPath)
	}
	if cfg.LogLevel != defaultLogLevel {
		t.Fatalf("expected default log_level %q, got %q", defaultLogLevel, cfg.LogLevel)
	}
}

func TestLoadFromMissingRequired(t *testing.T) {
	path := writeTempConfig(t, "gemini_api_key: g\n")
	_, err := LoadFrom(path)
	if err == nil {
		t.Fatalf("expected error for missing telegram_token")
	}

	path = writeTempConfig(t, "telegram_token: t\n")
	_, err = LoadFrom(path)
	if err == nil {
		t.Fatalf("expected error for missing gemini_api_key")
	}
}

func TestLoadFromInvalidTime(t *testing.T) {
	path := writeTempConfig(t, "telegram_token: t\n"+"gemini_api_key: g\n"+"digest_time: 9:00\n")
	_, err := LoadFrom(path)
	if err == nil {
		t.Fatalf("expected error for invalid digest_time")
	}
}

func TestLoadFromInvalidTimezone(t *testing.T) {
	path := writeTempConfig(t, "telegram_token: t\n"+"gemini_api_key: g\n"+"timezone: Not/AZone\n")
	_, err := LoadFrom(path)
	if err == nil {
		t.Fatalf("expected error for invalid timezone")
	}
}

func TestLoadFromEnvDBOverride(t *testing.T) {
	path := writeTempConfig(t, "telegram_token: t\n"+"gemini_api_key: g\n"+"db_path: ./custom.db\n")
	t.Setenv("HN_BOT_DB", "./override.db")
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom error: %v", err)
	}
	if cfg.DBPath != "./override.db" {
		t.Fatalf("expected db_path override, got %q", cfg.DBPath)
	}
}

func TestLoadUsesEnvConfigPath(t *testing.T) {
	path := writeTempConfig(t, "telegram_token: t\n"+"gemini_api_key: g\n")
	t.Setenv("HN_BOT_CONFIG", path)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.TelegramToken != "t" {
		t.Fatalf("expected telegram_token t, got %q", cfg.TelegramToken)
	}
}
