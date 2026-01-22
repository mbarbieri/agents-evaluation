package hn

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetTopStories(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0/topstories.json" {
			t.Errorf("expected /v0/topstories.json, got %s", r.URL.Path)
		}
		stories := []int{1, 2, 3}
		json.NewEncoder(w).Encode(stories)
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: time.Second},
	}

	ids, err := client.GetTopStories()
	if err != nil {
		t.Fatal(err)
	}

	expected := []int{1, 2, 3}
	if len(ids) != len(expected) {
		t.Errorf("expected %d ids, got %d", len(expected), len(ids))
	}
	for i, id := range ids {
		if id != expected[i] {
			t.Errorf("expected id %d, got %d", expected[i], id)
		}
	}
}

func TestGetItem(t *testing.T) {
	item := Item{
		ID:          1,
		Title:       "Test Story",
		URL:         "https://example.com",
		Score:       100,
		Descendants: 10,
		By:          "testuser",
		Time:        1640995200, // 2022-01-01
		Type:        "story",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/v0/item/1.json"
		if r.URL.Path != expectedPath {
			t.Errorf("expected %s, got %s", expectedPath, r.URL.Path)
		}
		json.NewEncoder(w).Encode(item)
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: time.Second},
	}

	got, err := client.GetItem(1)
	if err != nil {
		t.Fatal(err)
	}

	if got.ID != item.ID {
		t.Errorf("expected ID %d, got %d", item.ID, got.ID)
	}
	if got.Title != item.Title {
		t.Errorf("expected Title %s, got %s", item.Title, got.Title)
	}
	if got.URL != item.URL {
		t.Errorf("expected URL %s, got %s", item.URL, got.URL)
	}
	if got.Score != item.Score {
		t.Errorf("expected Score %d, got %d", item.Score, got.Score)
	}
	if got.Descendants != item.Descendants {
		t.Errorf("expected Descendants %d, got %d", item.Descendants, got.Descendants)
	}
	if got.By != item.By {
		t.Errorf("expected By %s, got %s", item.By, got.By)
	}
	if got.Time != item.Time {
		t.Errorf("expected Time %d, got %d", item.Time, got.Time)
	}
	if got.Type != item.Type {
		t.Errorf("expected Type %s, got %s", item.Type, got.Type)
	}
}

func TestGetItem_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: time.Second},
	}

	_, err := client.GetItem(1)
	if err == nil {
		t.Error("expected error for 404")
	}
}
