package server

import (
	"context"
	"sort"
	"time"

	"github.com/dgnsrekt/gexbot-downloader/internal/api/generated"
	"github.com/dgnsrekt/gexbot-downloader/internal/config"
)

const (
	// expiryHorizonDays is how far the published expiry window reaches, matching
	// the live API (current date + 90 calendar days).
	expiryHorizonDays = 90
	// dailyExpiryWindowDays is how long near-term daily expirations run before the
	// generated calendar thins to weekly.
	dailyExpiryWindowDays = 28
)

// dailyExpiryTickers have 0DTE / daily option expirations (every trading day).
// Classification is by ticker, independent of days-to-expiry — a weekly-only
// stock loaded on a Friday also has a 0DTE that day but is not a daily ticker.
var dailyExpiryTickers = map[string]bool{
	"SPX": true, "SPY": true, "QQQ": true, "NDX": true,
	"IWM": true, "XSP": true, "DIA": true, "RUT": true, "VIX": true,
}

// nthWeekday returns the nth (1-based) given weekday of a month.
func nthWeekday(year int, month time.Month, wd time.Weekday, n int) time.Time {
	first := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	offset := (int(wd) - int(first.Weekday()) + 7) % 7
	return first.AddDate(0, 0, offset+(n-1)*7)
}

// lastWeekday returns the last given weekday of a month.
func lastWeekday(year int, month time.Month, wd time.Weekday) time.Time {
	last := time.Date(year, month+1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -1)
	offset := (int(last.Weekday()) - int(wd) + 7) % 7
	return last.AddDate(0, 0, -offset)
}

// easterSunday computes Western Easter (anonymous Gregorian algorithm).
func easterSunday(year int) time.Time {
	a := year % 19
	b := year / 100
	c := year % 100
	d := b / 4
	e := b % 4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	h := (19*a + b - d - g + 15) % 30
	i := c / 4
	k := c % 4
	l := (32 + 2*e + 2*i - h - k) % 7
	m := (a + 11*h + 22*l) / 451
	month := (h + l - 7*m + 114) / 31
	day := ((h + l - 7*m + 114) % 31) + 1
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

// observedHoliday shifts a fixed-date holiday to the NYSE-observed day (Sat -> Fri,
// Sun -> Mon).
func observedHoliday(d time.Time) time.Time {
	switch d.Weekday() {
	case time.Saturday:
		return d.AddDate(0, 0, -1)
	case time.Sunday:
		return d.AddDate(0, 0, 1)
	}
	return d
}

// usMarketHolidays returns the NYSE full-day closures for a year as a set of
// YYYY-MM-DD strings.
func usMarketHolidays(year int) map[string]bool {
	fixed := []time.Time{
		observedHoliday(time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)),   // New Year's
		nthWeekday(year, time.January, time.Monday, 3),                            // MLK
		nthWeekday(year, time.February, time.Monday, 3),                           // Presidents
		easterSunday(year).AddDate(0, 0, -2),                                      // Good Friday
		lastWeekday(year, time.May, time.Monday),                                  // Memorial
		observedHoliday(time.Date(year, time.June, 19, 0, 0, 0, 0, time.UTC)),     // Juneteenth
		observedHoliday(time.Date(year, time.July, 4, 0, 0, 0, 0, time.UTC)),      // Independence
		nthWeekday(year, time.September, time.Monday, 1),                          // Labor
		nthWeekday(year, time.November, time.Thursday, 4),                         // Thanksgiving
		observedHoliday(time.Date(year, time.December, 25, 0, 0, 0, 0, time.UTC)), // Christmas
	}
	set := make(map[string]bool, len(fixed))
	for _, d := range fixed {
		set[d.Format("2006-01-02")] = true
	}
	return set
}

// generateExpiries returns valid option expiration dates (YYYY-MM-DD) within
// [anchor, end], inclusive. It uses the NYSE session/holiday calendar: daily
// tickers get every trading day for the first dailyExpiryWindowDays; all tickers
// get weekly expirations (Fridays, shifted earlier when the Friday is closed).
// pinned dates come from real data and are trusted (horizon-bounded only).
func generateExpiries(anchor, end time.Time, daily bool, pinned ...time.Time) []string {
	holidays := map[string]bool{}
	for y := anchor.Year(); y <= end.Year(); y++ {
		for k := range usMarketHolidays(y) {
			holidays[k] = true
		}
	}
	isTradingDay := func(t time.Time) bool {
		if wd := t.Weekday(); wd == time.Saturday || wd == time.Sunday {
			return false
		}
		return !holidays[t.Format("2006-01-02")]
	}

	set := map[string]bool{}
	add := func(t time.Time) {
		if !t.Before(anchor) && !t.After(end) && isTradingDay(t) {
			set[t.Format("2006-01-02")] = true
		}
	}
	dailyUntil := anchor.AddDate(0, 0, dailyExpiryWindowDays)
	for d := anchor; !d.After(end); d = d.AddDate(0, 0, 1) {
		if daily && !d.After(dailyUntil) {
			add(d) // daily near-term (add() skips weekends/holidays)
		}
		if d.Weekday() == time.Friday {
			// Weekly expiration; roll earlier when the Friday is a market holiday.
			exp := d
			for !isTradingDay(exp) && !exp.Before(anchor) {
				exp = exp.AddDate(0, 0, -1)
			}
			add(exp)
		}
	}
	// Pinned expiries are grounded in real data — trust them, bound only by horizon.
	for _, p := range pinned {
		if !p.Before(anchor) && !p.After(end) {
			set[p.Format("2006-01-02")] = true
		}
	}

	dates := make([]string, 0, len(set))
	for d := range set {
		dates = append(dates, d)
	}
	sort.Strings(dates)
	return dates
}

// GetOptionsExpiries implements generated.StrictServerInterface. It returns the
// option expiries for the loaded date's 90-day horizon, grounding the near-term
// dates in the loaded data's min_dte/sec_min_dte when available.
func (s *Server) GetOptionsExpiries(ctx context.Context, request generated.GetOptionsExpiriesRequestObject) (generated.GetOptionsExpiriesResponseObject, error) {
	ticker := request.Ticker
	// Only supported option-owning tickers (stocks/indexes) — not futures.
	if !config.ValidTickers[ticker] || config.FutureTickers[ticker] {
		return generated.GetOptionsExpiries400JSONResponse{
			Error: ptr("unsupported option ticker: " + ticker),
		}, nil
	}
	anchor, err := time.Parse("2006-01-02", s.config.DataDate)
	if err != nil {
		return generated.GetOptionsExpiries404JSONResponse{
			Error: ptr("current date is not a calendar date: " + s.config.DataDate),
		}, nil
	}
	end := anchor.AddDate(0, 0, expiryHorizonDays)
	daily := dailyExpiryTickers[ticker]

	// Ground the nearest expiries in real data: min_dte/sec_min_dte are the
	// days-to-expiry of the nearest two expirations at that date.
	var pinned []time.Time
	if s.loader.Exists(ticker, "classic", "gex_zero") {
		if d, err := s.loader.GetAtIndex(ctx, ticker, "classic", "gex_zero", 0); err == nil && d != nil {
			pinned = append(pinned, anchor.AddDate(0, 0, d.MinDTE), anchor.AddDate(0, 0, d.SecMinDTE))
		}
	}

	return generated.GetOptionsExpiries200JSONResponse{
		Ticker:    ticker,
		StartDate: anchor.Format("2006-01-02"),
		EndDate:   end.Format("2006-01-02"),
		Expiries:  generateExpiries(anchor, end, daily, pinned...),
	}, nil
}
