package ranker

import (
	"math"
	"sort"

	"hn-telegram-bot/model"
)

// ScoredArticle wraps an article with scoring details.
type ScoredArticle struct {
	Article  model.Article
	Score    float64
	TagScore float64
	HNScore  float64
}

// Rank scores and sorts articles by relevance.
func Rank(articles []model.Article, weights map[string]model.TagWeight) []ScoredArticle {
	if len(articles) == 0 {
		return nil
	}
	scored := make([]ScoredArticle, 0, len(articles))
	for _, article := range articles {
		tagScore := 0.0
		for _, tag := range article.Tags {
			if weight, ok := weights[tag]; ok {
				tagScore += weight.Weight
			}
		}
		hnScore := math.Log10(float64(article.HNScore) + 1)
		final := tagScore*0.7 + hnScore*0.3
		scored = append(scored, ScoredArticle{Article: article, Score: final, TagScore: tagScore, HNScore: hnScore})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})
	return scored
}
