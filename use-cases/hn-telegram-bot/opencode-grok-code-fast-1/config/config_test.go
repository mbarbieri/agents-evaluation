package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ValidConfig(t *testing.T) {
	// Create a temp config file
	content := `
telegram_token: "test_token"
gemini_api_key: "test_key"
chat_id: 123
gemini_model: "gemini-2.0-flash-lite"
digest_time: "09:00"
timezone: "UTC"
article_count: 30
fetch_timeout_secs: 10
tag_decay_rate: 0.02
min_tag_weight: 0.1
tag_boost_on_like: 0.2
db_path: "./test.db"
log_level: "info"
`
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(configPath, []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("HN_BOT_CONFIG", configPath)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.TelegramToken != "test_token" {
		t.Errorf("expected telegram_token 'test_token', got %s", cfg.TelegramToken)
	}
	if cfg.GeminiAPIKey != "test_key" {
		t.Errorf("expected gemini_api_key 'test_key', got %s", cfg.GeminiAPIKey)
	}
	if cfg.ChatID != 123 {
		t.Errorf("expected chat_id 123, got %d", cfg.ChatID)
	}
	if cfg.DigestTime != "09:00" {
		t.Errorf("expected digest_time '09:00', got %s", cfg.DigestTime)
	}
	if cfg.Timezone != "UTC" {
		t.Errorf("expected timezone 'UTC', got %s", cfg.Timezone)
	}
	if cfg.ArticleCount != 30 {
		t.Errorf("expected article_count 30, got %d", cfg.ArticleCount)
	}
	if cfg.FetchTimeoutSecs != 10 {
		t.Errorf("expected fetch_timeout_secs 10, got %d", cfg.FetchTimeoutSecs)
	}
	if cfg.TagDecayRate != 0.02 {
		t.Errorf("expected tag_decay_rate 0.02, got %f", cfg.TagDecayRate)
	}
	if cfg.MinTagWeight != 0.1 {
		t.Errorf("expected min_tag_weight 0.1, got %f", cfg.MinTagWeight)
	}
	if cfg.TagBoostOnLike != 0.2 {
		t.Errorf("expected tag_boost_on_like 0.2, got %f", cfg.TagBoostOnLike)
	}
	if cfg.DBPath != "./test.db" {
		t.Errorf("expected db_path './test.db', got %s", cfg.DBPath)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected log_level 'info', got %s", cfg.LogLevel)
	}
}

func TestLoad_MissingRequired(t *testing.T) {
	content := `
gemini_api_key: "test_key"
`
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(configPath, []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("HN_BOT_CONFIG", configPath)

	_, err = Load()
	if err == nil {
		t.Fatal("expected error for missing telegram_token")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	content := `invalid yaml: [`
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(configPath, []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("HN_BOT_CONFIG", configPath)

	_, err = Load()
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoad_InvalidTimeFormat(t *testing.T) {
	content := `
telegram_token: "test"
gemini_api_key: "key"
digest_time: "25:00"
`
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(configPath, []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("HN_BOT_CONFIG", configPath)

	_, err = Load()
	if err == nil {
		t.Fatal("expected error for invalid time format")
	}
}

func TestLoad_InvalidTimezone(t *testing.T) {
	content := `
telegram_token: "test"
gemini_api_key: "key"
timezone: "Invalid/Timezone"
`
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(configPath, []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("HN_BOT_CONFIG", configPath)

	_, err = Load()
	if err == nil {
		t.Fatal("expected error for invalid timezone")
	}
}

func TestLoad_Defaults(t *testing.T) {
	content := `
telegram_token: "test"
gemini_api_key: "key"
`
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(configPath, []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("HN_BOT_CONFIG", configPath)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.ChatID != 0 {
		t.Errorf("expected default chat_id 0, got %d", cfg.ChatID)
	}
	if cfg.GeminiModel != "gemini-2.0-flash-lite" {
		t.Errorf("expected default gemini_model 'gemini-2.0-flash-lite', got %s", cfg.GeminiModel)
	}
	if cfg.DigestTime != "09:00" {
		t.Errorf("expected default digest_time '09:00', got %s", cfg.DigestTime)
	}
	if cfg.Timezone != "UTC" {
		t.Errorf("expected default timezone 'UTC', got %s", cfg.Timezone)
	}
	if cfg.ArticleCount != 30 {
		t.Errorf("expected default article_count 30, got %d", cfg.ArticleCount)
	}
	if cfg.FetchTimeoutSecs != 10 {
		t.Errorf("expected default fetch_timeout_secs 10, got %d", cfg.FetchTimeoutSecs)
	}
	if cfg.TagDecayRate != 0.02 {
		t.Errorf("expected default tag_decay_rate 0.02, got %f", cfg.TagDecayRate)
	}
	if cfg.MinTagWeight != 0.1 {
		t.Errorf("expected default min_tag_weight 0.1, got %f", cfg.MinTagWeight)
	}
	if cfg.TagBoostOnLike != 0.2 {
		t.Errorf("expected default tag_boost_on_like 0.2, got %f", cfg.TagBoostOnLike)
	}
	if cfg.DBPath != "./hn-bot.db" {
		t.Errorf("expected default db_path './hn-bot.db', got %s", cfg.DBPath)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected default log_level 'info', got %s", cfg.LogLevel)
	}
}

func TestLoad_DBPathOverride(t *testing.T) {
	content := `
telegram_token: "test"
gemini_api_key: "key"
db_path: "./config.db"
`
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(configPath, []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("HN_BOT_CONFIG", configPath)
	t.Setenv("HN_BOT_DB", "./env.db")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.DBPath != "./env.db" {
		t.Errorf("expected db_path './env.db', got %s", cfg.DBPath)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	t.Setenv("HN_BOT_CONFIG", "/nonexistent/config.yaml")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}
