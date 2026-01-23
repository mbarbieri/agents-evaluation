package hn

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

const defaultBaseURL = "https://hacker-news.firebaseio.com"

type Client struct {
	baseURL string
	http    *http.Client
}

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

func NewClient(baseURL string, httpClient *http.Client) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{baseURL: baseURL, http: httpClient}
}

func (c *Client) TopStories(ctx context.Context) ([]int64, error) {
	if c == nil || c.http == nil {
		return nil, errors.New("client not initialized")
	}
	url := fmt.Sprintf("%s/v0/topstories.json", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	var ids []int64
	if err := json.NewDecoder(resp.Body).Decode(&ids); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return ids, nil
}

func (c *Client) Item(ctx context.Context, id int64) (Item, error) {
	if c == nil || c.http == nil {
		return Item{}, errors.New("client not initialized")
	}
	if id == 0 {
		return Item{}, errors.New("item id required")
	}
	url := fmt.Sprintf("%s/v0/item/%d.json", c.baseURL, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Item{}, fmt.Errorf("create request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return Item{}, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Item{}, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	var item Item
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return Item{}, fmt.Errorf("decode response: %w", err)
	}
	return item, nil
}
