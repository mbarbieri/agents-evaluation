package summarizer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"hn-telegram-bot/model"
)

const geminiBaseURL = "https://generativelanguage.googleapis.com/v1beta/models"

// Summarizer generates summaries and tags.
type Summarizer interface {
	Summarize(ctx context.Context, content string) (model.SummaryResult, error)
}

// GeminiSummarizer calls the Gemini API over HTTP.
type GeminiSummarizer struct {
	APIKey     string
	Model      string
	BaseURL    string
	HTTPClient *http.Client
}

// NewGeminiSummarizer constructs a GeminiSummarizer.
func NewGeminiSummarizer(apiKey, model string, httpClient *http.Client) *GeminiSummarizer {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	return &GeminiSummarizer{APIKey: apiKey, Model: model, BaseURL: geminiBaseURL, HTTPClient: httpClient}
}

// Summarize requests a summary and tags from Gemini.
func (g *GeminiSummarizer) Summarize(ctx context.Context, content string) (model.SummaryResult, error) {
	if g.APIKey == "" || g.Model == "" {
		return model.SummaryResult{}, errors.New("gemini api key and model are required")
	}
	prompt := buildPrompt(content)
	body := geminiRequest{Contents: []geminiContent{{Parts: []geminiPart{{Text: prompt}}}}}
	payload, err := json.Marshal(body)
	if err != nil {
		return model.SummaryResult{}, fmt.Errorf("marshal request: %w", err)
	}

	baseURL := g.BaseURL
	if baseURL == "" {
		baseURL = geminiBaseURL
	}
	url := fmt.Sprintf("%s/%s:generateContent?key=%s", baseURL, g.Model, g.APIKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return model.SummaryResult{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.HTTPClient.Do(req)
	if err != nil {
		return model.SummaryResult{}, fmt.Errorf("request gemini: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return model.SummaryResult{}, fmt.Errorf("gemini status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	var parsed geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return model.SummaryResult{}, fmt.Errorf("decode response: %w", err)
	}
	text, err := extractText(parsed)
	if err != nil {
		return model.SummaryResult{}, err
	}
	text = stripCodeFences(text)
	result, err := parseSummaryJSON(text)
	if err != nil {
		return model.SummaryResult{}, err
	}
	for i := range result.Tags {
		result.Tags[i] = strings.ToLower(strings.TrimSpace(result.Tags[i]))
	}
	return result, nil
}

func buildPrompt(content string) string {
	return fmt.Sprintf("Summarize the following article in 1-2 sentences and provide 3-5 lowercase tags. Respond with a JSON object with fields \"summary\" (string) and \"tags\" (array of strings). Article:\n%s", content)
}

type geminiRequest struct {
	Contents []geminiContent `json:"contents"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func extractText(resp geminiResponse) (string, error) {
	if len(resp.Candidates) == 0 {
		return "", errors.New("gemini response missing candidates")
	}
	parts := resp.Candidates[0].Content.Parts
	if len(parts) == 0 {
		return "", errors.New("gemini response missing content parts")
	}
	var b strings.Builder
	for _, part := range parts {
		b.WriteString(part.Text)
	}
	return b.String(), nil
}

func stripCodeFences(input string) string {
	trimmed := strings.TrimSpace(input)
	if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```")
		trimmed = strings.TrimPrefix(trimmed, "json")
		trimmed = strings.TrimSpace(trimmed)
		if idx := strings.LastIndex(trimmed, "```"); idx != -1 {
			trimmed = strings.TrimSpace(trimmed[:idx])
		}
	}
	return trimmed
}

func parseSummaryJSON(input string) (model.SummaryResult, error) {
	var payload struct {
		Summary string   `json:"summary"`
		Tags    []string `json:"tags"`
	}
	if err := json.Unmarshal([]byte(input), &payload); err != nil {
		return model.SummaryResult{}, fmt.Errorf("parse summary json: %w", err)
	}
	if payload.Summary == "" {
		return model.SummaryResult{}, errors.New("summary missing")
	}
	if len(payload.Tags) == 0 {
		return model.SummaryResult{}, errors.New("tags missing")
	}
	return model.SummaryResult{Summary: payload.Summary, Tags: payload.Tags}, nil
}
