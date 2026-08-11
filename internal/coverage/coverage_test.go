package coverage

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dgnsrekt/gexbot-downloader/internal/eod"
)

func packTs(t *testing.T, root, date, ticker string, ts []int64) {
	t.Helper()
	p := filepath.Join(root, date, ticker, "classic", "gex_full.jsonl")
	if err := os.MkdirAll(filepath.Dir(p), 0750); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, v := range ts {
		b.WriteString(`{"timestamp":` + strconv.FormatInt(v, 10) + "}\n")
	}
	if err := os.WriteFile(p, []byte(b.String()), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := eod.Pack(root, date, ticker, "test"); err != nil {
		t.Fatal(err)
	}
}

// seq returns n timestamps one second apart starting at start.
func seq(start int64, n int) []int64 {
	out := make([]int64, n)
	for i := range out {
		out[i] = start + int64(i)
	}
	return out
}

func has(rep Report, ticker, kind string) bool {
	for _, f := range rep.Findings {
		if f.Ticker == ticker && f.Kind == kind {
			return true
		}
	}
	return false
}

func TestCheckLowSnapshots(t *testing.T) {
	root := t.TempDir()
	// 6 baseline days at 100 snapshots each; current day drops to 70 (-30%).
	base := []string{"2026-07-27", "2026-07-28", "2026-07-29", "2026-07-30", "2026-07-31", "2026-08-03"}
	for _, d := range base {
		packTs(t, root, d, "SPX", seq(1000, 100))
	}
	packTs(t, root, "2026-08-04", "SPX", seq(1000, 70))

	rep, err := Check(root, "2026-08-04")
	if err != nil {
		t.Fatal(err)
	}
	if !has(rep, "SPX", "low-snapshots") {
		t.Errorf("expected a low-snapshots finding for SPX, got %+v", rep.Findings)
	}
}

func TestCheckThinBaselineNoDeviation(t *testing.T) {
	root := t.TempDir()
	// Only 3 prior days (< minBaseline) — deviation must be skipped even on a drop.
	for _, d := range []string{"2026-08-01", "2026-08-02", "2026-08-03"} {
		packTs(t, root, d, "SPX", seq(1000, 100))
	}
	packTs(t, root, "2026-08-04", "SPX", seq(1000, 10))

	rep, err := Check(root, "2026-08-04")
	if err != nil {
		t.Fatal(err)
	}
	if has(rep, "SPX", "low-snapshots") {
		t.Errorf("thin baseline should not produce a deviation finding, got %+v", rep.Findings)
	}
}

func TestCheckSessionShape(t *testing.T) {
	et, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("no tzdata for America/New_York")
	}
	root := t.TempDir()
	date := "2026-08-07"

	// A healthy session: 09:30:00 → 16:00:00 ET, 1s cadence (dense, no gaps).
	open := time.Date(2026, 8, 7, 9, 30, 0, 0, et).Unix()
	healthy := seq(open, 6*60*60+30*60+1) // full 6.5h at 1/s
	packTs(t, root, "2026-08-06", "SPY", healthy)
	repOK, _ := Check(root, "2026-08-06")
	for _, f := range repOK.Findings {
		if f.Ticker == "SPY" {
			t.Errorf("healthy session flagged: %+v", f)
		}
	}

	// A truncated session: opens 09:30 but stops at ~14:00 (early close), and a
	// 5-minute hole partway through (gap).
	morning := seq(open, 2*60*60)               // 09:30 → 11:30
	afternoon := seq(open+2*60*60+300, 90*60)   // resume 5 min later → ~14:00
	packTs(t, root, date, "SPX", append(morning, afternoon...))

	rep, err := Check(root, date)
	if err != nil {
		t.Fatal(err)
	}
	if !has(rep, "SPX", "early-close") {
		t.Errorf("expected early-close for the truncated session, got %+v", rep.Findings)
	}
	if !has(rep, "SPX", "gap") {
		t.Errorf("expected a gap finding for the 5-minute hole, got %+v", rep.Findings)
	}
}
