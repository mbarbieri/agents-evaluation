package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestScrapeArticle(t *testing.T) {
	htmlContent := `<!DOCTYPE html>
<html>
<head><title>Test Article</title></head>
<body>
<article>
<h1>Test Article Title</h1>
<p>This is the main content of the article. It contains important information that should be extracted.</p>
<p>Second paragraph with more details about the topic.</p>
</article>
</body>
</html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(htmlContent))
	}))
	defer server.Close()

	s := NewScraper(WithTimeout(5 * time.Second))
	ctx := context.Background()

	content, err := s.Scrape(ctx, server.URL)
	if err != nil {
		t.Fatalf("Scrape failed: %v", err)
	}

	if content == "" {
		t.Fatal("expected non-empty content")
	}

	// Should contain main content
	if !strings.Contains(content, "main content") {
		t.Errorf("content should contain 'main content', got: %s", content)
	}
}

func TestScrapeContentLimit(t *testing.T) {
	// Create content larger than limit
	largeContent := strings.Repeat("x", 5000)
	htmlContent := `<!DOCTYPE html>
<html>
<head><title>Test</title></head>
<body><p>` + largeContent + `</p></body>
</html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(htmlContent))
	}))
	defer server.Close()

	s := NewScraper(WithMaxContentLength(1000))
	ctx := context.Background()

	content, err := s.Scrape(ctx, server.URL)
	if err != nil {
		t.Fatalf("Scrape failed: %v", err)
	}

	if len(content) > 1000 {
		t.Errorf("content length = %d, want <= 1000", len(content))
	}
}

func TestScrapeServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	s := NewScraper()
	ctx := context.Background()

	_, err := s.Scrape(ctx, server.URL)
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

func TestScrapeContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Write([]byte("<html><body>content</body></html>"))
	}))
	defer server.Close()

	s := NewScraper()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := s.Scrape(ctx, server.URL)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestScrapeInvalidURL(t *testing.T) {
	s := NewScraper()
	ctx := context.Background()

	_, err := s.Scrape(ctx, "not-a-valid-url")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestScrapeEmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(""))
	}))
	defer server.Close()

	s := NewScraper()
	ctx := context.Background()

	// Empty body should still work (return empty content)
	content, err := s.Scrape(ctx, server.URL)
	if err != nil {
		t.Fatalf("Scrape failed: %v", err)
	}
	if content != "" {
		t.Errorf("expected empty content for empty body, got: %q", content)
	}
}

func TestScrapeMinimalHTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html><body><p>Simple text</p></body></html>"))
	}))
	defer server.Close()

	s := NewScraper()
	ctx := context.Background()

	content, err := s.Scrape(ctx, server.URL)
	if err != nil {
		t.Fatalf("Scrape failed: %v", err)
	}
	if !strings.Contains(content, "Simple text") {
		t.Errorf("content should contain 'Simple text', got: %q", content)
	}
}

func TestDefaultScraper(t *testing.T) {
	s := NewScraper()
	if s.maxContentLen != 4000 {
		t.Errorf("default maxContentLen = %d, want 4000", s.maxContentLen)
	}
}
