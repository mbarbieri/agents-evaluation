package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_FileNotFound(t *testing.T) {
	cfg, err := LoadConfig("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if cfg != nil {
		t.Error("Expected nil config on error")
	}
}

func TestLoadConfig_ValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
telegram_token: "test-token"
gemini_api_key: "test-api-key"
chat_id: 12345
digest_time: "09:30"
timezone: "America/New_York"
article_count: 25
fetch_timeout_secs: 15
tag_decay_rate: 0.03
min_tag_weight: 0.2
tag_boost_on_like: 0.25
db_path: "./test.db"
log_level: "debug"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if cfg.TelegramToken != "test-token" {
		t.Errorf("Expected telegram_token 'test-token', got '%s'", cfg.TelegramToken)
	}
	if cfg.GeminiAPIKey != "test-api-key" {
		t.Errorf("Expected gemini_api_key 'test-api-key', got '%s'", cfg.GeminiAPIKey)
	}
	if cfg.ChatID != 12345 {
		t.Errorf("Expected chat_id 12345, got %d", cfg.ChatID)
	}
	if cfg.DigestTime != "09:30" {
		t.Errorf("Expected digest_time '09:30', got '%s'", cfg.DigestTime)
	}
	if cfg.Timezone != "America/New_York" {
		t.Errorf("Expected timezone 'America/New_York', got '%s'", cfg.Timezone)
	}
	if cfg.ArticleCount != 25 {
		t.Errorf("Expected article_count 25, got %d", cfg.ArticleCount)
	}
	if cfg.FetchTimeoutSecs != 15 {
		t.Errorf("Expected fetch_timeout_secs 15, got %d", cfg.FetchTimeoutSecs)
	}
	if cfg.TagDecayRate != 0.03 {
		t.Errorf("Expected tag_decay_rate 0.03, got %f", cfg.TagDecayRate)
	}
	if cfg.MinTagWeight != 0.2 {
		t.Errorf("Expected min_tag_weight 0.2, got %f", cfg.MinTagWeight)
	}
	if cfg.TagBoostOnLike != 0.25 {
		t.Errorf("Expected tag_boost_on_like 0.25, got %f", cfg.TagBoostOnLike)
	}
	if cfg.DBPath != "./test.db" {
		t.Errorf("Expected db_path './test.db', got '%s'", cfg.DBPath)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("Expected log_level 'debug', got '%s'", cfg.LogLevel)
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
telegram_token: "test-token"
gemini_api_key: "test-api-key"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if cfg.ChatID != 0 {
		t.Errorf("Expected default chat_id 0, got %d", cfg.ChatID)
	}
	if cfg.GeminiModel != "gemini-2.0-flash-lite" {
		t.Errorf("Expected default gemini_model 'gemini-2.0-flash-lite', got '%s'", cfg.GeminiModel)
	}
	if cfg.DigestTime != "09:00" {
		t.Errorf("Expected default digest_time '09:00', got '%s'", cfg.DigestTime)
	}
	if cfg.Timezone != "UTC" {
		t.Errorf("Expected default timezone 'UTC', got '%s'", cfg.Timezone)
	}
	if cfg.ArticleCount != 30 {
		t.Errorf("Expected default article_count 30, got %d", cfg.ArticleCount)
	}
	if cfg.FetchTimeoutSecs != 10 {
		t.Errorf("Expected default fetch_timeout_secs 10, got %d", cfg.FetchTimeoutSecs)
	}
	if cfg.TagDecayRate != 0.02 {
		t.Errorf("Expected default tag_decay_rate 0.02, got %f", cfg.TagDecayRate)
	}
	if cfg.MinTagWeight != 0.1 {
		t.Errorf("Expected default min_tag_weight 0.1, got %f", cfg.MinTagWeight)
	}
	if cfg.TagBoostOnLike != 0.2 {
		t.Errorf("Expected default tag_boost_on_like 0.2, got %f", cfg.TagBoostOnLike)
	}
	if cfg.DBPath != "./hn-bot.db" {
		t.Errorf("Expected default db_path './hn-bot.db', got '%s'", cfg.DBPath)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("Expected default log_level 'info', got '%s'", cfg.LogLevel)
	}
}

func TestLoadConfig_MissingRequiredFields(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	testCases := []struct {
		name    string
		content string
	}{
		{"missing_telegram_token", "gemini_api_key: \"test-key\""},
		{"missing_gemini_api_key", "telegram_token: \"test-token\""},
		{"both_missing", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := os.WriteFile(configPath, []byte(tc.content), 0644)
			if err != nil {
				t.Fatalf("Failed to write test config: %v", err)
			}

			_, err = LoadConfig(configPath)
			if err == nil {
				t.Error("Expected error for missing required fields")
			}
		})
	}
}

func TestLoadConfig_EnvironmentOverride(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
telegram_token: "config-token"
gemini_api_key: "config-api-key"
db_path: "./config.db"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	os.Setenv("HN_BOT_DB", "./override.db")
	defer os.Unsetenv("HN_BOT_DB")

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if cfg.DBPath != "./override.db" {
		t.Errorf("Expected db_path './override.db' from env, got '%s'", cfg.DBPath)
	}
}

func TestValidateConfig_Valid(t *testing.T) {
	cfg := &Config{
		TelegramToken:    "test-token",
		GeminiAPIKey:     "test-api-key",
		DigestTime:       "09:30",
		Timezone:         "America/New_York",
		ArticleCount:     50,
		FetchTimeoutSecs: 15,
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Expected valid config, got error: %v", err)
	}
}

func TestValidateConfig_InvalidTimeFormat(t *testing.T) {
	testCases := []struct {
		name       string
		digestTime string
	}{
		{"invalid_hour", "24:00"},
		{"invalid_minute", "09:60"},
		{"bad_format", "9:30"},
		{"bad_format_2", "0930"},
		{"empty", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				TelegramToken: "test-token",
				GeminiAPIKey:  "test-api-key",
				DigestTime:    tc.digestTime,
				Timezone:      "UTC",
			}

			err := cfg.Validate()
			if err == nil {
				t.Error("Expected error for invalid time format")
			}
		})
	}
}

func TestValidateConfig_InvalidTimezone(t *testing.T) {
	cfg := &Config{
		TelegramToken: "test-token",
		GeminiAPIKey:  "test-api-key",
		DigestTime:    "09:00",
		Timezone:      "Invalid/Timezone",
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected error for invalid timezone")
	}
}

func TestValidateConfig_InvalidArticleCount(t *testing.T) {
	cfg := &Config{
		TelegramToken: "test-token",
		GeminiAPIKey:  "test-api-key",
		DigestTime:    "09:00",
		Timezone:      "UTC",
		ArticleCount:  0,
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected error for invalid article count")
	}
}
