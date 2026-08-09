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
	got, dropped := s.MissedMarketDays("2026-08-05", 90)
	if want := []string{"2026-08-06", "2026-08-07"}; !reflect.DeepEqual(got, want) || dropped {
		t.Fatalf("missed = %v (dropped=%v), want %v (dropped=false)", got, dropped, want)
	}

	// Gap spans only a weekend -> nothing missed.
	if got, _ := s.MissedMarketDays("2026-08-07", 90); len(got) != 0 {
		t.Fatalf("weekend-only gap should be empty, got %v", got)
	}

	// last == today -> nothing.
	if got, _ := s.MissedMarketDays("2026-08-10", 90); len(got) != 0 {
		t.Fatalf("no gap should be empty, got %v", got)
	}

	// Stale lastDate (older than the look-back window): the scan must anchor to
	// today, drop the unfetchable older days, and return only recent dates —
	// never a stale year that all falls outside the window.
	got, dropped = s.MissedMarketDays("2026-01-01", 90)
	if !dropped {
		t.Fatal("stale lastDate should report dropped=true")
	}
	if len(got) == 0 || got[0] < "2026-05-01" {
		t.Fatalf("stale lastDate must return only recent (within-window) days, got first=%v", got)
	}
	if last := got[len(got)-1]; last >= "2026-08-10" {
		t.Fatalf("must exclude today and later, got last=%v", last)
	}
}
