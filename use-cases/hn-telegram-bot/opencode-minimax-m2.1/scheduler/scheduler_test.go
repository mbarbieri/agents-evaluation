package scheduler

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestScheduler_NewScheduler_ValidTimezone(t *testing.T) {
	s, err := NewScheduler("America/New_York")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer s.Stop()

	if s.location.String() != "America/New_York" {
		t.Errorf("Expected timezone America/New_York, got %s", s.location.String())
	}
}

func TestScheduler_NewScheduler_InvalidTimezone(t *testing.T) {
	_, err := NewScheduler("Invalid/Timezone")
	if err == nil {
		t.Error("Expected error for invalid timezone")
	}
}

func TestScheduler_Schedule(t *testing.T) {
	s, err := NewScheduler("UTC")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer s.Stop()

	nextMinute := time.Now().Add(2 * time.Minute)
	digestTime := fmt.Sprintf("%02d:%02d", nextMinute.Hour(), nextMinute.Minute())

	err = s.Schedule(digestTime, func() {})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	s.Start()

	time.Sleep(100 * time.Millisecond)

	nextRun := s.NextRun()
	if nextRun.IsZero() {
		t.Error("Expected non-zero next run time")
	}

	if nextRun.Before(time.Now()) {
		t.Error("Next run time should be in the future")
	}

	s.Stop()
}

func TestScheduler_UpdateSchedule(t *testing.T) {
	s, err := NewScheduler("UTC")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer s.Stop()

	executed := false
	var mu sync.Mutex

	err = s.Schedule("09:00", func() {
		mu.Lock()
		executed = true
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	s.Start()

	err = s.UpdateSchedule("10:00", func() {})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	mu.Lock()
	wasExecuted := executed
	mu.Unlock()

	if wasExecuted {
		t.Error("Function should not have been executed with old schedule")
	}
}

func TestScheduler_NextRun_NoJobs(t *testing.T) {
	s, err := NewScheduler("UTC")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer s.Stop()

	next := s.NextRun()
	if !next.IsZero() {
		t.Error("Expected zero time when no jobs scheduled")
	}
}

func TestScheduler_Schedule_InvalidTime(t *testing.T) {
	s, err := NewScheduler("UTC")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer s.Stop()

	testCases := []string{
		"9:00",
		"0900",
		"24:00",
		"09:60",
		"",
	}

	for _, tc := range testCases {
		err = s.Schedule(tc, func() {})
		if err == nil {
			t.Errorf("Expected error for invalid time: %s", tc)
		}
	}
}

func TestConvertToCron_Valid(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"09:00", "0 9 * * *"},
		{"00:00", "0 0 * * *"},
		{"23:59", "59 23 * * *"},
	}

	for _, tc := range tests {
		result := convertToCron(tc.input)
		if result != tc.expected {
			t.Errorf("convertToCron(%s): expected %s, got %s", tc.input, tc.expected, result)
		}
	}
}

func TestConvertToCron_Invalid(t *testing.T) {
	tests := []string{
		"9:00",
		"0900",
		"24:00",
		"09:60",
		"",
		"abc",
	}

	for _, input := range tests {
		result := convertToCron(input)
		if result != "" {
			t.Errorf("convertToCron(%s): expected empty string, got %s", input, result)
		}
	}
}
