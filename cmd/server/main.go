package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/config"
	"github.com/dgnsrekt/gexbot-downloader/internal/data"
	"github.com/dgnsrekt/gexbot-downloader/internal/server"
	"github.com/dgnsrekt/gexbot-downloader/internal/ws"
)

func main() {
	os.Exit(run())
}

func run() int {
	// Setup logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger: %v\n", err)
		return 1
	}
	defer logger.Sync()

	// Load config
	cfg, err := config.LoadServerConfig()
	if err != nil {
		logger.Error("failed to load config", zap.Error(err))
		return 1
	}

	logger.Info("configuration loaded",
		zap.String("port", cfg.Port),
		zap.String("dataDir", cfg.DataDir),
		zap.String("dataDate", cfg.DataDate),
		zap.String("dataMode", cfg.DataMode),
		zap.String("cacheMode", cfg.CacheMode),
		zap.String("endpointCacheMode", cfg.EndpointCacheMode),
		zap.Bool("wsEnabled", cfg.WSEnabled),
		zap.Duration("wsStreamInterval", cfg.WSStreamInterval),
	)

	// Load data
	logger.Info("loading data...", zap.String("mode", cfg.DataMode))
	start := time.Now()

	var loader data.DataLoader
	switch cfg.DataMode {
	case "memory":
		loader, err = data.NewMemoryLoader(cfg.DataDir, cfg.DataDate, logger)
	case "stream":
		loader, err = data.NewStreamLoader(cfg.DataDir, cfg.DataDate, logger)
	default:
		logger.Error("unknown data mode", zap.String("mode", cfg.DataMode))
		return 1
	}
	if err != nil {
		logger.Error("failed to load data", zap.Error(err))
		return 1
	}
	defer loader.Close()

	logger.Info("data loaded", zap.Duration("duration", time.Since(start)))

	// Create index cache
	cacheMode := data.CacheModeExhaust
	if cfg.CacheMode == "rotation" {
		cacheMode = data.CacheModeRotation
	}
	cache := data.NewIndexCache(cacheMode)

	// Create server
	srv := server.NewServer(loader, cache, cfg, logger)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// WebSocket components (optional)
	var wsHub *ws.Hub
	var negotiateHandler *ws.NegotiateHandler
	var streamer *ws.Streamer

	if cfg.WSEnabled {
		// Create WebSocket hub
		wsHub = ws.NewHub("orderflow", logger)
		go wsHub.Run(ctx)

		// Create negotiate handler
		negotiateHandler = ws.NewNegotiateHandler(logger)

		// Create and start streamer
		var err error
		streamer, err = ws.NewStreamer(wsHub, loader, cfg.WSStreamInterval, logger)
		if err != nil {
			logger.Error("failed to create streamer", zap.Error(err))
			return 1
		}
		go streamer.Run(ctx)

		logger.Info("WebSocket enabled",
			zap.String("hub", "orderflow"),
			zap.Duration("streamInterval", cfg.WSStreamInterval),
		)
	}

	// Create router
	router, err := server.NewRouter(srv, wsHub, negotiateHandler, logger)
	if err != nil {
		logger.Error("failed to create router", zap.Error(err))
		return 1
	}

	// Setup HTTP server
	httpServer := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Start server in goroutine
	go func() {
		logger.Info("starting server", zap.String("addr", httpServer.Addr))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", zap.Error(err))
		}
	}()

	// Wait for interrupt
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	// Cancel context to stop WebSocket components
	cancel()

	// Graceful HTTP server shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", zap.Error(err))
		return 1
	}

	// Silence unused variable warning
	_ = streamer

	logger.Info("server stopped")
	return 0
}
