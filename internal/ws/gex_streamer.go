package ws

import (
	"context"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/data"
)

// GexStreamer broadcasts GEX data from JSONL files to subscribed clients.
// Supports gex_full, gex_zero, and gex_one categories.
type GexStreamer struct {
	hub      *Hub
	loader   data.DataLoader
	encoder  *Encoder
	interval time.Duration
	indexes  map[string]int // ticker:category -> current index
	mu       sync.RWMutex
	logger   *zap.Logger
}

// NewGexStreamer creates a new GexStreamer.
func NewGexStreamer(hub *Hub, loader data.DataLoader, interval time.Duration, logger *zap.Logger) (*GexStreamer, error) {
	enc, err := NewEncoder()
	if err != nil {
		return nil, err
	}

	return &GexStreamer{
		hub:      hub,
		loader:   loader,
		encoder:  enc,
		interval: interval,
		indexes:  make(map[string]int),
		logger:   logger,
	}, nil
}

// Run starts the streaming loop. Call in a goroutine.
// Returns when context is cancelled.
func (s *GexStreamer) Run(ctx context.Context) {
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

		// Create composite key for index tracking
		indexKey := ticker + ":" + category

		// Get current index for this ticker:category
		s.mu.Lock()
		idx := s.indexes[indexKey]
		s.mu.Unlock()

		// Get data length
		length, err := s.loader.GetLength(ticker, "state", category)
		if err != nil {
			s.logger.Debug("failed to get data length",
				zap.String("ticker", ticker),
				zap.String("category", category),
				zap.Error(err),
			)
			continue
		}

		// Wrap around for continuous playback
		if idx >= length {
			idx = 0
		}

		// Get raw JSON data at index
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

		// Broadcast to all clients (JSON clients get raw JSON, protobuf clients get encoded)
		s.hub.BroadcastDataDual(group, encoded, rawJSON, "proto.gex")

		// Advance index
		s.mu.Lock()
		s.indexes[indexKey] = idx + 1
		s.mu.Unlock()

		s.logger.Debug("broadcast gex",
			zap.String("ticker", ticker),
			zap.String("category", category),
			zap.Int("index", idx),
			zap.Int("encodedSize", len(encoded)),
		)
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
