package scraper

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}
	scraper := New(client)

	if scraper == nil {
		t.Error("New() returned nil")
	}
	if scraper.httpClient != client {
		t.Error("New() did not set httpClient correctly")
	}
}

func TestScrape(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		contentType string
		maxLength   int
		wantErr     bool
		errIs       error
	}{
		{
			name: "successful scrape",
			content: `
				<!DOCTYPE html>
				<html>
				<head><title>Test Article</title></head>
				<body>
					<article>
						<h1>Test Article Title</h1>
						<p>This is the main content of the article. It should be extracted.</p>
						<p>More content here.</p>
					</article>
				</body>
				</html>
			`,
			contentType: "text/html",
			maxLength:   1000,
			wantErr:     false,
		},
		{
			name: "content truncated to max length",
			content: `
				<!DOCTYPE html>
				<html>
				<head><title>Long Article</title></head>
				<body>
					<article>
						<p>This is a very long article content that should be truncated.</p>
						<p>More and more content here to make it longer.</p>
						<p>Even more content to ensure we exceed the limit.</p>
						<p>Lorem ipsum dolor sit amet, consectetur adipiscing elit.</p>
					</article>
				</body>
				</html>
			`,
			contentType: "text/html",
			maxLength:   50,
			wantErr:     false,
		},
		{
			name:        "non-HTML content type",
			content:     `{"message": "This is JSON"}`,
			contentType: "application/json",
			maxLength:   1000,
			wantErr:     true,
			errIs:       ErrNonHTMLContent,
		},
		{
			name:        "empty content",
			content:     "",
			contentType: "text/html",
			maxLength:   1000,
			wantErr:     false, // readability may return empty content without error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tt.contentType)
				w.Write([]byte(tt.content))
			}))
			defer server.Close()

			client := &http.Client{Timeout: 5 * time.Second}
			scraper := New(client)

			content, err := scraper.Scrape(server.URL, tt.maxLength)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Scrape() expected error, got nil")
					return
				}
				if tt.errIs != nil && !errors.Is(err, tt.errIs) {
					t.Errorf("Scrape() error = %v, expected %v", err, tt.errIs)
				}
				return
			}

			if err != nil {
				t.Errorf("Scrape() unexpected error = %v", err)
				return
			}

			if content == "" && tt.name != "empty content" {
				t.Error("Scrape() returned empty content")
			}

			if len(content) > tt.maxLength {
				t.Errorf("Content length %d exceeds max %d", len(content), tt.maxLength)
			}
		})
	}
}

func TestScrapeNetworkError(t *testing.T) {
	client := &http.Client{Timeout: 1 * time.Second}
	scraper := New(client)

	// Use an invalid URL that will cause a network error
	_, err := scraper.Scrape("http://invalid.localhost:99999", 1000)
	if err == nil {
		t.Error("Scrape() expected network error, got nil")
	}
}

func TestScrapeWithFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
			<!DOCTYPE html>
			<html>
			<body>
				<p>Readable content here.</p>
			</body>
			</html>
		`))
	}))
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	scraper := New(client)

	content, err := scraper.Scrape(server.URL, 100)
	if err != nil {
		t.Errorf("Scrape() unexpected error = %v", err)
		return
	}

	// Content should be truncated to 100 characters
	if len(content) > 100 {
		t.Errorf("Content length %d exceeds max 100", len(content))
	}
}

func TestErrorTypes(t *testing.T) {
	if ErrNonHTMLContent == nil {
		t.Error("ErrNonHTMLContent should not be nil")
	}

	if ErrScrapeFailed == nil {
		t.Error("ErrScrapeFailed should not be nil")
	}
}

func TestScrapeRealWorldHTML(t *testing.T) {
	html := `
	<!DOCTYPE html>
	<html lang="en">
	<head>
		<meta charset="UTF-8">
		<title>Sample Article</title>
	</head>
	<body>
		<header>
			<nav>Navigation links here</nav>
		</header>
		<main>
			<article>
				<h1>Article Title</h1>
				<p class="byline">By Author Name</p>
				<div class="content">
					<p>This is the first paragraph of the article. It contains important information about the topic.</p>
					<p>This is the second paragraph with more details and context.</p>
					<p>The conclusion wraps up the key points discussed.</p>
				</div>
			</article>
		</main>
		<footer>
			<p>Copyright 2024</p>
		</footer>
	</body>
	</html>
	`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	}))
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	scraper := New(client)

	content, err := scraper.Scrape(server.URL, 500)
	if err != nil {
		t.Errorf("Scrape() unexpected error = %v", err)
		return
	}

	// Content should contain the article text, not navigation or footer
	if content == "" {
		t.Error("Scrape() returned empty content")
	}

	// Should not contain navigation or footer text
	if strings.Contains(content, "Navigation links here") {
		t.Error("Content should not contain navigation text")
	}

	if strings.Contains(content, "Copyright") {
		t.Error("Content should not contain footer text")
	}
}
