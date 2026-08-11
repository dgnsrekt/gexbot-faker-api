package main

import (
	"encoding/json"
	"net/url"
	"time"

	"github.com/spf13/cobra"
)

// Multi-day range replay commands. load-range materializes + loads a span so seek/replay crosses
// day boundaries; current-range and coverage are read-only (no token needed).

func loadRangeCmd() *cobra.Command {
	var from, to, dates string
	var noWait bool
	var timeoutSec int
	cmd := &cobra.Command{
		Use:   "load-range",
		Short: "Load a span of days for continuous cross-day replay (async; waits by default)",
		Long: "Provide --from and --to (inclusive; the archived days in that span are loaded) or an\n" +
			"explicit --dates list. Materializes any missing day, then serves the span as one\n" +
			"continuous dataset so seek/replay crosses day boundaries. Kicks off an async job and\n" +
			"polls it to completion unless --no-wait. Mutating: presents --token when set.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			body := map[string]any{}
			switch {
			case dates != "":
				body["dates"] = splitCSV(dates)
			case from != "" && to != "":
				body["from"], body["to"] = from, to
			default:
				return fail(&apiError{Msg: "provide --from and --to, or --dates"})
			}

			c := newClient()
			raw, err := c.postControlJSON(cmd.Context(), "/load-range", body)
			if err != nil {
				return fail(err)
			}
			var job struct {
				JobID string `json:"job_id"`
			}
			_ = json.Unmarshal(raw, &job)
			if noWait || job.JobID == "" {
				return emit(raw)
			}

			// Poll the job to a terminal state. Status is read-only (open route).
			deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
			for {
				st, err := c.get(cmd.Context(), "/load-range/status/"+job.JobID, false, nil)
				if err != nil {
					return fail(err)
				}
				var s struct {
					State string `json:"state"`
					Done  int    `json:"done"`
					Total int    `json:"total"`
				}
				_ = json.Unmarshal(st, &s)
				progress("load-range", "loading span", "state", s.State, "done", s.Done, "total", s.Total)
				if s.State == "done" || s.State == "error" {
					return emit(st)
				}
				if time.Now().After(deadline) {
					return fail(&apiError{Msg: "load-range did not finish before --timeout", Hint: "increase --timeout or check the faker logs; the job keeps running server-side"})
				}
				select {
				case <-cmd.Context().Done():
					return fail(&apiError{Msg: cmd.Context().Err().Error()})
				case <-time.After(500 * time.Millisecond):
				}
			}
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "span start YYYY-MM-DD (with --to)")
	cmd.Flags().StringVar(&to, "to", "", "span end YYYY-MM-DD (with --from)")
	cmd.Flags().StringVar(&dates, "dates", "", "explicit comma-separated dates (overrides --from/--to)")
	cmd.Flags().BoolVar(&noWait, "no-wait", false, "return the job immediately instead of polling to completion")
	cmd.Flags().IntVar(&timeoutSec, "timeout", 300, "max seconds to wait for the load to finish")
	return cmd
}

func currentRangeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "current-range",
		Short: "Show the currently loaded span (dates, from/to, files loaded)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			raw, err := newClient().get(cmd.Context(), "/current-range", false, nil)
			if err != nil {
				return fail(err)
			}
			return emit(raw)
		},
	}
}

func coverageCmd() *cobra.Command {
	var from, to string
	cmd := &cobra.Command{
		Use:   "coverage",
		Short: "Ticker coverage across a date span, pre-load (per-day + union + intersection)",
		Long: "Reads the archive inventory (no load required) and reports which tickers each day in\n" +
			"[--from, --to] covers, plus the union and intersection — so you can tell before loading\n" +
			"that a ticker isn't present on every day of the span.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if from == "" || to == "" {
				return fail(&apiError{Msg: "provide --from and --to"})
			}
			q := url.Values{"from": {from}, "to": {to}}
			raw, err := newClient().get(cmd.Context(), "/range-coverage", false, q)
			if err != nil {
				return fail(err)
			}
			return emit(raw)
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "span start YYYY-MM-DD")
	cmd.Flags().StringVar(&to, "to", "", "span end YYYY-MM-DD")
	return cmd
}
