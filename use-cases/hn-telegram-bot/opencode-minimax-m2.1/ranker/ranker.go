package ranker

import (
	"encoding/json"
	"math"
	"sort"
)

type Article struct {
	ID        int64
	Title     string
	URL       string
	Summary   string
	Tags      []string
	Score     int
	HNContent float64
}

type Ranker struct{}

func NewRanker() *Ranker {
	return &Ranker{}
}

func (r *Ranker) Rank(articles []Article, tagWeights map[string]float64) []Article {
	for i := range articles {
		articles[i].HNContent = r.calculateHNScore(articles[i].Score)
	}

	sort.Slice(articles, func(i, j int) bool {
		scoreI := r.calculateFinalScore(articles[i], tagWeights)
		scoreJ := r.calculateFinalScore(articles[j], tagWeights)
		if scoreI != scoreJ {
			return scoreI > scoreJ
		}
		return articles[i].Score > articles[j].Score
	})

	return articles
}

func (r *Ranker) calculateHNScore(score int) float64 {
	if score <= 0 {
		return 0
	}
	return math.Log10(float64(score) + 1)
}

func (r *Ranker) calculateFinalScore(article Article, tagWeights map[string]float64) float64 {
	tagScore := r.calculateTagScore(article.Tags, tagWeights)
	hnScore := article.HNContent

	return tagScore*0.7 + hnScore*0.3
}

func (r *Ranker) calculateTagScore(tags []string, weights map[string]float64) float64 {
	var total float64
	for _, tag := range tags {
		if weight, ok := weights[tag]; ok {
			total += weight
		}
	}
	return total
}

func (r *Ranker) ParseTags(tagsJSON string) ([]string, error) {
	if tagsJSON == "" {
		return []string{}, nil
	}

	var tags []string
	if err := json.Unmarshal([]byte(tagsJSON), &tags); err != nil {
		return nil, err
	}

	return tags, nil
}

func (r *Ranker) BoostTags(currentTags map[string]float64, tagsToBoost []string, boostAmount float64) map[string]float64 {
	if currentTags == nil {
		currentTags = make(map[string]float64)
	}

	for _, tag := range tagsToBoost {
		currentTags[tag] += boostAmount
	}

	return currentTags
}
