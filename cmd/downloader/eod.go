package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"

	"github.com/dgnsrekt/gexbot-downloader/internal/eod"
)

func eodCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "eod", Short: "Manage EOD archives"}
	cmd.AddCommand(eodPackCmd(), eodVerifyCmd(), eodMaterializeCmd(), eodPruneCmd())
	return cmd
}

func eodPackCmd() *cobra.Command {
	var ticker string
	var prune bool
	cmd := &cobra.Command{
		Use:   "pack <YYYY-MM-DD|all>",
		Short: "Pack existing JSONL data into EOD-compatible archives",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dates, err := eodDates(cfg.Output.Directory, args[0])
			if err != nil {
				return err
			}
			for _, date := range dates {
				tickers, err := eodTickers(cfg.Output.Directory, date, ticker)
				if err != nil {
					return err
				}
				for _, symbol := range tickers {
					manifest, err := eod.Pack(cfg.Output.Directory, date, symbol, "legacy-jsonl")
					if err != nil {
						return err
					}
					fmt.Printf("packed %s/%s: %d members\n", date, symbol, len(manifest.Members))
					if prune {
						if err := eod.PruneTicker(cfg.Output.Directory, date, symbol); err != nil {
							return err
						}
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&ticker, "ticker", "", "only process one ticker")
	cmd.Flags().BoolVar(&prune, "prune", false, "delete verified source JSONL")
	return cmd
}

func eodVerifyCmd() *cobra.Command {
	var ticker string
	cmd := &cobra.Command{
		Use:   "verify <YYYY-MM-DD|all>",
		Short: "Verify EOD archives and refresh their manifests",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dates, err := eodArchiveDates(cfg.Output.Directory, args[0])
			if err != nil {
				return err
			}
			for _, date := range dates {
				entries, err := os.ReadDir(filepath.Join(cfg.Output.Directory, "eod", date))
				if err != nil {
					return err
				}
				for _, entry := range entries {
					if !entry.IsDir() || (ticker != "" && ticker != entry.Name()) {
						continue
					}
					if _, err := eod.Verify(eod.ArchivePath(cfg.Output.Directory, date, entry.Name()), date, entry.Name(), ""); err != nil {
						return err
					}
					fmt.Printf("verified %s/%s\n", date, entry.Name())
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&ticker, "ticker", "", "only process one ticker")
	return cmd
}

func eodMaterializeCmd() *cobra.Command {
	var ticker string
	cmd := &cobra.Command{
		Use:   "materialize YYYY-MM-DD",
		Short: "Materialize EOD archives into the JSONL replay tree",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if ticker != "" {
				return eod.MaterializeTicker(cfg.Output.Directory, args[0], ticker)
			}
			return eod.MaterializeDate(cfg.Output.Directory, args[0])
		},
	}
	cmd.Flags().StringVar(&ticker, "ticker", "", "only process one ticker")
	return cmd
}

func eodPruneCmd() *cobra.Command {
	var ticker string
	cmd := &cobra.Command{
		Use:   "prune <YYYY-MM-DD|all>",
		Short: "Delete source JSONL for archived dates (verify-gated, no re-pack)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Only dates that already have an archive are candidates.
			dates, err := eodArchiveDates(cfg.Output.Directory, args[0])
			if err != nil {
				return err
			}
			for _, date := range dates {
				// Source JSONL may already be gone (pruned earlier) - skip.
				if _, err := os.Stat(filepath.Join(cfg.Output.Directory, date)); os.IsNotExist(err) {
					continue
				}
				tickers, err := eodTickers(cfg.Output.Directory, date, ticker)
				if err != nil {
					return err
				}
				for _, symbol := range tickers {
					// PruneTicker re-verifies the archive (hash + record count)
					// before deleting the source, so nothing is dropped unless a
					// good archive exists.
					if err := eod.PruneTicker(cfg.Output.Directory, date, symbol); err != nil {
						return fmt.Errorf("%s/%s: %w", date, symbol, err)
					}
					fmt.Printf("pruned %s/%s\n", date, symbol)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&ticker, "ticker", "", "only process one ticker")
	return cmd
}

func eodDates(root, requested string) ([]string, error) {
	if requested != "all" {
		return []string{requested}, nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var dates []string
	for _, entry := range entries {
		if entry.IsDir() && len(entry.Name()) == len("2006-01-02") {
			dates = append(dates, entry.Name())
		}
	}
	sort.Strings(dates)
	return dates, nil
}

func eodArchiveDates(root, requested string) ([]string, error) {
	if requested != "all" {
		return []string{requested}, nil
	}
	return eodDates(filepath.Join(root, "eod"), "all")
}

func eodTickers(root, date, requested string) ([]string, error) {
	if requested != "" {
		return []string{requested}, nil
	}
	entries, err := os.ReadDir(filepath.Join(root, date))
	if err != nil {
		return nil, err
	}
	var tickers []string
	for _, entry := range entries {
		if entry.IsDir() {
			tickers = append(tickers, entry.Name())
		}
	}
	sort.Strings(tickers)
	return tickers, nil
}
