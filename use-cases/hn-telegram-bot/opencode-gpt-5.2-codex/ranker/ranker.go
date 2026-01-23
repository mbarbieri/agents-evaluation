package ranker

import (
	"math"
	"sort"
)

type Article struct {
	ID    int64
	Tags  []string
	Score int
}

func Rank(articles []Article, weights map[string]float64) {
	if len(articles) == 0 {
		return
	}
	if weights == nil {
		weights = map[string]float64{}
	}
	scores := make(map[int64]float64, len(articles))
	for _, article := range articles {
		scores[article.ID] = score(article, weights)
	}
	sort.SliceStable(articles, func(i, j int) bool {
		return scores[articles[i].ID] > scores[articles[j].ID]
	})
}

func score(article Article, weights map[string]float64) float64 {
	tagScore := 0.0
	for _, tag := range article.Tags {
		tagScore += weights[tag]
	}
	hnScore := math.Log10(float64(article.Score) + 1)
	return tagScore*0.7 + hnScore*0.3
}
