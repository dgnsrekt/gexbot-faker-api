package eod

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// packTimestamps writes a classic/gex_full.jsonl for date/ticker with one record
// per given timestamp, then packs it into an EOD archive.
func packTimestamps(t *testing.T, root, date, ticker string, ts []int64) {
	t.Helper()
	p := filepath.Join(root, date, ticker, "classic", "gex_full.jsonl")
	if err := os.MkdirAll(filepath.Dir(p), 0750); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, v := range ts {
		b.WriteString(`{"timestamp":` + strconv.FormatInt(v, 10) + `,"spot":1}` + "\n")
	}
	if err := os.WriteFile(p, []byte(b.String()), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Pack(root, date, ticker, "test"); err != nil {
		t.Fatal(err)
	}
}

func TestTickerSnapshots(t *testing.T) {
	root := t.TempDir()
	date := "2026-08-07"
	packTimestamps(t, root, date, "SPX", []int64{1, 2, 3})
	packTimestamps(t, root, date, "NDX", []int64{1, 2, 3, 4, 5})

	got, err := TickerSnapshots(root, date)
	if err != nil {
		t.Fatal(err)
	}
	if got["SPX"] != 3 || got["NDX"] != 5 {
		t.Errorf("TickerSnapshots = %v, want SPX:3 NDX:5", got)
	}

	// A date with no archives is empty, not an error.
	empty, err := TickerSnapshots(root, "1999-01-01")
	if err != nil || len(empty) != 0 {
		t.Errorf("missing date: got %v err %v, want empty", empty, err)
	}
}

func TestTickerSnapshotsIncludesEmptyMember(t *testing.T) {
	root := t.TempDir()
	date := "2026-08-07"
	packTimestamps(t, root, date, "SPX", nil) // empty member → 0 records

	got, err := TickerSnapshots(root, date)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := got["SPX"]; !ok || v != 0 {
		t.Errorf("an empty-member ticker must be present at 0, got %d (present=%v)", v, ok)
	}
}

func TestMemberTimestamps(t *testing.T) {
	root := t.TempDir()
	date := "2026-08-07"
	want := []int64{1785000000, 1785000001, 1785000005}
	packTimestamps(t, root, date, "SPX", want)

	got, err := MemberTimestamps(root, date, "SPX", "classic", "gex_full")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d timestamps, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ts[%d] = %d, want %d", i, got[i], want[i])
		}
	}

	// A member that doesn't exist is an error, not a silent empty.
	if _, err := MemberTimestamps(root, date, "SPX", "orderflow", "orderflow"); err == nil {
		t.Error("expected error for a missing member")
	}
}
