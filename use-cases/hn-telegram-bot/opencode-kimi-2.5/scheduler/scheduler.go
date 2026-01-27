package scheduler

import (
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

// Scheduler manages cron jobs for the bot
type Scheduler struct {
	cron    *cron.Cron
	jobID   cron.EntryID
	running bool
}

// New creates a new Scheduler with the specified timezone
func New(timezone string) (*Scheduler, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("invalid timezone: %w", err)
	}

	return &Scheduler{
		cron: cron.New(cron.WithLocation(loc)),
	}, nil
}

// ScheduleDigest schedules the digest job to run at the specified time daily
// timeStr should be in HH:MM format (24-hour)
func (s *Scheduler) ScheduleDigest(timeStr string, job func()) error {
	// Validate time format
	if len(timeStr) != 5 || timeStr[2] != ':' {
		return fmt.Errorf("invalid time format: %s (expected HH:MM)", timeStr)
	}

	// Create cron expression for daily at specified time (minute hour * * *)
	cronExpr := fmt.Sprintf("%s %s * * *", timeStr[3:5], timeStr[0:2])

	// Remove existing job if any
	if s.jobID != 0 {
		s.cron.Remove(s.jobID)
	}

	// Schedule new job
	id, err := s.cron.AddFunc(cronExpr, job)
	if err != nil {
		return fmt.Errorf("failed to schedule job: %w", err)
	}

	s.jobID = id
	return nil
}

// Start begins running the scheduler
func (s *Scheduler) Start() {
	if !s.running {
		s.cron.Start()
		s.running = true
	}
}

// Stop halts the scheduler
func (s *Scheduler) Stop() {
	if s.running {
		s.cron.Stop()
		s.running = false
	}
}

// IsRunning returns whether the scheduler is currently running
func (s *Scheduler) IsRunning() bool {
	return s.running
}

// NextRun returns the time of the next scheduled run
func (s *Scheduler) NextRun() time.Time {
	if s.jobID == 0 {
		return time.Time{}
	}
	entry := s.cron.Entry(s.jobID)
	return entry.Next
}
