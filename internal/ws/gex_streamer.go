package ws

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/data"
)

// GexStreamer broadcasts GEX data from JSONL files to subscribed clients.
// Supports gex_full, gex_zero, and gex_one categories.
// Uses per-API-key position tracking via shared IndexCache.
type GexStreamer struct {
	hub      *Hub
	loader   data.DataLoader
	cache    *data.IndexCache
	encoder  *Encoder
	interval time.Duration
	logger   *zap.Logger
}

// NewGexStreamer creates a new GexStreamer with shared cache for per-API-key tracking.
func NewGexStreamer(hub *Hub, loader data.DataLoader, cache *data.IndexCache, interval time.Duration, logger *zap.Logger) (*GexStreamer, error) {
	enc, err := NewEncoder()
	if err != nil {
		return nil, err
	}

	return &GexStreamer{
		hub:      hub,
		loader:   loader,
		cache:    cache,
		encoder:  enc,
		interval: interval,
		logger:   logger,
	}, nil
}

// Run starts the streaming loop. Call in a goroutine.
// Returns when context is cancelled.
func (s *GexStreamer) Run(ctx context.Context) {
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
		s.logger.Info("gex streamer cancelled during alignment")
		s.encoder.Close()
		return
	case <-time.After(time.Until(nextSecond)):
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.logger.Info("gex streamer started",
		zap.Duration("interval", s.interval),
	)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("gex streamer stopping")
			s.encoder.Close()
			return

		case <-ticker.C:
			s.broadcastNext(ctx)
		}
	}
}

// broadcastNext sends the next data point to all active groups.
// Each API key receives data from its own position in the stream.
func (s *GexStreamer) broadcastNext(ctx context.Context) {
	groups := s.hub.GetActiveGroups()
	if len(groups) == 0 {
		return
	}

	for _, group := range groups {
		// Parse group name: blue_{ticker}_state_{category}
		ticker, category := extractGexTickerAndCategory(group)
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
			cacheKey := data.WSCacheKey("state_gex", ticker, category, apiKey)
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
			encoded, err := s.encoder.EncodeGex(rawJSON)
			if err != nil {
				s.logger.Debug("failed to encode gex",
					zap.String("ticker", ticker),
					zap.String("category", category),
					zap.Error(err),
				)
				continue
			}

			// Broadcast to all clients with this API key
			s.hub.BroadcastToClients(clients, group, encoded, rawJSON, "proto.gex")

			s.logger.Debug("broadcast gex",
				zap.String("ticker", ticker),
				zap.String("category", category),
				zap.String("apiKey", maskAPIKey(apiKey)),
				zap.Int("index", idx),
				zap.Int("clientCount", len(clients)),
			)
		}
	}
}

// extractGexTickerAndCategory extracts the ticker and category from a state_gex group name.
// Group format: blue_{ticker}_state_{category}
// Examples:
//   - blue_SPX_state_gex_full -> ticker="SPX", category="gex_full"
//   - blue_ES_SPX_state_gex_zero -> ticker="ES_SPX", category="gex_zero"
func extractGexTickerAndCategory(group string) (ticker, category string) {
	if !strings.HasPrefix(group, "blue_") {
		return "", ""
	}
	trimmed := strings.TrimPrefix(group, "blue_")

	// Find _state_ separator to isolate ticker and category
	separator := "_state_"
	idx := strings.Index(trimmed, separator)
	if idx < 0 {
		return "", ""
	}

	ticker = trimmed[:idx]
	category = trimmed[idx+len(separator):]

	// Validate category is one of the expected GEX categories
	switch category {
	case "gex_full", "gex_zero", "gex_one":
		return ticker, category
	default:
		return "", ""
	}
}
