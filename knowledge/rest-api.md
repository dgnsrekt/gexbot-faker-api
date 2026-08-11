---
type: Reference
title: REST API reference
description: The faker's REST endpoint surface — market-data pulls, discovery, control, and health — plus the interactive Swagger UI at /docs and the OpenAPI spec at /openapi.yaml.
tags: [rest, api, endpoints, openapi, swagger, reference]
timestamp: 2026-08-11T00:00:00Z
---

# REST API reference

The full, always-current reference is the **Swagger UI at `/docs`**
(http://localhost:8080/docs), generated from the OpenAPI spec served at
**`/openapi.yaml`**. This page is the map; use Swagger for exact schemas and
try-it-out.

## Market-data pulls (auth header required; advance the cursor)

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

## Discovery (no auth)

| Endpoint | Returns |
| --- | --- |
| `GET /tickers` · `GET /tickers/quant` | Available tickers |
| `GET /{package}/categories` | Categories in a package |
| `GET /available-dates` | Materialized dates ready to load |
| `GET /available-data/{date}` | Data tree for a date (materializes on demand) |
| `GET /current-date` | The currently loaded date |
| `GET /health` | Status, loaded date, data/cache mode |

## Control (no auth)

| Endpoint | Effect |
| --- | --- |
| `POST /reload-date` `{date}` | Load a date (materializes if needed); 409 if a reload is in progress |
| `POST /reset-cache?key=` | Rewind a key's cursor (all keys if no `key`) |
| `POST /seek-to-timestamp` `{timestamp,key}` | Seek a key to a unix timestamp |

## Related

- `GET /negotiate` and the `/ws/*` hubs → [WebSocket streaming](websockets.md).
- `GET /sync/stream` (SSE market-time broadcast) → see the README's Sync Broadcast
  section.
- The download link endpoints (`/download/{date}/{ticker}/...`) mirror GexBot's
  historical layout.

For request/response bodies and examples, open **`/docs`**.
