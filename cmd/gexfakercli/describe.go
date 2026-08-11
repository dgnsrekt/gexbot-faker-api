package main

import (
	"github.com/spf13/cobra"
)

// describe emits the whole capability surface as one JSON document. It is the
// agent's self-teaching entry point: SKILL.md tells the agent to run it first so
// it learns every command, endpoint, auth requirement, and the cursor semantics
// without reading source.

type cmdDoc struct {
	Command  string   `json:"command"`
	Endpoint string   `json:"endpoint"`
	Method   string   `json:"method"`
	Auth     bool     `json:"auth"`
	Consumes bool     `json:"consumes_cursor,omitempty"`
	Args     []string `json:"args,omitempty"`
	Flags    []string `json:"flags,omitempty"`
	Summary  string   `json:"summary"`
}

type describeDoc struct {
	Tool        string              `json:"tool"`
	Description string              `json:"description"`
	BaseURL     string              `json:"base_url"`
	Key         string              `json:"key"`
	Auth        map[string]string   `json:"auth"`
	Cursor      map[string]string   `json:"cursor"`
	Output      map[string]string   `json:"output"`
	Discovery   []cmdDoc            `json:"discovery"`
	Data        []cmdDoc            `json:"data"`
	Control     []cmdDoc            `json:"control"`
	Meta        []cmdDoc            `json:"meta"`
	Packages    map[string][]string `json:"packages"`
	Websocket   map[string]any      `json:"websocket"`
	Recipes     []string            `json:"recipes"`
}

func describeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "describe",
		Short: "Emit the full capability surface as one JSON document (for agents)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return emitValue(buildDescribe())
		},
	}
}

func buildDescribe() describeDoc {
	return describeDoc{
		Tool:        "gexfakercli",
		Description: "JSON-first client over the GEX Faker REST API, made for LLM agents.",
		BaseURL:     newClient().base,
		Key:         flagKey,
		Auth: map[string]string{
			"model":     "Only data routes require an Authorization header. Any non-empty token authenticates; it is never validated.",
			"purpose":   "The token seeds a per-key sequential playback cursor — pick a stable key per agent session.",
			"discovery": "tickers/categories/dates/available/status/reset/seek/load need no auth.",
		},
		Cursor: map[string]string{
			"advance":  "Each successful data pull returns the current row then advances this key's cursor by one.",
			"exhaust":  "cache_mode=exhaust: after the last row, data pulls return HTTP 404 {\"error\":\"No more data available\"}.",
			"rotation": "cache_mode=rotation: the cursor wraps to the start instead of 404. Check `status` for the mode.",
			"control":  "`reset` rewinds to index 0; `seek <ts>` jumps to the first row at/after a unix timestamp.",
		},
		Output: map[string]string{
			"stdout":   "A single JSON document per command. Parse this.",
			"stderr":   "Structured JSON errors and (for setup) progress lines. Never mix into stdout.",
			"--fields": "Comma-separated top-level keys to keep, e.g. --fields timestamp,spot.",
			"--pretty": "Indent the JSON output.",
			"exit":     "Nonzero exit code on any error; the error object is on stderr.",
		},
		Discovery: []cmdDoc{
			{Command: "tickers", Endpoint: "/tickers", Method: "GET", Summary: "List stocks/indexes/futures. --quant for /tickers/quant.", Flags: []string{"--quant"}},
			{Command: "categories <pkg>", Endpoint: "/{pkg}/categories", Method: "GET", Args: []string{"state|classic|orderflow"}, Summary: "List categories in a package."},
			{Command: "dates", Endpoint: "/available-dates", Method: "GET", Summary: "List materialized dates available to load."},
			{Command: "available <date>", Endpoint: "/available-data/{date}", Method: "GET", Args: []string{"date"}, Flags: []string{"--ticker"}, Summary: "Data tree for a date; materializes on demand."},
			{Command: "status", Endpoint: "/health + /current-date", Method: "GET", Summary: "Merged health and currently-loaded date."},
		},
		Data: []cmdDoc{
			{Command: "classic <ticker> <agg>", Endpoint: "/{ticker}/classic/{agg}", Method: "GET", Auth: true, Consumes: true, Args: []string{"ticker", "gex_full|gex_zero|gex_one"}, Flags: []string{"--majors", "--maxchange"}, Summary: "Next classic GEX snapshot (or majors/maxchange variant)."},
			{Command: "state <ticker> <type>", Endpoint: "/{ticker}/state/{type}", Method: "GET", Auth: true, Consumes: true, Args: []string{"ticker", "gex_*|delta_zero|gamma_zero|vanna_zero|charm_zero|delta_one|gamma_one|vanna_one|charm_one"}, Summary: "Next state GEX or greek-profile snapshot."},
			{Command: "orderflow <ticker>", Endpoint: "/{ticker}/orderflow/orderflow", Method: "GET", Auth: true, Consumes: true, Args: []string{"ticker"}, Summary: "Next orderflow snapshot."},
			{Command: "expiries <ticker>", Endpoint: "/options/{ticker}/expiries", Method: "GET", Auth: true, Args: []string{"ticker"}, Summary: "Option expiries within the published horizon."},
			{Command: "conversion", Endpoint: "/futures/conversion", Method: "GET", Auth: true, Flags: []string{"--ticker", "--future", "--model"}, Summary: "Futures->index affine conversion."},
		},
		Control: []cmdDoc{
			{Command: "reset", Endpoint: "/reset-cache", Method: "POST", Flags: []string{"--key"}, Summary: "Reset cursor(s) to index 0 (all keys, or one with --key)."},
			{Command: "seek <ts>", Endpoint: "/seek-to-timestamp", Method: "POST", Args: []string{"unix-timestamp"}, Flags: []string{"--key"}, Summary: "Seek a key's cursor to a timestamp."},
			{Command: "load <date>", Endpoint: "/reload-date", Method: "POST", Args: []string{"date"}, Summary: "Load a date; materializes its EOD archive if needed."},
		},
		Meta: []cmdDoc{
			{Command: "setup", Summary: "Zero->ready bootstrap: find/start a faker, load a date, verify a pull, print ready state."},
			{Command: "describe", Summary: "This document."},
			{Command: "skill install", Summary: "Install the embedded SKILL.md into Claude and/or Codex skills dirs."},
		},
		Packages: map[string][]string{
			"state":     stateTypes,
			"classic":   classicAggs,
			"orderflow": {"orderflow"},
		},
		Websocket: map[string]any{
			"status":    "phase 2 — not yet exposed by this CLI",
			"negotiate": "/negotiate (auth required; returns per-hub websocket URLs)",
			"hubs":      []string{"orderflow", "classic", "state_gex", "state_greeks_zero", "state_greeks_one"},
			"framing":   "Azure Web PubSub protobuf frames, zstd-compressed (a JSON variant also exists).",
		},
		Recipes: []string{
			"gexfakercli setup            # bring a faker to ready and print base_url/key",
			"gexfakercli status           # confirm it is up and which date is loaded",
			"gexfakercli classic SPX gex_zero --fields timestamp,spot,zero_gamma",
			"gexfakercli reset            # rewind to replay the day from the start",
		},
	}
}
