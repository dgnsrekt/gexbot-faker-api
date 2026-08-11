package main

import (
	"context"
	"encoding/json"
	"net/url"
	"time"

	"github.com/spf13/cobra"
)

// Load commands. `load` materializes (if needed) and loads one day or a contiguous span as a single
// continuous dataset so seek/replay crosses day boundaries — it drives the async POST /load job and
// polls it to done. current-load and coverage are read-only (no token needed).

func loadCmd() *cobra.Command {
	var from, to, dates string
	var noWait bool
	var timeoutSec int
	cmd := &cobra.Command{
		Use:   "load [date]",
		Short: "Load a day or a span for replay (async; waits by default)",
		Long: "Load one day or a contiguous span as a single continuous dataset so seek/replay crosses\n" +
			"day boundaries. Give a positional YYYY-MM-DD date, or --from and --to (inclusive; the\n" +
			"archived days in that span are loaded), or an explicit --dates list. Materializes any\n" +
			"missing day, then kicks off an async job and polls it to completion unless --no-wait.\n" +
			"Mutating: presents --token when set.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			switch {
			case len(args) == 1:
				body["date"] = args[0]
			case dates != "":
				body["dates"] = splitCSV(dates)
			case from != "" && to != "":
				body["from"], body["to"] = from, to
			default:
				return fail(&apiError{Msg: "provide a date, --from and --to, or --dates"})
			}
			if !noWait && timeoutSec <= 0 {
				return fail(&apiError{Msg: "--timeout must be a positive number of seconds"})
			}

			c := newClient()
			raw, err := c.postControlJSON(cmd.Context(), "/load", body)
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

			// Bound the WHOLE polling phase (including any in-flight status GET) by --timeout, so a
			// small --timeout can't be swallowed by the HTTP client's own longer timeout.
			pollCtx, cancel := context.WithTimeout(cmd.Context(), time.Duration(timeoutSec)*time.Second)
			defer cancel()
			st, err := pollLoad(pollCtx, c, job.JobID)
			if err != nil {
				if cmd.Context().Err() == nil && pollCtx.Err() != nil {
					return fail(&apiError{Msg: "load did not finish before --timeout", Hint: "increase --timeout or check the faker logs; the job keeps running server-side"})
				}
				return fail(err)
			}
			return emit(st)
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "span start YYYY-MM-DD (with --to)")
	cmd.Flags().StringVar(&to, "to", "", "span end YYYY-MM-DD (with --from)")
	cmd.Flags().StringVar(&dates, "dates", "", "explicit comma-separated dates (overrides --from/--to)")
	cmd.Flags().BoolVar(&noWait, "no-wait", false, "return the job immediately instead of polling to completion")
	cmd.Flags().IntVar(&timeoutSec, "timeout", 300, "max seconds to wait for the load to finish")
	return cmd
}

// pollLoad polls GET /load/status/{jobId} until the job is done or errors, emitting progress lines to
// stderr. Returns the final status JSON on success. Honors ctx cancellation/timeout.
func pollLoad(ctx context.Context, c *apiClient, jobID string) ([]byte, error) {
	for {
		st, err := c.get(ctx, "/load/status/"+jobID, false, nil)
		if err != nil {
			return nil, err
		}
		var s struct {
			State string `json:"state"`
			Done  int    `json:"done"`
			Total int    `json:"total"`
			Error string `json:"error"`
		}
		_ = json.Unmarshal(st, &s)
		progress("load", "loading", "state", s.State, "done", s.Done, "total", s.Total)
		switch s.State {
		case "done":
			return st, nil
		case "error":
			msg := s.Error
			if msg == "" {
				msg = "load job failed"
			}
			return nil, &apiError{Msg: msg, Hint: "check the faker logs; the requested day(s) may not be archived"}
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// loadAndWait POSTs /load and polls it to completion, returning the final status JSON. Used by setup
// where the load must finish before the next step. Bounded by ctx.
func loadAndWait(ctx context.Context, c *apiClient, body map[string]any) ([]byte, error) {
	raw, err := c.postControlJSON(ctx, "/load", body)
	if err != nil {
		return nil, err
	}
	var job struct {
		JobID string `json:"job_id"`
	}
	_ = json.Unmarshal(raw, &job)
	if job.JobID == "" {
		return raw, nil
	}
	return pollLoad(ctx, c, job.JobID)
}

func currentLoadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "current-load",
		Short: "Show what's currently loaded (dates, from/to, files loaded)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			raw, err := newClient().get(cmd.Context(), "/current-load", false, nil)
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
			raw, err := newClient().get(cmd.Context(), "/coverage", false, q)
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
