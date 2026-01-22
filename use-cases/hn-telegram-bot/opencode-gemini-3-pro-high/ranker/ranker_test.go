package ranker

import (
	"hn-telegram-bot/storage"
	"math"
	"testing"
)

func TestRankArticles(t *testing.T) {
	articles := []storage.Article{
		{ID: 1, Title: "Low Score High Tag", Tags: []string{"fav"}, Score: 10},
		{ID: 2, Title: "High Score Low Tag", Tags: []string{"meh"}, Score: 1000},
		{ID: 3, Title: "No Tags", Tags: []string{}, Score: 50},
	}

	weights := map[string]float64{
		"fav": 5.0,
		"meh": 0.1,
	}

	ranked := Rank(articles, weights)

	if len(ranked) != 3 {
		t.Fatalf("Expected 3 ranked articles, got %d", len(ranked))
	}

	// Calculate expected scores
	// Art 1: Tag=5.0. HN=log10(11)=1.04. Final = 5.0*0.7 + 1.04*0.3 = 3.5 + 0.312 = 3.812
	// Art 2: Tag=0.1. HN=log10(1001)=3.0. Final = 0.1*0.7 + 3.0*0.3 = 0.07 + 0.9 = 0.97
	// Art 3: Tag=0. HN=log10(51)=1.7. Final = 0 + 0.51 = 0.51

	// Order should be 1, 2, 3
	if ranked[0].ID != 1 {
		t.Errorf("Expected ID 1 first, got %d", ranked[0].ID)
	}
	if ranked[1].ID != 2 {
		t.Errorf("Expected ID 2 second, got %d", ranked[1].ID)
	}
	if ranked[2].ID != 3 {
		t.Errorf("Expected ID 3 third, got %d", ranked[2].ID)
	}
}

func TestScoreCalculation(t *testing.T) {
	// Unit test specific formula logic if needed, but integration test above covers it.
	score := calculateScore([]string{"test"}, 100, map[string]float64{"test": 2.0})

	// Tag=2.0. HN=log10(101)~=2.0043
	// 2.0*0.7 + 2.0043*0.3 = 1.4 + 0.60129 = 2.00129

	expected := 2.0*0.7 + math.Log10(101)*0.3
	if math.Abs(score-expected) > 0.0001 {
		t.Errorf("Score calc error. Got %f, expected %f", score, expected)
	}
}
