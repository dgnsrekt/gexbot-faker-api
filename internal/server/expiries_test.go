package server

import (
	"testing"
	"time"
)

func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestGenerateExpiriesHorizonAndOrder(t *testing.T) {
	anchor := mustDate(t, "2026-08-07") // a Friday
	end := anchor.AddDate(0, 0, expiryHorizonDays)
	got := generateExpiries(anchor, end, true)

	if len(got) == 0 {
		t.Fatal("expected expiries")
	}
	// Sorted ascending, all within [anchor, end], no weekends.
	for i, d := range got {
		day := mustDate(t, d)
		if day.Before(anchor) || day.After(end) {
			t.Errorf("%s outside horizon [%s, %s]", d, anchor.Format("2006-01-02"), end.Format("2006-01-02"))
		}
		if wd := day.Weekday(); wd == time.Saturday || wd == time.Sunday {
			t.Errorf("%s is a weekend", d)
		}
		if i > 0 && d <= got[i-1] {
			t.Errorf("not sorted/unique at %d: %s <= %s", i, d, got[i-1])
		}
	}
}

func TestGenerateExpiriesDailyVsWeekly(t *testing.T) {
	anchor := mustDate(t, "2026-08-07")
	end := anchor.AddDate(0, 0, expiryHorizonDays)

	daily := generateExpiries(anchor, end, true)
	weekly := generateExpiries(anchor, end, false)

	// Daily includes consecutive near-term weekdays (Mon-Fri) that weekly omits.
	set := func(xs []string) map[string]bool {
		m := map[string]bool{}
		for _, x := range xs {
			m[x] = true
		}
		return m
	}
	d := set(daily)
	for _, want := range []string{"2026-08-07", "2026-08-10", "2026-08-11", "2026-08-12", "2026-08-13"} {
		if !d[want] {
			t.Errorf("daily calendar missing near-term weekday %s", want)
		}
	}
	// Weekly is Fridays only.
	for _, w := range weekly {
		if mustDate(t, w).Weekday() != time.Friday {
			t.Errorf("weekly calendar has non-Friday %s", w)
		}
	}
	if len(daily) <= len(weekly) {
		t.Errorf("daily (%d) should have more expiries than weekly (%d)", len(daily), len(weekly))
	}
}

func TestGenerateExpiriesPinsRealDates(t *testing.T) {
	anchor := mustDate(t, "2026-08-07")
	end := anchor.AddDate(0, 0, expiryHorizonDays)
	// A pinned mid-week date the weekly calendar wouldn't otherwise include.
	pinned := mustDate(t, "2026-09-15") // a Tuesday
	got := generateExpiries(anchor, end, false, pinned)

	found := false
	for _, d := range got {
		if d == "2026-09-15" {
			found = true
		}
	}
	if !found {
		t.Errorf("pinned real expiry 2026-09-15 not included: %v", got)
	}
	// A pinned date outside the horizon is dropped.
	out := generateExpiries(anchor, end, false, anchor.AddDate(0, 0, 200))
	for _, d := range out {
		if mustDate(t, d).After(end) {
			t.Errorf("out-of-horizon pinned date leaked: %s", d)
		}
	}
}
