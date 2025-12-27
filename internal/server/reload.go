package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/config"
	"github.com/dgnsrekt/gexbot-downloader/internal/data"
)

// ReloadManager coordinates data reloading across server components.
// It manages the atomic swap of data loaders and cache reset during hot reload.
type ReloadManager struct {
	loader *data.ReloadableLoader
	cache  *data.IndexCache
	config *config.ServerConfig
	logger *zap.Logger

	// Reload state
	isReloading atomic.Bool
	reloadMu    sync.Mutex // prevents concurrent reloads

	// Current state
	currentDate string
	loadedAt    time.Time
	stateMu     sync.RWMutex
}

// NewReloadManager creates a new ReloadManager.
func NewReloadManager(
	loader *data.ReloadableLoader,
	cache *data.IndexCache,
	cfg *config.ServerConfig,
	logger *zap.Logger,
) *ReloadManager {
	return &ReloadManager{
		loader:      loader,
		cache:       cache,
		config:      cfg,
		logger:      logger,
		currentDate: cfg.DataDate,
		loadedAt:    time.Now(),
	}
}

// IsReloading returns true if a reload is currently in progress.
// WebSocket streamers should check this and skip broadcasts during reload.
func (rm *ReloadManager) IsReloading() bool {
	return rm.isReloading.Load()
}

// CurrentDate returns the currently loaded data date.
func (rm *ReloadManager) CurrentDate() string {
	rm.stateMu.RLock()
	defer rm.stateMu.RUnlock()
	return rm.currentDate
}

// LoadedAt returns the timestamp when the current data was loaded.
func (rm *ReloadManager) LoadedAt() time.Time {
	rm.stateMu.RLock()
	defer rm.stateMu.RUnlock()
	return rm.loadedAt
}

// ReloadResult contains the result of a successful reload operation.
type ReloadResult struct {
	PreviousDate string
	NewDate      string
	LoadedAt     time.Time
	FilesLoaded  int
}

// Reload validates the new date, loads new data, swaps the loader, and resets the cache.
// Returns error if reload fails (original data remains intact in that case).
func (rm *ReloadManager) Reload(ctx context.Context, newDate string) (*ReloadResult, error) {
	// Prevent concurrent reloads
	if !rm.reloadMu.TryLock() {
		return nil, fmt.Errorf("reload already in progress")
	}
	defer rm.reloadMu.Unlock()

	previousDate := rm.CurrentDate()

	rm.logger.Info("starting hot reload",
		zap.String("previousDate", previousDate),
		zap.String("newDate", newDate),
	)

	// Validate date format
	if !isValidDateFormat(newDate) {
		return nil, fmt.Errorf("invalid date format: %s (expected YYYY-MM-DD)", newDate)
	}

	// Check if date directory exists
	datePath := filepath.Join(rm.config.DataDir, newDate)
	info, err := os.Stat(datePath)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("date not found: %s", newDate)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to check date directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("date path is not a directory: %s", newDate)
	}

	// Create new loader for the new date
	newLoader, err := rm.createLoader(newDate)
	if err != nil {
		return nil, fmt.Errorf("failed to load data for %s: %w", newDate, err)
	}

	// Check if we actually loaded any data
	loadedKeys := newLoader.GetLoadedKeys()
	if len(loadedKeys) == 0 {
		if closeErr := newLoader.Close(); closeErr != nil {
			rm.logger.Warn("failed to close new loader after empty load", zap.Error(closeErr))
		}
		return nil, fmt.Errorf("no data files found for date: %s", newDate)
	}

	// Signal streamers to pause
	rm.isReloading.Store(true)

	// Give streamers time to finish current broadcast cycle
	time.Sleep(100 * time.Millisecond)

	// Swap the loader atomically
	oldLoader := rm.loader.Swap(newLoader)

	// Reset all cache positions
	resetCount := rm.cache.Reset("")

	// Update current state
	rm.stateMu.Lock()
	rm.currentDate = newDate
	rm.loadedAt = time.Now()
	rm.config.DataDate = newDate
	loadedAt := rm.loadedAt
	rm.stateMu.Unlock()

	// Resume streamers
	rm.isReloading.Store(false)

	// Close old loader (release resources)
	if err := oldLoader.Close(); err != nil {
		rm.logger.Warn("failed to close old loader", zap.Error(err))
	}

	rm.logger.Info("hot reload complete",
		zap.String("previousDate", previousDate),
		zap.String("newDate", newDate),
		zap.Time("loadedAt", loadedAt),
		zap.Int("filesLoaded", len(loadedKeys)),
		zap.Int("cachePositionsReset", resetCount),
	)

	return &ReloadResult{
		PreviousDate: previousDate,
		NewDate:      newDate,
		LoadedAt:     loadedAt,
		FilesLoaded:  len(loadedKeys),
	}, nil
}

// createLoader creates a new DataLoader based on the configured data mode.
func (rm *ReloadManager) createLoader(date string) (data.DataLoader, error) {
	switch rm.config.DataMode {
	case "memory":
		return data.NewMemoryLoader(rm.config.DataDir, date, rm.logger)
	case "stream":
		return data.NewStreamLoader(rm.config.DataDir, date, rm.logger)
	default:
		return nil, fmt.Errorf("unknown data mode: %s", rm.config.DataMode)
	}
}

// isValidDateFormat checks if the date matches YYYY-MM-DD format.
func isValidDateFormat(date string) bool {
	pattern := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	return pattern.MatchString(date)
}
