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
	Parity   bool     `json:"gexbot_parity"` // true = behaves like the real GexBot API; false = faker-only control plane
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
			"model":     "Data routes require an Authorization header (any non-empty --key; never validated, seeds the cursor).",
			"purpose":   "The data key seeds a per-key sequential playback cursor — pick a stable key per agent session.",
			"discovery": "Read-only routes (tickers/categories/dates/available/status/current-load/coverage/seek/load status) need no auth.",
			"control":   "When the faker sets STUDIO_AUTH_TOKEN, the MUTATING control routes (load, reset) require it — pass --token or set GEXFAKER_TOKEN (Basic or Bearer). Empty = open (local dev).",
			"parity":    "gexbot_parity=true marks routes that behave like the real GexBot API (a GexBot client works unchanged); false marks the faker-only control plane (load/reset/seek/dates/available/coverage/current-load), which the real API does not have.",
		},
		Cursor: map[string]string{
			"advance":  "Each successful data pull returns the current row then advances this key's cursor by one.",
			"exhaust":  "cache_mode=exhaust: after the last row, data pulls return HTTP 404 {\"error\":\"No more data available\"}.",
			"rotation": "cache_mode=rotation: the cursor wraps to the start instead of 404. Check `status` for the mode.",
			"control":  "`reset` rewinds to index 0; `seek <ts>` jumps to the first row at/after a unix timestamp.",
			"multiday": "`load --from A --to B` (or `--dates`) loads a contiguous span; then the cursor rolls from one day's last row into the next and `seek` resolves across the whole span. A seek into an inter-session gap clamps to the next open (in_gap); before/after the span clamp (clamped=start|end) per RANGE_END_POLICY (clamp|error).",
		},
		Output: map[string]string{
			"stdout":   "A single JSON document per command. Parse this.",
			"stderr":   "Structured JSON errors and (for setup) progress lines. Never mix into stdout.",
			"--fields": "Comma-separated top-level keys to keep, e.g. --fields timestamp,spot.",
			"--pretty": "Indent the JSON output.",
			"exit":     "Nonzero exit code on any error; the error object is on stderr.",
		},
		Discovery: []cmdDoc{
			{Command: "tickers", Endpoint: "/tickers", Method: "GET", Parity: true, Summary: "List stocks/indexes/futures. --quant for /tickers/quant.", Flags: []string{"--quant"}},
			{Command: "categories <pkg>", Endpoint: "/{pkg}/categories", Method: "GET", Parity: true, Args: []string{"state|classic|orderflow"}, Summary: "List categories in a package."},
			{Command: "dates", Endpoint: "/dates", Method: "GET", Summary: "List materialized dates available to load."},
			{Command: "available <date>", Endpoint: "/available/{date}", Method: "GET", Args: []string{"date"}, Flags: []string{"--ticker"}, Summary: "Data tree for a date; materializes on demand."},
			{Command: "status", Endpoint: "/health + /current-load", Method: "GET", Summary: "Merged health and what's currently loaded."},
		},
		Data: []cmdDoc{
			{Command: "classic <ticker> <agg>", Endpoint: "/{ticker}/classic/{agg}", Method: "GET", Auth: true, Parity: true, Consumes: true, Args: []string{"ticker", "gex_full|gex_zero|gex_one"}, Flags: []string{"--majors", "--maxchange"}, Summary: "Next classic GEX snapshot (or majors/maxchange variant)."},
			{Command: "state <ticker> <type>", Endpoint: "/{ticker}/state/{type}", Method: "GET", Auth: true, Parity: true, Consumes: true, Args: []string{"ticker", "gex_*|delta_zero|gamma_zero|vanna_zero|charm_zero|delta_one|gamma_one|vanna_one|charm_one"}, Summary: "Next state GEX or greek-profile snapshot."},
			{Command: "orderflow <ticker>", Endpoint: "/{ticker}/orderflow/orderflow", Method: "GET", Auth: true, Parity: true, Consumes: true, Args: []string{"ticker"}, Summary: "Next orderflow snapshot."},
			{Command: "expiries <ticker>", Endpoint: "/options/{ticker}/expiries", Method: "GET", Auth: true, Parity: true, Args: []string{"ticker"}, Summary: "Option expiries within the published horizon."},
			{Command: "conversion", Endpoint: "/futures/conversion", Method: "GET", Auth: true, Parity: true, Flags: []string{"--ticker", "--future", "--model"}, Summary: "Futures->index affine conversion."},
		},
		Control: []cmdDoc{
			{Command: "reset", Endpoint: "/reset", Method: "POST", Flags: []string{"--all"}, Summary: "Rewind the active --key's cursor to index 0 (--all resets every key). Mutating: presents --token when set."},
			{Command: "seek <ts>", Endpoint: "/seek", Method: "POST", Args: []string{"unix-timestamp"}, Flags: []string{"--key"}, Summary: "Seek a key's cursor to the first row at/after a timestamp. Response adds resolved_ts/day/in_gap/clamped and per-stream details[] (range mode)."},
			{Command: "load [date]", Endpoint: "/load (+ /load/status/{jobId})", Method: "POST", Args: []string{"date"}, Flags: []string{"--from", "--to", "--dates", "--no-wait", "--timeout"}, Summary: "Load one day (positional date) or a contiguous span (--from/--to or --dates) as one continuous dataset so seek/replay crosses day boundaries. Async; polls to done unless --no-wait. Mutating: presents --token when set."},
			{Command: "current-load", Endpoint: "/current-load", Method: "GET", Summary: "What's currently loaded (dates, from/to, files_loaded)."},
			{Command: "coverage", Endpoint: "/coverage", Method: "GET", Flags: []string{"--from", "--to"}, Summary: "Pre-load ticker coverage across a span: per-day tickers + union + intersection (from the archive inventory)."},
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
			"gexfakercli coverage --from 2026-08-06 --to 2026-08-10   # which tickers cover the span",
			"gexfakercli load 2026-08-07                              # load a single day (waits to done)",
			"gexfakercli load --from 2026-08-06 --to 2026-08-10       # load a span (waits to done)",
			"gexfakercli current-load     # confirm what's loaded",
			"gexfakercli seek 1786370149 --key sess1                  # cross-day seek (resolved_ts/day/in_gap/clamped)",
			"gexfakercli load --dates 2026-08-06,2026-08-07 --token $TOKEN   # gated faker",
		},
	}
}
