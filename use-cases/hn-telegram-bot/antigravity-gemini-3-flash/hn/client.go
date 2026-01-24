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

type Client struct {
	baseURL string
	hc      *http.Client
}

func NewClient(baseURL string, hc *http.Client) *Client {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &Client{
		baseURL: baseURL,
		hc:      hc,
	}
}

func (c *Client) GetTopStories() ([]int, error) {
	resp, err := c.hc.Get(fmt.Sprintf("%s/v0/topstories.json", c.baseURL))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var ids []int
	if err := json.NewDecoder(resp.Body).Decode(&ids); err != nil {
		return nil, err
	}
	return ids, nil
}

func (c *Client) GetItem(id int) (*Item, error) {
	resp, err := c.hc.Get(fmt.Sprintf("%s/v0/item/%d.json", c.baseURL, id))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var item Item
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, err
	}
	return &item, nil
}
