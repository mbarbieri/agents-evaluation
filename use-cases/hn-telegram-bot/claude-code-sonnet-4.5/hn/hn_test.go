package hn

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetTopStories_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0/topstories.json" {
			t.Errorf("Expected path /v0/topstories.json, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode([]int{1, 2, 3, 4, 5})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ids, err := client.GetTopStories()
	if err != nil {
		t.Fatalf("GetTopStories() error = %v", err)
	}

	if len(ids) != 5 {
		t.Errorf("GetTopStories() returned %d items, want 5", len(ids))
	}

	if ids[0] != 1 || ids[4] != 5 {
		t.Errorf("GetTopStories() = %v, want [1 2 3 4 5]", ids)
	}
}

func TestGetTopStories_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.GetTopStories()
	if err == nil {
		t.Error("GetTopStories() expected error for HTTP 500, got nil")
	}
}

func TestGetTopStories_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.GetTopStories()
	if err == nil {
		t.Error("GetTopStories() expected error for invalid JSON, got nil")
	}
}

func TestGetItem_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0/item/12345.json" {
			t.Errorf("Expected path /v0/item/12345.json, got %s", r.URL.Path)
		}

		item := Item{
			ID:          12345,
			Title:       "Test Article",
			URL:         "https://example.com",
			Score:       100,
			Descendants: 50,
			By:          "testuser",
			Time:        1234567890,
			Type:        "story",
		}
		json.NewEncoder(w).Encode(item)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	item, err := client.GetItem(12345)
	if err != nil {
		t.Fatalf("GetItem() error = %v", err)
	}

	if item.ID != 12345 {
		t.Errorf("Item.ID = %v, want 12345", item.ID)
	}
	if item.Title != "Test Article" {
		t.Errorf("Item.Title = %v, want Test Article", item.Title)
	}
	if item.Score != 100 {
		t.Errorf("Item.Score = %v, want 100", item.Score)
	}
	if item.URL != "https://example.com" {
		t.Errorf("Item.URL = %v, want https://example.com", item.URL)
	}
}

func TestGetItem_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.GetItem(99999)
	if err == nil {
		t.Error("GetItem() expected error for HTTP 404, got nil")
	}
}

func TestGetItem_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.GetItem(12345)
	if err == nil {
		t.Error("GetItem() expected error for invalid JSON, got nil")
	}
}

func TestNewClient_DefaultBaseURL(t *testing.T) {
	client := NewClient("")
	if client.baseURL != "https://hacker-news.firebaseio.com" {
		t.Errorf("Default baseURL = %v, want https://hacker-news.firebaseio.com", client.baseURL)
	}
}
