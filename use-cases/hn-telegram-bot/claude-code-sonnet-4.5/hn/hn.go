package hn

import (
	"encoding/json"
	"fmt"
	"net/http"
)

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

type Client interface {
	GetTopStories() ([]int, error)
	GetItem(id int) (*Item, error)
}

type HTTPClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *HTTPClient {
	if baseURL == "" {
		baseURL = "https://hacker-news.firebaseio.com"
	}
	return &HTTPClient{
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

func (c *HTTPClient) GetTopStories() ([]int, error) {
	url := fmt.Sprintf("%s/v0/topstories.json", c.baseURL)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch top stories: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HN API returned status %d", resp.StatusCode)
	}

	var ids []int
	if err := json.NewDecoder(resp.Body).Decode(&ids); err != nil {
		return nil, fmt.Errorf("failed to decode top stories: %w", err)
	}

	return ids, nil
}

func (c *HTTPClient) GetItem(id int) (*Item, error) {
	url := fmt.Sprintf("%s/v0/item/%d.json", c.baseURL, id)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch item %d: %w", id, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HN API returned status %d for item %d", resp.StatusCode, id)
	}

	var item Item
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, fmt.Errorf("failed to decode item %d: %w", id, err)
	}

	return &item, nil
}
