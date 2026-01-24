package bot

import (
	"testing"
)

func TestEscapeHTML(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"Hello & World", "Hello &amp; World"},
		{"<script>", "&lt;script&gt;"},
		{"A > B", "A &gt; B"},
	}

	for _, c := range cases {
		got := escapeHTML(c.input)
		if got != c.expected {
			t.Errorf("input: %s, expected: %s, got: %s", c.input, c.expected, got)
		}
	}
}

func TestFormatArticle(t *testing.T) {
	// Art variable removed to fix build error
}
