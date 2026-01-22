package scheduler

import (
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	cron     *cron.Cron
	location *time.Location
	mu       sync.RWMutex
}

func NewScheduler(timezone string) (*Scheduler, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("failed to load timezone %s: %w", timezone, err)
	}

	return &Scheduler{
		cron:     cron.New(cron.WithLocation(loc)),
		location: loc,
	}, nil
}

func (s *Scheduler) Schedule(digestTime string, fn func()) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cronExpr := convertToCron(digestTime)
	if cronExpr == "" {
		return fmt.Errorf("invalid time format: %s", digestTime)
	}

	_, err := s.cron.AddFunc(cronExpr, fn)
	if err != nil {
		return fmt.Errorf("failed to schedule job: %w", err)
	}

	return nil
}

func (s *Scheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cron.Start()
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cron.Stop()
}

func (s *Scheduler) UpdateSchedule(digestTime string, fn func()) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cron.Stop()
	s.cron = cron.New(cron.WithLocation(s.location))

	cronExpr := convertToCron(digestTime)
	if cronExpr == "" {
		return fmt.Errorf("invalid time format: %s", digestTime)
	}

	_, err := s.cron.AddFunc(cronExpr, fn)
	if err != nil {
		return fmt.Errorf("failed to schedule job: %w", err)
	}

	s.cron.Start()
	return nil
}

func (s *Scheduler) NextRun() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries := s.cron.Entries()
	if len(entries) == 0 {
		return time.Time{}
	}
	return entries[0].Next
}

func convertToCron(digestTime string) string {
	if len(digestTime) != 5 || digestTime[2] != ':' {
		return ""
	}

	hour := digestTime[:2]
	minute := digestTime[3:]

	var hourInt, minuteInt int
	_, err := fmt.Sscanf(hour, "%d", &hourInt)
	if err != nil {
		return ""
	}
	_, err = fmt.Sscanf(minute, "%d", &minuteInt)
	if err != nil {
		return ""
	}

	if hourInt < 0 || hourInt > 23 {
		return ""
	}
	if minuteInt < 0 || minuteInt > 59 {
		return ""
	}

	return fmt.Sprintf("%d %d * * *", minuteInt, hourInt)
}
