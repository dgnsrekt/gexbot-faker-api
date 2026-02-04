package ws

import (
	"time"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/data"
)

// GexStreamer broadcasts GEX data from JSONL files to subscribed clients.
// Supports gex_full, gex_zero, and gex_one categories.
// Uses per-API-key position tracking via shared IndexCache.
// This is a thin wrapper around BaseStreamer with state_gex-specific configuration.
type GexStreamer struct {
	*BaseStreamer
}

// gexCategories are the valid categories for state_gex streaming.
var gexCategories = []string{"gex_full", "gex_zero", "gex_one"}

// NewGexStreamer creates a new GexStreamer with shared cache for per-API-key tracking.
func NewGexStreamer(hub *Hub, loader data.DataLoader, cache *data.IndexCache, interval time.Duration, logger *zap.Logger, reloadChecker ReloadChecker) (*GexStreamer, error) {
	base, err := NewBaseStreamer(
		StreamerConfig{
			Name:           "gex",
			GroupParser:    MakeGroupParser("_state_", gexCategories),
			Package:        "state",
			CacheKeyPrefix: "state_gex",
			ProtoTypeURL:   "proto.gex",
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
	base.config.EncodeMethod = base.encoder.EncodeGex

	return &GexStreamer{BaseStreamer: base}, nil
}
