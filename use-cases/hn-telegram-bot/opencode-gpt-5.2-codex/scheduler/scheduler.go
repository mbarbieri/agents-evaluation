package scheduler

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	cron     *cron.Cron
	location *time.Location
	jobID    cron.EntryID
	job      func()
	mu       sync.Mutex
}

func New(timezone, digestTime string, job func()) (*Scheduler, error) {
	if job == nil {
		return nil, errors.New("job required")
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("load timezone: %w", err)
	}
	hour, minute, err := ParseDigestTime(digestTime)
	if err != nil {
		return nil, err
	}
	c := cron.New(cron.WithLocation(loc))
	s := &Scheduler{cron: c, location: loc, job: job}
	if err := s.setSchedule(hour, minute); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Scheduler) Start() error {
	if s == nil || s.cron == nil {
		return errors.New("scheduler not initialized")
	}
	s.cron.Start()
	return nil
}

func (s *Scheduler) Stop() error {
	if s == nil || s.cron == nil {
		return nil
	}
	ctx := s.cron.Stop()
	<-ctx.Done()
	return nil
}

func (s *Scheduler) Update(digestTime string) error {
	if s == nil || s.cron == nil {
		return errors.New("scheduler not initialized")
	}
	hour, minute, err := ParseDigestTime(digestTime)
	if err != nil {
		return err
	}
	return s.setSchedule(hour, minute)
}

func (s *Scheduler) setSchedule(hour, minute int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.jobID != 0 {
		s.cron.Remove(s.jobID)
	}
	expr := CronExpression(hour, minute)
	entryID, err := s.cron.AddFunc(expr, s.job)
	if err != nil {
		return fmt.Errorf("add cron: %w", err)
	}
	s.jobID = entryID
	return nil
}

func (s *Scheduler) nextTime() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.cron.Entry(s.jobID)
	return entry.Next
}

func ParseDigestTime(value string) (int, int, error) {
	parsed, err := time.Parse("15:04", value)
	if err != nil {
		return 0, 0, fmt.Errorf("digest time must be HH:MM: %w", err)
	}
	return parsed.Hour(), parsed.Minute(), nil
}

func CronExpression(hour, minute int) string {
	return fmt.Sprintf("%d %d * * *", minute, hour)
}
