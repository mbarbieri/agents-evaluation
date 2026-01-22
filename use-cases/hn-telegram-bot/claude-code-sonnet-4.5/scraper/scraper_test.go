package scraper

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestScrape_Success(t *testing.T) {
	htmlContent := `
	<!DOCTYPE html>
	<html>
	<head><title>Test Article</title></head>
	<body>
		<article>
			<h1>Test Article Title</h1>
			<p>This is a test article with some content that should be extracted by the readability library.</p>
			<p>Multiple paragraphs should be included in the extracted content.</p>
		</article>
	</body>
	</html>
	`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(htmlContent))
	}))
	defer server.Close()

	s := New(10)
	content, err := s.Scrape(server.URL)
	if err != nil {
		t.Fatalf("Scrape() error = %v", err)
	}

	if content == "" {
		t.Error("Scrape() returned empty content")
	}

	// Should contain some of the article text
	if !strings.Contains(content, "test article") {
		t.Errorf("Scrape() content does not contain expected text, got: %s", content)
	}
}

func TestScrape_TruncatesLongContent(t *testing.T) {
	longText := strings.Repeat("This is a very long paragraph. ", 200)
	htmlContent := `
	<!DOCTYPE html>
	<html>
	<body>
		<article><p>` + longText + `</p></article>
	</body>
	</html>
	`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(htmlContent))
	}))
	defer server.Close()

	s := New(10)
	content, err := s.Scrape(server.URL)
	if err != nil {
		t.Fatalf("Scrape() error = %v", err)
	}

	if len(content) > 4000 {
		t.Errorf("Scrape() content length = %d, want <= 4000", len(content))
	}
}

func TestScrape_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	s := New(10)
	_, err := s.Scrape(server.URL)
	if err == nil {
		t.Error("Scrape() expected error for HTTP 404, got nil")
	}
}

func TestScrape_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.Write([]byte("<html><body>Too slow</body></html>"))
	}))
	defer server.Close()

	s := New(1) // 1 second timeout
	_, err := s.Scrape(server.URL)
	if err == nil {
		t.Error("Scrape() expected timeout error, got nil")
	}
}

func TestScrape_InvalidURL(t *testing.T) {
	s := New(10)
	_, err := s.Scrape("http://invalid-url-that-does-not-exist-12345.com")
	if err == nil {
		t.Error("Scrape() expected error for invalid URL, got nil")
	}
}

func TestNew_SetsTimeout(t *testing.T) {
	s := New(5)
	if s.timeout != 5*time.Second {
		t.Errorf("Timeout = %v, want 5s", s.timeout)
	}
}
