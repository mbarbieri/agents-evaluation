package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		wantErr     bool
		errMsg      string
		validate    func(*testing.T, *Config)
	}{
		{
			name: "valid config with all fields",
			yamlContent: `telegram_token: "test-token"
gemini_api_key: "test-key"
chat_id: 12345
gemini_model: "gemini-2.0-flash-lite"
digest_time: "09:00"
timezone: "America/New_York"
article_count: 20
fetch_timeout_secs: 15
tag_decay_rate: 0.03
min_tag_weight: 0.2
tag_boost_on_like: 0.3
db_path: "/custom/db.db"
log_level: "debug"
`,
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.TelegramToken != "test-token" {
					t.Errorf("TelegramToken = %v, want test-token", cfg.TelegramToken)
				}
				if cfg.GeminiAPIKey != "test-key" {
					t.Errorf("GeminiAPIKey = %v, want test-key", cfg.GeminiAPIKey)
				}
				if cfg.ChatID != 12345 {
					t.Errorf("ChatID = %v, want 12345", cfg.ChatID)
				}
				if cfg.GeminiModel != "gemini-2.0-flash-lite" {
					t.Errorf("GeminiModel = %v, want gemini-2.0-flash-lite", cfg.GeminiModel)
				}
				if cfg.DigestTime != "09:00" {
					t.Errorf("DigestTime = %v, want 09:00", cfg.DigestTime)
				}
				if cfg.Timezone != "America/New_York" {
					t.Errorf("Timezone = %v, want America/New_York", cfg.Timezone)
				}
				if cfg.ArticleCount != 20 {
					t.Errorf("ArticleCount = %v, want 20", cfg.ArticleCount)
				}
				if cfg.FetchTimeoutSecs != 15 {
					t.Errorf("FetchTimeoutSecs = %v, want 15", cfg.FetchTimeoutSecs)
				}
				if cfg.TagDecayRate != 0.03 {
					t.Errorf("TagDecayRate = %v, want 0.03", cfg.TagDecayRate)
				}
				if cfg.MinTagWeight != 0.2 {
					t.Errorf("MinTagWeight = %v, want 0.2", cfg.MinTagWeight)
				}
				if cfg.TagBoostOnLike != 0.3 {
					t.Errorf("TagBoostOnLike = %v, want 0.3", cfg.TagBoostOnLike)
				}
				if cfg.DBPath != "/custom/db.db" {
					t.Errorf("DBPath = %v, want /custom/db.db", cfg.DBPath)
				}
				if cfg.LogLevel != "debug" {
					t.Errorf("LogLevel = %v, want debug", cfg.LogLevel)
				}
			},
		},
		{
			name: "valid config with only required fields",
			yamlContent: `telegram_token: "test-token"
gemini_api_key: "test-key"
`,
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.ChatID != 0 {
					t.Errorf("ChatID = %v, want 0", cfg.ChatID)
				}
				if cfg.GeminiModel != "gemini-2.0-flash-lite" {
					t.Errorf("GeminiModel = %v, want gemini-2.0-flash-lite", cfg.GeminiModel)
				}
				if cfg.DigestTime != "09:00" {
					t.Errorf("DigestTime = %v, want 09:00", cfg.DigestTime)
				}
				if cfg.Timezone != "UTC" {
					t.Errorf("Timezone = %v, want UTC", cfg.Timezone)
				}
				if cfg.ArticleCount != 30 {
					t.Errorf("ArticleCount = %v, want 30", cfg.ArticleCount)
				}
				if cfg.FetchTimeoutSecs != 10 {
					t.Errorf("FetchTimeoutSecs = %v, want 10", cfg.FetchTimeoutSecs)
				}
				if cfg.TagDecayRate != 0.02 {
					t.Errorf("TagDecayRate = %v, want 0.02", cfg.TagDecayRate)
				}
				if cfg.MinTagWeight != 0.1 {
					t.Errorf("MinTagWeight = %v, want 0.1", cfg.MinTagWeight)
				}
				if cfg.TagBoostOnLike != 0.2 {
					t.Errorf("TagBoostOnLike = %v, want 0.2", cfg.TagBoostOnLike)
				}
				if cfg.DBPath != "./hn-bot.db" {
					t.Errorf("DBPath = %v, want ./hn-bot.db", cfg.DBPath)
				}
				if cfg.LogLevel != "info" {
					t.Errorf("LogLevel = %v, want info", cfg.LogLevel)
				}
			},
		},
		{
			name:        "missing telegram_token",
			yamlContent: `gemini_api_key: "test-key"`,
			wantErr:     true,
			errMsg:      "telegram_token is required",
		},
		{
			name:        "missing gemini_api_key",
			yamlContent: `telegram_token: "test-token"`,
			wantErr:     true,
			errMsg:      "gemini_api_key is required",
		},
		{
			name: "invalid time format - invalid format",
			yamlContent: `telegram_token: "test-token"
gemini_api_key: "test-key"
digest_time: "invalid"
`,
			wantErr: true,
			errMsg:  "digest_time must be in HH:MM format",
		},
		{
			name: "invalid time format - invalid hours",
			yamlContent: `telegram_token: "test-token"
gemini_api_key: "test-key"
digest_time: "25:00"
`,
			wantErr: true,
			errMsg:  "digest_time must be in HH:MM format",
		},
		{
			name: "invalid time format - invalid minutes",
			yamlContent: `telegram_token: "test-token"
gemini_api_key: "test-key"
digest_time: "09:60"
`,
			wantErr: true,
			errMsg:  "digest_time must be in HH:MM format",
		},
		{
			name: "invalid timezone",
			yamlContent: `telegram_token: "test-token"
gemini_api_key: "test-key"
timezone: "Invalid/Timezone"
`,
			wantErr: true,
			errMsg:  "invalid timezone",
		},
		{
			name: "valid timezone",
			yamlContent: `telegram_token: "test-token"
gemini_api_key: "test-key"
timezone: "Europe/London"
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpfile, err := os.CreateTemp("", "config-*.yaml")
			if err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}
			defer os.Remove(tmpfile.Name())

			if _, err := tmpfile.WriteString(tt.yamlContent); err != nil {
				t.Fatalf("failed to write to temp file: %v", err)
			}
			tmpfile.Close()

			cfg, err := Load(tmpfile.Name())

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
					return
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("error = %v, want error containing %q", err, tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.validate != nil {
				tt.validate(t, cfg)
			}
		})
	}
}

func TestLoadEnvOverrides(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		envVars     map[string]string
		wantDBPath  string
		wantConfig  string
	}{
		{
			name: "db path overridden by env var",
			yamlContent: `telegram_token: "test-token"
gemini_api_key: "test-key"
db_path: "./config.db"
`,
			envVars: map[string]string{
				"HN_BOT_DB": "./env.db",
			},
			wantDBPath: "./env.db",
		},
		{
			name: "db path from config when env not set",
			yamlContent: `telegram_token: "test-token"
gemini_api_key: "test-key"
db_path: "./config.db"
`,
			envVars:    map[string]string{},
			wantDBPath: "./config.db",
		},
		{
			name: "default db path when neither set",
			yamlContent: `telegram_token: "test-token"
gemini_api_key: "test-key"
`,
			envVars:    map[string]string{},
			wantDBPath: "./hn-bot.db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			tmpfile, err := os.CreateTemp("", "config-*.yaml")
			if err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}
			defer os.Remove(tmpfile.Name())

			if _, err := tmpfile.WriteString(tt.yamlContent); err != nil {
				t.Fatalf("failed to write to temp file: %v", err)
			}
			tmpfile.Close()

			cfg, err := Load(tmpfile.Name())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if cfg.DBPath != tt.wantDBPath {
				t.Errorf("DBPath = %v, want %v", cfg.DBPath, tt.wantDBPath)
			}
		})
	}
}

func TestLoadDefaultConfigPath(t *testing.T) {
	t.Setenv("HN_BOT_CONFIG", "")

	_, err := Load("./config.yaml")
	if err == nil {
		t.Error("expected error for non-existent file, got nil")
	}
}

func TestLoadCustomConfigPath(t *testing.T) {
	content := `telegram_token: "test-token"
gemini_api_key: "test-key"
`

	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.WriteString(content); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpfile.Close()

	t.Setenv("HN_BOT_CONFIG", tmpfile.Name())

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.TelegramToken != "test-token" {
		t.Errorf("TelegramToken = %v, want test-token", cfg.TelegramToken)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
