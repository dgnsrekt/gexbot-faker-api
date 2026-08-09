package main

import (
	"reflect"
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

func TestSchedulerMissedMarketDays(t *testing.T) {
	s := NewScheduler(17, 5, "America/New_York")
	loc := s.Location()
	// Monday 2026-08-10. Preceding week: Mon 08-03 ... Fri 08-07, Sat/Sun 08-08/09.
	s.now = func() time.Time { return time.Date(2026, 8, 10, 9, 0, 0, 0, loc) }

	// last=Wed 08-05 -> Thu 08-06, Fri 08-07 (weekend skipped, today 08-10 excluded).
	got, capped := s.MissedMarketDays("2026-08-05", 90)
	if want := []string{"2026-08-06", "2026-08-07"}; !reflect.DeepEqual(got, want) || capped {
		t.Fatalf("missed = %v (capped=%v), want %v (capped=false)", got, capped, want)
	}

	// Gap spans only a weekend -> nothing missed.
	if got, _ := s.MissedMarketDays("2026-08-07", 90); len(got) != 0 {
		t.Fatalf("weekend-only gap should be empty, got %v", got)
	}

	// last == today -> nothing.
	if got, _ := s.MissedMarketDays("2026-08-10", 90); len(got) != 0 {
		t.Fatalf("no gap should be empty, got %v", got)
	}

	// Cap keeps only the most recent maxDays and flags it.
	got, capped = s.MissedMarketDays("2026-08-05", 1)
	if !capped || !reflect.DeepEqual(got, []string{"2026-08-07"}) {
		t.Fatalf("cap=1 should keep only 2026-08-07 (capped), got %v capped=%v", got, capped)
	}
}
