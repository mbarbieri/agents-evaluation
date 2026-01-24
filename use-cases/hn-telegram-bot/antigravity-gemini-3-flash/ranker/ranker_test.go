package ranker

import (
	"math"
	"testing"
)

func TestRanker(t *testing.T) {
	tagWeights := map[string]float64{
		"go":    2.0,
		"rust":  1.5,
		"linux": 1.0,
	}

	r := NewRanker(tagWeights)

	t.Run("Scoring", func(t *testing.T) {
		tags := []string{"go", "linux"}
		hnScore := 99 // log10(99+1) = 2.0

		expectedTagScore := 2.0 + 1.0                                                // 3.0
		expectedHNScoreComp := math.Log10(float64(hnScore + 1))                      // 2.0
		expectedFinalScore := (expectedTagScore * 0.7) + (expectedHNScoreComp * 0.3) // 2.1 + 0.6 = 2.7

		got := r.Score(tags, hnScore)
		if math.Abs(got-expectedFinalScore) > 0.0001 {
			t.Errorf("expected score %f, got %f", expectedFinalScore, got)
		}
	})

	t.Run("Ranking", func(t *testing.T) {
		articles := []Article{
			{ID: 1, Tags: []string{"go"}, HNScore: 10},
			{ID: 2, Tags: []string{"rust"}, HNScore: 100},
			{ID: 3, Tags: []string{"misc"}, HNScore: 1000},
		}

		ranked := r.Rank(articles)
		if len(ranked) != 3 {
			t.Fatalf("expected 3 articles, got %d", len(ranked))
		}

		// ID 2 has rust (1.5) and score 100 (log10=2). Score = 1.5*0.7 + 2*0.3 = 1.05 + 0.6 = 1.65
		// ID 1 has go (2.0) and score 10 (log10=~1). Score = 2.0*0.7 + 1.04*0.3 = 1.4 + 0.31 = 1.71
		// ID 3 has nothing (0) and score 1000 (log10=3). Score = 0*0.7 + 3*0.3 = 0.9

		if ranked[0].ID != 1 {
			t.Errorf("expected ID 1 to be first, got %d", ranked[0].ID)
		}
	})
}
