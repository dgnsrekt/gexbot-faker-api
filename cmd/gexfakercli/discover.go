package main

import (
	"encoding/json"
	"net/url"

	"github.com/spf13/cobra"
)

// discoverCmds returns the unauthenticated discovery commands: what tickers,
// categories, and dates exist, and the merged server status.
func discoverCmds() []*cobra.Command {
	return []*cobra.Command{
		tickersCmd(),
		categoriesCmd(),
		datesCmd(),
		availableCmd(),
		statusCmd(),
	}
}

func tickersCmd() *cobra.Command {
	var quant bool
	cmd := &cobra.Command{
		Use:   "tickers",
		Short: "List available tickers (stocks, indexes, futures)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path := "/tickers"
			if quant {
				path = "/tickers/quant"
			}
			raw, err := newClient().get(cmd.Context(), path, false, nil)
			if err != nil {
				return fail(err)
			}
			return emit(raw)
		},
	}
	cmd.Flags().BoolVar(&quant, "quant", false, "list only quant-supported tickers (/tickers/quant)")
	return cmd
}

func categoriesCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "categories <state|classic|orderflow>",
		Short:     "List the categories in a data package",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"state", "classic", "orderflow"},
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := newClient().get(cmd.Context(), "/"+args[0]+"/categories", false, nil)
			if err != nil {
				return fail(err)
			}
			return emit(raw)
		},
	}
}

func datesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dates",
		Short: "List materialized dates available to load",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			raw, err := newClient().get(cmd.Context(), "/dates", false, nil)
			if err != nil {
				return fail(err)
			}
			return emit(raw)
		},
	}
}

func availableCmd() *cobra.Command {
	var ticker string
	cmd := &cobra.Command{
		Use:   "available <date>",
		Short: "Show tickers/packages/categories available for a date",
		Long: "Show the data tree for a date (YYYY-MM-DD). The server materializes the\n" +
			"date's EOD archive on demand if it is not yet unpacked.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var q url.Values
			if ticker != "" {
				q = url.Values{"ticker": {ticker}}
			}
			raw, err := newClient().get(cmd.Context(), "/available/"+args[0], false, q)
			if err != nil {
				return fail(err)
			}
			return emit(raw)
		},
	}
	cmd.Flags().StringVar(&ticker, "ticker", "", "filter to a single ticker")
	return cmd
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Merged health + currently-loaded date",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c := newClient()
			ctx := cmd.Context()

			health, err := c.get(ctx, "/health", false, nil)
			if err != nil {
				return fail(err)
			}
			out := map[string]json.RawMessage{}
			_ = json.Unmarshal(health, &out)

			// current-load is best-effort: merge its fields (dates/from/to/files_loaded)
			// when reachable so a single `status` call answers "is it up and what is loaded?".
			if cur, err := c.get(ctx, "/current-load", false, nil); err == nil {
				var m map[string]json.RawMessage
				if json.Unmarshal(cur, &m) == nil {
					for k, v := range m {
						out[k] = v
					}
				}
			}
			return emitValue(out)
		},
	}
}
