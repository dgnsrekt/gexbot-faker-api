---
type: Guide
title: Point a client at the faker
description: How to repoint an existing GexBot client at the faker — the base URL, the header auth model where any non-empty token works, and per-API-key sequential playback.
tags: [client, base-url, auth, playback, cursor, integration]
timestamp: 2026-08-11T00:00:00Z
---

# Point a client at the faker

The faker mirrors the real GexBot API's **primary data routes and payload
shapes**, so pointing an existing client at it is usually a one-line change. It is
not a total mirror — see the parity note in [overview](overview.md) and the
[compatibility matrix](https://github.com/dgnsrekt/gexbot-faker-api/blob/main/compatibility/matrix.json)
for known differences (e.g. `/tickers`).

## 1. Change the base URL

```
https://api.gexbot.com   →   http://localhost:8080
```

For example, [quant-python-sockets](https://github.com/nfa-llc/quant-python-sockets)
just needs `BASE_URL = "http://localhost:8080"`. The origin the Studio was reached
on is also the API base, so copied URLs stay correct behind a reverse proxy.

## 2. Auth: any non-empty token works

Market-data routes require an `Authorization` header, but the faker **never
validates the token** — any non-empty value authenticates:

```bash
export GEXBOT_API_KEY=test123   # the faker accepts any key
```

`Basic`, `Bearer`, and a bare token are all accepted. A **missing** header on a
data route returns `400 {"error":"Authorization header not found."}`. Read-only
discovery (`/tickers`, `/health`, `/available-dates`, …) needs no header.

### Control routes and the Studio token

The **mutating** control routes — `reload-date`, `reset-cache`, `load-range` —
change global server state, so they're gated behind the server's **Studio auth
token** (`STUDIO_AUTH_TOKEN`) **when one is set** (typical for a LAN/remote faker):
present it as `Basic`/`Bearer` or you get `401 {"error":"control route requires the
Studio auth token"}`. An **unset** token (local dev) leaves them open. Reads and the
per-client `seek-to-timestamp` are never gated. From the CLI, pass the token with
`--token` / `GEXFAKER_TOKEN` (see [gexfakercli](gexfakercli.md)).

## 3. Understand per-key playback

The token isn't just a passphrase — it **seeds a per-key playback cursor**. Each
distinct key walks the loaded day's snapshots independently:

- Every successful data pull returns the **current** snapshot, then **advances**
  that key's cursor by one.
- Two clients with different keys replay at their own pace; two with the same key
  share one cursor.
- `cache_mode=exhaust` (default): after the last snapshot, pulls return
  `404 {"error":"No more data available"}`.
- `cache_mode=rotation`: the cursor wraps to the start instead of 404.

Control the cursor with `POST /reset-cache` (rewind) and `POST /seek-to-timestamp`
(jump to a time) — or via [gexfakercli](gexfakercli.md) (`reset`, `seek`).

## 4. Multi-day range replay

A client can load a **contiguous span of days** as one cross-day dataset with
`POST /load-range` (`{from,to}` or `{dates[]}`), so the cursor rolls from one day's
last snapshot into the next instead of ending at a session boundary, and
`seek-to-timestamp` resolves anywhere in the span (with `in_gap`/`clamped` for
overnight gaps and span edges). See [gexfakercli](gexfakercli.md) (`load-range`,
`current-range`, `coverage`) and [materialize & load](materialize-load.md).

## What to read next

- The full endpoint list and Swagger UI → [REST API](rest-api.md).
- Live streaming instead of polling → [WebSocket streaming](websockets.md).
- A ready-made client for agents → [gexfakercli](gexfakercli.md).
