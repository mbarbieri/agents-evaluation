package hn

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

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

func NewClient(httpClient *http.Client) Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return Client{
		BaseURL:    "https://hacker-news.firebaseio.com",
		HTTPClient: httpClient,
	}
}

func (c Client) TopStories(ctx context.Context) ([]int, error) {
	u, err := url.JoinPath(c.BaseURL, "/v0/topstories.json")
	if err != nil {
		return nil, fmt.Errorf("build url: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request topstories: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("topstories status %d: %s", resp.StatusCode, string(b))
	}
	var ids []int
	if err := json.NewDecoder(resp.Body).Decode(&ids); err != nil {
		return nil, fmt.Errorf("decode topstories: %w", err)
	}
	return ids, nil
}

func (c Client) Item(ctx context.Context, id int) (Item, error) {
	u, err := url.JoinPath(c.BaseURL, "/v0/item", strconv.Itoa(id)+".json")
	if err != nil {
		return Item{}, fmt.Errorf("build url: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return Item{}, fmt.Errorf("new request: %w", err)
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return Item{}, fmt.Errorf("request item: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return Item{}, fmt.Errorf("item status %d: %s", resp.StatusCode, string(b))
	}
	var it Item
	if err := json.NewDecoder(resp.Body).Decode(&it); err != nil {
		return Item{}, fmt.Errorf("decode item: %w", err)
	}
	return it, nil
}
