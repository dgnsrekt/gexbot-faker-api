package main

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dgnsrekt/gexbot-downloader/internal/config"
	"github.com/dgnsrekt/gexbot-downloader/internal/notify"
)

// buildDaemonStatus reports effective config + runtime state and NEVER leaks a
// secret (API key, ntfy token) — the whole point of a sanitized /status.
func TestBuildDaemonStatusSanitized(t *testing.T) {
	dc := &DaemonConfig{
		ConfigPath: "/app/configs/custom.yaml", ScheduleHour: 17, ScheduleMinute: 5,
		Timezone: "America/New_York", RunOnStartup: true, RunTimeoutMinutes: 45,
		StateFile: filepath.Join(t.TempDir(), ".daemon-state"),
	}
	cfg := &config.Config{Tickers: []string{"SPX", "NDX"}}
	cfg.API.APIKey = "SUPER_SECRET_API_KEY"
	cfg.Output.Directory = "/app/data"
	cfg.Output.AutoCleanup = true
	cfg.Output.CleanupAfterDays = 7
	cfg.Packages.State.Enabled = true
	cfg.Packages.State.Categories = []string{"gex_zero"}
	cfg.Packages.Orderflow.Enabled = true

	nc := &notify.Config{Enabled: true, Server: "https://ntfy.sh", Topic: "gex", Priority: "high", Tags: "package", Token: "NTFY_SECRET_TOKEN"}
	tracker := NewDownloadTracker(dc.StateFile)

	st := buildDaemonStatus(dc, cfg, nc, tracker, &daemonState{})

	if st.ConfigPath != dc.ConfigPath || len(st.Tickers) != 2 {
		t.Errorf("status did not reflect effective config: %+v", st)
	}
	if len(st.Packages) != 2 || st.Packages[0].Name != "state" || st.Packages[0].Categories[0] != "gex_zero" {
		t.Errorf("packages = %+v, want state(gex_zero) + orderflow", st.Packages)
	}
	if !st.Cleanup.Enabled || st.Cleanup.RetentionDays != 7 {
		t.Errorf("cleanup = %+v", st.Cleanup)
	}
	if st.Notifications.Topic != "gex" {
		t.Errorf("notifications = %+v", st.Notifications)
	}

	// The wire form must contain NO secret.
	blob, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"SUPER_SECRET_API_KEY", "NTFY_SECRET_TOKEN", "api_key", "token"} {
		if strings.Contains(string(blob), secret) {
			t.Errorf("daemon status leaked %q: %s", secret, blob)
		}
	}
}

// /readyz reflects real init state; a failed run stays ready (degraded, not unready).
func TestDaemonStateReadyz(t *testing.T) {
	s := &daemonState{}
	if s.isReady() {
		t.Error("a fresh daemon should not be ready before init")
	}
	s.setReady()
	if !s.isReady() {
		t.Error("after setReady it should be ready")
	}

	// startRun → in_progress; every finishRun path (incl. the backfill tracker-write
	// failure branch) must clear it, so /status never reports a run that has stopped.
	s.startRun()
	if !s.inProgress {
		t.Error("startRun should mark in_progress")
	}
	s.finishRun(false, errors.New("EOD unavailable"))
	if s.inProgress {
		t.Error("finishRun must clear in_progress even on failure")
	}
	if !s.isReady() {
		t.Error("a failed download is degraded, not unready — readyz must stay true")
	}
	if s.lastResult != "failed" || s.lastError != "EOD unavailable" {
		t.Errorf("after failed run: result=%q err=%q", s.lastResult, s.lastError)
	}

	s.finishRun(true, nil)
	if s.lastResult != "success" || s.lastError != "" {
		t.Errorf("after success run: result=%q err=%q", s.lastResult, s.lastError)
	}
}
