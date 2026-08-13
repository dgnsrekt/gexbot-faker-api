package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/config"
	"github.com/dgnsrekt/gexbot-downloader/internal/data"
	"github.com/dgnsrekt/gexbot-downloader/internal/eod"
	"github.com/dgnsrekt/gexbot-downloader/internal/observability"
	"github.com/dgnsrekt/gexbot-downloader/internal/server"
	"github.com/dgnsrekt/gexbot-downloader/internal/sync"
	"github.com/dgnsrekt/gexbot-downloader/internal/ws"
)

func main() {
	os.Exit(run())
}

func run() int {
	// Setup logger
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger: %v\n", err)
		return 1
	}
	defer func() { _ = logger.Sync() }()

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
		zap.Bool("syncBroadcastSystemEnabled", cfg.SyncBroadcastSystemEnabled),
		zap.Duration("syncBroadcastSystemInterval", cfg.SyncBroadcastSystemInterval),
	)

	// Load data
	logger.Info("loading data...", zap.String("mode", cfg.DataMode))
	start := time.Now()
	if _, statErr := os.Stat(filepath.Join(cfg.DataDir, cfg.DataDate)); os.IsNotExist(statErr) {
		if err := eod.MaterializeDate(cfg.DataDir, cfg.DataDate, logger); err != nil {
			logger.Error("failed to materialize EOD archive", zap.Error(err))
			return 1
		}
	}

	var initialLoader data.DataLoader
	switch cfg.DataMode {
	case "memory":
		initialLoader, err = data.NewMemoryLoader(cfg.DataDir, cfg.DataDate, logger)
	case "stream":
		initialLoader, err = data.NewStreamLoader(cfg.DataDir, cfg.DataDate, logger)
	default:
		logger.Error("unknown data mode", zap.String("mode", cfg.DataMode))
		return 1
	}
	if err != nil {
		logger.Error("failed to load data", zap.Error(err))
		return 1
	}
	// Record the load so the daemon's TTL cleanup keeps this date warm. Proactive
	// cleanup is now owned by the daemon (see internal/eod.CleanupStale).
	if err := eod.TouchLoaded(cfg.DataDir, cfg.DataDate); err != nil {
		logger.Warn("failed to mark loaded date", zap.Error(err))
	}

	// Wrap in reloadable loader for hot reload support
	reloadableLoader := data.NewReloadableLoader(initialLoader)
	defer func() { _ = reloadableLoader.Close() }()

	logger.Info("data loaded", zap.Duration("duration", time.Since(start)))
	observability.RegisterServer()
	observability.RegisterDataVolume(cfg.DataDir)
	observability.DataLoadedTimestamp.SetToCurrentTime()
	if date, parseErr := time.Parse("2006-01-02", cfg.DataDate); parseErr == nil {
		observability.DataDateTimestamp.Set(float64(date.Unix()))
	}

	// Create index cache
	cacheMode := data.CacheModeExhaust
	if cfg.CacheMode == "rotation" {
		cacheMode = data.CacheModeRotation
	}
	cache := data.NewIndexCache(cacheMode)

	// Create reload manager for hot reload support
	reloadManager := server.NewReloadManager(reloadableLoader, cache, cfg, logger)

	// Create server with reload manager
	srv := server.NewServer(reloadableLoader, cache, cfg, logger, reloadManager)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Heartbeat: keep the actively-served date's .last-loaded marker fresh so the
	// daemon's TTL cleanup never evicts a date we're still serving.
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := eod.TouchLoaded(cfg.DataDir, reloadManager.CurrentDate()); err != nil {
					logger.Warn("heartbeat: failed to mark loaded date", zap.Error(err))
				}
			}
		}
	}()

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
		negotiateHandler = ws.NewNegotiateHandler(logger, cfg.WSGroupPrefix)

		// Create and start orderflow streamer
		orderflowStreamer, err := ws.NewStreamer(orderflowHub, reloadableLoader, cache, cfg.WSStreamInterval, logger, reloadManager)
		if err != nil {
			logger.Error("failed to create orderflow streamer", zap.Error(err))
			return 1
		}
		go orderflowStreamer.Run(ctx)

		// Create and start GEX streamer
		gexStreamer, err := ws.NewGexStreamer(stateGexHub, reloadableLoader, cache, cfg.WSStreamInterval, logger, reloadManager)
		if err != nil {
			logger.Error("failed to create gex streamer", zap.Error(err))
			return 1
		}
		go gexStreamer.Run(ctx)

		// Create and start classic streamer
		classicStreamer, err := ws.NewClassicStreamer(classicHub, reloadableLoader, cache, cfg.WSStreamInterval, logger, reloadManager)
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
		greekStreamer, err := ws.NewGreekStreamer(stateGreeksZeroHub, reloadableLoader, cache, cfg.WSStreamInterval, logger, reloadManager)
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
		greekOneStreamer, err := ws.NewGreekOneStreamer(stateGreeksOneHub, reloadableLoader, cache, cfg.WSStreamInterval, logger, reloadManager)
		if err != nil {
			logger.Error("failed to create greek one streamer", zap.Error(err))
			return 1
		}
		go greekOneStreamer.Run(ctx)

		// Wire the hubs into the negotiate handler so PATCH /negotiate can manage
		// group memberships (keys match the websocket_urls hub names).
		negotiateHandler.SetHubs(map[string]*ws.Hub{
			"orderflow":         orderflowHub,
			"state_gex":         stateGexHub,
			"classic":           classicHub,
			"state_greeks_zero": stateGreeksZeroHub,
			"state_greeks_one":  stateGreeksOneHub,
		})

		logger.Info("WebSocket enabled",
			zap.Strings("hubs", []string{"orderflow", "state_gex", "classic", "state_greeks_zero", "state_greeks_one"}),
			zap.Duration("streamInterval", cfg.WSStreamInterval),
		)
	}

	// Sync Broadcast System (optional)
	var syncBroadcaster *sync.SyncBroadcaster
	if cfg.SyncBroadcastSystemEnabled {
		syncBroadcaster = sync.NewSyncBroadcaster(cache, reloadableLoader, cfg, logger)
		go syncBroadcaster.Run(ctx)

		logger.Info("Sync Broadcast System enabled",
			zap.String("broadcasterID", cfg.SyncBroadcastSystemID),
			zap.Duration("interval", cfg.SyncBroadcastSystemInterval),
		)
	}

	diagnostics := observability.NewDiagnostics(":9090", func() bool {
		if reloadManager.IsReloading() || reloadManager.CurrentDate() == "" || len(reloadableLoader.GetLoadedKeys()) == 0 {
			return false
		}
		return !cfg.WSEnabled || wsHubs != nil
	}, logger)
	diagnostics.Start(logger)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := diagnostics.Stop(ctx); err != nil {
			logger.Warn("diagnostics shutdown error", zap.Error(err))
		}
	}()

	// Create router
	router, err := server.NewRouter(srv, wsHubs, negotiateHandler, syncBroadcaster, logger)
	if err != nil {
		logger.Error("failed to create router", zap.Error(err))
		return 1
	}

	// Setup HTTP server
	httpServer := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // Disabled for SSE/WebSocket support
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
