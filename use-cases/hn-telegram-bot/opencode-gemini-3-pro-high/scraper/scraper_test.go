package scraper

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestScrape(t *testing.T) {
	htmlContent := `
		<html>
			<head><title>Test Page</title></head>
			<body>
				<nav>Menu</nav>
				<article>
					<h1>Real Content</h1>
					<p>This is the paragraph that should be extracted.</p>
					<p>Another meaningful paragraph.</p>
				</article>
				<footer>Footer stuff</footer>
			</body>
		</html>
	`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, htmlContent)
	}))
	defer ts.Close()

	scraper := New(10 * time.Second)
	content, err := scraper.Scrape(ts.URL)
	if err != nil {
		t.Fatalf("Scrape failed: %v", err)
	}

	if !strings.Contains(content, "This is the paragraph") {
		t.Error("Content missing first paragraph")
	}
	if strings.Contains(content, "Footer stuff") {
		t.Error("Content should not contain footer")
	}
}

func TestScrapeTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer ts.Close()

	// Short timeout
	scraper := New(50 * time.Millisecond)
	_, err := scraper.Scrape(ts.URL)
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
}
