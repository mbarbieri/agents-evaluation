package summarizer

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStripMarkdownCodeFence(t *testing.T) {
	t.Parallel()
	in := "```json\n{\"summary\":\"hi\",\"tags\":[\"go\"]}\n```"
	out := stripMarkdownCodeFence(in)
	if strings.Contains(out, "```") {
		t.Fatalf("expected stripped")
	}
}

func TestSummarize_ParsesCandidateTextAndJSON(t *testing.T) {
	t.Parallel()
	modelText := "```json\n{\"summary\":\"Hello\",\"tags\":[\"Go\",\"Ai\"]}\n```"
	respJSON := fmt.Sprintf(`{"candidates":[{"content":{"parts":[{"text":%q}]}}]}`, modelText)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(respJSON))
	}))
	t.Cleanup(srv.Close)

	hc := srv.Client()
	c := New("k", "m", hc)
	// Patch endpoint by overriding model to include full URL host in tests.
	c.Model = "m" // no-op

	// Override by temporarily using a client with modified transport via URL rewrite.
	hc.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		req.URL.Scheme = "http"
		req.URL.Host = strings.TrimPrefix(srv.URL, "http://")
		return http.DefaultTransport.RoundTrip(req)
	})

	res, err := c.Summarize(context.Background(), "content")
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if res.Summary != "Hello" {
		t.Fatalf("unexpected summary: %q", res.Summary)
	}
	if len(res.Tags) != 2 || res.Tags[0] != "go" || res.Tags[1] != "ai" {
		t.Fatalf("unexpected tags: %+v", res.Tags)
	}
}

func TestSummarize_Non200(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte("no"))
	}))
	t.Cleanup(srv.Close)

	hc := srv.Client()
	hc.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		req.URL.Scheme = "http"
		req.URL.Host = strings.TrimPrefix(srv.URL, "http://")
		return http.DefaultTransport.RoundTrip(req)
	})

	c := New("k", "m", hc)
	_, err := c.Summarize(context.Background(), "content")
	if err == nil {
		t.Fatalf("expected error")
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
