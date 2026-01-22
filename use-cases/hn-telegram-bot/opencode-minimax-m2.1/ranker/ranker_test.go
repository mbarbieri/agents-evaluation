package ranker

import (
	"math"
	"testing"
)

func TestRanker_Rank_SortsByScore(t *testing.T) {
	r := NewRanker()

	articles := []Article{
		{ID: 1, Title: "Article 1", Score: 100, Tags: []string{"go"}},
		{ID: 2, Title: "Article 2", Score: 500, Tags: []string{"rust"}},
		{ID: 3, Title: "Article 3", Score: 50, Tags: []string{"python"}},
	}

	tagWeights := map[string]float64{
		"go":     1.0,
		"rust":   1.0,
		"python": 1.0,
	}

	ranked := r.Rank(articles, tagWeights)

	if ranked[0].ID != 2 {
		t.Errorf("Expected article 2 first (highest score), got %d", ranked[0].ID)
	}
	if ranked[1].ID != 1 {
		t.Errorf("Expected article 1 second, got %d", ranked[1].ID)
	}
	if ranked[2].ID != 3 {
		t.Errorf("Expected article 3 third, got %d", ranked[2].ID)
	}
}

func TestRanker_Rank_PrefersLikedTags(t *testing.T) {
	r := NewRanker()

	articles := []Article{
		{ID: 1, Title: "Article 1", Score: 100, Tags: []string{"go"}},
		{ID: 2, Title: "Article 2", Score: 100, Tags: []string{"rust"}},
	}

	tagWeights := map[string]float64{
		"go":   3.0,
		"rust": 1.0,
	}

	ranked := r.Rank(articles, tagWeights)

	if ranked[0].ID != 1 {
		t.Errorf("Expected article 1 first (higher tag weight), got %d", ranked[0].ID)
	}
}

func TestRanker_Rank_EmptyArticles(t *testing.T) {
	r := NewRanker()

	articles := []Article{}
	tagWeights := map[string]float64{}

	ranked := r.Rank(articles, tagWeights)

	if len(ranked) != 0 {
		t.Errorf("Expected 0 articles, got %d", len(ranked))
	}
}

func TestRanker_Rank_NoTagWeights(t *testing.T) {
	r := NewRanker()

	articles := []Article{
		{ID: 1, Title: "Article 1", Score: 100, Tags: []string{"go"}},
		{ID: 2, Title: "Article 2", Score: 50, Tags: []string{"rust"}},
	}

	ranked := r.Rank(articles, nil)

	if ranked[0].ID != 1 {
		t.Errorf("Expected article 1 first (higher HN score), got %d", ranked[0].ID)
	}
}

func TestRanker_ParseTags_ValidJSON(t *testing.T) {
	r := NewRanker()

	tags, err := r.ParseTags(`["go", "programming", "api"]`)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(tags) != 3 {
		t.Errorf("Expected 3 tags, got %d", len(tags))
	}
	if tags[0] != "go" {
		t.Errorf("Expected first tag 'go', got '%s'", tags[0])
	}
}

func TestRanker_ParseTags_Empty(t *testing.T) {
	r := NewRanker()

	tags, err := r.ParseTags("")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(tags) != 0 {
		t.Errorf("Expected 0 tags, got %d", len(tags))
	}
}

func TestRanker_ParseTags_InvalidJSON(t *testing.T) {
	r := NewRanker()

	_, err := r.ParseTags("not valid json")
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestRanker_BoostTags(t *testing.T) {
	r := NewRanker()

	currentWeights := map[string]float64{
		"go": 1.0,
	}

	boosted := r.BoostTags(currentWeights, []string{"go", "rust"}, 0.2)

	if boosted["go"] != 1.2 {
		t.Errorf("Expected 'go' weight 1.2, got %f", boosted["go"])
	}
	if boosted["rust"] != 0.2 {
		t.Errorf("Expected 'rust' weight 0.2, got %f", boosted["rust"])
	}
}

func TestRanker_BoostTags_NilMap(t *testing.T) {
	r := NewRanker()

	boosted := r.BoostTags(nil, []string{"go"}, 0.5)

	if boosted["go"] != 0.5 {
		t.Errorf("Expected 'go' weight 0.5, got %f", boosted["go"])
	}
}

func TestRanker_CalculateHNScore(t *testing.T) {
	r := NewRanker()

	tests := []struct {
		score    int
		expected float64
	}{
		{0, 0},
		{1, 0.3010},
		{10, 1.0414},
		{100, 2.0043},
		{1000, 3.0},
	}

	for _, tc := range tests {
		result := r.calculateHNScore(tc.score)
		if math.Abs(result-tc.expected) > 0.01 {
			t.Errorf("For score %d: expected %f, got %f", tc.score, tc.expected, result)
		}
	}
}

func TestRanker_Rank_UnknownTagsIgnored(t *testing.T) {
	r := NewRanker()

	articles := []Article{
		{ID: 1, Title: "Article 1", Score: 100, Tags: []string{"go"}},
		{ID: 2, Title: "Article 2", Score: 100, Tags: []string{"rust"}},
	}

	tagWeights := map[string]float64{
		"go": 2.0,
	}

	ranked := r.Rank(articles, tagWeights)

	if ranked[0].ID != 1 {
		t.Errorf("Expected article 1 first (known tag), got %d", ranked[0].ID)
	}
	if ranked[1].ID != 2 {
		t.Errorf("Expected article 2 second (unknown tag), got %d", ranked[1].ID)
	}
}
