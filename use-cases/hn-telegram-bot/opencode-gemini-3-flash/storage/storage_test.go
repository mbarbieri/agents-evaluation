package storage

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorage(t *testing.T) {
	dbPath := "test.db"
	defer os.Remove(dbPath)

	s, err := NewStorage(dbPath)
	require.NoError(t, err)
	defer s.Close()

	ctx := context.Background()

	t.Run("Articles", func(t *testing.T) {
		art := &Article{
			ID:        12345,
			Title:     "Test Article",
			URL:       "https://example.com",
			Summary:   "Summary",
			Tags:      []string{"go", "rust"},
			Score:     100,
			FetchedAt: time.Now(),
		}

		err := s.SaveArticle(ctx, art)
		require.NoError(t, err)

		// Test retrieval
		got, err := s.GetArticle(ctx, 12345)
		require.NoError(t, err)
		assert.Equal(t, art.Title, got.Title)
		assert.Equal(t, art.Tags, got.Tags)

		// Update message ID
		err = s.MarkArticleSent(ctx, 12345, 67890)
		require.NoError(t, err)

		got, err = s.GetArticleByMessageID(ctx, 67890)
		require.NoError(t, err)
		assert.Equal(t, int64(12345), got.ID)
	})

	t.Run("Likes", func(t *testing.T) {
		liked, err := s.IsArticleLiked(ctx, 12345)
		require.NoError(t, err)
		assert.False(t, liked)

		err = s.LikeArticle(ctx, 12345)
		require.NoError(t, err)

		liked, err = s.IsArticleLiked(ctx, 12345)
		require.NoError(t, err)
		assert.True(t, liked)

		count, err := s.GetTotalLikes(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("TagWeights", func(t *testing.T) {
		err := s.UpdateTagWeight(ctx, "go", 1.2, 1)
		require.NoError(t, err)

		weights, err := s.GetTopTags(ctx, 10)
		require.NoError(t, err)
		assert.Len(t, weights, 1)
		assert.Equal(t, "go", weights[0].Name)
		assert.Equal(t, 1.2, weights[0].Weight)

		allWeights, err := s.GetAllTagWeights(ctx)
		require.NoError(t, err)
		assert.Contains(t, allWeights, "go")
		assert.Equal(t, 1.2, allWeights["go"])
	})

	t.Run("Settings", func(t *testing.T) {
		err := s.SetSetting(ctx, "chat_id", "999")
		require.NoError(t, err)

		val, err := s.GetSetting(ctx, "chat_id")
		require.NoError(t, err)
		assert.Equal(t, "999", val)

		val, err = s.GetSetting(ctx, "non_existent")
		require.NoError(t, err)
		assert.Equal(t, "", val)
	})
}
