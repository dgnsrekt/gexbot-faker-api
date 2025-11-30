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
	var wsHubs *server.WebSocketHubs
	var negotiateHandler *ws.NegotiateHandler

	if cfg.WSEnabled {
		wsHubs = &server.WebSocketHubs{}

		// Create orderflow hub with validator
		orderflowHub := ws.NewHub("orderflow", logger, ws.IsValidOrderflowGroup)
		go orderflowHub.Run(ctx)
		wsHubs.Orderflow = orderflowHub

		// Create state_gex hub with validator
		stateGexHub := ws.NewHub("state_gex", logger, ws.IsValidStateGexGroup)
		go stateGexHub.Run(ctx)
		wsHubs.StateGex = stateGexHub

		// Create classic hub with validator
		classicHub := ws.NewHub("classic", logger, ws.IsValidClassicGroup)
		go classicHub.Run(ctx)
		wsHubs.Classic = classicHub

		// Create negotiate handler
		negotiateHandler = ws.NewNegotiateHandler(logger)

		// Create and start orderflow streamer
		orderflowStreamer, err := ws.NewStreamer(orderflowHub, loader, cfg.WSStreamInterval, logger)
		if err != nil {
			logger.Error("failed to create orderflow streamer", zap.Error(err))
			return 1
		}
		go orderflowStreamer.Run(ctx)

		// Create and start GEX streamer
		gexStreamer, err := ws.NewGexStreamer(stateGexHub, loader, cfg.WSStreamInterval, logger)
		if err != nil {
			logger.Error("failed to create gex streamer", zap.Error(err))
			return 1
		}
		go gexStreamer.Run(ctx)

		// Create and start classic streamer
		classicStreamer, err := ws.NewClassicStreamer(classicHub, loader, cfg.WSStreamInterval, logger)
		if err != nil {
			logger.Error("failed to create classic streamer", zap.Error(err))
			return 1
		}
		go classicStreamer.Run(ctx)

		// Create state_greeks_zero hub with validator
		stateGreeksZeroHub := ws.NewHub("state_greeks_zero", logger, ws.IsValidStateGreeksZeroGroup)
		go stateGreeksZeroHub.Run(ctx)
		wsHubs.StateGreeksZero = stateGreeksZeroHub

		// Create and start greek streamer
		greekStreamer, err := ws.NewGreekStreamer(stateGreeksZeroHub, loader, cfg.WSStreamInterval, logger)
		if err != nil {
			logger.Error("failed to create greek streamer", zap.Error(err))
			return 1
		}
		go greekStreamer.Run(ctx)

		// Create state_greeks_one hub with validator
		stateGreeksOneHub := ws.NewHub("state_greeks_one", logger, ws.IsValidStateGreeksOneGroup)
		go stateGreeksOneHub.Run(ctx)
		wsHubs.StateGreeksOne = stateGreeksOneHub

		// Create and start greek one streamer
		greekOneStreamer, err := ws.NewGreekOneStreamer(stateGreeksOneHub, loader, cfg.WSStreamInterval, logger)
		if err != nil {
			logger.Error("failed to create greek one streamer", zap.Error(err))
			return 1
		}
		go greekOneStreamer.Run(ctx)

		logger.Info("WebSocket enabled",
			zap.Strings("hubs", []string{"orderflow", "state_gex", "classic", "state_greeks_zero", "state_greeks_one"}),
			zap.Duration("streamInterval", cfg.WSStreamInterval),
		)
	}

	// Create router
	router, err := server.NewRouter(srv, wsHubs, negotiateHandler, logger)
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

	logger.Info("server stopped")
	return 0
}
