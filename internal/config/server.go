package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
	// LokiURL is the base URL of a Loki instance the Studio Logs screen queries
	// (e.g. http://loki:3100). Empty disables the Logs feed (it degrades with a
	// "start the observability stack" message).
	LokiURL string
	// StudioAuthToken, when set, gates all /studio routes behind HTTP Basic auth
	// (any username, password = the token). Empty leaves the Studio open (local
	// dev default). Set it whenever the API port is reachable beyond a trusted
	// host — the Studio exposes control endpoints and container logs.
	StudioAuthToken string
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
		LokiURL:                     getEnvOrDefault("LOKI_URL", ""),
		StudioAuthToken:             getEnvOrDefault("STUDIO_AUTH_TOKEN", ""),
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

// detectLatestDate returns the most recent available date. It prefers the newest
// EOD archive under dataDir/eod — archives exist even for dates the server hasn't
// materialized yet (those materialize on demand at load) — and falls back to the
// newest non-empty materialized date dir.
func detectLatestDate(dataDir string) (string, error) {
	datePattern := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

	if archived := newestDateDir(filepath.Join(dataDir, "eod"), datePattern, false); archived != "" {
		return archived, nil
	}
	if materialized := newestDateDir(dataDir, datePattern, true); materialized != "" {
		return materialized, nil
	}
	return "", fmt.Errorf("no date folders found in %s", dataDir)
}

// newestDateDir returns the greatest (newest, since YYYY-MM-DD sorts lexically)
// date-named subdir of dir. When requireNonEmpty, only dirs with at least one
// entry qualify. Returns "" if none match or dir can't be read.
func newestDateDir(dir string, pat *regexp.Regexp, requireNonEmpty bool) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	best := ""
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || !pat.MatchString(name) || name <= best {
			continue
		}
		if requireNonEmpty {
			if sub, err := os.ReadDir(filepath.Join(dir, name)); err != nil || len(sub) == 0 {
				continue
			}
		}
		best = name
	}
	return best
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
