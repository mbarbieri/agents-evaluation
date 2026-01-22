package scheduler

import (
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

// Scheduler manages the cron job for digest delivery
type Scheduler struct {
	cron    *cron.Cron
	job     func()
	entryID cron.EntryID
}

// New creates a new scheduler with the given job function
func New(job func()) *Scheduler {
	return &Scheduler{
		job: job,
	}
}

// UpdateSchedule sets the cron schedule for the job
func (s *Scheduler) UpdateSchedule(timeStr, timezone string) error {
	// Parse time
	parsedTime, err := time.Parse("15:04", timeStr)
	if err != nil {
		return fmt.Errorf("invalid time format: %w", err)
	}

	// Load location
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return fmt.Errorf("invalid timezone: %w", err)
	}

	// Stop existing cron if running
	if s.cron != nil {
		s.cron.Stop()
	}

	// Create new cron with location
	s.cron = cron.New(cron.WithLocation(loc), cron.WithSeconds())

	// Cron spec: "0 MM HH * * *"
	spec := fmt.Sprintf("0 %d %d * * *", parsedTime.Minute(), parsedTime.Hour())

	// Add job
	entryID, err := s.cron.AddFunc(spec, s.job)
	if err != nil {
		return fmt.Errorf("failed to add cron job: %w", err)
	}
	s.entryID = entryID

	return nil
}

// Start starts the scheduler
func (s *Scheduler) Start() {
	if s.cron != nil {
		s.cron.Start()
	}
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	if s.cron != nil {
		s.cron.Stop()
	}
}
