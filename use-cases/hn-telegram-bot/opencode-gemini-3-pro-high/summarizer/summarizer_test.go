package summarizer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSummarize(t *testing.T) {
	mockResponse := `{
	  "candidates": [
	    {
	      "content": {
	        "parts": [
	          {
	            "text": "\u0060\u0060\u0060json\n{\"summary\": \"This is a test summary.\", \"tags\": [\"test\", \"go\"]}\n\u0060\u0060\u0060"
	          }
	        ]
	      }
	    }
	  ]
	}`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "generateContent") {
			t.Errorf("Unexpected URL path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("key") != "fake_key" {
			t.Errorf("Missing or wrong API key")
		}

		// Validate request body
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)
		// checks omitted for brevity but could be added

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockResponse)
	}))
	defer ts.Close()

	s := New("fake_key", "gemini-pro")
	s.BaseURL = ts.URL // Inject mock URL if possible, or we need to design for it.
	// Since standard lib HTTP, I can probably override the base URL pattern or just construct full URL in method.
	// I'll make BaseURL configurable in struct.

	summary, tags, err := s.Summarize("Article content here")
	if err != nil {
		t.Fatalf("Summarize failed: %v", err)
	}

	if summary != "This is a test summary." {
		t.Errorf("Expected summary mismatch, got: %s", summary)
	}
	if len(tags) != 2 || tags[0] != "test" {
		t.Errorf("Tags mismatch, got: %v", tags)
	}
}

func TestSummarizeError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	s := New("k", "m")
	s.BaseURL = ts.URL

	_, _, err := s.Summarize("text")
	if err == nil {
		t.Error("Expected error, got nil")
	}
}
