// Package envutil provides helpers for reading environment variables with defaults.
package envutil

import (
	"os"
	"strconv"
	"time"
)

// GetString returns the value of the environment variable named by key,
// or defaultVal if the environment variable is empty or unset.
func GetString(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// GetBool returns the boolean value of the environment variable named by key,
// or defaultVal if the environment variable is empty, unset, or cannot be parsed.
// Parsing uses strconv.ParseBool which accepts 1, t, T, TRUE, true, True, 0, f, F, FALSE, false, False.
func GetBool(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	}
	return defaultVal
}

// GetInt returns the integer value of the environment variable named by key,
// or defaultVal if the environment variable is empty, unset, or cannot be parsed.
func GetInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

// GetDuration returns the duration value of the environment variable named by key,
// or defaultVal if the environment variable is empty, unset, or cannot be parsed.
// Parsing uses time.ParseDuration which accepts strings like "300ms", "1.5h", "2h45m".
func GetDuration(key string, defaultVal time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return defaultVal
}
