package digest

import "testing"

func TestFormatArticleMessageEscapesHTML(t *testing.T) {
	t.Parallel()
	article := Article{
		ID:       1,
		Title:    "<Title>",
		Summary:  "Summary & more",
		Score:    10,
		Comments: 5,
		URL:      "https://example.com",
	}
	msg := FormatArticleMessage(article)
	if msg == "" {
		t.Fatalf("expected message")
	}
	if msg == "<Title>" {
		t.Fatalf("expected escaping")
	}
}
