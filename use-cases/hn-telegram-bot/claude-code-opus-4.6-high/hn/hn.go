package hn

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const BaseURL = "https://hacker-news.firebaseio.com"

// Item represents a Hacker News item.
type Item struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Score       int    `json:"score"`
	Descendants int    `json:"descendants"`
	By          string `json:"by"`
	Time        int64  `json:"time"`
	Type        string `json:"type"`
}

// Client interface for HN API operations.
type Client interface {
	TopStories(ctx context.Context, limit int) ([]int, error)
	GetItem(ctx context.Context, id int) (*Item, error)
}

type httpClient struct {
	client  *http.Client
	baseURL string
}

// NewClient creates a new HN API client with the given HTTP client.
func NewClient(client *http.Client) Client {
	if client == nil {
		client = http.DefaultClient
	}
	return &httpClient{
		client:  client,
		baseURL: BaseURL,
	}
}

// NewClientWithBaseURL creates a new HN API client with a custom base URL (for testing).
func NewClientWithBaseURL(client *http.Client, baseURL string) Client {
	if client == nil {
		client = http.DefaultClient
	}
	return &httpClient{
		client:  client,
		baseURL: baseURL,
	}
}

// TopStories fetches the top story IDs from HN, returning up to limit IDs.
func (c *httpClient) TopStories(ctx context.Context, limit int) ([]int, error) {
	url := fmt.Sprintf("%s/v0/topstories.json", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating top stories request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching top stories: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("top stories returned status %d", resp.StatusCode)
	}

	var ids []int
	if err := json.NewDecoder(resp.Body).Decode(&ids); err != nil {
		return nil, fmt.Errorf("decoding top stories response: %w", err)
	}

	if limit > 0 && limit < len(ids) {
		ids = ids[:limit]
	}

	return ids, nil
}

// GetItem fetches a single HN item by ID.
func (c *httpClient) GetItem(ctx context.Context, id int) (*Item, error) {
	url := fmt.Sprintf("%s/v0/item/%d.json", c.baseURL, id)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating item request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching item %d: %w", id, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("item %d not found", id)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("item %d returned status %d", id, resp.StatusCode)
	}

	var item Item
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, fmt.Errorf("decoding item %d: %w", id, err)
	}

	return &item, nil
}
