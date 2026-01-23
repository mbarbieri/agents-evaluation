package scraper

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-shiori/go-readability"
)

type Scraper struct {
	HTTPClient *http.Client
	Timeout    time.Duration
	MaxChars   int
}

func New(httpClient *http.Client, timeout time.Duration) Scraper {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: timeout}
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return Scraper{HTTPClient: httpClient, Timeout: timeout, MaxChars: 4000}
}

func (s Scraper) Extract(ctx context.Context, pageURL string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, s.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}
	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch url: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("fetch status %d: %s", resp.StatusCode, string(b))
	}

	article, err := readability.FromReader(resp.Body, req.URL)
	if err != nil {
		return "", fmt.Errorf("readability: %w", err)
	}

	text := strings.TrimSpace(article.TextContent)
	if text == "" {
		text = strings.TrimSpace(article.Title)
	}
	if s.MaxChars > 0 && len(text) > s.MaxChars {
		text = text[:s.MaxChars]
	}
	return text, nil
}
