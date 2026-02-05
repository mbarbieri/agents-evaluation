package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDefaults(t *testing.T) {
	d := Defaults()
	if d.GeminiModel != "gemini-2.0-flash-lite" {
		t.Errorf("expected default gemini model gemini-2.0-flash-lite, got %s", d.GeminiModel)
	}
	if d.DigestTime != "09:00" {
		t.Errorf("expected default digest time 09:00, got %s", d.DigestTime)
	}
	if d.Timezone != "UTC" {
		t.Errorf("expected default timezone UTC, got %s", d.Timezone)
	}
	if d.ArticleCount != 30 {
		t.Errorf("expected default article count 30, got %d", d.ArticleCount)
	}
	if d.FetchTimeoutSec != 10 {
		t.Errorf("expected default fetch timeout 10, got %d", d.FetchTimeoutSec)
	}
	if d.TagDecayRate != 0.02 {
		t.Errorf("expected default tag decay rate 0.02, got %f", d.TagDecayRate)
	}
	if d.MinTagWeight != 0.1 {
		t.Errorf("expected default min tag weight 0.1, got %f", d.MinTagWeight)
	}
	if d.TagBoostOnLike != 0.2 {
		t.Errorf("expected default tag boost 0.2, got %f", d.TagBoostOnLike)
	}
	if d.DBPath != "./hn-bot.db" {
		t.Errorf("expected default db path ./hn-bot.db, got %s", d.DBPath)
	}
	if d.LogLevel != "info" {
		t.Errorf("expected default log level info, got %s", d.LogLevel)
	}
}

func TestLoad_ValidConfig(t *testing.T) {
	path := writeConfig(t, `
telegram_token: "test-token"
gemini_api_key: "test-key"
chat_id: 12345
digest_time: "18:30"
timezone: "Europe/Rome"
article_count: 20
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.TelegramToken != "test-token" {
		t.Errorf("expected telegram_token test-token, got %s", cfg.TelegramToken)
	}
	if cfg.GeminiAPIKey != "test-key" {
		t.Errorf("expected gemini_api_key test-key, got %s", cfg.GeminiAPIKey)
	}
	if cfg.ChatID != 12345 {
		t.Errorf("expected chat_id 12345, got %d", cfg.ChatID)
	}
	if cfg.DigestTime != "18:30" {
		t.Errorf("expected digest_time 18:30, got %s", cfg.DigestTime)
	}
	if cfg.Timezone != "Europe/Rome" {
		t.Errorf("expected timezone Europe/Rome, got %s", cfg.Timezone)
	}
	if cfg.ArticleCount != 20 {
		t.Errorf("expected article_count 20, got %d", cfg.ArticleCount)
	}
	// Defaults should be preserved for unset fields
	if cfg.GeminiModel != "gemini-2.0-flash-lite" {
		t.Errorf("expected default gemini model, got %s", cfg.GeminiModel)
	}
}

func TestLoad_MissingTelegramToken(t *testing.T) {
	path := writeConfig(t, `
gemini_api_key: "test-key"
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing telegram_token")
	}
}

func TestLoad_MissingGeminiKey(t *testing.T) {
	path := writeConfig(t, `
telegram_token: "test-token"
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing gemini_api_key")
	}
}

func TestLoad_InvalidTime(t *testing.T) {
	path := writeConfig(t, `
telegram_token: "test-token"
gemini_api_key: "test-key"
digest_time: "25:00"
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid time")
	}
}

func TestLoad_InvalidTimezone(t *testing.T) {
	path := writeConfig(t, `
telegram_token: "test-token"
gemini_api_key: "test-key"
timezone: "Invalid/Zone"
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid timezone")
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	path := writeConfig(t, `
telegram_token: "test
  invalid: yaml: [
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoad_EnvConfigPath(t *testing.T) {
	path := writeConfig(t, `
telegram_token: "env-token"
gemini_api_key: "env-key"
`)
	t.Setenv("HN_BOT_CONFIG", path)
	cfg, err := Load("wrong-path.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.TelegramToken != "env-token" {
		t.Errorf("expected env-token, got %s", cfg.TelegramToken)
	}
}

func TestLoad_EnvDBPath(t *testing.T) {
	path := writeConfig(t, `
telegram_token: "test-token"
gemini_api_key: "test-key"
`)
	t.Setenv("HN_BOT_DB", "/custom/db.sqlite")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DBPath != "/custom/db.sqlite" {
		t.Errorf("expected /custom/db.sqlite, got %s", cfg.DBPath)
	}
}

func TestValidateTime(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"00:00", true},
		{"09:00", true},
		{"23:59", true},
		{"12:30", true},
		{"24:00", false},
		{"23:60", false},
		{"9:00", false},
		{"abc", false},
		{"12:0a", false},
		{"", false},
	}

	for _, tt := range tests {
		err := ValidateTime(tt.input)
		if tt.valid && err != nil {
			t.Errorf("ValidateTime(%q) returned unexpected error: %v", tt.input, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("ValidateTime(%q) expected error, got nil", tt.input)
		}
	}
}
