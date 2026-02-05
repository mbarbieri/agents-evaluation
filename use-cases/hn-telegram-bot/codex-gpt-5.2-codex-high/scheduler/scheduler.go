package scheduler

import (
	"errors"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// Scheduler triggers a daily job at a configured time.
type Scheduler struct {
	mu       sync.Mutex
	cron     *cron.Cron
	jobID    cron.EntryID
	location *time.Location
	job      func()
}

var timeHHMM = regexp.MustCompile(`^(?:[01]\d|2[0-3]):[0-5]\d$`)

// New creates a scheduler for the given time and timezone.
func New(digestTime, timezone string, job func()) (*Scheduler, error) {
	if job == nil {
		return nil, errors.New("job must not be nil")
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("load timezone: %w", err)
	}
	hour, minute, err := parseTime(digestTime)
	if err != nil {
		return nil, err
	}

	c := cron.New(cron.WithLocation(loc))
	s := &Scheduler{cron: c, location: loc, job: job}
	if err := s.schedule(hour, minute); err != nil {
		return nil, err
	}
	return s, nil
}

// Start begins cron execution.
func (s *Scheduler) Start() {
	s.cron.Start()
}

// Stop stops the scheduler.
func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
}

// UpdateTime changes the scheduled daily time.
func (s *Scheduler) UpdateTime(digestTime string) error {
	hour, minute, err := parseTime(digestTime)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.jobID != 0 {
		s.cron.Remove(s.jobID)
	}
	return s.schedule(hour, minute)
}

// Location returns the scheduler location.
func (s *Scheduler) Location() *time.Location {
	return s.location
}

func (s *Scheduler) schedule(hour, minute int) error {
	spec := fmt.Sprintf("%d %d * * *", minute, hour)
	id, err := s.cron.AddFunc(spec, s.job)
	if err != nil {
		return fmt.Errorf("add cron: %w", err)
	}
	s.jobID = id
	return nil
}

func parseTime(value string) (int, int, error) {
	if !timeHHMM.MatchString(value) {
		return 0, 0, errors.New("invalid time format")
	}
	parsed, err := time.Parse("15:04", value)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid time: %w", err)
	}
	hour, minute := parsed.Hour(), parsed.Minute()
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, 0, errors.New("time out of range")
	}
	return hour, minute, nil
}
