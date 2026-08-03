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
)

const markerName = ".eod-materialized"

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

func MaterializeTicker(root, date, ticker string) error {
	if err := validateDateTicker(date, ticker); err != nil {
		return err
	}
	dest := filepath.Join(root, date, ticker)
	if info, err := os.Stat(dest); err == nil && info.IsDir() {
		return nil
	}
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
		gz, err := gzip.NewReader(r)
		if err == nil {
			_, _, err = scanArray(gz, out)
			_ = gz.Close()
		}
		_ = r.Close()
		if closeErr := out.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			return fmt.Errorf("%s: %w", f.Name, err)
		}
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
	return nil
}

func MaterializeDate(root, date string) error {
	if !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`).MatchString(date) {
		return fmt.Errorf("invalid date %q", date)
	}
	tickers, err := os.ReadDir(filepath.Join(root, "eod", date))
	if err != nil {
		return err
	}
	for _, ticker := range tickers {
		if ticker.IsDir() {
			if err := MaterializeTicker(root, date, ticker.Name()); err != nil {
				return err
			}
		}
	}
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

func CleanupMaterialized(root string, keep ...string) error {
	protected := make(map[string]bool, len(keep))
	for _, date := range keep {
		protected[date] = true
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		date := entry.Name()
		if !entry.IsDir() || protected[date] || len(date) != len("2006-01-02") {
			continue
		}
		tickers, err := os.ReadDir(filepath.Join(root, date))
		if err != nil || len(tickers) == 0 {
			continue
		}
		generated := true
		for _, ticker := range tickers {
			if !ticker.IsDir() {
				continue
			}
			if _, err := os.Stat(filepath.Join(root, date, ticker.Name(), markerName)); err != nil {
				generated = false
				break
			}
		}
		if generated {
			if err := os.RemoveAll(filepath.Join(root, date)); err != nil {
				return err
			}
		}
	}
	return nil
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
	// intact. Without this, the empty leftover makes GetAvailableData report the
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
