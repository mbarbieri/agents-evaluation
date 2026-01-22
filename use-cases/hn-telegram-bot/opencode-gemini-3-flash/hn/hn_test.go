package hn

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	client := NewClient(server.URL)

	t.Run("GetTopStories", func(t *testing.T) {
		mux.HandleFunc("/v0/topstories.json", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "[1, 2, 3]")
		})

		ids, err := client.GetTopStories(context.Background())
		require.NoError(t, err)
		assert.Equal(t, []int64{1, 2, 3}, ids)
	})

	t.Run("GetItem", func(t *testing.T) {
		mux.HandleFunc("/v0/item/1.json", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"id": 1, "title": "Test", "url": "https://example.com", "score": 100, "descendants": 10}`)
		})

		item, err := client.GetItem(context.Background(), 1)
		require.NoError(t, err)
		assert.Equal(t, int64(1), item.ID)
		assert.Equal(t, "Test", item.Title)
		assert.Equal(t, 100, item.Score)
		assert.Equal(t, 10, item.Descendants)
	})
}
