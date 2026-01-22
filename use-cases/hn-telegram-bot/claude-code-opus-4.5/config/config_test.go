package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	// Create a minimal valid config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	content := `
telegram_token: "test-token"
gemini_api_key: "test-key"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Check defaults are applied
	if cfg.GeminiModel != "gemini-2.0-flash-lite" {
		t.Errorf("GeminiModel = %q, want %q", cfg.GeminiModel, "gemini-2.0-flash-lite")
	}
	if cfg.DigestTime != "09:00" {
		t.Errorf("DigestTime = %q, want %q", cfg.DigestTime, "09:00")
	}
	if cfg.Timezone != "UTC" {
		t.Errorf("Timezone = %q, want %q", cfg.Timezone, "UTC")
	}
	if cfg.ArticleCount != 30 {
		t.Errorf("ArticleCount = %d, want %d", cfg.ArticleCount, 30)
	}
	if cfg.FetchTimeoutSecs != 10 {
		t.Errorf("FetchTimeoutSecs = %d, want %d", cfg.FetchTimeoutSecs, 10)
	}
	if cfg.TagDecayRate != 0.02 {
		t.Errorf("TagDecayRate = %f, want %f", cfg.TagDecayRate, 0.02)
	}
	if cfg.MinTagWeight != 0.1 {
		t.Errorf("MinTagWeight = %f, want %f", cfg.MinTagWeight, 0.1)
	}
	if cfg.TagBoostOnLike != 0.2 {
		t.Errorf("TagBoostOnLike = %f, want %f", cfg.TagBoostOnLike, 0.2)
	}
	if cfg.DBPath != "./hn-bot.db" {
		t.Errorf("DBPath = %q, want %q", cfg.DBPath, "./hn-bot.db")
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
}

func TestLoadOverrideDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	content := `
telegram_token: "test-token"
gemini_api_key: "test-key"
gemini_model: "gemini-pro"
digest_time: "18:30"
timezone: "America/New_York"
article_count: 50
fetch_timeout_secs: 30
tag_decay_rate: 0.05
min_tag_weight: 0.2
tag_boost_on_like: 0.5
db_path: "/data/bot.db"
log_level: "debug"
chat_id: 123456
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.TelegramToken != "test-token" {
		t.Errorf("TelegramToken = %q, want %q", cfg.TelegramToken, "test-token")
	}
	if cfg.GeminiAPIKey != "test-key" {
		t.Errorf("GeminiAPIKey = %q, want %q", cfg.GeminiAPIKey, "test-key")
	}
	if cfg.GeminiModel != "gemini-pro" {
		t.Errorf("GeminiModel = %q, want %q", cfg.GeminiModel, "gemini-pro")
	}
	if cfg.DigestTime != "18:30" {
		t.Errorf("DigestTime = %q, want %q", cfg.DigestTime, "18:30")
	}
	if cfg.Timezone != "America/New_York" {
		t.Errorf("Timezone = %q, want %q", cfg.Timezone, "America/New_York")
	}
	if cfg.ArticleCount != 50 {
		t.Errorf("ArticleCount = %d, want %d", cfg.ArticleCount, 50)
	}
	if cfg.FetchTimeoutSecs != 30 {
		t.Errorf("FetchTimeoutSecs = %d, want %d", cfg.FetchTimeoutSecs, 30)
	}
	if cfg.TagDecayRate != 0.05 {
		t.Errorf("TagDecayRate = %f, want %f", cfg.TagDecayRate, 0.05)
	}
	if cfg.MinTagWeight != 0.2 {
		t.Errorf("MinTagWeight = %f, want %f", cfg.MinTagWeight, 0.2)
	}
	if cfg.TagBoostOnLike != 0.5 {
		t.Errorf("TagBoostOnLike = %f, want %f", cfg.TagBoostOnLike, 0.5)
	}
	if cfg.DBPath != "/data/bot.db" {
		t.Errorf("DBPath = %q, want %q", cfg.DBPath, "/data/bot.db")
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
	if cfg.ChatID != 123456 {
		t.Errorf("ChatID = %d, want %d", cfg.ChatID, 123456)
	}
}

func TestLoadMissingTelegramToken(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	content := `
gemini_api_key: "test-key"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for missing telegram_token")
	}
}

func TestLoadMissingGeminiAPIKey(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	content := `
telegram_token: "test-token"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for missing gemini_api_key")
	}
}

func TestLoadInvalidDigestTime(t *testing.T) {
	tests := []struct {
		name string
		time string
	}{
		{"invalid format", "9:00"},
		{"invalid hours", "25:00"},
		{"invalid minutes", "09:60"},
		{"text", "nine"},
		{"missing colon", "0900"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			content := `
telegram_token: "test-token"
gemini_api_key: "test-key"
digest_time: "` + tt.time + `"
`
			if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
				t.Fatal(err)
			}

			_, err := Load(configPath)
			if err == nil {
				t.Errorf("expected error for invalid digest_time %q", tt.time)
			}
		})
	}
}

func TestLoadValidDigestTimes(t *testing.T) {
	tests := []string{"00:00", "09:00", "12:30", "23:59"}

	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			content := `
telegram_token: "test-token"
gemini_api_key: "test-key"
digest_time: "` + tt + `"
`
			if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
				t.Fatal(err)
			}

			cfg, err := Load(configPath)
			if err != nil {
				t.Errorf("unexpected error for digest_time %q: %v", tt, err)
			}
			if cfg.DigestTime != tt {
				t.Errorf("DigestTime = %q, want %q", cfg.DigestTime, tt)
			}
		})
	}
}

func TestLoadInvalidTimezone(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	content := `
telegram_token: "test-token"
gemini_api_key: "test-key"
timezone: "Invalid/Zone"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for invalid timezone")
	}
}

func TestLoadFileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	content := `invalid: yaml: content:`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestEnvironmentVariableOverride(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	content := `
telegram_token: "test-token"
gemini_api_key: "test-key"
db_path: "/original/path.db"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Set environment variable
	os.Setenv("HN_BOT_DB", "/override/path.db")
	defer os.Unsetenv("HN_BOT_DB")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.DBPath != "/override/path.db" {
		t.Errorf("DBPath = %q, want %q (from env)", cfg.DBPath, "/override/path.db")
	}
}

func TestGetConfigPath(t *testing.T) {
	// Test default
	os.Unsetenv("HN_BOT_CONFIG")
	path := GetConfigPath()
	if path != "./config.yaml" {
		t.Errorf("GetConfigPath() = %q, want %q", path, "./config.yaml")
	}

	// Test with env var
	os.Setenv("HN_BOT_CONFIG", "/custom/config.yaml")
	defer os.Unsetenv("HN_BOT_CONFIG")
	path = GetConfigPath()
	if path != "/custom/config.yaml" {
		t.Errorf("GetConfigPath() = %q, want %q", path, "/custom/config.yaml")
	}
}
