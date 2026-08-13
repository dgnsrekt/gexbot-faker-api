package eod

import (
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/offsetindex"
)

const markerName = ".eod-materialized"

// loadedMarker records when the server last loaded a date; its mtime drives the
// daemon's TTL cleanup (CleanupStale).
const loadedMarker = ".last-loaded"

var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

type Manifest struct {
	Version       int      `json:"version"`
	Date          string   `json:"date"`
	Ticker        string   `json:"ticker"`
	Source        string   `json:"source"`
	ArchiveSHA256 string   `json:"archive_sha256"`
	CreatedAt     string   `json:"created_at"`
	Members       []Member `json:"members"`
}

type Member struct {
	Name          string `json:"name"`
	Package       string `json:"package"`
	Category      string `json:"category"`
	Records       int    `json:"records"`
	ContentSHA256 string `json:"content_sha256"`
}

func ArchivePath(root, date, ticker string) string {
	return filepath.Join(root, "eod", date, ticker, fmt.Sprintf("eod_report_%s_%s.zip", ticker, date))
}

func ManifestPath(archivePath string) string { return archivePath + ".manifest.json" }

func Pack(root, date, ticker, source string) (*Manifest, error) {
	if err := validateDateTicker(date, ticker); err != nil {
		return nil, err
	}
	input := filepath.Join(root, date, ticker)
	files, err := jsonlFiles(input)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no JSONL files found in %s", input)
	}

	dest := ArchivePath(root, date, ticker)
	if err := os.MkdirAll(filepath.Dir(dest), 0750); err != nil {
		return nil, err
	}
	tmp := dest + ".tmp"
	_ = os.Remove(tmp)
	out, err := os.Create(tmp)
	if err != nil {
		return nil, err
	}
	zw := zip.NewWriter(out)

	for _, path := range files {
		rel, _ := filepath.Rel(input, path)
		pkg := filepath.Dir(rel)
		category := strings.TrimSuffix(filepath.Base(rel), ".jsonl")
		name := filepath.ToSlash(filepath.Join(ticker, pkg, category,
			fmt.Sprintf("%s_%s_%s_%s.json.gz", date, ticker, pkg, category)))
		h := &zip.FileHeader{Name: name, Method: zip.Store}
		h.SetModTime(time.Unix(0, 0))
		member, err := zw.CreateHeader(h)
		if err != nil {
			_ = out.Close()
			_ = os.Remove(tmp)
			return nil, err
		}
		gz := gzip.NewWriter(member)
		if err := jsonlToArray(path, gz); err != nil {
			_ = gz.Close()
			_ = out.Close()
			_ = os.Remove(tmp)
			return nil, err
		}
		if err := gz.Close(); err != nil {
			_ = out.Close()
			_ = os.Remove(tmp)
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return nil, err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return nil, err
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return nil, err
	}
	return Verify(dest, date, ticker, source)
}

func Verify(path, date, ticker, source string) (*Manifest, error) {
	if err := validateDateTicker(date, ticker); err != nil {
		return nil, err
	}
	if source == "" {
		var previous Manifest
		if data, err := os.ReadFile(ManifestPath(path)); err == nil && json.Unmarshal(data, &previous) == nil {
			source = previous.Source
		}
	}
	if source == "" {
		source = "unknown"
	}
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	if len(zr.File) == 0 {
		return nil, fmt.Errorf("empty EOD archive")
	}

	manifest := &Manifest{Version: 1, Date: date, Ticker: ticker, Source: source, CreatedAt: time.Now().UTC().Format(time.RFC3339)}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		pkg, category, err := parseMember(f.Name, date, ticker)
		if err != nil {
			return nil, err
		}
		r, err := f.Open()
		if err != nil {
			return nil, err
		}
		gz, err := gzip.NewReader(r)
		if err != nil {
			_ = r.Close()
			return nil, fmt.Errorf("%s: %w", f.Name, err)
		}
		records, digest, err := scanArray(gz, io.Discard)
		_ = gz.Close()
		_ = r.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.Name, err)
		}
		manifest.Members = append(manifest.Members, Member{
			Name: f.Name, Package: pkg, Category: category, Records: records, ContentSHA256: digest,
		})
	}
	sort.Slice(manifest.Members, func(i, j int) bool { return manifest.Members[i].Name < manifest.Members[j].Name })
	manifest.ArchiveSHA256, err = fileSHA256(path)
	if err != nil {
		return nil, err
	}
	if err := writeJSONAtomic(ManifestPath(path), manifest); err != nil {
		return nil, err
	}
	return manifest, nil
}

func MaterializeTicker(root, date, ticker string, logger *zap.Logger) error {
	if logger == nil {
		logger = zap.NewNop()
	}
	if err := validateDateTicker(date, ticker); err != nil {
		return err
	}
	dest := filepath.Join(root, date, ticker)
	if info, err := os.Stat(dest); err == nil && info.IsDir() {
		return nil
	}
	start := time.Now()
	archive := ArchivePath(root, date, ticker)
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer zr.Close()

	stagingRoot := filepath.Join(root, ".eod-staging", date)
	if err := os.MkdirAll(stagingRoot, 0750); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(stagingRoot, ticker+"-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		pkg, category, err := parseMember(f.Name, date, ticker)
		if err != nil {
			return err
		}
		target := filepath.Join(tmp, pkg, category+".jsonl")
		if err := os.MkdirAll(filepath.Dir(target), 0750); err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		r, err := f.Open()
		if err != nil {
			_ = out.Close()
			return err
		}
		var records int
		gz, err := gzip.NewReader(r)
		if err == nil {
			records, _, err = scanArray(gz, out)
			_ = gz.Close()
		}
		_ = r.Close()
		if closeErr := out.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			return fmt.Errorf("%s: %w", f.Name, err)
		}
		// Eagerly build the offset-index sidecar for the just-written, closed JSONL
		// (in the staging dir, so the rename below promotes it atomically with its
		// JSONL) — a later stream/range load reads it instead of re-scanning. Best
		// effort: temp+rename never promotes a partial, so a failure just means a
		// future load rescans; never fail materialization over a cache.
		if ierr := offsetindex.Build(target); ierr != nil {
			logger.Warn("failed to build offset index at materialize", zap.String("path", target), zap.Error(ierr))
		}
		// Per-member progress — a materialize can take tens of seconds per ticker, so
		// surface each unpacked package/category (with its snapshot count) as it lands.
		logger.Info("unpacked member",
			zap.String("date", date), zap.String("ticker", ticker),
			zap.String("pkg", pkg), zap.String("category", category),
			zap.Int("records", records))
	}
	if err := os.WriteFile(filepath.Join(tmp, markerName), []byte(archive+"\n"), 0600); err != nil {
		return err
	}
	if _, err := os.Stat(dest); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0750); err != nil {
		return err
	}
	if err := os.Rename(tmp, dest); err != nil {
		if info, statErr := os.Stat(dest); statErr == nil && info.IsDir() {
			return nil
		}
		return err
	}
	logger.Info("materialized ticker from EOD archive",
		zap.String("date", date),
		zap.String("ticker", ticker),
		zap.Duration("duration", time.Since(start)),
	)
	return nil
}

func MaterializeDate(root, date string, logger *zap.Logger) error {
	if logger == nil {
		logger = zap.NewNop()
	}
	if !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`).MatchString(date) {
		return fmt.Errorf("invalid date %q", date)
	}
	tickers, err := os.ReadDir(filepath.Join(root, "eod", date))
	if err != nil {
		return err
	}
	count := 0
	for _, ticker := range tickers {
		if ticker.IsDir() {
			count++
		}
	}
	if count == 0 {
		return nil
	}
	logger.Info("materializing date from EOD archive", zap.String("date", date), zap.Int("tickers", count))
	start := time.Now()
	done := 0
	for _, ticker := range tickers {
		if ticker.IsDir() {
			done++
			logger.Info("materializing ticker",
				zap.String("date", date), zap.String("ticker", ticker.Name()),
				zap.Int("progress", done), zap.Int("total", count))
			if err := MaterializeTicker(root, date, ticker.Name(), logger); err != nil {
				return err
			}
		}
	}
	logger.Info("materialized date from EOD archive",
		zap.String("date", date), zap.Int("tickers", count), zap.Duration("duration", time.Since(start)))
	return nil
}

func LatestDate(root string) (string, error) {
	entries, err := os.ReadDir(filepath.Join(root, "eod"))
	if err != nil {
		return "", err
	}
	var dates []string
	for _, entry := range entries {
		if entry.IsDir() && len(entry.Name()) == len("2006-01-02") {
			dates = append(dates, entry.Name())
		}
	}
	if len(dates) == 0 {
		return "", fmt.Errorf("no EOD archives found")
	}
	sort.Strings(dates)
	return dates[len(dates)-1], nil
}

// LatestTickerDate returns the newest date that has a completed EOD archive for
// the given ticker (manifest present), searching newest-first. Unlike LatestDate
// it does not assume the globally-newest date contains every ticker. Returns ""
// if the ticker has no archive on any date.
func LatestTickerDate(root, ticker string) string {
	entries, err := os.ReadDir(filepath.Join(root, "eod"))
	if err != nil {
		return ""
	}
	var dates []string
	for _, entry := range entries {
		if entry.IsDir() && len(entry.Name()) == len("2006-01-02") {
			dates = append(dates, entry.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))
	for _, date := range dates {
		if _, err := os.Stat(ManifestPath(ArchivePath(root, date, ticker))); err == nil {
			return date
		}
	}
	return ""
}

// ArchiveInfo is a per-date summary of the EOD archives on disk, aggregated
// across every ticker for that date. It backs the Studio UI's Data Library table.
type ArchiveInfo struct {
	Date     string   `json:"date"`
	Tickers  []string `json:"tickers"`
	Packages []string `json:"packages"`
	Size     int64    `json:"size_bytes"`
	Records  int      `json:"records"`
	Status   string   `json:"status"` // "ok" | "corrupt"
	// Materialized is how many of this date's archived tickers have their JSONL
	// materialized on disk (the .eod-materialized marker present). When it equals
	// len(Tickers) the date can be loaded instantly; when 0 it is archive-only.
	Materialized int `json:"materialized"`
	// SnapshotsByTicker is each ticker's intraday snapshot count for the date.
	// Each category has one row per snapshot, so a ticker's count is its max
	// member record count (~23,400 ≈ a full 1/sec session). Kept per-ticker rather
	// than averaged so coverage can be judged against each ticker's own history —
	// an unweighted cross-ticker average shifts when the ticker set changes even
	// if every ticker is individually healthy. See CoverageIndices.
	SnapshotsByTicker map[string]int `json:"snapshots_by_ticker,omitempty"`
}

// HasArchive reports whether date has at least one completed EOD archive zip on
// disk. date must be a valid YYYY-MM-DD; anything else (including path-traversal
// attempts) returns false without touching the filesystem.
func HasArchive(root, date string) bool {
	if !dateRe.MatchString(date) {
		return false
	}
	tickerEntries, err := os.ReadDir(filepath.Join(root, "eod", date))
	if err != nil {
		return false
	}
	for _, te := range tickerEntries {
		if !te.IsDir() {
			continue
		}
		if _, err := os.Stat(ArchivePath(root, date, te.Name())); err == nil {
			return true
		}
	}
	return false
}

// ListArchives enumerates the EOD archives under <root>/eod, one ArchiveInfo per
// date (newest first). For each date it unions the tickers and packages, sums the
// zip sizes and manifest record counts, and marks status "corrupt" if any present
// archive is missing or has an unreadable manifest. Dates with no archive zips are
// skipped. A missing eod dir yields an empty slice, not an error.
func ListArchives(root string) ([]ArchiveInfo, error) {
	entries, err := os.ReadDir(filepath.Join(root, "eod"))
	if err != nil {
		if os.IsNotExist(err) {
			return []ArchiveInfo{}, nil
		}
		return nil, err
	}

	var out []ArchiveInfo
	for _, dateEntry := range entries {
		if !dateEntry.IsDir() || !dateRe.MatchString(dateEntry.Name()) {
			continue
		}
		date := dateEntry.Name()
		tickerEntries, err := os.ReadDir(filepath.Join(root, "eod", date))
		if err != nil {
			continue
		}
		info := ArchiveInfo{Date: date, Status: "ok"}
		tickerSet := map[string]bool{}
		pkgSet := map[string]bool{}
		hasArchive := false
		snaps := map[string]int{} // per-ticker snapshot counts
		for _, te := range tickerEntries {
			if !te.IsDir() {
				continue
			}
			ticker := te.Name()
			archive := ArchivePath(root, date, ticker)
			stat, err := os.Stat(archive)
			if err != nil {
				continue // no zip for this ticker
			}
			hasArchive = true
			tickerSet[ticker] = true
			info.Size += stat.Size()
			if _, err := os.Stat(filepath.Join(root, date, ticker, markerName)); err == nil {
				info.Materialized++
			}

			manifestBytes, err := os.ReadFile(ManifestPath(archive))
			if err != nil {
				info.Status = "corrupt"
				continue
			}
			var man Manifest
			if err := json.Unmarshal(manifestBytes, &man); err != nil {
				info.Status = "corrupt"
				continue
			}
			tickerMax := 0
			for _, m := range man.Members {
				info.Records += m.Records
				if m.Records > tickerMax {
					tickerMax = m.Records // all categories share timestamps → max = snapshot count
				}
				if m.Package != "" {
					pkgSet[m.Package] = true
				}
			}
			if tickerMax > 0 {
				snaps[ticker] = tickerMax
			}
		}
		if !hasArchive {
			continue
		}
		if len(snaps) > 0 {
			info.SnapshotsByTicker = snaps
		}
		info.Tickers = sortedKeys(tickerSet)
		info.Packages = sortedKeys(pkgSet)
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date > out[j].Date })
	return out, nil
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// CoverageIndices returns, per archive (aligned to the input slice), a
// composition-stable coverage index: the mean over that date's tickers of each
// ticker's snapshots divided by that ticker's median across all the archives.
// ~1.0 means every ticker is at its own norm; well below 1 flags a real
// per-ticker collection drop. Because each ticker is compared only to itself,
// adding or removing tickers from the set does not move the index (unlike an
// unweighted cross-ticker average). A date with no usable per-ticker data gets 0.
func CoverageIndices(archives []ArchiveInfo) []float64 {
	hist := map[string][]int{}
	for _, a := range archives {
		for tk, n := range a.SnapshotsByTicker {
			hist[tk] = append(hist[tk], n)
		}
	}
	med := make(map[string]int, len(hist))
	for tk, xs := range hist {
		med[tk] = medianInt(xs)
	}
	out := make([]float64, len(archives))
	for i, a := range archives {
		var sum float64
		var n int
		for tk, v := range a.SnapshotsByTicker {
			if m := med[tk]; m > 0 {
				sum += float64(v) / float64(m)
				n++
			}
		}
		if n > 0 {
			out[i] = sum / float64(n)
		}
	}
	return out
}

func medianInt(xs []int) int {
	v := append([]int(nil), xs...)
	sort.Ints(v)
	n := len(v)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return v[n/2]
	}
	return (v[n/2-1] + v[n/2]) / 2
}

// MarkMaterialized records that a ticker's JSONL is already on disk (e.g.
// produced directly by a download's auto-convert) so ListArchives reports it
// materialized without a redundant unpack. It writes the same .eod-materialized
// marker MaterializeTicker does (the source archive path). No-op if the ticker's
// data dir is absent.
func MarkMaterialized(root, date, ticker string) error {
	dir := filepath.Join(root, date, ticker)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return nil
	}
	archive := ArchivePath(root, date, ticker)
	return os.WriteFile(filepath.Join(dir, markerName), []byte(archive+"\n"), 0600)
}

// TouchLoaded records that the server just loaded date, refreshing the mtime of
// <root>/<date>/.last-loaded (read by CleanupStale). No-op if the materialized
// date dir doesn't exist.
func TouchLoaded(root, date string) error {
	if !dateRe.MatchString(date) {
		return nil
	}
	dir := filepath.Join(root, date)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return nil
	}
	return os.WriteFile(filepath.Join(dir, loadedMarker), []byte(time.Now().UTC().Format(time.RFC3339)+"\n"), 0600)
}

// CleanupStale removes materialized date dirs that haven't been loaded within ttl
// (their .last-loaded mtime is older than ttl), reclaiming the JSONL while the
// archive under <root>/eod/<date> stays put so the date re-materializes on demand.
//
// It only touches fully-materialized dates (every ticker carries the
// .eod-materialized marker), so raw/downloaded data is never deleted. Dates in
// keep (e.g. the newest archive) are always retained; the actively-served date is
// retained because the server keeps its .last-loaded mtime fresh. Returns the
// dates removed.
// ArchiveRecoverable reports whether the EOD archive for date/ticker can restore
// the ticker's JSONL: the sidecar manifest parses, the ZIP exists and is
// non-empty, and its SHA-256 matches the archive_sha256 recorded at pack time.
// CleanupStale uses this so materialized JSONL is never evicted unless a valid
// archive can bring it back — a missing, truncated, or byte-corrupted ZIP keeps
// the JSONL, which is then the only surviving copy.
func ArchiveRecoverable(root, date, ticker string) bool {
	archive := ArchivePath(root, date, ticker)
	data, err := os.ReadFile(ManifestPath(archive))
	if err != nil {
		return false
	}
	var man Manifest
	if json.Unmarshal(data, &man) != nil || man.ArchiveSHA256 == "" {
		return false
	}
	if fi, err := os.Stat(archive); err != nil || fi.Size() == 0 {
		return false
	}
	sum, err := fileSHA256(archive)
	return err == nil && sum == man.ArchiveSHA256
}

func CleanupStale(root string, ttl time.Duration, logger *zap.Logger, keep ...string) ([]string, error) {
	if logger == nil {
		logger = zap.NewNop()
	}
	protected := make(map[string]bool, len(keep))
	for _, date := range keep {
		protected[date] = true
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	cutoff := time.Now().Add(-ttl)
	var removed []string
	for _, entry := range entries {
		date := entry.Name()
		if !entry.IsDir() || protected[date] || len(date) != len("2006-01-02") {
			continue
		}
		dateDir := filepath.Join(root, date)
		tickers, err := os.ReadDir(dateDir)
		if err != nil || len(tickers) == 0 {
			continue
		}
		// Marker gate: only evict dates that were fully materialized from archives.
		generated := true
		for _, ticker := range tickers {
			if !ticker.IsDir() {
				continue
			}
			if _, err := os.Stat(filepath.Join(dateDir, ticker.Name(), markerName)); err != nil {
				generated = false
				break
			}
		}
		if !generated {
			continue
		}
		// TTL gate: keep if loaded within ttl. Prefer the .last-loaded mtime;
		// fall back to the date dir's own mtime when the marker is absent.
		var mtime time.Time
		if info, err := os.Stat(filepath.Join(dateDir, loadedMarker)); err == nil {
			mtime = info.ModTime()
		} else if info, err := os.Stat(dateDir); err == nil {
			mtime = info.ModTime()
		} else {
			continue
		}
		if mtime.After(cutoff) {
			continue // loaded recently — keep
		}
		// Recoverability gate: never evict JSONL unless every ticker has a
		// restorable archive (exists + SHA matches its manifest). A materialized
		// date whose ZIP was lost or byte-corrupted must keep its JSONL — it is
		// the only remaining copy. Runs only for otherwise-evictable dates, so the
		// per-archive SHA cost stays off the hot path.
		recoverable := true
		for _, ticker := range tickers {
			if !ticker.IsDir() {
				continue
			}
			if !ArchiveRecoverable(root, date, ticker.Name()) {
				logger.Warn("keeping materialized date: archive not recoverable",
					zap.String("date", date), zap.String("ticker", ticker.Name()))
				recoverable = false
				break
			}
		}
		if !recoverable {
			continue
		}
		if err := os.RemoveAll(dateDir); err != nil {
			return removed, err
		}
		logger.Info("evicted stale materialized date",
			zap.String("date", date), zap.String("idle", time.Since(mtime).Round(time.Hour).String()))
		removed = append(removed, date)
	}
	if len(removed) > 0 {
		logger.Info("materialize cleanup complete", zap.Int("evicted", len(removed)), zap.Strings("dates", removed))
	}
	return removed, nil
}

func PruneTicker(root, date, ticker string) error {
	if err := validateDateTicker(date, ticker); err != nil {
		return err
	}
	archive := ArchivePath(root, date, ticker)
	if _, err := os.Stat(ManifestPath(archive)); err != nil {
		return fmt.Errorf("verified manifest missing: %w", err)
	}
	if _, err := Verify(archive, date, ticker, "legacy-jsonl"); err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Join(root, date, ticker)); err != nil {
		return err
	}
	// Drop the parent date dir once the last ticker is pruned. os.Remove only
	// succeeds on an empty dir, so a date still holding other tickers is left
	// intact. Without this, the empty leftover makes GetAvailable report the
	// date as having no data instead of re-materializing it from the archive.
	_ = os.Remove(filepath.Join(root, date))
	return nil
}

func validateDateTicker(date, ticker string) error {
	if !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`).MatchString(date) {
		return fmt.Errorf("invalid date %q", date)
	}
	if ticker == "" || ticker != filepath.Base(ticker) || strings.ContainsAny(ticker, `/\`) {
		return fmt.Errorf("invalid ticker %q", ticker)
	}
	return nil
}

func parseMember(name, date, ticker string) (string, string, error) {
	clean := filepath.ToSlash(filepath.Clean(name))
	parts := strings.Split(clean, "/")
	if len(parts) != 4 || parts[0] != ticker || parts[1] == "" || parts[2] == "" || strings.Contains(clean, "..") {
		return "", "", fmt.Errorf("invalid EOD member path %q", name)
	}
	expected := fmt.Sprintf("%s_%s_%s_%s.json.gz", date, ticker, parts[1], parts[2])
	if parts[3] != expected {
		return "", "", fmt.Errorf("member %q does not match report date/ticker", name)
	}
	return parts[1], parts[2], nil
}

func jsonlFiles(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".jsonl") {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

func jsonlToArray(path string, out io.Writer) error {
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer in.Close()
	reader := bufio.NewReader(in)
	if _, err := io.WriteString(out, "["); err != nil {
		return err
	}
	first := true
	for {
		line, readErr := reader.ReadBytes('\n')
		line = bytes.TrimSpace(line)
		if len(line) > 0 {
			if !json.Valid(line) {
				return fmt.Errorf("%s contains invalid JSON", path)
			}
			if !first {
				if _, err := io.WriteString(out, ","); err != nil {
					return err
				}
			}
			if _, err := out.Write(line); err != nil {
				return err
			}
			first = false
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	_, err = io.WriteString(out, "]")
	return err
}

func scanArray(in io.Reader, out io.Writer) (int, string, error) {
	dec := json.NewDecoder(in)
	token, err := dec.Token()
	if err != nil || token != json.Delim('[') {
		return 0, "", fmt.Errorf("expected JSON array")
	}
	hash := sha256.New()
	count := 0
	for dec.More() {
		var item json.RawMessage
		if err := dec.Decode(&item); err != nil {
			return 0, "", err
		}
		item = bytes.TrimSpace(item)
		if _, err := out.Write(item); err != nil {
			return 0, "", err
		}
		if _, err := out.Write([]byte("\n")); err != nil {
			return 0, "", err
		}
		_, _ = hash.Write(item)
		_, _ = hash.Write([]byte("\n"))
		count++
	}
	if _, err := dec.Token(); err != nil {
		return 0, "", err
	}
	return count, hex.EncodeToString(hash.Sum(nil)), nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func writeJSONAtomic(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path+".tmp", append(data, '\n'), 0600); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}
