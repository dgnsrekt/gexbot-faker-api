package data

import "sync"

// CacheMode defines how playback handles end-of-data
type CacheMode string

const (
	CacheModeExhaust  CacheMode = "exhaust"  // 404 at end
	CacheModeRotation CacheMode = "rotation" // wrap to 0
)

// IndexCache tracks playback positions per API key
type IndexCache struct {
	mu      sync.RWMutex
	indexes map[string]int // key: ticker/pkg/category/apiKey
	mode    CacheMode
}

func NewIndexCache(mode CacheMode) *IndexCache {
	return &IndexCache{
		indexes: make(map[string]int),
		mode:    mode,
	}
}

// CacheKey creates the composite key for index tracking (independent mode)
func CacheKey(ticker, pkg, category, apiKey string) string {
	return ticker + "/" + pkg + "/" + category + "/" + apiKey
}

// SharedCacheKey creates a cache key for shared mode (ignores category)
// All endpoints for the same ticker/pkg share the same index counter
func SharedCacheKey(ticker, pkg, apiKey string) string {
	return ticker + "/" + pkg + "/" + apiKey
}

// GetAndAdvance returns the current index and advances it
// Returns (index, isExhausted)
func (c *IndexCache) GetAndAdvance(key string, dataLength int) (int, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	idx := c.indexes[key]

	// Check exhaustion in exhaust mode
	if c.mode == CacheModeExhaust && idx >= dataLength {
		return idx, true
	}

	// Get current index (may need wrap in rotation mode)
	currentIdx := idx
	if c.mode == CacheModeRotation && idx >= dataLength {
		currentIdx = idx % dataLength
	}

	// Advance for next request
	if c.mode == CacheModeRotation {
		c.indexes[key] = (idx + 1) % dataLength
	} else {
		c.indexes[key] = idx + 1
	}

	return currentIdx, false
}

// Reset resets indexes, optionally for a specific API key pattern
func (c *IndexCache) Reset(apiKey string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	if apiKey == "" {
		// Reset all
		count := len(c.indexes)
		c.indexes = make(map[string]int)
		return count
	}

	// Reset matching keys (ending with /apiKey)
	suffix := "/" + apiKey
	count := 0
	for k := range c.indexes {
		if len(k) > len(suffix) && k[len(k)-len(suffix):] == suffix {
			delete(c.indexes, k)
			count++
		}
	}
	return count
}

// GetIndex returns current index without advancing (for debugging)
func (c *IndexCache) GetIndex(key string) int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.indexes[key]
}
