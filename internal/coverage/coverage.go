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
	// openTol/closeTol: tolerance around the 09:30–16:00 ET regular session.
	openTol  = 5 * time.Minute
	closeTol = 5 * time.Minute

	sessionPkg = "classic"
	sessionCat = "gex_full"
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
func Check(dataDir, date string) (Report, error) {
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
			rep.Findings = append(rep.Findings, sessionShape(dataDir, date, tk, et)...)
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
// open, early close, or oversized intraday gap. Best-effort: a read error (or a
// member with too few points) yields no findings.
func sessionShape(dataDir, date, ticker string, et *time.Location) []Finding {
	ts, err := eod.MemberTimestamps(dataDir, date, ticker, sessionPkg, sessionCat)
	if err != nil || len(ts) < 2 {
		return nil
	}
	sort.Slice(ts, func(i, j int) bool { return ts[i] < ts[j] })
	first := time.Unix(ts[0], 0).In(et)
	last := time.Unix(ts[len(ts)-1], 0).In(et)
	open := time.Date(first.Year(), first.Month(), first.Day(), 9, 30, 0, 0, et)
	close := time.Date(first.Year(), first.Month(), first.Day(), 16, 0, 0, 0, et)

	var out []Finding
	if first.After(open.Add(openTol)) {
		out = append(out, Finding{ticker, "late-open",
			fmt.Sprintf("first snapshot %s ET (expected ~09:30)", first.Format("15:04:05"))})
	}
	if last.Before(close.Add(-closeTol)) {
		out = append(out, Finding{ticker, "early-close",
			fmt.Sprintf("last snapshot %s ET (expected ~16:00)", last.Format("15:04:05"))})
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
