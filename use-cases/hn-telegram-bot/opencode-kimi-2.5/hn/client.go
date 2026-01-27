package hn

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const defaultBaseURL = "https://hacker-news.firebaseio.com"

// Story represents a Hacker News story/item
type Story struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Score       int    `json:"score"`
	Descendants int    `json:"descendants"`
	By          string `json:"by"`
	Time        int64  `json:"time"`
	Type        string `json:"type"`
}

// Client is the Hacker News API client
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient creates a new HN client with the specified timeout
func NewClient(timeout time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		baseURL: defaultBaseURL,
	}
}

// NewClientWithBaseURL creates a client with a custom base URL (for testing)
func NewClientWithBaseURL(timeout time.Duration, baseURL string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		baseURL: baseURL,
	}
}

// GetTopStories returns the IDs of the top stories
func (c *Client) GetTopStories(limit int) ([]int64, error) {
	url := fmt.Sprintf("%s/v0/topstories.json", c.baseURL)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch top stories: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var ids []int64
	if err := json.NewDecoder(resp.Body).Decode(&ids); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if limit > 0 && len(ids) > limit {
		ids = ids[:limit]
	}

	return ids, nil
}

// GetItem retrieves a specific item by ID
func (c *Client) GetItem(id int64) (*Story, error) {
	url := fmt.Sprintf("%s/v0/item/%d.json", c.baseURL, id)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch item %d: %w", id, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code for item %d: %d", id, resp.StatusCode)
	}

	var story Story
	if err := json.NewDecoder(resp.Body).Decode(&story); err != nil {
		return nil, fmt.Errorf("failed to decode item %d: %w", id, err)
	}

	return &story, nil
}
