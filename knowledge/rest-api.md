---
type: Reference
title: REST API reference
description: The faker's REST endpoint surface, split into real-GexBot parity routes and the faker-only control plane — market-data pulls, discovery, the load/cursor control routes, and health — plus the Swagger UI at /docs and the OpenAPI spec at /openapi.yaml.
tags: [rest, api, endpoints, openapi, swagger, parity, reference]
timestamp: 2026-08-11T00:00:00Z
---

# REST API reference

The full, always-current reference is the **Swagger UI at `/docs`**
(http://localhost:8080/docs), generated from the OpenAPI spec served at
**`/openapi.yaml`**. This page is the map; use Swagger for exact schemas and
try-it-out.

## Parity vs faker-only

Every endpoint is one of two kinds — Swagger tags them exactly this way:

- **GexBot parity** — behaves like the real GexBot API. A client written against
  GexBot works against the faker unchanged: same paths, shapes, and header auth.
  These are the market-data pulls and the discovery/history routes that mirror
  production.
- **Faker control plane** — **faker-only**. These drive the mock — load data for a
  day or span, move the replay cursor, inspect what's loaded. **The real GexBot API
  has none of them**; they're how you operate the faker.

## Real-GexBot parity

### Market-data pulls (auth header required; advance the cursor)

| Endpoint | Returns |
| --- | --- |
| `GET /{ticker}/classic/{aggregation}` | Classic GEX chain (`gex_full`/`gex_zero`/`gex_one`) |
| `GET /{ticker}/classic/{aggregation}/majors` | Classic majors summary |
| `GET /{ticker}/classic/{aggregation}/maxchange` | Max GEX-change lookbacks |
| `GET /{ticker}/state/{type}` | State GEX or greek profile (`gex_*`, `delta/gamma/vanna/charm_zero|one`) |
| `GET /{ticker}/orderflow/orderflow` | Orderflow metrics |
| `GET /options/{ticker}/expiries` | Option expiries within the horizon |
| `GET /futures/conversion` | Futures→index affine conversion |

Each successful pull returns the current snapshot and advances that key's cursor;
see [point a client](point-a-client.md) for the auth + cursor model.

### Discovery (no auth)

| Endpoint | Returns |
| --- | --- |
| `GET /tickers` · `GET /tickers/quant` | Available tickers |
| `GET /{package}/categories` | Categories in a package |

The historical download routes (`GET /hist/...`, `GET /download/{date}/{ticker}/...`)
also mirror GexBot's layout.

## Faker control plane

**Faker-only** — not present on the real GexBot API. The vocabulary matches the
[gexfakercli](gexfakercli.md) subcommands 1:1.

Mutating routes (`load`, `reset`) require the **Studio auth token** —
`STUDIO_AUTH_TOKEN`, presented as Basic/Bearer — **only when the server has one set**
(401 `{"error":"control route requires the Studio auth token"}` otherwise); an unset
token leaves them open (local dev). The reads below and `seek` (per-client) are never
gated. See [point a client](point-a-client.md).

### Load & inspect

| Endpoint | Effect |
| --- | --- |
| `POST /load` `{date}` \| `{from,to}` \| `{dates[]}` | Load a day or a span as one cross-day dataset. Async → `{job_id}`; single day = span-of-1 · *token-gated* |
| `GET /load/status/{job_id}` | Progress of an async load job |
| `GET /current-load` | What's loaded: `dates[]`, `from`, `to`, `files_loaded`, `loaded_at` |
| `GET /dates` | Materialized dates ready to load |
| `GET /available/{date}` | Data tree for a date (materializes on demand) |
| `GET /coverage?from=&to=` | Per-day tickers + union/intersection for a span (works pre-load) |

`POST /load` is **async-uniform**: it always returns a `job_id`; poll
`/load/status/{job_id}` until `state=done` (a single-day load just completes fast).

### Playback cursor

| Endpoint | Effect |
| --- | --- |
| `POST /reset?key=` | Rewind a key's cursor (all keys if no `key`) · *token-gated* |
| `POST /seek` `{timestamp,key}` | Seek a key to a unix timestamp; in range mode resolves across the span (`resolved_ts`, `day`, `in_gap`, `clamped`) |

### Health

| Endpoint | Returns |
| --- | --- |
| `GET /health` | Status, loaded date, data/cache mode |

## Related

- `GET /negotiate` and the `/ws/*` hubs → [WebSocket streaming](websockets.md).
- `GET /sync/stream` (SSE market-time broadcast) → see the README's Sync Broadcast
  section.

For request/response bodies and examples, open **`/docs`**.
