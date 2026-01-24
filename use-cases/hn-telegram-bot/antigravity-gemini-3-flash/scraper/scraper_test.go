package scraper

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestScrape(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/article", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><body><article><h1>Title</h1><p>This is a test article content that is long enough to be extracted.</p></article></body></html>`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	s := NewScraper(10 * time.Second)

	t.Run("ScrapeSuccess", func(t *testing.T) {
		content, err := s.Scrape(server.URL + "/article")
		if err != nil {
			t.Fatalf("failed to scrape: %v", err)
		}
		if content == "" {
			t.Error("expected content, got empty string")
		}
	})

	t.Run("ScrapeFailure", func(t *testing.T) {
		_, err := s.Scrape(server.URL + "/404")
		if err == nil {
			t.Error("expected error for 404, got nil")
		}
	})
}
