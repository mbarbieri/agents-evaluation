package ranker

import (
	"math"
	"sort"
)

type Article struct {
	ID      int64
	Title   string
	Tags    []string
	HNScore int
}

type Ranker struct {
	hnScoreWeight  float64
	tagScoreWeight float64
}

func NewRanker(hnScoreWeight, tagScoreWeight float64) *Ranker {
	return &Ranker{
		hnScoreWeight:  hnScoreWeight,
		tagScoreWeight: tagScoreWeight,
	}
}

func (r *Ranker) ScoreArticle(article Article, tagWeights map[string]float64) float64 {
	tagScore := 0.0
	for _, tag := range article.Tags {
		if weight, ok := tagWeights[tag]; ok {
			tagScore += weight
		}
	}

	hnScore := math.Log10(float64(article.HNScore + 1))

	finalScore := (tagScore * r.tagScoreWeight) + (hnScore * r.hnScoreWeight)
	return finalScore
}

func (r *Ranker) RankArticles(articles []Article, tagWeights map[string]float64) []Article {
	sorted := make([]Article, len(articles))
	copy(sorted, articles)

	sort.Slice(sorted, func(i, j int) bool {
		scoreI := r.ScoreArticle(sorted[i], tagWeights)
		scoreJ := r.ScoreArticle(sorted[j], tagWeights)
		return scoreI > scoreJ
	})

	return sorted
}

func (r *Ranker) GetTopArticles(articles []Article, tagWeights map[string]float64, limit int) []Article {
	ranked := r.RankArticles(articles, tagWeights)

	if limit > len(ranked) {
		limit = len(ranked)
	}

	return ranked[:limit]
}
