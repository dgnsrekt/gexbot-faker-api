package eod

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/dgnsrekt/gexbot-downloader/internal/offsetindex"
)

// packDate packs each ticker into an EOD archive under <root>/eod/<date> and prunes the
// source JSONL, so a subsequent MaterializeDate must rebuild every ticker from its archive.
func packDate(t *testing.T, root, date string, tickers ...string) {
	t.Helper()
	for _, tk := range tickers {
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
		if err := PruneTicker(root, date, tk); err != nil {
			t.Fatal(err)
		}
	}
}

// assertMaterialized checks a ticker's JSONL, offset-index sidecar, and marker are present.
func assertMaterialized(t *testing.T, root, date, ticker string) {
	t.Helper()
	jsonl := filepath.Join(root, date, ticker, "classic", "gex_full.jsonl")
	if _, err := os.Stat(jsonl); err != nil {
		t.Errorf("%s: JSONL missing: %v", ticker, err)
	}
	if _, err := os.Stat(offsetindex.SidecarPath(jsonl)); err != nil {
		t.Errorf("%s: offset-index sidecar missing: %v", ticker, err)
	}
	if _, err := os.Stat(filepath.Join(root, date, ticker, markerName)); err != nil {
		t.Errorf("%s: materialize marker missing: %v", ticker, err)
	}
}

// TestMaterializeRepairsMissingMarker is the 2026-07-28 regression: a ticker dir with
// materialized JSONL but no .eod-materialized marker (e.g. created by an older build)
// must get its marker stamped on re-materialize, so the Studio recognizes it as ready
// instead of showing "Materialize" forever.
func TestMaterializeRepairsMissingMarker(t *testing.T) {
	root := t.TempDir()
	date, ticker := "2026-07-28", "AAPL"
	packDate(t, root, date, ticker) // archives + prunes source
	if err := MaterializeDate(root, date, zap.NewNop()); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, date, ticker, markerName)
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("precondition: marker should exist after materialize: %v", err)
	}
	// Simulate a legacy dir: JSONL present, marker removed.
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	if got, _ := ListArchives(root); got[0].Materialized != 0 {
		t.Fatalf("without marker, Materialized should be 0, got %d", got[0].Materialized)
	}
	// Re-materialize must repair the marker (not no-op).
	if err := MaterializeDate(root, date, zap.NewNop()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Errorf("re-materialize must stamp the missing marker: %v", err)
	}
	if got, _ := ListArchives(root); got[0].Materialized != 1 {
		t.Errorf("after repair, Materialized should be 1, got %d", got[0].Materialized)
	}
	// The JSONL must be untouched (repair only writes the marker).
	jsonl := filepath.Join(root, date, ticker, "classic", "gex_full.jsonl")
	if _, err := os.Stat(jsonl); err != nil {
		t.Errorf("repair must not disturb the JSONL: %v", err)
	}
}

func TestMaterializeDateMultiTicker(t *testing.T) {
	root := t.TempDir()
	date := "2026-07-17"
	tickers := []string{"AAPL", "NVDA", "SPX", "SPY", "QQQ"}
	packDate(t, root, date, tickers...)

	if err := MaterializeDate(root, date, zap.NewNop()); err != nil {
		t.Fatalf("MaterializeDate: %v", err)
	}
	for _, tk := range tickers {
		assertMaterialized(t, root, date, tk)
	}
	archives, err := ListArchives(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(archives) != 1 || archives[0].Materialized != len(tickers) {
		t.Fatalf("Materialized = %d, want %d", archives[0].Materialized, len(tickers))
	}
}

func TestMaterializeDateCorruptAmongHealthy(t *testing.T) {
	root := t.TempDir()
	date := "2026-07-17"
	healthy := []string{"AAPL", "SPY"}
	corrupt := "NVDA"
	packDate(t, root, date, append(append([]string{}, healthy...), corrupt)...)

	// Save the good archive so we can restore it for the retry, then corrupt it.
	archive := ArchivePath(root, date, corrupt)
	good, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archive, []byte("not a zip"), 0600); err != nil {
		t.Fatal(err)
	}

	err = MaterializeDate(root, date, zap.NewNop())
	if err == nil {
		t.Fatal("expected an error from the corrupt ticker")
	}
	if !strings.Contains(err.Error(), corrupt) {
		t.Errorf("error should name the corrupt ticker %q: %v", corrupt, err)
	}
	// Healthy tickers still materialized despite the failure.
	for _, tk := range healthy {
		assertMaterialized(t, root, date, tk)
	}
	// The corrupt ticker left no promoted destination...
	if _, err := os.Stat(filepath.Join(root, date, corrupt)); !os.IsNotExist(err) {
		t.Errorf("corrupt ticker must not have a promoted dest, err=%v", err)
	}
	// ...and no staging residue.
	staging := filepath.Join(root, ".eod-staging", date)
	if entries, err := os.ReadDir(staging); err == nil {
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), corrupt+"-") {
				t.Errorf("corrupt ticker left staging residue: %s", e.Name())
			}
		}
	}
	archives, _ := ListArchives(root)
	if archives[0].Materialized != len(healthy) {
		t.Errorf("Materialized = %d, want %d (successes only)", archives[0].Materialized, len(healthy))
	}

	// Retry after replacing the archive: healthy tickers skip (dest exists), the
	// previously-corrupt one finishes, and the date completes with no error.
	if err := os.WriteFile(archive, good, 0600); err != nil {
		t.Fatal(err)
	}
	if err := MaterializeDate(root, date, zap.NewNop()); err != nil {
		t.Fatalf("retry should complete the date: %v", err)
	}
	assertMaterialized(t, root, date, corrupt)
	archives, _ = ListArchives(root)
	if archives[0].Materialized != len(healthy)+1 {
		t.Errorf("after retry Materialized = %d, want %d", archives[0].Materialized, len(healthy)+1)
	}
}

func TestMaterializeWorkers(t *testing.T) {
	// Unset → GOMAXPROCS.
	t.Setenv("GEXBOT_MATERIALIZE_WORKERS", "")
	os.Unsetenv("GEXBOT_MATERIALIZE_WORKERS")
	if got := materializeWorkers(zap.NewNop()); got < 1 {
		t.Errorf("default workers = %d, want >= 1", got)
	}
	// Valid override.
	t.Setenv("GEXBOT_MATERIALIZE_WORKERS", "3")
	if got := materializeWorkers(zap.NewNop()); got != 3 {
		t.Errorf("override workers = %d, want 3", got)
	}
	// Garbage and non-positive values warn and fall back to the default (>= 1).
	for _, bad := range []string{"foo", "0", "-4"} {
		t.Setenv("GEXBOT_MATERIALIZE_WORKERS", bad)
		core, logs := observer.New(zap.WarnLevel)
		got := materializeWorkers(zap.New(core))
		if got < 1 {
			t.Errorf("%q: workers = %d, want >= 1 fallback", bad, got)
		}
		if logs.FilterMessage("invalid GEXBOT_MATERIALIZE_WORKERS; using default").Len() != 1 {
			t.Errorf("%q: expected one warn about the invalid value", bad)
		}
	}
	// An absurd positive override is clamped to the ceiling (4*GOMAXPROCS) with a warn.
	t.Setenv("GEXBOT_MATERIALIZE_WORKERS", "1000000")
	core, logs := observer.New(zap.WarnLevel)
	ceiling := 4 * runtime.GOMAXPROCS(0)
	if got := materializeWorkers(zap.New(core)); got != ceiling {
		t.Errorf("oversized override = %d, want ceiling %d", got, ceiling)
	}
	if logs.FilterMessage("GEXBOT_MATERIALIZE_WORKERS above ceiling; clamping").Len() != 1 {
		t.Error("expected one warn about clamping the oversized override")
	}
}

// TestMaterializeDateDeterministicErrorOrder: with two corrupt tickers, the joined error
// is in sorted-ticker order regardless of completion order.
func TestMaterializeDateDeterministicErrorOrder(t *testing.T) {
	root := t.TempDir()
	date := "2026-07-17"
	packDate(t, root, date, "AAA", "BBB", "ZZZ") // ZZZ stays healthy
	for _, tk := range []string{"AAA", "BBB"} {
		if err := os.WriteFile(ArchivePath(root, date, tk), []byte("not a zip"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	err := MaterializeDate(root, date, zap.NewNop())
	if err == nil {
		t.Fatal("expected an error from the corrupt tickers")
	}
	msg := err.Error()
	iA, iB := strings.Index(msg, "AAA:"), strings.Index(msg, "BBB:")
	if iA < 0 || iB < 0 || iA > iB {
		t.Errorf("joined error must list AAA before BBB (sorted order): %q", msg)
	}
}

// TestMaterializeDatePeakConcurrency proves the process-wide limiter bounds concurrent
// ticker work at the configured limit across BOTH entry points — MaterializeDate's
// scheduled tickers and direct MaterializeTicker callers — by swapping the shared worker
// for a probe that records peak simultaneous execution.
func TestMaterializeDatePeakConcurrency(t *testing.T) {
	root := t.TempDir()
	date := "2026-07-17"
	// Only the eod/<date>/<ticker> dirs need to exist; the worker is stubbed.
	for _, tk := range []string{"A", "B", "C", "D", "E", "F", "G", "H"} {
		if err := os.MkdirAll(filepath.Join(root, "eod", date, tk), 0750); err != nil {
			t.Fatal(err)
		}
	}

	const limit = 3
	materializeSemMu.Lock()
	materializeSem = make(chan struct{}, limit)
	materializeSemMu.Unlock()
	defer func() {
		materializeSemMu.Lock()
		materializeSem = nil // let later tests re-init from the default
		materializeSemMu.Unlock()
	}()

	orig := materializeTickerFn
	defer func() { materializeTickerFn = orig }()
	var active, peak atomic.Int64
	var pmu sync.Mutex
	materializeTickerFn = func(_, _, _ string, _ *zap.Logger) error {
		cur := active.Add(1)
		pmu.Lock()
		if cur > peak.Load() {
			peak.Store(cur)
		}
		pmu.Unlock()
		time.Sleep(20 * time.Millisecond)
		active.Add(-1)
		return nil
	}

	// Run the date scheduler alongside several direct MaterializeTicker callers — they
	// all share materializeSem, so peak must never exceed the limit.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := MaterializeDate(root, date, zap.NewNop()); err != nil {
			t.Errorf("MaterializeDate: %v", err)
		}
	}()
	for _, tk := range []string{"X", "Y", "Z", "W"} {
		tk := tk
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := MaterializeTicker(root, date, tk, zap.NewNop()); err != nil {
				t.Errorf("MaterializeTicker(%s): %v", tk, err)
			}
		}()
	}
	wg.Wait()

	if p := peak.Load(); p > limit {
		t.Errorf("peak concurrency %d exceeded the limit %d", p, limit)
	} else if p != limit {
		t.Errorf("peak concurrency %d never reached the limit %d", p, limit)
	}
}

func TestMaterializeDateProgressFields(t *testing.T) {
	root := t.TempDir()
	date := "2026-07-17"
	tickers := []string{"AAPL", "NVDA", "SPY"}
	packDate(t, root, date, tickers...)

	core, logs := observer.New(zap.InfoLevel)
	if err := MaterializeDate(root, date, zap.New(core)); err != nil {
		t.Fatal(err)
	}
	if n := logs.FilterMessage("materializing date from EOD archive").Len(); n != 1 {
		t.Errorf("date-start logs = %d, want 1", n)
	}
	if n := logs.FilterMessage("materialized date from EOD archive").Len(); n != 1 {
		t.Errorf("date-success logs = %d, want 1", n)
	}
	progress := logs.FilterMessage("materialize progress").All()
	if len(progress) != len(tickers) {
		t.Fatalf("progress logs = %d, want %d", len(progress), len(tickers))
	}
	// done values must be unique and cover 1..N — regardless of completion order.
	var dones []int
	for _, e := range progress {
		dones = append(dones, int(e.ContextMap()["done"].(int64)))
	}
	sort.Ints(dones)
	for i, d := range dones {
		if d != i+1 {
			t.Errorf("done values = %v, want 1..%d", dones, len(tickers))
			break
		}
	}
}

// TestMaterializeDateConcurrentCallers stresses the process-wide limiter under overlapping
// callers (same date and different dates at once) — run with -race.
func TestMaterializeDateConcurrentCallers(t *testing.T) {
	root := t.TempDir()
	d1, d2 := "2026-07-17", "2026-07-18"
	packDate(t, root, d1, "AAPL", "SPY", "NVDA")
	packDate(t, root, d2, "QQQ", "IWM")

	var wg sync.WaitGroup
	run := func(date string) {
		defer wg.Done()
		if err := MaterializeDate(root, date, zap.NewNop()); err != nil {
			t.Errorf("MaterializeDate(%s): %v", date, err)
		}
	}
	wg.Add(4)
	go run(d1)
	go run(d1) // two callers, same date — promotion must stay race-free
	go run(d2)
	go run(d2)
	wg.Wait()

	assertMaterialized(t, root, d1, "AAPL")
	assertMaterialized(t, root, d1, "NVDA")
	assertMaterialized(t, root, d2, "QQQ")
}

// Materialization eagerly writes an offset-index sidecar next to each JSONL, so a
// later stream/range load reads it instead of re-scanning.
func TestMaterializeBuildsOffsetIndex(t *testing.T) {
	root := t.TempDir()
	materializeFixture(t, root, "2026-07-17", "SPX")

	jsonl := filepath.Join(root, "2026-07-17", "SPX", "classic", "gex_full.jsonl")
	if _, err := os.Stat(offsetindex.SidecarPath(jsonl)); err != nil {
		t.Fatalf("materialize should have written a sidecar: %v", err)
	}
	fi, err := os.Stat(jsonl)
	if err != nil {
		t.Fatal(err)
	}
	offs, ok := offsetindex.Read(jsonl, fi)
	if !ok || len(offs) != 1 || offs[0] != 0 { // fixture writes one line
		t.Fatalf("eager index not valid/consumable: ok=%v offs=%v", ok, offs)
	}
}

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

// TestCleanupStaleKeepsCorruptArchive is the #40 re-review regression: a stale,
// materialized date whose ZIP is non-empty but no longer matches its manifest
// (truncated/byte-corrupted) must NOT be evicted — its JSONL is the only copy
// that can still restore the data.
func TestCleanupStaleKeepsCorruptArchive(t *testing.T) {
	root := t.TempDir()
	date := "2026-07-06"
	materializeFixture(t, root, date, "SPY")
	if err := TouchLoaded(root, date); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-10 * 24 * time.Hour)
	if err := os.Chtimes(filepath.Join(root, date, loadedMarker), old, old); err != nil {
		t.Fatal(err)
	}

	// Corrupt the ZIP: non-empty, so a size-only guard would still evict — but
	// the bytes no longer hash to the manifest's archive_sha256.
	archive := ArchivePath(root, date, "SPY")
	if err := os.WriteFile(archive, []byte("non-empty but not the real zip"), 0600); err != nil {
		t.Fatal(err)
	}

	removed, err := CleanupStale(root, 7*24*time.Hour, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 0 {
		t.Errorf("date with a corrupt archive must not be evicted, got %v", removed)
	}
	jsonl := filepath.Join(root, date, "SPY", "classic", "gex_full.jsonl")
	if _, err := os.Stat(jsonl); err != nil {
		t.Errorf("JSONL must be kept when the archive can't restore it: %v", err)
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
