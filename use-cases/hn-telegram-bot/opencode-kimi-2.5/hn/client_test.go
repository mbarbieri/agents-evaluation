package hn

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	client := NewClient(10 * time.Second)
	if client == nil {
		t.Error("NewClient() returned nil")
	}
	if client.httpClient == nil {
		t.Error("NewClient() httpClient is nil")
	}
	if client.httpClient.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v, want 10s", client.httpClient.Timeout)
	}
}

func TestGetTopStories(t *testing.T) {
	tests := []struct {
		name       string
		response   []int64
		statusCode int
		limit      int
		wantErr    bool
		wantIDs    []int64
	}{
		{
			name:       "successful fetch",
			response:   []int64{1, 2, 3, 4, 5},
			statusCode: http.StatusOK,
			limit:      0,
			wantErr:    false,
			wantIDs:    []int64{1, 2, 3, 4, 5},
		},
		{
			name:       "with limit",
			response:   []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			statusCode: http.StatusOK,
			limit:      3,
			wantErr:    false,
			wantIDs:    []int64{1, 2, 3},
		},
		{
			name:       "server error",
			response:   nil,
			statusCode: http.StatusInternalServerError,
			limit:      0,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v0/topstories.json" {
					t.Errorf("Expected path /v0/topstories.json, got %s", r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
				if tt.response != nil {
					json.NewEncoder(w).Encode(tt.response)
				}
			}))
			defer server.Close()

			client := NewClientWithBaseURL(5*time.Second, server.URL)

			ids, err := client.GetTopStories(tt.limit)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(ids) != len(tt.wantIDs) {
				t.Errorf("Got %d IDs, want %d", len(ids), len(tt.wantIDs))
			}

			for i, id := range ids {
				if id != tt.wantIDs[i] {
					t.Errorf("ID[%d] = %d, want %d", i, id, tt.wantIDs[i])
				}
			}
		})
	}
}

func TestGetItem(t *testing.T) {
	story := Story{
		ID:          123,
		Title:       "Test Story",
		URL:         "https://example.com",
		Score:       100,
		Descendants: 50,
		By:          "testuser",
		Time:        1234567890,
		Type:        "story",
	}

	tests := []struct {
		name       string
		storyID    int64
		response   interface{}
		statusCode int
		wantErr    bool
	}{
		{
			name:       "successful fetch",
			storyID:    123,
			response:   story,
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "not found",
			storyID:    999,
			response:   nil,
			statusCode: http.StatusNotFound,
			wantErr:    true,
		},
		{
			name:       "invalid json",
			storyID:    456,
			response:   "invalid json",
			statusCode: http.StatusOK,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := "/v0/item/"
				if len(r.URL.Path) < len(expectedPath) || r.URL.Path[:len(expectedPath)] != expectedPath {
					t.Errorf("Expected path starting with %s, got %s", expectedPath, r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
				if tt.response != nil {
					switch v := tt.response.(type) {
					case Story:
						json.NewEncoder(w).Encode(v)
					case string:
						w.Write([]byte(v))
					}
				}
			}))
			defer server.Close()

			client := NewClientWithBaseURL(5*time.Second, server.URL)

			story, err := client.GetItem(tt.storyID)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if story == nil {
				t.Error("Expected story, got nil")
				return
			}

			if story.ID != tt.storyID {
				t.Errorf("Story.ID = %d, want %d", story.ID, tt.storyID)
			}
		})
	}
}

func TestStoryFields(t *testing.T) {
	story := Story{
		ID:          12345,
		Title:       "Test Title",
		URL:         "https://example.com/test",
		Score:       150,
		Descendants: 42,
		By:          "username",
		Time:        1609459200,
		Type:        "story",
	}

	if story.ID != 12345 {
		t.Errorf("ID = %v, want 12345", story.ID)
	}
	if story.Title != "Test Title" {
		t.Errorf("Title = %v, want 'Test Title'", story.Title)
	}
	if story.URL != "https://example.com/test" {
		t.Errorf("URL = %v, want 'https://example.com/test'", story.URL)
	}
	if story.Score != 150 {
		t.Errorf("Score = %v, want 150", story.Score)
	}
	if story.Descendants != 42 {
		t.Errorf("Descendants = %v, want 42", story.Descendants)
	}
	if story.By != "username" {
		t.Errorf("By = %v, want 'username'", story.By)
	}
	if story.Time != 1609459200 {
		t.Errorf("Time = %v, want 1609459200", story.Time)
	}
	if story.Type != "story" {
		t.Errorf("Type = %v, want 'story'", story.Type)
	}
}
