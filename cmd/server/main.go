package main

import (
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

	// Create router
	router, err := server.NewRouter(srv, logger)
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
	return 0
}
