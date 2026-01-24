package summarizer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSummarize(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1beta/models/gemini-2.0-flash-lite:generateContent", func(w http.ResponseWriter, r *http.Request) {
		// Mock Gemini response with markdown JSON
		resp := map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]interface{}{
							{
								"text": "```json\n{\"summary\": \"A test summary.\", \"tags\": [\"test\", \"go\"]}\n```",
							},
						},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	s := NewSummarizer("test-key", "gemini-2.0-flash-lite", server.URL)

	t.Run("SummarizeSuccess", func(t *testing.T) {
		summary, tags, err := s.Summarize("Title", "Content")
		if err != nil {
			t.Fatalf("failed to summarize: %v", err)
		}
		if summary != "A test summary." {
			t.Errorf("expected summary 'A test summary.', got '%s'", summary)
		}
		if len(tags) != 2 || tags[0] != "test" {
			t.Errorf("tags mismatch: %v", tags)
		}
	})
}

func TestStripMarkdown(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"```json\n{\"a\":1}\n```", "{\"a\":1}"},
		{"{\"a\":1}", "{\"a\":1}"},
		{"   ```json {\"a\":1} ```  ", "{\"a\":1}"},
	}

	for _, c := range cases {
		got := stripMarkdown(c.input)
		if got != c.expected {
			t.Errorf("input: %s, expected: %s, got: %s", c.input, c.expected, got)
		}
	}
}
