package scraper

import (
	"context"
	"fmt"
	"net/http"
	"time"

	readability "github.com/go-shiori/go-readability"
)

const maxContentLength = 4000

// Scraper interface for article content extraction.
type Scraper interface {
	Scrape(ctx context.Context, url string) (string, error)
}

type httpScraper struct {
	client *http.Client
}

// NewScraper creates a new Scraper with the given timeout for HTTP requests.
func NewScraper(timeout time.Duration) Scraper {
	return &httpScraper{
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// NewScraperWithClient creates a new Scraper with a custom HTTP client (for testing).
func NewScraperWithClient(client *http.Client) Scraper {
	return &httpScraper{
		client: client,
	}
}

// Scrape fetches the given URL and extracts readable text content.
// Content is truncated to 4000 characters.
func (s *httpScraper) Scrape(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("creating scrape request for %s: %w", url, err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("scraping %s returned status %d", url, resp.StatusCode)
	}

	article, err := readability.FromReader(resp.Body, nil)
	if err != nil {
		return "", fmt.Errorf("extracting content from %s: %w", url, err)
	}

	content := article.TextContent
	if len(content) > maxContentLength {
		content = content[:maxContentLength]
	}

	return content, nil
}
