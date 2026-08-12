package main

import (
	"sync"
	"time"

	"github.com/dgnsrekt/gexbot-downloader/internal/config"
	"github.com/dgnsrekt/gexbot-downloader/internal/notify"
	"github.com/dgnsrekt/gexbot-downloader/internal/redact"
)

// daemonState is the mutex-guarded runtime state that /status and /readyz read. It
// is a package-level singleton (dstate), mirroring the observability gauges, so the
// run loop updates it without threading it through every function.
type daemonState struct {
	mu         sync.RWMutex
	ready      bool
	inProgress bool
	lastResult string    // "" | "success" | "failed"
	lastError  string    // redacted summary of the last failure (never a secret)
	lastRunAt  time.Time // when the last run finished (zero = none this process)
}

var dstate = &daemonState{}

// setReady marks the daemon ready — called once startup init finishes and the
// scheduling loop begins. During startup backfill readyz reports "unavailable".
func (s *daemonState) setReady() { s.mu.Lock(); s.ready = true; s.mu.Unlock() }

// isReady drives /readyz. A *failed download run* is degraded, not unready — it
// stays true and the failure surfaces via /status.last_result, so orchestration
// doesn't churn the daemon over a transient upstream gap.
func (s *daemonState) isReady() bool { s.mu.RLock(); defer s.mu.RUnlock(); return s.ready }

func (s *daemonState) startRun() { s.mu.Lock(); s.inProgress = true; s.mu.Unlock() }

func (s *daemonState) finishRun(ok bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inProgress = false
	s.lastRunAt = time.Now()
	if ok {
		s.lastResult = "success"
		s.lastError = ""
		return
	}
	s.lastResult = "failed"
	if err != nil {
		s.lastError = redact.Text(err.Error()) // scrub any signed-URL query strings
	}
}

// --- sanitized status wire types ---

type daemonStatusPackage struct {
	Name       string   `json:"name"`
	Categories []string `json:"categories"`
}

type daemonSchedule struct {
	Hour              int    `json:"hour"`
	Minute            int    `json:"minute"`
	Timezone          string `json:"timezone"`
	RunOnStartup      bool   `json:"run_on_startup"`
	RunTimeoutMinutes int    `json:"run_timeout_minutes"`
}

type daemonCleanup struct {
	Enabled       bool `json:"enabled"`
	RetentionDays int  `json:"retention_days"`
}

// daemonNotifications is the ntfy config MINUS the token — never expose secrets.
type daemonNotifications struct {
	Enabled  bool   `json:"enabled"`
	Server   string `json:"server"`
	Topic    string `json:"topic"`
	Priority string `json:"priority"`
	Tags     string `json:"tags"`
}

// daemonStatus is the sanitized effective config + runtime state served at
// GET :9091/status. It carries only explicit, safe fields — no GEXBOT_API_KEY,
// NTFY_TOKEN, or auth headers.
type daemonStatus struct {
	Ready              bool                  `json:"ready"`
	InProgress         bool                  `json:"in_progress"`
	LastResult         string                `json:"last_result,omitempty"`
	LastError          string                `json:"last_error,omitempty"`
	LastRunAt          string                `json:"last_run_at,omitempty"`
	LastDownloadedDate string                `json:"last_downloaded_date,omitempty"`
	ConfigPath         string                `json:"config_path"`
	OutputDir          string                `json:"output_dir"`
	Tickers            []string              `json:"tickers"`
	Packages           []daemonStatusPackage `json:"packages"`
	Schedule           daemonSchedule        `json:"schedule"`
	Cleanup            daemonCleanup         `json:"cleanup"`
	Notifications      daemonNotifications   `json:"notifications"`
}

// buildDaemonStatus assembles the sanitized daemon status from effective config +
// runtime state. Explicit fields only — no struct-dump — so a secret can never leak
// by accident.
func buildDaemonStatus(dc *DaemonConfig, cfg *config.Config, nc *notify.Config, tracker *DownloadTracker, s *daemonState) daemonStatus {
	var st daemonStatus

	s.mu.RLock()
	st.Ready = s.ready
	st.InProgress = s.inProgress
	st.LastResult = s.lastResult
	st.LastError = s.lastError
	if !s.lastRunAt.IsZero() {
		st.LastRunAt = s.lastRunAt.UTC().Format(time.RFC3339)
	}
	s.mu.RUnlock()

	st.LastDownloadedDate = tracker.GetLastDownloadDate()
	st.ConfigPath = dc.ConfigPath
	st.OutputDir = cfg.Output.Directory
	st.Tickers = config.EffectiveTickers(cfg)
	for _, p := range config.EffectivePackages(cfg) {
		st.Packages = append(st.Packages, daemonStatusPackage{Name: p.Name, Categories: p.Categories})
	}
	st.Schedule = daemonSchedule{
		Hour:              dc.ScheduleHour,
		Minute:            dc.ScheduleMinute,
		Timezone:          dc.Timezone,
		RunOnStartup:      dc.RunOnStartup,
		RunTimeoutMinutes: dc.RunTimeoutMinutes,
	}
	st.Cleanup = daemonCleanup{Enabled: cfg.Output.AutoCleanup, RetentionDays: cfg.Output.CleanupAfterDays}
	if nc != nil {
		st.Notifications = daemonNotifications{
			Enabled:  nc.Enabled,
			Server:   nc.Server,
			Topic:    nc.Topic,
			Priority: nc.Priority,
			Tags:     nc.Tags,
		}
	}
	return st
}
