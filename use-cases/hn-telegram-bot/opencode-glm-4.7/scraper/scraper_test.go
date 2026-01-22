package scraper

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestScrape(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<head>
	<title>Test Article</title>
</head>
<body>
	<h1>Main Heading</h1>
	<p>This is a test article with multiple paragraphs.</p>
	<p>Here is the second paragraph with more content.</p>
	<p>And a third paragraph to ensure we have enough text.</p>
</body>
</html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	}))
	defer server.Close()

	scraper := NewScraper(10)
	content, err := scraper.Scrape(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if content == "" {
		t.Error("expected non-empty content")
	}

	if !strings.Contains(content, "test article") {
		t.Error("content should contain 'test article'")
	}
}

func TestScrapeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	scraper := NewScraper(10)
	_, err := scraper.Scrape(server.URL)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestScrapeTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	scraper := NewScraper(1)
	_, err := scraper.Scrape(server.URL)
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

func TestScrapeTruncatesLongContent(t *testing.T) {
	longContent := strings.Repeat("Word ", 1000)
	html := `<!DOCTYPE html>
<html>
<body>
	<p>` + longContent + `</p>
</body>
</html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	}))
	defer server.Close()

	scraper := NewScraper(10)
	content, err := scraper.Scrape(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(content) > 4000 {
		t.Errorf("content length = %d, want <= 4000", len(content))
	}
}

func TestScrapeNonHTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("Plain text content"))
	}))
	defer server.Close()

	scraper := NewScraper(10)
	content, err := scraper.Scrape(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if content == "" {
		t.Error("expected some content to be extracted")
	}
}

func TestScrapeEmptyHTML(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<body>
</body>
</html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	}))
	defer server.Close()

	scraper := NewScraper(10)
	content, err := scraper.Scrape(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if content != "" {
		t.Error("expected empty content from empty HTML body")
	}
}

func TestScrapeInvalidURL(t *testing.T) {
	scraper := NewScraper(10)
	_, err := scraper.Scrape("://invalid-url")
	if err == nil {
		t.Error("expected error for invalid URL, got nil")
	}
}
