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
	client *http.Client
}

func New(timeout time.Duration) *Scraper {
	return &Scraper{
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (s *Scraper) Scrape(pageURL string) (string, error) {
	parsedURL, err := url.Parse(pageURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	// Fetch manually to control context/timeout if needed beyond client timeout,
	// and to ensure we get a reader.
	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return "", err
	}
	// Add user agent just in case
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; HNBot/1.0)")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status: %d", resp.StatusCode)
	}

	article, err := readability.FromReader(resp.Body, parsedURL)
	if err != nil {
		return "", fmt.Errorf("readability parsing failed: %w", err)
	}

	content := article.TextContent
	if len(content) > 4000 {
		content = content[:4000] + "..."
	}

	// Clean up extra whitespace usually left by text conversion
	content = strings.Join(strings.Fields(content), " ")

	return content, nil
}
