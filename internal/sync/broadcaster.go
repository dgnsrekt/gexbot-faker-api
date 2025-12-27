package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	gosync "sync"
	"time"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/config"
	"github.com/dgnsrekt/gexbot-downloader/internal/data"
)

// timestampExtractor is a minimal struct for extracting just the timestamp from raw JSON.
type timestampExtractor struct {
	Timestamp int64 `json:"timestamp"`
}

// SyncBroadcaster broadcasts position updates to connected SSE clients.
type SyncBroadcaster struct {
	broadcasterID string
	cache         *data.IndexCache
	loader        data.DataLoader
	config        *config.ServerConfig
	logger        *zap.Logger

	mu       gosync.RWMutex
	sequence uint64
	clients  map[*sseClient]bool

	interval time.Duration
}

// sseClient represents a connected SSE subscriber.
type sseClient struct {
	apiKey   string
	dataCh   chan []byte
	doneCh   chan struct{}
	flusher  http.Flusher
	writer   http.ResponseWriter
}

// NewSyncBroadcaster creates a new sync broadcaster.
func NewSyncBroadcaster(
	cache *data.IndexCache,
	loader data.DataLoader,
	cfg *config.ServerConfig,
	logger *zap.Logger,
) *SyncBroadcaster {
	return &SyncBroadcaster{
		broadcasterID: cfg.SyncBroadcastSystemID,
		cache:         cache,
		loader:        loader,
		config:        cfg,
		logger:        logger,
		clients:       make(map[*sseClient]bool),
		interval:      cfg.SyncBroadcastSystemInterval,
	}
}

// Run starts the periodic broadcast loop.
func (sb *SyncBroadcaster) Run(ctx context.Context) {
	sb.logger.Info("sync broadcaster starting",
		zap.String("broadcaster_id", sb.broadcasterID),
		zap.Duration("interval", sb.interval),
	)

	ticker := time.NewTicker(sb.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			sb.logger.Info("sync broadcaster stopping")
			return
		case <-ticker.C:
			sb.broadcastToAll(ctx)
		}
	}
}

// HandleSSE handles the SSE endpoint for subscribers.
func (sb *SyncBroadcaster) HandleSSE(w http.ResponseWriter, r *http.Request) {
	// Get API key from query parameter
	apiKey := r.URL.Query().Get("key")
	if apiKey == "" {
		http.Error(w, "missing required 'key' query parameter", http.StatusBadRequest)
		return
	}

	// Check if SSE is supported
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create client
	client := &sseClient{
		apiKey:  apiKey,
		dataCh:  make(chan []byte, 10),
		doneCh:  make(chan struct{}),
		flusher: flusher,
		writer:  w,
	}

	// Register client
	sb.addClient(client)
	defer sb.removeClient(client)

	sb.logger.Info("sync client connected",
		zap.String("api_key", apiKey),
		zap.String("remote_addr", r.RemoteAddr),
	)

	// Send initial snapshot
	snapshot := sb.buildSnapshot(r.Context(), apiKey)
	if err := sb.sendEvent(client, "snapshot", snapshot); err != nil {
		sb.logger.Error("failed to send snapshot", zap.Error(err))
		return
	}

	// Stream events
	for {
		select {
		case <-r.Context().Done():
			sb.logger.Info("sync client disconnected",
				zap.String("api_key", apiKey),
			)
			return
		case <-client.doneCh:
			return
		case eventData := <-client.dataCh:
			if _, err := client.writer.Write(eventData); err != nil {
				sb.logger.Debug("failed to write to client", zap.Error(err))
				return
			}
			client.flusher.Flush()
		}
	}
}

func (sb *SyncBroadcaster) addClient(client *sseClient) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	sb.clients[client] = true
}

func (sb *SyncBroadcaster) removeClient(client *sseClient) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	delete(sb.clients, client)
	close(client.doneCh)
}

func (sb *SyncBroadcaster) buildSnapshot(ctx context.Context, apiKey string) *SyncSnapshot {
	positions := sb.buildPositions(ctx, apiKey)

	sb.mu.Lock()
	sb.sequence++
	seq := sb.sequence
	sb.mu.Unlock()

	return &SyncSnapshot{
		BroadcasterID: sb.broadcasterID,
		DataDate:      sb.config.DataDate,
		CacheMode:     sb.config.CacheMode,
		Timestamp:     time.Now().UnixMilli(),
		Sequence:      seq,
		Positions:     positions,
	}
}

func (sb *SyncBroadcaster) buildBatch(ctx context.Context, apiKey string) *SyncBatch {
	positions := sb.buildPositions(ctx, apiKey)

	sb.mu.Lock()
	sb.sequence++
	seq := sb.sequence
	sb.mu.Unlock()

	return &SyncBatch{
		BroadcasterID: sb.broadcasterID,
		DataDate:      sb.config.DataDate,
		CacheMode:     sb.config.CacheMode,
		Timestamp:     time.Now().UnixMilli(),
		Sequence:      seq,
		Positions:     positions,
	}
}

func (sb *SyncBroadcaster) buildPositions(ctx context.Context, apiKey string) []SyncPosition {
	cachePositions := sb.cache.GetPositionsByAPIKey(apiKey)
	positions := make([]SyncPosition, 0, len(cachePositions))

	for cacheKey, index := range cachePositions {
		// Parse cache key to get data path
		ticker, pkg, category := sb.parseCacheKey(cacheKey)
		if ticker == "" {
			continue
		}

		// Get data length
		length, err := sb.loader.GetLength(ticker, pkg, category)
		if err != nil {
			sb.logger.Debug("failed to get data length",
				zap.String("cache_key", cacheKey),
				zap.Error(err),
			)
			continue
		}

		// Check if exhausted
		exhausted := false
		if sb.cache.GetMode() == data.CacheModeExhaust && index >= length {
			exhausted = true
		}

		// Get data timestamp at current position
		dataTimestamp := int64(0)
		if !exhausted && index < length {
			dataTimestamp = sb.getDataTimestamp(ctx, ticker, pkg, category, index)
		}

		positions = append(positions, SyncPosition{
			CacheKey:      maskCacheKey(cacheKey),
			Index:         index,
			DataLength:    length,
			DataTimestamp: dataTimestamp,
			Exhausted:     exhausted,
		})
	}

	return positions
}

// parseCacheKey extracts ticker, pkg, and category from a cache key.
// REST independent format: ticker/pkg/category/apiKey (e.g., SPX/classic/gex_full/api123)
// REST shared format: ticker/pkg/apiKey (e.g., SPX/classic/api123) - category defaults to pkg default
// WS format: ws/hub/ticker/category/apiKey (e.g., ws/orderflow/SPX/orderflow/api123)
func (sb *SyncBroadcaster) parseCacheKey(cacheKey string) (ticker, pkg, category string) {
	parts := strings.Split(cacheKey, "/")

	if len(parts) >= 5 && parts[0] == "ws" {
		// WebSocket format: ws/hub/ticker/category/apiKey
		// hub maps to pkg for data lookup
		hub := parts[1]
		ticker = parts[2]
		category = parts[3]
		// Map hub to pkg
		pkg = sb.hubToPkg(hub)
		return ticker, pkg, category
	}

	if len(parts) >= 4 {
		// REST independent format: ticker/pkg/category/apiKey
		ticker = parts[0]
		pkg = parts[1]
		category = parts[2]
		return ticker, pkg, category
	}

	if len(parts) == 3 {
		// REST shared format: ticker/pkg/apiKey
		// Use default category for the package
		ticker = parts[0]
		pkg = parts[1]
		category = sb.pkgDefaultCategory(pkg)
		return ticker, pkg, category
	}

	return "", "", ""
}

// pkgDefaultCategory returns the default category for a package in shared mode.
func (sb *SyncBroadcaster) pkgDefaultCategory(pkg string) string {
	switch pkg {
	case "classic":
		return "gex_full"
	case "state":
		return "gex_full"
	case "orderflow":
		return "orderflow"
	default:
		return ""
	}
}

// hubToPkg maps WebSocket hub names to data package names.
func (sb *SyncBroadcaster) hubToPkg(hub string) string {
	switch hub {
	case "orderflow":
		return "orderflow"
	case "classic":
		return "classic"
	case "state_gex", "state_greeks_zero", "state_greeks_one":
		return "state"
	default:
		return hub
	}
}

func (sb *SyncBroadcaster) getDataTimestamp(ctx context.Context, ticker, pkg, category string, index int) int64 {
	rawJSON, err := sb.loader.GetRawAtIndex(ctx, ticker, pkg, category, index)
	if err != nil {
		return 0
	}

	var extractor timestampExtractor
	if err := json.Unmarshal(rawJSON, &extractor); err != nil {
		return 0
	}

	return extractor.Timestamp
}

func (sb *SyncBroadcaster) broadcastToAll(ctx context.Context) {
	sb.mu.RLock()
	clients := make([]*sseClient, 0, len(sb.clients))
	for client := range sb.clients {
		clients = append(clients, client)
	}
	sb.mu.RUnlock()

	if len(clients) == 0 {
		return
	}

	// Build batch per API key and send
	for _, client := range clients {
		batch := sb.buildBatch(ctx, client.apiKey)
		eventData, err := sb.formatEvent("batch", batch)
		if err != nil {
			continue
		}

		select {
		case client.dataCh <- eventData:
		default:
			// Channel full, client is slow
			sb.logger.Debug("client channel full, dropping batch",
				zap.String("api_key", client.apiKey),
			)
		}
	}
}

func (sb *SyncBroadcaster) sendEvent(client *sseClient, eventType string, data interface{}) error {
	eventData, err := sb.formatEvent(eventType, data)
	if err != nil {
		return err
	}

	if _, err := client.writer.Write(eventData); err != nil {
		return err
	}
	client.flusher.Flush()
	return nil
}

func (sb *SyncBroadcaster) formatEvent(eventType string, data interface{}) ([]byte, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	sb.mu.RLock()
	seq := sb.sequence
	sb.mu.RUnlock()

	event := fmt.Sprintf("event: %s\nid: %d\ndata: %s\n\n", eventType, seq, jsonData)
	return []byte(event), nil
}

// maskAPIKey masks an API key, showing only the first 4 characters.
func maskAPIKey(key string) string {
	if len(key) <= 4 {
		return "****"
	}
	return key[:4] + "****"
}

// maskCacheKey masks the API key portion of a cache key.
// Supports formats: ticker/pkg/category/apiKey, ticker/pkg/apiKey, ws/hub/ticker/category/apiKey
func maskCacheKey(cacheKey string) string {
	parts := strings.Split(cacheKey, "/")
	if len(parts) >= 3 {
		parts[len(parts)-1] = maskAPIKey(parts[len(parts)-1])
		return strings.Join(parts, "/")
	}
	return cacheKey
}
