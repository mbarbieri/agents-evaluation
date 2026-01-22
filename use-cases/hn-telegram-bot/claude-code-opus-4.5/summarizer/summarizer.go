package summarizer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	defaultModel   = "gemini-2.0-flash-lite"
	defaultBaseURL = "https://generativelanguage.googleapis.com"
)

// Result contains the summarization output.
type Result struct {
	Summary string   `json:"summary"`
	Tags    []string `json:"tags"`
}

// Summarizer generates article summaries using the Gemini API.
type Summarizer struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

// Option configures a Summarizer.
type Option func(*Summarizer)

// WithModel sets the Gemini model to use.
func WithModel(model string) Option {
	return func(s *Summarizer) {
		s.model = model
	}
}

// WithBaseURL sets a custom base URL (for testing).
func WithBaseURL(url string) Option {
	return func(s *Summarizer) {
		s.baseURL = url
	}
}

// NewSummarizer creates a new Gemini-based summarizer.
func NewSummarizer(apiKey string, opts ...Option) *Summarizer {
	s := &Summarizer{
		apiKey:     apiKey,
		model:      defaultModel,
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Summarize generates a summary and tags for the given content.
func (s *Summarizer) Summarize(ctx context.Context, title, content string) (*Result, error) {
	prompt := buildPrompt(title, content)

	reqBody := geminiRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{{Text: prompt}},
			},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", s.baseURL, s.model, s.apiKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var geminiResp geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return parseGeminiResponse(&geminiResp)
}

func buildPrompt(title, content string) string {
	return fmt.Sprintf(`Summarize the following article in 1-2 sentences and provide 3-5 lowercase tags categorizing the topic.

Title: %s

Content:
%s

Respond with JSON only, in this exact format:
{"summary": "Your 1-2 sentence summary here", "tags": ["tag1", "tag2", "tag3"]}`, title, content)
}

func parseGeminiResponse(resp *geminiResponse) (*Result, error) {
	if len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates in response")
	}

	candidate := resp.Candidates[0]
	if len(candidate.Content.Parts) == 0 {
		return nil, fmt.Errorf("no parts in candidate")
	}

	text := candidate.Content.Parts[0].Text
	text = stripMarkdownCodeBlock(text)

	var result Result
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, fmt.Errorf("parse summary JSON: %w", err)
	}

	return &result, nil
}

var codeBlockRegex = regexp.MustCompile("(?s)^\\s*```(?:json)?\\s*(.+?)\\s*```\\s*$")

func stripMarkdownCodeBlock(s string) string {
	s = strings.TrimSpace(s)
	if matches := codeBlockRegex.FindStringSubmatch(s); len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return s
}

// Gemini API types

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
	Candidates []geminiCandidate `json:"candidates"`
}

type geminiCandidate struct {
	Content geminiContent `json:"content"`
}
