package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

type Runner interface {
	Run(ctx context.Context) error
}

type Settings interface {
	DigestTime() string
	Timezone() string
}

type Scheduler struct {
	mu     sync.Mutex
	cron   *cron.Cron
	entry  cron.EntryID
	runner Runner
	log    cron.Logger
	ctx    context.Context
}

func New(r Runner, logger cron.Logger) *Scheduler {
	return &Scheduler{runner: r, log: logger}
}

func (s *Scheduler) Start(ctx context.Context, settings Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cron != nil {
		return fmt.Errorf("already started")
	}
	loc, err := time.LoadLocation(settings.Timezone())
	if err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	s.ctx = ctx
	sched := cron.New(cron.WithLocation(loc), cron.WithLogger(s.log))
	entry, err := sched.AddFunc(CronSpecDailyHHMM(settings.DigestTime()), func() {
		select {
		case <-s.ctx.Done():
			return
		default:
		}
		_ = s.runner.Run(s.ctx)
	})
	if err != nil {
		return err
	}
	s.cron = sched
	s.entry = entry
	sched.Start()
	return nil
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cron == nil {
		return
	}
	ctx := s.cron.Stop()
	<-ctx.Done()
	s.cron = nil
	s.entry = 0
	s.ctx = nil
}

func (s *Scheduler) Update(settings Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cron == nil {
		return fmt.Errorf("not started")
	}
	loc, err := time.LoadLocation(settings.Timezone())
	if err != nil {
		return err
	}
	s.cron.Stop()
	// Recreate to ensure location changes are applied.
	s.cron = cron.New(cron.WithLocation(loc), cron.WithLogger(s.log))
	entry, err := s.cron.AddFunc(CronSpecDailyHHMM(settings.DigestTime()), func() {
		select {
		case <-s.ctx.Done():
			return
		default:
		}
		_ = s.runner.Run(s.ctx)
	})
	if err != nil {
		return err
	}
	s.entry = entry
	s.cron.Start()
	return nil
}

func CronSpecDailyHHMM(hhmm string) string {
	// cron v3 default parser: "min hour dom month dow"
	// We intentionally ignore seconds.
	parts := [2]int{0, 0}
	_, _ = fmt.Sscanf(hhmm, "%d:%d", &parts[0], &parts[1])
	return fmt.Sprintf("%d %d * * *", parts[1], parts[0])
}
