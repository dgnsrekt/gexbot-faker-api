---
type: Guide
title: Troubleshooting
description: Fixes for the common faker snags — no data for a ticker, a date stuck on archived vs ready, a 400 missing-auth error, an exhausted playback cursor, a disabled Download screen, and agent-sandbox localhost blocks.
tags: [troubleshooting, errors, debugging, faq]
timestamp: 2026-08-11T00:00:00Z
---

# Troubleshooting

## "Data not found for TICKER/…"

The loaded date doesn't have that ticker/package/category. Check what's actually
loaded with `gexfakercli available <date>` (or `GET /available/{date}`). A
loaded day only contains the tickers you downloaded — e.g. it may have QQQ but not
SPX. Download the ticker you want, then reload.

## A date shows "Materialize", not "Load"

Expected for CLI/daemon downloads: they land as `archived` (compressed archive
only). Click **Materialize** to unpack it, then **Load** — or just `load` it
(the server materializes on demand). Full explanation:
[materialize & load](materialize-load.md).

## A date has JSONL on disk but still shows "archived"

The daemon's after-hours `/hist` fallback writes JSONL without the
`.eod-materialized` marker (issue #38). Re-materialize it (Studio **Materialize**,
`gexbot-downloader eod materialize <date>`, or a `load`) to write the marker and
flip it to `ready`.

## `400 {"error":"Authorization header not found."}`

You hit a market-data route without an `Authorization` header. Send any non-empty
token — the faker doesn't validate it (`export GEXBOT_API_KEY=test123`, or
`--key` in gexfakercli). Discovery/control routes need no header. See
[point a client](point-a-client.md).

## `404 {"error":"No more data available"}`

The key's playback cursor reached the end of the day in `exhaust` mode. Rewind it
with `gexfakercli reset` (or `POST /reset`), or run the server with
`CACHE_MODE=rotation` to loop instead of 404.

## The Download screen says "set GEXBOT_API_KEY"

Downloading real history needs a key. Set `GEXBOT_API_KEY` on the server and
restart. Replaying already-downloaded data needs no key.

## Logs / Monitoring show "unavailable"

Those screens proxy Loki (`LOKI_URL`) and Prometheus (`PROMETHEUS_URL`), which
run in the docker stack. Running the server via `go run` without the stack leaves
them unset, so the screens degrade with a message — expected. Use `just up` for
the full stack. See [docker & observability](docker-observability.md).

## An agent's first `gexfakercli` call fails with "operation not permitted"

Some agent sandboxes (e.g. Codex) block localhost by default; the first call to
`127.0.0.1:8080` is denied by the sandbox, not the faker. Approve local network
access and retry — the structured error distinguishes a sandbox denial from the
service being down.

## Studio not built (running `go run`)

`/studio` shows a "UI hasn't been built" hint if `web/dist` is empty. Run
`just studio-build` (or `just serve-gex-faker`, which builds it) and restart. The
same applies to `/guides` — run `just docs-build`.
