package scheduler

import (
	"sync"
	"testing"
	"time"
)

func TestScheduler(t *testing.T) {
	s := NewScheduler("UTC")
	s.Start()
	defer s.Stop()

	var mu sync.Mutex
	called := 0

	job := func() {
		mu.Lock()
		called++
		mu.Unlock()
	}

	t.Run("ScheduleJob", func(t *testing.T) {
		// Schedule to run every second for testing
		err := s.UpdateSchedule("* * * * * *", job)
		if err != nil {
			t.Fatalf("failed to update schedule: %v", err)
		}

		// Wait for it to trigger
		time.Sleep(1500 * time.Millisecond)

		mu.Lock()
		if called == 0 {
			t.Error("job was not called")
		}
		mu.Unlock()
	})

	t.Run("InvalidCron", func(t *testing.T) {
		err := s.UpdateSchedule("invalid", job)
		if err == nil {
			t.Error("expected error for invalid cron, got nil")
		}
	})
}
