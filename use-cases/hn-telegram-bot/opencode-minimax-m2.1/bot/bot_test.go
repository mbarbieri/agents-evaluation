package bot

import (
	"encoding/json"
	"html"
	"testing"
)

func TestEscapeHTML(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"<script>alert('xss')</script>", "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;"},
		{"A & B", "A &amp; B"},
		{"1 < 2", "1 &lt; 2"},
		{"2 > 1", "2 &gt; 1"},
		{"Normal text", "Normal text"},
	}

	for _, tc := range tests {
		result := EscapeHTML(tc.input)
		if result != tc.expected {
			t.Errorf("EscapeHTML(%s): expected %s, got %s", tc.input, tc.expected, result)
		}
	}
}

func TestFormatArticleMessage(t *testing.T) {
	msg := FormatArticleMessage(
		"🚀",
		"Test Article Title",
		"This is a test summary of the article.",
		100,
		50,
		"https://example.com",
		12345,
	)

	if !contains(msg, "<b>🚀 Test Article Title</b>") {
		t.Error("Message should contain formatted title")
	}
	if !contains(msg, "<i>This is a test summary of the article.</i>") {
		t.Error("Message should contain formatted summary")
	}
	if !contains(msg, "100") {
		t.Error("Message should contain score")
	}
	if !contains(msg, "50") {
		t.Error("Message should contain comment count")
	}
	if !contains(msg, "https://example.com") {
		t.Error("Message should contain article URL")
	}
	if !contains(msg, "news.ycombinator.com/item?id=12345") {
		t.Error("Message should contain HN URL")
	}
}

func TestFormatStartMessage(t *testing.T) {
	msg := FormatStartMessage()

	if !contains(msg, "/fetch") {
		t.Error("Message should mention /fetch command")
	}
	if !contains(msg, "/settings") {
		t.Error("Message should mention /settings command")
	}
	if !contains(msg, "/stats") {
		t.Error("Message should mention /stats command")
	}
}

func TestFormatSettingsDisplay(t *testing.T) {
	msg := FormatSettingsDisplay("09:30", 25)

	if !contains(msg, "09:30") {
		t.Error("Message should contain digest time")
	}
	if !contains(msg, "25") {
		t.Error("Message should contain article count")
	}
}

func TestFormatSettingsUpdate(t *testing.T) {
	msg := FormatSettingsUpdate("time", "10:00")
	if !contains(msg, "10:00") {
		t.Error("Message should contain new time")
	}

	msg = FormatSettingsUpdate("count", "50")
	if !contains(msg, "50") {
		t.Error("Message should contain new count")
	}
}

func TestFormatStats(t *testing.T) {
	tags := []struct {
		Name   string
		Weight float64
	}{
		{"go", 2.5},
		{"rust", 1.8},
		{"python", 1.2},
	}

	msg := FormatStats(tags, 42)

	if !contains(msg, "42") {
		t.Error("Message should contain like count")
	}
	if !contains(msg, "go") {
		t.Error("Message should contain tag 'go'")
	}
	if !contains(msg, "2.5") {
		t.Error("Message should contain weight 2.5")
	}
}

func TestFormatStatsEmpty(t *testing.T) {
	msg := FormatStats([]struct {
		Name   string
		Weight float64
	}{}, 0)

	if !contains(msg, "thumbs-up") {
		t.Error("Message should encourage thumbs-up reactions")
	}
}

func TestFormatWelcomeMessage(t *testing.T) {
	msg := FormatWelcomeMessage()

	if !contains(msg, "👋") {
		t.Error("Message should contain wave emoji")
	}
}

func TestParseCommand(t *testing.T) {
	tests := []struct {
		input   string
		command string
		args    string
	}{
		{"/start", "start", ""},
		{"/fetch", "fetch", ""},
		{"/settings", "settings", ""},
		{"/stats", "stats", ""},
		{"/settings time 09:00", "settings", "time 09:00"},
		{"/settings count 30", "settings", "count 30"},
		{"/start@bot", "start", ""},
	}

	for _, tc := range tests {
		cmd, args := ParseCommand(tc.input)
		if cmd != tc.command {
			t.Errorf("ParseCommand(%s): expected command '%s', got '%s'", tc.input, tc.command, cmd)
		}
		if args != tc.args {
			t.Errorf("ParseCommand(%s): expected args '%s', got '%s'", tc.input, tc.args, args)
		}
	}
}

func TestParseSettingsArgs(t *testing.T) {
	tests := []struct {
		input       string
		expectedKey string
		expectedVal string
		shouldErr   bool
	}{
		{"time 09:00", "time", "09:00", false},
		{"count 30", "count", "30", false},
		{"time", "", "", true},
		{"09:00", "", "", true},
		{"", "", "", true},
	}

	for _, tc := range tests {
		key, val, err := ParseSettingsArgs(tc.input)
		if tc.shouldErr {
			if err == nil {
				t.Errorf("ParseSettingsArgs(%s): expected error, got nil", tc.input)
			}
		} else {
			if err != nil {
				t.Errorf("ParseSettingsArgs(%s): unexpected error: %v", tc.input, err)
			}
			if key != tc.expectedKey || val != tc.expectedVal {
				t.Errorf("ParseSettingsArgs(%s): expected (%s, %s), got (%s, %s)",
					tc.input, tc.expectedKey, tc.expectedVal, key, val)
			}
		}
	}
}

func TestIsThumbsUpReaction(t *testing.T) {
	reaction := `{"type":{"emoji":"👍"},"emoji":"👍"}`

	var reactionData ReactionData
	if err := json.Unmarshal([]byte(reaction), &reactionData); err != nil {
		t.Fatalf("Failed to unmarshal reaction: %v", err)
	}

	if !IsThumbsUpReaction(&reactionData) {
		t.Error("Should detect thumbs-up reaction")
	}

	reactionData.Type.Emoji = "👎"
	reactionData.Emoji = "👎"
	if IsThumbsUpReaction(&reactionData) {
		t.Error("Should not detect non-thumbs-up reaction")
	}
}

func TestContains(t *testing.T) {
	if !contains("hello world", "world") {
		t.Error("Should find substring")
	}
	if contains("hello", "world") {
		t.Error("Should not find missing substring")
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

func TestUnescapeHTML(t *testing.T) {
	input := "&lt;script&gt;"
	result := html.UnescapeString(input)
	expected := "<script>"

	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}
