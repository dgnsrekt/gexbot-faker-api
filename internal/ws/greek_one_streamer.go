package ws

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/data"
)

// GreekOneStreamer broadcasts Greek profile data from JSONL files to subscribed clients.
// Supports delta_one, gamma_one, vanna_one, and charm_one categories.
// Uses per-API-key position tracking via shared IndexCache.
type GreekOneStreamer struct {
	hub           *Hub
	loader        data.DataLoader
	cache         *data.IndexCache
	encoder       *Encoder
	interval      time.Duration
	logger        *zap.Logger
	reloadChecker ReloadChecker
}

// NewGreekOneStreamer creates a new GreekOneStreamer with shared cache for per-API-key tracking.
func NewGreekOneStreamer(hub *Hub, loader data.DataLoader, cache *data.IndexCache, interval time.Duration, logger *zap.Logger, reloadChecker ReloadChecker) (*GreekOneStreamer, error) {
	enc, err := NewEncoder()
	if err != nil {
		return nil, err
	}

	return &GreekOneStreamer{
		hub:           hub,
		loader:        loader,
		cache:         cache,
		encoder:       enc,
		interval:      interval,
		logger:        logger,
		reloadChecker: reloadChecker,
	}, nil
}

// Run starts the streaming loop. Call in a goroutine.
// Returns when context is cancelled.
func (s *GreekOneStreamer) Run(ctx context.Context) {
	// Align first tick to top of second for predictable timing
	now := time.Now()
	nextSecond := now.Truncate(time.Second).Add(time.Second)
	s.logger.Debug("aligning to next second",
		zap.Time("now", now),
		zap.Time("nextSecond", nextSecond),
		zap.Duration("wait", time.Until(nextSecond)),
	)

	select {
	case <-ctx.Done():
		s.logger.Info("greek one streamer cancelled during alignment")
		s.encoder.Close()
		return
	case <-time.After(time.Until(nextSecond)):
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.logger.Info("greek one streamer started",
		zap.Duration("interval", s.interval),
	)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("greek one streamer stopping")
			s.encoder.Close()
			return

		case <-ticker.C:
			s.broadcastNext(ctx)
		}
	}
}

// broadcastNext sends the next data point to all active groups.
// Each API key receives data from its own position in the stream.
func (s *GreekOneStreamer) broadcastNext(ctx context.Context) {
	// Skip broadcast during data reload
	if s.reloadChecker != nil && s.reloadChecker.IsReloading() {
		return
	}

	groups := s.hub.GetActiveGroups()
	if len(groups) == 0 {
		return
	}

	for _, group := range groups {
		// Parse group name: blue_{ticker}_state_{category}
		ticker, category := extractGreekOneTickerAndCategory(group)
		if ticker == "" || category == "" {
			continue
		}

		// Get data length once for this ticker:category
		length, err := s.loader.GetLength(ticker, "state", category)
		if err != nil {
			s.logger.Debug("failed to get data length",
				zap.String("ticker", ticker),
				zap.String("category", category),
				zap.Error(err),
			)
			continue
		}

		// Get clients grouped by API key
		clientsByAPIKey := s.hub.GetClientsByAPIKey(group)
		if len(clientsByAPIKey) == 0 {
			continue
		}

		// For each API key, get their position and broadcast their data
		for apiKey, clients := range clientsByAPIKey {
			cacheKey := data.WSCacheKey("state_greeks_one", ticker, category, apiKey)
			idx, exhausted := s.cache.GetAndAdvance(cacheKey, length)

			// In exhaust mode, skip this API key if exhausted
			if exhausted {
				s.logger.Debug("data exhausted for API key",
					zap.String("ticker", ticker),
					zap.String("category", category),
					zap.String("apiKey", maskAPIKey(apiKey)),
				)
				continue
			}

			// Get raw JSON data at this API key's index
			rawJSON, err := s.loader.GetRawAtIndex(ctx, ticker, "state", category, idx)
			if err != nil {
				s.logger.Debug("failed to get data at index",
					zap.String("ticker", ticker),
					zap.String("category", category),
					zap.Int("index", idx),
					zap.Error(err),
				)
				continue
			}

			// Encode to protobuf + zstd
			encoded, err := s.encoder.EncodeGreek(rawJSON)
			if err != nil {
				s.logger.Debug("failed to encode greek",
					zap.String("ticker", ticker),
					zap.String("category", category),
					zap.Error(err),
				)
				continue
			}

			// Broadcast to all clients with this API key
			s.hub.BroadcastToClients(clients, group, encoded, rawJSON, "proto.greek")

			s.logger.Debug("broadcast greek one",
				zap.String("ticker", ticker),
				zap.String("category", category),
				zap.String("apiKey", maskAPIKey(apiKey)),
				zap.Int("index", idx),
				zap.Int("clientCount", len(clients)),
			)
		}
	}
}

// extractGreekOneTickerAndCategory extracts the ticker and category from a state_greeks_one group name.
// Group format: {prefix}_{ticker}_state_{category}
// Examples:
//   - blue_SPX_state_delta_one -> ticker="SPX", category="delta_one"
//   - blue_ES_SPX_state_gamma_one -> ticker="ES_SPX", category="gamma_one"
func extractGreekOneTickerAndCategory(group string) (ticker, category string) {
	// Find _state_ separator to isolate prefix_ticker and category
	separator := "_state_"
	separatorIdx := strings.Index(group, separator)
	if separatorIdx < 0 {
		return "", ""
	}

	// Everything before _state_ is prefix_ticker
	prefixAndTicker := group[:separatorIdx]

	// Find first underscore to separate prefix from ticker
	firstUnderscore := strings.Index(prefixAndTicker, "_")
	if firstUnderscore < 0 || firstUnderscore >= len(prefixAndTicker)-1 {
		return "", ""
	}

	ticker = prefixAndTicker[firstUnderscore+1:]
	category = group[separatorIdx+len(separator):]

	// Validate category is one of the expected Greek one categories
	switch category {
	case "delta_one", "gamma_one", "vanna_one", "charm_one":
		return ticker, category
	default:
		return "", ""
	}
}
