package eod

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
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
	if err := MaterializeTicker(root, date, ticker); err != nil {
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
			errs <- MaterializeTicker(root, date, ticker)
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
