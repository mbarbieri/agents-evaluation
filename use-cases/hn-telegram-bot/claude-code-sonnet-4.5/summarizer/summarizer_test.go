package summarizer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSummarize_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request path
		if !strings.Contains(r.URL.Path, "gemini-test-model:generateContent") {
			t.Errorf("Expected path to contain model name, got %s", r.URL.Path)
		}

		// Verify API key query param
		if r.URL.Query().Get("key") != "test-api-key" {
			t.Errorf("Expected API key test-api-key, got %s", r.URL.Query().Get("key"))
		}

		// Return mock response
		response := map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]interface{}{
							{"text": `{"summary": "Test summary", "tags": ["golang", "testing"]}`},
						},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	s := New("test-api-key", "gemini-test-model", server.URL)
	result, err := s.Summarize("Test article title", "Test article content")
	if err != nil {
		t.Fatalf("Summarize() error = %v", err)
	}

	if result.Summary != "Test summary" {
		t.Errorf("Summary = %v, want Test summary", result.Summary)
	}

	if len(result.Tags) != 2 {
		t.Fatalf("Tags length = %v, want 2", len(result.Tags))
	}

	if result.Tags[0] != "golang" || result.Tags[1] != "testing" {
		t.Errorf("Tags = %v, want [golang testing]", result.Tags)
	}
}

func TestSummarize_WithMarkdownCodeBlock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]interface{}{
							{"text": "```json\n{\"summary\": \"Test summary\", \"tags\": [\"test\"]}\n```"},
						},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	s := New("test-key", "test-model", server.URL)
	result, err := s.Summarize("Title", "Content")
	if err != nil {
		t.Fatalf("Summarize() error = %v", err)
	}

	if result.Summary != "Test summary" {
		t.Errorf("Summary = %v, want Test summary", result.Summary)
	}
}

func TestSummarize_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	s := New("test-key", "test-model", server.URL)
	_, err := s.Summarize("Title", "Content")
	if err == nil {
		t.Error("Summarize() expected error for HTTP 500, got nil")
	}
}

func TestSummarize_InvalidResponseJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	s := New("test-key", "test-model", server.URL)
	_, err := s.Summarize("Title", "Content")
	if err == nil {
		t.Error("Summarize() expected error for invalid JSON, got nil")
	}
}

func TestSummarize_InvalidContentJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]interface{}{
							{"text": "not valid json content"},
						},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	s := New("test-key", "test-model", server.URL)
	_, err := s.Summarize("Title", "Content")
	if err == nil {
		t.Error("Summarize() expected error for invalid content JSON, got nil")
	}
}

func TestSummarize_EmptyCandidates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"candidates": []map[string]interface{}{},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	s := New("test-key", "test-model", server.URL)
	_, err := s.Summarize("Title", "Content")
	if err == nil {
		t.Error("Summarize() expected error for empty candidates, got nil")
	}
}

func TestNew_DefaultBaseURL(t *testing.T) {
	s := New("key", "model", "")
	if s.baseURL != "https://generativelanguage.googleapis.com" {
		t.Errorf("Default baseURL = %v, want https://generativelanguage.googleapis.com", s.baseURL)
	}
}

func TestStripMarkdownCodeBlock(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "WithJSONBlock",
			input: "```json\n{\"key\": \"value\"}\n```",
			want:  "{\"key\": \"value\"}",
		},
		{
			name:  "WithPlainBlock",
			input: "```\n{\"key\": \"value\"}\n```",
			want:  "{\"key\": \"value\"}",
		},
		{
			name:  "NoBlock",
			input: "{\"key\": \"value\"}",
			want:  "{\"key\": \"value\"}",
		},
		{
			name:  "WithExtraWhitespace",
			input: "```json\n  {\"key\": \"value\"}  \n```",
			want:  "{\"key\": \"value\"}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripMarkdownCodeBlock(tt.input)
			if got != tt.want {
				t.Errorf("stripMarkdownCodeBlock() = %v, want %v", got, tt.want)
			}
		})
	}
}
