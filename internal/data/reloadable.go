package data

import (
	"context"
	"sync"
)

// ReloadableLoader wraps a DataLoader and allows atomic replacement.
// All DataLoader methods delegate to the current underlying loader.
// This enables hot-reloading of data without stopping the server.
type ReloadableLoader struct {
	mu      sync.RWMutex
	current DataLoader
}

// NewReloadableLoader creates a new ReloadableLoader with the given initial loader.
func NewReloadableLoader(initial DataLoader) *ReloadableLoader {
	return &ReloadableLoader{
		current: initial,
	}
}

// Swap atomically replaces the underlying loader and returns the old one.
// Caller is responsible for closing the old loader after swap.
func (r *ReloadableLoader) Swap(newLoader DataLoader) DataLoader {
	r.mu.Lock()
	defer r.mu.Unlock()
	old := r.current
	r.current = newLoader
	return old
}

// GetAtIndex returns the GexData at the given index.
func (r *ReloadableLoader) GetAtIndex(ctx context.Context, ticker, pkg, category string, index int) (*GexData, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.current.GetAtIndex(ctx, ticker, pkg, category, index)
}

// GetRawAtIndex returns the raw JSON bytes at the given index.
func (r *ReloadableLoader) GetRawAtIndex(ctx context.Context, ticker, pkg, category string, index int) ([]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.current.GetRawAtIndex(ctx, ticker, pkg, category, index)
}

// GetLength returns the number of data points available.
func (r *ReloadableLoader) GetLength(ticker, pkg, category string) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.current.GetLength(ticker, pkg, category)
}

// Exists checks if data exists for the given combination.
func (r *ReloadableLoader) Exists(ticker, pkg, category string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.current.Exists(ticker, pkg, category)
}

// GetLoadedKeys returns all loaded data keys.
func (r *ReloadableLoader) GetLoadedKeys() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.current.GetLoadedKeys()
}

// Close releases any resources held by the current loader.
func (r *ReloadableLoader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.current.Close()
}

// Compile-time interface verification
var _ DataLoader = (*ReloadableLoader)(nil)
