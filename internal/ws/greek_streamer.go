package ws

import (
	"time"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/data"
)

// GreekStreamer broadcasts Greek profile data from JSONL files to subscribed clients.
// Supports delta_zero, gamma_zero, vanna_zero, and charm_zero categories.
// Uses per-API-key position tracking via shared IndexCache.
// This is a thin wrapper around BaseStreamer with state_greeks_zero-specific configuration.
type GreekStreamer struct {
	*BaseStreamer
}

// greekZeroCategories are the valid categories for state_greeks_zero streaming.
var greekZeroCategories = []string{"delta_zero", "gamma_zero", "vanna_zero", "charm_zero"}

// NewGreekStreamer creates a new GreekStreamer with shared cache for per-API-key tracking.
func NewGreekStreamer(hub *Hub, loader data.DataLoader, cache *data.IndexCache, interval time.Duration, logger *zap.Logger, reloadChecker ReloadChecker) (*GreekStreamer, error) {
	base, err := NewBaseStreamer(
		StreamerConfig{
			Name:           "greek",
			GroupParser:    MakeGroupParser("_state_", greekZeroCategories),
			Package:        "state",
			CacheKeyPrefix: "state_greeks_zero",
			ProtoTypeURL:   "proto.greek",
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
	base.config.EncodeMethod = base.encoder.EncodeGreek

	return &GreekStreamer{BaseStreamer: base}, nil
}
