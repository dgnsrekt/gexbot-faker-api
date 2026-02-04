package data

import (
	"context"
	"encoding/json"
	"errors"
)

var (
	ErrNotFound            = errors.New("data not found")
	ErrIndexOutOfBounds    = errors.New("index out of bounds")
	ErrTimestampOutOfRange = errors.New("timestamp out of range")
)

// DataLoader provides random access to GEX data
type DataLoader interface {
	// GetAtIndex returns the GexData at the given index
	GetAtIndex(ctx context.Context, ticker, pkg, category string, index int) (*GexData, error)

	// GetRawAtIndex returns the raw JSON bytes at the given index
	// This allows handlers to parse into different data types (GexData, GreekData, etc.)
	GetRawAtIndex(ctx context.Context, ticker, pkg, category string, index int) ([]byte, error)

	// GetLength returns the number of data points available
	GetLength(ticker, pkg, category string) (int, error)

	// Exists checks if data exists for the given combination
	Exists(ticker, pkg, category string) bool

	// GetLoadedKeys returns all loaded data keys (for /tickers endpoint)
	GetLoadedKeys() []string

	// FindIndexByTimestamp returns the first index where timestamp >= target.
	// Returns (index, actualTimestamp, error).
	// Returns ErrNotFound if no data exists for the key.
	// Returns ErrTimestampOutOfRange if target is after all data.
	FindIndexByTimestamp(ctx context.Context, ticker, pkg, category string, targetTimestamp int64) (int, int64, error)

	// Close releases any resources
	Close() error
}

// DataKey creates a unique key for ticker/package/category
func DataKey(ticker, pkg, category string) string {
	return ticker + "/" + pkg + "/" + category
}

// ExtractTimestamp quickly extracts timestamp from JSON without full unmarshal.
// This is used by both MemoryLoader and StreamLoader for binary search operations.
func ExtractTimestamp(raw []byte) int64 {
	var partial struct {
		Timestamp int64 `json:"timestamp"`
	}
	_ = json.Unmarshal(raw, &partial)
	return partial.Timestamp
}
