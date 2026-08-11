package main

import (
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/dgnsrekt/gexbot-downloader/internal/api/generated"
)

// Control commands steer playback (reset/seek) and what data is loaded (load). reset/seek default to
// the CLI's --key so a plain `gexfakercli reset` rewinds the same cursor the data pulls advance. The
// MUTATING routes (reset, load) present the --token/GEXFAKER_TOKEN as Bearer when set, so they work
// against a token-gated faker; seek and the read-only queries (current-load, coverage) stay open.

func controlCmds() []*cobra.Command {
	return []*cobra.Command{
		resetCmd(),
		seekCmd(),
		loadCmd(),
		currentLoadCmd(),
		coverageCmd(),
	}
}

func resetCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Rewind the active key's playback cursor to the start",
		Long: "Rewinds the cursor selected by --key/GEXFAKER_KEY — the same cursor the data\n" +
			"pulls advance. Use --all to reset every key's cursor (affects other sessions).",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Default: scope the reset to this CLI's key so a plain `reset` rewinds
			// only the cursor our data pulls walk, not every other client's.
			var q url.Values
			if !all {
				q = url.Values{"key": {flagKey}}
			}
			raw, err := newClient().postControlJSON(cmd.Context(), "/reset?"+q.Encode(), nil)
			if err != nil {
				return fail(err)
			}
			return emit(raw)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "reset every key's cursor, not just the active --key")
	return cmd
}

func seekCmd() *cobra.Command {
	var key string
	cmd := &cobra.Command{
		Use:   "seek <unix-timestamp>",
		Short: "Seek a key's cursor to the first row at/after a timestamp",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ts, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fail(&apiError{Msg: "timestamp must be a unix integer: " + err.Error()})
			}
			k := key
			if k == "" {
				k = flagKey // seek requires a non-empty key; default to the CLI key
			}
			body := generated.SeekRequest{Timestamp: ts, Key: k}
			raw, err := newClient().postJSON(cmd.Context(), "/seek", false, body)
			if err != nil {
				return fail(err)
			}
			return emit(raw)
		},
	}
	cmd.Flags().StringVar(&key, "key", "", "key to seek (default: the CLI --key)")
	return cmd
}
