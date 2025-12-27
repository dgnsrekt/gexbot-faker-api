package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

type ServerConfig struct {
	Port              string
	DataDir           string
	DataDate          string
	DataMode          string // "memory" or "stream"
	CacheMode         string // "exhaust" or "rotation"
	EndpointCacheMode string // "shared" or "independent"
	// WebSocket configuration
	WSEnabled        bool
	WSStreamInterval time.Duration
	WSGroupPrefix    string
	// Sync Broadcast System configuration
	SyncBroadcastSystemEnabled  bool
	SyncBroadcastSystemID       string
	SyncBroadcastSystemInterval time.Duration
}

func LoadServerConfig() (*ServerConfig, error) {
	dataDir := getEnvOrDefault("DATA_DIR", "./data")
	dataDate := getEnvOrDefault("DATA_DATE", "")

	// Auto-detect latest date if DATA_DATE is empty or "latest"
	if dataDate == "" || dataDate == "latest" {
		detected, err := detectLatestDate(dataDir)
		if err != nil {
			return nil, fmt.Errorf("failed to detect latest date in %s: %w", dataDir, err)
		}
		dataDate = detected
	}

	// Parse WebSocket stream interval
	wsIntervalStr := getEnvOrDefault("WS_STREAM_INTERVAL", "1s")
	wsInterval, err := time.ParseDuration(wsIntervalStr)
	if err != nil {
		wsInterval = time.Second // Default to 1s on parse error
	}

	// Parse Sync Broadcast System interval
	syncIntervalStr := getEnvOrDefault("SYNC_BROADCAST_SYSTEM_INTERVAL", "1s")
	syncInterval, err := time.ParseDuration(syncIntervalStr)
	if err != nil {
		syncInterval = time.Second // Default to 1s on parse error
	}

	// Get default broadcast ID from hostname
	syncBroadcastID := getEnvOrDefault("SYNC_BROADCAST_SYSTEM_ID", "")
	if syncBroadcastID == "" {
		hostname, _ := os.Hostname()
		if hostname != "" {
			syncBroadcastID = hostname
		} else {
			syncBroadcastID = "gexbot-faker"
		}
	}

	cfg := &ServerConfig{
		Port:              getEnvOrDefault("PORT", "8080"),
		DataDir:           dataDir,
		DataDate:          dataDate,
		DataMode:          getEnvOrDefault("DATA_MODE", "memory"),
		CacheMode:         getEnvOrDefault("CACHE_MODE", "exhaust"),
		EndpointCacheMode: getEnvOrDefault("ENDPOINT_CACHE_MODE", "shared"),
		WSEnabled:         getEnvOrDefault("WS_ENABLED", "true") == "true",
		WSStreamInterval:  wsInterval,
		WSGroupPrefix:     getEnvOrDefault("WS_GROUP_PREFIX", "blue"),
		// Sync Broadcast System
		SyncBroadcastSystemEnabled:  getEnvOrDefault("SYNC_BROADCAST_SYSTEM_ENABLED", "false") == "true",
		SyncBroadcastSystemID:       syncBroadcastID,
		SyncBroadcastSystemInterval: syncInterval,
	}

	// Validate
	if cfg.DataMode != "memory" && cfg.DataMode != "stream" {
		return nil, fmt.Errorf("invalid DATA_MODE: %s (must be 'memory' or 'stream')", cfg.DataMode)
	}
	if cfg.CacheMode != "exhaust" && cfg.CacheMode != "rotation" {
		return nil, fmt.Errorf("invalid CACHE_MODE: %s (must be 'exhaust' or 'rotation')", cfg.CacheMode)
	}
	if cfg.EndpointCacheMode != "shared" && cfg.EndpointCacheMode != "independent" {
		return nil, fmt.Errorf("invalid ENDPOINT_CACHE_MODE: %s (must be 'shared' or 'independent')", cfg.EndpointCacheMode)
	}

	return cfg, nil
}

// detectLatestDate scans the data directory for date folders and returns the most recent one
func detectLatestDate(dataDir string) (string, error) {
	datePattern := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return "", fmt.Errorf("reading data directory: %w", err)
	}

	var dates []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if datePattern.MatchString(name) {
			// Verify it's not empty (has at least one file/folder inside)
			subPath := filepath.Join(dataDir, name)
			subEntries, err := os.ReadDir(subPath)
			if err == nil && len(subEntries) > 0 {
				dates = append(dates, name)
			}
		}
	}

	if len(dates) == 0 {
		return "", fmt.Errorf("no date folders found in %s", dataDir)
	}

	// Sort descending (newest first) - YYYY-MM-DD format sorts lexicographically
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))

	return dates[0], nil
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
