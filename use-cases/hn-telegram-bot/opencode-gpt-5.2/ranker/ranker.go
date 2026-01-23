package ranker

import (
	"context"
	"math"
	"sort"
)

type WeightStore interface {
	GetTagWeights(ctx context.Context, tags []string) (map[string]float64, error)
}

type Article struct {
	ID         int
	Tags       []string
	HNScore    int
	FinalScore float64
}

type Ranker struct {
	Weights WeightStore
}

func (r Ranker) Rank(ctx context.Context, articles []Article) ([]Article, error) {
	if len(articles) == 0 {
		return nil, nil
	}

	// Collect unique tags.
	tagSet := map[string]struct{}{}
	for _, a := range articles {
		for _, t := range a.Tags {
			if t != "" {
				tagSet[t] = struct{}{}
			}
		}
	}
	var allTags []string
	for t := range tagSet {
		allTags = append(allTags, t)
	}

	weights, err := r.Weights.GetTagWeights(ctx, allTags)
	if err != nil {
		return nil, err
	}

	out := make([]Article, 0, len(articles))
	for _, a := range articles {
		tagScore := 0.0
		for _, t := range a.Tags {
			if t == "" {
				continue
			}
			w, ok := weights[t]
			if !ok {
				w = 1.0
			}
			tagScore += w
		}
		hnScore := math.Log10(float64(a.HNScore) + 1)
		a.FinalScore = tagScore*0.7 + hnScore*0.3
		out = append(out, a)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].FinalScore == out[j].FinalScore {
			return out[i].ID < out[j].ID
		}
		return out[i].FinalScore > out[j].FinalScore
	})
	return out, nil
}
