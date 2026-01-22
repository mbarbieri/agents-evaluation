package hn

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient_GetTopStories(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0/topstories.json" {
			t.Errorf("Expected path /v0/topstories.json, got %s", r.URL.Path)
		}
		storyIDs := []int{1, 2, 3, 4, 5}
		json.NewEncoder(w).Encode(storyIDs)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ids, err := client.GetTopStories()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(ids) != 5 {
		t.Errorf("Expected 5 story IDs, got %d", len(ids))
	}
	if ids[0] != 1 || ids[4] != 5 {
		t.Errorf("Expected IDs [1, 2, 3, 4, 5], got %v", ids)
	}
}

func TestClient_GetTopStories_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.GetTopStories()
	if err == nil {
		t.Error("Expected error for HTTP error")
	}
}

func TestClient_GetItem(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0/item/12345.json" {
			t.Errorf("Expected path /v0/item/12345.json, got %s", r.URL.Path)
		}
		item := Item{
			ID:          12345,
			Title:       "Test Story",
			URL:         "https://example.com/test",
			Score:       100,
			Descendants: 50,
			Author:      "testuser",
			Time:        time.Now().Unix(),
			Type:        "story",
		}
		json.NewEncoder(w).Encode(item)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	item, err := client.GetItem(12345)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if item.ID != 12345 {
		t.Errorf("Expected item ID 12345, got %d", item.ID)
	}
	if item.Title != "Test Story" {
		t.Errorf("Expected title 'Test Story', got '%s'", item.Title)
	}
	if item.URL != "https://example.com/test" {
		t.Errorf("Expected URL 'https://example.com/test', got '%s'", item.URL)
	}
	if item.Score != 100 {
		t.Errorf("Expected score 100, got %d", item.Score)
	}
	if item.Descendants != 50 {
		t.Errorf("Expected descendants 50, got %d", item.Descendants)
	}
	if item.Author != "testuser" {
		t.Errorf("Expected author 'testuser', got '%s'", item.Author)
	}
	if item.Type != "story" {
		t.Errorf("Expected type 'story', got '%s'", item.Type)
	}
}

func TestClient_GetItem_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.GetItem(99999)
	if err == nil {
		t.Error("Expected error for not found item")
	}
}

func TestClient_GetItem_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.GetItem(12345)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestClient_GetItem_Job(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		item := Item{
			ID:     12345,
			Title:  "Hiring Test Engineer",
			Score:  50,
			Time:   time.Now().Unix(),
			Type:   "job",
			Author: "employer",
		}
		json.NewEncoder(w).Encode(item)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	item, err := client.GetItem(12345)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if item.Type != "job" {
		t.Errorf("Expected type 'job', got '%s'", item.Type)
	}
}

func TestClient_GetTopStories_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]int{})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ids, err := client.GetTopStories()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(ids) != 0 {
		t.Errorf("Expected 0 story IDs, got %d", len(ids))
	}
}
