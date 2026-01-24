package hn

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHNClient(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v0/topstories.json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]int{1, 2, 3})
	})
	mux.HandleFunc("/v0/item/1.json", func(w http.ResponseWriter, r *http.Request) {
		item := Item{
			ID:          1,
			Title:       "Test Title",
			URL:         "https://example.com",
			Score:       100,
			Descendants: 10,
			By:          "author",
			Time:        123456789,
			Type:        "story",
		}
		json.NewEncoder(w).Encode(item)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := NewClient(server.URL, http.DefaultClient)

	t.Run("GetTopStories", func(t *testing.T) {
		ids, err := client.GetTopStories()
		if err != nil {
			t.Fatalf("failed to get top stories: %v", err)
		}
		if len(ids) != 3 || ids[0] != 1 {
			t.Errorf("ids mismatch: %v", ids)
		}
	})

	t.Run("GetItem", func(t *testing.T) {
		item, err := client.GetItem(1)
		if err != nil {
			t.Fatalf("failed to get item: %v", err)
		}
		if item.Title != "Test Title" {
			t.Errorf("title mismatch: %s", item.Title)
		}
	})
}
