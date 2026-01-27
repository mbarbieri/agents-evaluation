package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name          string
		configContent string
		envVars       map[string]string
		wantErr       bool
		errContains   string
		validate      func(t *testing.T, cfg *Config)
	}{
		{
			name: "valid config with all required fields",
			configContent: `
telegram_token: "test-token"
gemini_api_key: "test-key"
`,
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.TelegramToken != "test-token" {
					t.Errorf("TelegramToken = %v, want test-token", cfg.TelegramToken)
				}
				if cfg.GeminiAPIKey != "test-key" {
					t.Errorf("GeminiAPIKey = %v, want test-key", cfg.GeminiAPIKey)
				}
				// Check defaults
				if cfg.GeminiModel != "gemini-2.0-flash-lite" {
					t.Errorf("GeminiModel default = %v, want gemini-2.0-flash-lite", cfg.GeminiModel)
				}
				if cfg.DigestTime != "09:00" {
					t.Errorf("DigestTime default = %v, want 09:00", cfg.DigestTime)
				}
				if cfg.Timezone != "UTC" {
					t.Errorf("Timezone default = %v, want UTC", cfg.Timezone)
				}
				if cfg.ArticleCount != 30 {
					t.Errorf("ArticleCount default = %v, want 30", cfg.ArticleCount)
				}
				if cfg.FetchTimeoutSecs != 10 {
					t.Errorf("FetchTimeoutSecs default = %v, want 10", cfg.FetchTimeoutSecs)
				}
				if cfg.TagDecayRate != 0.02 {
					t.Errorf("TagDecayRate default = %v, want 0.02", cfg.TagDecayRate)
				}
				if cfg.MinTagWeight != 0.1 {
					t.Errorf("MinTagWeight default = %v, want 0.1", cfg.MinTagWeight)
				}
				if cfg.TagBoostOnLike != 0.2 {
					t.Errorf("TagBoostOnLike default = %v, want 0.2", cfg.TagBoostOnLike)
				}
				if cfg.DBPath != "./hn-bot.db" {
					t.Errorf("DBPath default = %v, want ./hn-bot.db", cfg.DBPath)
				}
				if cfg.LogLevel != "info" {
					t.Errorf("LogLevel default = %v, want info", cfg.LogLevel)
				}
			},
		},
		{
			name: "missing telegram_token",
			configContent: `
gemini_api_key: "test-key"
`,
			wantErr:     true,
			errContains: "telegram_token",
		},
		{
			name: "missing gemini_api_key",
			configContent: `
telegram_token: "test-token"
`,
			wantErr:     true,
			errContains: "gemini_api_key",
		},
		{
			name: "custom values override defaults",
			configContent: `
telegram_token: "token"
gemini_api_key: "key"
gemini_model: "custom-model"
digest_time: "14:30"
timezone: "America/New_York"
article_count: 50
fetch_timeout_secs: 20
tag_decay_rate: 0.05
min_tag_weight: 0.2
tag_boost_on_like: 0.5
db_path: "/custom/path.db"
log_level: "debug"
chat_id: 123456
`,
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.GeminiModel != "custom-model" {
					t.Errorf("GeminiModel = %v, want custom-model", cfg.GeminiModel)
				}
				if cfg.DigestTime != "14:30" {
					t.Errorf("DigestTime = %v, want 14:30", cfg.DigestTime)
				}
				if cfg.Timezone != "America/New_York" {
					t.Errorf("Timezone = %v, want America/New_York", cfg.Timezone)
				}
				if cfg.ArticleCount != 50 {
					t.Errorf("ArticleCount = %v, want 50", cfg.ArticleCount)
				}
				if cfg.FetchTimeoutSecs != 20 {
					t.Errorf("FetchTimeoutSecs = %v, want 20", cfg.FetchTimeoutSecs)
				}
				if cfg.TagDecayRate != 0.05 {
					t.Errorf("TagDecayRate = %v, want 0.05", cfg.TagDecayRate)
				}
				if cfg.MinTagWeight != 0.2 {
					t.Errorf("MinTagWeight = %v, want 0.2", cfg.MinTagWeight)
				}
				if cfg.TagBoostOnLike != 0.5 {
					t.Errorf("TagBoostOnLike = %v, want 0.5", cfg.TagBoostOnLike)
				}
				if cfg.DBPath != "/custom/path.db" {
					t.Errorf("DBPath = %v, want /custom/path.db", cfg.DBPath)
				}
				if cfg.LogLevel != "debug" {
					t.Errorf("LogLevel = %v, want debug", cfg.LogLevel)
				}
				if cfg.ChatID != 123456 {
					t.Errorf("ChatID = %v, want 123456", cfg.ChatID)
				}
			},
		},
		{
			name: "invalid digest time format",
			configContent: `
telegram_token: "token"
gemini_api_key: "key"
digest_time: "25:00"
`,
			wantErr:     true,
			errContains: "digest_time",
		},
		{
			name: "invalid digest time minutes",
			configContent: `
telegram_token: "token"
gemini_api_key: "key"
digest_time: "12:60"
`,
			wantErr:     true,
			errContains: "digest_time",
		},
		{
			name: "invalid timezone",
			configContent: `
telegram_token: "token"
gemini_api_key: "key"
timezone: "Invalid/Timezone"
`,
			wantErr:     true,
			errContains: "timezone",
		},
		{
			name: "article count too low",
			configContent: `
telegram_token: "token"
gemini_api_key: "key"
article_count: 0
`,
			wantErr:     true,
			errContains: "article_count",
		},
		{
			name: "article count too high",
			configContent: `
telegram_token: "token"
gemini_api_key: "key"
article_count: 101
`,
			wantErr:     true,
			errContains: "article_count",
		},
		{
			name: "environment variable overrides db_path",
			configContent: `
telegram_token: "token"
gemini_api_key: "key"
db_path: "./from-config.db"
`,
			envVars: map[string]string{
				"HN_BOT_DB": "./from-env.db",
			},
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.DBPath != "./from-env.db" {
					t.Errorf("DBPath = %v, want ./from-env.db", cfg.DBPath)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp config file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			if err := os.WriteFile(configPath, []byte(tt.configContent), 0644); err != nil {
				t.Fatalf("Failed to create temp config file: %v", err)
			}

			// Set environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
				defer os.Unsetenv(key)
			}

			// Set HN_BOT_CONFIG to point to our temp file
			os.Setenv("HN_BOT_CONFIG", configPath)
			defer os.Unsetenv("HN_BOT_CONFIG")

			cfg, err := Load()

			if tt.wantErr {
				if err == nil {
					t.Errorf("Load() error = nil, wantErr = true")
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("Load() error = %v, should contain %v", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("Load() unexpected error = %v", err)
				return
			}

			if tt.validate != nil {
				tt.validate(t, cfg)
			}
		})
	}
}

func TestDefaultConfigPath(t *testing.T) {
	// Clear environment variable
	os.Unsetenv("HN_BOT_CONFIG")

	// Create temp directory and config file
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	configContent := `
telegram_token: "test-token"
gemini_api_key: "test-key"
`
	if err := os.WriteFile("config.yaml", []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config.yaml: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Errorf("Load() with default path error = %v", err)
		return
	}

	if cfg.TelegramToken != "test-token" {
		t.Errorf("TelegramToken = %v, want test-token", cfg.TelegramToken)
	}
}

func TestMissingConfigFile(t *testing.T) {
	os.Unsetenv("HN_BOT_CONFIG")

	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	_, err := Load()
	if err == nil {
		t.Error("Load() with missing config should error")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
