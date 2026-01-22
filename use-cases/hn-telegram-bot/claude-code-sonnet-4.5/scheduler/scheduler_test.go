package scheduler

import (
	"testing"
)

func TestNew(t *testing.T) {
	s, err := New("UTC")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer s.Stop()

	if s == nil {
		t.Error("New() returned nil scheduler")
	}
}

func TestNew_InvalidTimezone(t *testing.T) {
	_, err := New("Invalid/Timezone")
	if err == nil {
		t.Error("New() expected error for invalid timezone, got nil")
	}
}

func TestSchedule_ValidTime(t *testing.T) {
	s, _ := New("UTC")
	defer s.Stop()

	callback := func() {}

	err := s.Schedule("09:00", callback)
	if err != nil {
		t.Fatalf("Schedule() error = %v", err)
	}

	// Note: We can't easily test if it actually fires at 09:00 UTC
	// but we can verify the schedule was accepted
}

func TestSchedule_InvalidTime(t *testing.T) {
	s, _ := New("UTC")
	defer s.Stop()

	callback := func() {}

	tests := []string{
		"25:00",
		"10:60",
		"not a time",
		"",
	}

	for _, timeStr := range tests {
		err := s.Schedule(timeStr, callback)
		if err == nil {
			t.Errorf("Schedule() expected error for time %s, got nil", timeStr)
		}
	}
}

func TestSchedule_UpdatesExisting(t *testing.T) {
	s, _ := New("UTC")
	defer s.Stop()

	callback1 := func() {}
	callback2 := func() {}

	s.Schedule("09:00", callback1)
	s.Schedule("10:00", callback2)

	// Verify scheduler accepted both schedules
	if s.entryID == 0 {
		t.Error("Schedule() did not set entry ID")
	}
}

func TestParseCronExpression(t *testing.T) {
	tests := []struct {
		name     string
		timeStr  string
		wantErr  bool
		wantExpr string
	}{
		{
			name:     "ValidMorning",
			timeStr:  "09:00",
			wantErr:  false,
			wantExpr: "0 9 * * *",
		},
		{
			name:     "ValidAfternoon",
			timeStr:  "14:30",
			wantErr:  false,
			wantExpr: "30 14 * * *",
		},
		{
			name:     "Midnight",
			timeStr:  "00:00",
			wantErr:  false,
			wantExpr: "0 0 * * *",
		},
		{
			name:     "BeforeMidnight",
			timeStr:  "23:59",
			wantErr:  false,
			wantExpr: "59 23 * * *",
		},
		{
			name:    "InvalidHour",
			timeStr: "25:00",
			wantErr: true,
		},
		{
			name:    "InvalidMinute",
			timeStr: "10:60",
			wantErr: true,
		},
		{
			name:    "WrongFormat",
			timeStr: "9:30",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := parseCronExpression(tt.timeStr)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseCronExpression() expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("parseCronExpression() error = %v", err)
				}
				if expr != tt.wantExpr {
					t.Errorf("parseCronExpression() = %v, want %v", expr, tt.wantExpr)
				}
			}
		})
	}
}

func TestStart_Stop(t *testing.T) {
	s, _ := New("UTC")

	s.Start()
	s.Stop()

	// Should be safe to call multiple times
	s.Stop()
	s.Stop()
}
