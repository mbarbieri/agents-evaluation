package summarizer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	APIKey     string
	Model      string
	HTTPClient *http.Client
}

type Result struct {
	Summary string   `json:"summary"`
	Tags    []string `json:"tags"`
}

func New(apiKey string, model string, httpClient *http.Client) Client {
	if model == "" {
		model = "gemini-2.0-flash-lite"
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return Client{APIKey: apiKey, Model: model, HTTPClient: httpClient}
}

func (c Client) Summarize(ctx context.Context, content string) (Result, error) {
	if c.APIKey == "" {
		return Result{}, errors.New("missing api key")
	}
	endpoint, err := url.Parse("https://generativelanguage.googleapis.com/v1beta/models/" + c.Model + ":generateContent")
	if err != nil {
		return Result{}, fmt.Errorf("parse endpoint: %w", err)
	}
	q := endpoint.Query()
	q.Set("key", c.APIKey)
	endpoint.RawQuery = q.Encode()

	prompt := "Summarize the following article in 1-2 sentences and provide 3-5 lowercase tags. Return ONLY valid JSON with keys \"summary\" and \"tags\".\n\n" + content
	body := map[string]any{
		"contents": []any{
			map[string]any{
				"parts": []any{map[string]any{"text": prompt}},
			},
		},
	}
	b, err := json.Marshal(body)
	if err != nil {
		return Result{}, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(b))
	if err != nil {
		return Result{}, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("request gemini: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		rb, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		return Result{}, fmt.Errorf("gemini status %d: %s", resp.StatusCode, string(rb))
	}

	rb, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Result{}, fmt.Errorf("read gemini response: %w", err)
	}
	text, err := extractFirstCandidateText(rb)
	if err != nil {
		return Result{}, err
	}
	text = stripMarkdownCodeFence(text)

	var res Result
	if err := json.Unmarshal([]byte(text), &res); err != nil {
		return Result{}, fmt.Errorf("parse model json: %w", err)
	}
	res.Summary = strings.TrimSpace(res.Summary)
	if res.Summary == "" || len(res.Tags) == 0 {
		return Result{}, fmt.Errorf("invalid model response")
	}
	for i := range res.Tags {
		res.Tags[i] = strings.TrimSpace(strings.ToLower(res.Tags[i]))
	}
	return res, nil
}

type generateContentResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func extractFirstCandidateText(b []byte) (string, error) {
	var resp generateContentResponse
	if err := json.Unmarshal(b, &resp); err != nil {
		return "", fmt.Errorf("parse gemini response: %w", err)
	}
	if len(resp.Candidates) == 0 {
		return "", fmt.Errorf("no candidates")
	}
	parts := resp.Candidates[0].Content.Parts
	if len(parts) == 0 {
		return "", fmt.Errorf("no parts")
	}
	var sb strings.Builder
	for _, p := range parts {
		sb.WriteString(p.Text)
	}
	return strings.TrimSpace(sb.String()), nil
}

func stripMarkdownCodeFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSpace(s)
		if strings.HasPrefix(strings.ToLower(s), "json") {
			s = strings.TrimSpace(s[len("json"):])
		}
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = strings.TrimSpace(s[:idx])
		}
	}
	return s
}
