package summarizer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type SummaryResult struct {
	Summary string   `json:"summary"`
	Tags    []string `json:"tags"`
}

type Summarizer interface {
	Summarize(title string, content string) (*SummaryResult, error)
}

type GeminiSummarizer struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

func New(apiKey string, model string, baseURL string) *GeminiSummarizer {
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com"
	}
	return &GeminiSummarizer{
		apiKey:     apiKey,
		model:      model,
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

func (s *GeminiSummarizer) Summarize(title string, content string) (*SummaryResult, error) {
	prompt := fmt.Sprintf(`Summarize the following article in 1-2 sentences and provide 3-5 lowercase tags categorizing the topic.

Title: %s

Content: %s

Respond with JSON in this exact format:
{"summary": "your summary here", "tags": ["tag1", "tag2", "tag3"]}`, title, content)

	requestBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{"text": prompt},
				},
			},
		},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", s.baseURL, s.model, s.apiKey)
	resp, err := s.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to call Gemini API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Gemini API returned status %d", resp.StatusCode)
	}

	var apiResponse struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %w", err)
	}

	if len(apiResponse.Candidates) == 0 || len(apiResponse.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response from Gemini API")
	}

	responseText := apiResponse.Candidates[0].Content.Parts[0].Text
	responseText = stripMarkdownCodeBlock(responseText)

	var result SummaryResult
	if err := json.Unmarshal([]byte(responseText), &result); err != nil {
		return nil, fmt.Errorf("failed to parse summary JSON: %w", err)
	}

	return &result, nil
}

func stripMarkdownCodeBlock(text string) string {
	text = strings.TrimSpace(text)

	// Remove ```json or ``` prefix
	if strings.HasPrefix(text, "```json") {
		text = strings.TrimPrefix(text, "```json")
	} else if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```")
	}

	// Remove ``` suffix
	if strings.HasSuffix(text, "```") {
		text = strings.TrimSuffix(text, "```")
	}

	return strings.TrimSpace(text)
}
