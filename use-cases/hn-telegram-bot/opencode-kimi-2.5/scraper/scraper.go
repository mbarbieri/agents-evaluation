package scraper

import (
	"errors"
	"fmt"
	"net/http"
	nurl "net/url"
	"strings"

	"github.com/go-shiori/go-readability"
)

var (
	// ErrNonHTMLContent is returned when the content type is not HTML
	ErrNonHTMLContent = errors.New("content is not HTML")
	// ErrScrapeFailed is returned when scraping fails
	ErrScrapeFailed = errors.New("failed to scrape article")
)

// Scraper extracts readable content from web pages
type Scraper struct {
	httpClient *http.Client
}

// New creates a new Scraper instance
func New(httpClient *http.Client) *Scraper {
	return &Scraper{
		httpClient: httpClient,
	}
}

// Scrape fetches and extracts readable content from a URL
// Returns the extracted text limited to maxLength characters
func (s *Scraper) Scrape(url string, maxLength int) (string, error) {
	resp, err := s.httpClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrScrapeFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: HTTP %d", ErrScrapeFailed, resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "" && !strings.Contains(contentType, "text/html") {
		return "", ErrNonHTMLContent
	}

	pageURL, err := nurl.Parse(url)
	if err != nil {
		return "", fmt.Errorf("%w: invalid URL: %v", ErrScrapeFailed, err)
	}

	article, err := readability.FromReader(resp.Body, pageURL)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrScrapeFailed, err)
	}

	content := article.TextContent
	if len(content) > maxLength {
		content = content[:maxLength]
	}

	return content, nil
}
