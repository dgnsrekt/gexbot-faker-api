package data

import (
	"context"
	"errors"
)

var (
	ErrNotFound         = errors.New("data not found")
	ErrIndexOutOfBounds = errors.New("index out of bounds")
)

// DataLoader provides random access to GEX data
type DataLoader interface {
	// GetAtIndex returns the GexData at the given index
	GetAtIndex(ctx context.Context, ticker, pkg, category string, index int) (*GexData, error)

	// GetLength returns the number of data points available
	GetLength(ticker, pkg, category string) (int, error)

	// Exists checks if data exists for the given combination
	Exists(ticker, pkg, category string) bool

	// Close releases any resources
	Close() error
}

// DataKey creates a unique key for ticker/package/category
func DataKey(ticker, pkg, category string) string {
	return ticker + "/" + pkg + "/" + category
}
