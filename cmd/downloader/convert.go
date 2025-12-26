package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func convertCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "convert-to-jsonl YYYY-MM-DD",
		Short: "Convert JSON files to JSONL format",
		Long: `Convert JSON array files to JSONL (JSON Lines) format.

Each JSON file containing an array will be converted to JSONL format,
where each array element becomes a single line. Original JSON files
are deleted after successful conversion.

Examples:
  # Convert JSON files for specific date
  gexbot-downloader convert 2025-11-14`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			date := args[0]
			dir := filepath.Join(cfg.Output.Directory, date)

			return convertJSONToJSONL(dir)
		},
	}

	return cmd
}

func convertJSONToJSONL(dir string) error {
	var converted, skipped, failed int

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-JSON files
		if info.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}

		// Skip staging directory
		if strings.Contains(path, ".staging") {
			return nil
		}

		jsonlPath := strings.TrimSuffix(path, ".json") + ".jsonl"

		// Skip if JSONL already exists
		if _, err := os.Stat(jsonlPath); err == nil {
			logger.Debug("skipping, JSONL exists", zap.String("file", path))
			skipped++
			return nil
		}

		logger.Info("converting", zap.String("file", path))

		if err := convertFile(path, jsonlPath); err != nil {
			logger.Error("conversion failed", zap.String("file", path), zap.Error(err))
			failed++
			return nil // Continue with other files
		}

		// Delete original JSON after successful conversion
		if err := os.Remove(path); err != nil {
			logger.Warn("failed to delete original", zap.String("file", path), zap.Error(err))
		}

		converted++
		return nil
	})

	if err != nil {
		return fmt.Errorf("walking directory: %w", err)
	}

	logger.Info("conversion complete",
		zap.Int("converted", converted),
		zap.Int("skipped", skipped),
		zap.Int("failed", failed),
	)

	if failed > 0 {
		return fmt.Errorf("%d files failed to convert", failed)
	}

	return nil
}

func convertFile(jsonPath, jsonlPath string) error {
	// Read JSON file
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	// Parse as array of raw JSON messages
	var items []json.RawMessage
	if err := json.Unmarshal(data, &items); err != nil {
		return fmt.Errorf("parsing JSON array: %w", err)
	}

	// Create JSONL file
	outFile, err := os.Create(jsonlPath)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer func() { _ = outFile.Close() }()

	// Write each item as a line
	for _, item := range items {
		// Compact the JSON (remove whitespace)
		compact, err := json.Marshal(item)
		if err != nil {
			return fmt.Errorf("compacting JSON: %w", err)
		}

		if _, err := outFile.Write(compact); err != nil {
			return fmt.Errorf("writing line: %w", err)
		}
		if _, err := outFile.WriteString("\n"); err != nil {
			return fmt.Errorf("writing newline: %w", err)
		}
	}

	return nil
}
