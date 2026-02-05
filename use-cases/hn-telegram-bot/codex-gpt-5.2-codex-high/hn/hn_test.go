package hn

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTopStories(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0/topstories.json" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode([]int64{1, 2, 3})
	}))
	defer server.Close()

	client := NewClientWithBaseURL(server.Client(), server.URL)
	ids, err := client.TopStories(context.Background())
	if err != nil {
		t.Fatalf("TopStories: %v", err)
	}
	if len(ids) != 3 || ids[0] != 1 || ids[2] != 3 {
		t.Fatalf("unexpected ids: %v", ids)
	}
}

func TestItem(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0/item/42.json" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		item := Item{ID: 42, Title: "Hello", URL: "https://example.com", Score: 99, Descendants: 5, By: "alice", Time: 123, Type: "story"}
		_ = json.NewEncoder(w).Encode(item)
	}))
	defer server.Close()

	client := NewClientWithBaseURL(server.Client(), server.URL)
	item, err := client.Item(context.Background(), 42)
	if err != nil {
		t.Fatalf("Item: %v", err)
	}
	if item.ID != 42 || item.Title != "Hello" || item.Score != 99 {
		t.Fatalf("unexpected item: %+v", item)
	}
}
