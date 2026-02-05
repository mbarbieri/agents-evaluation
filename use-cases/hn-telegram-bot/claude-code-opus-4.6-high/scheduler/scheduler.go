package scheduler

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// Scheduler manages cron-based digest scheduling.
type Scheduler struct {
	cron     *cron.Cron
	mu       sync.Mutex
	entryID  cron.EntryID
	task     func()
	location *time.Location
}

// New creates a Scheduler in the given timezone.
func New(timezone string) (*Scheduler, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("loading timezone %q: %w", timezone, err)
	}

	c := cron.New(cron.WithLocation(loc))

	return &Scheduler{
		cron:     c,
		location: loc,
	}, nil
}

// Schedule sets up the daily digest at the given time (HH:MM format).
// If a previous schedule exists, it is replaced.
func (s *Scheduler) Schedule(digestTime string, task func()) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	hour, minute, err := parseTime(digestTime)
	if err != nil {
		return err
	}

	// Remove previous entry if it exists
	if s.entryID != 0 {
		s.cron.Remove(s.entryID)
	}

	expr := fmt.Sprintf("%d %d * * *", minute, hour)
	entryID, err := s.cron.AddFunc(expr, task)
	if err != nil {
		return fmt.Errorf("adding cron entry: %w", err)
	}

	s.entryID = entryID
	s.task = task
	slog.Info("digest scheduled", "time", digestTime, "cron", expr, "timezone", s.location.String())
	return nil
}

// Start begins the cron scheduler.
func (s *Scheduler) Start() {
	s.cron.Start()
}

// Stop halts the cron scheduler.
func (s *Scheduler) Stop() {
	s.cron.Stop()
}

// parseTime extracts hour and minute from HH:MM format.
func parseTime(t string) (int, int, error) {
	if len(t) != 5 || t[2] != ':' {
		return 0, 0, fmt.Errorf("invalid time format %q: must be HH:MM", t)
	}

	hour := (int(t[0]-'0') * 10) + int(t[1]-'0')
	minute := (int(t[3]-'0') * 10) + int(t[4]-'0')

	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("invalid time %q: hour 0-23, minute 0-59", t)
	}

	return hour, minute, nil
}
