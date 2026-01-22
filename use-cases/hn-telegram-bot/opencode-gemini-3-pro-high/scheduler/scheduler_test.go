package scheduler

import (
	"sync"
	"testing"
)

func TestScheduler(t *testing.T) {
	s, err := New("UTC")
	if err != nil {
		t.Fatalf("Failed to create scheduler: %v", err)
	}

	// Test Start/Stop
	s.Start()
	defer s.Stop()

	// Test UpdateSchedule with immediate execution (unlikely to test exact timing reliably in unit test)
	// Instead, check if it accepts valid time and rejects invalid.

	var wg sync.WaitGroup
	// wg.Add(1) // Not waiting in this test

	job := func() {
		wg.Done()
	}

	// This adds a schedule. We can't easily wait for a daily job in a test.
	// But we can check if it parses correctly.
	err = s.UpdateSchedule("10:00", job)
	if err != nil {
		t.Errorf("UpdateSchedule failed for valid time: %v", err)
	}

	err = s.UpdateSchedule("25:00", job)
	if err == nil {
		t.Error("Expected error for invalid time 25:00")
	}

	err = s.UpdateSchedule("invalid", job)
	if err == nil {
		t.Error("Expected error for invalid string")
	}
}

func TestTimezone(t *testing.T) {
	_, err := New("Invalid/Timezone")
	if err == nil {
		t.Error("Expected error for invalid timezone")
	}

	s, err := New("America/New_York")
	if err != nil {
		t.Fatalf("Failed to load valid timezone: %v", err)
	}
	s.Stop()
}
