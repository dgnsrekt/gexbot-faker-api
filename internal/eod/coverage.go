package eod

import (
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// TickerSnapshots returns the per-ticker intraday snapshot count for date, read
// from the EOD manifests (cheap — no unpack). Each category has one row per
// snapshot, so a ticker's snapshot count is its max member record count. Tickers
// with no readable manifest are omitted.
func TickerSnapshots(root, date string) (map[string]int, error) {
	dir := filepath.Join(root, "eod", date)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]int{}, nil
		}
		return nil, err
	}
	out := map[string]int{}
	for _, te := range entries {
		if !te.IsDir() {
			continue
		}
		ticker := te.Name()
		data, err := os.ReadFile(ManifestPath(ArchivePath(root, date, ticker)))
		if err != nil {
			continue
		}
		var man Manifest
		if json.Unmarshal(data, &man) != nil {
			continue
		}
		max := 0
		for _, m := range man.Members {
			if m.Records > max {
				max = m.Records
			}
		}
		// Keep manifest-backed tickers even at 0 so an empty member (an extreme
		// truncation) is still inspected by the deviation and session-shape checks
		// rather than vanishing from the map.
		out[ticker] = max
	}
	return out, nil
}

// RepresentativeMember picks a member of date/ticker's archive to read session
// timestamps from, preferring classic/gex_full (the canonical full-session
// series) but falling back to any available member so a package-subset archive
// (state-only, orderflow-only) is still checkable. Returns ok=false only when
// the manifest is unreadable or empty.
func RepresentativeMember(root, date, ticker string) (pkg, category string, ok bool) {
	data, err := os.ReadFile(ManifestPath(ArchivePath(root, date, ticker)))
	if err != nil {
		return "", "", false
	}
	var man Manifest
	if json.Unmarshal(data, &man) != nil || len(man.Members) == 0 {
		return "", "", false
	}
	// Preference order, then first available.
	prefer := [][2]string{{"classic", "gex_full"}, {"classic", "gex_zero"}}
	for _, p := range prefer {
		for _, m := range man.Members {
			if m.Package == p[0] && m.Category == p[1] {
				return m.Package, m.Category, true
			}
		}
	}
	m := man.Members[0]
	return m.Package, m.Category, true
}

// MemberTimestamps streams the "timestamp" of every record in the given archive
// member (e.g. classic/gex_full) for date/ticker, in file order. It decodes one
// element at a time so a large member (SPX gex_full carries hundreds of strikes
// per row) never has to be held in memory at once. Used by session-shape checks.
func MemberTimestamps(root, date, ticker, pkg, category string) ([]int64, error) {
	archive := ArchivePath(root, date, ticker)
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		p, c, err := parseMember(f.Name, date, ticker)
		if err != nil || p != pkg || c != category {
			continue
		}
		r, err := f.Open()
		if err != nil {
			return nil, err
		}
		gz, err := gzip.NewReader(r)
		if err != nil {
			_ = r.Close()
			return nil, err
		}
		ts, decErr := decodeTimestamps(gz)
		_ = gz.Close()
		_ = r.Close()
		if decErr != nil {
			return nil, fmt.Errorf("%s: %w", f.Name, decErr)
		}
		return ts, nil
	}
	return nil, fmt.Errorf("member %s/%s not found in %s", pkg, category, archive)
}

// decodeTimestamps streams a JSON array of records and returns each record's
// "timestamp" (unix seconds), ignoring all other fields.
func decodeTimestamps(r interface{ Read([]byte) (int, error) }) ([]int64, error) {
	dec := json.NewDecoder(r)
	if _, err := dec.Token(); err != nil { // opening '['
		return nil, err
	}
	var out []int64
	for dec.More() {
		var rec struct {
			Timestamp int64 `json:"timestamp"`
		}
		if err := dec.Decode(&rec); err != nil {
			return nil, err
		}
		out = append(out, rec.Timestamp)
	}
	return out, nil
}
