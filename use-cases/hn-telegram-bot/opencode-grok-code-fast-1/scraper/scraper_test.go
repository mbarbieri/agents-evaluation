package scraper

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestScrape_Success(t *testing.T) {
	html := `
	<html>
	<head><title>Test Article</title></head>
	<body>
		<h1>Test Article</h1>
		<p>This is the main content of the article.</p>
		<p>It has multiple paragraphs.</p>
	</body>
	</html>
	`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	}))
	defer server.Close()

	scraper := New(time.Second * 10)
	content := scraper.Scrape(server.URL, "fallback")

	if !strings.Contains(content, "main content") {
		t.Errorf("expected content to contain 'main content', got %s", content)
	}
	if len(content) > 4000 {
		t.Errorf("expected content length <= 4000, got %d", len(content))
	}
}

func TestScrape_FallbackToTitle(t *testing.T) {
	// HTML without readable content, but has title
	html := `
	<html>
	<head><title>Fallback Title</title></head>
	<body>
		<script>some script</script>
	</body>
	</html>
	`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	}))
	defer server.Close()

	scraper := New(time.Second * 10)
	content := scraper.Scrape(server.URL, "fallback")

	if content != "Fallback Title" {
		t.Errorf("expected fallback to title 'Fallback Title', got %s", content)
	}
}

func TestScrape_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	scraper := New(time.Second * 10)
	content := scraper.Scrape(server.URL, "fallback title")

	if content != "fallback title" {
		t.Errorf("expected fallback 'fallback title', got %s", content)
	}
}

func TestScrape_Truncate(t *testing.T) {
	longContent := strings.Repeat("a", 5000)
	html := `
	<html>
	<head><title>Title</title></head>
	<body>
		<p>` + longContent + `</p>
	</body>
	</html>
	`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	}))
	defer server.Close()

	scraper := New(time.Second * 10)
	content := scraper.Scrape(server.URL, "fallback")

	if len(content) != 4000 {
		t.Errorf("expected content length 4000, got %d", len(content))
	}
}
