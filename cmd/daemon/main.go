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
	"github.com/dgnsrekt/gexbot-downloader/internal/notify"
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
	defer func() { _ = logger.Sync() }()

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
	tickers := cfg.Tickers
	if len(tickers) == 0 {
		tickers = config.DefaultTickers()
	}
	if err := config.ValidateDownloadConfig(tickers, cfg.Packages); err != nil {
		logger.Error("invalid downloader configuration", zap.Error(err))
		return 1
	}

	logger.Info("downloader configuration loaded",
		zap.String("outputDir", cfg.Output.Directory),
		zap.Int("workers", cfg.Download.Workers),
		zap.Int("tickers", len(cfg.Tickers)),
	)

	// Load notification config
	notifyCfg := notify.LoadConfig()
	if err := notifyCfg.Validate(); err != nil {
		logger.Error("invalid notification config", zap.Error(err))
		return 1
	}
	notifier := notify.New(notifyCfg, logger)

	logger.Info("notification configuration loaded",
		zap.Bool("enabled", notifyCfg.Enabled),
		zap.String("server", notifyCfg.Server),
		zap.String("topic", notifyCfg.Topic),
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
	var nextAttempt time.Time
	if daemonCfg.RunOnStartup {
		logger.Info("checking for missed download on startup")
		if shouldDownload(scheduler, tracker, logger) {
			if !runDownload(ctx, cfg, scheduler, tracker, notifier, logger) {
				nextAttempt = time.Now().Add(5 * time.Minute)
			}
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
			if shouldDownload(scheduler, tracker, logger) && (nextAttempt.IsZero() || !time.Now().Before(nextAttempt)) {
				if runDownload(ctx, cfg, scheduler, tracker, notifier, logger) {
					nextAttempt = time.Time{}
				} else {
					nextAttempt = time.Now().Add(5 * time.Minute)
				}
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
	if !scheduler.IsScheduledOrLater() {
		return false
	}

	logger.Info("download conditions met",
		zap.String("date", today),
		zap.String("time", time.Now().In(scheduler.Location()).Format("15:04:05")),
	)

	return true
}

// runDownload executes the download and updates the tracker
func runDownload(ctx context.Context, cfg *config.Config, scheduler *Scheduler, tracker *DownloadTracker, notifier notify.Notifier, logger *zap.Logger) bool {
	today := scheduler.TodayDate()

	logger.Info("starting scheduled EOD download", zap.String("date", today))
	start := time.Now()

	result, err := executeEODDownload(ctx, cfg, today, logger)
	duration := time.Since(start)

	if (err != nil || result == nil || result.Failed > 0) && scheduler.IsFallbackTime() {
		logger.Warn("EOD unavailable at fallback deadline; using individual downloads", zap.String("date", today))
		result, err = executeDownload(ctx, cfg, today, logger)
		if err == nil && result.Failed == 0 {
			err = packMissingArchives(cfg, today)
		}
	}

	duration = time.Since(start)
	if err != nil || result == nil || result.Failed > 0 {
		if err == nil {
			err = fmt.Errorf("%d EOD reports unavailable", result.Failed)
		}
		retryErr := fmt.Errorf("%w; retrying in 5 minutes", err)
		logger.Warn("download incomplete", zap.String("date", today), zap.Error(retryErr))
		if notifyErr := notifier.SendFailure(ctx, result, today, duration, retryErr); notifyErr != nil {
			logger.Warn("failed to send failure notification", zap.Error(notifyErr))
		}
		return false
	}

	logger.Info("download succeeded", zap.String("date", today), zap.Duration("duration", duration))
	if notifyErr := notifier.SendSuccess(ctx, result, today, duration); notifyErr != nil {
		logger.Warn("failed to send success notification", zap.Error(notifyErr))
	}

	if err := tracker.SetLastDownloadDate(today); err != nil {
		logger.Error("failed to update tracker", zap.Error(err))
		return false
	}
	return true
}
