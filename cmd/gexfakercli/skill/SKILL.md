---
name: gexfakercli
description: Drive the GEX Faker API (replayed historical options/GEX data) from the command line. Use when asked to fetch GEX/greek/orderflow snapshots, list tickers or dates, load a data date or a span of days for continuous cross-day replay, seek playback, or bring a local faker up. Wraps the REST endpoints as JSON-first subcommands made for agents.
---

# gexfakercli

`gexfakercli` is a JSON-first CLI over the **GEX Faker** — a server that replays
historical options/GEX data as a mock of the real GexBot API. Every command prints
**one JSON document to stdout**; errors and progress go to **stderr** as JSON. Parse
stdout, check the exit code (nonzero = failure, error object on stderr).

## First move: get to a ready state

```bash
gexfakercli setup
```

`setup` is a zero→ready bootstrap. It finds a running faker (or brings one up via
`docker compose`), makes sure a date is loaded (unpacking an existing on-disk EOD
archive — **no API key needed**), verifies with a sample pull, and prints the ready
state: `{base_url, key, loaded_date, tickers, cache_mode, verified}`. It never
downloads without `GEXBOT_API_KEY` and never hangs silently — if it can't proceed it
tells you exactly what to do.

Then learn the full surface:

```bash
gexfakercli describe   # every command, endpoint, auth rule, and cursor semantics as JSON
```

## The key thing to understand: the playback cursor

Data pulls replay a day's snapshots **in order**. Each successful pull returns the
current row and **advances a per-key cursor by one**. The `--key` (default
`gexfakercli`; any non-empty token works, it is never validated) selects which
cursor you walk.

- `cache_mode=exhaust` → after the last row, pulls return HTTP 404 `No more data`.
- `cache_mode=rotation` → the cursor wraps to the start. Check `gexfakercli status`.
- `gexfakercli reset` rewinds to the start; `gexfakercli seek <unix-ts>` jumps to a time.
- **Multi-day**: `gexfakercli load-range` loads a span of days as one continuous dataset; the
  cursor then rolls from one day's last row into the next and `seek` resolves across the whole span
  (see below).

## Common commands

```bash
gexfakercli status                      # is it up + which date is loaded
gexfakercli tickers                     # stocks/indexes/futures  (--quant for quant set)
gexfakercli dates                       # dates available to load
gexfakercli load 2026-07-17             # load a date (materializes its archive if needed)

# Data pulls (advance the cursor). --fields trims the payload for token thrift:
gexfakercli classic SPX gex_zero --fields timestamp,spot,zero_gamma
gexfakercli state SPX gamma_zero
gexfakercli orderflow SPX
gexfakercli reset                       # replay from the start
```

Aggregations: `gex_full|gex_zero|gex_one`. State types: `gex_*`, and greeks
`delta_zero|gamma_zero|vanna_zero|charm_zero` (0DTE) / `..._one` (1DTE+).

## Multi-day replay (across trading days)

Load a contiguous **span** so replay crosses day boundaries instead of dying at the first session
end. Check coverage first (some days have fewer tickers), then load:

```bash
gexfakercli coverage --from 2026-08-06 --to 2026-08-10   # per-day tickers + union + intersection (pre-load)
gexfakercli load-range --from 2026-08-06 --to 2026-08-10 # materialize+load the span (async; waits to done)
gexfakercli load-range --dates 2026-08-06,2026-08-07     # or an explicit list
gexfakercli current-range                                # confirm the loaded span
```

`load-range` is asynchronous — it kicks off a job and polls to completion (progress lines on stderr);
pass `--no-wait` to return the job id immediately, `--timeout <sec>` to bound the wait.

Once a span is loaded, `seek <unix-ts>` resolves across the whole range. The response carries
`resolved_ts`, `day`, `in_gap`, `clamped`, and a per-stream `details[]`:

- **in a session gap** (overnight/weekend) → clamps forward to the next open, `in_gap: true`.
- **before / after the span** → `clamped: "start" | "end"`. After-end is `clamp` (last row) or `error`
  (HTTP 400) per the server's `RANGE_END_POLICY`.

Data pulls (`classic/state/orderflow`) then roll from one day's last row straight into the next.

## Control-route auth (gated fakers)

Mutating control routes — `load`, `reset`, `load-range` — require the faker's **Studio auth token**
when it is set (`STUDIO_AUTH_TOKEN` on the server, e.g. a LAN box). Present it with `--token` or
`GEXFAKER_TOKEN` (sent as Bearer). A `401` with the hint "requires the faker's Studio auth token"
means you need it. Read-only routes (`status`, `dates`, `coverage`, `current-range`, `seek`, data
pulls) never need it; an unset token means everything is open (local dev).

## Output control

- `--fields a,b,c` — keep only those top-level keys.
- `--pretty` — indent.
- `--url` / `GEXFAKER_URL` — target a non-default faker (default `http://127.0.0.1:8080`).
- `--key` / `GEXFAKER_KEY` — pick the data-route cursor.
- `--token` / `GEXFAKER_TOKEN` — the Studio auth token for mutating control routes (only when the
  faker is gated; empty = open).

## WebSocket streaming

Live streaming (5 hubs, protobuf/zstd frames) exists on the server but is **not yet
wrapped** by this CLI — it is a planned fast-follow. `gexfakercli describe` lists the
`/negotiate` details under `websocket`.
