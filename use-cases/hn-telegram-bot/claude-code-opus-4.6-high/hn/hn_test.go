package hn

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func setupTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, Client) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client := NewClientWithBaseURL(server.Client(), server.URL)
	return server, client
}

func TestTopStories_Success(t *testing.T) {
	ids := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	_, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0/topstories.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ids)
	})

	result, err := client.TopStories(context.Background(), 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 5 {
		t.Fatalf("expected 5 IDs, got %d", len(result))
	}
	for i := 0; i < 5; i++ {
		if result[i] != ids[i] {
			t.Errorf("expected ID %d at index %d, got %d", ids[i], i, result[i])
		}
	}
}

func TestTopStories_LimitLargerThanAvailable(t *testing.T) {
	ids := []int{1, 2, 3}
	_, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ids)
	})

	result, err := client.TopStories(context.Background(), 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("expected 3 IDs, got %d", len(result))
	}
}

func TestTopStories_ServerError(t *testing.T) {
	_, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := client.TopStories(context.Background(), 10)
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

func TestTopStories_InvalidJSON(t *testing.T) {
	_, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	})

	_, err := client.TopStories(context.Background(), 10)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestTopStories_ContextCancellation(t *testing.T) {
	_, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]int{1, 2, 3})
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := client.TopStories(ctx, 10)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestGetItem_Success(t *testing.T) {
	item := Item{
		ID:          12345,
		Title:       "Test Article",
		URL:         "https://example.com",
		Score:       100,
		Descendants: 50,
		By:          "testuser",
		Time:        1700000000,
		Type:        "story",
	}

	_, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0/item/12345.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(item)
	})

	result, err := client.GetItem(context.Background(), 12345)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != 12345 {
		t.Errorf("expected ID 12345, got %d", result.ID)
	}
	if result.Title != "Test Article" {
		t.Errorf("expected title 'Test Article', got %s", result.Title)
	}
	if result.URL != "https://example.com" {
		t.Errorf("expected URL https://example.com, got %s", result.URL)
	}
	if result.Score != 100 {
		t.Errorf("expected score 100, got %d", result.Score)
	}
	if result.Descendants != 50 {
		t.Errorf("expected descendants 50, got %d", result.Descendants)
	}
	if result.By != "testuser" {
		t.Errorf("expected by 'testuser', got %s", result.By)
	}
	if result.Time != 1700000000 {
		t.Errorf("expected time 1700000000, got %d", result.Time)
	}
	if result.Type != "story" {
		t.Errorf("expected type 'story', got %s", result.Type)
	}
}

func TestGetItem_NotFound(t *testing.T) {
	_, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := client.GetItem(context.Background(), 99999)
	if err == nil {
		t.Fatal("expected error for not found item")
	}
}

func TestGetItem_ServerError(t *testing.T) {
	_, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := client.GetItem(context.Background(), 12345)
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

func TestGetItem_InvalidJSON(t *testing.T) {
	_, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	})

	_, err := client.GetItem(context.Background(), 12345)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestGetItem_ContextCancellation(t *testing.T) {
	_, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Item{ID: 1})
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.GetItem(ctx, 1)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestNewClient_NilHTTPClient(t *testing.T) {
	client := NewClient(nil)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNewClient_DefaultBaseURL(t *testing.T) {
	client := NewClient(&http.Client{}).(*httpClient)
	if client.baseURL != BaseURL {
		t.Errorf("expected base URL %s, got %s", BaseURL, client.baseURL)
	}
}
