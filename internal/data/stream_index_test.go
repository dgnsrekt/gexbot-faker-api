package data

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/offsetindex"
)

// A cold StreamLoader scans and writes a sidecar; a warm one reads it. Both must
// return byte-identical rows, including non-sequential reads (which catch reliance
// on the file handle's initial position).
func TestStreamLoader_ColdAndCachedIdentical(t *testing.T) {
	root := t.TempDir()
	writeDay(t, root, "2026-08-06", "SPX", "state", "gex_full", []int64{100, 200, 300, 400})
	jsonl := filepath.Join(root, "2026-08-06", "SPX", "state", "gex_full.jsonl")

	readNonSeq := func(sl *StreamLoader) []string {
		var out []string
		for _, i := range []int{2, 0, 3, 1} { // deliberately out of order
			raw, err := sl.GetRawAtIndex(context.Background(), "SPX", "state", "gex_full", i)
			if err != nil {
				t.Fatalf("GetRawAtIndex(%d): %v", i, err)
			}
			out = append(out, strings.TrimSpace(string(raw)))
		}
		return out
	}

	sl1, err := NewStreamLoader(root, "2026-08-06", zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(offsetindex.SidecarPath(jsonl)); err != nil {
		t.Fatalf("cold load should have written a sidecar: %v", err)
	}
	cold := readNonSeq(sl1)
	_ = sl1.Close()

	sl2, err := NewStreamLoader(root, "2026-08-06", zap.NewNop()) // sidecar present → cache path
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sl2.Close() }()
	warm := readNonSeq(sl2)

	if l, _ := sl2.GetLength("SPX", "state", "gex_full"); l != 4 {
		t.Errorf("cached length = %d, want 4", l)
	}
	for i := range cold {
		if cold[i] != warm[i] {
			t.Fatalf("cold vs cached mismatch at read %d: %q vs %q", i, cold[i], warm[i])
		}
	}
	if !strings.Contains(cold[0], `"timestamp":300`) { // first read was index 2
		t.Errorf("index 2 = %q, want timestamp 300", cold[0])
	}
}

// A stale sidecar (source changed) must be ignored — the loader rescans and sees the
// new data.
func TestStreamLoader_RescansOnSourceChange(t *testing.T) {
	root := t.TempDir()
	writeDay(t, root, "2026-08-06", "SPX", "state", "gex_full", []int64{100, 200})
	jsonl := filepath.Join(root, "2026-08-06", "SPX", "state", "gex_full.jsonl")

	sl1, err := NewStreamLoader(root, "2026-08-06", zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	_ = sl1.Close() // sidecar now describes 2 rows

	f, err := os.OpenFile(jsonl, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"timestamp":300,"spot":300}` + "\n"); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	sl2, err := NewStreamLoader(root, "2026-08-06", zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sl2.Close() }()
	if l, _ := sl2.GetLength("SPX", "state", "gex_full"); l != 3 {
		t.Fatalf("after append, length = %d, want 3 (stale sidecar must be ignored)", l)
	}
}
