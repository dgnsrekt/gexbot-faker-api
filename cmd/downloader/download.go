package main

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/api"
	"github.com/dgnsrekt/gexbot-downloader/internal/config"
	"github.com/dgnsrekt/gexbot-downloader/internal/download"
	"github.com/dgnsrekt/gexbot-downloader/internal/staging"
)

func downloadCmd() *cobra.Command {
	var (
		dryRun   bool
		tickers  []string
		packages []string
	)

	cmd := &cobra.Command{
		Use:   "download YYYY-MM-DD [END_DATE]",
		Short: "Download historical data for specified date(s)",
		Long: `Download historical data from Gexbot API for the specified date(s).

Date format: YYYY-MM-DD (e.g., 2025-11-14)

Examples:
  # Download single date
  gexbot-downloader download 2025-11-14

  # Download date range
  gexbot-downloader download 2025-11-01 2025-11-14

  # Override tickers from config
  gexbot-downloader download --tickers SPX,NDX 2025-11-14

  # Dry run to see what would be downloaded
  gexbot-downloader download --dry-run 2025-11-14`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Parse dates
			dates, err := parseDates(args)
			if err != nil {
				return err
			}

			// Determine effective tickers for validation
			effectiveTickers := cfg.Tickers
			if len(tickers) > 0 {
				effectiveTickers = tickers
			}
			if len(effectiveTickers) == 0 {
				effectiveTickers = config.DefaultTickers()
			}

			// Validate configuration before downloading
			if err := config.ValidateDownloadConfig(effectiveTickers, cfg.Packages); err != nil {
				return err
			}

			// Generate tasks
			tasks := generateTasks(cfg, dates, tickers, packages)

			logger.Info("generated tasks", zap.Int("count", len(tasks)))

			if dryRun {
				for _, t := range tasks {
					fmt.Printf("Would download: %s\n", t)
				}
				return nil
			}

			// Create client
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

			// Execute downloads
			result, err := dlMgr.Execute(ctx, tasks)
			if err != nil {
				return err
			}

			// Commit staging to final location and cleanup (only if there were actual downloads)
			if result.Success > 0 {
				for _, date := range dates {
					if err := stgMgr.CommitStaging(date); err != nil {
						logger.Warn("failed to commit staging", zap.String("date", date), zap.Error(err))
					}
					if err := stgMgr.CleanupStaging(date); err != nil {
						logger.Warn("failed to cleanup staging", zap.String("date", date), zap.Error(err))
					}
				}

				// Auto-convert JSON to JSONL if enabled
				if cfg.Output.AutoConvertToJSONL {
					logger.Info("auto-converting JSON to JSONL")
					for _, date := range dates {
						dir := filepath.Join(cfg.Output.Directory, date)
						if err := convertJSONToJSONL(dir); err != nil {
							logger.Warn("auto-conversion failed", zap.String("date", date), zap.Error(err))
						}
					}
				}
			}

			// Print summary
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
				return fmt.Errorf("%d downloads failed", result.Failed)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be downloaded")
	cmd.Flags().StringSliceVar(&tickers, "tickers", nil, "override tickers from config")
	cmd.Flags().StringSliceVar(&packages, "packages", nil, "override packages from config (state,classic,orderflow)")

	return cmd
}
