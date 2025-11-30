package data

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"go.uber.org/zap"
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

		// Build index and open file
		offsets, file, err := loader.indexFile(path)
		if err != nil {
			logger.Warn("failed to index file", zap.String("path", path), zap.Error(err))
			return nil
		}

		loader.indexes[key] = offsets
		loader.files[key] = file

		logger.Info("indexed data",
			zap.String("key", key),
			zap.Int("count", len(offsets)),
		)
		return nil
	})

	if err != nil {
		loader.Close()
		return nil, fmt.Errorf("walking data directory: %w", err)
	}

	if len(loader.indexes) == 0 {
		return nil, fmt.Errorf("no JSONL files found in %s", dateDir)
	}

	return loader, nil
}

// indexFile scans the file and records byte offsets for each line.
// Returns the offsets slice and keeps the file open for later reads.
func (s *StreamLoader) indexFile(path string) ([]int64, *os.File, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}

	var offsets []int64
	var offset int64 = 0

	reader := bufio.NewReader(file)
	for {
		// Record start of line
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			// Skip empty lines
			trimmed := line
			if len(trimmed) > 0 && trimmed[len(trimmed)-1] == '\n' {
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
			file.Close()
			return nil, nil, err
		}

		offset += int64(len(line))
	}

	return offsets, file, nil
}

func (s *StreamLoader) GetAtIndex(ctx context.Context, ticker, pkg, category string, index int) (*GexData, error) {
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
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("read error: %w", err)
	}

	// Unmarshal
	var gex GexData
	if err := json.Unmarshal(line, &gex); err != nil {
		return nil, fmt.Errorf("unmarshal error: %w", err)
	}

	return &gex, nil
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
