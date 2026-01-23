package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromAppliesDefaultsAndOverrides(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	content := []byte("telegram_token: token\n" +
		"gemini_api_key: key\n" +
		"timezone: UTC\n")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := os.Setenv("HN_BOT_DB", "./override.db"); err != nil {
		t.Fatalf("set env: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Unsetenv("HN_BOT_DB")
	})

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}

	if cfg.GeminiModel != defaultGeminiModel {
		t.Fatalf("expected default gemini model, got %q", cfg.GeminiModel)
	}
	if cfg.DigestTime != defaultDigestTime {
		t.Fatalf("expected default digest time, got %q", cfg.DigestTime)
	}
	if cfg.ArticleCount != defaultArticleCount {
		t.Fatalf("expected default article count, got %d", cfg.ArticleCount)
	}
	if cfg.DBPath != "./override.db" {
		t.Fatalf("expected env override db path, got %q", cfg.DBPath)
	}
}

func TestLoadFromMissingFile(t *testing.T) {
	t.Parallel()

	_, err := LoadFrom("/does/not/exist.yaml")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateRejectsMissingRequiredFields(t *testing.T) {
	t.Parallel()

	cfg := Defaults()
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error for missing required fields")
	}
}

func TestValidateRejectsBadTime(t *testing.T) {
	t.Parallel()

	cfg := Defaults()
	cfg.TelegramToken = "token"
	cfg.GeminiAPIKey = "key"
	cfg.DigestTime = "99:00"
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error for invalid digest time")
	}
}

func TestValidateRejectsBadTimezone(t *testing.T) {
	t.Parallel()

	cfg := Defaults()
	cfg.TelegramToken = "token"
	cfg.GeminiAPIKey = "key"
	cfg.Timezone = "Nope/Nowhere"
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error for invalid timezone")
	}
}

func TestValidateAcceptsValidConfig(t *testing.T) {
	t.Parallel()

	cfg := Defaults()
	cfg.TelegramToken = "token"
	cfg.GeminiAPIKey = "key"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid config, got %v", err)
	}
}
