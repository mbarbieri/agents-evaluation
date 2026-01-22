package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// 1. Create a temporary config file
	content := `
telegram_token: "test_token"
gemini_api_key: "test_key"
digest_time: "10:30"
article_count: 50
`
	tmpFile, err := os.CreateTemp("", "config_*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatal(err)
	}

	// 2. Set environment variable to point to temp file
	os.Setenv("HN_BOT_CONFIG", tmpFile.Name())
	defer os.Unsetenv("HN_BOT_CONFIG")

	// 3. Load config
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// 4. Assert values
	if cfg.TelegramToken != "test_token" {
		t.Errorf("Expected telegram_token 'test_token', got '%s'", cfg.TelegramToken)
	}
	if cfg.GeminiAPIKey != "test_key" {
		t.Errorf("Expected gemini_api_key 'test_key', got '%s'", cfg.GeminiAPIKey)
	}
	if cfg.DigestTime != "10:30" {
		t.Errorf("Expected digest_time '10:30', got '%s'", cfg.DigestTime)
	}
	if cfg.ArticleCount != 50 {
		t.Errorf("Expected article_count 50, got %d", cfg.ArticleCount)
	}
	// Check defaults
	if cfg.GeminiModel != "gemini-2.0-flash-lite" {
		t.Errorf("Expected default gemini_model, got '%s'", cfg.GeminiModel)
	}
	if cfg.Timezone != "UTC" {
		t.Errorf("Expected default timezone 'UTC', got '%s'", cfg.Timezone)
	}
	if cfg.TagDecayRate != 0.02 {
		t.Errorf("Expected default tag_decay_rate 0.02, got %f", cfg.TagDecayRate)
	}

	// 5. Test Environment Override for DB Path
	os.Setenv("HN_BOT_DB", "/tmp/override.db")
	defer os.Unsetenv("HN_BOT_DB")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DBPath != "/tmp/override.db" {
		t.Errorf("Expected DBPath override, got '%s'", cfg.DBPath)
	}
}

func TestValidation(t *testing.T) {
	// Test missing required fields
	content := `
digest_time: "09:00"
`
	tmpFile, err := os.CreateTemp("", "bad_config_*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Write([]byte(content))
	tmpFile.Close()

	os.Setenv("HN_BOT_CONFIG", tmpFile.Name())
	defer os.Unsetenv("HN_BOT_CONFIG")

	_, err = Load()
	if err == nil {
		t.Error("Expected error for missing required fields, got nil")
	}

	// Test invalid time format
	content = `
telegram_token: "t"
gemini_api_key: "k"
digest_time: "25:00"
`
	tmpFile2, _ := os.CreateTemp("", "bad_config_2_*.yaml")
	defer os.Remove(tmpFile2.Name())
	tmpFile2.Write([]byte(content))
	tmpFile2.Close()

	os.Setenv("HN_BOT_CONFIG", tmpFile2.Name())
	_, err = Load()
	if err == nil {
		t.Error("Expected error for invalid time format, got nil")
	}
}

func TestTimezoneValidation(t *testing.T) {
	content := `
telegram_token: "t"
gemini_api_key: "k"
timezone: "Invalid/Timezone"
`
	tmpFile, _ := os.CreateTemp("", "bad_tz_*.yaml")
	defer os.Remove(tmpFile.Name())
	tmpFile.Write([]byte(content))
	tmpFile.Close()

	os.Setenv("HN_BOT_CONFIG", tmpFile.Name())
	_, err := Load()
	if err == nil {
		t.Error("Expected error for invalid timezone, got nil")
	}
}
