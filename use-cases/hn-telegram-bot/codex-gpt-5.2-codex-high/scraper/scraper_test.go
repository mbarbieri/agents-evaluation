package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Fatalf("unexpected truncate result: %q", got)
	}
	if got := truncate("hello", 4); got != "hell" {
		t.Fatalf("unexpected truncate result: %q", got)
	}
}

func TestScrape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!doctype html><html><head><title>Test</title></head><body><article><p>Hello world.</p></article></body></html>`))
	}))
	defer server.Close()

	s := NewReadabilityScraper(2 * time.Second)
	text, err := s.Scrape(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("Scrape: %v", err)
	}
	if !strings.Contains(text, "Hello world") {
		t.Fatalf("expected content in text, got %q", text)
	}
}
