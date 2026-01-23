package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestScrapeReturnsContent(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body><article><p>Hello</p></article></body></html>"))
	}))
	defer server.Close()

	s := New(&http.Client{Timeout: time.Second}, 5*time.Second)
	content, err := s.Extract(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if content == "" {
		t.Fatalf("expected content")
	}
}

func TestScrapeInvalidURL(t *testing.T) {
	t.Parallel()
	s := New(&http.Client{Timeout: time.Second}, 5*time.Second)
	_, err := s.Extract(context.Background(), "")
	if err == nil {
		t.Fatalf("expected error")
	}
}
