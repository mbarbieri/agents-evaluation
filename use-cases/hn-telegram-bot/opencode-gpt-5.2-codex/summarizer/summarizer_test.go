package summarizer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSummarizeParsesJSON(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]any{
			"candidates": []map[string]any{
				{
					"content": map[string]any{
						"parts": []map[string]string{{"text": "{\"summary\":\"ok\",\"tags\":[\"go\",\"ai\"]}"}},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(server.URL, "model", "key", &http.Client{Timeout: time.Second})
	res, err := client.Summarize(context.Background(), "content")
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if res.Summary != "ok" {
		t.Fatalf("expected summary")
	}
	if len(res.Tags) != 2 {
		t.Fatalf("expected tags")
	}
}

func TestSummarizeStripsCodeBlock(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]any{
			"candidates": []map[string]any{
				{
					"content": map[string]any{
						"parts": []map[string]string{{"text": "```json\n{\"summary\":\"ok\",\"tags\":[\"go\"]}\n```"}},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(server.URL, "model", "key", &http.Client{Timeout: time.Second})
	res, err := client.Summarize(context.Background(), "content")
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if res.Summary != "ok" {
		t.Fatalf("expected summary")
	}
}

func TestSummarizeHandlesBadStatus(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewClient(server.URL, "model", "key", &http.Client{Timeout: time.Second})
	_, err := client.Summarize(context.Background(), "content")
	if err == nil {
		t.Fatalf("expected error")
	}
}
