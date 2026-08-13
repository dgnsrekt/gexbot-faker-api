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

func TestSchedulerNextRunAt(t *testing.T) {
	s := NewScheduler(17, 5, "America/New_York")
	loc := s.Location()
	at := func(tm time.Time) string {
		s.now = func() time.Time { return tm }
		return s.NextRunAt().In(loc).Format("2006-01-02 15:04")
	}

	// Fri 2026-08-07 16:00, before the 17:05 slot → today's slot.
	if got := at(time.Date(2026, 8, 7, 16, 0, 0, 0, loc)); got != "2026-08-07 17:05" {
		t.Errorf("Fri pre-slot next run = %s, want 2026-08-07 17:05", got)
	}
	// Fri 18:00, slot passed → next market day (Mon, skipping the weekend).
	if got := at(time.Date(2026, 8, 7, 18, 0, 0, 0, loc)); got != "2026-08-10 17:05" {
		t.Errorf("Fri post-slot next run = %s, want 2026-08-10 17:05", got)
	}
	// Sat → Mon.
	if got := at(time.Date(2026, 8, 8, 12, 0, 0, 0, loc)); got != "2026-08-10 17:05" {
		t.Errorf("Sat next run = %s, want 2026-08-10 17:05", got)
	}
}

func TestSchedulerExpectedLatestDate(t *testing.T) {
	s := NewScheduler(17, 5, "America/New_York")
	loc := s.Location()
	cases := []struct {
		when time.Time
		want string
	}{
		{time.Date(2026, 8, 7, 16, 0, 0, 0, loc), "2026-08-06"},  // Fri pre-slot → Thu (today not due yet)
		{time.Date(2026, 8, 7, 18, 0, 0, 0, loc), "2026-08-07"},  // Fri post-slot → Fri
		{time.Date(2026, 8, 8, 12, 0, 0, 0, loc), "2026-08-07"},  // Sat → last market day (Fri)
		{time.Date(2026, 8, 10, 9, 0, 0, 0, loc), "2026-08-07"},  // Mon pre-slot → Fri
		{time.Date(2026, 8, 10, 18, 0, 0, 0, loc), "2026-08-10"}, // Mon post-slot → Mon
	}
	for _, c := range cases {
		s.now = func() time.Time { return c.when }
		if got := s.ExpectedLatestDate(); got != c.want {
			t.Errorf("ExpectedLatestDate(%s) = %s, want %s", c.when.Format("2006-01-02 15:04"), got, c.want)
		}
	}
}
