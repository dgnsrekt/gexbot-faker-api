package ws

import (
	"context"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/data"
)

// ClassicStreamer broadcasts classic GEX data from JSONL files to subscribed clients.
// Supports gex_full, gex_zero, and gex_one categories.
type ClassicStreamer struct {
	hub      *Hub
	loader   data.DataLoader
	encoder  *Encoder
	interval time.Duration
	indexes  map[string]int // ticker:category -> current index
	mu       sync.RWMutex
	logger   *zap.Logger
}

// NewClassicStreamer creates a new ClassicStreamer.
func NewClassicStreamer(hub *Hub, loader data.DataLoader, interval time.Duration, logger *zap.Logger) (*ClassicStreamer, error) {
	enc, err := NewEncoder()
	if err != nil {
		return nil, err
	}

	return &ClassicStreamer{
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
func (s *ClassicStreamer) Run(ctx context.Context) {
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
func (s *ClassicStreamer) broadcastNext(ctx context.Context) {
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

		// Create composite key for index tracking
		indexKey := ticker + ":" + category

		// Get current index for this ticker:category
		s.mu.Lock()
		idx := s.indexes[indexKey]
		s.mu.Unlock()

		// Get data length
		length, err := s.loader.GetLength(ticker, "classic", category)
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

		// Broadcast to all clients (each client formats per its protocol)
		s.hub.BroadcastData(group, encoded, "proto.gex")

		// Advance index
		s.mu.Lock()
		s.indexes[indexKey] = idx + 1
		s.mu.Unlock()

		s.logger.Debug("broadcast classic gex",
			zap.String("ticker", ticker),
			zap.String("category", category),
			zap.Int("index", idx),
			zap.Int("encodedSize", len(encoded)),
		)
	}
}

// extractClassicTickerAndCategory extracts the ticker and category from a classic group name.
// Group format: blue_{ticker}_classic_{category}
// Examples:
//   - blue_SPX_classic_gex_full -> ticker="SPX", category="gex_full"
//   - blue_ES_SPX_classic_gex_zero -> ticker="ES_SPX", category="gex_zero"
func extractClassicTickerAndCategory(group string) (ticker, category string) {
	if !strings.HasPrefix(group, "blue_") {
		return "", ""
	}
	trimmed := strings.TrimPrefix(group, "blue_")

	// Find _classic_ separator to isolate ticker and category
	separator := "_classic_"
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
