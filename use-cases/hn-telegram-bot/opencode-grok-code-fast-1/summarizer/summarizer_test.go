package summarizer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestSummarize_Success(t *testing.T) {
	expected := Summary{
		Summary: "This is a summary.",
		Tags:    []string{"go", "programming", "tech"},
	}
	response := map[string]interface{}{
		"candidates": []map[string]interface{}{
			{
				"content": map[string]interface{}{
					"parts": []map[string]interface{}{
						{
							"text": `{"summary": "This is a summary.", "tags": ["go", "programming", "tech"]}`,
						},
					},
				},
			},
		},
	}
	responseJSON, _ := json.Marshal(response)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(responseJSON)
	}))
	defer server.Close()

	summarizer := New("test_key", "gemini-1.0", server.URL, time.Second)
	summary, err := summarizer.Summarize("test content")
	if err != nil {
		t.Fatal(err)
	}

	if summary.Summary != expected.Summary {
		t.Errorf("expected summary %s, got %s", expected.Summary, summary.Summary)
	}
	if !reflect.DeepEqual(summary.Tags, expected.Tags) {
		t.Errorf("expected tags %v, got %v", expected.Tags, summary.Tags)
	}
}

func TestSummarize_MarkdownWrapped(t *testing.T) {
	response := map[string]interface{}{
		"candidates": []map[string]interface{}{
			{
				"content": map[string]interface{}{
					"parts": []map[string]interface{}{
						{
							"text": "```json\n{\"summary\": \"Wrapped summary.\", \"tags\": [\"ai\"]}\n```",
						},
					},
				},
			},
		},
	}
	responseJSON, _ := json.Marshal(response)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(responseJSON)
	}))
	defer server.Close()

	summarizer := New("test_key", "gemini-1.0", server.URL, time.Second)
	summary, err := summarizer.Summarize("test content")
	if err != nil {
		t.Fatal(err)
	}

	if summary.Summary != "Wrapped summary." {
		t.Errorf("expected summary 'Wrapped summary.', got %s", summary.Summary)
	}
	if !reflect.DeepEqual(summary.Tags, []string{"ai"}) {
		t.Errorf("expected tags ['ai'], got %v", summary.Tags)
	}
}

func TestSummarize_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	summarizer := New("test_key", "gemini-1.0", server.URL, time.Second)
	_, err := summarizer.Summarize("test content")
	if err == nil {
		t.Error("expected error")
	}
}

func TestSummarize_InvalidJSON(t *testing.T) {
	response := map[string]interface{}{
		"candidates": []map[string]interface{}{
			{
				"content": map[string]interface{}{
					"parts": []map[string]interface{}{
						{
							"text": "invalid json",
						},
					},
				},
			},
		},
	}
	responseJSON, _ := json.Marshal(response)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(responseJSON)
	}))
	defer server.Close()

	summarizer := New("test_key", "gemini-1.0", server.URL, time.Second)
	_, err := summarizer.Summarize("test content")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
