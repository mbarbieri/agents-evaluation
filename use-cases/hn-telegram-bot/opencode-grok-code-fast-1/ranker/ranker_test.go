package ranker

import (
	"math"
	"testing"
)

func TestCalculateScore(t *testing.T) {
	tests := []struct {
		name       string
		tags       []string
		hnScore    int
		tagWeights map[string]float64
		expected   float64
	}{
		{
			name:       "no tags, low hn",
			tags:       []string{},
			hnScore:    1,
			tagWeights: map[string]float64{},
			expected:   0.3 * math.Log10(2),
		},
		{
			name:       "some tags",
			tags:       []string{"go", "programming"},
			hnScore:    100,
			tagWeights: map[string]float64{"go": 1.5, "programming": 2.0, "ai": 1.0},
			expected:   (1.5+2.0)*0.7 + math.Log10(101)*0.3,
		},
		{
			name:       "unknown tags",
			tags:       []string{"unknown"},
			hnScore:    50,
			tagWeights: map[string]float64{"go": 1.0},
			expected:   0*0.7 + math.Log10(51)*0.3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := calculateScore(tt.tags, tt.hnScore, tt.tagWeights)
			if math.Abs(score-tt.expected) > 0.001 {
				t.Errorf("expected score %.3f, got %.3f", tt.expected, score)
			}
		})
	}
}

func TestRank(t *testing.T) {
	articles := []Article{
		{Tags: []string{"go"}, HNScore: 10},
		{Tags: []string{"ai", "ml"}, HNScore: 50},
		{Tags: []string{"go", "programming"}, HNScore: 20},
	}

	tagWeights := map[string]float64{
		"go":          2.0,
		"programming": 1.5,
		"ai":          3.0,
		"ml":          2.5,
	}

	ranked := Rank(articles, tagWeights)

	// Check sorted by score descending
	if len(ranked) != 3 {
		t.Fatalf("expected 3 ranked articles, got %d", len(ranked))
	}

	// First should be ai ml with high weights
	if ranked[0].Score <= ranked[1].Score || ranked[1].Score <= ranked[2].Score {
		t.Error("articles not sorted by score descending")
	}

	// Check scores are set
	for i, r := range ranked {
		expectedScore := calculateScore(r.Tags, r.HNScore, tagWeights)
		if math.Abs(r.Score-expectedScore) > 0.001 {
			t.Errorf("ranked article %d score mismatch: expected %.3f, got %.3f", i, expectedScore, r.Score)
		}
	}
}
