package ranker

import (
	"math"
	"sort"
)

// Article holds the data needed for ranking.
type Article struct {
	ID      int
	Tags    []string
	HNScore int
	// Final computed score, set by Rank.
	Score float64
}

// TagWeightMap maps tag names to their learned weights.
type TagWeightMap map[string]float64

// Rank scores and sorts articles by blended preference score.
// Formula: score = (tag_score * 0.7) + (hn_score * 0.3)
// where tag_score = sum of weights for all article tags
// and hn_score = log10(article_score + 1)
func Rank(articles []Article, weights TagWeightMap) []Article {
	for i := range articles {
		tagScore := 0.0
		for _, tag := range articles[i].Tags {
			if w, ok := weights[tag]; ok {
				tagScore += w
			}
		}
		hnScore := math.Log10(float64(articles[i].HNScore) + 1)
		articles[i].Score = (tagScore * 0.7) + (hnScore * 0.3)
	}

	sort.Slice(articles, func(i, j int) bool {
		return articles[i].Score > articles[j].Score
	})

	return articles
}
