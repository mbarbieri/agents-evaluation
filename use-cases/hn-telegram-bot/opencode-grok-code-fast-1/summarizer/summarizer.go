package summarizer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// Summary contains the summarized article and tags
type Summary struct {
	Summary string   `json:"summary"`
	Tags    []string `json:"tags"`
}

// Summarizer handles Gemini API interactions
type Summarizer struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// New creates a new summarizer
func New(apiKey, model, baseURL string, timeout time.Duration) *Summarizer {
	return &Summarizer{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		client:  &http.Client{Timeout: timeout},
	}
}

// Summarize generates a summary and tags for the given content
func (s *Summarizer) Summarize(content string) (Summary, error) {
	prompt := fmt.Sprintf(`Summarize the following article in 1-2 sentences and provide 3-5 lowercase tags categorizing the topic. Respond with JSON in the format {"summary": "...", "tags": ["tag1", "tag2"]}.

%s`, content)

	requestBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{"text": prompt},
				},
			},
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return Summary{}, err
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", s.baseURL, s.model, s.apiKey)
	resp, err := s.client.Post(url, "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		return Summary{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Summary{}, fmt.Errorf("Gemini API returned status %d", resp.StatusCode)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return Summary{}, err
	}

	candidates, ok := response["candidates"].([]interface{})
	if !ok || len(candidates) == 0 {
		return Summary{}, fmt.Errorf("no candidates in response")
	}

	candidate, ok := candidates[0].(map[string]interface{})
	if !ok {
		return Summary{}, fmt.Errorf("invalid candidate format")
	}

	contentMap, ok := candidate["content"].(map[string]interface{})
	if !ok {
		return Summary{}, fmt.Errorf("invalid content format")
	}

	parts, ok := contentMap["parts"].([]interface{})
	if !ok || len(parts) == 0 {
		return Summary{}, fmt.Errorf("no parts in content")
	}

	part, ok := parts[0].(map[string]interface{})
	if !ok {
		return Summary{}, fmt.Errorf("invalid part format")
	}

	text, ok := part["text"].(string)
	if !ok {
		return Summary{}, fmt.Errorf("no text in part")
	}

	// Strip markdown code blocks
	text = strings.TrimSpace(text)
	re := regexp.MustCompile("```json\\n(.*)\\n```")
	matches := re.FindStringSubmatch(text)
	if len(matches) > 1 {
		text = matches[1]
	}

	var summary Summary
	if err := json.Unmarshal([]byte(text), &summary); err != nil {
		return Summary{}, fmt.Errorf("failed to parse summary JSON: %w", err)
	}

	return summary, nil
}
