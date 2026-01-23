package ranker

import "testing"

func TestScoreAndSort(t *testing.T) {
	t.Parallel()
	articles := []Article{
		{ID: 1, Score: 10, Tags: []string{"go"}},
		{ID: 2, Score: 100, Tags: []string{"ai"}},
	}
	weights := map[string]float64{"go": 2.0, "ai": 0.5}
	Rank(articles, weights)
	if articles[0].ID != 1 {
		t.Fatalf("expected article 1 to rank first")
	}
}
