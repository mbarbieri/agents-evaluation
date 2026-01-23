package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
)

type fakeRunner struct{ calls atomic.Int64 }

func (f *fakeRunner) Run(ctx context.Context) error {
	f.calls.Add(1)
	return nil
}

type fakeSettings struct{ t, tz string }

func (f fakeSettings) DigestTime() string { return f.t }
func (f fakeSettings) Timezone() string   { return f.tz }

type noopCronLogger struct{}

func (n noopCronLogger) Info(msg string, keysAndValues ...any)             {}
func (n noopCronLogger) Error(err error, msg string, keysAndValues ...any) {}

func TestCronSpecDailyHHMM(t *testing.T) {
	t.Parallel()
	if got := CronSpecDailyHHMM("09:30"); got != "30 9 * * *" {
		t.Fatalf("got %q", got)
	}
}

func TestScheduler_StartUpdateStop(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{}
	s := New(r, noopCronLogger{})

	if err := s.Start(context.Background(), fakeSettings{t: "00:00", tz: "UTC"}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := s.Update(fakeSettings{t: "00:01", tz: "UTC"}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	s.Stop()
}

func TestScheduler_StartRejectsBadTimezone(t *testing.T) {
	t.Parallel()
	s := New(&fakeRunner{}, cron.VerbosePrintfLogger(nil))
	if err := s.Start(context.Background(), fakeSettings{t: "00:00", tz: "Nope/Nope"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestScheduler_UpdateRequiresStart(t *testing.T) {
	t.Parallel()
	s := New(&fakeRunner{}, noopCronLogger{})
	if err := s.Update(fakeSettings{t: "00:00", tz: "UTC"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestScheduler_StartIsIdempotentError(t *testing.T) {
	t.Parallel()
	s := New(&fakeRunner{}, noopCronLogger{})
	if err := s.Start(context.Background(), fakeSettings{t: "00:00", tz: "UTC"}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := s.Start(context.Background(), fakeSettings{t: "00:00", tz: "UTC"}); err == nil {
		t.Fatalf("expected error")
	}
	s.Stop()
}

func TestScheduler_TriggersRunner(t *testing.T) {
	// Non-parallel: time-based.
	r := &fakeRunner{}
	s := New(r, noopCronLogger{})

	// Schedule for the next minute boundary in UTC.
	now := time.Now().UTC().Add(2 * time.Second)
	nextMin := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), 0, 0, time.UTC).Add(1 * time.Minute)
	hhmm := nextMin.Format("15:04")

	if err := s.Start(context.Background(), fakeSettings{t: hhmm, tz: "UTC"}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Stop()

	deadline := time.NewTimer(time.Until(nextMin.Add(3 * time.Second)))
	ticker := time.NewTicker(200 * time.Millisecond)
	defer deadline.Stop()
	defer ticker.Stop()

	for {
		select {
		case <-deadline.C:
			if r.calls.Load() == 0 {
				t.Fatalf("expected runner call")
			}
			return
		case <-ticker.C:
			if r.calls.Load() > 0 {
				return
			}
		}
	}
}
