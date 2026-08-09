package server

import (
	"context"
	"sort"
	"time"

	"github.com/dgnsrekt/gexbot-downloader/internal/api/generated"
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
// Other tickers get weekly (Friday) expirations only.
var dailyExpiryTickers = map[string]bool{
	"SPX": true, "SPY": true, "QQQ": true, "NDX": true,
	"IWM": true, "XSP": true, "DIA": true, "RUT": true, "VIX": true,
}

// generateExpiries returns option expiration dates (YYYY-MM-DD) within
// [anchor, end], inclusive. Daily-expiry tickers get every weekday for the first
// dailyExpiryWindowDays, then weekly Fridays; others get weekly Fridays only.
// pinned dates (grounded in real data, e.g. anchor+min_dte) are always included
// when inside the horizon.
func generateExpiries(anchor, end time.Time, daily bool, pinned ...time.Time) []string {
	set := map[string]bool{}
	add := func(t time.Time) {
		if !t.Before(anchor) && !t.After(end) {
			set[t.Format("2006-01-02")] = true
		}
	}
	dailyUntil := anchor.AddDate(0, 0, dailyExpiryWindowDays)
	for d := anchor; !d.After(end); d = d.AddDate(0, 0, 1) {
		if wd := d.Weekday(); wd == time.Saturday || wd == time.Sunday {
			continue
		} else if daily && !d.After(dailyUntil) {
			add(d) // daily near-term
		} else if wd == time.Friday {
			add(d) // weekly (and monthly 3rd-Friday) expiration
		}
	}
	for _, p := range pinned {
		add(p)
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
	anchor, err := time.Parse("2006-01-02", s.config.DataDate)
	if err != nil {
		return generated.GetOptionsExpiries404JSONResponse{
			Error: ptr("current date is not a calendar date: " + s.config.DataDate),
		}, nil
	}
	end := anchor.AddDate(0, 0, expiryHorizonDays)
	daily := dailyExpiryTickers[ticker]

	// Ground the nearest expiries in real data: min_dte/sec_min_dte are the
	// days-to-expiry of the nearest two expirations at that date. A 0DTE today
	// also implies this ticker has daily expirations.
	var pinned []time.Time
	if s.loader.Exists(ticker, "classic", "gex_zero") {
		if d, err := s.loader.GetAtIndex(ctx, ticker, "classic", "gex_zero", 0); err == nil && d != nil {
			pinned = append(pinned, anchor.AddDate(0, 0, d.MinDTE), anchor.AddDate(0, 0, d.SecMinDTE))
			if d.MinDTE == 0 {
				daily = true
			}
		}
	}

	return generated.GetOptionsExpiries200JSONResponse{
		Ticker:    ticker,
		StartDate: anchor.Format("2006-01-02"),
		EndDate:   end.Format("2006-01-02"),
		Expiries:  generateExpiries(anchor, end, daily, pinned...),
	}, nil
}
