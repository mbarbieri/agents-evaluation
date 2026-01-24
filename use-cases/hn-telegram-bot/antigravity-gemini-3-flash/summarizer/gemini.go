package summarizer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Summarizer struct {
	apiKey  string
	model   string
	baseURL string
}

func NewSummarizer(apiKey, model, baseURL string) *Summarizer {
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com"
	}
	return &Summarizer{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
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

func (s *Summarizer) Summarize(title, content string) (string, []string, error) {
	prompt := fmt.Sprintf(`Summarize the following Hacker News article in 1-2 sentences and provide 3-5 lowercase tags categorizing the topic.
Return the response in JSON format with "summary" and "tags" fields.

Title: %s
Content: %s`, title, content)

	reqBody := geminiRequest{}
	reqBody.Contents = append(reqBody.Contents, struct {
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	}{
		Parts: []struct {
			Text string `json:"text"`
		}{
			{Text: prompt},
		},
	})

	jsonData, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", s.baseURL, s.model, s.apiKey)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", nil, fmt.Errorf("gemini api error (status %d): %s", resp.StatusCode, string(body))
	}

	var gResp geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&gResp); err != nil {
		return "", nil, err
	}

	if len(gResp.Candidates) == 0 || len(gResp.Candidates[0].Content.Parts) == 0 {
		return "", nil, fmt.Errorf("no content in gemini response")
	}

	rawText := gResp.Candidates[0].Content.Parts[0].Text
	jsonText := stripMarkdown(rawText)

	var result struct {
		Summary string   `json:"summary"`
		Tags    []string `json:"tags"`
	}
	if err := json.Unmarshal([]byte(jsonText), &result); err != nil {
		return "", nil, fmt.Errorf("failed to parse gemini JSON: %w", err)
	}

	return result.Summary, result.Tags, nil
}

func stripMarkdown(s string) string {
	s = strings.TrimSpace(s)

	// If it contains ```, we need to extract what's inside
	if strings.Contains(s, "```") {
		// Try to find the first and last ```
		first := strings.Index(s, "```")
		last := strings.LastIndex(s, "```")
		if first != -1 && last != -1 && first != last {
			inner := s[first+3 : last]
			// Trim "json" if it's there
			inner = strings.TrimSpace(inner)
			if strings.HasPrefix(inner, "json") {
				inner = strings.TrimPrefix(inner, "json")
			}
			return strings.TrimSpace(inner)
		}
	}

	return s
}
