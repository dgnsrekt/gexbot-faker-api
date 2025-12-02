package ws

import (
	"context"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/data"
)

// Streamer broadcasts data from JSONL files to subscribed clients.
type Streamer struct {
	hub      *Hub
	loader   data.DataLoader
	encoder  *Encoder
	interval time.Duration
	indexes  map[string]int // ticker -> current index
	mu       sync.RWMutex
	logger   *zap.Logger
}

// NewStreamer creates a new Streamer.
func NewStreamer(hub *Hub, loader data.DataLoader, interval time.Duration, logger *zap.Logger) (*Streamer, error) {
	enc, err := NewEncoder()
	if err != nil {
		return nil, err
	}

	return &Streamer{
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
func (s *Streamer) Run(ctx context.Context) {
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
		s.logger.Info("streamer cancelled during alignment")
		s.encoder.Close()
		return
	case <-time.After(time.Until(nextSecond)):
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.logger.Info("streamer started",
		zap.Duration("interval", s.interval),
	)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("streamer stopping")
			s.encoder.Close()
			return

		case <-ticker.C:
			s.broadcastNext(ctx)
		}
	}
}

// broadcastNext sends the next data point to all active groups.
func (s *Streamer) broadcastNext(ctx context.Context) {
	groups := s.hub.GetActiveGroups()
	if len(groups) == 0 {
		return
	}

	for _, group := range groups {
		// Parse group name: blue_{ticker}_orderflow_orderflow
		ticker := extractTicker(group)
		if ticker == "" {
			continue
		}

		// Get current index for this ticker
		s.mu.Lock()
		idx := s.indexes[ticker]
		s.mu.Unlock()

		// Get data length
		length, err := s.loader.GetLength(ticker, "orderflow", "orderflow")
		if err != nil {
			s.logger.Debug("failed to get data length",
				zap.String("ticker", ticker),
				zap.Error(err),
			)
			continue
		}

		// Wrap around for continuous playback
		if idx >= length {
			idx = 0
		}

		// Get raw JSON data at index
		rawJSON, err := s.loader.GetRawAtIndex(ctx, ticker, "orderflow", "orderflow", idx)
		if err != nil {
			s.logger.Debug("failed to get data at index",
				zap.String("ticker", ticker),
				zap.Int("index", idx),
				zap.Error(err),
			)
			continue
		}

		// Encode to protobuf + zstd
		encoded, err := s.encoder.EncodeOrderflow(rawJSON)
		if err != nil {
			s.logger.Debug("failed to encode orderflow",
				zap.String("ticker", ticker),
				zap.Error(err),
			)
			continue
		}

		// Broadcast to all clients (JSON clients get raw JSON, protobuf clients get encoded)
		s.hub.BroadcastDataDual(group, encoded, rawJSON, "proto.orderflow")

		// Advance index
		s.mu.Lock()
		s.indexes[ticker] = idx + 1
		s.mu.Unlock()

		s.logger.Debug("broadcast orderflow",
			zap.String("ticker", ticker),
			zap.Int("index", idx),
			zap.Int("encodedSize", len(encoded)),
		)
	}
}

// extractTicker extracts the ticker from an orderflow group name.
// Group format: blue_{ticker}_orderflow_orderflow
func extractTicker(group string) string {
	if !strings.HasPrefix(group, "blue_") {
		return ""
	}
	trimmed := strings.TrimPrefix(group, "blue_")

	// Find _orderflow_orderflow suffix
	suffix := "_orderflow_orderflow"
	idx := strings.Index(trimmed, suffix)
	if idx < 0 {
		return ""
	}

	return trimmed[:idx]
}
