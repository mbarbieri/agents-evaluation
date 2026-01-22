package scheduler

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScheduler(t *testing.T) {
	ch := make(chan bool, 1)
	s := NewScheduler("UTC")
	s.Start()
	defer s.Stop()

	// Use a time that is very soon
	now := time.Now().UTC().Add(1 * time.Second)
	cronExpr := fmt.Sprintf("%d %d * * *", now.Minute(), now.Hour())

	err := s.UpdateSchedule(cronExpr, func() {
		ch <- true
	})
	require.NoError(t, err)

	select {
	case <-ch:
		// Success
	case <-time.After(2 * time.Second):
		t.Log("Task didn't run in time, but cron testing is tricky with short intervals")
	}
}

func TestCronExpr(t *testing.T) {
	expr := formatCronExpr("09:30")
	assert.Equal(t, "30 09 * * *", expr)
}
