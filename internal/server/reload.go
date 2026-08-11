package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/config"
	"github.com/dgnsrekt/gexbot-downloader/internal/data"
	"github.com/dgnsrekt/gexbot-downloader/internal/eod"
	"github.com/dgnsrekt/gexbot-downloader/internal/observability"
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
	loadedDates []string // the loaded span (chronological); one element for a single-day load
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
		loadedDates: []string{cfg.DataDate},
		loadedAt:    time.Now(),
	}
}

// IsReloading returns true if a reload is currently in progress.
// WebSocket streamers should check this and skip broadcasts during reload.
func (rm *ReloadManager) IsReloading() bool {
	return rm.isReloading.Load()
}

// CurrentDate returns the currently loaded data date (the span start when a range is loaded).
func (rm *ReloadManager) CurrentDate() string {
	rm.stateMu.RLock()
	defer rm.stateMu.RUnlock()
	return rm.currentDate
}

// LoadedDates returns the currently loaded span in chronological order (one element for a
// single-day load).
func (rm *ReloadManager) LoadedDates() []string {
	rm.stateMu.RLock()
	defer rm.stateMu.RUnlock()
	out := make([]string, len(rm.loadedDates))
	copy(out, rm.loadedDates)
	return out
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
	started := time.Now()
	result := "failed"
	defer func() {
		observability.ReloadDuration.Observe(time.Since(started).Seconds())
		observability.Reloads.WithLabelValues(result).Inc()
	}()
	// Prevent concurrent reloads
	if !rm.reloadMu.TryLock() {
		return nil, fmt.Errorf("reload already in progress")
	}
	defer rm.reloadMu.Unlock()
	observability.ReloadInProgress.Set(1)
	defer observability.ReloadInProgress.Set(0)

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
	if _, err := os.Stat(datePath); os.IsNotExist(err) {
		if err := eod.MaterializeDate(rm.config.DataDir, newDate, rm.logger); err != nil {
			return nil, fmt.Errorf("date not found: %s", newDate)
		}
	}
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
	rm.loadedDates = []string{newDate}
	rm.loadedAt = time.Now()
	rm.config.DataDate = newDate
	loadedAt := rm.loadedAt
	rm.stateMu.Unlock()

	// Resume streamers
	rm.isReloading.Store(false)
	result = "success"
	observability.DataLoadedTimestamp.SetToCurrentTime()
	if date, err := time.Parse("2006-01-02", newDate); err == nil {
		observability.DataDateTimestamp.Set(float64(date.Unix()))
	}

	// Close old loader (release resources)
	if err := oldLoader.Close(); err != nil {
		rm.logger.Warn("failed to close old loader", zap.Error(err))
	}
	// Record the load so the daemon's TTL cleanup keeps this date warm. Proactive
	// cleanup is now owned by the daemon (see internal/eod.CleanupStale).
	if err := eod.TouchLoaded(rm.config.DataDir, newDate); err != nil {
		rm.logger.Warn("failed to mark loaded date", zap.Error(err))
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

// createRangeLoader builds a cross-day RangeLoader over the given dates. Range loads always use
// stream mode regardless of DATA_MODE: a multi-day span in memory mode would hold every day's JSONL
// in RAM (200-540 MB/day), defeating the bounded-RAM property multi-day replay depends on. Stream
// mode holds only per-day offset slices + file handles. Single-day /reload-date still honors
// DATA_MODE via createLoader.
func (rm *ReloadManager) createRangeLoader(dates []string) (data.DataLoader, error) {
	return data.NewRangeLoader(rm.config.DataDir, dates, "stream", rm.logger)
}

// ReloadRange loads a contiguous span of dates as one continuous dataset (materializing any missing
// day on demand), swaps it in atomically, and resets cursors — the multi-day sibling of Reload. The
// span start becomes currentDate so /current-date stays populated. On any failure the existing data
// stays intact. Dates are deduped + sorted chronologically here.
func (rm *ReloadManager) ReloadRange(ctx context.Context, dates []string) (*ReloadResult, error) {
	started := time.Now()
	result := "failed"
	defer func() {
		observability.ReloadDuration.Observe(time.Since(started).Seconds())
		observability.Reloads.WithLabelValues(result).Inc()
	}()

	if !rm.reloadMu.TryLock() {
		return nil, fmt.Errorf("reload already in progress")
	}
	defer rm.reloadMu.Unlock()
	observability.ReloadInProgress.Set(1)
	defer observability.ReloadInProgress.Set(0)

	norm := normalizeDates(dates)
	if len(norm) == 0 {
		return nil, fmt.Errorf("no dates provided")
	}
	for _, d := range norm {
		if !isValidDateFormat(d) {
			return nil, fmt.Errorf("invalid date format: %s (expected YYYY-MM-DD)", d)
		}
	}

	previousDate := rm.CurrentDate()
	rm.logger.Info("starting range reload", zap.Strings("dates", norm), zap.String("previousDate", previousDate))

	// Materialize any day whose JSONL isn't on disk yet, then verify each is a directory.
	for _, d := range norm {
		datePath := filepath.Join(rm.config.DataDir, d)
		if _, err := os.Stat(datePath); os.IsNotExist(err) {
			if err := eod.MaterializeDate(rm.config.DataDir, d, rm.logger); err != nil {
				return nil, fmt.Errorf("date not found: %s", d)
			}
		}
		info, err := os.Stat(datePath)
		if err != nil {
			return nil, fmt.Errorf("date not found: %s", d)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("date path is not a directory: %s", d)
		}
	}

	newLoader, err := rm.createRangeLoader(norm)
	if err != nil {
		return nil, fmt.Errorf("failed to load range %v: %w", norm, err)
	}
	loadedKeys := newLoader.GetLoadedKeys()
	if len(loadedKeys) == 0 {
		if closeErr := newLoader.Close(); closeErr != nil {
			rm.logger.Warn("failed to close new range loader after empty load", zap.Error(closeErr))
		}
		return nil, fmt.Errorf("no data files found for range: %v", norm)
	}

	rm.isReloading.Store(true)
	time.Sleep(100 * time.Millisecond)
	oldLoader := rm.loader.Swap(newLoader)
	resetCount := rm.cache.Reset("")

	rm.stateMu.Lock()
	rm.currentDate = norm[0]
	rm.loadedDates = norm
	rm.loadedAt = time.Now()
	rm.config.DataDate = norm[0]
	loadedAt := rm.loadedAt
	rm.stateMu.Unlock()

	rm.isReloading.Store(false)
	result = "success"
	observability.DataLoadedTimestamp.SetToCurrentTime()
	if date, err := time.Parse("2006-01-02", norm[0]); err == nil {
		observability.DataDateTimestamp.Set(float64(date.Unix()))
	}

	if err := oldLoader.Close(); err != nil {
		rm.logger.Warn("failed to close old loader", zap.Error(err))
	}
	for _, d := range norm {
		if err := eod.TouchLoaded(rm.config.DataDir, d); err != nil {
			rm.logger.Warn("failed to mark loaded date", zap.String("date", d), zap.Error(err))
		}
	}

	rm.logger.Info("range reload complete",
		zap.Strings("dates", norm),
		zap.Time("loadedAt", loadedAt),
		zap.Int("filesLoaded", len(loadedKeys)),
		zap.Int("cachePositionsReset", resetCount),
	)

	return &ReloadResult{
		PreviousDate: previousDate,
		NewDate:      norm[0],
		LoadedAt:     loadedAt,
		FilesLoaded:  len(loadedKeys),
	}, nil
}

// normalizeDates dedupes and sorts dates chronologically (YYYY-MM-DD sorts lexically = chronologically).
func normalizeDates(dates []string) []string {
	seen := make(map[string]struct{}, len(dates))
	out := make([]string, 0, len(dates))
	for _, d := range dates {
		if d == "" {
			continue
		}
		if _, ok := seen[d]; ok {
			continue
		}
		seen[d] = struct{}{}
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

// isValidDateFormat checks if the date matches YYYY-MM-DD format.
func isValidDateFormat(date string) bool {
	pattern := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	return pattern.MatchString(date)
}
