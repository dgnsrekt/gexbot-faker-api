package main

import (
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/dgnsrekt/gexbot-downloader/internal/api/generated"
)

// Control commands are unauthenticated. They steer playback (reset/seek) and swap
// the loaded date (load). reset/seek default to the CLI's --key so a plain
// `gexfakercli reset` rewinds the same cursor the data pulls advance.

func controlCmds() []*cobra.Command {
	return []*cobra.Command{
		resetCmd(),
		seekCmd(),
		loadCmd(),
	}
}

func resetCmd() *cobra.Command {
	var key string
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset playback cursor(s) to the start",
		Long:  "With no --key, resets every key's positions. With --key, resets just that key.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var q url.Values
			// An explicitly-set --key scopes the reset; otherwise reset all keys.
			if cmd.Flags().Changed("key") {
				q = url.Values{"key": {key}}
			}
			raw, err := newClient().postJSON(cmd.Context(), "/reset-cache?"+q.Encode(), false, nil)
			if err != nil {
				return fail(err)
			}
			return emit(raw)
		},
	}
	cmd.Flags().StringVar(&key, "key", "", "reset only this key's cursor (default: all keys)")
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
			body := generated.SeekToTimestampRequest{Timestamp: ts, Key: k}
			raw, err := newClient().postJSON(cmd.Context(), "/seek-to-timestamp", false, body)
			if err != nil {
				return fail(err)
			}
			return emit(raw)
		},
	}
	cmd.Flags().StringVar(&key, "key", "", "key to seek (default: the CLI --key)")
	return cmd
}

func loadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "load <date>",
		Short: "Load a date (YYYY-MM-DD); materializes its EOD archive if needed",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := generated.ReloadDateRequest{Date: args[0]}
			raw, err := newClient().postJSON(cmd.Context(), "/reload-date", false, body)
			if err != nil {
				return fail(err)
			}
			return emit(raw)
		},
	}
}
