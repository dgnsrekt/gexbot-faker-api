// Package coverage inspects freshly downloaded EOD data and flags coverage
// regressions — a ticker whose intraday snapshot count drops well below its
// recent norm, or a session that opens late / closes early / has a large gap.
// It exists so a silent data-source change (e.g. a feed thinning SPX/NDX
// sampling) is caught the day it happens instead of by eyeballing record counts.
package coverage

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/eod"
)

var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// priorDates returns up to n archived date dirs strictly before date, newest first.
func priorDates(dataDir, date string, n int) []string {
	entries, err := os.ReadDir(filepath.Join(dataDir, "eod"))
	if err != nil {
		return nil
	}
	var dates []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() && dateRe.MatchString(name) && name < date {
			dates = append(dates, name)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))
	if len(dates) > n {
		dates = dates[:n]
	}
	return dates
}

const (
	// deviationPct: alert when a ticker's snapshots fall this far below its median.
	deviationPct = 0.10
	// baselineDays: rolling window of prior dates for the per-ticker median.
	baselineDays = 20
	// minBaseline: need at least this many prior days to judge a deviation.
	minBaseline = 5
	// maxGapSec: alert when the largest intraday gap exceeds this.
	maxGapSec = 120
	// openTol/closeTol: tolerance around the scheduled session bounds (ET).
	openTol  = 5 * time.Minute
	closeTol = 5 * time.Minute
)

// Finding is one coverage problem for one ticker.
type Finding struct {
	Ticker string
	Kind   string // low-snapshots | late-open | early-close | gap
	Detail string
}

// Report is the result of checking one date.
type Report struct {
	Date     string
	Findings []Finding
}

// Empty reports whether the date passed all checks.
func (r Report) Empty() bool { return len(r.Findings) == 0 }

// Check inspects date's archives under dataDir and returns any coverage findings.
// It reads manifests for the snapshot-deviation check (cheap) and one member per
// ticker for the session-shape check. A thin baseline (fewer than minBaseline
// prior days) skips the deviation check for that ticker rather than guessing.
// logger (may be nil) records when a session-shape check could not run, so a
// skipped check is never silent.
func Check(dataDir, date string, logger *zap.Logger) (Report, error) {
	if logger == nil {
		logger = zap.NewNop()
	}
	rep := Report{Date: date}
	cur, err := eod.TickerSnapshots(dataDir, date)
	if err != nil {
		return rep, err
	}
	if len(cur) == 0 {
		return rep, nil
	}

	baseline := gatherBaseline(dataDir, date)

	tickers := make([]string, 0, len(cur))
	for tk := range cur {
		tickers = append(tickers, tk)
	}
	sort.Strings(tickers)

	et, etErr := time.LoadLocation("America/New_York")
	if etErr != nil {
		logger.Warn("coverage: session-shape checks skipped (no America/New_York tzdata)", zap.Error(etErr))
	}
	for _, tk := range tickers {
		if hist := baseline[tk]; len(hist) >= minBaseline {
			if med := median(hist); med > 0 {
				dev := float64(cur[tk]-med) / float64(med)
				if dev <= -deviationPct {
					rep.Findings = append(rep.Findings, Finding{
						Ticker: tk, Kind: "low-snapshots",
						Detail: fmt.Sprintf("%d snapshots, %.0f%% below the %d-day median of %d",
							cur[tk], -dev*100, len(hist), med),
					})
				}
			}
		}
		if etErr == nil {
			rep.Findings = append(rep.Findings, sessionShape(dataDir, date, tk, et, logger)...)
		}
	}
	return rep, nil
}

// gatherBaseline collects, per ticker, the snapshot counts of up to baselineDays
// archived dates before date.
func gatherBaseline(dataDir, date string) map[string][]int {
	out := map[string][]int{}
	for _, d := range priorDates(dataDir, date, baselineDays) {
		s, err := eod.TickerSnapshots(dataDir, d)
		if err != nil {
			continue
		}
		for tk, n := range s {
			out[tk] = append(out[tk], n)
		}
	}
	return out
}

// sessionShape reads one representative member's timestamps and flags a late
// open, early close, or oversized intraday gap. The expected close is date-aware
// (13:00 ET on NYSE half-days, else 16:00) so legitimate early-close sessions
// don't false-alarm. When no member can be read, it logs rather than silently
// passing.
func sessionShape(dataDir, date, ticker string, et *time.Location, logger *zap.Logger) []Finding {
	pkg, cat, ok := eod.RepresentativeMember(dataDir, date, ticker)
	if !ok {
		logger.Warn("coverage: session-shape skipped, no readable member",
			zap.String("date", date), zap.String("ticker", ticker))
		return nil
	}
	ts, err := eod.MemberTimestamps(dataDir, date, ticker, pkg, cat)
	if err != nil {
		logger.Warn("coverage: session-shape skipped, member unreadable",
			zap.String("date", date), zap.String("ticker", ticker),
			zap.String("member", pkg+"/"+cat), zap.Error(err))
		return nil
	}
	if len(ts) < 2 {
		return nil
	}
	sort.Slice(ts, func(i, j int) bool { return ts[i] < ts[j] })
	first := time.Unix(ts[0], 0).In(et)
	last := time.Unix(ts[len(ts)-1], 0).In(et)
	open := time.Date(first.Year(), first.Month(), first.Day(), 9, 30, 0, 0, et)
	closeH, closeM := 16, 0
	if earlyClose(first) {
		closeH = 13 // NYSE half-day close is 13:00 ET
	}
	close := time.Date(first.Year(), first.Month(), first.Day(), closeH, closeM, 0, 0, et)

	var out []Finding
	if first.After(open.Add(openTol)) {
		out = append(out, Finding{ticker, "late-open",
			fmt.Sprintf("first snapshot %s ET (expected ~09:30)", first.Format("15:04:05"))})
	}
	if last.Before(close.Add(-closeTol)) {
		out = append(out, Finding{ticker, "early-close",
			fmt.Sprintf("last snapshot %s ET (expected ~%s)", last.Format("15:04:05"), close.Format("15:04"))})
	}
	var maxGap int64
	for i := 1; i < len(ts); i++ {
		if g := ts[i] - ts[i-1]; g > maxGap {
			maxGap = g
		}
	}
	if maxGap > maxGapSec {
		out = append(out, Finding{ticker, "gap",
			fmt.Sprintf("largest intraday gap %ds (threshold %ds)", maxGap, maxGapSec)})
	}
	return out
}

// earlyClose reports whether d is a NYSE half-day (13:00 ET close): the day
// after Thanksgiving, July 3, and December 24 when each is a weekday. These are
// the recurring 1:00 PM ET early closes; the exact-date rules avoid a false
// early-close finding on legitimately short sessions.
func earlyClose(d time.Time) bool {
	if wd := d.Weekday(); wd == time.Saturday || wd == time.Sunday {
		return false
	}
	// Day after Thanksgiving (4th Thursday of November + 1 day).
	thanksgiving := nthWeekday(d.Year(), time.November, time.Thursday, 4)
	if sameYMD(d, thanksgiving.AddDate(0, 0, 1)) {
		return true
	}
	if d.Month() == time.July && d.Day() == 3 {
		return true
	}
	if d.Month() == time.December && d.Day() == 24 {
		return true
	}
	return false
}

// nthWeekday returns the date of the nth given weekday in month/year.
func nthWeekday(year int, month time.Month, wd time.Weekday, n int) time.Time {
	first := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	offset := (int(wd) - int(first.Weekday()) + 7) % 7
	return first.AddDate(0, 0, offset+(n-1)*7)
}

func sameYMD(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func median(xs []int) int {
	v := append([]int(nil), xs...)
	sort.Ints(v)
	n := len(v)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return v[n/2]
	}
	return (v[n/2-1] + v[n/2]) / 2
}
