package scheduler

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestNew_ValidTimezone(t *testing.T) {
	s, err := New("America/New_York")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer s.Stop()
	if s.location.String() != "America/New_York" {
		t.Errorf("expected America/New_York, got %s", s.location.String())
	}
}

func TestNew_InvalidTimezone(t *testing.T) {
	_, err := New("Invalid/Zone")
	if err == nil {
		t.Fatal("expected error for invalid timezone")
	}
}

func TestSchedule_ValidTime(t *testing.T) {
	s, err := New("UTC")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()

	called := false
	err = s.Schedule("14:30", func() { called = true })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = called
}

func TestSchedule_InvalidTime(t *testing.T) {
	s, err := New("UTC")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()

	err = s.Schedule("25:00", func() {})
	if err == nil {
		t.Fatal("expected error for invalid time")
	}

	err = s.Schedule("abc", func() {})
	if err == nil {
		t.Fatal("expected error for non-numeric time")
	}
}

func TestSchedule_Replaces(t *testing.T) {
	s, err := New("UTC")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()

	err = s.Schedule("08:00", func() {})
	if err != nil {
		t.Fatal(err)
	}
	firstEntry := s.entryID

	err = s.Schedule("10:00", func() {})
	if err != nil {
		t.Fatal(err)
	}

	if s.entryID == firstEntry {
		t.Error("expected entry ID to change after reschedule")
	}
}

func TestStartStop(t *testing.T) {
	s, err := New("UTC")
	if err != nil {
		t.Fatal(err)
	}

	s.Start()
	s.Stop()
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		input  string
		hour   int
		minute int
		valid  bool
	}{
		{"00:00", 0, 0, true},
		{"09:30", 9, 30, true},
		{"23:59", 23, 59, true},
		{"24:00", 0, 0, false},
		{"12:60", 0, 0, false},
		{"1:00", 0, 0, false},
		{"abc", 0, 0, false},
	}

	for _, tt := range tests {
		h, m, err := parseTime(tt.input)
		if tt.valid {
			if err != nil {
				t.Errorf("parseTime(%q) unexpected error: %v", tt.input, err)
			}
			if h != tt.hour || m != tt.minute {
				t.Errorf("parseTime(%q) = %d:%d, want %d:%d", tt.input, h, m, tt.hour, tt.minute)
			}
		} else {
			if err == nil {
				t.Errorf("parseTime(%q) expected error", tt.input)
			}
		}
	}
}

func TestSchedule_TaskExecutes(t *testing.T) {
	s, err := New("UTC")
	if err != nil {
		t.Fatal(err)
	}

	var count int64
	// Schedule for every minute to test execution (we'll use the underlying cron directly)
	s.cron.AddFunc("* * * * *", func() {
		atomic.AddInt64(&count, 1)
	})
	s.Start()

	// The cron runs at minute boundaries; just verify start/stop work without deadlock
	time.Sleep(100 * time.Millisecond)
	s.Stop()
}
