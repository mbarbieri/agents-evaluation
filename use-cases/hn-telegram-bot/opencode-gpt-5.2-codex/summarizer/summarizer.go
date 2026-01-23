package summarizer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type Client struct {
	baseURL string
	model   string
	apiKey  string
	http    *http.Client
}

type Result struct {
	Summary string
	Tags    []string
}

func NewClient(baseURL, model, apiKey string, httpClient *http.Client) *Client {
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com"
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{baseURL: baseURL, model: model, apiKey: apiKey, http: httpClient}
}

func (c *Client) Summarize(ctx context.Context, content string) (Result, error) {
	if c == nil || c.http == nil {
		return Result{}, errors.New("client not initialized")
	}
	if c.model == "" || c.apiKey == "" {
		return Result{}, errors.New("model and api key required")
	}
	if content == "" {
		return Result{}, errors.New("content required")
	}
	endpoint := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", c.baseURL, c.model, c.apiKey)

	request := map[string]any{
		"contents": []map[string]any{
			{
				"parts": []map[string]string{
					{"text": prompt(content)},
				},
			},
		},
	}
	body, err := json.Marshal(request)
	if err != nil {
		return Result{}, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return Result{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var raw response
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return Result{}, fmt.Errorf("decode response: %w", err)
	}
	text, err := raw.firstText()
	if err != nil {
		return Result{}, err
	}
	text = stripCodeBlock(text)
	var parsed Result
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		return Result{}, fmt.Errorf("parse summary json: %w", err)
	}
	if parsed.Summary == "" {
		return Result{}, errors.New("summary missing")
	}
	return parsed, nil
}

type response struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func (r response) firstText() (string, error) {
	if len(r.Candidates) == 0 {
		return "", errors.New("no candidates")
	}
	parts := r.Candidates[0].Content.Parts
	if len(parts) == 0 {
		return "", errors.New("no parts")
	}
	if parts[0].Text == "" {
		return "", errors.New("empty text")
	}
	return parts[0].Text, nil
}

func prompt(content string) string {
	return "Summarize the article in 1-2 sentences and provide 3-5 lowercase tags categorizing the topic. Respond with JSON containing fields 'summary' and 'tags'. Article content:\n" + content
}

func stripCodeBlock(text string) string {
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```")
		trimmed = strings.TrimSpace(trimmed)
		if strings.HasPrefix(trimmed, "json") {
			trimmed = strings.TrimPrefix(trimmed, "json")
			trimmed = strings.TrimSpace(trimmed)
		}
		trimmed = strings.TrimSuffix(trimmed, "```")
		trimmed = strings.TrimSpace(trimmed)
	}
	return trimmed
}
