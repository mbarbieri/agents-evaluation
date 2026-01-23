package hn

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTopStories(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0/topstories.json" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[1,2,3]`))
	}))
	t.Cleanup(srv.Close)

	c := NewClient(srv.Client())
	c.BaseURL = srv.URL
	ids, err := c.TopStories(context.Background())
	if err != nil {
		t.Fatalf("TopStories: %v", err)
	}
	if len(ids) != 3 || ids[0] != 1 || ids[2] != 3 {
		t.Fatalf("unexpected ids: %+v", ids)
	}
}

func TestTopStories_Non200(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte("nope"))
	}))
	t.Cleanup(srv.Close)

	c := NewClient(srv.Client())
	c.BaseURL = srv.URL
	_, err := c.TopStories(context.Background())
	if err == nil || !strings.Contains(err.Error(), "status 500") {
		t.Fatalf("expected status error, got %v", err)
	}
}

func TestItem(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0/item/42.json" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":42,"title":"T","url":"U","score":10,"descendants":5,"by":"a","time":123,"type":"story"}`))
	}))
	t.Cleanup(srv.Close)

	c := NewClient(srv.Client())
	c.BaseURL = srv.URL
	it, err := c.Item(context.Background(), 42)
	if err != nil {
		t.Fatalf("Item: %v", err)
	}
	if it.ID != 42 || it.Score != 10 || it.Descendants != 5 {
		t.Fatalf("unexpected item: %+v", it)
	}
}
