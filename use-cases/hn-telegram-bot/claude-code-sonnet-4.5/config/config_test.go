package config

import (
	"os"
	"testing"
)

func TestLoadConfig_Success(t *testing.T) {
	// Create temp config file
	content := `telegram_token: "test-token"
gemini_api_key: "test-key"
digest_time: "10:30"
article_count: 25
`
	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	cfg, err := Load(tmpfile.Name())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.TelegramToken != "test-token" {
		t.Errorf("TelegramToken = %v, want test-token", cfg.TelegramToken)
	}
	if cfg.GeminiAPIKey != "test-key" {
		t.Errorf("GeminiAPIKey = %v, want test-key", cfg.GeminiAPIKey)
	}
	if cfg.DigestTime != "10:30" {
		t.Errorf("DigestTime = %v, want 10:30", cfg.DigestTime)
	}
	if cfg.ArticleCount != 25 {
		t.Errorf("ArticleCount = %v, want 25", cfg.ArticleCount)
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	content := `telegram_token: "test-token"
gemini_api_key: "test-key"
`
	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	cfg, err := Load(tmpfile.Name())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	tests := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"ChatID", cfg.ChatID, int64(0)},
		{"GeminiModel", cfg.GeminiModel, "gemini-2.0-flash-lite"},
		{"DigestTime", cfg.DigestTime, "09:00"},
		{"Timezone", cfg.Timezone, "UTC"},
		{"ArticleCount", cfg.ArticleCount, 30},
		{"FetchTimeoutSecs", cfg.FetchTimeoutSecs, 10},
		{"TagDecayRate", cfg.TagDecayRate, 0.02},
		{"MinTagWeight", cfg.MinTagWeight, 0.1},
		{"TagBoostOnLike", cfg.TagBoostOnLike, 0.2},
		{"DBPath", cfg.DBPath, "./hn-bot.db"},
		{"LogLevel", cfg.LogLevel, "info"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestLoadConfig_MissingTelegramToken(t *testing.T) {
	content := `gemini_api_key: "test-key"`
	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	_, err = Load(tmpfile.Name())
	if err == nil {
		t.Error("Load() expected error for missing telegram_token, got nil")
	}
}

func TestLoadConfig_MissingGeminiAPIKey(t *testing.T) {
	content := `telegram_token: "test-token"`
	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	_, err = Load(tmpfile.Name())
	if err == nil {
		t.Error("Load() expected error for missing gemini_api_key, got nil")
	}
}

func TestLoadConfig_InvalidTimeFormat(t *testing.T) {
	tests := []struct {
		name string
		time string
	}{
		{"InvalidFormat", "25:00"},
		{"InvalidMinutes", "10:60"},
		{"NotHHMM", "9:30"},
		{"Letters", "ab:cd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := `telegram_token: "test-token"
gemini_api_key: "test-key"
digest_time: "` + tt.time + `"`
			tmpfile, err := os.CreateTemp("", "config-*.yaml")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tmpfile.Name())

			if _, err := tmpfile.Write([]byte(content)); err != nil {
				t.Fatal(err)
			}
			tmpfile.Close()

			_, err = Load(tmpfile.Name())
			if err == nil {
				t.Errorf("Load() expected error for time %s, got nil", tt.time)
			}
		})
	}
}

func TestLoadConfig_ValidTimezones(t *testing.T) {
	tests := []string{"UTC", "America/New_York", "Europe/London", "Asia/Tokyo"}

	for _, tz := range tests {
		t.Run(tz, func(t *testing.T) {
			content := `telegram_token: "test-token"
gemini_api_key: "test-key"
timezone: "` + tz + `"`
			tmpfile, err := os.CreateTemp("", "config-*.yaml")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tmpfile.Name())

			if _, err := tmpfile.Write([]byte(content)); err != nil {
				t.Fatal(err)
			}
			tmpfile.Close()

			cfg, err := Load(tmpfile.Name())
			if err != nil {
				t.Errorf("Load() error = %v for timezone %s", err, tz)
			}
			if cfg.Timezone != tz {
				t.Errorf("Timezone = %v, want %v", cfg.Timezone, tz)
			}
		})
	}
}

func TestLoadConfig_InvalidTimezone(t *testing.T) {
	content := `telegram_token: "test-token"
gemini_api_key: "test-key"
timezone: "Invalid/Timezone"`
	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	_, err = Load(tmpfile.Name())
	if err == nil {
		t.Error("Load() expected error for invalid timezone, got nil")
	}
}

func TestLoadConfig_EnvVarOverride(t *testing.T) {
	// Test HN_BOT_DB override
	content := `telegram_token: "test-token"
gemini_api_key: "test-key"
db_path: "./original.db"`
	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	os.Setenv("HN_BOT_DB", "/override/path.db")
	defer os.Unsetenv("HN_BOT_DB")

	cfg, err := Load(tmpfile.Name())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.DBPath != "/override/path.db" {
		t.Errorf("DBPath = %v, want /override/path.db", cfg.DBPath)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Error("Load() expected error for nonexistent file, got nil")
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	content := `invalid: yaml: content:`
	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	_, err = Load(tmpfile.Name())
	if err == nil {
		t.Error("Load() expected error for invalid YAML, got nil")
	}
}
