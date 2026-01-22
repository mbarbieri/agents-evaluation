package summarizer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const defaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"

type Client struct {
	ApiKey     string
	Model      string
	BaseURL    string
	httpClient *http.Client
}

func New(apiKey, model string) *Client {
	return &Client{
		ApiKey:  apiKey,
		Model:   model,
		BaseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type generateRequest struct {
	Contents []contentPart `json:"contents"`
}

type contentPart struct {
	Parts []textPart `json:"parts"`
}

type textPart struct {
	Text string `json:"text"`
}

type generateResponse struct {
	Candidates []candidate `json:"candidates"`
}

type candidate struct {
	Content contentPart `json:"content"`
}

type summaryResult struct {
	Summary string   `json:"summary"`
	Tags    []string `json:"tags"`
}

func (c *Client) Summarize(text string) (string, []string, error) {
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", c.BaseURL, c.Model, c.ApiKey)

	prompt := fmt.Sprintf(`Summarize the following article in 1-2 sentences. Also provide 3-5 lowercase tags categorizing the topic. Return ONLY valid JSON with keys "summary" and "tags". Do not include any other text. Content: %s`, text)

	reqBody := generateRequest{
		Contents: []contentPart{
			{
				Parts: []textPart{
					{Text: prompt},
				},
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", nil, err
	}

	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", nil, fmt.Errorf("gemini api call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("gemini api returned status: %d", resp.StatusCode)
	}

	var geminiResp generateResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return "", nil, fmt.Errorf("failed to decode gemini response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", nil, fmt.Errorf("empty response from gemini")
	}

	rawText := geminiResp.Candidates[0].Content.Parts[0].Text

	// Clean markdown code blocks
	rawText = strings.TrimSpace(rawText)
	rawText = strings.TrimPrefix(rawText, "```json")
	rawText = strings.TrimPrefix(rawText, "```")
	rawText = strings.TrimSuffix(rawText, "```")
	rawText = strings.TrimSpace(rawText)

	var result summaryResult
	if err := json.Unmarshal([]byte(rawText), &result); err != nil {
		return "", nil, fmt.Errorf("failed to parse json summary: %w, text: %s", err, rawText)
	}

	return result.Summary, result.Tags, nil
}
