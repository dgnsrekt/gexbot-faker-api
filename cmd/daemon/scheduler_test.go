package main

import (
	"testing"
	"time"
)

func TestSchedulerCatchUpAndFallback(t *testing.T) {
	s := NewScheduler(17, 5, "America/New_York")
	loc := s.Location()

	s.now = func() time.Time { return time.Date(2026, 7, 20, 17, 4, 0, 0, loc) }
	if s.IsScheduledOrLater() {
		t.Fatal("scheduled before 17:05")
	}
	s.now = func() time.Time { return time.Date(2026, 7, 20, 17, 6, 0, 0, loc) }
	if !s.IsScheduledOrLater() || s.IsFallbackTime() {
		t.Fatal("expected catch-up before fallback")
	}
	s.now = func() time.Time { return time.Date(2026, 7, 20, 20, 0, 0, 0, loc) }
	if !s.IsFallbackTime() {
		t.Fatal("expected fallback at 20:00")
	}
}
