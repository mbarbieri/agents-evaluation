package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestExtract_TruncatesToMaxChars(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("a", 5000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><head><title>T</title></head><body><article>" + long + "</article></body></html>"))
	}))
	t.Cleanup(srv.Close)

	s := New(srv.Client(), 2*time.Second)
	s.MaxChars = 4000
	text, err := s.Extract(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(text) != 4000 {
		t.Fatalf("expected 4000 chars, got %d", len(text))
	}
}

func TestExtract_Non200(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte("no"))
	}))
	t.Cleanup(srv.Close)

	s := New(srv.Client(), time.Second)
	_, err := s.Extract(context.Background(), srv.URL)
	if err == nil {
		t.Fatalf("expected error")
	}
}
