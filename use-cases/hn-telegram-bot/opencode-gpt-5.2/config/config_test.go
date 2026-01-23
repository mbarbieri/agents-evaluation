package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFile_AppliesDefaultsAndOverrides(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	p := filepath.Join(d, "config.yaml")
	if err := os.WriteFile(p, []byte("telegram_token: t\n"+"gemini_api_key: g\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadFile(p)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if cfg.DigestTime != "09:00" {
		t.Fatalf("digest time default: got %q", cfg.DigestTime)
	}
	if cfg.ArticleCount != 30 {
		t.Fatalf("article_count default: got %d", cfg.ArticleCount)
	}
}

func TestLoadFromEnv_UsesConfigPathAndDBOverride(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	p := filepath.Join(d, "config.yaml")
	if err := os.WriteFile(p, []byte("telegram_token: t\n"+"gemini_api_key: g\n"+"db_path: ./a.db\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	getenv := func(k string) string {
		switch k {
		case "HN_BOT_CONFIG":
			return p
		case "HN_BOT_DB":
			return "./override.db"
		default:
			return ""
		}
	}

	cfg, err := LoadFromEnv(getenv)
	if err != nil {
		t.Fatalf("LoadFromEnv: %v", err)
	}
	if cfg.DBPath != "./override.db" {
		t.Fatalf("db override: got %q", cfg.DBPath)
	}
}

func TestConfigValidate_RejectsMissingRequiredFields(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.TelegramToken = ""
	cfg.GeminiAPIKey = ""

	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error")
	}
}

func TestParseHHMM(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		ok   bool
		hour int
		min  int
	}{
		{in: "09:00", ok: true, hour: 9, min: 0},
		{in: "23:59", ok: true, hour: 23, min: 59},
		{in: "24:00", ok: false},
		{in: "00:60", ok: false},
		{in: "9:00", ok: false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			h, m, err := ParseHHMM(tt.in)
			if tt.ok {
				if err != nil {
					t.Fatalf("expected nil err, got %v", err)
				}
				if h != tt.hour || m != tt.min {
					t.Fatalf("got %d:%d", h, m)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error")
			}
		})
	}
}
