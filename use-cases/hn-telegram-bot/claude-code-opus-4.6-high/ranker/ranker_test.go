package ranker

import (
	"math"
	"testing"
)

func TestRank_BasicScoring(t *testing.T) {
	weights := TagWeightMap{
		"go":          2.0,
		"programming": 1.5,
		"rust":        1.0,
	}

	articles := []Article{
		{ID: 1, Tags: []string{"rust"}, HNScore: 100},
		{ID: 2, Tags: []string{"go", "programming"}, HNScore: 50},
		{ID: 3, Tags: []string{"unknown"}, HNScore: 500},
	}

	ranked := Rank(articles, weights)

	// Article 2 has tag_score=3.5, hn_score=log10(51)≈1.71 → score=3.5*0.7+1.71*0.3≈2.96
	// Article 1 has tag_score=1.0, hn_score=log10(101)≈2.004 → score=1.0*0.7+2.004*0.3≈1.30
	// Article 3 has tag_score=0.0, hn_score=log10(501)≈2.70 → score=0*0.7+2.70*0.3≈0.81

	if ranked[0].ID != 2 {
		t.Errorf("expected article 2 ranked first, got %d (score=%.4f)", ranked[0].ID, ranked[0].Score)
	}
	if ranked[1].ID != 1 {
		t.Errorf("expected article 1 ranked second, got %d (score=%.4f)", ranked[1].ID, ranked[1].Score)
	}
	if ranked[2].ID != 3 {
		t.Errorf("expected article 3 ranked third, got %d (score=%.4f)", ranked[2].ID, ranked[2].Score)
	}
}

func TestRank_EmptyArticles(t *testing.T) {
	result := Rank(nil, TagWeightMap{"go": 1.0})
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d", len(result))
	}
}

func TestRank_NoWeights(t *testing.T) {
	articles := []Article{
		{ID: 1, Tags: []string{"go"}, HNScore: 200},
		{ID: 2, Tags: []string{"rust"}, HNScore: 100},
	}

	ranked := Rank(articles, TagWeightMap{})

	// With no tag weights, ranking is purely by HN score
	if ranked[0].ID != 1 {
		t.Errorf("expected article 1 first (higher HN score), got %d", ranked[0].ID)
	}
}

func TestRank_ScoreFormula(t *testing.T) {
	weights := TagWeightMap{"go": 2.0}
	articles := []Article{
		{ID: 1, Tags: []string{"go"}, HNScore: 99},
	}

	Rank(articles, weights)

	expectedTagScore := 2.0
	expectedHNScore := math.Log10(100)
	expectedTotal := (expectedTagScore * 0.7) + (expectedHNScore * 0.3)

	if math.Abs(articles[0].Score-expectedTotal) > 0.001 {
		t.Errorf("expected score %.4f, got %.4f", expectedTotal, articles[0].Score)
	}
}

func TestRank_ZeroHNScore(t *testing.T) {
	articles := []Article{
		{ID: 1, Tags: []string{}, HNScore: 0},
	}

	Rank(articles, TagWeightMap{})

	expectedScore := math.Log10(1) * 0.3 // log10(0+1) = 0
	if math.Abs(articles[0].Score-expectedScore) > 0.001 {
		t.Errorf("expected score %.4f, got %.4f", expectedScore, articles[0].Score)
	}
}

func TestRank_MultipleTags(t *testing.T) {
	weights := TagWeightMap{
		"ai":  1.0,
		"ml":  1.5,
		"nlp": 0.5,
	}

	articles := []Article{
		{ID: 1, Tags: []string{"ai", "ml", "nlp"}, HNScore: 10},
	}

	Rank(articles, weights)

	expectedTagScore := 3.0 // 1.0 + 1.5 + 0.5
	expectedHNScore := math.Log10(11)
	expected := (expectedTagScore * 0.7) + (expectedHNScore * 0.3)

	if math.Abs(articles[0].Score-expected) > 0.001 {
		t.Errorf("expected score %.4f, got %.4f", expected, articles[0].Score)
	}
}
