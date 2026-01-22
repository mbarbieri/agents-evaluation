package summarizer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Summary struct {
	Summary string   `json:"summary"`
	Tags    []string `json:"tags"`
}

type Summarizer struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

func NewSummarizer(apiKey, model, baseURL string) *Summarizer {
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com"
	}
	return &Summarizer{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type geminiRequest struct {
	Contents []struct {
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	} `json:"contents"`
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

func (s *Summarizer) Summarize(ctx context.Context, title, content string) (*Summary, error) {
	prompt := fmt.Sprintf(`Summarize the following Hacker News article in 1-2 sentences and provide 3-5 lowercase tags categorizing the topic.
Respond ONLY with a JSON object containing "summary" and "tags" fields.

Title: %s
Content: %s`, title, content)

	reqBody := geminiRequest{
		Contents: []struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		}{
			{
				Parts: []struct {
					Text string `json:"text"`
				}{
					{Text: prompt},
				},
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", s.baseURL, s.model, s.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini API returned status %d", resp.StatusCode)
	}

	var geminiResp geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return nil, err
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response from Gemini")
	}

	rawText := geminiResp.Candidates[0].Content.Parts[0].Text
	cleanJSON := stripMarkdown(rawText)

	var summary Summary
	if err := json.Unmarshal([]byte(cleanJSON), &summary); err != nil {
		return nil, fmt.Errorf("failed to parse summary JSON: %w (raw: %s)", err, rawText)
	}

	return &summary, nil
}

func stripMarkdown(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		lines := strings.Split(s, "\n")
		if len(lines) >= 3 {
			// Remove first and last lines (the ``` markers)
			return strings.Join(lines[1:len(lines)-1], "\n")
		}
	}
	return s
}
