package scraper

import (
	"context"
	"strings"
	"time"

	"github.com/go-shiori/go-readability"
)

const maxContentChars = 4000

// Scraper extracts readable article content from a URL.
type Scraper interface {
	Scrape(ctx context.Context, url string) (string, error)
}

// ReadabilityScraper uses go-readability to parse articles.
type ReadabilityScraper struct {
	Timeout time.Duration
}

// NewReadabilityScraper creates a new scraper with a timeout.
func NewReadabilityScraper(timeout time.Duration) *ReadabilityScraper {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &ReadabilityScraper{Timeout: timeout}
}

// Scrape fetches and extracts the article text.
func (s *ReadabilityScraper) Scrape(ctx context.Context, url string) (string, error) {
	_ = ctx
	article, err := readability.FromURL(url, s.Timeout)
	if err != nil {
		return "", err
	}
	text := strings.TrimSpace(article.TextContent)
	return truncate(text, maxContentChars), nil
}

func truncate(input string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if len(input) <= limit {
		return input
	}
	return string([]rune(input)[:limit])
}
