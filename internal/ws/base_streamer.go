package ws

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/data"
)

// EncodeFunc is a function that encodes raw JSON data to compressed protobuf.
type EncodeFunc func(rawJSON []byte) ([]byte, error)

// GroupParseResult contains the parsed ticker and category from a group name.
type GroupParseResult struct {
	Ticker   string
	Category string
}

// GroupParserFunc parses a group name and returns the ticker and category.
// Returns empty strings if the group doesn't match this streamer's format.
type GroupParserFunc func(group string) GroupParseResult

// StreamerConfig contains the configuration for a BaseStreamer instance.
type StreamerConfig struct {
	// Name is used for logging (e.g., "orderflow", "gex", "classic")
	Name string

	// GroupParser extracts ticker and category from group names
	GroupParser GroupParserFunc

	// Package is the data package name (e.g., "orderflow", "state", "classic")
	Package string

	// CacheKeyPrefix is used for WebSocket cache keys (e.g., "orderflow", "state_gex")
	CacheKeyPrefix string

	// EncodeMethod is the encoder function to use
	EncodeMethod EncodeFunc

	// ProtoTypeURL is the protobuf type URL (e.g., "proto.orderflow", "proto.gex")
	ProtoTypeURL string
}

// BaseStreamer is a parameterized streamer that handles the common broadcast loop.
type BaseStreamer struct {
	config        StreamerConfig
	hub           *Hub
	loader        data.DataLoader
	cache         *data.IndexCache
	encoder       *Encoder
	interval      time.Duration
	logger        *zap.Logger
	reloadChecker ReloadChecker
}

// NewBaseStreamer creates a new BaseStreamer with the given configuration.
func NewBaseStreamer(
	config StreamerConfig,
	hub *Hub,
	loader data.DataLoader,
	cache *data.IndexCache,
	interval time.Duration,
	logger *zap.Logger,
	reloadChecker ReloadChecker,
) (*BaseStreamer, error) {
	enc, err := NewEncoder()
	if err != nil {
		return nil, err
	}

	return &BaseStreamer{
		config:        config,
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
func (s *BaseStreamer) Run(ctx context.Context) {
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
		s.logger.Info(s.config.Name+" streamer cancelled during alignment")
		s.encoder.Close()
		return
	case <-time.After(time.Until(nextSecond)):
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.logger.Info(s.config.Name+" streamer started",
		zap.Duration("interval", s.interval),
	)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info(s.config.Name + " streamer stopping")
			s.encoder.Close()
			return

		case <-ticker.C:
			s.broadcastNext(ctx)
		}
	}
}

// broadcastNext sends the next data point to all active groups.
// Each API key receives data from its own position in the stream.
func (s *BaseStreamer) broadcastNext(ctx context.Context) {
	// Skip broadcast during data reload
	if s.reloadChecker != nil && s.reloadChecker.IsReloading() {
		return
	}

	groups := s.hub.GetActiveGroups()
	if len(groups) == 0 {
		return
	}

	for _, group := range groups {
		// Parse group name using the configured parser
		result := s.config.GroupParser(group)
		if result.Ticker == "" {
			continue
		}

		// Get data length once for this ticker:category
		length, err := s.loader.GetLength(result.Ticker, s.config.Package, result.Category)
		if err != nil {
			s.logger.Debug("failed to get data length",
				zap.String("ticker", result.Ticker),
				zap.String("category", result.Category),
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
			cacheKey := data.WSCacheKey(s.config.CacheKeyPrefix, result.Ticker, result.Category, apiKey)
			idx, exhausted := s.cache.GetAndAdvance(cacheKey, length)

			// In exhaust mode, skip this API key if exhausted
			if exhausted {
				s.logger.Debug("data exhausted for API key",
					zap.String("ticker", result.Ticker),
					zap.String("category", result.Category),
					zap.String("apiKey", maskAPIKey(apiKey)),
				)
				continue
			}

			// Get raw JSON data at this API key's index
			rawJSON, err := s.loader.GetRawAtIndex(ctx, result.Ticker, s.config.Package, result.Category, idx)
			if err != nil {
				s.logger.Debug("failed to get data at index",
					zap.String("ticker", result.Ticker),
					zap.String("category", result.Category),
					zap.Int("index", idx),
					zap.Error(err),
				)
				continue
			}

			// Encode to protobuf + zstd using the configured encoder
			encoded, err := s.config.EncodeMethod(rawJSON)
			if err != nil {
				s.logger.Debug("failed to encode "+s.config.Name,
					zap.String("ticker", result.Ticker),
					zap.String("category", result.Category),
					zap.Error(err),
				)
				continue
			}

			// Broadcast to all clients with this API key
			s.hub.BroadcastToClients(clients, group, encoded, rawJSON, s.config.ProtoTypeURL)

			s.logger.Debug("broadcast "+s.config.Name,
				zap.String("ticker", result.Ticker),
				zap.String("category", result.Category),
				zap.String("apiKey", maskAPIKey(apiKey)),
				zap.Int("index", idx),
				zap.Int("clientCount", len(clients)),
			)
		}
	}
}

// ExtractTickerAndCategory is a generic helper for parsing group names.
// Group format: {prefix}_{ticker}_{separator}_{category}
// Returns empty strings if parsing fails or category is not in validCategories.
func ExtractTickerAndCategory(group, separator string, validCategories []string) (ticker, category string) {
	// Find separator to isolate prefix_ticker and category
	separatorIdx := strings.Index(group, separator)
	if separatorIdx < 0 {
		return "", ""
	}

	// Everything before separator is prefix_ticker
	prefixAndTicker := group[:separatorIdx]

	// Find first underscore to separate prefix from ticker
	firstUnderscore := strings.Index(prefixAndTicker, "_")
	if firstUnderscore < 0 || firstUnderscore >= len(prefixAndTicker)-1 {
		return "", ""
	}

	ticker = prefixAndTicker[firstUnderscore+1:]
	category = group[separatorIdx+len(separator):]

	// Validate category if validCategories provided
	if len(validCategories) > 0 {
		for _, valid := range validCategories {
			if category == valid {
				return ticker, category
			}
		}
		return "", ""
	}

	return ticker, category
}

// MakeGroupParser creates a GroupParserFunc for the given separator and valid categories.
func MakeGroupParser(separator string, validCategories []string) GroupParserFunc {
	return func(group string) GroupParseResult {
		ticker, category := ExtractTickerAndCategory(group, separator, validCategories)
		return GroupParseResult{Ticker: ticker, Category: category}
	}
}

// MakeOrderflowGroupParser creates a GroupParserFunc for orderflow groups.
// Orderflow groups have format: {prefix}_{ticker}_orderflow_orderflow
func MakeOrderflowGroupParser() GroupParserFunc {
	return func(group string) GroupParseResult {
		// Find _orderflow_orderflow suffix
		suffix := "_orderflow_orderflow"
		suffixIdx := strings.Index(group, suffix)
		if suffixIdx < 0 {
			return GroupParseResult{}
		}

		// Everything before suffix is prefix_ticker
		prefixAndTicker := group[:suffixIdx]

		// Find first underscore to separate prefix from ticker
		firstUnderscore := strings.Index(prefixAndTicker, "_")
		if firstUnderscore < 0 || firstUnderscore >= len(prefixAndTicker)-1 {
			return GroupParseResult{}
		}

		return GroupParseResult{
			Ticker:   prefixAndTicker[firstUnderscore+1:],
			Category: "orderflow",
		}
	}
}
