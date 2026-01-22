package hn

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const defaultBaseURL = "https://hacker-news.firebaseio.com"

// Item represents a Hacker News item (story, comment, etc.)
type Item struct {
	ID          int    `json:"id"`
	Deleted     bool   `json:"deleted"`
	Type        string `json:"type"`
	By          string `json:"by"`
	Time        int64  `json:"time"`
	Text        string `json:"text"`
	Dead        bool   `json:"dead"`
	Parent      int    `json:"parent"`
	Poll        int    `json:"poll"`
	Kids        []int  `json:"kids"`
	URL         string `json:"url"`
	Score       int    `json:"score"`
	Title       string `json:"title"`
	Parts       []int  `json:"parts"`
	Descendants int    `json:"descendants"`
}

// Client interacts with the Hacker News API
type Client struct {
	BaseURL    string
	httpClient *http.Client
}

// NewClient creates a new HN API client
func NewClient() *Client {
	return &Client{
		BaseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetTopStories returns the top 500 story IDs
func (c *Client) GetTopStories() ([]int, error) {
	url := fmt.Sprintf("%s/v0/topstories.json", c.BaseURL)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch top stories: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var ids []int
	if err := json.NewDecoder(resp.Body).Decode(&ids); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return ids, nil
}

// GetItem returns details for a specific item ID
func (c *Client) GetItem(id int) (*Item, error) {
	url := fmt.Sprintf("%s/v0/item/%d.json", c.BaseURL, id)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch item %d: %w", id, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var item Item
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, fmt.Errorf("failed to decode item: %w", err)
	}

	return &item, nil
}
