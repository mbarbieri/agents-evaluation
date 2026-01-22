package scheduler

import (
	"testing"
)

func TestUpdateSchedule_Valid(t *testing.T) {
	sched := New(func() {})
	err := sched.UpdateSchedule("09:00", "UTC")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestUpdateSchedule_InvalidTime(t *testing.T) {
	sched := New(func() {})
	err := sched.UpdateSchedule("25:00", "UTC")
	if err == nil {
		t.Error("expected error for invalid time")
	}
}

func TestUpdateSchedule_InvalidTimezone(t *testing.T) {
	sched := New(func() {})
	err := sched.UpdateSchedule("09:00", "Invalid/Timezone")
	if err == nil {
		t.Error("expected error for invalid timezone")
	}
}

func TestStartStop(t *testing.T) {
	sched := New(func() {})
	sched.Start()
	sched.Stop() // Should not panic
}
