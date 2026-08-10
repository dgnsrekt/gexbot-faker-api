// Package downloadjob holds the per-date download orchestration shared by the
// daemon (scheduled/backfill runs) and the server (Studio Download screen). It
// combines the download engine, staging, and EOD packing for one market day; it
// was lifted verbatim out of cmd/daemon so both binaries can call it.
package downloadjob

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/api"
	"github.com/dgnsrekt/gexbot-downloader/internal/config"
	"github.com/dgnsrekt/gexbot-downloader/internal/download"
	"github.com/dgnsrekt/gexbot-downloader/internal/eod"
	"github.com/dgnsrekt/gexbot-downloader/internal/staging"
)

// ExecuteDownload downloads a date via the individual /hist endpoint (works for
// any past date), commits staging, and optionally converts JSON→JSONL. progress
// (may be nil) is invoked once per completed task with (done, total) for a live
// UI feed.
func ExecuteDownload(ctx context.Context, cfg *config.Config, date string, logger *zap.Logger, progress func(done, total int)) (*download.BatchResult, error) {
	logger.Info("starting download", zap.String("date", date))

	client := api.NewClient(
		cfg.API.BaseURL,
		cfg.API.APIKey,
		cfg.Download.RatePerSecond,
		time.Duration(cfg.API.TimeoutSec)*time.Second,
		time.Duration(cfg.API.RetryDelay)*time.Second,
		cfg.API.RetryCount,
		logger,
	)
	stgMgr := staging.NewManager(cfg.Output.Directory)
	dlMgr := download.NewManager(client, stgMgr, cfg.Download.Workers, logger)
	if progress != nil {
		dlMgr.OnProgress(progress)
	}

	tasks := GenerateTasksForDate(cfg, date)
	logger.Info("generated tasks", zap.Int("count", len(tasks)))
	if len(tasks) == 0 {
		return nil, fmt.Errorf("no download tasks generated for %s (check package config)", date)
	}

	result, err := dlMgr.Execute(ctx, tasks)
	if err != nil {
		return result, err
	}

	if result.Success > 0 {
		if err := stgMgr.CommitStaging(date); err != nil {
			logger.Warn("failed to commit staging", zap.String("date", date), zap.Error(err))
		}
		if err := stgMgr.CleanupStaging(date); err != nil {
			logger.Warn("failed to cleanup staging", zap.String("date", date), zap.Error(err))
		}
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

// ExecuteEODDownload downloads the single latest EOD report per ticker (cannot
// target an arbitrary past date). Used by the daemon's scheduled run.
func ExecuteEODDownload(ctx context.Context, cfg *config.Config, date string, logger *zap.Logger) (*download.BatchResult, error) {
	tickers := cfg.Tickers
	if len(tickers) == 0 {
		tickers = config.DefaultTickers()
	}
	result := &download.BatchResult{Total: len(tickers)}
	client := api.NewClient(
		cfg.API.BaseURL, cfg.API.APIKey, cfg.Download.RatePerSecond,
		time.Duration(cfg.API.TimeoutSec)*time.Second,
		time.Duration(cfg.API.RetryDelay)*time.Second, cfg.API.RetryCount, logger,
	)

	for _, ticker := range tickers {
		archive := eod.ArchivePath(cfg.Output.Directory, date, ticker)
		if _, err := os.Stat(eod.ManifestPath(archive)); err == nil {
			result.Skipped++
			continue
		}
		if err := os.MkdirAll(filepath.Dir(archive), 0750); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", ticker, err))
			continue
		}
		tmp := archive + ".tmp"
		file, err := os.Create(tmp)
		if err == nil {
			_, err = client.DownloadEODReport(ctx, ticker, file)
			if closeErr := file.Close(); err == nil {
				err = closeErr
			}
		}
		if err == nil {
			err = os.Rename(tmp, archive)
		}
		if err == nil {
			var manifest *eod.Manifest
			manifest, err = eod.Verify(archive, date, ticker, "gexbot-eod")
			if err == nil {
				err = validateEODManifest(cfg, ticker, manifest)
			}
		}
		if err != nil {
			_ = os.Remove(tmp)
			_ = os.Remove(archive)
			_ = os.Remove(eod.ManifestPath(archive))
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", ticker, err))
			continue
		}
		result.Success++
	}
	return result, nil
}

func validateEODManifest(cfg *config.Config, ticker string, manifest *eod.Manifest) error {
	have := make(map[string]bool, len(manifest.Members))
	for _, member := range manifest.Members {
		have[member.Package+"/"+member.Category] = true
	}
	for _, task := range GenerateTasksForDate(cfg, manifest.Date) {
		if task.Ticker == ticker && !have[task.Package+"/"+task.Category] {
			return fmt.Errorf("missing EOD member %s/%s", task.Package, task.Category)
		}
	}
	return nil
}

// PackMissingArchives packs individually-downloaded files into EOD archives for
// tickers lacking a manifest (the fallback that turns ExecuteDownload output into
// archives the Library can load).
func PackMissingArchives(cfg *config.Config, date string) error {
	tickers := cfg.Tickers
	if len(tickers) == 0 {
		tickers = config.DefaultTickers()
	}
	for _, ticker := range tickers {
		archive := eod.ArchivePath(cfg.Output.Directory, date, ticker)
		if _, err := os.Stat(eod.ManifestPath(archive)); err == nil {
			continue
		}
		if _, err := eod.Pack(cfg.Output.Directory, date, ticker, "individual-fallback"); err != nil {
			return fmt.Errorf("%s: %w", ticker, err)
		}
	}
	return nil
}

// GenerateTasksForDate expands the configured tickers × enabled packages ×
// categories into download tasks for one date.
func GenerateTasksForDate(cfg *config.Config, date string) []download.Task {
	var tasks []download.Task
	tickers := cfg.Tickers
	if len(tickers) == 0 {
		tickers = config.DefaultTickers()
	}

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

func convertJSONToJSONL(dir string, logger *zap.Logger) error {
	var converted, skipped, failed int
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		if strings.Contains(path, ".staging") {
			return nil
		}
		jsonlPath := strings.TrimSuffix(path, ".json") + ".jsonl"
		if _, err := os.Stat(jsonlPath); err == nil {
			skipped++
			return nil
		}
		if err := convertFile(path, jsonlPath); err != nil {
			logger.Error("conversion failed", zap.String("file", path), zap.Error(err))
			failed++
			return nil
		}
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
		zap.Int("converted", converted), zap.Int("skipped", skipped), zap.Int("failed", failed))
	return nil
}

func convertFile(jsonPath, jsonlPath string) error {
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return err
	}
	var items []json.RawMessage
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}
	outFile, err := os.Create(jsonlPath)
	if err != nil {
		return err
	}
	defer func() { _ = outFile.Close() }()
	for _, item := range items {
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
