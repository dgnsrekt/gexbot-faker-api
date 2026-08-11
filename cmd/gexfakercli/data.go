package main

import (
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

// Data-pull commands hit the authenticated market-data routes. Each successful
// call consumes one row of this key's playback cursor, so repeated calls walk the
// day's snapshots in order (cache_mode=exhaust stops at the end with HTTP 404;
// rotation wraps). Use `reset`/`seek` to control the cursor.

var (
	stateTypes = []string{
		"gex_full", "gex_zero", "gex_one",
		"delta_zero", "gamma_zero", "vanna_zero", "charm_zero",
		"delta_one", "gamma_one", "vanna_one", "charm_one",
	}
	classicAggs = []string{"gex_full", "gex_zero", "gex_one"}
)

func dataCmds() []*cobra.Command {
	return []*cobra.Command{
		classicCmd(),
		stateCmd(),
		orderflowCmd(),
		expiriesCmd(),
		conversionCmd(),
	}
}

func classicCmd() *cobra.Command {
	var majors, maxchange bool
	cmd := &cobra.Command{
		Use:   "classic <ticker> <gex_full|gex_zero|gex_one>",
		Short: "Pull the next classic GEX snapshot for a ticker",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ticker, agg := strings.ToUpper(args[0]), args[1]
			if !contains(classicAggs, agg) {
				return fail(&apiError{Msg: "invalid aggregation " + agg + " (want " + strings.Join(classicAggs, "|") + ")"})
			}
			path := "/" + ticker + "/classic/" + agg
			switch {
			case majors:
				path += "/majors"
			case maxchange:
				path += "/maxchange"
			}
			raw, err := newClient().get(cmd.Context(), path, true, nil)
			if err != nil {
				return fail(err)
			}
			return emit(raw)
		},
	}
	cmd.Flags().BoolVar(&majors, "majors", false, "pull the majors variant (.../majors)")
	cmd.Flags().BoolVar(&maxchange, "maxchange", false, "pull the max-change variant (.../maxchange)")
	return cmd
}

func stateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "state <ticker> <type>",
		Short: "Pull the next state GEX or greek-profile snapshot",
		Long:  "type is one of: " + strings.Join(stateTypes, ", "),
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ticker, typ := strings.ToUpper(args[0]), args[1]
			if !contains(stateTypes, typ) {
				return fail(&apiError{Msg: "invalid state type " + typ + " (want one of " + strings.Join(stateTypes, "|") + ")"})
			}
			raw, err := newClient().get(cmd.Context(), "/"+ticker+"/state/"+typ, true, nil)
			if err != nil {
				return fail(err)
			}
			return emit(raw)
		},
	}
}

func orderflowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "orderflow <ticker>",
		Short: "Pull the next orderflow snapshot for a ticker",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ticker := strings.ToUpper(args[0])
			raw, err := newClient().get(cmd.Context(), "/"+ticker+"/orderflow/orderflow", true, nil)
			if err != nil {
				return fail(err)
			}
			return emit(raw)
		},
	}
}

func expiriesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "expiries <ticker>",
		Short: "List option expiries for a ticker within the published horizon",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ticker := strings.ToUpper(args[0])
			raw, err := newClient().get(cmd.Context(), "/options/"+ticker+"/expiries", true, nil)
			if err != nil {
				return fail(err)
			}
			return emit(raw)
		},
	}
}

func conversionCmd() *cobra.Command {
	var ticker, future, model string
	cmd := &cobra.Command{
		Use:   "conversion",
		Short: "Get the futures->index affine conversion (multiplier, additive)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if ticker == "" || future == "" {
				return fail(&apiError{Msg: "--ticker and --future are required"})
			}
			q := url.Values{"ticker": {strings.ToUpper(ticker)}, "future": {strings.ToUpper(future)}}
			if model != "" {
				q.Set("model", model)
			}
			raw, err := newClient().get(cmd.Context(), "/futures/conversion", true, q)
			if err != nil {
				return fail(err)
			}
			return emit(raw)
		},
	}
	cmd.Flags().StringVar(&ticker, "ticker", "", "index ticker (e.g. SPX)")
	cmd.Flags().StringVar(&future, "future", "", "future contract root (e.g. ES)")
	cmd.Flags().StringVar(&model, "model", "", "conversion model (optional)")
	return cmd
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
