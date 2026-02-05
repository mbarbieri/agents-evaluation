package summarizer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSummarizeSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := geminiResponse{Candidates: []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		}{{}}}
		resp.Candidates[0].Content.Parts = []struct{ Text string `json:"text"` }{{Text: `{"summary":"Great","tags":["Go","AI"]}`}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	s := NewGeminiSummarizer("key", "model", server.Client())
	s.BaseURL = server.URL
	result, err := s.Summarize(context.Background(), "content")
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if result.Summary != "Great" {
		t.Fatalf("unexpected summary: %q", result.Summary)
	}
	if len(result.Tags) != 2 || result.Tags[0] != "go" {
		t.Fatalf("unexpected tags: %v", result.Tags)
	}
}

func TestSummarizeCodeFence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := geminiResponse{Candidates: []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		}{{}}}
		resp.Candidates[0].Content.Parts = []struct{ Text string `json:"text"` }{{Text: "```json\n{\"summary\":\"Ok\",\"tags\":[\"news\"]}\n```"}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	s := NewGeminiSummarizer("key", "model", server.Client())
	s.BaseURL = server.URL
	result, err := s.Summarize(context.Background(), "content")
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if result.Summary != "Ok" || result.Tags[0] != "news" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestSummarizeMissingCandidates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(geminiResponse{})
	}))
	defer server.Close()

	s := NewGeminiSummarizer("key", "model", server.Client())
	s.BaseURL = server.URL
	_, err := s.Summarize(context.Background(), "content")
	if err == nil {
		t.Fatalf("expected error")
	}
}
