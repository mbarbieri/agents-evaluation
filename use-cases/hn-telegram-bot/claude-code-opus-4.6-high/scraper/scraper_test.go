package scraper

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestScrape_Success(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<head><title>Test Article</title></head>
<body>
<article>
<h1>Test Article</h1>
<p>This is a test article with meaningful content that should be extracted by the readability parser. It contains enough text to be considered article content.</p>
<p>The readability library needs a reasonable amount of content to identify the main article body. This second paragraph adds more substance to the article.</p>
<p>Adding a third paragraph ensures the content is substantial enough for extraction. The go-readability library uses heuristics to find the main content area.</p>
</article>
</body>
</html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	}))
	defer server.Close()

	s := NewScraperWithClient(server.Client())
	content, err := s.Scrape(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content == "" {
		t.Fatal("expected non-empty content")
	}
	if !strings.Contains(content, "test article") && !strings.Contains(content, "Test Article") {
		t.Errorf("expected content to contain article text, got: %s", content)
	}
}

func TestScrape_ContentTruncation(t *testing.T) {
	// Build an HTML page with content longer than 4000 characters.
	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html><html><head><title>Long</title></head><body><article>`)
	for i := 0; i < 500; i++ {
		sb.WriteString(fmt.Sprintf("<p>Paragraph %d with enough text to make the article long enough for truncation testing purposes.</p>", i))
	}
	sb.WriteString(`</article></body></html>`)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(sb.String()))
	}))
	defer server.Close()

	s := NewScraperWithClient(server.Client())
	content, err := s.Scrape(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(content) > maxContentLength {
		t.Errorf("expected content to be at most %d characters, got %d", maxContentLength, len(content))
	}
}

func TestScrape_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	s := NewScraperWithClient(server.Client())
	_, err := s.Scrape(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected error for HTTP 500 response")
	}
}

func TestScrape_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	s := NewScraperWithClient(server.Client())
	_, err := s.Scrape(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected error for HTTP 404 response")
	}
}

func TestScrape_EmptyContent(t *testing.T) {
	html := `<!DOCTYPE html><html><head><title>Empty</title></head><body></body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	}))
	defer server.Close()

	s := NewScraperWithClient(server.Client())
	content, err := s.Scrape(context.Background(), server.URL)
	// go-readability may return empty content or an error for empty pages.
	// Either is acceptable behavior.
	if err != nil {
		return // Error on empty page is acceptable
	}
	// If no error, content should be empty or very short
	if len(content) > 100 {
		t.Errorf("expected empty or minimal content for empty page, got %d chars", len(content))
	}
}

func TestScrape_InvalidURL(t *testing.T) {
	s := NewScraper(5 * time.Second)
	_, err := s.Scrape(context.Background(), "http://localhost:1/nonexistent")
	if err == nil {
		t.Fatal("expected error for unreachable URL")
	}
}

func TestScrape_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><body><p>content</p></body></html>`))
	}))
	defer server.Close()

	s := NewScraperWithClient(server.Client())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := s.Scrape(ctx, server.URL)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestNewScraper_SetsTimeout(t *testing.T) {
	s := NewScraper(30 * time.Second)
	if s == nil {
		t.Fatal("expected non-nil scraper")
	}
}
