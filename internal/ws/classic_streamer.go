package ws

import (
	"time"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/data"
)

// ClassicStreamer broadcasts classic GEX data from JSONL files to subscribed clients.
// Supports gex_full, gex_zero, and gex_one categories.
// Uses per-API-key position tracking via shared IndexCache.
// This is a thin wrapper around BaseStreamer with classic-specific configuration.
type ClassicStreamer struct {
	*BaseStreamer
}

// classicCategories are the valid categories for classic GEX streaming.
var classicCategories = []string{"gex_full", "gex_zero", "gex_one"}

// NewClassicStreamer creates a new ClassicStreamer with shared cache for per-API-key tracking.
func NewClassicStreamer(hub *Hub, loader data.DataLoader, cache *data.IndexCache, interval time.Duration, logger *zap.Logger, reloadChecker ReloadChecker) (*ClassicStreamer, error) {
	base, err := NewBaseStreamer(
		StreamerConfig{
			Name:           "classic",
			GroupParser:    MakeGroupParser("_classic_", classicCategories),
			Package:        "classic",
			CacheKeyPrefix: "classic",
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

	return &ClassicStreamer{BaseStreamer: base}, nil
}
