package sync

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dgnsrekt/gexbot-downloader/internal/config"
	"github.com/dgnsrekt/gexbot-downloader/internal/data"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// errLoader implements data.DataLoader with GetLength always failing, so
// buildPositions takes its error-logging path.
type errLoader struct{}

func (errLoader) GetAtIndex(context.Context, string, string, string, int) (*data.GexData, error) {
	return nil, errors.New("nope")
}
func (errLoader) GetRawAtIndex(context.Context, string, string, string, int) ([]byte, error) {
	return nil, errors.New("nope")
}
func (errLoader) GetLength(string, string, string) (int, error) { return 0, errors.New("boom") }
func (errLoader) Exists(string, string, string) bool            { return true }
func (errLoader) GetLoadedKeys() []string                       { return nil }
func (errLoader) FindIndexByTimestamp(context.Context, string, string, string, int64) (int, int64, error) {
	return 0, 0, errors.New("nope")
}
func (errLoader) Close() error { return nil }

// TestBuildPositionsRedactsCacheKeyOnError covers the PR #29 review note: the
// buildPositions error path logs the cache key, whose final segment is the API
// key. Because logs persist in Loki, it must be masked, not emitted raw.
func TestBuildPositionsRedactsCacheKeyOnError(t *testing.T) {
	const apiKey = "supersecretapikey"
	cache := data.NewIndexCache(data.CacheModeExhaust)
	cache.SetIndex("SPX/classic/gex_full/"+apiKey, 0)

	core, logs := observer.New(zap.DebugLevel)
	sb := NewSyncBroadcaster(cache, errLoader{}, &config.ServerConfig{}, zap.New(core))

	_ = sb.buildPositions(context.Background(), apiKey)

	entries := logs.FilterMessage("failed to get data length").All()
	if len(entries) == 0 {
		t.Fatal("expected the error-path log entry, got none")
	}
	sawCacheKey := false
	for _, e := range entries {
		for _, f := range e.Context {
			if f.Key != "cache_key" {
				continue
			}
			sawCacheKey = true
			if strings.Contains(f.String, apiKey) {
				t.Errorf("cache_key log leaks the API key: %q", f.String)
			}
			if !strings.Contains(f.String, "[REDACTED]") {
				t.Errorf("cache_key was not redacted: %q", f.String)
			}
		}
	}
	if !sawCacheKey {
		t.Fatal("error-path log had no cache_key field to check")
	}
}
