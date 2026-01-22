package hn

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Item struct {
	ID          int
	Title       string
	URL         string
	Score       int
	Descendants int
	By          string
	Time        int64
	Type        string
}

type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{},
	}
}

func (c *Client) GetTopStories() ([]int, error) {
	url := fmt.Sprintf("%s/v0/topstories.json", c.baseURL)
	resp, err := c.http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get top stories: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var storyIDs []int
	if err := json.Unmarshal(body, &storyIDs); err != nil {
		return nil, fmt.Errorf("failed to parse story IDs: %w", err)
	}

	return storyIDs, nil
}

func (c *Client) GetItem(id int) (Item, error) {
	url := fmt.Sprintf("%s/v0/item/%d.json", c.baseURL, id)
	resp, err := c.http.Get(url)
	if err != nil {
		return Item{}, fmt.Errorf("failed to get item: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Item{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Item{}, fmt.Errorf("failed to read response body: %w", err)
	}

	var item Item
	if err := json.Unmarshal(body, &item); err != nil {
		return Item{}, fmt.Errorf("failed to parse item: %w", err)
	}

	return item, nil
}
