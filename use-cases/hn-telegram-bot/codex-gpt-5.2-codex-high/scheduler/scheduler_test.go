package scheduler

import "testing"

func TestParseTime(t *testing.T) {
	h, m, err := parseTime("09:30")
	if err != nil {
		t.Fatalf("parseTime: %v", err)
	}
	if h != 9 || m != 30 {
		t.Fatalf("unexpected time %d:%d", h, m)
	}

	_, _, err = parseTime("9:30")
	if err == nil {
		t.Fatalf("expected error for invalid format")
	}
}

func TestUpdateTimeInvalid(t *testing.T) {
	s, err := New("09:00", "UTC", func() {})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := s.UpdateTime("25:00"); err == nil {
		t.Fatalf("expected error")
	}
}
