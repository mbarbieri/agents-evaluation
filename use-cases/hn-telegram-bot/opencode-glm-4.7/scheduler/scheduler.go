package scheduler

import (
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	cron     *cron.Cron
	loc      *time.Location
	timezone string
	stopped  bool
}

func NewScheduler(timezone string) (*Scheduler, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("invalid timezone: %w", err)
	}

	return &Scheduler{
		cron:     cron.New(cron.WithLocation(loc)),
		loc:      loc,
		timezone: timezone,
		stopped:  false,
	}, nil
}

func (s *Scheduler) Schedule(digestTime string, job func()) error {
	if s.stopped {
		return fmt.Errorf("scheduler is stopped")
	}

	parsedTime, err := time.Parse("15:04", digestTime)
	if err != nil {
		return fmt.Errorf("invalid time format: %w", err)
	}

	hour := parsedTime.Hour()
	minute := parsedTime.Minute()

	cronExpr := fmt.Sprintf("%d %d * * *", minute, hour)
	_, err = s.cron.AddFunc(cronExpr, job)
	if err != nil {
		return fmt.Errorf("failed to schedule job: %w", err)
	}

	s.cron.Start()
	return nil
}

func (s *Scheduler) Stop() error {
	if s.stopped {
		return fmt.Errorf("scheduler already stopped")
	}

	s.cron.Stop()
	s.stopped = true
	return nil
}

func (s *Scheduler) GetNextRun() time.Time {
	entries := s.cron.Entries()
	if len(entries) == 0 {
		return time.Time{}
	}

	return entries[0].Next
}
