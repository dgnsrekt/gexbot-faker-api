package eod

import (
	"os"
	"path/filepath"
	"testing"
)

// packFixture writes one classic/gex_full record for date/ticker and packs it
// into an EOD archive (+ manifest), leaving the archive on disk.
func packFixture(t *testing.T, root, date, ticker string, records int) {
	t.Helper()
	src := filepath.Join(root, date, ticker, "classic", "gex_full.jsonl")
	if err := os.MkdirAll(filepath.Dir(src), 0750); err != nil {
		t.Fatal(err)
	}
	line := []byte("{\"timestamp\":1}\n")
	f, err := os.Create(src)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < records; i++ {
		if _, err := f.Write(line); err != nil {
			t.Fatal(err)
		}
	}
	_ = f.Close()
	if _, err := Pack(root, date, ticker, "test"); err != nil {
		t.Fatal(err)
	}
}

func TestListArchives(t *testing.T) {
	root := t.TempDir()
	packFixture(t, root, "2026-08-06", "SPX", 3)
	packFixture(t, root, "2026-08-07", "SPX", 2)
	packFixture(t, root, "2026-08-07", "NDX", 4)

	got, err := ListArchives(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 dates, got %d: %v", len(got), got)
	}

	// Newest first.
	if got[0].Date != "2026-08-07" || got[1].Date != "2026-08-06" {
		t.Fatalf("dates not sorted newest-first: %s, %s", got[0].Date, got[1].Date)
	}

	newest := got[0]
	if len(newest.Tickers) != 2 || newest.Tickers[0] != "NDX" || newest.Tickers[1] != "SPX" {
		t.Errorf("2026-08-07 tickers = %v, want [NDX SPX]", newest.Tickers)
	}
	if newest.Records != 6 { // 2 + 4
		t.Errorf("2026-08-07 records = %d, want 6", newest.Records)
	}
	// Per-ticker snapshots = max member records: SPX 2, NDX 4.
	if newest.SnapshotsByTicker["SPX"] != 2 || newest.SnapshotsByTicker["NDX"] != 4 {
		t.Errorf("2026-08-07 snapshots_by_ticker = %v, want SPX:2 NDX:4", newest.SnapshotsByTicker)
	}
	if len(newest.Packages) != 1 || newest.Packages[0] != "classic" {
		t.Errorf("2026-08-07 packages = %v, want [classic]", newest.Packages)
	}
	if newest.Size <= 0 {
		t.Errorf("2026-08-07 size = %d, want > 0", newest.Size)
	}
	if newest.Status != "ok" {
		t.Errorf("2026-08-07 status = %q, want ok", newest.Status)
	}
}

func TestListArchivesCorruptManifest(t *testing.T) {
	root := t.TempDir()
	packFixture(t, root, "2026-08-07", "SPX", 1)
	// Corrupt the manifest.
	man := ManifestPath(ArchivePath(root, "2026-08-07", "SPX"))
	if err := os.WriteFile(man, []byte("{ not json"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := ListArchives(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Status != "corrupt" {
		t.Fatalf("expected one corrupt archive, got %v", got)
	}
}

func TestMarkMaterialized(t *testing.T) {
	root := t.TempDir()
	date, ticker := "2026-08-07", "SPX"
	packFixture(t, root, date, ticker, 1) // archive present, no marker

	// Before marking: archived (0 materialized).
	got, _ := ListArchives(root)
	if len(got) != 1 || got[0].Materialized != 0 {
		t.Fatalf("pre-mark: %+v, want materialized 0", got)
	}

	// A download would leave JSONL on disk; simulate that + mark.
	writeCatFixture(t, root, date, ticker, "classic", "gex_full")
	if err := MarkMaterialized(root, date, ticker); err != nil {
		t.Fatal(err)
	}
	got, _ = ListArchives(root)
	if got[0].Materialized != 1 {
		t.Errorf("post-mark materialized = %d, want 1", got[0].Materialized)
	}

	// No-op when the ticker's data dir is absent.
	if err := MarkMaterialized(root, date, "NOPE"); err != nil {
		t.Errorf("marking an absent ticker should be a no-op, got %v", err)
	}
}

func TestMarkMaterializedWriteError(t *testing.T) {
	root := t.TempDir()
	date, ticker := "2026-08-07", "SPX"
	// Ticker dir exists (so it's not the no-op path), but the marker path is
	// occupied by a directory, so the marker write must fail — the download
	// worker relies on this error to fail the job instead of falsely reporting
	// the date "ready" with no marker on disk.
	if err := os.MkdirAll(filepath.Join(root, date, ticker, markerName), 0750); err != nil {
		t.Fatal(err)
	}
	if err := MarkMaterialized(root, date, ticker); err == nil {
		t.Error("MarkMaterialized should return an error when the marker cannot be written")
	}
}

func writeCatFixture(t *testing.T, root, date, ticker, pkg, cat string) {
	t.Helper()
	p := filepath.Join(root, date, ticker, pkg, cat+".jsonl")
	if err := os.MkdirAll(filepath.Dir(p), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("{\"timestamp\":1}\n"), 0600); err != nil {
		t.Fatal(err)
	}
}

// TestCoverageIndicesStableUnderComposition is the #42 regression: when the
// ticker set changes but each ticker keeps a constant cadence, the coverage
// index must stay ~1.0 (no artificial steps). A real per-ticker drop moves it.
func TestCoverageIndicesStableUnderComposition(t *testing.T) {
	archives := []ArchiveInfo{
		{Date: "2026-08-05", SnapshotsByTicker: map[string]int{"SPX": 1000, "NDX": 1000, "SPY": 1000}},
		{Date: "2026-08-04", SnapshotsByTicker: map[string]int{"SPX": 1000}},                 // only SPX
		{Date: "2026-08-03", SnapshotsByTicker: map[string]int{"SPX": 1000, "NDX": 1000}},    // no SPY
		{Date: "2026-08-02", SnapshotsByTicker: map[string]int{"SPX": 700, "NDX": 1000}},     // SPX drop
	}
	idx := CoverageIndices(archives)
	// Composition changes (SPY added/removed, SPX-only) but each ticker constant → ~1.0.
	for i := 0; i < 3; i++ {
		if idx[i] < 0.99 || idx[i] > 1.01 {
			t.Errorf("index[%d] (%s) = %.3f, want ~1.0 despite composition change", i, archives[i].Date, idx[i])
		}
	}
	// The real SPX drop (700 vs 1000 median) pulls its date below 1.
	// mean(700/1000, 1000/1000) = 0.85.
	if idx[3] < 0.84 || idx[3] > 0.86 {
		t.Errorf("index[3] (SPX drop) = %.3f, want ~0.85", idx[3])
	}
}

func TestListArchivesMissingDir(t *testing.T) {
	got, err := ListArchives(t.TempDir()) // no eod/ subdir
	if err != nil {
		t.Fatalf("missing eod dir should not error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestListArchivesMaterializedCount(t *testing.T) {
	root := t.TempDir()
	packFixture(t, root, "2026-08-07", "SPX", 1)
	packFixture(t, root, "2026-08-07", "NDX", 1)
	// Materialize only SPX (marker present); NDX stays archive-only.
	marker := filepath.Join(root, "2026-08-07", "SPX", markerName)
	if err := os.MkdirAll(filepath.Dir(marker), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, []byte("archive\n"), 0600); err != nil {
		t.Fatal(err)
	}

	got, err := ListArchives(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 date, got %d", len(got))
	}
	if got[0].Materialized != 1 || len(got[0].Tickers) != 2 {
		t.Errorf("materialized=%d of %d tickers, want 1 of 2", got[0].Materialized, len(got[0].Tickers))
	}
}
