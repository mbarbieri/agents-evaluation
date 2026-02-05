package ranker

import (
	"math"
	"testing"

	"hn-telegram-bot/model"
)

func TestRank(t *testing.T) {
	articles := []model.Article{
		{ID: 1, Tags: []string{"go"}, HNScore: 10},
		{ID: 2, Tags: []string{"ai"}, HNScore: 100},
	}
	weights := map[string]model.TagWeight{
		"go": {Tag: "go", Weight: 2.0},
		"ai": {Tag: "ai", Weight: 0.1},
	}

	scored := Rank(articles, weights)
	if len(scored) != 2 {
		t.Fatalf("expected 2 scored articles")
	}
	if scored[0].Article.ID != 1 {
		t.Fatalf("expected article 1 to rank first, got %d", scored[0].Article.ID)
	}
	expectedHN := math.Log10(11)
	if math.Abs(scored[0].HNScore-expectedHN) > 0.0001 {
		t.Fatalf("unexpected HN score: %v", scored[0].HNScore)
	}
	expectedScore := 2.0*0.7 + expectedHN*0.3
	if math.Abs(scored[0].Score-expectedScore) > 0.0001 {
		t.Fatalf("unexpected final score: %v", scored[0].Score)
	}
}
