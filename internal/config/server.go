package config

import (
	"fmt"
	"os"
)

type ServerConfig struct {
	Port              string
	DataDir           string
	DataDate          string
	DataMode          string // "memory" or "stream"
	CacheMode         string // "exhaust" or "rotation"
	EndpointCacheMode string // "shared" or "independent"
}

func LoadServerConfig() (*ServerConfig, error) {
	cfg := &ServerConfig{
		Port:              getEnvOrDefault("PORT", "8080"),
		DataDir:           getEnvOrDefault("DATA_DIR", "./data"),
		DataDate:          getEnvOrDefault("DATA_DATE", "2025-11-28"),
		DataMode:          getEnvOrDefault("DATA_MODE", "memory"),
		CacheMode:         getEnvOrDefault("CACHE_MODE", "exhaust"),
		EndpointCacheMode: getEnvOrDefault("ENDPOINT_CACHE_MODE", "shared"),
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

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
