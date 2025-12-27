package ws

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/data"
)

// ClassicStreamer broadcasts classic GEX data from JSONL files to subscribed clients.
// Supports gex_full, gex_zero, and gex_one categories.
// Uses per-API-key position tracking via shared IndexCache.
type ClassicStreamer struct {
	hub           *Hub
	loader        data.DataLoader
	cache         *data.IndexCache
	encoder       *Encoder
	interval      time.Duration
	logger        *zap.Logger
	reloadChecker ReloadChecker
}

// NewClassicStreamer creates a new ClassicStreamer with shared cache for per-API-key tracking.
func NewClassicStreamer(hub *Hub, loader data.DataLoader, cache *data.IndexCache, interval time.Duration, logger *zap.Logger, reloadChecker ReloadChecker) (*ClassicStreamer, error) {
	enc, err := NewEncoder()
	if err != nil {
		return nil, err
	}

	return &ClassicStreamer{
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
func (s *ClassicStreamer) Run(ctx context.Context) {
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
		s.logger.Info("classic streamer cancelled during alignment")
		s.encoder.Close()
		return
	case <-time.After(time.Until(nextSecond)):
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.logger.Info("classic streamer started",
		zap.Duration("interval", s.interval),
	)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("classic streamer stopping")
			s.encoder.Close()
			return

		case <-ticker.C:
			s.broadcastNext(ctx)
		}
	}
}

// broadcastNext sends the next data point to all active groups.
// Each API key receives data from its own position in the stream.
func (s *ClassicStreamer) broadcastNext(ctx context.Context) {
	// Skip broadcast during data reload
	if s.reloadChecker != nil && s.reloadChecker.IsReloading() {
		return
	}

	groups := s.hub.GetActiveGroups()
	if len(groups) == 0 {
		return
	}

	for _, group := range groups {
		// Parse group name: blue_{ticker}_classic_{category}
		ticker, category := extractClassicTickerAndCategory(group)
		if ticker == "" || category == "" {
			continue
		}

		// Get data length once for this ticker:category
		length, err := s.loader.GetLength(ticker, "classic", category)
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
			cacheKey := data.WSCacheKey("classic", ticker, category, apiKey)
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
			rawJSON, err := s.loader.GetRawAtIndex(ctx, ticker, "classic", category, idx)
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

			s.logger.Debug("broadcast classic gex",
				zap.String("ticker", ticker),
				zap.String("category", category),
				zap.String("apiKey", maskAPIKey(apiKey)),
				zap.Int("index", idx),
				zap.Int("clientCount", len(clients)),
			)
		}
	}
}

// extractClassicTickerAndCategory extracts the ticker and category from a classic group name.
// Group format: {prefix}_{ticker}_classic_{category}
// Examples:
//   - blue_SPX_classic_gex_full -> ticker="SPX", category="gex_full"
//   - blue_ES_SPX_classic_gex_zero -> ticker="ES_SPX", category="gex_zero"
func extractClassicTickerAndCategory(group string) (ticker, category string) {
	// Find _classic_ separator to isolate prefix_ticker and category
	separator := "_classic_"
	separatorIdx := strings.Index(group, separator)
	if separatorIdx < 0 {
		return "", ""
	}

	// Everything before _classic_ is prefix_ticker
	prefixAndTicker := group[:separatorIdx]

	// Find first underscore to separate prefix from ticker
	firstUnderscore := strings.Index(prefixAndTicker, "_")
	if firstUnderscore < 0 || firstUnderscore >= len(prefixAndTicker)-1 {
		return "", ""
	}

	ticker = prefixAndTicker[firstUnderscore+1:]
	category = group[separatorIdx+len(separator):]

	// Validate category is one of the expected GEX categories
	switch category {
	case "gex_full", "gex_zero", "gex_one":
		return ticker, category
	default:
		return "", ""
	}
}
