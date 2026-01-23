package ranker

import (
	"context"
	"errors"
	"testing"
)

type fakeWeights struct {
	weights map[string]float64
	err     error
}

func (f fakeWeights) GetTagWeights(ctx context.Context, tags []string) (map[string]float64, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := map[string]float64{}
	for k, v := range f.weights {
		out[k] = v
	}
	return out, nil
}

func TestRanker_RanksByFinalScore(t *testing.T) {
	t.Parallel()
	r := Ranker{Weights: fakeWeights{weights: map[string]float64{"go": 3, "rust": 1}}}
	ctx := context.Background()

	articles := []Article{
		{ID: 1, Tags: []string{"rust"}, HNScore: 100},
		{ID: 2, Tags: []string{"go"}, HNScore: 10},
	}
	got, err := r.Rank(ctx, articles)
	if err != nil {
		t.Fatalf("Rank: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
	if got[0].ID != 2 {
		t.Fatalf("expected go article first, got %d", got[0].ID)
	}
	if got[0].FinalScore <= got[1].FinalScore {
		t.Fatalf("expected score descending")
	}
}

func TestRanker_DefaultWeightIsOne(t *testing.T) {
	t.Parallel()
	r := Ranker{Weights: fakeWeights{weights: map[string]float64{}}}
	ctx := context.Background()

	articles := []Article{{ID: 1, Tags: []string{"unknown"}, HNScore: 0}}
	got, err := r.Rank(ctx, articles)
	if err != nil {
		t.Fatalf("Rank: %v", err)
	}
	if got[0].FinalScore == 0 {
		t.Fatalf("expected non-zero final score")
	}
}

func TestRanker_PropagatesWeightErrors(t *testing.T) {
	t.Parallel()
	exp := errors.New("boom")
	r := Ranker{Weights: fakeWeights{err: exp}}
	_, err := r.Rank(context.Background(), []Article{{ID: 1, Tags: []string{"go"}, HNScore: 1}})
	if !errors.Is(err, exp) {
		t.Fatalf("expected boom, got %v", err)
	}
}
