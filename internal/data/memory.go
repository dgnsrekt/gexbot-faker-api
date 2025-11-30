package data

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
)

type MemoryLoader struct {
	data   map[string][][]byte // key: ticker/pkg/category, stores raw JSON lines
	logger *zap.Logger
}

func NewMemoryLoader(dataDir, date string, logger *zap.Logger) (*MemoryLoader, error) {
	loader := &MemoryLoader{
		data:   make(map[string][][]byte),
		logger: logger,
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

		data, err := loader.loadJSONL(path)
		if err != nil {
			logger.Warn("failed to load file", zap.String("path", path), zap.Error(err))
			return nil
		}

		loader.data[key] = data
		logger.Info("loaded data",
			zap.String("key", key),
			zap.Int("count", len(data)),
		)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walking data directory: %w", err)
	}

	if len(loader.data) == 0 {
		return nil, fmt.Errorf("no JSONL files found in %s", dateDir)
	}

	return loader, nil
}

func (m *MemoryLoader) loadJSONL(path string) ([][]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var data [][]byte
	scanner := bufio.NewScanner(file)

	// Increase buffer size for large lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Make a copy since scanner reuses the buffer
		lineCopy := make([]byte, len(line))
		copy(lineCopy, line)
		data = append(data, lineCopy)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return data, nil
}

func (m *MemoryLoader) GetAtIndex(ctx context.Context, ticker, pkg, category string, index int) (*GexData, error) {
	rawData, err := m.GetRawAtIndex(ctx, ticker, pkg, category, index)
	if err != nil {
		return nil, err
	}

	var gex GexData
	if err := json.Unmarshal(rawData, &gex); err != nil {
		return nil, fmt.Errorf("unmarshal error: %w", err)
	}
	return &gex, nil
}

func (m *MemoryLoader) GetRawAtIndex(ctx context.Context, ticker, pkg, category string, index int) ([]byte, error) {
	key := DataKey(ticker, pkg, category)
	data, ok := m.data[key]
	if !ok {
		return nil, ErrNotFound
	}
	if index < 0 || index >= len(data) {
		return nil, ErrIndexOutOfBounds
	}
	return data[index], nil
}

func (m *MemoryLoader) GetLength(ticker, pkg, category string) (int, error) {
	key := DataKey(ticker, pkg, category)
	data, ok := m.data[key]
	if !ok {
		return 0, ErrNotFound
	}
	return len(data), nil
}

func (m *MemoryLoader) Exists(ticker, pkg, category string) bool {
	key := DataKey(ticker, pkg, category)
	_, ok := m.data[key]
	return ok
}

func (m *MemoryLoader) Close() error {
	m.data = nil
	return nil
}

// GetLoadedKeys returns all loaded data keys (for /tickers endpoint)
func (m *MemoryLoader) GetLoadedKeys() []string {
	keys := make([]string, 0, len(m.data))
	for k := range m.data {
		keys = append(keys, k)
	}
	return keys
}
