package ranker

import (
	"math"
	"sort"
)

type Article struct {
	ID      int
	Tags    []string
	HNScore int
}

type RankedArticle struct {
	Article Article
	Score   float64
}

type Ranker interface {
	Rank(articles []Article, tagWeights map[string]float64) []RankedArticle
}

type WeightedRanker struct{}

func New() *WeightedRanker {
	return &WeightedRanker{}
}

func (r *WeightedRanker) Rank(articles []Article, tagWeights map[string]float64) []RankedArticle {
	ranked := make([]RankedArticle, 0, len(articles))

	for _, article := range articles {
		score := calculateScore(article, tagWeights)
		ranked = append(ranked, RankedArticle{
			Article: article,
			Score:   score,
		})
	}

	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].Score > ranked[j].Score
	})

	return ranked
}

func calculateScore(article Article, tagWeights map[string]float64) float64 {
	// Calculate tag score
	tagScore := 0.0
	for _, tag := range article.Tags {
		weight, exists := tagWeights[tag]
		if !exists {
			weight = 1.0 // Default weight for unknown tags
		}
		tagScore += weight
	}

	// Calculate HN score component
	hnScore := math.Log10(float64(article.HNScore) + 1.0)

	// Blended score: 70% tag weights + 30% HN score
	return tagScore*0.7 + hnScore*0.3
}
