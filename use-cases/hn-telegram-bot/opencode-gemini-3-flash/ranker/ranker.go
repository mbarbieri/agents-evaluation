package ranker

import (
	"math"
	"sort"

	"github.com/opencode/hn-telegram-bot/storage"
)

func Rank(articles []*storage.Article, tagWeights map[string]float64) []*storage.Article {
	type scoredArticle struct {
		article *storage.Article
		score   float64
	}

	scored := make([]scoredArticle, len(articles))
	for i, a := range articles {
		tagScore := 0.0
		for _, tag := range a.Tags {
			if weight, ok := tagWeights[tag]; ok {
				tagScore += weight
			} else {
				tagScore += 1.0 // Default weight
			}
		}

		hnScore := math.Log10(float64(a.Score) + 1.0)
		finalScore := (tagScore * 0.7) + (hnScore * 0.3)
		scored[i] = scoredArticle{article: a, score: finalScore}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	result := make([]*storage.Article, len(articles))
	for i, s := range scored {
		result[i] = s.article
	}
	return result
}
