package main

import (
	"os"
	"strconv"
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
		ConfigPath:     getEnvOrDefault("DAEMON_CONFIG_PATH", "/app/configs/default.yaml"),
		ScheduleHour:   getEnvIntOrDefault("DAEMON_SCHEDULE_HOUR", 20),
		ScheduleMinute: getEnvIntOrDefault("DAEMON_SCHEDULE_MINUTE", 0),
		Timezone:       getEnvOrDefault("DAEMON_TIMEZONE", "America/New_York"),
		StateFile:      getEnvOrDefault("DAEMON_STATE_FILE", "/app/data/.daemon-state"),
		RunOnStartup:   getEnvBoolOrDefault("DAEMON_RUN_ON_STARTUP", true),
	}
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvIntOrDefault(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvBoolOrDefault(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	}
	return defaultVal
}
