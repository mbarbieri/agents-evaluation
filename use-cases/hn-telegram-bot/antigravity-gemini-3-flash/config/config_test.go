package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	// Create a temporary config file
	content := `
telegram_token: "test_token"
gemini_api_key: "test_key"
digest_time: "10:30"
timezone: "Europe/Rome"
`
	tmpfile, err := os.CreateTemp("", "config*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	t.Run("LoadFromYAML", func(t *testing.T) {
		cfg, err := Load(tmpfile.Name())
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		if cfg.TelegramToken != "test_token" {
			t.Errorf("expected TelegramToken test_token, got %s", cfg.TelegramToken)
		}
		if cfg.DigestTime != "10:30" {
			t.Errorf("expected DigestTime 10:30, got %s", cfg.DigestTime)
		}
	})

	t.Run("EnvironmentOverride", func(t *testing.T) {
		os.Setenv("HN_BOT_DB", "/tmp/test.db")
		defer os.Unsetenv("HN_BOT_DB")

		cfg, err := Load(tmpfile.Name())
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		if cfg.DBPath != "/tmp/test.db" {
			t.Errorf("expected DBPath /tmp/test.db, got %s", cfg.DBPath)
		}
	})

	t.Run("ValidationMissingTokens", func(t *testing.T) {
		invalidContent := `
digest_time: "10:30"
`
		tmpInvalid, _ := os.CreateTemp("", "invalid*.yaml")
		defer os.Remove(tmpInvalid.Name())
		tmpInvalid.Write([]byte(invalidContent))
		tmpInvalid.Close()

		_, err := Load(tmpInvalid.Name())
		if err == nil {
			t.Error("expected error for missing tokens, got nil")
		}
	})

	t.Run("ValidationInvalidTime", func(t *testing.T) {
		invalidContent := `
telegram_token: "t"
gemini_api_key: "g"
digest_time: "25:00"
`
		tmpInvalid, _ := os.CreateTemp("", "invalid*.yaml")
		defer os.Remove(tmpInvalid.Name())
		tmpInvalid.Write([]byte(invalidContent))
		tmpInvalid.Close()

		_, err := Load(tmpInvalid.Name())
		if err == nil {
			t.Error("expected error for invalid digest_time, got nil")
		}
	})
}
