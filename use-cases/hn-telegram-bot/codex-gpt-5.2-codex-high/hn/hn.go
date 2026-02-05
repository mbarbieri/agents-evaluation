package hn

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const defaultBaseURL = "https://hacker-news.firebaseio.com"

// Client fetches data from the Hacker News API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// Item represents a Hacker News item.
type Item struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Score       int    `json:"score"`
	Descendants int    `json:"descendants"`
	By          string `json:"by"`
	Time        int64  `json:"time"`
	Type        string `json:"type"`
}

// NewClient creates a new HN API client.
func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{baseURL: defaultBaseURL, httpClient: httpClient}
}

// NewClientWithBaseURL allows overriding the base URL (useful for tests).
func NewClientWithBaseURL(httpClient *http.Client, baseURL string) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{baseURL: baseURL, httpClient: httpClient}
}

// TopStories returns the top story IDs.
func (c *Client) TopStories(ctx context.Context) ([]int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/v0/topstories.json", c.baseURL), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request topstories: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("topstories status: %d", resp.StatusCode)
	}
	var ids []int64
	if err := json.NewDecoder(resp.Body).Decode(&ids); err != nil {
		return nil, fmt.Errorf("decode topstories: %w", err)
	}
	return ids, nil
}

// Item fetches a single item by ID.
func (c *Client) Item(ctx context.Context, id int64) (Item, error) {
	url := fmt.Sprintf("%s/v0/item/%d.json", c.baseURL, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Item{}, fmt.Errorf("build request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Item{}, fmt.Errorf("request item: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Item{}, fmt.Errorf("item status: %d", resp.StatusCode)
	}
	var item Item
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return Item{}, fmt.Errorf("decode item: %w", err)
	}
	return item, nil
}
