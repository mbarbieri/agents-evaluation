package ranker

import (
	"math"
	"sort"
)

// Article represents an article for ranking
type Article struct {
	Tags    []string
	HNScore int
}

// RankedArticle represents an article with its calculated score
type RankedArticle struct {
	Article
	Score float64
}

// calculateScore computes the score for an article
func calculateScore(tags []string, hnScore int, tagWeights map[string]float64) float64 {
	tagScore := 0.0
	for _, tag := range tags {
		if weight, ok := tagWeights[tag]; ok {
			tagScore += weight
		}
	}

	hnScoreFloat := math.Log10(float64(hnScore) + 1)

	return tagScore*0.7 + hnScoreFloat*0.3
}

// Rank sorts articles by their calculated scores in descending order
func Rank(articles []Article, tagWeights map[string]float64) []RankedArticle {
	ranked := make([]RankedArticle, len(articles))
	for i, article := range articles {
		score := calculateScore(article.Tags, article.HNScore, tagWeights)
		ranked[i] = RankedArticle{
			Article: article,
			Score:   score,
		}
	}

	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].Score > ranked[j].Score
	})

	return ranked
}
