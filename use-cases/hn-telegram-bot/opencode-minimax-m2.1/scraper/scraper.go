package scraper

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
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

func (s *Scraper) Scrape(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "HN-Bot/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	article, err := readability.FromReader(resp.Body, u)
	if err != nil {
		return "", fmt.Errorf("failed to parse article: %w", err)
	}

	content := strings.TrimSpace(article.TextContent)
	if len(content) > 4000 {
		content = content[:4000]
	}

	return content, nil
}
