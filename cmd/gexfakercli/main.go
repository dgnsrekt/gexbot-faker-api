// Command gexfakercli is an LLM-first CLI over the GEX Faker REST API.
//
// It has three faces of one binary:
//   - client:  discovery, data pulls, and control mapped to endpoints, JSON-first
//     output for agents (`--fields` for token thrift, `describe` for self-teaching).
//   - skill:   `skill install` writes an embedded SKILL.md into the Claude/Codex
//     skills directories so an agent can drive the faker without prior knowledge.
//   - setup:   a zero->ready bootstrap that discovers or brings up a faker, loads a
//     date, verifies a sample pull, and prints the ready state.
//
// Scope is REST-first; WebSocket streaming is a documented fast-follow (see
// `describe`). The faker accepts any non-empty token and uses it only to seed a
// per-key sequential playback cursor, so the default `--key` needs no real
// credential.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Subcommands emit their own JSON error to stderr via fail(); the root has
	// SilenceErrors/SilenceUsage set so cobra does not also print a plain "Error:"
	// line. A nonzero exit is the machine-readable failure signal.
	if err := rootCmd().ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}
