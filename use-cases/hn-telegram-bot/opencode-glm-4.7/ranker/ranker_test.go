package ranker

import (
	"math"
	"testing"
)

func TestScoreArticle(t *testing.T) {
	tests := []struct {
		name           string
		article        Article
		tagWeights     map[string]float64
		hnScoreWeight  float64
		tagScoreWeight float64
		expectedScore  float64
	}{
		{
			name: "article with matching tags",
			article: Article{
				Tags:    []string{"rust", "programming"},
				HNScore: 100,
			},
			tagWeights:     map[string]float64{"rust": 2.0, "programming": 1.5, "go": 1.0},
			hnScoreWeight:  0.3,
			tagScoreWeight: 0.7,
			expectedScore:  (2.0+1.5)*0.7 + math.Log10(float64(100+1))*0.3,
		},
		{
			name: "article with no matching tags",
			article: Article{
				Tags:    []string{"python"},
				HNScore: 100,
			},
			tagWeights:     map[string]float64{"rust": 2.0, "go": 1.0},
			hnScoreWeight:  0.3,
			tagScoreWeight: 0.7,
			expectedScore:  0*0.7 + math.Log10(float64(100+1))*0.3,
		},
		{
			name: "article with all tags matching",
			article: Article{
				Tags:    []string{"rust", "go", "programming"},
				HNScore: 50,
			},
			tagWeights:     map[string]float64{"rust": 1.0, "go": 1.0, "programming": 1.0},
			hnScoreWeight:  0.3,
			tagScoreWeight: 0.7,
			expectedScore:  (1.0+1.0+1.0)*0.7 + math.Log10(float64(50+1))*0.3,
		},
		{
			name: "article with zero HN score",
			article: Article{
				Tags:    []string{"rust"},
				HNScore: 0,
			},
			tagWeights:     map[string]float64{"rust": 2.0},
			hnScoreWeight:  0.3,
			tagScoreWeight: 0.7,
			expectedScore:  2.0*0.7 + math.Log10(float64(0+1))*0.3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ranker := NewRanker(tt.hnScoreWeight, tt.tagScoreWeight)
			score := ranker.ScoreArticle(tt.article, tt.tagWeights)

			diff := math.Abs(score - tt.expectedScore)
			if diff > 0.0001 {
				t.Errorf("score = %v, want %v (diff = %v)", score, tt.expectedScore, diff)
			}
		})
	}
}

func TestRankArticles(t *testing.T) {
	ranker := NewRanker(0.3, 0.7)

	articles := []Article{
		{
			ID:      1,
			Title:   "Low HN score, high tag match",
			Tags:    []string{"rust"},
			HNScore: 10,
		},
		{
			ID:      2,
			Title:   "High HN score, low tag match",
			Tags:    []string{"other"},
			HNScore: 200,
		},
		{
			ID:      3,
			Title:   "Medium both",
			Tags:    []string{"go"},
			HNScore: 100,
		},
	}

	tagWeights := map[string]float64{
		"rust":  5.0,
		"go":    2.0,
		"other": 0.5,
	}

	ranked := ranker.RankArticles(articles, tagWeights)

	if len(ranked) != 3 {
		t.Errorf("got %d ranked articles, want 3", len(ranked))
	}

	if ranked[0].ID != 1 {
		t.Errorf("first article ID = %v, want 1", ranked[0].ID)
	}

	if ranked[1].ID != 3 {
		t.Errorf("second article ID = %v, want 3", ranked[1].ID)
	}

	if ranked[2].ID != 2 {
		t.Errorf("third article ID = %v, want 2", ranked[2].ID)
	}
}

func TestRankArticlesEmpty(t *testing.T) {
	ranker := NewRanker(0.3, 0.7)
	tagWeights := map[string]float64{"rust": 2.0}

	ranked := ranker.RankArticles([]Article{}, tagWeights)

	if len(ranked) != 0 {
		t.Errorf("got %d ranked articles, want 0", len(ranked))
	}
}

func TestRankArticlesSingle(t *testing.T) {
	ranker := NewRanker(0.3, 0.7)

	articles := []Article{
		{
			ID:      1,
			Title:   "Single article",
			Tags:    []string{"rust"},
			HNScore: 100,
		},
	}

	tagWeights := map[string]float64{"rust": 2.0}

	ranked := ranker.RankArticles(articles, tagWeights)

	if len(ranked) != 1 {
		t.Errorf("got %d ranked articles, want 1", len(ranked))
	}

	if ranked[0].ID != 1 {
		t.Errorf("article ID = %v, want 1", ranked[0].ID)
	}
}

func TestGetTopArticles(t *testing.T) {
	ranker := NewRanker(0.3, 0.7)

	articles := []Article{
		{ID: 1, Title: "First", Tags: []string{"rust"}, HNScore: 10},
		{ID: 2, Title: "Second", Tags: []string{"rust"}, HNScore: 20},
		{ID: 3, Title: "Third", Tags: []string{"rust"}, HNScore: 30},
		{ID: 4, Title: "Fourth", Tags: []string{"rust"}, HNScore: 40},
		{ID: 5, Title: "Fifth", Tags: []string{"rust"}, HNScore: 50},
	}

	tagWeights := map[string]float64{"rust": 1.0}

	top := ranker.GetTopArticles(articles, tagWeights, 3)

	if len(top) != 3 {
		t.Errorf("got %d top articles, want 3", len(top))
	}

	if top[0].ID != 5 {
		t.Errorf("first article ID = %v, want 5", top[0].ID)
	}

	if top[1].ID != 4 {
		t.Errorf("second article ID = %v, want 4", top[1].ID)
	}

	if top[2].ID != 3 {
		t.Errorf("third article ID = %v, want 3", top[2].ID)
	}
}

func TestGetTopArticlesLimitGreaterThanAvailable(t *testing.T) {
	ranker := NewRanker(0.3, 0.7)

	articles := []Article{
		{ID: 1, Title: "First", Tags: []string{"rust"}, HNScore: 10},
		{ID: 2, Title: "Second", Tags: []string{"rust"}, HNScore: 20},
	}

	tagWeights := map[string]float64{"rust": 1.0}

	top := ranker.GetTopArticles(articles, tagWeights, 10)

	if len(top) != 2 {
		t.Errorf("got %d top articles, want 2", len(top))
	}
}

func TestGetTopArticlesZeroLimit(t *testing.T) {
	ranker := NewRanker(0.3, 0.7)

	articles := []Article{
		{ID: 1, Title: "First", Tags: []string{"rust"}, HNScore: 10},
	}

	tagWeights := map[string]float64{"rust": 1.0}

	top := ranker.GetTopArticles(articles, tagWeights, 0)

	if len(top) != 0 {
		t.Errorf("got %d top articles, want 0", len(top))
	}
}
