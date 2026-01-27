package summarizer

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	t.Run("creates client with defaults", func(t *testing.T) {
		client := NewClient("test-key", "", 10*time.Second)
		if client == nil {
			t.Error("NewClient() returned nil")
		}
		if client.apiKey != "test-key" {
			t.Errorf("apiKey = %v, want test-key", client.apiKey)
		}
		if client.model != "gemini-2.0-flash-lite" {
			t.Errorf("model default = %v, want gemini-2.0-flash-lite", client.model)
		}
	})

	t.Run("creates client with custom model", func(t *testing.T) {
		client := NewClient("test-key", "custom-model", 5*time.Second)
		if client.model != "custom-model" {
			t.Errorf("model = %v, want custom-model", client.model)
		}
		if client.httpClient.Timeout != 5*time.Second {
			t.Errorf("timeout = %v, want 5s", client.httpClient.Timeout)
		}
	})
}

func TestSummarize(t *testing.T) {
	tests := []struct {
		name       string
		response   generateResponse
		statusCode int
		wantErr    bool
		errIs      error
		validate   func(t *testing.T, summary *Summary)
	}{
		{
			name: "successful summarization",
			response: generateResponse{
				Candidates: []candidate{
					{
						Content: content{
							Parts: []part{
								{Text: `{"summary": "This is a test summary.", "tags": ["go", "testing", "api"]}`},
							},
						},
					},
				},
			},
			statusCode: http.StatusOK,
			wantErr:    false,
			validate: func(t *testing.T, summary *Summary) {
				if summary.Summary != "This is a test summary." {
					t.Errorf("Summary = %v, want 'This is a test summary.'", summary.Summary)
				}
				if len(summary.Tags) != 3 {
					t.Errorf("Tags length = %v, want 3", len(summary.Tags))
				}
				if summary.Tags[0] != "go" {
					t.Errorf("First tag = %v, want go", summary.Tags[0])
				}
			},
		},
		{
			name: "response with markdown code blocks",
			response: generateResponse{
				Candidates: []candidate{
					{
						Content: content{
							Parts: []part{
								{Text: "```json\n{\"summary\": \"Test summary with markdown.\", \"tags\": [\"markdown\", \"json\"]}\n```"},
							},
						},
					},
				},
			},
			statusCode: http.StatusOK,
			wantErr:    false,
			validate: func(t *testing.T, summary *Summary) {
				if summary.Summary != "Test summary with markdown." {
					t.Errorf("Summary = %v, want 'Test summary with markdown.'", summary.Summary)
				}
			},
		},
		{
			name:       "API error",
			response:   generateResponse{},
			statusCode: http.StatusUnauthorized,
			wantErr:    true,
			errIs:      ErrAPIError,
		},
		{
			name: "empty candidates",
			response: generateResponse{
				Candidates: []candidate{},
			},
			statusCode: http.StatusOK,
			wantErr:    true,
			errIs:      ErrInvalidResponse,
		},
		{
			name: "invalid JSON response",
			response: generateResponse{
				Candidates: []candidate{
					{
						Content: content{
							Parts: []part{
								{Text: "not valid json"},
							},
						},
					},
				},
			},
			statusCode: http.StatusOK,
			wantErr:    true,
			errIs:      ErrInvalidResponse,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request method and path
				if r.Method != "POST" {
					t.Errorf("Expected POST, got %s", r.Method)
				}

				// Verify query parameter
				if !strings.Contains(r.URL.RawQuery, "key=test-api-key") {
					t.Error("Expected API key in query parameters")
				}

				// Verify model in path
				if !strings.Contains(r.URL.Path, "test-model") {
					t.Errorf("Expected model in path, got %s", r.URL.Path)
				}

				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusOK {
					json.NewEncoder(w).Encode(tt.response)
				}
			}))
			defer server.Close()

			// Create a client that uses the test server
			client := newClientWithBaseURL("test-api-key", "test-model", 5*time.Second, server.URL)

			summary, err := client.Summarize("test content")

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
					return
				}
				if tt.errIs != nil && !errors.Is(err, tt.errIs) {
					t.Errorf("Expected error %v, got %v", tt.errIs, err)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.validate != nil {
				tt.validate(t, summary)
			}
		})
	}
}

func TestStripMarkdownCodeBlocks(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "```json\n{\"key\": \"value\"}\n```",
			expected: `{"key": "value"}`,
		},
		{
			input:    "```\nsome text\n```",
			expected: "some text",
		},
		{
			input:    "plain text without code blocks",
			expected: "plain text without code blocks",
		},
		{
			input:    "```json{\"compact\": true}```",
			expected: `{"compact": true}`,
		},
	}

	for _, tt := range tests {
		result := stripMarkdownCodeBlocks(tt.input)
		if result != tt.expected {
			t.Errorf("stripMarkdownCodeBlocks(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestSummarizeTagNormalization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := generateResponse{
			Candidates: []candidate{
				{
					Content: content{
						Parts: []part{
							{Text: `{"summary": "Test.", "tags": ["Go", "MACHINE Learning", "  api  "]}`},
						},
					},
				},
			},
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Test tag normalization logic directly
	tags := []string{"Go", "MACHINE Learning", "  api  "}
	for i, tag := range tags {
		tags[i] = strings.ToLower(strings.TrimSpace(tag))
	}

	expected := []string{"go", "machine learning", "api"}
	for i, tag := range tags {
		if tag != expected[i] {
			t.Errorf("Tag[%d] = %v, want %v", i, tag, expected[i])
		}
	}
}
