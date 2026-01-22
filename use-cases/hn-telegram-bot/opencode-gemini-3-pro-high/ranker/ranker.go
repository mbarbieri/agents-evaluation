package ranker

import (
	"hn-telegram-bot/storage"
	"math"
	"sort"
)

// Rank sorts articles based on preference weights and HN score.
// Returns a new slice of articles sorted by score descending.
func Rank(articles []storage.Article, weights map[string]float64) []storage.Article {
	// Create a wrapper to hold score to avoid re-calculating during sort
	type scoredArticle struct {
		article storage.Article
		score   float64
	}

	scored := make([]scoredArticle, len(articles))
	for i, a := range articles {
		scored[i] = scoredArticle{
			article: a,
			score:   calculateScore(a.Tags, a.Score, weights),
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	result := make([]storage.Article, len(articles))
	for i, sa := range scored {
		result[i] = sa.article
	}

	return result
}

func calculateScore(tags []string, hnScore int, weights map[string]float64) float64 {
	tagScore := 0.0
	for _, tag := range tags {
		if w, ok := weights[tag]; ok {
			tagScore += w
		}
	}

	// Log10(0) is undefined, so ensure input is at least 1 (score can be negative? HN min is 0 I think, but usually >= 1)
	// Spec says: log10(article_score + 1)
	hnComponent := math.Log10(float64(hnScore) + 1.0)

	// Final score = (tag_score × 0.7) + (hn_score × 0.3)
	return (tagScore * 0.7) + (hnComponent * 0.3)
}
