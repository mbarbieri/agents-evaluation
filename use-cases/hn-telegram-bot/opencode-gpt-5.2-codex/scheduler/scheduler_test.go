package scheduler

import (
	"testing"
	"time"
)

func TestParseDigestTimeValid(t *testing.T) {
	t.Parallel()

	hour, minute, err := ParseDigestTime("09:30")
	if err != nil {
		t.Fatalf("ParseDigestTime: %v", err)
	}
	if hour != 9 || minute != 30 {
		t.Fatalf("expected 9:30 got %02d:%02d", hour, minute)
	}
}

func TestParseDigestTimeInvalid(t *testing.T) {
	t.Parallel()

	_, _, err := ParseDigestTime("99:30")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestCronExpression(t *testing.T) {
	t.Parallel()

	expr := CronExpression(7, 45)
	if expr != "45 7 * * *" {
		t.Fatalf("unexpected expr %q", expr)
	}
}

func TestNewSchedulerValidatesTimezone(t *testing.T) {
	t.Parallel()

	_, err := New("Nope/Zone", "09:00", func() {})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestSchedulerUpdate(t *testing.T) {
	t.Parallel()

	s, err := New("UTC", "09:00", func() {})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := s.Update("10:15"); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if s.nextTime().IsZero() {
		t.Fatalf("expected next time to be set")
	}
	if err := s.Update("99:99"); err == nil {
		t.Fatalf("expected error")
	}
	_ = s.Stop()
}

func TestSchedulerStartStop(t *testing.T) {
	t.Parallel()

	s, err := New("UTC", "09:00", func() {})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := s.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}
