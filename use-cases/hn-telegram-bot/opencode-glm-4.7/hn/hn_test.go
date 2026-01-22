package hn

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetTopStories(t *testing.T) {
	stories := []int{123, 456, 789}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0/topstories.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stories)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ids, err := client.GetTopStories()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ids) != 3 {
		t.Errorf("got %d stories, want 3", len(ids))
	}

	if ids[0] != 123 || ids[1] != 456 || ids[2] != 789 {
		t.Errorf("unexpected story IDs: %v", ids)
	}
}

func TestGetTopStoriesError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.GetTopStories()
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestGetTopStoriesInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.GetTopStories()
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestGetItem(t *testing.T) {
	itemData := map[string]any{
		"id":          123,
		"title":       "Test Article",
		"url":         "https://example.com",
		"score":       100,
		"descendants": 50,
		"by":          "author",
		"time":        1234567890,
		"type":        "story",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0/item/123.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(itemData)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	item, err := client.GetItem(123)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if item.ID != 123 {
		t.Errorf("ID = %v, want 123", item.ID)
	}
	if item.Title != "Test Article" {
		t.Errorf("Title = %v, want Test Article", item.Title)
	}
	if item.URL != "https://example.com" {
		t.Errorf("URL = %v, want https://example.com", item.URL)
	}
	if item.Score != 100 {
		t.Errorf("Score = %v, want 100", item.Score)
	}
	if item.Descendants != 50 {
		t.Errorf("Descendants = %v, want 50", item.Descendants)
	}
	if item.By != "author" {
		t.Errorf("By = %v, want author", item.By)
	}
	if item.Time != 1234567890 {
		t.Errorf("Time = %v, want 1234567890", item.Time)
	}
	if item.Type != "story" {
		t.Errorf("Type = %v, want story", item.Type)
	}
}

func TestGetItemError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.GetItem(123)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestGetItemInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.GetItem(123)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestGetItemStoryWithNoURL(t *testing.T) {
	itemData := map[string]any{
		"id":          123,
		"title":       "Ask HN",
		"score":       50,
		"descendants": 10,
		"by":          "author",
		"time":        1234567890,
		"type":        "story",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(itemData)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	item, err := client.GetItem(123)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if item.URL != "" {
		t.Errorf("URL = %v, want empty string", item.URL)
	}
}
