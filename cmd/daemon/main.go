package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/config"
)

func main() {
	os.Exit(run())
}

func run() int {
	// Setup logger
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger: %v\n", err)
		return 1
	}
	defer logger.Sync()

	// Load daemon config
	daemonCfg := LoadDaemonConfig()

	logger.Info("daemon configuration loaded",
		zap.Int("scheduleHour", daemonCfg.ScheduleHour),
		zap.Int("scheduleMinute", daemonCfg.ScheduleMinute),
		zap.String("timezone", daemonCfg.Timezone),
		zap.String("configPath", daemonCfg.ConfigPath),
		zap.String("stateFile", daemonCfg.StateFile),
		zap.Bool("runOnStartup", daemonCfg.RunOnStartup),
	)

	// Load downloader config
	cfg, err := config.Load(daemonCfg.ConfigPath)
	if err != nil {
		logger.Error("failed to load downloader config", zap.Error(err))
		return 1
	}

	logger.Info("downloader configuration loaded",
		zap.String("outputDir", cfg.Output.Directory),
		zap.Int("workers", cfg.Download.Workers),
		zap.Int("tickers", len(cfg.Tickers)),
	)

	// Setup context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Create scheduler and tracker
	scheduler := NewScheduler(daemonCfg.ScheduleHour, daemonCfg.ScheduleMinute, daemonCfg.Timezone)
	tracker := NewDownloadTracker(daemonCfg.StateFile)

	logger.Info("daemon started",
		zap.String("schedule", fmt.Sprintf("%02d:%02d %s", daemonCfg.ScheduleHour, daemonCfg.ScheduleMinute, daemonCfg.Timezone)),
	)

	// Check on startup if enabled
	if daemonCfg.RunOnStartup {
		logger.Info("checking for missed download on startup")
		if shouldDownload(scheduler, tracker, logger) {
			runDownload(ctx, cfg, scheduler, tracker, logger)
		}
	}

	// Main loop - check every minute
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case sig := <-sigCh:
			logger.Info("received shutdown signal", zap.String("signal", sig.String()))
			cancel()
			return 0

		case <-ticker.C:
			if shouldDownload(scheduler, tracker, logger) {
				runDownload(ctx, cfg, scheduler, tracker, logger)
			}

		case <-ctx.Done():
			logger.Info("context cancelled, shutting down")
			return 0
		}
	}
}

// shouldDownload checks if conditions are met for triggering a download
func shouldDownload(scheduler *Scheduler, tracker *DownloadTracker, logger *zap.Logger) bool {
	today := scheduler.TodayDate()

	// Check if already downloaded today
	if tracker.AlreadyDownloaded(today) {
		return false
	}

	// Check if it's a market day
	if !scheduler.IsMarketDay(today) {
		logger.Debug("not a market day", zap.String("date", today))
		return false
	}

	// Check if it's the scheduled time
	if !scheduler.IsScheduledTime() {
		return false
	}

	logger.Info("download conditions met",
		zap.String("date", today),
		zap.String("time", time.Now().In(scheduler.Location()).Format("15:04:05")),
	)

	return true
}

// runDownload executes the download and updates the tracker
func runDownload(ctx context.Context, cfg *config.Config, scheduler *Scheduler, tracker *DownloadTracker, logger *zap.Logger) {
	today := scheduler.TodayDate()

	logger.Info("starting scheduled download", zap.String("date", today))
	start := time.Now()

	if err := executeDownload(ctx, cfg, today, logger); err != nil {
		logger.Error("download failed", zap.Error(err), zap.String("date", today))
		return
	}

	duration := time.Since(start)
	logger.Info("download succeeded",
		zap.String("date", today),
		zap.Duration("duration", duration),
	)

	// Update tracker to prevent re-download
	if err := tracker.SetLastDownloadDate(today); err != nil {
		logger.Error("failed to update tracker", zap.Error(err))
	}
}
