package ranker

import (
	"math"
	"testing"
)

func TestRank_Scoring(t *testing.T) {
	tagWeights := map[string]float64{
		"golang": 2.0,
		"rust":   1.5,
		"python": 1.0,
	}

	articles := []Article{
		{
			ID:      1,
			Tags:    []string{"golang", "rust"},
			HNScore: 100,
		},
		{
			ID:      2,
			Tags:    []string{"python"},
			HNScore: 200,
		},
		{
			ID:      3,
			Tags:    []string{"golang"},
			HNScore: 50,
		},
	}

	r := New()
	ranked := r.Rank(articles, tagWeights)

	if len(ranked) != 3 {
		t.Fatalf("Rank() returned %d articles, want 3", len(ranked))
	}

	// Verify scores are calculated correctly
	// Article 1: tag_score = 2.0 + 1.5 = 3.5, hn_score = log10(101) ≈ 2.004
	// final = 3.5 * 0.7 + 2.004 * 0.3 = 2.45 + 0.601 = 3.051
	expectedScore1 := 3.5*0.7 + math.Log10(101)*0.3
	if math.Abs(ranked[0].Score-expectedScore1) > 0.01 {
		t.Errorf("Article 1 score = %v, want %v", ranked[0].Score, expectedScore1)
	}

	// Verify article 1 (highest tag score) is ranked first
	if ranked[0].Article.ID != 1 {
		t.Errorf("Top ranked article ID = %v, want 1", ranked[0].Article.ID)
	}
}

func TestRank_SortOrder(t *testing.T) {
	tagWeights := map[string]float64{
		"high": 5.0,
		"low":  0.5,
	}

	articles := []Article{
		{ID: 1, Tags: []string{"low"}, HNScore: 10},
		{ID: 2, Tags: []string{"high"}, HNScore: 10},
		{ID: 3, Tags: []string{"high", "high"}, HNScore: 10},
	}

	r := New()
	ranked := r.Rank(articles, tagWeights)

	// Article 3 should be first (2x high tag)
	if ranked[0].Article.ID != 3 {
		t.Errorf("First article ID = %v, want 3", ranked[0].Article.ID)
	}

	// Article 2 should be second (1x high tag)
	if ranked[1].Article.ID != 2 {
		t.Errorf("Second article ID = %v, want 2", ranked[1].Article.ID)
	}

	// Article 1 should be last (1x low tag)
	if ranked[2].Article.ID != 1 {
		t.Errorf("Third article ID = %v, want 1", ranked[2].Article.ID)
	}
}

func TestRank_NoTags(t *testing.T) {
	tagWeights := map[string]float64{}

	articles := []Article{
		{ID: 1, Tags: []string{}, HNScore: 100},
		{ID: 2, Tags: []string{}, HNScore: 200},
	}

	r := New()
	ranked := r.Rank(articles, tagWeights)

	// Article 2 should be first (higher HN score)
	if ranked[0].Article.ID != 2 {
		t.Errorf("First article ID = %v, want 2", ranked[0].Article.ID)
	}

	// With no tags, score should be purely from HN score
	expectedScore := math.Log10(201) * 0.3
	if math.Abs(ranked[0].Score-expectedScore) > 0.01 {
		t.Errorf("Article 2 score = %v, want %v", ranked[0].Score, expectedScore)
	}
}

func TestRank_UnknownTags(t *testing.T) {
	tagWeights := map[string]float64{
		"known": 2.0,
	}

	articles := []Article{
		{ID: 1, Tags: []string{"unknown"}, HNScore: 100},
	}

	r := New()
	ranked := r.Rank(articles, tagWeights)

	// Unknown tags should default to weight 1.0
	expectedTagScore := 1.0
	expectedScore := expectedTagScore*0.7 + math.Log10(101)*0.3
	if math.Abs(ranked[0].Score-expectedScore) > 0.01 {
		t.Errorf("Article score = %v, want %v", ranked[0].Score, expectedScore)
	}
}

func TestRank_ZeroHNScore(t *testing.T) {
	tagWeights := map[string]float64{
		"test": 1.5,
	}

	articles := []Article{
		{ID: 1, Tags: []string{"test"}, HNScore: 0},
	}

	r := New()
	ranked := r.Rank(articles, tagWeights)

	// log10(0 + 1) = 0
	expectedScore := 1.5*0.7 + 0.0
	if math.Abs(ranked[0].Score-expectedScore) > 0.01 {
		t.Errorf("Article score = %v, want %v", ranked[0].Score, expectedScore)
	}
}

func TestRank_EmptyArticles(t *testing.T) {
	r := New()
	ranked := r.Rank([]Article{}, map[string]float64{})

	if len(ranked) != 0 {
		t.Errorf("Rank() with empty articles returned %d results, want 0", len(ranked))
	}
}
