package server

import (
	"context"
	"testing"
	"time"

	"github.com/dgnsrekt/gexbot-downloader/internal/api/generated"
	"github.com/dgnsrekt/gexbot-downloader/internal/config"
	"github.com/dgnsrekt/gexbot-downloader/internal/data"
)

func dateSet(xs []string) map[string]bool {
	m := make(map[string]bool, len(xs))
	for _, x := range xs {
		m[x] = true
	}
	return m
}

// stubExpiryLoader is a minimal DataLoader for the expiries handler tests.
type stubExpiryLoader struct {
	exists bool
	minDTE int
	secDTE int
}

func (s stubExpiryLoader) GetAtIndex(_ context.Context, _, _, _ string, _ int) (*data.GexData, error) {
	return &data.GexData{MinDTE: s.minDTE, SecMinDTE: s.secDTE}, nil
}
func (s stubExpiryLoader) GetRawAtIndex(_ context.Context, _, _, _ string, _ int) ([]byte, error) {
	return nil, nil
}
func (s stubExpiryLoader) GetLength(_, _, _ string) (int, error) { return 1, nil }
func (s stubExpiryLoader) Exists(_, _, _ string) bool            { return s.exists }
func (s stubExpiryLoader) GetLoadedKeys() []string               { return nil }
func (s stubExpiryLoader) FindIndexByTimestamp(_ context.Context, _, _, _ string, _ int64) (int, int64, error) {
	return 0, 0, nil
}
func (s stubExpiryLoader) Close() error { return nil }

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

func TestGenerateExpiriesExcludesHolidays(t *testing.T) {
	// Window spanning observed Independence Day: 2026-07-04 is a Saturday, so the
	// market closes Friday 2026-07-03.
	anchor := mustDate(t, "2026-06-26")
	end := anchor.AddDate(0, 0, expiryHorizonDays)
	got := dateSet(generateExpiries(anchor, end, true))

	if got["2026-07-03"] {
		t.Error("observed Independence Day 2026-07-03 must not be an expiry")
	}
	// The weekly expiration shifts to the prior trading day (Thursday 2026-07-02).
	if !got["2026-07-02"] {
		t.Error("weekly expiry should shift to 2026-07-02 when the Friday is a holiday")
	}
}

func TestGenerateExpiriesGoodFridayShift(t *testing.T) {
	// Good Friday 2026 is 2026-04-03 (a Friday market holiday).
	anchor := mustDate(t, "2026-03-27")
	end := anchor.AddDate(0, 0, expiryHorizonDays)
	got := dateSet(generateExpiries(anchor, end, false)) // weekly-only ticker

	if got["2026-04-03"] {
		t.Error("Good Friday 2026-04-03 must not be an expiry")
	}
	if !got["2026-04-02"] {
		t.Error("weekly expiry should shift to Thursday 2026-04-02 on Good Friday week")
	}
}

func TestGenerateExpiriesWeeklyStockOnFriday(t *testing.T) {
	// A weekly-only ticker (daily=false) loaded on a Friday must produce Fridays
	// only — no Mon-Thu contracts.
	anchor := mustDate(t, "2026-08-07") // Friday
	end := anchor.AddDate(0, 0, expiryHorizonDays)
	got := generateExpiries(anchor, end, false)
	for _, d := range got {
		if mustDate(t, d).Weekday() != time.Friday {
			t.Errorf("weekly-only calendar has non-Friday %s", d)
		}
	}
	if !dateSet(got)["2026-08-07"] {
		t.Error("anchor Friday should be a weekly expiry")
	}
}

func TestOptionsExpiriesWeeklyStockStaysWeekly(t *testing.T) {
	// A weekly stock loaded on a Friday has min_dte==0 (an expiry today) but must
	// NOT be classified daily — regression for the removed min_dte inference.
	const ticker = "AAPL"
	if !config.ValidTickers[ticker] || config.FutureTickers[ticker] || dailyExpiryTickers[ticker] {
		t.Skipf("%s is not a valid weekly stock in this build", ticker)
	}
	srv := &Server{
		loader: stubExpiryLoader{exists: true, minDTE: 0, secDTE: 7},
		config: &config.ServerConfig{DataDate: "2026-08-07"}, // Friday
	}
	resp, err := srv.GetOptionsExpiries(context.Background(),
		generated.GetOptionsExpiriesRequestObject{Ticker: ticker})
	if err != nil {
		t.Fatal(err)
	}
	ok, isOK := resp.(generated.GetOptionsExpiries200JSONResponse)
	if !isOK {
		t.Fatalf("expected 200, got %T", resp)
	}
	for _, d := range ok.Expiries {
		if mustDate(t, d).Weekday() != time.Friday {
			t.Errorf("weekly stock produced non-Friday expiry %s (min_dte=0 wrongly treated as daily)", d)
		}
	}
}

func TestOptionsExpiriesUnknownTicker(t *testing.T) {
	srv := &Server{
		loader: stubExpiryLoader{},
		config: &config.ServerConfig{DataDate: "2026-08-07"},
	}
	resp, err := srv.GetOptionsExpiries(context.Background(),
		generated.GetOptionsExpiriesRequestObject{Ticker: "ZZZZ"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := resp.(generated.GetOptionsExpiries400JSONResponse); !ok {
		t.Errorf("unsupported ticker should return 400, got %T", resp)
	}
}
