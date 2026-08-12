package data

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/offsetindex"
)

// StreamLoader reads JSONL files on-demand using byte offset indexing.
// It keeps file handles open for efficient access.
type StreamLoader struct {
	indexes map[string][]int64  // key -> line byte offsets
	files   map[string]*os.File // key -> open file handle
	mu      sync.RWMutex        // protects file seeks/reads
	logger  *zap.Logger
}

// Compile-time interface verification
var _ DataLoader = (*StreamLoader)(nil)

func NewStreamLoader(dataDir, date string, logger *zap.Logger) (*StreamLoader, error) {
	loader := &StreamLoader{
		indexes: make(map[string][]int64),
		files:   make(map[string]*os.File),
		logger:  logger,
	}

	dateDir := filepath.Join(dataDir, date)

	// Walk the date directory
	err := filepath.Walk(dateDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || filepath.Ext(path) != ".jsonl" {
			return nil
		}

		// Extract ticker/pkg/category from path
		// Format: data/{date}/{ticker}/{pkg}/{category}.jsonl
		rel, _ := filepath.Rel(dateDir, path)
		// rel = "SPX/state/gex_full.jsonl"

		ticker := filepath.Dir(filepath.Dir(rel))
		pkg := filepath.Base(filepath.Dir(rel))
		category := filepath.Base(rel)
		category = category[:len(category)-6] // Remove .jsonl

		key := DataKey(ticker, pkg, category)

		// Build index (from the persisted sidecar when valid) and open file.
		offsets, file, source, err := loader.indexFile(path)
		if err != nil {
			logger.Warn("failed to index file", zap.String("path", path), zap.Error(err))
			return nil
		}

		loader.indexes[key] = offsets
		loader.files[key] = file

		logger.Info("indexed data",
			zap.String("key", key),
			zap.Int("count", len(offsets)),
			zap.String("source", source), // "cache" (read .idx) | "scan" (full scan)
		)
		return nil
	})

	if err != nil {
		_ = loader.Close()
		return nil, fmt.Errorf("walking data directory: %w", err)
	}

	if len(loader.indexes) == 0 {
		return nil, fmt.Errorf("no JSONL files found in %s", dateDir)
	}

	return loader, nil
}

// indexFile returns the line byte offsets for path, keeping the file open for later
// seeks. It reads the persisted ".idx" sidecar when it's valid (fast path); otherwise
// it scans the file once and best-effort persists a sidecar for next time. The offset
// semantics live in offsetindex.Scan so the sidecar always matches a fresh scan.
// Returns source "cache" or "scan" for logging.
func (s *StreamLoader) indexFile(path string) ([]int64, *os.File, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, "", err
	}
	// Stat the RETAINED handle (not os.Stat(path)) so validation refers to the exact
	// inode we'll read from, closing ordinary append/truncate/replace races.
	fi, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, nil, "", err
	}

	if offs, ok := offsetindex.Read(path, fi); ok {
		return offs, file, "cache", nil
	}

	offs, err := offsetindex.Scan(file)
	if err != nil {
		_ = file.Close()
		return nil, nil, "", err
	}
	// Persist for next time, but only if the source hasn't changed under us during the
	// scan (best-effort — a failure just means a future load rescans).
	if fi2, statErr := file.Stat(); statErr == nil &&
		fi2.Size() == fi.Size() && fi2.ModTime().UnixNano() == fi.ModTime().UnixNano() {
		if werr := offsetindex.WriteAtomic(path, offs, fi2); werr != nil {
			s.logger.Warn("failed to persist offset index", zap.String("path", path), zap.Error(werr))
		}
	}
	return offs, file, "scan", nil
}

func (s *StreamLoader) GetAtIndex(ctx context.Context, ticker, pkg, category string, index int) (*GexData, error) {
	rawData, err := s.GetRawAtIndex(ctx, ticker, pkg, category, index)
	if err != nil {
		return nil, err
	}

	var gex GexData
	if err := json.Unmarshal(rawData, &gex); err != nil {
		return nil, fmt.Errorf("unmarshal error: %w", err)
	}

	return &gex, nil
}

func (s *StreamLoader) GetRawAtIndex(ctx context.Context, ticker, pkg, category string, index int) ([]byte, error) {
	key := DataKey(ticker, pkg, category)

	s.mu.RLock()
	offsets, ok := s.indexes[key]
	file := s.files[key]
	s.mu.RUnlock()

	if !ok {
		return nil, ErrNotFound
	}
	if index < 0 || index >= len(offsets) {
		return nil, ErrIndexOutOfBounds
	}

	// Lock for seek+read operation
	s.mu.Lock()
	defer s.mu.Unlock()

	// Seek to line offset
	_, err := file.Seek(offsets[index], io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("seek error: %w", err)
	}

	// Read the line
	reader := bufio.NewReader(file)
	line, err := reader.ReadBytes('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("read error: %w", err)
	}

	return line, nil
}

func (s *StreamLoader) GetLength(ticker, pkg, category string) (int, error) {
	key := DataKey(ticker, pkg, category)

	s.mu.RLock()
	offsets, ok := s.indexes[key]
	s.mu.RUnlock()

	if !ok {
		return 0, ErrNotFound
	}
	return len(offsets), nil
}

func (s *StreamLoader) Exists(ticker, pkg, category string) bool {
	key := DataKey(ticker, pkg, category)

	s.mu.RLock()
	_, ok := s.indexes[key]
	s.mu.RUnlock()

	return ok
}

func (s *StreamLoader) GetLoadedKeys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]string, 0, len(s.indexes))
	for k := range s.indexes {
		keys = append(keys, k)
	}
	return keys
}

func (s *StreamLoader) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key, file := range s.files {
		if err := file.Close(); err != nil {
			s.logger.Warn("failed to close file", zap.String("key", key), zap.Error(err))
		}
	}

	s.indexes = nil
	s.files = nil
	return nil
}

// FindIndexByTimestamp returns the first index where timestamp >= target.
// Uses binary search for O(log n) performance, reading from disk at each probe.
func (s *StreamLoader) FindIndexByTimestamp(ctx context.Context, ticker, pkg, category string, target int64) (int, int64, error) {
	key := DataKey(ticker, pkg, category)

	s.mu.RLock()
	offsets, ok := s.indexes[key]
	s.mu.RUnlock()

	if !ok {
		return 0, 0, ErrNotFound
	}

	if len(offsets) == 0 {
		return 0, 0, ErrNotFound
	}

	// Binary search for first index where timestamp >= target
	idx := sort.Search(len(offsets), func(i int) bool {
		raw, err := s.GetRawAtIndex(ctx, ticker, pkg, category, i)
		if err != nil {
			return false
		}
		ts := extractTimestampFromRaw(raw)
		return ts >= target
	})

	// If idx == len(offsets), target is after all data
	if idx >= len(offsets) {
		return 0, 0, ErrTimestampOutOfRange
	}

	// Get actual timestamp at found index
	raw, err := s.GetRawAtIndex(ctx, ticker, pkg, category, idx)
	if err != nil {
		return 0, 0, err
	}
	actualTs := extractTimestampFromRaw(raw)

	return idx, actualTs, nil
}

// extractTimestampFromRaw quickly extracts timestamp from JSON without full unmarshal
func extractTimestampFromRaw(raw []byte) int64 {
	var partial struct {
		Timestamp int64 `json:"timestamp"`
	}
	_ = json.Unmarshal(raw, &partial)
	return partial.Timestamp
}
