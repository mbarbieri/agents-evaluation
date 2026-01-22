package summarizer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSummarize(t *testing.T) {
	response := map[string]any{
		"candidates": []map[string]any{
			{
				"content": map[string]any{
					"parts": []map[string]string{
						{"text": `{"summary": "This is a summary", "tags": ["rust", "programming"]}`},
					},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	summarizer := NewSummarizer("test-key", "gemini-2.0-flash-lite", server.URL)
	summary, tags, err := summarizer.Summarize("Article content")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if summary != "This is a summary" {
		t.Errorf("summary = %v, want This is a summary", summary)
	}

	if len(tags) != 2 {
		t.Errorf("got %d tags, want 2", len(tags))
	}

	if tags[0] != "rust" {
		t.Errorf("first tag = %v, want rust", tags[0])
	}
}

func TestSummarizeWithMarkdownCodeBlock(t *testing.T) {
	response := map[string]any{
		"candidates": []map[string]any{
			{
				"content": map[string]any{
					"parts": []map[string]string{
						{"text": "```\n{\"summary\": \"Summary\", \"tags\": [\"go\"]}\n```"},
					},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	summarizer := NewSummarizer("test-key", "gemini-2.0-flash-lite", server.URL)
	summary, tags, err := summarizer.Summarize("Content")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if summary != "Summary" {
		t.Errorf("summary = %v, want Summary", summary)
	}

	if len(tags) != 1 || tags[0] != "go" {
		t.Errorf("tags = %v, want [go]", tags)
	}
}

func TestSummarizeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	summarizer := NewSummarizer("test-key", "gemini-2.0-flash-lite", server.URL)
	_, _, err := summarizer.Summarize("Content")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestSummarizeInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	summarizer := NewSummarizer("test-key", "gemini-2.0-flash-lite", server.URL)
	_, _, err := summarizer.Summarize("Content")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestSummarizeMalformedResponse(t *testing.T) {
	response := map[string]any{
		"candidates": []map[string]any{},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	summarizer := NewSummarizer("test-key", "gemini-2.0-flash-lite", server.URL)
	_, _, err := summarizer.Summarize("Content")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestSummarizeInvalidJSONInResponse(t *testing.T) {
	response := map[string]any{
		"candidates": []map[string]any{
			{
				"content": map[string]any{
					"parts": []map[string]string{
						{"text": "not json"},
					},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	summarizer := NewSummarizer("test-key", "gemini-2.0-flash-lite", server.URL)
	_, _, err := summarizer.Summarize("Content")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestStripMarkdownCodeBlocks(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no markdown",
			input:    `{"summary": "test"}`,
			expected: `{"summary": "test"}`,
		},
		{
			name:     "with markdown code block",
			input:    "```json\n{\"summary\": \"test\"}\n```",
			expected: `{"summary": "test"}`,
		},
		{
			name:     "with markdown code block no language",
			input:    "```\n{\"summary\": \"test\"}\n```",
			expected: `{"summary": "test"}`,
		},
		{
			name:     "multiple markdown blocks",
			input:    "```\ntext\n```\n{\"summary\": \"test\"}",
			expected: "text\n{\"summary\": \"test\"}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripMarkdownCodeBlocks(tt.input)
			if result != tt.expected {
				t.Errorf("got %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestParseSummaryResponse(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantErr     bool
		wantSummary string
		wantTags    []string
	}{
		{
			name:        "valid response",
			input:       `{"summary": "Test summary", "tags": ["rust", "go"]}`,
			wantErr:     false,
			wantSummary: "Test summary",
			wantTags:    []string{"rust", "go"},
		},
		{
			name:    "invalid json",
			input:   "not json",
			wantErr: true,
		},
		{
			name:    "missing fields",
			input:   `{"summary": "test"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary, tags, err := parseSummaryResponse(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if summary != tt.wantSummary {
				t.Errorf("summary = %v, want %v", summary, tt.wantSummary)
			}

			if !equalStringSlices(tags, tt.wantTags) {
				t.Errorf("tags = %v, want %v", tags, tt.wantTags)
			}
		})
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestBuildPrompt(t *testing.T) {
	prompt := buildPrompt("Article content about Rust programming")

	if !strings.Contains(prompt, "Article content about Rust programming") {
		t.Error("prompt should contain the article content")
	}

	if !strings.Contains(prompt, "summary") {
		t.Error("prompt should ask for summary")
	}

	if !strings.Contains(prompt, "tags") {
		t.Error("prompt should ask for tags")
	}
}
