package scheduler

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	cron     *cron.Cron
	entryID  cron.EntryID
	mu       sync.Mutex
	location *time.Location
}

func New(timezone string) (*Scheduler, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("invalid timezone: %w", err)
	}

	c := cron.New(cron.WithLocation(loc))

	return &Scheduler{
		cron:     c,
		location: loc,
	}, nil
}

func (s *Scheduler) Start() {
	s.cron.Start()
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
}

func (s *Scheduler) UpdateSchedule(timeStr string, job func()) error {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return fmt.Errorf("invalid time format, expected HH:MM")
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return fmt.Errorf("invalid hour")
	}

	min, err := strconv.Atoi(parts[1])
	if err != nil || min < 0 || min > 59 {
		return fmt.Errorf("invalid minute")
	}

	spec := fmt.Sprintf("%d %d * * *", min, hour)

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.entryID != 0 {
		s.cron.Remove(s.entryID)
	}

	id, err := s.cron.AddFunc(spec, job)
	if err != nil {
		return fmt.Errorf("failed to schedule job: %w", err)
	}

	s.entryID = id
	return nil
}
