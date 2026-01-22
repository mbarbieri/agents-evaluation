package ranker

import (
	"testing"

	"github.com/opencode/hn-telegram-bot/storage"
	"github.com/stretchr/testify/assert"
)

func TestRank(t *testing.T) {
	articles := []*storage.Article{
		{ID: 1, Title: "A", Tags: []string{"go", "rust"}, Score: 10},
		{ID: 2, Title: "B", Tags: []string{"java"}, Score: 100},
		{ID: 3, Title: "C", Tags: []string{"go"}, Score: 50},
	}

	tagWeights := map[string]float64{
		"go":   2.0,
		"rust": 1.5,
		"java": 1.0,
	}

	ranked := Rank(articles, tagWeights)

	assert.Len(t, ranked, 3)
	assert.Equal(t, int64(1), ranked[0].ID) // (2.0+1.5)*0.7 + log10(11)*0.3 = 3.5*0.7 + 1.04*0.3 = 2.45 + 0.312 = 2.762
	assert.Equal(t, int64(3), ranked[1].ID) // (2.0)*0.7 + log10(51)*0.3 = 1.4 + 1.70*0.3 = 1.4 + 0.51 = 1.91
	assert.Equal(t, int64(2), ranked[2].ID) // (1.0)*0.7 + log10(101)*0.3 = 0.7 + 2.0*0.3 = 0.7 + 0.6 = 1.3
}
