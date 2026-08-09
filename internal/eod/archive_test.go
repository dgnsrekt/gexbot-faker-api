package eod

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

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
