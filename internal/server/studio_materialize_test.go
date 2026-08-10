package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/eod"
)

func TestStudioLibraryState(t *testing.T) {
	cases := []struct {
		name                      string
		loaded, running           bool
		materialized, total, want any
	}{
		{"loaded wins", true, false, 0, 3, "loaded"},
		{"loaded even while a job runs", true, true, 1, 3, "loaded"},
		{"running -> materializing", false, true, 1, 3, "materializing"},
		{"all materialized -> ready", false, false, 3, 3, "ready"},
		{"none -> archived", false, false, 0, 3, "archived"},
		{"partial, no job -> archived", false, false, 2, 3, "archived"},
		{"no tickers -> archived", false, false, 0, 0, "archived"},
	}
	for _, tc := range cases {
		got := studioLibraryState(tc.loaded, tc.running, tc.materialized.(int), tc.total.(int))
		if got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

// packFixture writes one classic/gex_full record for date/ticker and packs it
// into an EOD archive so MaterializeDate has something to unpack.
func packServerFixture(t *testing.T, root, date, ticker string) {
	t.Helper()
	src := filepath.Join(root, date, ticker, "classic", "gex_full.jsonl")
	if err := os.MkdirAll(filepath.Dir(src), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("{\"timestamp\":1}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := eod.Pack(root, date, ticker, "test"); err != nil {
		t.Fatal(err)
	}
	// Remove the source JSONL so materialization actually has work to do.
	if err := os.RemoveAll(filepath.Join(root, date)); err != nil {
		t.Fatal(err)
	}
}

func TestMaterializeManager(t *testing.T) {
	root := t.TempDir()
	packServerFixture(t, root, "2026-08-07", "SPX")

	m := newMaterializeManager(root, zap.NewNop())
	job := m.start("2026-08-07")
	if job.State != "queued" && job.State != "running" {
		t.Fatalf("start returned state %q, want queued/running", job.State)
	}

	// start again while in flight → same job, no duplicate goroutine.
	if again := m.start("2026-08-07"); again.State != "queued" && again.State != "running" {
		t.Errorf("re-start returned %q, want queued/running", again.State)
	}

	// Wait (bounded) for completion.
	deadline := time.Now().Add(5 * time.Second)
	for m.inProgress("2026-08-07") {
		if time.Now().After(deadline) {
			t.Fatal("materialize did not finish within 5s")
		}
		time.Sleep(20 * time.Millisecond)
	}
	st, ok := m.status("2026-08-07")
	if !ok || st.State != "done" {
		t.Fatalf("final status = %+v (ok=%v), want done", st, ok)
	}
	// The materialized marker must now exist.
	if _, err := os.Stat(filepath.Join(root, "2026-08-07", "SPX", ".eod-materialized")); err != nil {
		t.Errorf("expected .eod-materialized marker after materialize: %v", err)
	}
}
