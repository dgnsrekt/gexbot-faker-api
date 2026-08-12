// Package offsetindex persists the byte offsets of each JSONL snapshot line to a
// small ".idx" sidecar next to the ".jsonl", so the stream loader can skip the
// expensive full-file scan on re-load (multi-day range loads went from ~40s to
// sub-second). It is a neutral package shared by the loader (internal/data), the
// materializer (internal/eod), and the download converter (internal/downloadjob).
//
// Correctness contract: the sidecar is a CACHE keyed on the source file's
// size + mtime, valid only because materialized EOD JSONL is IMMUTABLE. It is not
// content proof — an in-place same-size+same-mtime rewrite is undetectable. Any
// mismatch (or corruption) falls back to a safe rescan. The version field lets a
// future format add a content fingerprint without breaking old sidecars.
package offsetindex

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	suffix     = ".idx"
	magic      = "GXI1"
	version    = uint32(1)
	headerSize = 32 // magic(4) + version(4) + srcSize(8) + srcModNano(8) + n(8)
)

// SidecarPath returns the index path for a jsonl path.
func SidecarPath(jsonlPath string) string { return jsonlPath + suffix }

// Scan records the byte offset of each non-empty JSONL line, byte-for-byte
// identical to StreamLoader's original indexFile loop. It intentionally strips only
// a single trailing '\n' (so "\r\n" counts as "\r", whitespace-only counts), skips
// '\n'-only lines, counts a final unterminated non-empty line, and advances by the
// original line length including the newline. Do NOT "clean up" this behavior — the
// loader seeks to exactly these offsets.
func Scan(r io.Reader) ([]int64, error) {
	var offsets []int64
	var offset int64
	reader := bufio.NewReader(r)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			trimmed := line
			if trimmed[len(trimmed)-1] == '\n' {
				trimmed = trimmed[:len(trimmed)-1]
			}
			if len(trimmed) > 0 {
				offsets = append(offsets, offset)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		offset += int64(len(line))
	}
	return offsets, nil
}

// Read returns the cached offsets for jsonlPath, or ok=false when the sidecar is
// missing, stale (size/mtime differ from fi), or fails strict validation. It never
// panics or makes an allocation sized from untrusted header bytes.
func Read(jsonlPath string, fi os.FileInfo) ([]int64, bool) {
	data, err := os.ReadFile(SidecarPath(jsonlPath))
	if err != nil || len(data) < headerSize {
		return nil, false
	}
	if string(data[0:4]) != magic || binary.LittleEndian.Uint32(data[4:8]) != version {
		return nil, false
	}
	srcSize := int64(binary.LittleEndian.Uint64(data[8:16]))
	srcModNano := int64(binary.LittleEndian.Uint64(data[16:24]))
	hdrN := int64(binary.LittleEndian.Uint64(data[24:32]))

	// Staleness: the source must be exactly what we indexed.
	if srcSize != fi.Size() || srcModNano != fi.ModTime().UnixNano() {
		return nil, false
	}

	// Derive the count from the real file length (no n*8 overflow risk) and require
	// the header's n to match exactly. Also cap n by srcSize (can't have more
	// non-empty lines than bytes) and reject trailing garbage.
	body := len(data) - headerSize
	if body < 0 || body%8 != 0 {
		return nil, false
	}
	n := int64(body / 8)
	if n != hdrN || n < 0 || n > srcSize {
		return nil, false
	}

	offsets := make([]int64, n)
	prev := int64(-1)
	for i := int64(0); i < n; i++ {
		o := int64(binary.LittleEndian.Uint64(data[headerSize+i*8:]))
		if o < 0 || o >= srcSize || o <= prev { // in-range, strictly increasing
			return nil, false
		}
		offsets[i] = o
		prev = o
	}
	return offsets, true
}

// WriteAtomic encodes offsets for jsonlPath (stamped with fi's size+mtime) to the
// sidecar via a unique temp file + rename, so concurrent writers and readers never
// see a partial file. Best-effort at the call site: a failure just means a later
// load rescans.
func WriteAtomic(jsonlPath string, offsets []int64, fi os.FileInfo) error {
	sidecar := SidecarPath(jsonlPath)
	tmp, err := os.CreateTemp(filepath.Dir(sidecar), "."+filepath.Base(sidecar)+"-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	renamed := false
	defer func() {
		if !renamed {
			_ = os.Remove(tmpName)
		}
	}()

	buf := make([]byte, headerSize+len(offsets)*8)
	copy(buf[0:4], magic)
	binary.LittleEndian.PutUint32(buf[4:8], version)
	binary.LittleEndian.PutUint64(buf[8:16], uint64(fi.Size()))
	binary.LittleEndian.PutUint64(buf[16:24], uint64(fi.ModTime().UnixNano()))
	binary.LittleEndian.PutUint64(buf[24:32], uint64(len(offsets)))
	for i, o := range offsets {
		binary.LittleEndian.PutUint64(buf[headerSize+i*8:], uint64(o))
	}

	if _, err := tmp.Write(buf); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, sidecar); err != nil {
		return err
	}
	renamed = true
	return nil
}

// Build scans jsonlPath and writes its sidecar — the eager convenience for callers
// that just created the JSONL (materialize/convert). It re-stats after scanning and
// refuses to persist if the source changed under it. No partial sidecar is ever
// promoted (WriteAtomic uses temp+rename).
func Build(jsonlPath string) error {
	f, err := os.Open(jsonlPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	offsets, err := Scan(f)
	if err != nil {
		return err
	}
	fi2, err := f.Stat()
	if err != nil {
		return err
	}
	if fi2.Size() != fi.Size() || fi2.ModTime().UnixNano() != fi.ModTime().UnixNano() {
		return fmt.Errorf("offsetindex: source changed during scan: %s", jsonlPath)
	}
	return WriteAtomic(jsonlPath, offsets, fi2)
}
