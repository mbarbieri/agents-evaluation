package scheduler

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/robfig/cron/v3"
)

type Scheduler interface {
	Schedule(timeStr string, callback func()) error
	Start()
	Stop()
}

type CronScheduler struct {
	cron    *cron.Cron
	entryID cron.EntryID
}

func New(timezone string) (*CronScheduler, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("invalid timezone: %w", err)
	}

	c := cron.New(cron.WithLocation(loc))

	return &CronScheduler{
		cron: c,
	}, nil
}

func (s *CronScheduler) Schedule(timeStr string, callback func()) error {
	cronExpr, err := parseCronExpression(timeStr)
	if err != nil {
		return err
	}

	// Remove existing entry if any
	if s.entryID != 0 {
		s.cron.Remove(s.entryID)
	}

	// Add new entry
	entryID, err := s.cron.AddFunc(cronExpr, callback)
	if err != nil {
		return fmt.Errorf("failed to schedule: %w", err)
	}

	s.entryID = entryID
	return nil
}

func (s *CronScheduler) Start() {
	s.cron.Start()
}

func (s *CronScheduler) Stop() {
	s.cron.Stop()
}

func parseCronExpression(timeStr string) (string, error) {
	// Validate format: HH:MM
	re := regexp.MustCompile(`^([0-1][0-9]|2[0-3]):([0-5][0-9])$`)
	matches := re.FindStringSubmatch(timeStr)
	if matches == nil {
		return "", fmt.Errorf("invalid time format: %s (expected HH:MM)", timeStr)
	}

	hour, _ := strconv.Atoi(matches[1])
	minute, _ := strconv.Atoi(matches[2])

	// Cron format: minute hour day month weekday
	// We want daily at specific time: "minute hour * * *"
	return fmt.Sprintf("%d %d * * *", minute, hour), nil
}
