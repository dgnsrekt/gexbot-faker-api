package eod

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// materializeFixture packs a one-ticker date and materializes it from the
// archive, leaving <root>/<date>/<ticker> with the .eod-materialized marker.
func materializeFixture(t *testing.T, root, date, ticker string) {
	t.Helper()
	src := filepath.Join(root, date, ticker, "classic", "gex_full.jsonl")
	if err := os.MkdirAll(filepath.Dir(src), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("{\"timestamp\":1}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Pack(root, date, ticker, "legacy-jsonl"); err != nil {
		t.Fatal(err)
	}
	if err := PruneTicker(root, date, ticker); err != nil { // drop the source JSONL
		t.Fatal(err)
	}
	if err := MaterializeTicker(root, date, ticker, nil); err != nil { // rebuild + marker
		t.Fatal(err)
	}
}

func TestTouchLoaded(t *testing.T) {
	root := t.TempDir()
	date := "2026-07-17"
	if err := os.MkdirAll(filepath.Join(root, date, "SPY"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := TouchLoaded(root, date); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, date, loadedMarker)
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("marker not written: %v", err)
	}
	// A second touch must advance the mtime.
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(marker, old, old); err != nil {
		t.Fatal(err)
	}
	if err := TouchLoaded(root, date); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(marker); !fi.ModTime().After(old) {
		t.Error("TouchLoaded should advance the marker mtime")
	}
	// No-op (and no dir creation) when the date dir doesn't exist.
	if err := TouchLoaded(root, "2099-01-01"); err != nil {
		t.Fatalf("missing dir should be a no-op: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "2099-01-01")); !os.IsNotExist(err) {
		t.Error("TouchLoaded must not create the date dir")
	}
}

func TestCleanupStale(t *testing.T) {
	root := t.TempDir()
	stale, fresh, keep := "2026-07-06", "2026-07-17", "2026-07-31"
	for _, d := range []string{stale, fresh, keep} {
		materializeFixture(t, root, d, "SPY")
		if err := TouchLoaded(root, d); err != nil {
			t.Fatal(err)
		}
	}
	// Backdate the stale date's marker to 10 days ago; the others stay fresh.
	old := time.Now().Add(-10 * 24 * time.Hour)
	if err := os.Chtimes(filepath.Join(root, stale, loadedMarker), old, old); err != nil {
		t.Fatal(err)
	}
	// A raw (unmarked) date must never be evicted even if old.
	raw := "2026-06-01"
	rawFile := filepath.Join(root, raw, "SPY", "classic", "gex_full.jsonl")
	if err := os.MkdirAll(filepath.Dir(rawFile), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rawFile, []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	os.Chtimes(filepath.Join(root, raw), old, old)

	removed, err := CleanupStale(root, 7*24*time.Hour, nil, keep)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 1 || removed[0] != stale {
		t.Fatalf("expected only %s evicted, got %v", stale, removed)
	}
	if _, err := os.Stat(filepath.Join(root, stale)); !os.IsNotExist(err) {
		t.Errorf("%s (10d idle) should be evicted", stale)
	}
	for _, d := range []string{fresh, keep, raw} {
		if _, err := os.Stat(filepath.Join(root, d)); err != nil {
			t.Errorf("%s should be kept, got %v", d, err)
		}
	}
	// The archive survives, so an evicted date re-materializes on demand.
	if err := MaterializeTicker(root, stale, "SPY", nil); err != nil {
		t.Fatalf("evicted date should re-materialize from archive: %v", err)
	}
}

func TestPackVerifyMaterialize(t *testing.T) {
	root := t.TempDir()
	date, ticker := "2026-07-17", "SPY"
	source := filepath.Join(root, date, ticker, "classic", "gex_full.jsonl")
	if err := os.MkdirAll(filepath.Dir(source), 0750); err != nil {
		t.Fatal(err)
	}
	want := []byte("{\"timestamp\":1,\"value\":2}\n{\"timestamp\":2,\"value\":3}\n")
	if err := os.WriteFile(source, want, 0600); err != nil {
		t.Fatal(err)
	}

	manifest, err := Pack(root, date, ticker, "legacy-jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Members) != 1 || manifest.Members[0].Records != 2 {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	if err := PruneTicker(root, date, ticker); err != nil {
		t.Fatal(err)
	}
	if err := MaterializeTicker(root, date, ticker, nil); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("round trip mismatch:\n%s\nwant:\n%s", got, want)
	}
}

func TestPruneRemovesEmptyDateDir(t *testing.T) {
	root := t.TempDir()
	date := "2026-07-17"
	for _, tk := range []string{"SPY", "SPX"} {
		src := filepath.Join(root, date, tk, "classic", "gex_full.jsonl")
		if err := os.MkdirAll(filepath.Dir(src), 0750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(src, []byte("{\"timestamp\":1}\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Pack(root, date, tk, "legacy-jsonl"); err != nil {
			t.Fatal(err)
		}
	}
	// Pruning one of two tickers must leave the date dir (SPX still present).
	if err := PruneTicker(root, date, "SPY"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, date)); err != nil {
		t.Fatalf("date dir should remain while another ticker is present: %v", err)
	}
	// Pruning the last ticker must remove the now-empty date dir.
	if err := PruneTicker(root, date, "SPX"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, date)); !os.IsNotExist(err) {
		t.Fatalf("empty date dir should be removed after last prune, got err=%v", err)
	}
}

func TestConcurrentMaterializeTicker(t *testing.T) {
	root := t.TempDir()
	date, ticker := "2026-07-17", "SPY"
	source := filepath.Join(root, date, ticker, "classic", "gex_full.jsonl")
	if err := os.MkdirAll(filepath.Dir(source), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("{\"timestamp\":1}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Pack(root, date, ticker, "test"); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, date)); err != nil {
		t.Fatal(err)
	}

	errs := make(chan error, 2)
	var start sync.WaitGroup
	start.Add(1)
	for range 2 {
		go func() {
			start.Wait()
			errs <- MaterializeTicker(root, date, ticker, nil)
		}()
	}
	start.Done()
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatal(err)
	}
}

func TestMaterializeDateLogsProgress(t *testing.T) {
	root := t.TempDir()
	date, ticker := "2026-07-17", "SPY"
	source := filepath.Join(root, date, ticker, "classic", "gex_full.jsonl")
	if err := os.MkdirAll(filepath.Dir(source), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("{\"timestamp\":1}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Pack(root, date, ticker, "legacy-jsonl"); err != nil {
		t.Fatal(err)
	}
	// Remove the JSONL so materialize actually does work (and logs it).
	if err := PruneTicker(root, date, ticker); err != nil {
		t.Fatal(err)
	}

	core, logs := observer.New(zap.InfoLevel)
	if err := MaterializeDate(root, date, zap.New(core)); err != nil {
		t.Fatal(err)
	}
	if logs.FilterMessage("materializing date from EOD archive").Len() == 0 {
		t.Error("expected a start log for the date materialize")
	}
	if logs.FilterMessage("materialized ticker from EOD archive").Len() == 0 {
		t.Error("expected a per-ticker materialize log")
	}
	if logs.FilterMessage("materialized date from EOD archive").Len() == 0 {
		t.Error("expected a date-summary materialize log")
	}
}
