package scheduler

import (
	"testing"
)

func TestNewScheduler(t *testing.T) {
	s, err := NewScheduler("America/New_York")
	if err != nil {
		t.Fatalf("NewScheduler failed: %v", err)
	}
	defer s.Stop()

	if s.location.String() != "America/New_York" {
		t.Errorf("location = %q, want 'America/New_York'", s.location.String())
	}
}

func TestNewSchedulerInvalidTimezone(t *testing.T) {
	_, err := NewScheduler("Invalid/Zone")
	if err == nil {
		t.Fatal("expected error for invalid timezone")
	}
}

func TestScheduleAndStop(t *testing.T) {
	s, _ := NewScheduler("UTC")
	defer s.Stop()

	// Simply test that we can schedule and start without errors
	// Testing actual cron execution timing is unreliable in unit tests
	err := s.Schedule("12:00", func() {})
	if err != nil {
		t.Fatalf("Schedule failed: %v", err)
	}

	s.Start()

	// Verify scheduler is running (entries should exist)
	entries := s.cron.Entries()
	if len(entries) != 1 {
		t.Errorf("expected 1 cron entry, got %d", len(entries))
	}
}

func TestScheduleInvalidTime(t *testing.T) {
	s, _ := NewScheduler("UTC")
	defer s.Stop()

	tests := []string{
		"invalid",
		"25:00",
		"12:60",
		"9:00",   // Missing leading zero
		"12:0",   // Missing leading zero
	}

	for _, tt := range tests {
		err := s.Schedule(tt, func() {})
		if err == nil {
			t.Errorf("expected error for invalid time %q", tt)
		}
	}
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		input   string
		hour    int
		minute  int
		wantErr bool
	}{
		{"09:00", 9, 0, false},
		{"00:00", 0, 0, false},
		{"23:59", 23, 59, false},
		{"12:30", 12, 30, false},
		{"25:00", 0, 0, true},
		{"12:60", 0, 0, true},
		{"invalid", 0, 0, true},
	}

	for _, tt := range tests {
		hour, minute, err := parseTime(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseTime(%q) should return error", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("parseTime(%q) unexpected error: %v", tt.input, err)
			}
			if hour != tt.hour || minute != tt.minute {
				t.Errorf("parseTime(%q) = (%d, %d), want (%d, %d)",
					tt.input, hour, minute, tt.hour, tt.minute)
			}
		}
	}
}

func TestBuildCronSpec(t *testing.T) {
	tests := []struct {
		hour     int
		minute   int
		expected string
	}{
		{9, 0, "0 9 * * *"},
		{0, 0, "0 0 * * *"},
		{23, 59, "59 23 * * *"},
		{12, 30, "30 12 * * *"},
	}

	for _, tt := range tests {
		spec := buildCronSpec(tt.hour, tt.minute)
		if spec != tt.expected {
			t.Errorf("buildCronSpec(%d, %d) = %q, want %q",
				tt.hour, tt.minute, spec, tt.expected)
		}
	}
}

func TestReschedule(t *testing.T) {
	s, _ := NewScheduler("UTC")
	defer s.Stop()

	fn := func() {}

	// Initial schedule
	if err := s.Schedule("12:00", fn); err != nil {
		t.Fatalf("initial Schedule failed: %v", err)
	}

	// Verify one entry
	if len(s.cron.Entries()) != 1 {
		t.Error("expected 1 entry after initial schedule")
	}

	// Reschedule to different time
	if err := s.Schedule("14:00", fn); err != nil {
		t.Fatalf("reschedule failed: %v", err)
	}

	// Still should have only one entry (old one removed)
	if len(s.cron.Entries()) != 1 {
		t.Error("expected 1 entry after reschedule")
	}

	// Verify we can still start
	s.Start()
}

func TestMultipleStartStop(t *testing.T) {
	s, _ := NewScheduler("UTC")

	s.Schedule("12:00", func() {})

	// Multiple starts shouldn't panic
	s.Start()
	s.Start()

	// Multiple stops shouldn't panic
	s.Stop()
	s.Stop()
}
