package scheduler

import (
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	cron     *cron.Cron
	timezone string
	entryID  cron.EntryID
}

func NewScheduler(timezone string) *Scheduler {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}
	return &Scheduler{
		cron:     cron.New(cron.WithLocation(loc)),
		timezone: timezone,
	}
}

func (s *Scheduler) Start() {
	s.cron.Start()
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
}

func (s *Scheduler) UpdateSchedule(timeStr string, task func()) error {
	if s.entryID != 0 {
		s.cron.Remove(s.entryID)
	}

	cronExpr := formatCronExpr(timeStr)
	id, err := s.cron.AddFunc(cronExpr, task)
	if err != nil {
		return fmt.Errorf("failed to add cron job: %w", err)
	}
	s.entryID = id
	return nil
}

func formatCronExpr(timeStr string) string {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return "0 9 * * *" // Default
	}
	return fmt.Sprintf("%s %s * * *", parts[1], parts[0])
}
