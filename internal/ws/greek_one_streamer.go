package ws

import (
	"time"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/data"
)

// GreekOneStreamer broadcasts Greek profile data from JSONL files to subscribed clients.
// Supports delta_one, gamma_one, vanna_one, and charm_one categories.
// Uses per-API-key position tracking via shared IndexCache.
// This is a thin wrapper around BaseStreamer with state_greeks_one-specific configuration.
type GreekOneStreamer struct {
	*BaseStreamer
}

// greekOneCategories are the valid categories for state_greeks_one streaming.
var greekOneCategories = []string{"delta_one", "gamma_one", "vanna_one", "charm_one"}

// NewGreekOneStreamer creates a new GreekOneStreamer with shared cache for per-API-key tracking.
func NewGreekOneStreamer(hub *Hub, loader data.DataLoader, cache *data.IndexCache, interval time.Duration, logger *zap.Logger, reloadChecker ReloadChecker) (*GreekOneStreamer, error) {
	base, err := NewBaseStreamer(
		StreamerConfig{
			Name:           "greek one",
			GroupParser:    MakeGroupParser("_state_", greekOneCategories),
			Package:        "state",
			CacheKeyPrefix: "state_greeks_one",
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

	return &GreekOneStreamer{BaseStreamer: base}, nil
}
