package summarizer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
)

type Summarizer struct {
	apiKey  string
	model   string
	baseURL string
	http    *http.Client
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
	Candidates []geminiCandidate `json:"candidates"`
}

type geminiCandidate struct {
	Content geminiContent `json:"content"`
}

type summaryResponse struct {
	Summary string   `json:"summary"`
	Tags    []string `json:"tags"`
}

func NewSummarizer(apiKey, model, baseURL string) *Summarizer {
	return &Summarizer{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		http:    &http.Client{},
	}
}

func (s *Summarizer) Summarize(content string) (string, []string, error) {
	prompt := buildPrompt(content)

	reqBody := geminiRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{{Text: prompt}},
			},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", s.baseURL, s.model, s.apiKey)
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.http.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return "", nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 {
		return "", nil, fmt.Errorf("no candidates in response")
	}

	responseText := geminiResp.Candidates[0].Content.Parts[0].Text
	responseText = stripMarkdownCodeBlocks(responseText)

	summary, tags, err := parseSummaryResponse(responseText)
	if err != nil {
		return "", nil, fmt.Errorf("failed to parse summary response: %w", err)
	}

	return summary, tags, nil
}

func buildPrompt(content string) string {
	return fmt.Sprintf(`Please summarize the following article in 1-2 sentences and provide 3-5 lowercase tags categorizing the topic.

Article content:
%s

Respond with JSON in this format:
{
  "summary": "1-2 sentence summary",
  "tags": ["tag1", "tag2", "tag3"]
}`, content)
}

func stripMarkdownCodeBlocks(text string) string {
	re := regexp.MustCompile("```(?:json)?\n?(.*?)\n?```")
	return re.ReplaceAllString(text, "$1")
}

func parseSummaryResponse(text string) (string, []string, error) {
	var resp summaryResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		return "", nil, fmt.Errorf("failed to unmarshal summary response: %w", err)
	}

	if resp.Summary == "" {
		return "", nil, fmt.Errorf("summary field is required")
	}

	if len(resp.Tags) == 0 {
		return "", nil, fmt.Errorf("tags field is required")
	}

	return resp.Summary, resp.Tags, nil
}
