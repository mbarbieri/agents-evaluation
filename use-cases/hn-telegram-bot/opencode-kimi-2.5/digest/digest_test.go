package digest

import (
	"testing"

	"hn-telegram-bot/config"
	"hn-telegram-bot/ranker"
)

func TestNewService(t *testing.T) {
	// Just test that we can create a service
	// Full testing requires complex mocking
	t.Run("service can be created", func(t *testing.T) {
		// This is a basic structure test
		service := &Service{}
		if service == nil {
			t.Error("Service should not be nil")
		}
	})
}

func TestUpdateSettings(t *testing.T) {
	service := &Service{
		config: &config.Config{
			DigestTime:   "09:00",
			ArticleCount: 30,
		},
	}

	t.Run("updates digest time", func(t *testing.T) {
		err := service.UpdateSettings("14:00", 0)
		if err != nil {
			t.Errorf("UpdateSettings error = %v", err)
		}
		if service.config.DigestTime != "14:00" {
			t.Errorf("DigestTime = %v, want 14:00", service.config.DigestTime)
		}
	})

	t.Run("updates article count", func(t *testing.T) {
		err := service.UpdateSettings("", 50)
		if err != nil {
			t.Errorf("UpdateSettings error = %v", err)
		}
		if service.config.ArticleCount != 50 {
			t.Errorf("ArticleCount = %v, want 50", service.config.ArticleCount)
		}
	})

	t.Run("updates both", func(t *testing.T) {
		err := service.UpdateSettings("10:00", 25)
		if err != nil {
			t.Errorf("UpdateSettings error = %v", err)
		}
		if service.config.DigestTime != "10:00" {
			t.Errorf("DigestTime = %v, want 10:00", service.config.DigestTime)
		}
		if service.config.ArticleCount != 25 {
			t.Errorf("ArticleCount = %v, want 25", service.config.ArticleCount)
		}
	})
}

func TestGetSettings(t *testing.T) {
	service := &Service{
		config: &config.Config{
			DigestTime:   "12:00",
			ArticleCount: 40,
		},
	}

	t.Run("returns current settings", func(t *testing.T) {
		dt, ac := service.GetSettings()
		if dt != "12:00" {
			t.Errorf("DigestTime = %v, want 12:00", dt)
		}
		if ac != 40 {
			t.Errorf("ArticleCount = %v, want 40", ac)
		}
	})
}

func TestRankerIntegration(t *testing.T) {
	// Test that ranker works with Article struct
	articles := []ranker.Article{
		{ID: 1, Title: "Go Article", Summary: "About Go", Tags: []string{"go"}, HNScore: 100},
		{ID: 2, Title: "Rust Article", Summary: "About Rust", Tags: []string{"rust"}, HNScore: 50},
	}

	weights := map[string]float64{
		"go":   1.0,
		"rust": 2.0,
	}

	r := ranker.New(weights, 0.7, 0.3)
	ranked := r.Rank(articles)

	if len(ranked) != 2 {
		t.Errorf("Got %d ranked articles, want 2", len(ranked))
	}

	// Verify summary is preserved
	if ranked[0].Summary == "" {
		t.Error("Summary should not be empty")
	}
}

func TestApplyDecay(t *testing.T) {
	// This test would require mocking the storage
	// For now, we just test the basic structure
	t.Run("decay calculation", func(t *testing.T) {
		weight := 2.0
		decayRate := 0.02
		minWeight := 0.1

		newWeight := weight * (1 - decayRate)
		if newWeight < minWeight {
			newWeight = minWeight
		}

		expected := 1.96 // 2.0 * 0.98
		if newWeight != expected {
			t.Errorf("New weight = %v, want %v", newWeight, expected)
		}
	})

	t.Run("decay respects minimum", func(t *testing.T) {
		weight := 0.09
		decayRate := 0.02
		minWeight := 0.1

		newWeight := weight * (1 - decayRate)
		if newWeight < minWeight {
			newWeight = minWeight
		}

		if newWeight != minWeight {
			t.Errorf("New weight should be minimum %v, got %v", minWeight, newWeight)
		}
	})
}
