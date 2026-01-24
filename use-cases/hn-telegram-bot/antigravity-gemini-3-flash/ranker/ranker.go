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
	Article
	FinalScore float64
}

type Ranker struct {
	tagWeights map[string]float64
}

func NewRanker(tagWeights map[string]float64) *Ranker {
	return &Ranker{tagWeights: tagWeights}
}

func (r *Ranker) Score(tags []string, hnScore int) float64 {
	tagScore := 0.0
	for _, tag := range tags {
		if w, ok := r.tagWeights[tag]; ok {
			tagScore += w
		}
	}

	hnScoreComp := math.Log10(float64(hnScore + 1))

	return (tagScore * 0.7) + (hnScoreComp * 0.3)
}

func (r *Ranker) Rank(articles []Article) []RankedArticle {
	ranked := make([]RankedArticle, len(articles))
	for i, a := range articles {
		ranked[i] = RankedArticle{
			Article:    a,
			FinalScore: r.Score(a.Tags, a.HNScore),
		}
	}

	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].FinalScore > ranked[j].FinalScore
	})

	return ranked
}
