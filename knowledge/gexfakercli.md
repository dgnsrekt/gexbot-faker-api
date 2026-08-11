---
type: Guide
title: The gexfakercli agent CLI
description: A JSON-first command-line client over the faker made for LLM agents — the setup bootstrap, the describe capability dump, data pulls, the playback cursor, and installing it as a Claude/Codex skill.
tags: [gexfakercli, cli, agent, llm, setup, describe, cursor]
timestamp: 2026-08-11T00:00:00Z
---

# The gexfakercli agent CLI

`gexfakercli` (`cmd/gexfakercli`) is a **JSON-first CLI over the faker, made for
LLM agents**. Every command prints **one JSON document to stdout**; errors and
progress go to **stderr** as JSON; the exit code is nonzero on failure. Build it
with `just build-gexfakercli`.

## First move: get to a ready state

```bash
gexfakercli setup
```

`setup` is a zero→ready bootstrap: it finds a running faker (or brings one up via
`docker compose`), ensures a date is loaded by unpacking an on-disk EOD archive
(**no API key needed** — the keyless [materialize & load](materialize-load.md)
path), verifies with a sample pull, and prints the ready state
(`{base_url, key, loaded_date, tickers, cache_mode, verified}`). It never
downloads without `GEXBOT_API_KEY` and never hangs silently.

Then learn the surface:

```bash
gexfakercli describe   # every command, endpoint, auth rule, and cursor rule as JSON
```

## The playback cursor

Data pulls replay the day in order; each pull advances a **per-`--key` cursor**
(default key `gexfakercli`, never validated — see [point a client](point-a-client.md)).

- `cache_mode=exhaust` → `404 "No more data"` at the end; `rotation` → wraps.
- `gexfakercli reset` rewinds the active key's cursor (`--all` resets every key).
- `gexfakercli seek <unix-ts>` jumps to the first snapshot at/after a timestamp.

## Common commands

```bash
gexfakercli status                      # is it up + which date is loaded
gexfakercli tickers                     # stocks/indexes/futures (--quant for the quant set)
gexfakercli dates                       # dates available to load
gexfakercli load 2026-07-17             # load a date (materializes if needed)

# data pulls (advance the cursor); --fields trims the payload for token thrift
gexfakercli classic SPX gex_zero --fields timestamp,spot,zero_gamma
gexfakercli state SPX gamma_zero
gexfakercli orderflow SPX
gexfakercli reset
```

Aggregations: `gex_full|gex_zero|gex_one`. State types: `gex_*` and greeks
`delta|gamma|vanna|charm_zero` (0DTE) / `..._one` (1DTE+).

Output control: `--fields a,b,c`, `--pretty`, `--url` / `GEXFAKER_URL`
(default `http://127.0.0.1:8080`), `--key` / `GEXFAKER_KEY`.

## Install it as an agent skill

The binary embeds a `SKILL.md`; install it into Claude and/or Codex so an agent
discovers it automatically:

```bash
gexfakercli skill install            # both agents if their dirs exist
gexfakercli skill install --codex    # or target one
```

This writes to `~/.claude/skills/gexfakercli/` and/or
`~/.codex/skills/gexfakercli/`.

## WebSocket streaming

Live streaming (the five hubs, protobuf/zstd) exists on the server but is **not
yet wrapped** by this CLI — a planned fast-follow. `describe` lists the
`/negotiate` details under `websocket`. To stream today, see
[WebSocket streaming](websockets.md).
