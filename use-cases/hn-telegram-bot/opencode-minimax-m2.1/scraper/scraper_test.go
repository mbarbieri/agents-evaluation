package scraper

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestScraper_Scrape_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`
			<!DOCTYPE html>
			<html>
			<head><title>Test Article</title></head>
			<body>
				<article>
					<h1>Test Article Title</h1>
					<p>This is the content of the test article. It has some interesting information.</p>
					<p>More content here with more details about the topic.</p>
				</article>
			</body>
			</html>
		`))
	}))
	defer server.Close()

	s := NewScraper(10)
	content, err := s.Scrape(server.URL)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if content == "" {
		t.Error("Expected non-empty content")
	}

	if len(content) > 4000 {
		t.Errorf("Expected content truncated to 4000 chars, got %d", len(content))
	}
}

func TestScraper_Scrape_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		<-r.Context().Done()
	}))
	defer server.Close()

	s := NewScraper(1) // 1 second timeout
	_, err := s.Scrape(server.URL)
	if err == nil {
		t.Error("Expected error for timeout")
	}
}

func TestScraper_Scrape_InvalidURL(t *testing.T) {
	s := NewScraper(10)
	_, err := s.Scrape("not-a-valid-url")
	if err == nil {
		t.Error("Expected error for invalid URL")
	}
}

func TestScraper_Scrape_EmptyContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<!DOCTYPE html><html><body></body></html>`))
	}))
	defer server.Close()

	s := NewScraper(10)
	content, err := s.Scrape(server.URL)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if content != "" {
		t.Errorf("Expected empty content for page with no article, got '%s'", content)
	}
}

func TestScraper_Scrape_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	s := NewScraper(10)
	_, err := s.Scrape(server.URL)
	if err == nil {
		t.Error("Expected error for HTTP error")
	}
}

func TestScraper_Scrape_Truncation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create content longer than 4000 characters
		content := "<article><p>"
		for i := 0; i < 5000; i++ {
			content += "x"
		}
		content += "</p></article>"
		w.Write([]byte(content))
	}))
	defer server.Close()

	s := NewScraper(10)
	content, err := s.Scrape(server.URL)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(content) > 4000 {
		t.Errorf("Expected content truncated to 4000 chars, got %d", len(content))
	}
}
