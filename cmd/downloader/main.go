package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/dgnsrekt/gexbot-downloader/internal/config"
)

var (
	cfgFile string
	verbose bool
	logger  *zap.Logger
	cfg     *config.Config
)

func setupLogger(verbose bool, logCfg *config.LoggingConfig) (*zap.Logger, error) {
	var zapConfig zap.Config
	if verbose {
		zapConfig = zap.NewDevelopmentConfig()
	} else {
		zapConfig = zap.NewProductionConfig()
		zapConfig.DisableStacktrace = true
	}

	// Set log level from config
	if logCfg != nil && logCfg.Level != "" {
		var level zapcore.Level
		if err := level.UnmarshalText([]byte(logCfg.Level)); err == nil {
			zapConfig.Level = zap.NewAtomicLevelAt(level)
		}
	}

	// Add file output if enabled
	if logCfg != nil && logCfg.Enabled {
		if err := os.MkdirAll(logCfg.Directory, 0755); err != nil {
			return nil, fmt.Errorf("creating logs directory: %w", err)
		}
		timestamp := time.Now().Format("2006-01-02_15-04-05")
		logFile := filepath.Join(logCfg.Directory, fmt.Sprintf("downloader_%s.log", timestamp))
		zapConfig.OutputPaths = append(zapConfig.OutputPaths, logFile)
	}

	return zapConfig.Build()
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "gexbot-downloader",
		Short: "Download historical data from Gexbot API",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip config loading for help commands
			if cmd.Name() == "help" || cmd.Name() == "completion" {
				// Use basic logger for help commands
				var err error
				logger, err = setupLogger(verbose, nil)
				return err
			}

			// Load config
			var err error
			cfg, err = config.Load(cfgFile)
			if err != nil {
				return err
			}

			// Setup logger with config
			logger, err = setupLogger(verbose, &cfg.Logging)
			if err != nil {
				return err
			}

			return nil
		},
	}

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", os.Getenv("GEXBOT_DOWNLOADER_CONFIG"), "config file path (or set GEXBOT_DOWNLOADER_CONFIG)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	rootCmd.AddCommand(downloadCmd())
	rootCmd.AddCommand(convertCmd())

	// Setup signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}
