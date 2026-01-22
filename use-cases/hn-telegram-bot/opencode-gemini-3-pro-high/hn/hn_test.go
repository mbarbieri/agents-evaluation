package hn

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetTopStories(t *testing.T) {
	// Mock Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0/topstories.json" {
			t.Errorf("Expected path /v0/topstories.json, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `[1, 2, 3, 4, 5]`)
	}))
	defer ts.Close()

	client := NewClient()
	client.BaseURL = ts.URL // Override for testing

	ids, err := client.GetTopStories()
	if err != nil {
		t.Fatalf("GetTopStories failed: %v", err)
	}

	if len(ids) != 5 {
		t.Errorf("Expected 5 IDs, got %d", len(ids))
	}
	if ids[0] != 1 {
		t.Errorf("Expected first ID 1, got %d", ids[0])
	}
}

func TestGetItem(t *testing.T) {
	// Mock Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0/item/8863.json" {
			t.Errorf("Expected path /v0/item/8863.json, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		// Partial response
		fmt.Fprintln(w, `{
			"by": "dhouston",
			"descendants": 71,
			"id": 8863,
			"score": 111,
			"time": 1175714200,
			"title": "My YC app: Dropbox - Throw away your USB drive",
			"type": "story",
			"url": "http://www.getdropbox.com/u/2/screencast.html"
		}`)
	}))
	defer ts.Close()

	client := NewClient()
	client.BaseURL = ts.URL

	item, err := client.GetItem(8863)
	if err != nil {
		t.Fatalf("GetItem failed: %v", err)
	}

	if item.ID != 8863 {
		t.Errorf("Expected ID 8863, got %d", item.ID)
	}
	if item.Title != "My YC app: Dropbox - Throw away your USB drive" {
		t.Errorf("Unexpected title: %s", item.Title)
	}
	if item.Score != 111 {
		t.Errorf("Expected score 111, got %d", item.Score)
	}
}

func TestGetItemError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	client := NewClient()
	client.BaseURL = ts.URL

	_, err := client.GetItem(999)
	if err == nil {
		t.Error("Expected error for 404, got nil")
	}
}
