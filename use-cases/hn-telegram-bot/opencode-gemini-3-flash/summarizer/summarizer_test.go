package summarizer

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSummarizer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1beta/models/gemini-2.0-flash-lite:generateContent", r.URL.Path)
		assert.Equal(t, "test_key", r.URL.Query().Get("key"))

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"candidates": [
				{
					"content": {
						"parts": [
							{
								"text": "{\"summary\": \"This is a summary.\", \"tags\": [\"go\", \"test\"]}"
							}
						]
					}
				}
			]
		}`)
	}))
	defer server.Close()

	s := NewSummarizer("test_key", "gemini-2.0-flash-lite", server.URL)
	summary, err := s.Summarize(context.Background(), "Title", "Content")
	require.NoError(t, err)

	assert.Equal(t, "This is a summary.", summary.Summary)
	assert.Equal(t, []string{"go", "test"}, summary.Tags)
}

func TestStripMarkdown(t *testing.T) {
	input := "```json\n{\"summary\": \"test\"}\n```"
	expected := "{\"summary\": \"test\"}"
	assert.Equal(t, expected, stripMarkdown(input))

	input2 := "just some text"
	assert.Equal(t, input2, stripMarkdown(input2))
}
