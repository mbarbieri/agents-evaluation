package storage

import (
	"os"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	store, err := New(tmpfile.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if store == nil {
		t.Error("expected non-nil store")
	}

	if err := store.Close(); err != nil {
		t.Errorf("failed to close store: %v", err)
	}
}

func TestSaveArticle(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	store, err := New(tmpfile.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer store.Close()

	tags := []string{"rust", "programming"}
	article := Article{
		ID:        123,
		Title:     "Test Article",
		URL:       "https://example.com",
		Summary:   "Test summary",
		Tags:      tags,
		HNScore:   100,
		SentAt:    time.Now(),
		MessageID: 456,
	}

	err = store.SaveArticle(article)
	if err != nil {
		t.Fatalf("failed to save article: %v", err)
	}

	retrieved, err := store.GetArticle(123)
	if err != nil {
		t.Fatalf("failed to get article: %v", err)
	}

	if retrieved.ID != 123 {
		t.Errorf("ID = %v, want 123", retrieved.ID)
	}
	if retrieved.Title != "Test Article" {
		t.Errorf("Title = %v, want Test Article", retrieved.Title)
	}
	if retrieved.URL != "https://example.com" {
		t.Errorf("URL = %v, want https://example.com", retrieved.URL)
	}
	if retrieved.Summary != "Test summary" {
		t.Errorf("Summary = %v, want Test summary", retrieved.Summary)
	}
	if len(retrieved.Tags) != 2 {
		t.Errorf("Tags length = %v, want 2", len(retrieved.Tags))
	}
	if retrieved.HNScore != 100 {
		t.Errorf("HNScore = %v, want 100", retrieved.HNScore)
	}
	if retrieved.MessageID != 456 {
		t.Errorf("MessageID = %v, want 456", retrieved.MessageID)
	}
}

func TestGetArticle(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	store, err := New(tmpfile.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer store.Close()

	_, err = store.GetArticle(999)
	if err == nil {
		t.Error("expected error for non-existent article, got nil")
	}

	tags := []string{"go"}
	article := Article{
		ID:      123,
		Title:   "Test",
		URL:     "https://example.com",
		Summary: "Summary",
		Tags:    tags,
		HNScore: 50,
	}

	err = store.SaveArticle(article)
	if err != nil {
		t.Fatalf("failed to save article: %v", err)
	}

	retrieved, err := store.GetArticle(123)
	if err != nil {
		t.Fatalf("failed to get article: %v", err)
	}

	if retrieved.Title != "Test" {
		t.Errorf("Title = %v, want Test", retrieved.Title)
	}
}

func TestGetArticleByMessageID(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	store, err := New(tmpfile.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer store.Close()

	_, err = store.GetArticleByMessageID(999)
	if err == nil {
		t.Error("expected error for non-existent message, got nil")
	}

	tags := []string{"python"}
	article := Article{
		ID:        456,
		Title:     "Test",
		URL:       "https://example.com",
		Summary:   "Summary",
		Tags:      tags,
		HNScore:   75,
		MessageID: 789,
	}

	err = store.SaveArticle(article)
	if err != nil {
		t.Fatalf("failed to save article: %v", err)
	}

	retrieved, err := store.GetArticleByMessageID(789)
	if err != nil {
		t.Fatalf("failed to get article: %v", err)
	}

	if retrieved.ID != 456 {
		t.Errorf("ID = %v, want 456", retrieved.ID)
	}
}

func TestGetRecentArticles(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	store, err := New(tmpfile.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer store.Close()

	now := time.Now()
	oldTime := now.Add(-8 * 24 * time.Hour)

	oldArticle := Article{
		ID:      1,
		Title:   "Old Article",
		URL:     "https://example.com/old",
		Summary: "Old summary",
		Tags:    []string{"old"},
		HNScore: 10,
		SentAt:  oldTime,
	}

	recentArticle := Article{
		ID:      2,
		Title:   "Recent Article",
		URL:     "https://example.com/new",
		Summary: "New summary",
		Tags:    []string{"new"},
		HNScore: 20,
		SentAt:  now,
	}

	if err := store.SaveArticle(oldArticle); err != nil {
		t.Fatalf("failed to save old article: %v", err)
	}
	if err := store.SaveArticle(recentArticle); err != nil {
		t.Fatalf("failed to save recent article: %v", err)
	}

	cutoff := now.Add(-7 * 24 * time.Hour)
	recent, err := store.GetRecentArticles(cutoff)
	if err != nil {
		t.Fatalf("failed to get recent articles: %v", err)
	}

	if len(recent) != 1 {
		t.Errorf("got %d recent articles, want 1", len(recent))
	}

	if len(recent) > 0 && recent[0] != 2 {
		t.Errorf("recent article ID = %v, want 2", recent[0])
	}
}

func TestLikeArticle(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	store, err := New(tmpfile.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer store.Close()

	err = store.LikeArticle(123)
	if err != nil {
		t.Fatalf("failed to like article: %v", err)
	}

	liked, err := store.IsArticleLiked(123)
	if err != nil {
		t.Fatalf("failed to check if article is liked: %v", err)
	}

	if !liked {
		t.Error("expected article to be liked")
	}

	likedAgain, err := store.IsArticleLiked(123)
	if err != nil {
		t.Fatalf("failed to check if article is liked again: %v", err)
	}

	if !likedAgain {
		t.Error("expected article to still be liked")
	}
}

func TestIsArticleLiked(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	store, err := New(tmpfile.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer store.Close()

	liked, err := store.IsArticleLiked(999)
	if err != nil {
		t.Fatalf("failed to check if article is liked: %v", err)
	}

	if liked {
		t.Error("expected article to not be liked")
	}

	if err := store.LikeArticle(999); err != nil {
		t.Fatalf("failed to like article: %v", err)
	}

	liked, err = store.IsArticleLiked(999)
	if err != nil {
		t.Fatalf("failed to check if article is liked: %v", err)
	}

	if !liked {
		t.Error("expected article to be liked")
	}
}

func TestGetLikeCount(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	store, err := New(tmpfile.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer store.Close()

	count, err := store.GetLikeCount()
	if err != nil {
		t.Fatalf("failed to get like count: %v", err)
	}

	if count != 0 {
		t.Errorf("like count = %d, want 0", count)
	}

	for i := 1; i <= 5; i++ {
		if err := store.LikeArticle(int64(i)); err != nil {
			t.Fatalf("failed to like article: %v", err)
		}
	}

	count, err = store.GetLikeCount()
	if err != nil {
		t.Fatalf("failed to get like count: %v", err)
	}

	if count != 5 {
		t.Errorf("like count = %d, want 5", count)
	}
}

func TestSetTagWeight(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	store, err := New(tmpfile.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer store.Close()

	err = store.SetTagWeight("rust", 2.5, 3)
	if err != nil {
		t.Fatalf("failed to set tag weight: %v", err)
	}

	weight, count, err := store.GetTagWeight("rust")
	if err != nil {
		t.Fatalf("failed to get tag weight: %v", err)
	}

	if weight != 2.5 {
		t.Errorf("weight = %v, want 2.5", weight)
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

func TestGetTagWeight(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	store, err := New(tmpfile.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer store.Close()

	_, _, err = store.GetTagWeight("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent tag, got nil")
	}

	if err := store.SetTagWeight("go", 1.5, 2); err != nil {
		t.Fatalf("failed to set tag weight: %v", err)
	}

	weight, count, err := store.GetTagWeight("go")
	if err != nil {
		t.Fatalf("failed to get tag weight: %v", err)
	}

	if weight != 1.5 {
		t.Errorf("weight = %v, want 1.5", weight)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

func TestGetAllTagWeights(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	store, err := New(tmpfile.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer store.Close()

	tags := map[string]TagWeight{
		"rust":   {Weight: 2.0, Count: 5},
		"go":     {Weight: 1.5, Count: 3},
		"python": {Weight: 1.0, Count: 2},
	}

	for tag, tw := range tags {
		if err := store.SetTagWeight(tag, tw.Weight, tw.Count); err != nil {
			t.Fatalf("failed to set tag weight: %v", err)
		}
	}

	allTags, err := store.GetAllTagWeights()
	if err != nil {
		t.Fatalf("failed to get all tag weights: %v", err)
	}

	if len(allTags) != 3 {
		t.Errorf("got %d tags, want 3", len(allTags))
	}

	for tag, tw := range allTags {
		expected, ok := tags[tag]
		if !ok {
			t.Errorf("unexpected tag: %s", tag)
			continue
		}
		if tw.Weight != expected.Weight {
			t.Errorf("tag %s weight = %v, want %v", tag, tw.Weight, expected.Weight)
		}
		if tw.Count != expected.Count {
			t.Errorf("tag %s count = %d, want %d", tag, tw.Count, expected.Count)
		}
	}
}

func TestDecayTagWeights(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	store, err := New(tmpfile.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer store.Close()

	if err := store.SetTagWeight("rust", 2.0, 5); err != nil {
		t.Fatalf("failed to set tag weight: %v", err)
	}
	if err := store.SetTagWeight("go", 0.15, 3); err != nil {
		t.Fatalf("failed to set tag weight: %v", err)
	}

	err = store.DecayTagWeights(0.5, 0.2)
	if err != nil {
		t.Fatalf("failed to decay tag weights: %v", err)
	}

	rustWeight, _, err := store.GetTagWeight("rust")
	if err != nil {
		t.Fatalf("failed to get rust tag weight: %v", err)
	}

	goWeight, _, err := store.GetTagWeight("go")
	if err != nil {
		t.Fatalf("failed to get go tag weight: %v", err)
	}

	if rustWeight != 1.0 {
		t.Errorf("rust weight = %v, want 1.0", rustWeight)
	}

	if goWeight != 0.2 {
		t.Errorf("go weight = %v, want 0.2 (min weight)", goWeight)
	}
}

func TestSetSetting(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	store, err := New(tmpfile.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer store.Close()

	err = store.SetSetting("chat_id", "12345")
	if err != nil {
		t.Fatalf("failed to set setting: %v", err)
	}

	value, err := store.GetSetting("chat_id")
	if err != nil {
		t.Fatalf("failed to get setting: %v", err)
	}

	if value != "12345" {
		t.Errorf("value = %v, want 12345", value)
	}
}

func TestGetSetting(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	store, err := New(tmpfile.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer store.Close()

	_, err = store.GetSetting("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent setting, got nil")
	}

	if err := store.SetSetting("digest_time", "10:00"); err != nil {
		t.Fatalf("failed to set setting: %v", err)
	}

	value, err := store.GetSetting("digest_time")
	if err != nil {
		t.Fatalf("failed to get setting: %v", err)
	}

	if value != "10:00" {
		t.Errorf("value = %v, want 10:00", value)
	}
}

func TestGetTopTags(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	store, err := New(tmpfile.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer store.Close()

	tags, err := store.GetTopTags(10)
	if err != nil {
		t.Fatalf("failed to get top tags: %v", err)
	}

	if len(tags) != 0 {
		t.Errorf("got %d tags, want 0", len(tags))
	}

	if err := store.SetTagWeight("rust", 5.0, 10); err != nil {
		t.Fatalf("failed to set tag weight: %v", err)
	}
	if err := store.SetTagWeight("go", 3.0, 8); err != nil {
		t.Fatalf("failed to set tag weight: %v", err)
	}
	if err := store.SetTagWeight("python", 2.0, 5); err != nil {
		t.Fatalf("failed to set tag weight: %v", err)
	}

	tags, err = store.GetTopTags(2)
	if err != nil {
		t.Fatalf("failed to get top tags: %v", err)
	}

	if len(tags) != 2 {
		t.Errorf("got %d tags, want 2", len(tags))
	}

	if tags[0].Tag != "rust" {
		t.Errorf("first tag = %v, want rust", tags[0].Tag)
	}
	if tags[0].Weight != 5.0 {
		t.Errorf("rust weight = %v, want 5.0", tags[0].Weight)
	}
	if tags[1].Tag != "go" {
		t.Errorf("second tag = %v, want go", tags[1].Tag)
	}
}
