package summarizer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

const baseURL = "https://generativelanguage.googleapis.com/v1beta/models"

// Result holds the AI-generated summary and tags for an article.
type Result struct {
	Summary string   `json:"summary"`
	Tags    []string `json:"tags"`
}

// Summarizer generates summaries and tags for article content.
type Summarizer interface {
	Summarize(ctx context.Context, title, content string) (*Result, error)
}

type geminiSummarizer struct {
	apiKey  string
	model   string
	client  *http.Client
	baseURL string
}

// NewSummarizer creates a Summarizer backed by the Gemini API.
func NewSummarizer(apiKey, model string, client *http.Client) Summarizer {
	return &geminiSummarizer{
		apiKey:  apiKey,
		model:   model,
		client:  client,
		baseURL: baseURL,
	}
}

// newSummarizerWithURL creates a Summarizer with a custom base URL for testing.
func newSummarizerWithURL(apiKey, model string, client *http.Client, url string) Summarizer {
	return &geminiSummarizer{
		apiKey:  apiKey,
		model:   model,
		client:  client,
		baseURL: url,
	}
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

func (g *geminiSummarizer) Summarize(ctx context.Context, title, content string) (*Result, error) {
	prompt := fmt.Sprintf(
		`Summarize this article in 1-2 sentences and provide 3-5 lowercase tags categorizing the topic.
Return a JSON object with "summary" and "tags" fields only. No markdown formatting.

Title: %s

Content: %s`, title, content)

	reqBody := geminiRequest{
		Contents: []geminiContent{
			{Parts: []geminiPart{{Text: prompt}}},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	url := fmt.Sprintf("%s/%s:generateContent?key=%s", g.baseURL, g.model, g.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling Gemini API: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Gemini API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var gemResp geminiResponse
	if err := json.Unmarshal(respBody, &gemResp); err != nil {
		return nil, fmt.Errorf("parsing Gemini response: %w", err)
	}

	if len(gemResp.Candidates) == 0 || len(gemResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response from Gemini API")
	}

	text := gemResp.Candidates[0].Content.Parts[0].Text
	text = stripMarkdownCodeBlock(text)

	var result Result
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		slog.Warn("failed to parse Gemini JSON response", "error", err, "text", text)
		return nil, fmt.Errorf("parsing summary JSON: %w", err)
	}

	return &result, nil
}

// stripMarkdownCodeBlock removes markdown code block wrappers from text.
// Gemini may wrap JSON responses in ```json ... ``` blocks.
func stripMarkdownCodeBlock(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		// Remove opening fence (possibly with language tag)
		if idx := strings.Index(s, "\n"); idx != -1 {
			s = s[idx+1:]
		}
		// Remove closing fence
		if idx := strings.LastIndex(s, "```"); idx != -1 {
			s = s[:idx]
		}
		s = strings.TrimSpace(s)
	}
	return s
}
