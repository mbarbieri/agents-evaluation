package scheduler

import (
	"fmt"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

var timeRegex = regexp.MustCompile(`^([01][0-9]|2[0-3]):([0-5][0-9])$`)

// Scheduler manages cron-based job scheduling with timezone support.
type Scheduler struct {
	cron     *cron.Cron
	location *time.Location
	mu       sync.Mutex
	entryID  cron.EntryID
	started  bool
}

// NewScheduler creates a new scheduler for the given timezone.
func NewScheduler(timezone string) (*Scheduler, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("load timezone %q: %w", timezone, err)
	}

	return &Scheduler{
		cron:     cron.New(cron.WithLocation(loc)),
		location: loc,
	}, nil
}

// Schedule sets up a daily job at the specified time (HH:MM format).
func (s *Scheduler) Schedule(timeStr string, fn func()) error {
	hour, minute, err := parseTime(timeStr)
	if err != nil {
		return err
	}

	spec := buildCronSpec(hour, minute)

	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove existing job if any
	if s.entryID != 0 {
		s.cron.Remove(s.entryID)
	}

	entryID, err := s.cron.AddFunc(spec, fn)
	if err != nil {
		return fmt.Errorf("add cron job: %w", err)
	}
	s.entryID = entryID

	return nil
}

// Start begins the scheduler.
func (s *Scheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		s.cron.Start()
		s.started = true
	}
}

// Stop halts the scheduler.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		s.cron.Stop()
		s.started = false
	}
}

func parseTime(timeStr string) (int, int, error) {
	matches := timeRegex.FindStringSubmatch(timeStr)
	if len(matches) != 3 {
		return 0, 0, fmt.Errorf("invalid time format: %q (expected HH:MM)", timeStr)
	}

	hour, _ := strconv.Atoi(matches[1])
	minute, _ := strconv.Atoi(matches[2])

	return hour, minute, nil
}

func buildCronSpec(hour, minute int) string {
	// Cron format: minute hour day month weekday
	return fmt.Sprintf("%d %d * * *", minute, hour)
}
