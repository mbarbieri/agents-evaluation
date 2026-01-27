package ranker

import (
	"testing"
)

func TestNew(t *testing.T) {
	t.Run("creates ranker with defaults", func(t *testing.T) {
		r := New(nil, 0, 0)
		if r == nil {
			t.Error("New() returned nil")
		}
		if r.tagWeights == nil {
			t.Error("tagWeights should not be nil")
		}
		if r.tagWeightPct != 0.7 {
			t.Errorf("tagWeightPct default = %v, want 0.7", r.tagWeightPct)
		}
		if r.hnWeightPct != 0.3 {
			t.Errorf("hnWeightPct default = %v, want 0.3", r.hnWeightPct)
		}
	})

	t.Run("creates ranker with custom weights", func(t *testing.T) {
		weights := map[string]float64{"go": 1.5, "rust": 2.0}
		r := New(weights, 0.8, 0.2)
		if r.tagWeightPct != 0.8 {
			t.Errorf("tagWeightPct = %v, want 0.8", r.tagWeightPct)
		}
		if r.hnWeightPct != 0.2 {
			t.Errorf("hnWeightPct = %v, want 0.2", r.hnWeightPct)
		}
		if r.tagWeights["go"] != 1.5 {
			t.Errorf("tagWeights[go] = %v, want 1.5", r.tagWeights["go"])
		}
	})
}

func TestRank(t *testing.T) {
	t.Run("ranks articles by final score", func(t *testing.T) {
		weights := map[string]float64{
			"go":   1.0,
			"rust": 2.0,
		}
		r := New(weights, 0.7, 0.3)

		articles := []Article{
			{ID: 1, Title: "Go Article", Tags: []string{"go"}, HNScore: 100},
			{ID: 2, Title: "Rust Article", Tags: []string{"rust"}, HNScore: 50},
			{ID: 3, Title: "Python Article", Tags: []string{"python"}, HNScore: 200},
		}

		ranked := r.Rank(articles)

		if len(ranked) != 3 {
			t.Errorf("Got %d ranked articles, want 3", len(ranked))
		}

		// Rust article should be first (weight 2.0)
		if ranked[0].ID != 2 {
			t.Errorf("First article ID = %d, want 2 (Rust)", ranked[0].ID)
		}

		// Go article should be second (weight 1.0)
		if ranked[1].ID != 1 {
			t.Errorf("Second article ID = %d, want 1 (Go)", ranked[1].ID)
		}

		// Python article should be last (no weight, only HN score)
		if ranked[2].ID != 3 {
			t.Errorf("Third article ID = %d, want 3 (Python)", ranked[2].ID)
		}
	})

	t.Run("empty article list", func(t *testing.T) {
		r := New(nil, 0.7, 0.3)
		ranked := r.Rank([]Article{})
		if len(ranked) != 0 {
			t.Errorf("Got %d ranked articles, want 0", len(ranked))
		}
	})

	t.Run("single article", func(t *testing.T) {
		weights := map[string]float64{"go": 1.5}
		r := New(weights, 0.7, 0.3)

		articles := []Article{
			{ID: 1, Title: "Go Article", Tags: []string{"go"}, HNScore: 100},
		}

		ranked := r.Rank(articles)

		if len(ranked) != 1 {
			t.Errorf("Got %d ranked articles, want 1", len(ranked))
		}

		if ranked[0].ID != 1 {
			t.Errorf("Article ID = %d, want 1", ranked[0].ID)
		}
	})
}

func TestCalculateScore(t *testing.T) {
	tests := []struct {
		name           string
		weights        map[string]float64
		tagWeightPct   float64
		hnWeightPct    float64
		article        Article
		wantTagScore   float64
		wantHNScoreLog float64
	}{
		{
			name:         "article with matching tags",
			weights:      map[string]float64{"go": 1.0, "web": 0.5},
			tagWeightPct: 0.7,
			hnWeightPct:  0.3,
			article:      Article{ID: 1, Tags: []string{"go", "web"}, HNScore: 99},
			wantTagScore: 1.5, // 1.0 + 0.5
			// log10(99 + 1) = log10(100) = 2.0
			wantHNScoreLog: 2.0,
		},
		{
			name:         "article with no matching tags",
			weights:      map[string]float64{"go": 1.0},
			tagWeightPct: 0.7,
			hnWeightPct:  0.3,
			article:      Article{ID: 2, Tags: []string{"python", "rust"}, HNScore: 9},
			wantTagScore: 0.0,
			// log10(9 + 1) = log10(10) = 1.0
			wantHNScoreLog: 1.0,
		},
		{
			name:         "article with zero HN score",
			weights:      map[string]float64{"go": 1.0},
			tagWeightPct: 0.7,
			hnWeightPct:  0.3,
			article:      Article{ID: 3, Tags: []string{"go"}, HNScore: 0},
			wantTagScore: 1.0,
			// log10(0 + 1) = log10(1) = 0.0
			wantHNScoreLog: 0.0,
		},
		{
			name:         "article with multiple same tag",
			weights:      map[string]float64{"go": 1.0},
			tagWeightPct: 0.7,
			hnWeightPct:  0.3,
			article:      Article{ID: 4, Tags: []string{"go", "go"}, HNScore: 10},
			wantTagScore: 2.0, // Counted twice
			// log10(10 + 1) = log10(11) ≈ 1.04
			wantHNScoreLog: 1.0413926851582251,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New(tt.weights, tt.tagWeightPct, tt.hnWeightPct)
			ranked := r.calculateScore(tt.article)

			if ranked.TagScore != tt.wantTagScore {
				t.Errorf("TagScore = %v, want %v", ranked.TagScore, tt.wantTagScore)
			}

			// Use approximate comparison for floating point
			diff := ranked.HNScoreLog - tt.wantHNScoreLog
			if diff < 0 {
				diff = -diff
			}
			if diff > 0.0001 {
				t.Errorf("HNScoreLog = %v, want %v", ranked.HNScoreLog, tt.wantHNScoreLog)
			}

			// Verify final score calculation
			expectedFinal := (ranked.TagScore * tt.tagWeightPct) + (ranked.HNScoreLog * tt.hnWeightPct)
			diff = ranked.FinalScore - expectedFinal
			if diff < 0 {
				diff = -diff
			}
			if diff > 0.0001 {
				t.Errorf("FinalScore = %v, want %v", ranked.FinalScore, expectedFinal)
			}
		})
	}
}

func TestRankedArticleFields(t *testing.T) {
	ranked := RankedArticle{
		Article: Article{
			ID:      123,
			Title:   "Test Article",
			URL:     "https://example.com",
			Tags:    []string{"go", "testing"},
			HNScore: 100,
		},
		TagScore:   2.5,
		HNScoreLog: 2.0,
		FinalScore: 2.35,
	}

	if ranked.ID != 123 {
		t.Errorf("ID = %d, want 123", ranked.ID)
	}
	if ranked.Title != "Test Article" {
		t.Errorf("Title = %s, want 'Test Article'", ranked.Title)
	}
	if ranked.TagScore != 2.5 {
		t.Errorf("TagScore = %f, want 2.5", ranked.TagScore)
	}
	if ranked.FinalScore != 2.35 {
		t.Errorf("FinalScore = %f, want 2.35", ranked.FinalScore)
	}
}
