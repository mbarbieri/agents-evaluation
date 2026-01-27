package ranker

import (
	"math"
	"sort"
)

// Article represents an article with its tags and score for ranking
type Article struct {
	ID      int64
	Title   string
	URL     string
	Summary string
	Tags    []string
	HNScore int
}

// RankedArticle represents an article with its computed rank score
type RankedArticle struct {
	Article
	TagScore   float64
	HNScoreLog float64
	FinalScore float64
}

// Ranker calculates article scores based on learned preferences and HN score
type Ranker struct {
	tagWeights   map[string]float64
	tagWeightPct float64
	hnWeightPct  float64
}

// New creates a new Ranker with the specified tag weights and scoring percentages
func New(tagWeights map[string]float64, tagWeightPct, hnWeightPct float64) *Ranker {
	if tagWeights == nil {
		tagWeights = make(map[string]float64)
	}
	if tagWeightPct == 0 {
		tagWeightPct = 0.7
	}
	if hnWeightPct == 0 {
		hnWeightPct = 0.3
	}

	return &Ranker{
		tagWeights:   tagWeights,
		tagWeightPct: tagWeightPct,
		hnWeightPct:  hnWeightPct,
	}
}

// Rank calculates scores for all articles and returns them sorted by final score
func (r *Ranker) Rank(articles []Article) []RankedArticle {
	ranked := make([]RankedArticle, 0, len(articles))

	for _, article := range articles {
		ranked = append(ranked, r.calculateScore(article))
	}

	// Sort by final score descending
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].FinalScore > ranked[j].FinalScore
	})

	return ranked
}

// calculateScore computes the final score for a single article
func (r *Ranker) calculateScore(article Article) RankedArticle {
	// Calculate tag score: sum of weights for all article tags
	tagScore := 0.0
	for _, tag := range article.Tags {
		if weight, ok := r.tagWeights[tag]; ok {
			tagScore += weight
		}
	}

	// Calculate HN score component: log10(article_score + 1)
	hnScoreLog := math.Log10(float64(article.HNScore) + 1)

	// Final score = (tag_score × tagWeightPct) + (hn_score × hnWeightPct)
	finalScore := (tagScore * r.tagWeightPct) + (hnScoreLog * r.hnWeightPct)

	return RankedArticle{
		Article:    article,
		TagScore:   tagScore,
		HNScoreLog: hnScoreLog,
		FinalScore: finalScore,
	}
}
