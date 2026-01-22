package scheduler

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestNewScheduler(t *testing.T) {
	scheduler, err := NewScheduler("UTC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if scheduler == nil {
		t.Error("expected non-nil scheduler")
	}
}

func TestSchedule(t *testing.T) {
	scheduler, err := NewScheduler("UTC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var callCount int32

	err = scheduler.Schedule("09:00", func() {
		atomic.AddInt32(&callCount, 1)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	time.Sleep(2 * time.Second)

	if callCount != 0 {
		t.Errorf("call count = %d, want 0 (scheduled for future)", callCount)
	}

	if err = scheduler.Stop(); err != nil {
		t.Errorf("failed to stop scheduler: %v", err)
	}
}

func TestScheduleInvalidTime(t *testing.T) {
	scheduler, err := NewScheduler("UTC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer scheduler.Stop()

	err = scheduler.Schedule("invalid", func() {})
	if err == nil {
		t.Error("expected error for invalid time, got nil")
	}
}

func TestScheduleInvalidTimezone(t *testing.T) {
	_, err := NewScheduler("Invalid/Timezone")
	if err == nil {
		t.Error("expected error for invalid timezone, got nil")
	}
}

func TestScheduleWithPastTime(t *testing.T) {
	scheduler, err := NewScheduler("UTC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer scheduler.Stop()

	now := time.Now()
	pastTime := now.Add(-1 * time.Hour)
	timeStr := pastTime.Format("15:04")

	var callCount int32
	err = scheduler.Schedule(timeStr, func() {
		atomic.AddInt32(&callCount, 1)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	time.Sleep(2 * time.Second)

	if callCount != 0 {
		t.Errorf("call count = %d, want 0 (past time)", callCount)
	}
}

func TestScheduleWithDifferentTimezones(t *testing.T) {
	tests := []struct {
		name     string
		timezone string
		timeStr  string
	}{
		{"UTC", "UTC", "09:00"},
		{"America/New_York", "America/New_York", "09:00"},
		{"Europe/London", "Europe/London", "09:00"},
		{"Asia/Tokyo", "Asia/Tokyo", "09:00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheduler, err := NewScheduler(tt.timezone)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			defer scheduler.Stop()

			err = scheduler.Schedule(tt.timeStr, func() {})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestStop(t *testing.T) {
	scheduler, err := NewScheduler("UTC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = scheduler.Schedule("09:00", func() {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = scheduler.Stop()
	if err != nil {
		t.Errorf("failed to stop scheduler: %v", err)
	}

	var callCount int32
	err = scheduler.Schedule("10:00", func() {
		atomic.AddInt32(&callCount, 1)
	})
	if err == nil {
		t.Error("expected error when scheduling after stop, got nil")
	}
}

func TestStopMultipleTimes(t *testing.T) {
	scheduler, err := NewScheduler("UTC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = scheduler.Schedule("09:00", func() {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = scheduler.Stop()
	if err != nil {
		t.Errorf("failed to stop scheduler: %v", err)
	}

	err = scheduler.Stop()
	if err == nil {
		t.Error("expected error when stopping already stopped scheduler, got nil")
	}
}

func TestScheduleMultiple(t *testing.T) {
	scheduler, err := NewScheduler("UTC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer scheduler.Stop()

	var callCount int32

	for i := 0; i < 3; i++ {
		err = scheduler.Schedule("09:00", func() {
			atomic.AddInt32(&callCount, 1)
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	time.Sleep(1 * time.Second)
}

func TestGetNextRun(t *testing.T) {
	scheduler, err := NewScheduler("UTC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = scheduler.Schedule("09:00", func() {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	nextRun := scheduler.GetNextRun()
	if nextRun.IsZero() {
		t.Error("expected non-zero next run time")
	}
}

func TestGetNextRunNoJob(t *testing.T) {
	scheduler, err := NewScheduler("UTC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer scheduler.Stop()

	nextRun := scheduler.GetNextRun()
	if !nextRun.IsZero() {
		t.Error("expected zero next run time when no job scheduled")
	}
}
