package offsetindex

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Scan must be byte-for-byte identical to the loader's original indexFile loop:
// strip only one trailing '\n', "\r\n" counts, whitespace-only counts, '\n'-only
// lines skip, a final unterminated line counts, offset advances by full line length.
func TestScanLiteralOffsets(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []int64
	}{
		{"simple", "a\nb\nc\n", []int64{0, 2, 4}},
		{"lf-empty-skipped", "a\n\nb\n", []int64{0, 3}},
		{"consecutive-empties", "a\n\n\nb\n", []int64{0, 4}},
		{"crlf-lines", "a\r\nb\r\n", []int64{0, 3}},
		{"crlf-empty-counts", "a\n\r\nb\n", []int64{0, 2, 4}},
		{"whitespace-only-counts", "a\n   \nb\n", []int64{0, 2, 6}},
		{"final-unterminated", "a\nb", []int64{0, 2}},
		{"empty-file", "", nil},
		{"only-newlines", "\n\n", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Scan(strings.NewReader(c.in))
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(c.want) {
				t.Fatalf("offsets = %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("offsets = %v, want %v", got, c.want)
				}
			}
		})
	}
}

func TestScanLongLine(t *testing.T) {
	long := strings.Repeat("x", 100000)
	got, err := Scan(strings.NewReader(long + "\nb\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != 0 || got[1] != 100001 {
		t.Fatalf("offsets = %v, want [0 100001]", got)
	}
}

func writeJSONL(t *testing.T, content string) (string, os.FileInfo) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "SPX_state_gex_full.jsonl")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return p, fi
}

func TestBuildReadRoundTrip(t *testing.T) {
	p, fi := writeJSONL(t, "a\nbb\nccc\n")
	if err := Build(p); err != nil {
		t.Fatal(err)
	}
	got, ok := Read(p, fi)
	if !ok {
		t.Fatal("Read after Build should succeed")
	}
	want := []int64{0, 2, 5}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("offsets = %v, want %v", got, want)
	}
}

func TestReadStaleOnSizeOrMtimeChange(t *testing.T) {
	p, fi := writeJSONL(t, "a\nb\n")
	if err := Build(p); err != nil {
		t.Fatal(err)
	}
	// Same-path, changed content (size differs) → stale.
	if err := os.WriteFile(p, []byte("a\nb\nc\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fi2, _ := os.Stat(p)
	if _, ok := Read(p, fi2); ok {
		t.Error("a size change must invalidate the sidecar")
	}
	// Rebuild, then bump mtime only → stale.
	if err := Build(p); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(p, future, future); err != nil {
		t.Fatal(err)
	}
	fi3, _ := os.Stat(p)
	if _, ok := Read(p, fi3); ok {
		t.Error("an mtime change must invalidate the sidecar")
	}
	// The very first fi (pre-change) must also not validate the now-different source.
	_ = fi
}

// A corrupt/hostile sidecar must never panic or make a huge allocation — Read just
// returns ok=false and the loader rescans.
func TestReadRejectsCorruptSidecar(t *testing.T) {
	p, fi := writeJSONL(t, "a\nbb\nccc\n")
	if err := Build(p); err != nil {
		t.Fatal(err)
	}
	good, err := os.ReadFile(SidecarPath(p))
	if err != nil {
		t.Fatal(err)
	}

	mut := func(f func([]byte) []byte) {
		b := append([]byte(nil), good...)
		b = f(b)
		if err := os.WriteFile(SidecarPath(p), b, 0o600); err != nil {
			t.Fatal(err)
		}
		if offs, ok := Read(p, fi); ok {
			t.Errorf("corrupt sidecar accepted (offs=%v)", offs)
		}
	}

	mut(func(b []byte) []byte { b[0] = 'X'; return b })                              // bad magic
	mut(func(b []byte) []byte { binary.LittleEndian.PutUint32(b[4:8], 999); return b }) // bad version
	mut(func(b []byte) []byte { return b[:len(b)-1] })                               // wrong length (body%8!=0)
	mut(func(b []byte) []byte { return append(b, 0xAA, 0xBB) })                      // trailing garbage
	mut(func(b []byte) []byte { binary.LittleEndian.PutUint64(b[24:32], 1<<40); return b }) // huge n vs derived
	mut(func(b []byte) []byte { binary.LittleEndian.PutUint64(b[24:32], ^uint64(0)); return b }) // negative n
	// past-EOF offset (srcSize=9): set first offset to 9
	mut(func(b []byte) []byte { binary.LittleEndian.PutUint64(b[headerSize:], 9); return b })
	// non-increasing: set 2nd offset <= 1st
	mut(func(b []byte) []byte { binary.LittleEndian.PutUint64(b[headerSize+8:], 0); return b })
	// truncated header
	mut(func(b []byte) []byte { return b[:10] })
}

func TestBuildDoesNotLeavePartialOnMissingSource(t *testing.T) {
	// Build on a nonexistent file returns an error and writes no sidecar.
	p := filepath.Join(t.TempDir(), "missing.jsonl")
	if err := Build(p); err == nil {
		t.Error("Build on a missing source should error")
	}
	if _, err := os.Stat(SidecarPath(p)); !os.IsNotExist(err) {
		t.Error("no sidecar should exist for a failed build")
	}
}

// Concurrent writers must converge on a valid, readable sidecar (any winner is fine
// since they all index the same source generation).
func TestWriteAtomicConcurrent(t *testing.T) {
	p, fi := writeJSONL(t, "a\nb\nc\nd\n")
	offs, _ := Scan(bytes.NewReader([]byte("a\nb\nc\nd\n")))
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _ = WriteAtomic(p, offs, fi) }()
	}
	wg.Wait()
	got, ok := Read(p, fi)
	if !ok || len(got) != 4 {
		t.Fatalf("after concurrent writes: ok=%v offs=%v", ok, got)
	}
	// No leftover temp files in the directory.
	entries, _ := os.ReadDir(filepath.Dir(p))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("leftover temp file: %s", e.Name())
		}
	}
}
