package ranker

import (
	"math"
	"testing"
)

func TestRankArticles(t *testing.T) {
	weights := map[string]float64{
		"go":       2.0,
		"rust":     1.5,
		"python":   1.0,
		"security": 1.8,
	}

	articles := []RankableArticle{
		{ID: 1, Tags: []string{"go", "testing"}, HNScore: 100},
		{ID: 2, Tags: []string{"rust", "security"}, HNScore: 50},
		{ID: 3, Tags: []string{"python"}, HNScore: 200},
	}

	r := NewRanker(0.7, 0.3)
	ranked := r.Rank(articles, weights)

	if len(ranked) != 3 {
		t.Fatalf("got %d articles, want 3", len(ranked))
	}

	// Article 2 has tags with weights 1.5 + 1.8 = 3.3 (highest tag score)
	// Article 1 has tags with weights 2.0 + 1.0 (default) = 3.0
	// Article 3 has tags with weights 1.0
	// But HN scores also factor in
	// Let's verify articles are sorted by final score
	for i := 1; i < len(ranked); i++ {
		if ranked[i].FinalScore > ranked[i-1].FinalScore {
			t.Errorf("articles not sorted: %v has higher score than %v",
				ranked[i].ID, ranked[i-1].ID)
		}
	}
}

func TestRankArticlesEmpty(t *testing.T) {
	r := NewRanker(0.7, 0.3)
	ranked := r.Rank(nil, nil)

	if len(ranked) != 0 {
		t.Errorf("got %d articles for nil input, want 0", len(ranked))
	}

	ranked = r.Rank([]RankableArticle{}, map[string]float64{})
	if len(ranked) != 0 {
		t.Errorf("got %d articles for empty input, want 0", len(ranked))
	}
}

func TestScoreCalculation(t *testing.T) {
	weights := map[string]float64{
		"go": 2.0,
	}

	articles := []RankableArticle{
		{ID: 1, Tags: []string{"go"}, HNScore: 99}, // log10(100) = 2
	}

	r := NewRanker(0.7, 0.3)
	ranked := r.Rank(articles, weights)

	// Tag score = 2.0
	// HN score = log10(99 + 1) = 2.0
	// Final = 2.0 * 0.7 + 2.0 * 0.3 = 1.4 + 0.6 = 2.0
	expectedScore := 2.0
	if math.Abs(ranked[0].FinalScore-expectedScore) > 0.01 {
		t.Errorf("FinalScore = %f, want %f", ranked[0].FinalScore, expectedScore)
	}

	if math.Abs(ranked[0].TagScore-2.0) > 0.01 {
		t.Errorf("TagScore = %f, want 2.0", ranked[0].TagScore)
	}

	if math.Abs(ranked[0].HNScoreComponent-2.0) > 0.01 {
		t.Errorf("HNScoreComponent = %f, want 2.0", ranked[0].HNScoreComponent)
	}
}

func TestUnknownTagsGetDefaultWeight(t *testing.T) {
	weights := map[string]float64{} // empty weights

	articles := []RankableArticle{
		{ID: 1, Tags: []string{"unknown", "tag"}, HNScore: 0},
	}

	r := NewRanker(1.0, 0.0) // Only tag score
	ranked := r.Rank(articles, weights)

	// Each unknown tag gets default weight of 1.0
	// So tag score = 1.0 + 1.0 = 2.0
	if math.Abs(ranked[0].TagScore-2.0) > 0.01 {
		t.Errorf("TagScore = %f, want 2.0 (default weights)", ranked[0].TagScore)
	}
}

func TestZeroHNScore(t *testing.T) {
	articles := []RankableArticle{
		{ID: 1, Tags: []string{}, HNScore: 0},
	}

	r := NewRanker(0.0, 1.0) // Only HN score
	ranked := r.Rank(articles, nil)

	// log10(0 + 1) = log10(1) = 0
	if ranked[0].HNScoreComponent != 0 {
		t.Errorf("HNScoreComponent = %f, want 0", ranked[0].HNScoreComponent)
	}
}

func TestMultipleArticlesWithSameScore(t *testing.T) {
	articles := []RankableArticle{
		{ID: 1, Tags: []string{}, HNScore: 10},
		{ID: 2, Tags: []string{}, HNScore: 10},
	}

	r := NewRanker(0.0, 1.0)
	ranked := r.Rank(articles, nil)

	if ranked[0].FinalScore != ranked[1].FinalScore {
		t.Error("articles with same inputs should have same score")
	}
}

func TestCustomWeights(t *testing.T) {
	articles := []RankableArticle{
		{ID: 1, Tags: []string{"tag"}, HNScore: 99}, // tag=1.0, hn=2.0
	}

	// 100% tag weight
	r1 := NewRanker(1.0, 0.0)
	ranked1 := r1.Rank(articles, map[string]float64{"tag": 1.0})
	if ranked1[0].FinalScore != 1.0 {
		t.Errorf("with 100%% tag weight, score = %f, want 1.0", ranked1[0].FinalScore)
	}

	// 100% HN weight
	r2 := NewRanker(0.0, 1.0)
	ranked2 := r2.Rank(articles, nil)
	if math.Abs(ranked2[0].FinalScore-2.0) > 0.01 {
		t.Errorf("with 100%% HN weight, score = %f, want 2.0", ranked2[0].FinalScore)
	}
}
