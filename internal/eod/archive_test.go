package eod

import (
	"os"
	"path/filepath"
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
