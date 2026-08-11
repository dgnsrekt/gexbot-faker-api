---
type: Guide
title: Point a client at the faker
description: How to repoint an existing GexBot client at the faker — the base URL, the header auth model where any non-empty token works, and per-API-key sequential playback.
tags: [client, base-url, auth, playback, cursor, integration]
timestamp: 2026-08-11T00:00:00Z
---

# Point a client at the faker

The faker mirrors the real GexBot REST/WebSocket surface, so pointing an existing
client at it is usually a one-line change.

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
data route returns `400 {"error":"Authorization header not found."}`. Discovery
and control routes (`/tickers`, `/health`, `/reload-date`, …) need no header at
all.

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

## What to read next

- The full endpoint list and Swagger UI → [REST API](rest-api.md).
- Live streaming instead of polling → [WebSocket streaming](websockets.md).
- A ready-made client for agents → [gexfakercli](gexfakercli.md).
