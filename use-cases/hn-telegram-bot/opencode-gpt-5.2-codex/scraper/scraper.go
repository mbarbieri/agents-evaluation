package scraper

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-shiori/go-readability"
)

type Scraper struct {
	http    *http.Client
	timeout time.Duration
}

func New(httpClient *http.Client, timeout time.Duration) *Scraper {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Scraper{http: httpClient, timeout: timeout}
}

func (s *Scraper) Extract(ctx context.Context, url string) (string, error) {
	if s == nil || s.http == nil {
		return "", errors.New("scraper not initialized")
	}
	if url == "" {
		return "", errors.New("url required")
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	article, err := readability.FromURL(url, s.timeout)
	if err != nil {
		return "", fmt.Errorf("readability: %w", err)
	}
	return article.TextContent, nil
}
