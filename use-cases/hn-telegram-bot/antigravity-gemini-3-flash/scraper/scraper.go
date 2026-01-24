package scraper

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	readability "github.com/go-shiori/go-readability"
)

type Scraper struct {
	hc *http.Client
}

func NewScraper(timeout time.Duration) *Scraper {
	return &Scraper{
		hc: &http.Client{
			Timeout: timeout,
		},
	}
}

func (s *Scraper) Scrape(articleURL string) (string, error) {
	resp, err := s.hc.Get(articleURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	parsedURL, err := url.Parse(articleURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %w", err)
	}

	article, err := readability.FromReader(resp.Body, parsedURL)
	if err != nil {
		return "", fmt.Errorf("failed to extract content: %w", err)
	}

	content := article.TextContent
	if len(content) > 4000 {
		content = content[:4000]
	}

	return content, nil
}
