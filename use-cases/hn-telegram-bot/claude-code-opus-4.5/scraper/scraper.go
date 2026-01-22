package scraper

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-shiori/go-readability"
)

const defaultMaxContentLen = 4000

// Scraper extracts readable content from web pages.
type Scraper struct {
	httpClient    *http.Client
	maxContentLen int
}

// Option configures a Scraper.
type Option func(*Scraper)

// WithTimeout sets the HTTP client timeout.
func WithTimeout(d time.Duration) Option {
	return func(s *Scraper) {
		s.httpClient.Timeout = d
	}
}

// WithMaxContentLength sets the maximum content length to return.
func WithMaxContentLength(n int) Option {
	return func(s *Scraper) {
		s.maxContentLen = n
	}
}

// NewScraper creates a new content scraper.
func NewScraper(opts ...Option) *Scraper {
	s := &Scraper{
		httpClient:    &http.Client{Timeout: 10 * time.Second},
		maxContentLen: defaultMaxContentLen,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Scrape extracts readable text content from a URL.
func (s *Scraper) Scrape(ctx context.Context, rawURL string) (string, error) {
	// Validate URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return "", fmt.Errorf("invalid URL: %s", rawURL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	// Set a user agent to avoid being blocked
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; HN-Bot/1.0)")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	article, err := readability.FromReader(resp.Body, parsedURL)
	if err != nil {
		return "", fmt.Errorf("parse content: %w", err)
	}

	content := strings.TrimSpace(article.TextContent)

	// Truncate if necessary
	if len(content) > s.maxContentLen {
		content = content[:s.maxContentLen]
	}

	return content, nil
}
