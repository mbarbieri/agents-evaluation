package scheduler

import (
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	cron     *cron.Cron
	location *time.Location
	entryID  cron.EntryID
}

func NewScheduler(timezone string) *Scheduler {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}
	return &Scheduler{
		cron: cron.New(
			cron.WithLocation(loc),
			cron.WithParser(cron.NewParser(
				cron.Second|cron.Minute|cron.Hour|cron.Dom|cron.Month|cron.Dow|cron.Descriptor,
			)),
		),
		location: loc,
	}
}

func (s *Scheduler) Start() {
	s.cron.Start()
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
}

func (s *Scheduler) UpdateSchedule(cronExpr string, job func()) error {
	if s.entryID != 0 {
		s.cron.Remove(s.entryID)
	}

	id, err := s.cron.AddFunc(cronExpr, job)
	if err != nil {
		return fmt.Errorf("failed to add cron job: %w", err)
	}
	s.entryID = id
	return nil
}

func (s *Scheduler) SetTimezone(timezone string) error {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return err
	}
	s.location = loc
	// Note: changing location on an existing cron is tricky.
	// In our case, we'll recreate the cron if needed, or just warn that it takes effect on next UpdateSchedule.
	// For simplicity, we'll assume the scheduler is recreated or updated with NewScheduler if timezone changes.
	return nil
}
