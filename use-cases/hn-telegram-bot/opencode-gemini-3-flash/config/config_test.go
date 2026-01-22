package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	t.Run("ValidConfig", func(t *testing.T) {
		content := `
telegram_token: "test_token"
gemini_api_key: "test_key"
digest_time: "10:30"
timezone: "Europe/Rome"
`
		tmpfile, err := os.CreateTemp("", "config*.yaml")
		require.NoError(t, err)
		defer os.Remove(tmpfile.Name())

		_, err = tmpfile.Write([]byte(content))
		require.NoError(t, err)
		require.NoError(t, tmpfile.Close())

		cfg, err := Load(tmpfile.Name())
		require.NoError(t, err)

		assert.Equal(t, "test_token", cfg.TelegramToken)
		assert.Equal(t, "test_key", cfg.GeminiAPIKey)
		assert.Equal(t, "10:30", cfg.DigestTime)
		assert.Equal(t, "Europe/Rome", cfg.Timezone)
		assert.Equal(t, 30, cfg.ArticleCount) // Default value
	})

	t.Run("MissingRequiredFields", func(t *testing.T) {
		content := `
telegram_token: "test_token"
`
		tmpfile, err := os.CreateTemp("", "config*.yaml")
		require.NoError(t, err)
		defer os.Remove(tmpfile.Name())

		_, err = tmpfile.Write([]byte(content))
		require.NoError(t, err)
		require.NoError(t, tmpfile.Close())

		_, err = Load(tmpfile.Name())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "gemini_api_key is required")
	})

	t.Run("InvalidTimeFormat", func(t *testing.T) {
		content := `
telegram_token: "test_token"
gemini_api_key: "test_key"
digest_time: "25:00"
`
		tmpfile, err := os.CreateTemp("", "config*.yaml")
		require.NoError(t, err)
		defer os.Remove(tmpfile.Name())

		_, err = tmpfile.Write([]byte(content))
		require.NoError(t, err)
		require.NoError(t, tmpfile.Close())

		_, err = Load(tmpfile.Name())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid digest_time")
	})

	t.Run("InvalidTimezone", func(t *testing.T) {
		content := `
telegram_token: "test_token"
gemini_api_key: "test_key"
timezone: "Invalid/Timezone"
`
		tmpfile, err := os.CreateTemp("", "config*.yaml")
		require.NoError(t, err)
		defer os.Remove(tmpfile.Name())

		_, err = tmpfile.Write([]byte(content))
		require.NoError(t, err)
		require.NoError(t, tmpfile.Close())

		_, err = Load(tmpfile.Name())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid timezone")
	})
}
