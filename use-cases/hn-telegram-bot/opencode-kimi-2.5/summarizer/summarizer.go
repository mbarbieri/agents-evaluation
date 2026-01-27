package summarizer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var (
	ErrAPIError        = errors.New("Gemini API error")
	ErrInvalidResponse = errors.New("invalid response from Gemini")
)

// Summary represents the result from the Gemini API
type Summary struct {
	Summary string   `json:"summary"`
	Tags    []string `json:"tags"`
}

// Client is the Gemini API client
type Client struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

// request structures
type generateRequest struct {
	Contents []content `json:"contents"`
}

type content struct {
	Parts []part `json:"parts"`
}

type part struct {
	Text string `json:"text"`
}

// response structures
type generateResponse struct {
	Candidates []candidate `json:"candidates"`
}

type candidate struct {
	Content content `json:"content"`
}

// NewClient creates a new Gemini API client
func NewClient(apiKey, model string, timeout time.Duration) *Client {
	return newClientWithBaseURL(apiKey, model, timeout, "https://generativelanguage.googleapis.com")
}

// newClientWithBaseURL creates a client with a custom base URL (for testing)
func newClientWithBaseURL(apiKey, model string, timeout time.Duration, baseURL string) *Client {
	if model == "" {
		model = "gemini-2.0-flash-lite"
	}

	return &Client{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Summarize generates a summary and tags for the given article content
func (c *Client) Summarize(articleContent string) (*Summary, error) {
	prompt := fmt.Sprintf(`Summarize the following article in 1-2 sentences and provide 3-5 lowercase tags categorizing the topic.

Respond in JSON format with the following structure:
{
  "summary": "The summary here",
  "tags": ["tag1", "tag2", "tag3"]
}

Article content:
%s`, articleContent)

	reqBody := generateRequest{
		Contents: []content{
			{
				Parts: []part{
					{Text: prompt},
				},
			},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", c.baseURL, c.model, c.apiKey)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAPIError, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: HTTP %d", ErrAPIError, resp.StatusCode)
	}

	var genResp generateResponse
	if err := json.NewDecoder(resp.Body).Decode(&genResp); err != nil {
		return nil, fmt.Errorf("%w: failed to decode response: %v", ErrInvalidResponse, err)
	}

	if len(genResp.Candidates) == 0 {
		return nil, fmt.Errorf("%w: no candidates in response", ErrInvalidResponse)
	}

	if len(genResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("%w: no content parts in response", ErrInvalidResponse)
	}

	text := genResp.Candidates[0].Content.Parts[0].Text

	// Strip markdown code blocks if present
	text = stripMarkdownCodeBlocks(text)

	// Parse the JSON response
	var summary Summary
	if err := json.Unmarshal([]byte(text), &summary); err != nil {
		return nil, fmt.Errorf("%w: failed to parse summary JSON: %v", ErrInvalidResponse, err)
	}

	// Normalize tags to lowercase
	for i, tag := range summary.Tags {
		summary.Tags[i] = strings.ToLower(strings.TrimSpace(tag))
	}

	return &summary, nil
}

// stripMarkdownCodeBlocks removes markdown code block markers from text
func stripMarkdownCodeBlocks(text string) string {
	// Remove ```json and ``` markers
	re := regexp.MustCompile("(?s)^```(?:json)?\\s*\\n?|\\n?```\\s*$")
	return strings.TrimSpace(re.ReplaceAllString(text, ""))
}
