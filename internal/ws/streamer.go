package ws

import (
	"time"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/data"
)

// ReloadChecker provides a way to check if a data reload is in progress.
// Streamers should skip broadcasts during reload to prevent data inconsistencies.
type ReloadChecker interface {
	IsReloading() bool
}

// Streamer broadcasts orderflow data from JSONL files to subscribed clients.
// Uses per-API-key position tracking via shared IndexCache.
// This is a thin wrapper around BaseStreamer with orderflow-specific configuration.
type Streamer struct {
	*BaseStreamer
}

// NewStreamer creates a new Streamer with shared cache for per-API-key tracking.
func NewStreamer(hub *Hub, loader data.DataLoader, cache *data.IndexCache, interval time.Duration, logger *zap.Logger, reloadChecker ReloadChecker) (*Streamer, error) {
	base, err := NewBaseStreamer(
		StreamerConfig{
			Name:           "orderflow",
			GroupParser:    MakeOrderflowGroupParser(),
			Package:        "orderflow",
			CacheKeyPrefix: "orderflow",
			ProtoTypeURL:   "proto.orderflow",
		},
		hub,
		loader,
		cache,
		interval,
		logger,
		reloadChecker,
	)
	if err != nil {
		return nil, err
	}

	// Set the encode method after creation (needs encoder instance)
	base.config.EncodeMethod = base.encoder.EncodeOrderflow

	return &Streamer{BaseStreamer: base}, nil
}
