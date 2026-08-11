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

// FindIndexByTimestamp delegates to the underlying loader.
func (r *ReloadableLoader) FindIndexByTimestamp(ctx context.Context, ticker, pkg, category string, targetTimestamp int64) (int, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.current.FindIndexByTimestamp(ctx, ticker, pkg, category, targetTimestamp)
}

// SeekResolve gives the seek handler a single, range-aware call path. When the underlying loader is
// a RangeLoader it resolves across the span (gap/clamp/end-policy detail); otherwise (single-day
// loader) it falls back to FindIndexByTimestamp with default fields — identical to the pre-range
// behavior (after-span still errors), so the single-day seek path is unchanged. Date is left empty
// for the fallback; the handler fills it from the currently-loaded date.
func (r *ReloadableLoader) SeekResolve(ctx context.Context, ticker, pkg, category string, target int64, endPolicy string) (SeekResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if rs, ok := r.current.(RangeSeeker); ok {
		return rs.SeekResolve(ctx, ticker, pkg, category, target, endPolicy)
	}
	idx, ts, err := r.current.FindIndexByTimestamp(ctx, ticker, pkg, category, target)
	if err != nil {
		return SeekResult{}, err
	}
	return SeekResult{Index: idx, ResolvedTs: ts, Clamped: "none"}, nil
}

// Compile-time interface verification
var (
	_ DataLoader  = (*ReloadableLoader)(nil)
	_ RangeSeeker = (*ReloadableLoader)(nil)
)
