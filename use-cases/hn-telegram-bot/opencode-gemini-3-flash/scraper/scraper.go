package scraper

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/go-shiori/go-readability"
)

type Scraper struct {
	httpClient *http.Client
}

func NewScraper(timeoutSecs int) *Scraper {
	return &Scraper{
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSecs) * time.Second,
		},
	}
}

func (s *Scraper) Scrape(urlString string) (string, error) {
	parsedURL, err := url.Parse(urlString)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %w", err)
	}

	resp, err := s.httpClient.Get(urlString)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	article, err := readability.FromReader(resp.Body, parsedURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse article: %w", err)
	}

	// Limit to 4000 characters as per spec
	content := article.TextContent
	if len(content) > 4000 {
		content = content[:4000]
	}

	return content, nil
}
