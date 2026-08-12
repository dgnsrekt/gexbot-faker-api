package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/config"
	"github.com/dgnsrekt/gexbot-downloader/internal/coverage"
	"github.com/dgnsrekt/gexbot-downloader/internal/download"
	"github.com/dgnsrekt/gexbot-downloader/internal/downloadjob"
	"github.com/dgnsrekt/gexbot-downloader/internal/eod"
	"github.com/dgnsrekt/gexbot-downloader/internal/notify"
	"github.com/dgnsrekt/gexbot-downloader/internal/observability"
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

	observability.RegisterDaemon()
	// Seed last-success from the persisted tracker date so "last success age"
	// reflects the last known download instead of the Unix epoch (~56 years) when
	// this process hasn't performed a download yet (e.g. today's data was already
	// present on startup). A live successful download later overwrites it with the
	// exact time.
	if last := tracker.GetLastDownloadDate(); last != "" {
		if d, perr := time.Parse("2006-01-02", last); perr == nil {
			observability.DaemonLastSuccess.Set(float64(d.Unix()))
		}
	}

	runTimeout := time.Duration(daemonCfg.RunTimeoutMinutes) * time.Minute
	cleanupTTL := time.Duration(cfg.Output.CleanupAfterDays) * 24 * time.Hour
	var lastCleanup time.Time

	logger.Info("daemon started",
		zap.String("schedule", fmt.Sprintf("%02d:%02d %s", daemonCfg.ScheduleHour, daemonCfg.ScheduleMinute, daemonCfg.Timezone)),
		zap.Duration("runTimeout", runTimeout),
	)
	// /readyz reflects real init state (not a constant); /status serves sanitized
	// effective config + runtime state for the Studio (never any secret).
	statusProvider := func() any { return buildDaemonStatus(daemonCfg, cfg, notifyCfg, tracker, dstate) }
	diagnostics := observability.NewDiagnostics(":9091", dstate.isReady, logger, observability.WithStatus(statusProvider))
	diagnostics.Start(logger)
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := diagnostics.Stop(shutdownCtx); err != nil {
			logger.Warn("diagnostics shutdown error", zap.Error(err))
		}
	}()

	// Check on startup if enabled
	var nextAttempt time.Time
	// backfillComplete gates today's run: today must never advance the tracker
	// past an unfilled backfill gap (that would orphan the missed day forever).
	backfillComplete := true
	if daemonCfg.RunOnStartup {
		logger.Info("checking for missed downloads on startup")
		backfillComplete = backfillMissedDays(ctx, cfg, scheduler, tracker, notifier, logger, runTimeout)
		if !backfillComplete {
			nextAttempt = time.Now().Add(5 * time.Minute)
		} else if shouldDownload(scheduler, tracker, logger) {
			if !runDownload(ctx, cfg, scheduler, tracker, notifier, logger, runTimeout) {
				nextAttempt = time.Now().Add(5 * time.Minute)
			}
		}
	}

	// Reclaim stale materialized JSONL on startup (then hourly in the loop below).
	if cfg.Output.AutoCleanup {
		runCleanup(cfg, cleanupTTL, logger)
		lastCleanup = time.Now()
	}

	// Startup init (config load, seed, startup backfill/cleanup) is done — the daemon
	// is now scheduling, so /readyz reports ready. A later failed run is degraded, not
	// unready (see daemonState.isReady).
	dstate.setReady()

	// Main loop - check every minute
	ticker := time.NewTicker(1 * time.Minute)
	observability.DaemonNextRun.Set(float64(time.Now().Add(time.Minute).Unix()))
	defer ticker.Stop()

	for {
		select {
		case sig := <-sigCh:
			logger.Info("received shutdown signal", zap.String("signal", sig.String()))
			cancel()
			return 0

		case <-ticker.C:
			observability.DaemonNextRun.Set(float64(time.Now().Add(time.Minute).Unix()))
			if cfg.Output.AutoCleanup && time.Since(lastCleanup) >= cleanupInterval {
				runCleanup(cfg, cleanupTTL, logger)
				lastCleanup = time.Now()
			}
			if ready := nextAttempt.IsZero() || !time.Now().Before(nextAttempt); !ready {
				break // still backing off from a prior failure
			}
			// Retry an unfilled backfill gap before today's run, so a failed past
			// day is filled first and the tracker never advances past it.
			if !backfillComplete {
				if backfillComplete = backfillMissedDays(ctx, cfg, scheduler, tracker, notifier, logger, runTimeout); !backfillComplete {
					nextAttempt = time.Now().Add(5 * time.Minute)
					break
				}
			}
			if shouldDownload(scheduler, tracker, logger) {
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

// cleanupInterval is how often the daemon runs the materialized-JSONL TTL sweep
// (the retention window itself is output.cleanup_after_days).
const cleanupInterval = time.Hour

// runCleanup evicts materialized JSONL not loaded within the TTL, keeping the
// newest archive; the actively-served date is protected by the server's heartbeat
// keeping its .last-loaded fresh. The EOD archives are never touched.
func runCleanup(cfg *config.Config, ttl time.Duration, logger *zap.Logger) {
	latest, _ := eod.LatestDate(cfg.Output.Directory)
	if _, err := eod.CleanupStale(cfg.Output.Directory, ttl, logger, latest); err != nil {
		logger.Warn("materialize cleanup failed", zap.Error(err))
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
	observability.DaemonInProgress.Set(1)
	dstate.startRun()
	defer func() {
		observability.DaemonInProgress.Set(0)
		observability.DaemonDuration.Observe(time.Since(start).Seconds())
	}()

	// Hold the cross-process per-date lock so a Studio download for the same date
	// can't race this run over shared staging/tmp/archive files.
	var result *download.BatchResult
	var err error
	lockErr := downloadjob.WithDateLock(cfg.Output.Directory, today, func() error {
		// Bound the EOD attempt: a stalled request must fail into the 5-minute retry
		// loop, not wedge the daemon's single-goroutine ticker indefinitely.
		eodCtx, eodCancel := context.WithTimeout(ctx, runTimeout)
		result, err = downloadjob.ExecuteEODDownload(eodCtx, cfg, today, logger)
		eodCancel()

		if (err != nil || result == nil || result.Failed > 0) && scheduler.IsFallbackTime() {
			logger.Warn("EOD unavailable at fallback deadline; using individual downloads", zap.String("date", today))
			// Fresh deadline so an EOD stall that burned the full timeout can't leave
			// the fallback starting on an already-cancelled context.
			fbCtx, fbCancel := context.WithTimeout(ctx, runTimeout)
			result, err = downloadjob.ExecuteDownload(fbCtx, cfg, today, logger, nil)
			if err == nil && result.Failed == 0 {
				// PackAndMark (not PackMissingArchives) so the fallback's
				// auto-converted JSONL is marked materialized and the date shows
				// "ready", matching the Studio download path.
				err = downloadjob.PackAndMark(cfg, today)
			}
			fbCancel()
		}
		return nil // work-level errors are carried in err
	})
	if lockErr != nil && err == nil {
		err = lockErr
	}

	duration := time.Since(start)
	if err != nil || result == nil || result.Failed > 0 {
		observability.DaemonRuns.WithLabelValues("failed").Inc()
		if result != nil {
			observability.DaemonFiles.WithLabelValues("failed").Add(float64(result.Failed))
		}
		if err == nil {
			err = fmt.Errorf("%d EOD reports unavailable", result.Failed)
		}
		retryErr := fmt.Errorf("%w; retrying in 5 minutes", err)
		logger.Warn("download incomplete", zap.String("date", today), zap.Error(retryErr))
		dstate.finishRun(false, retryErr)
		if notifyErr := notifier.SendFailure(ctx, result, today, duration, retryErr); notifyErr != nil {
			logger.Warn("failed to send failure notification", zap.Error(notifyErr))
		}
		return false
	}

	logger.Info("download succeeded", zap.String("date", today), zap.Duration("duration", duration))
	observability.DaemonRuns.WithLabelValues("success").Inc()
	observability.DaemonLastSuccess.SetToCurrentTime()
	observability.DaemonFiles.WithLabelValues("success").Add(float64(result.Success))
	observability.DaemonFiles.WithLabelValues("skipped").Add(float64(result.Skipped))
	observability.DaemonFiles.WithLabelValues("not_found").Add(float64(result.NotFound))
	if notifyErr := notifier.SendSuccess(ctx, result, today, duration); notifyErr != nil {
		logger.Warn("failed to send success notification", zap.Error(notifyErr))
	}
	checkCoverage(ctx, cfg.Output.Directory, today, notifier, logger)

	if err := tracker.SetLastDownloadDate(today); err != nil {
		logger.Error("failed to update tracker", zap.Error(err))
		dstate.finishRun(false, err)
		return false
	}
	dstate.finishRun(true, nil)
	return true
}

// checkCoverage inspects a freshly downloaded date for coverage regressions (a
// ticker's intraday snapshots dropping well below its recent norm, or a session
// that opens late / closes early / has a large gap) and pushes an ntfy alert if
// any are found. Always logs findings so they're visible even when ntfy is off;
// never fails the run — a coverage warning is advisory, not a download failure.
func checkCoverage(ctx context.Context, dataDir, date string, notifier notify.Notifier, logger *zap.Logger) {
	// Export the per-ticker snapshot count so Grafana has the raw series to trend
	// and alert on, independent of the ntfy findings below. SetDaemonSnapshots
	// resets first so a ticker dropped from the set doesn't linger as a stale gauge.
	if snaps, err := eod.TickerSnapshots(dataDir, date); err == nil {
		observability.SetDaemonSnapshots(snaps)
	}
	rep, err := coverage.Check(dataDir, date, logger)
	if err != nil {
		logger.Warn("coverage check failed", zap.String("date", date), zap.Error(err))
		return
	}
	if rep.Empty() {
		return
	}
	var b strings.Builder
	for _, f := range rep.Findings {
		observability.DaemonCoverageFindings.WithLabelValues(f.Kind).Inc()
		fmt.Fprintf(&b, "%s [%s]: %s\n", f.Ticker, f.Kind, f.Detail)
		logger.Warn("coverage finding",
			zap.String("date", date), zap.String("ticker", f.Ticker),
			zap.String("kind", f.Kind), zap.String("detail", f.Detail))
	}
	title := fmt.Sprintf("Coverage warning: %s", date)
	if err := notifier.SendAlert(ctx, title, b.String(), "high"); err != nil {
		logger.Warn("failed to send coverage alert", zap.Error(err))
	}
}

// backfillLookbackDays is how far back (in calendar days) startup backfill
// reaches — the individual /hist endpoint only serves a ~90-day look-back window,
// so older missed days are unfetchable anyway.
const backfillLookbackDays = 90

// backfillMissedDays downloads any full market days missed between the last
// recorded download and today (today is left to the normal scheduled run). It
// returns true when the gap is fully closed (nothing missed, or every missed
// day within the look-back window succeeded) and false when it stopped on a
// failure — the caller must not advance the tracker past an unclosed gap.
//
// Past days must use the individual /hist/{date} endpoint (executeDownload):
// the EOD report endpoint only serves the single latest report, so it cannot
// fetch a specific past date. Each day is bounded by runTimeout so a stall in
// backfill can't wedge startup.
func backfillMissedDays(ctx context.Context, cfg *config.Config, scheduler *Scheduler, tracker *DownloadTracker, notifier notify.Notifier, logger *zap.Logger, runTimeout time.Duration) bool {
	last := tracker.GetLastDownloadDate()
	if last == "" {
		// No baseline — a fresh daemon must not backfill unbounded history.
		return true
	}
	missed, dropped := scheduler.MissedMarketDays(last, backfillLookbackDays)
	if len(missed) == 0 {
		return true
	}
	if dropped {
		logger.Warn("backfill gap exceeds look-back window; older days skipped",
			zap.String("since", last), zap.Int("backfilling", len(missed)), zap.Int("lookbackDays", backfillLookbackDays))
	}
	logger.Info("backfilling missed market days", zap.Strings("dates", missed))
	dstate.startRun()

	for _, date := range missed {
		var result *download.BatchResult
		var err error
		lockErr := downloadjob.WithDateLock(cfg.Output.Directory, date, func() error {
			runCtx, cancel := context.WithTimeout(ctx, runTimeout)
			defer cancel()
			result, err = downloadjob.ExecuteDownload(runCtx, cfg, date, logger, nil)
			if err == nil && result.Failed == 0 { // ExecuteDownload returns a non-nil error on zero tasks
				err = downloadjob.PackAndMark(cfg, date) // mark materialized → date shows "ready"
			}
			return nil // work-level errors carried in err
		})
		if lockErr != nil && err == nil {
			err = lockErr
		}

		if err != nil || result == nil || result.Failed > 0 {
			if err == nil {
				err = fmt.Errorf("%d downloads failed", result.Failed)
			}
			// Stop at the first gap and report incomplete so today's run is held
			// back; the tracker stays at the last contiguous date and the loop
			// retries from here next cycle.
			logger.Warn("backfill incomplete; holding today's run until gap fills", zap.String("date", date), zap.Error(err))
			dstate.finishRun(false, fmt.Errorf("backfill %s: %w", date, err))
			if notifyErr := notifier.SendFailure(ctx, result, date, 0, fmt.Errorf("backfill: %w", err)); notifyErr != nil {
				logger.Warn("failed to send backfill failure notification", zap.Error(notifyErr))
			}
			return false
		}

		logger.Info("backfilled missed date", zap.String("date", date))
		checkCoverage(ctx, cfg.Output.Directory, date, notifier, logger)
		if err := tracker.SetLastDownloadDate(date); err != nil {
			logger.Error("failed to update tracker after backfill", zap.Error(err))
			dstate.finishRun(false, err) // don't leave /status stuck in_progress
			return false
		}
		// A backfilled download just succeeded — record it so last-success stays
		// consistent with the tracker this run advanced. Without this, a
		// startup that only backfills (e.g. a weekend, no live runDownload after)
		// keeps reporting the pre-backfill seed until the next restart. Matches
		// runDownload's SetToCurrentTime, placed after the tracker write.
		observability.DaemonLastSuccess.SetToCurrentTime()
	}
	dstate.finishRun(true, nil)
	return true
}
