package summarizer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSummarizer_Summarize_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "generateContent") {
			t.Errorf("Expected generateContent in path, got %s", r.URL.Path)
		}

		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req)

		response := map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]interface{}{
							{"text": `{"summary": "This is a test summary of the article.", "tags": ["go", "programming"]}`},
						},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	s := NewSummarizer(server.URL, "test-model", "test-api-key")
	result, err := s.Summarize("Test article title", "This is the article content.")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Summary != "This is a test summary of the article." {
		t.Errorf("Expected summary 'This is a test summary of the article.', got '%s'", result.Summary)
	}

	if len(result.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(result.Tags))
	}
}

func TestSummarizer_Summarize_WithMarkdown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]interface{}{
							{"text": "```json\n{\"summary\": \"Test summary\", \"tags\": [\"rust\"]}\n```"},
						},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	s := NewSummarizer(server.URL, "test-model", "test-api-key")
	result, err := s.Summarize("Test article", "Article content")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Summary != "Test summary" {
		t.Errorf("Expected summary 'Test summary', got '%s'", result.Summary)
	}
	if len(result.Tags) != 1 || result.Tags[0] != "rust" {
		t.Errorf("Expected tag ['rust'], got %v", result.Tags)
	}
}

func TestSummarizer_Summarize_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	s := NewSummarizer(server.URL, "test-model", "test-api-key")
	_, err := s.Summarize("Test", "Content")
	if err == nil {
		t.Error("Expected error for API error")
	}
}

func TestSummarizer_Summarize_InvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	s := NewSummarizer(server.URL, "test-model", "test-api-key")
	_, err := s.Summarize("Test", "Content")
	if err == nil {
		t.Error("Expected error for invalid response")
	}
}

func TestSummarizer_Summarize_EmptyCandidates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"candidates": []map[string]interface{}{},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	s := NewSummarizer(server.URL, "test-model", "test-api-key")
	_, err := s.Summarize("Test", "Content")
	if err == nil {
		t.Error("Expected error for empty candidates")
	}
}

func TestSummarizer_Summarize_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]interface{}{
							{"text": "not valid json"},
						},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	s := NewSummarizer(server.URL, "test-model", "test-api-key")
	_, err := s.Summarize("Test", "Content")
	if err == nil {
		t.Error("Expected error for invalid JSON in response")
	}
}

func TestSummarizer_Summarize_MissingFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]interface{}{
							{"text": `{"summary": "Only summary"}`},
						},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	s := NewSummarizer(server.URL, "test-model", "test-api-key")
	result, err := s.Summarize("Test", "Content")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Summary != "Only summary" {
		t.Errorf("Expected summary 'Only summary', got '%s'", result.Summary)
	}
	if len(result.Tags) != 0 {
		t.Errorf("Expected empty tags, got %v", result.Tags)
	}
}
