package ranker

import (
	"math"
	"sort"
)

// RankableArticle contains the data needed for ranking.
type RankableArticle struct {
	ID      int64
	Tags    []string
	HNScore int
}

// RankedArticle contains an article with its computed scores.
type RankedArticle struct {
	RankableArticle
	TagScore         float64
	HNScoreComponent float64
	FinalScore       float64
}

// Ranker scores and ranks articles based on learned preferences.
type Ranker struct {
	tagWeight float64
	hnWeight  float64
}

// NewRanker creates a ranker with the given weighting factors.
func NewRanker(tagWeight, hnWeight float64) *Ranker {
	return &Ranker{
		tagWeight: tagWeight,
		hnWeight:  hnWeight,
	}
}

// Rank scores and sorts articles by their computed final score.
func (r *Ranker) Rank(articles []RankableArticle, weights map[string]float64) []RankedArticle {
	if len(articles) == 0 {
		return nil
	}

	if weights == nil {
		weights = make(map[string]float64)
	}

	ranked := make([]RankedArticle, len(articles))
	for i, article := range articles {
		tagScore := r.calculateTagScore(article.Tags, weights)
		hnScore := r.calculateHNScore(article.HNScore)
		finalScore := tagScore*r.tagWeight + hnScore*r.hnWeight

		ranked[i] = RankedArticle{
			RankableArticle:  article,
			TagScore:         tagScore,
			HNScoreComponent: hnScore,
			FinalScore:       finalScore,
		}
	}

	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].FinalScore > ranked[j].FinalScore
	})

	return ranked
}

func (r *Ranker) calculateTagScore(tags []string, weights map[string]float64) float64 {
	var score float64
	for _, tag := range tags {
		if w, ok := weights[tag]; ok {
			score += w
		} else {
			score += 1.0 // Default weight for unknown tags
		}
	}
	return score
}

func (r *Ranker) calculateHNScore(score int) float64 {
	// log10(score + 1) to handle score of 0
	return math.Log10(float64(score) + 1)
}
