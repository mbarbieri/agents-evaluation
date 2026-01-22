package hn

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client for Hacker News API
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// Item represents a Hacker News item
type Item struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	URL         string `json:"url,omitempty"`
	Score       int    `json:"score"`
	Descendants int    `json:"descendants"`
	By          string `json:"by"`
	Time        int    `json:"time"`
	Type        string `json:"type"`
}

// NewClient creates a new HN API client
func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// GetTopStories retrieves the top story IDs
func (c *Client) GetTopStories() ([]int, error) {
	url := c.baseURL + "/v0/topstories.json"
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HN API returned status %d", resp.StatusCode)
	}

	var ids []int
	if err := json.NewDecoder(resp.Body).Decode(&ids); err != nil {
		return nil, err
	}

	return ids, nil
}

// GetItem retrieves an item by ID
func (c *Client) GetItem(id int) (Item, error) {
	url := fmt.Sprintf("%s/v0/item/%d.json", c.baseURL, id)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return Item{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Item{}, fmt.Errorf("HN API returned status %d", resp.StatusCode)
	}

	var item Item
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return Item{}, err
	}

	return item, nil
}
