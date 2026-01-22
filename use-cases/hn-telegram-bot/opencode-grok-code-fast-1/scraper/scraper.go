package scraper

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-shiori/go-readability"
)

// Scraper extracts readable content from URLs
type Scraper struct {
	httpClient *http.Client
}

// New creates a new scraper with the given timeout
func New(timeout time.Duration) *Scraper {
	return &Scraper{
		httpClient: &http.Client{Timeout: timeout},
	}
}

// Scrape extracts readable content from the URL, or returns fallbackTitle if fails
func (s *Scraper) Scrape(pageURL, fallbackTitle string) string {
	resp, err := s.httpClient.Get(pageURL)
	if err != nil {
		return fallbackTitle
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fallbackTitle
	}

	parsedURL, err := url.Parse(pageURL)
	if err != nil {
		return fallbackTitle
	}

	article, err := readability.FromReader(resp.Body, parsedURL)
	if err != nil {
		return fallbackTitle
	}

	content := article.TextContent
	if content == "" {
		content = article.Title
		if content == "" {
			return fallbackTitle
		}
	}

	if len(content) > 4000 {
		content = content[:4000]
	}

	return strings.TrimSpace(content)
}
