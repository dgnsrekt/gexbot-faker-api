package main

import (
	"github.com/dgnsrekt/gexbot-downloader/internal/envutil"
)

// DaemonConfig holds daemon-specific configuration
type DaemonConfig struct {
	ConfigPath     string // Path to downloader config YAML
	ScheduleHour   int    // Hour in timezone (default: 20 for 8 PM)
	ScheduleMinute int    // Minute (default: 0)
	Timezone       string // Timezone (default: America/New_York)
	StateFile      string // File to track last download date
	RunOnStartup   bool   // Check/download on startup if missed
}

// LoadDaemonConfig loads configuration from environment variables
func LoadDaemonConfig() *DaemonConfig {
	return &DaemonConfig{
		ConfigPath:     envutil.GetString("DAEMON_CONFIG_PATH", "/app/configs/default.yaml"),
		ScheduleHour:   envutil.GetInt("DAEMON_SCHEDULE_HOUR", 20),
		ScheduleMinute: envutil.GetInt("DAEMON_SCHEDULE_MINUTE", 0),
		Timezone:       envutil.GetString("DAEMON_TIMEZONE", "America/New_York"),
		StateFile:      envutil.GetString("DAEMON_STATE_FILE", "/app/data/.daemon-state"),
		RunOnStartup:   envutil.GetBool("DAEMON_RUN_ON_STARTUP", true),
	}
}
