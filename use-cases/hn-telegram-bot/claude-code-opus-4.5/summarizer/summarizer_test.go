package summarizer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSummarize(t *testing.T) {
	// Mock Gemini API response
	geminiResp := map[string]interface{}{
		"candidates": []map[string]interface{}{
			{
				"content": map[string]interface{}{
					"parts": []map[string]interface{}{
						{
							"text": `{"summary": "Test article about Go programming", "tags": ["go", "programming", "testing"]}`,
						},
					},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		json.NewEncoder(w).Encode(geminiResp)
	}))
	defer server.Close()

	s := NewSummarizer("test-api-key",
		WithModel("gemini-pro"),
		WithBaseURL(server.URL),
	)
	ctx := context.Background()

	result, err := s.Summarize(ctx, "Test Article", "This is content about Go programming and testing.")
	if err != nil {
		t.Fatalf("Summarize failed: %v", err)
	}

	if result.Summary != "Test article about Go programming" {
		t.Errorf("Summary = %q, want 'Test article about Go programming'", result.Summary)
	}

	if len(result.Tags) != 3 {
		t.Errorf("got %d tags, want 3", len(result.Tags))
	}

	expectedTags := map[string]bool{"go": true, "programming": true, "testing": true}
	for _, tag := range result.Tags {
		if !expectedTags[tag] {
			t.Errorf("unexpected tag: %s", tag)
		}
	}
}

func TestSummarizeWithMarkdownCodeBlock(t *testing.T) {
	// Response wrapped in markdown code blocks
	geminiResp := map[string]interface{}{
		"candidates": []map[string]interface{}{
			{
				"content": map[string]interface{}{
					"parts": []map[string]interface{}{
						{
							"text": "```json\n{\"summary\": \"Summary here\", \"tags\": [\"tag1\"]}\n```",
						},
					},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(geminiResp)
	}))
	defer server.Close()

	s := NewSummarizer("test-key", WithBaseURL(server.URL))
	ctx := context.Background()

	result, err := s.Summarize(ctx, "Title", "Content")
	if err != nil {
		t.Fatalf("Summarize failed: %v", err)
	}

	if result.Summary != "Summary here" {
		t.Errorf("Summary = %q, want 'Summary here'", result.Summary)
	}
}

func TestSummarizeServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	s := NewSummarizer("test-key", WithBaseURL(server.URL))
	ctx := context.Background()

	_, err := s.Summarize(ctx, "Title", "Content")
	if err == nil {
		t.Fatal("expected error for server error")
	}
}

func TestSummarizeEmptyCandidates(t *testing.T) {
	geminiResp := map[string]interface{}{
		"candidates": []map[string]interface{}{},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(geminiResp)
	}))
	defer server.Close()

	s := NewSummarizer("test-key", WithBaseURL(server.URL))
	ctx := context.Background()

	_, err := s.Summarize(ctx, "Title", "Content")
	if err == nil {
		t.Fatal("expected error for empty candidates")
	}
}

func TestSummarizeInvalidJSON(t *testing.T) {
	geminiResp := map[string]interface{}{
		"candidates": []map[string]interface{}{
			{
				"content": map[string]interface{}{
					"parts": []map[string]interface{}{
						{
							"text": "not valid json",
						},
					},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(geminiResp)
	}))
	defer server.Close()

	s := NewSummarizer("test-key", WithBaseURL(server.URL))
	ctx := context.Background()

	_, err := s.Summarize(ctx, "Title", "Content")
	if err == nil {
		t.Fatal("expected error for invalid JSON in response")
	}
}

func TestSummarizeContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Write([]byte("{}"))
	}))
	defer server.Close()

	s := NewSummarizer("test-key", WithBaseURL(server.URL))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := s.Summarize(ctx, "Title", "Content")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestStripMarkdownCodeBlock(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`{"key": "value"}`, `{"key": "value"}`},
		{"```json\n{\"key\": \"value\"}\n```", `{"key": "value"}`},
		{"```\n{\"key\": \"value\"}\n```", `{"key": "value"}`},
		{"  ```json\n{\"key\": \"value\"}\n```  ", `{"key": "value"}`},
	}

	for _, tt := range tests {
		result := stripMarkdownCodeBlock(tt.input)
		if result != tt.expected {
			t.Errorf("stripMarkdownCodeBlock(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestDefaultSummarizer(t *testing.T) {
	s := NewSummarizer("test-key")
	if s.model != "gemini-2.0-flash-lite" {
		t.Errorf("default model = %q, want 'gemini-2.0-flash-lite'", s.model)
	}
}
