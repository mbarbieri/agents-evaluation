package scheduler

import (
	"sync/atomic"
	"testing"
)

func TestNew(t *testing.T) {
	t.Run("creates scheduler with valid timezone", func(t *testing.T) {
		s, err := New("UTC")
		if err != nil {
			t.Errorf("New() error = %v", err)
		}
		if s == nil {
			t.Error("New() returned nil")
		}
		if s.cron == nil {
			t.Error("cron is nil")
		}
	})

	t.Run("creates scheduler with different timezone", func(t *testing.T) {
		s, err := New("America/New_York")
		if err != nil {
			t.Errorf("New() error = %v", err)
		}
		if s == nil {
			t.Error("New() returned nil")
		}
	})

	t.Run("fails with invalid timezone", func(t *testing.T) {
		_, err := New("Invalid/Timezone")
		if err == nil {
			t.Error("New() should error with invalid timezone")
		}
	})
}

func TestScheduleDigest(t *testing.T) {
	s, _ := New("UTC")

	t.Run("schedules job with valid time", func(t *testing.T) {
		var called int32
		job := func() {
			atomic.AddInt32(&called, 1)
		}

		err := s.ScheduleDigest("09:00", job)
		if err != nil {
			t.Errorf("ScheduleDigest() error = %v", err)
		}
	})

	t.Run("fails with invalid time format", func(t *testing.T) {
		err := s.ScheduleDigest("9:00", func() {})
		if err == nil {
			t.Error("ScheduleDigest() should error with invalid format")
		}
	})

	t.Run("fails with malformed time", func(t *testing.T) {
		err := s.ScheduleDigest("invalid", func() {})
		if err == nil {
			t.Error("ScheduleDigest() should error with malformed time")
		}
	})

	t.Run("updates existing job", func(t *testing.T) {
		err := s.ScheduleDigest("10:00", func() {})
		if err != nil {
			t.Errorf("ScheduleDigest() error = %v", err)
		}

		err = s.ScheduleDigest("11:00", func() {})
		if err != nil {
			t.Errorf("ScheduleDigest() update error = %v", err)
		}

		// Job should be rescheduled successfully
		if s.jobID == 0 {
			t.Error("jobID should not be zero after rescheduling")
		}
	})
}

func TestStartStop(t *testing.T) {
	s, _ := New("UTC")

	t.Run("starts scheduler", func(t *testing.T) {
		s.ScheduleDigest("00:00", func() {})
		s.Start()

		if !s.IsRunning() {
			t.Error("IsRunning() should be true after Start()")
		}
	})

	t.Run("stops scheduler", func(t *testing.T) {
		s.Stop()

		if s.IsRunning() {
			t.Error("IsRunning() should be false after Stop()")
		}
	})

	t.Run("start is idempotent", func(t *testing.T) {
		s.Start()
		s.Start() // Should not panic

		if !s.IsRunning() {
			t.Error("IsRunning() should be true")
		}

		s.Stop()
	})

	t.Run("stop is idempotent", func(t *testing.T) {
		s.Stop()
		s.Stop() // Should not panic

		if s.IsRunning() {
			t.Error("IsRunning() should be false")
		}
	})
}

func TestNextRun(t *testing.T) {
	s, _ := New("UTC")

	t.Run("returns zero time when no job scheduled", func(t *testing.T) {
		next := s.NextRun()
		if !next.IsZero() {
			t.Errorf("NextRun() = %v, want zero time", next)
		}
	})

	t.Run("returns time after scheduling and starting", func(t *testing.T) {
		s.ScheduleDigest("12:00", func() {})
		s.Start()
		defer s.Stop()

		next := s.NextRun()

		if next.IsZero() {
			t.Error("NextRun() should not be zero after scheduling and starting")
		}
	})
}

func TestScheduleDifferentTimes(t *testing.T) {
	s, _ := New("UTC")

	tests := []struct {
		timeStr string
		wantErr bool
	}{
		{"00:00", false},
		{"23:59", false},
		{"12:30", false},
		{"09:00", false},
		{"9:00", true},  // Missing leading zero
		{"24:00", true}, // Invalid hour
		{"12:60", true}, // Invalid minutes
	}

	for _, tt := range tests {
		t.Run(tt.timeStr, func(t *testing.T) {
			err := s.ScheduleDigest(tt.timeStr, func() {})
			if tt.wantErr && err == nil {
				t.Errorf("ScheduleDigest(%s) expected error, got nil", tt.timeStr)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ScheduleDigest(%s) unexpected error: %v", tt.timeStr, err)
			}
		})
	}
}

func TestTimezoneHandling(t *testing.T) {
	// Test with different timezones
	timezones := []string{
		"UTC",
		"America/New_York",
		"Europe/London",
		"Asia/Tokyo",
		"Pacific/Auckland",
	}

	for _, tz := range timezones {
		t.Run(tz, func(t *testing.T) {
			s, err := New(tz)
			if err != nil {
				t.Errorf("New(%s) error = %v", tz, err)
				return
			}

			err = s.ScheduleDigest("09:00", func() {})
			if err != nil {
				t.Errorf("ScheduleDigest error = %v", err)
			}

			s.Start()
			if !s.IsRunning() {
				t.Error("Scheduler should be running")
			}
			s.Stop()
		})
	}
}
