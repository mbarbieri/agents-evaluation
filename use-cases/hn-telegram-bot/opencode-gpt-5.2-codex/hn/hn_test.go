package hn

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTopStories(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0/topstories.json" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode([]int64{1, 2, 3})
	}))
	defer server.Close()

	client := NewClient(server.URL, &http.Client{Timeout: time.Second})
	ids, err := client.TopStories(context.Background())
	if err != nil {
		t.Fatalf("TopStories: %v", err)
	}
	if len(ids) != 3 || ids[0] != 1 {
		t.Fatalf("unexpected ids %v", ids)
	}
}

func TestItem(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0/item/42.json" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(Item{ID: 42, Title: "Test", URL: "https://example.com", Score: 10, Descendants: 5})
	}))
	defer server.Close()

	client := NewClient(server.URL, &http.Client{Timeout: time.Second})
	item, err := client.Item(context.Background(), 42)
	if err != nil {
		t.Fatalf("Item: %v", err)
	}
	if item.ID != 42 || item.Title != "Test" {
		t.Fatalf("unexpected item %+v", item)
	}
}

func TestTopStoriesErrorOnBadStatus(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, &http.Client{Timeout: time.Second})
	_, err := client.TopStories(context.Background())
	if err == nil {
		t.Fatalf("expected error")
	}
}
