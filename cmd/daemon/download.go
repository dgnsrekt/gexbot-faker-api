package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/api"
	"github.com/dgnsrekt/gexbot-downloader/internal/config"
	"github.com/dgnsrekt/gexbot-downloader/internal/download"
	"github.com/dgnsrekt/gexbot-downloader/internal/staging"
)

// DownloadTracker tracks the last successfully downloaded date
type DownloadTracker struct {
	stateFile string
}

// NewDownloadTracker creates a new tracker with the given state file path
func NewDownloadTracker(stateFile string) *DownloadTracker {
	return &DownloadTracker{stateFile: stateFile}
}

// GetLastDownloadDate reads the last successful download date from state file
func (t *DownloadTracker) GetLastDownloadDate() string {
	data, err := os.ReadFile(t.stateFile)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// SetLastDownloadDate writes the date to the state file
func (t *DownloadTracker) SetLastDownloadDate(date string) error {
	// Ensure directory exists
	dir := filepath.Dir(t.stateFile)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}
	return os.WriteFile(t.stateFile, []byte(date+"\n"), 0600)
}

// AlreadyDownloaded checks if the given date was already downloaded
func (t *DownloadTracker) AlreadyDownloaded(date string) bool {
	return t.GetLastDownloadDate() == date
}

// executeDownload runs the download for the given date using existing internal packages.
// Returns the batch result and any error that occurred.
func executeDownload(ctx context.Context, cfg *config.Config, date string, logger *zap.Logger) (*download.BatchResult, error) {
	logger.Info("starting download", zap.String("date", date))

	// Create API client
	client := api.NewClient(
		cfg.API.BaseURL,
		cfg.API.APIKey,
		cfg.Download.RatePerSecond,
		time.Duration(cfg.API.TimeoutSec)*time.Second,
		time.Duration(cfg.API.RetryDelay)*time.Second,
		cfg.API.RetryCount,
		logger,
	)

	// Create staging manager
	stgMgr := staging.NewManager(cfg.Output.Directory)

	// Create download manager
	dlMgr := download.NewManager(client, stgMgr, cfg.Download.Workers, logger)

	// Generate tasks for this date
	tasks := generateTasksForDate(cfg, date)
	logger.Info("generated tasks", zap.Int("count", len(tasks)))

	if len(tasks) == 0 {
		logger.Warn("no tasks generated, check config")
		return nil, nil
	}

	// Execute downloads
	result, err := dlMgr.Execute(ctx, tasks)
	if err != nil {
		return result, err
	}

	// Commit staging to final location and cleanup (only if there were actual downloads)
	if result.Success > 0 {
		if err := stgMgr.CommitStaging(date); err != nil {
			logger.Warn("failed to commit staging", zap.String("date", date), zap.Error(err))
		}
		if err := stgMgr.CleanupStaging(date); err != nil {
			logger.Warn("failed to cleanup staging", zap.String("date", date), zap.Error(err))
		}

		// Auto-convert JSON to JSONL if enabled
		if cfg.Output.AutoConvertToJSONL {
			logger.Info("auto-converting JSON to JSONL")
			dir := filepath.Join(cfg.Output.Directory, date)
			if err := convertJSONToJSONL(dir, logger); err != nil {
				logger.Warn("auto-conversion failed", zap.String("date", date), zap.Error(err))
			}
		}
	}

	logger.Info("download complete",
		zap.Int("total", result.Total),
		zap.Int("success", result.Success),
		zap.Int("skipped", result.Skipped),
		zap.Int("not_found", result.NotFound),
		zap.Int("failed", result.Failed),
	)

	if result.Failed > 0 {
		for _, e := range result.Errors {
			logger.Error("download error", zap.String("error", e))
		}
	}

	return result, nil
}

// generateTasksForDate creates download tasks for a single date based on config
func generateTasksForDate(cfg *config.Config, date string) []download.Task {
	var tasks []download.Task

	// Determine tickers
	tickers := cfg.Tickers
	if len(tickers) == 0 {
		tickers = config.DefaultTickers()
	}

	// Build package/category map
	pkgCategories := make(map[string][]string)

	if cfg.Packages.State.Enabled {
		cats := cfg.Packages.State.Categories
		if len(cats) == 0 {
			cats = config.ValidCategories[config.PackageState]
		}
		pkgCategories["state"] = cats
	}
	if cfg.Packages.Classic.Enabled {
		cats := cfg.Packages.Classic.Categories
		if len(cats) == 0 {
			cats = config.ValidCategories[config.PackageClassic]
		}
		pkgCategories["classic"] = cats
	}
	if cfg.Packages.Orderflow.Enabled {
		cats := cfg.Packages.Orderflow.Categories
		if len(cats) == 0 {
			cats = config.ValidCategories[config.PackageOrderflow]
		}
		pkgCategories["orderflow"] = cats
	}

	// Generate tasks for all combinations
	for _, ticker := range tickers {
		for pkg, categories := range pkgCategories {
			for _, category := range categories {
				tasks = append(tasks, download.Task{
					Ticker:   ticker,
					Package:  pkg,
					Category: category,
					Date:     date,
				})
			}
		}
	}

	return tasks
}

// convertJSONToJSONL converts JSON files in a directory to JSONL format
func convertJSONToJSONL(dir string, logger *zap.Logger) error {
	var converted, skipped, failed int

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-JSON files
		if info.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}

		// Skip staging directory
		if strings.Contains(path, ".staging") {
			return nil
		}

		jsonlPath := strings.TrimSuffix(path, ".json") + ".jsonl"

		// Skip if JSONL already exists
		if _, err := os.Stat(jsonlPath); err == nil {
			logger.Debug("skipping, JSONL exists", zap.String("file", path))
			skipped++
			return nil
		}

		logger.Debug("converting", zap.String("file", path))

		if err := convertFile(path, jsonlPath); err != nil {
			logger.Error("conversion failed", zap.String("file", path), zap.Error(err))
			failed++
			return nil // Continue with other files
		}

		// Delete original JSON after successful conversion
		if err := os.Remove(path); err != nil {
			logger.Warn("failed to delete original", zap.String("file", path), zap.Error(err))
		}

		converted++
		return nil
	})

	if err != nil {
		return err
	}

	logger.Info("conversion complete",
		zap.Int("converted", converted),
		zap.Int("skipped", skipped),
		zap.Int("failed", failed),
	)

	return nil
}

// convertFile converts a single JSON array file to JSONL format
func convertFile(jsonPath, jsonlPath string) error {
	// Read JSON file
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return err
	}

	// Parse as array of raw JSON messages
	var items []json.RawMessage
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	// Create JSONL file
	outFile, err := os.Create(jsonlPath)
	if err != nil {
		return err
	}
	defer func() { _ = outFile.Close() }()

	// Write each item as a line
	for _, item := range items {
		// Compact the JSON (remove whitespace)
		compact, err := json.Marshal(item)
		if err != nil {
			return err
		}

		if _, err := outFile.Write(compact); err != nil {
			return err
		}
		if _, err := outFile.WriteString("\n"); err != nil {
			return err
		}
	}

	return nil
}
