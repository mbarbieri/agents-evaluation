package summarizer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func geminiResponseJSON(text string) string {
	resp := geminiResponse{
		Candidates: []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		}{
			{Content: struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			}{
				Parts: []struct {
					Text string `json:"text"`
				}{{Text: text}},
			}},
		},
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

func TestSummarize_Success(t *testing.T) {
	resultJSON := `{"summary":"A great article about Go.","tags":["go","programming","testing"]}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(geminiResponseJSON(resultJSON)))
	}))
	defer srv.Close()

	s := newSummarizerWithURL("test-key", "test-model", srv.Client(), srv.URL)

	result, err := s.Summarize(context.Background(), "Test Article", "Article content here")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Summary != "A great article about Go." {
		t.Errorf("unexpected summary: %s", result.Summary)
	}
	if len(result.Tags) != 3 {
		t.Errorf("expected 3 tags, got %d", len(result.Tags))
	}
	if result.Tags[0] != "go" {
		t.Errorf("expected first tag 'go', got %s", result.Tags[0])
	}
}

func TestSummarize_MarkdownCodeBlock(t *testing.T) {
	// Gemini may wrap JSON in markdown code blocks
	resultJSON := "```json\n{\"summary\":\"Wrapped result.\",\"tags\":[\"ai\"]}\n```"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(geminiResponseJSON(resultJSON)))
	}))
	defer srv.Close()

	s := newSummarizerWithURL("test-key", "test-model", srv.Client(), srv.URL)

	result, err := s.Summarize(context.Background(), "Test", "Content")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Summary != "Wrapped result." {
		t.Errorf("unexpected summary: %s", result.Summary)
	}
	if len(result.Tags) != 1 || result.Tags[0] != "ai" {
		t.Errorf("unexpected tags: %v", result.Tags)
	}
}

func TestSummarize_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal"}`))
	}))
	defer srv.Close()

	s := newSummarizerWithURL("test-key", "test-model", srv.Client(), srv.URL)

	_, err := s.Summarize(context.Background(), "Test", "Content")
	if err == nil {
		t.Fatal("expected error for API failure")
	}
}

func TestSummarize_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"candidates":[]}`))
	}))
	defer srv.Close()

	s := newSummarizerWithURL("test-key", "test-model", srv.Client(), srv.URL)

	_, err := s.Summarize(context.Background(), "Test", "Content")
	if err == nil {
		t.Fatal("expected error for empty response")
	}
}

func TestSummarize_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(geminiResponseJSON("not valid json at all")))
	}))
	defer srv.Close()

	s := newSummarizerWithURL("test-key", "test-model", srv.Client(), srv.URL)

	_, err := s.Summarize(context.Background(), "Test", "Content")
	if err == nil {
		t.Fatal("expected error for invalid JSON in response")
	}
}

func TestSummarize_ContextCanceled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := newSummarizerWithURL("test-key", "test-model", srv.Client(), srv.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := s.Summarize(ctx, "Test", "Content")
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}

func TestStripMarkdownCodeBlock(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`{"key":"val"}`, `{"key":"val"}`},
		{"```json\n{\"key\":\"val\"}\n```", `{"key":"val"}`},
		{"```\n{\"key\":\"val\"}\n```", `{"key":"val"}`},
		{"  ```json\n{\"key\":\"val\"}\n```  ", `{"key":"val"}`},
		{"no code block", "no code block"},
	}

	for _, tt := range tests {
		got := stripMarkdownCodeBlock(tt.input)
		if got != tt.expected {
			t.Errorf("stripMarkdownCodeBlock(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
