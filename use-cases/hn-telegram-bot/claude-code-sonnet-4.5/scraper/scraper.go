package scraper

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-shiori/go-readability"
)

const maxContentLength = 4000

type Scraper interface {
	Scrape(url string) (string, error)
}

type ReadabilityScraper struct {
	timeout time.Duration
	client  *http.Client
}

func New(timeoutSecs int) *ReadabilityScraper {
	timeout := time.Duration(timeoutSecs) * time.Second
	return &ReadabilityScraper{
		timeout: timeout,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (s *ReadabilityScraper) Scrape(url string) (string, error) {
	article, err := readability.FromURL(url, s.timeout)
	if err != nil {
		return "", fmt.Errorf("failed to scrape %s: %w", url, err)
	}

	content := article.TextContent
	if len(content) > maxContentLength {
		content = content[:maxContentLength]
	}

	return content, nil
}
