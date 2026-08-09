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
	runTimeout := time.Duration(daemonCfg.RunTimeoutMinutes) * time.Minute

	logger.Info("daemon started",
		zap.String("schedule", fmt.Sprintf("%02d:%02d %s", daemonCfg.ScheduleHour, daemonCfg.ScheduleMinute, daemonCfg.Timezone)),
		zap.Duration("runTimeout", runTimeout),
	)

	// Check on startup if enabled
	var nextAttempt time.Time
	if daemonCfg.RunOnStartup {
		logger.Info("checking for missed downloads on startup")
		// Backfill any full market days missed while the daemon was down/wedged
		// before handling today's scheduled run.
		backfillMissedDays(ctx, cfg, scheduler, tracker, notifier, logger, runTimeout)
		if shouldDownload(scheduler, tracker, logger) {
			if !runDownload(ctx, cfg, scheduler, tracker, notifier, logger, runTimeout) {
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
				if runDownload(ctx, cfg, scheduler, tracker, notifier, logger, runTimeout) {
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
func runDownload(ctx context.Context, cfg *config.Config, scheduler *Scheduler, tracker *DownloadTracker, notifier notify.Notifier, logger *zap.Logger, runTimeout time.Duration) bool {
	today := scheduler.TodayDate()

	logger.Info("starting scheduled EOD download", zap.String("date", today))
	start := time.Now()

	// Bound the run: a stalled request must fail into the 5-minute retry loop,
	// not wedge the daemon's single-goroutine ticker indefinitely.
	runCtx, cancel := context.WithTimeout(ctx, runTimeout)
	defer cancel()

	result, err := executeEODDownload(runCtx, cfg, today, logger)
	duration := time.Since(start)

	if (err != nil || result == nil || result.Failed > 0) && scheduler.IsFallbackTime() {
		logger.Warn("EOD unavailable at fallback deadline; using individual downloads", zap.String("date", today))
		result, err = executeDownload(runCtx, cfg, today, logger)
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

// maxBackfillDays caps how far back startup backfill reaches — the individual
// /hist endpoint only serves a ~90-day look-back window.
const maxBackfillDays = 90

// backfillMissedDays downloads any full market days missed between the last
// recorded download and today (today is left to the normal scheduled run).
//
// Past days must use the individual /hist/{date} endpoint (executeDownload):
// the EOD report endpoint only serves the single latest report, so it cannot
// fetch a specific past date. Each day is bounded by runTimeout so a stall in
// backfill can't wedge startup.
func backfillMissedDays(ctx context.Context, cfg *config.Config, scheduler *Scheduler, tracker *DownloadTracker, notifier notify.Notifier, logger *zap.Logger, runTimeout time.Duration) {
	last := tracker.GetLastDownloadDate()
	if last == "" {
		// No baseline — a fresh daemon must not backfill unbounded history.
		return
	}
	missed, capped := scheduler.MissedMarketDays(last, maxBackfillDays)
	if len(missed) == 0 {
		return
	}
	if capped {
		logger.Warn("backfill gap exceeds look-back window; older days skipped",
			zap.String("since", last), zap.Int("backfilling", len(missed)), zap.Int("maxDays", maxBackfillDays))
	}
	logger.Info("backfilling missed market days", zap.Strings("dates", missed))

	for _, date := range missed {
		runCtx, cancel := context.WithTimeout(ctx, runTimeout)
		result, err := executeDownload(runCtx, cfg, date, logger)
		if err == nil && result != nil && result.Failed == 0 {
			err = packMissingArchives(cfg, date)
		}
		cancel()

		if err != nil || result == nil || result.Failed > 0 {
			if err == nil {
				if result == nil {
					err = fmt.Errorf("no download tasks generated (check package config)")
				} else {
					err = fmt.Errorf("%d downloads failed", result.Failed)
				}
			}
			// Stop at the first gap so the tracker's high-water-mark stays
			// contiguous; the next startup retries from here.
			logger.Warn("backfill incomplete; stopping (retries next startup)", zap.String("date", date), zap.Error(err))
			if notifyErr := notifier.SendFailure(ctx, result, date, 0, fmt.Errorf("backfill: %w", err)); notifyErr != nil {
				logger.Warn("failed to send backfill failure notification", zap.Error(notifyErr))
			}
			return
		}

		logger.Info("backfilled missed date", zap.String("date", date))
		if err := tracker.SetLastDownloadDate(date); err != nil {
			logger.Error("failed to update tracker after backfill", zap.Error(err))
			return
		}
	}
}
