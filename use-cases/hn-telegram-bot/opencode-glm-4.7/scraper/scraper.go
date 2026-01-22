package scraper

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-shiori/go-readability"
)

type Scraper struct {
	httpClient *http.Client
	maxChars   int
}

func NewScraper(timeoutSecs int) *Scraper {
	return &Scraper{
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSecs) * time.Second,
		},
		maxChars: 4000,
	}
}

func (s *Scraper) Scrape(targetURL string) (string, error) {
	resp, err := s.httpClient.Get(targetURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		return string(body), nil
	}

	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %w", err)
	}

	article, err := readability.FromReader(strings.NewReader(string(body)), parsedURL)
	if err != nil {
		return "", fmt.Errorf("failed to extract article content: %w", err)
	}

	content := article.TextContent
	if len(content) > s.maxChars {
		content = content[:s.maxChars]
	}

	return content, nil
}
