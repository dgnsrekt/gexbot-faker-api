package coverage

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

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

// packMember writes date/ticker's given package/category with the timestamps and
// packs it, for archives whose representative member isn't classic/gex_full.
func packMember(t *testing.T, root, date, ticker, pkg, cat string, ts []int64) {
	t.Helper()
	p := filepath.Join(root, date, ticker, pkg, cat+".jsonl")
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

	rep, err := Check(root, "2026-08-04", zap.NewNop())
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

	rep, err := Check(root, "2026-08-04", zap.NewNop())
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
	repOK, _ := Check(root, "2026-08-06", zap.NewNop())
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

	rep, err := Check(root, date, zap.NewNop())
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

func TestCheckEarlyCloseDayNoFalseAlert(t *testing.T) {
	et, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("no tzdata for America/New_York")
	}
	root := t.TempDir()
	// 2026-11-27 is the day after Thanksgiving — a 13:00 ET half-day.
	date := "2026-11-27"
	if !earlyClose(time.Date(2026, 11, 27, 12, 0, 0, 0, et)) {
		t.Fatal("2026-11-27 should be detected as an early-close day")
	}
	open := time.Date(2026, 11, 27, 9, 30, 0, 0, et).Unix()
	half := seq(open, 3*60*60+30*60+1) // 09:30 → 13:00 at 1/s, complete half-day
	packTs(t, root, date, "SPX", half)

	rep, err := Check(root, date, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	if has(rep, "SPX", "early-close") {
		t.Errorf("a complete 13:00 half-day must not flag early-close, got %+v", rep.Findings)
	}
}

func TestCheckSparseSession(t *testing.T) {
	if _, err := time.LoadLocation("America/New_York"); err != nil {
		t.Skip("no tzdata for America/New_York")
	}
	root := t.TempDir()
	date := "2026-08-07"
	// A single-snapshot archive (extreme truncation) with no baseline: the
	// deviation check is skipped, so session-shape must be what surfaces it.
	packTs(t, root, date, "SPX", []int64{1785000000})

	rep, err := Check(root, date, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	if !has(rep, "SPX", "sparse-session") {
		t.Errorf("a 1-snapshot archive must produce a sparse-session finding, got %+v", rep.Findings)
	}
}

func TestCheckEmptySession(t *testing.T) {
	if _, err := time.LoadLocation("America/New_York"); err != nil {
		t.Skip("no tzdata for America/New_York")
	}
	root := t.TempDir()
	date := "2026-08-07"
	// A zero-snapshot archive (empty member) must not vanish from the check.
	packTs(t, root, date, "SPX", nil)

	rep, err := Check(root, date, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	if !has(rep, "SPX", "sparse-session") {
		t.Errorf("a 0-snapshot archive must produce a sparse-session finding, got %+v", rep.Findings)
	}
}

func TestCheckSessionShapeStateOnlyArchive(t *testing.T) {
	et, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("no tzdata for America/New_York")
	}
	root := t.TempDir()
	date := "2026-08-07"
	// No classic/gex_full — only state/gex_full. Session-shape must still run
	// against the representative member and catch a truncated session.
	open := time.Date(2026, 8, 7, 9, 30, 0, 0, et).Unix()
	truncated := seq(open, 2*60*60) // 09:30 → 11:30 only (early close)
	packMember(t, root, date, "SPX", "state", "gex_full", truncated)

	if _, _, ok := eod.RepresentativeMember(root, date, "SPX"); !ok {
		t.Fatal("representative member should be found for a state-only archive")
	}
	rep, err := Check(root, date, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	if !has(rep, "SPX", "early-close") {
		t.Errorf("session-shape must run on a state-only archive, got %+v", rep.Findings)
	}
}
