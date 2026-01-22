package hn

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetTopStories(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0/topstories.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode([]int64{1, 2, 3, 4, 5})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL), WithTimeout(5*time.Second))
	ctx := context.Background()

	ids, err := client.GetTopStories(ctx, 3)
	if err != nil {
		t.Fatalf("GetTopStories failed: %v", err)
	}

	if len(ids) != 3 {
		t.Errorf("got %d stories, want 3", len(ids))
	}
	expected := []int64{1, 2, 3}
	for i, id := range expected {
		if ids[i] != id {
			t.Errorf("ids[%d] = %d, want %d", i, ids[i], id)
		}
	}
}

func TestGetTopStoriesLimitExceedsAvailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]int64{1, 2})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	ctx := context.Background()

	ids, err := client.GetTopStories(ctx, 10)
	if err != nil {
		t.Fatalf("GetTopStories failed: %v", err)
	}

	if len(ids) != 2 {
		t.Errorf("got %d stories, want 2", len(ids))
	}
}

func TestGetItem(t *testing.T) {
	item := Item{
		ID:          12345,
		Title:       "Test Article",
		URL:         "https://example.com/article",
		Score:       100,
		Descendants: 50,
		By:          "testuser",
		Time:        1609459200,
		Type:        "story",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0/item/12345.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(item)
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	ctx := context.Background()

	result, err := client.GetItem(ctx, 12345)
	if err != nil {
		t.Fatalf("GetItem failed: %v", err)
	}

	if result.ID != item.ID {
		t.Errorf("ID = %d, want %d", result.ID, item.ID)
	}
	if result.Title != item.Title {
		t.Errorf("Title = %q, want %q", result.Title, item.Title)
	}
	if result.URL != item.URL {
		t.Errorf("URL = %q, want %q", result.URL, item.URL)
	}
	if result.Score != item.Score {
		t.Errorf("Score = %d, want %d", result.Score, item.Score)
	}
	if result.Descendants != item.Descendants {
		t.Errorf("Descendants = %d, want %d", result.Descendants, item.Descendants)
	}
	if result.By != item.By {
		t.Errorf("By = %q, want %q", result.By, item.By)
	}
}

func TestGetItemNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("null"))
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	ctx := context.Background()

	_, err := client.GetItem(ctx, 99999)
	if err == nil {
		t.Fatal("expected error for null response")
	}
}

func TestGetItemServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	ctx := context.Background()

	_, err := client.GetItem(ctx, 12345)
	if err == nil {
		t.Fatal("expected error for server error")
	}
}

func TestContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		json.NewEncoder(w).Encode([]int64{1, 2, 3})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.GetTopStories(ctx, 3)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	ctx := context.Background()

	_, err := client.GetTopStories(ctx, 3)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestDefaultClient(t *testing.T) {
	client := NewClient()
	if client.baseURL != "https://hacker-news.firebaseio.com" {
		t.Errorf("baseURL = %q, want HN API URL", client.baseURL)
	}
}
