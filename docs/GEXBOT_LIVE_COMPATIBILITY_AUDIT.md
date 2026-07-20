# GexBot live compatibility audit

Date: 2026-07-19

## Goal

Make `gexbot-faker-api` a drop-in replacement for the current GexBot market-data API. Research is out of scope. Faker health, playback, download, documentation, and synchronization routes are control-plane extensions and do not count against parity.

## Sources and precedence

1. Bounded observations from `https://www.gexbot.com/apidocs` and `https://api.gex.bot`.
2. `nfa-llc/gexbot-openapi` at `fb026f49af3e370ccf2fbd6a57092b284065e4ed`.
3. `nfa-llc/quant-python-sockets` and `nfa-llc/quant-historical`.
4. Existing faker code and archived implementation notes.

The frozen target contract and sanitized evidence are in `compatibility/`.

## Result

The current public market-data surface contains 16 method/path operations:

- 8 route shapes exist in the faker but have request or behavioral drift.
- 8 operations are missing.
- 0 operations currently meet the strict drop-in definition end to end.

The core replay payloads are in better shape than the route count suggests. Live probes confirmed that Classic, Greek profile, majors, max-change, and Orderflow top-level response fields match the faker models. The largest gaps are authentication, discovery/history routes, and the WebSocket subscription lifecycle.

## Endpoint matrix

| Operation | Faker | Primary gap |
|---|---|---|
| `GET /v2/{ticker}/classic/{category}` | Partial | Query key instead of Bearer auth and required headers |
| `GET /v2/{ticker}/state/{category}` | Partial | Auth drift; response selection is undocumented in upstream OpenAPI |
| `GET /v2/{ticker}/orderflow/orderflow` | Partial | Auth drift |
| `GET /v2/{ticker}/classic/{category}/majors` | Partial | Auth drift |
| `GET /v2/{ticker}/state/{category}/majors` | Partial | Auth drift |
| `GET /v2/{ticker}/classic/{category}/maxchange` | Partial | Auth drift |
| `GET /v2/{ticker}/state/{category}/maxchange` | Partial | Auth drift |
| `GET /v2/tickers` | Partial | Returns only loaded tickers, not the supported universe |
| `GET /{package}/categories` | Missing | No route |
| `GET /tickers/quant` | Missing | No route |
| `GET /v2/options/{ticker}/expiries` | Missing | No route or expiry model |
| `GET /v2/hist/{ticker}/{package}/{category}/{date}` | Missing | Existing faker download routes use a different contract |
| `GET /v2/hist/eod/{ticker}` | Missing | New live endpoint |
| `GET /v2/futures/conversion` | Missing | New live endpoint |
| `POST /v2/negotiate` | Missing | Faker implements deprecated GET only |
| `PATCH /v2/negotiate` | Missing | No server-side group replacement |

The live service accepts both `/hist/eod/{ticker}` and `/v2/hist/eod/{ticker}`. The target snapshot uses the v2 form inherited from its server URL.

## REST findings

### Authentication and headers

Current protected endpoints use:

```http
Authorization: Bearer <API_KEY>
User-Agent: <client identifier>
Accept: application/json
```

The faker's replay handlers require a `key` query parameter. Its historical client and legacy WebSocket negotiation use Basic authentication.

Observed live behavior:

- Missing Authorization returned `400`, not the documented `401`.
- Missing User-Agent returned `400`.
- Missing Accept was accepted for a Classic request even though documentation marks it required.

For compatibility, the faker should accept documented requests and reproduce observed error bodies/statuses where a client can depend on them. It should not reject a request the live service accepts.

### Payloads

Bounded live probes confirmed matching top-level fields for:

- Classic chain
- Classic and State majors
- Classic and State max-change
- State Greek profiles
- Orderflow

The upstream OpenAPI incorrectly describes State Greek responses as the Classic-style `basic_response`. It also lists additional GEX fields on Orderflow that were absent from the observed live response. Live fixtures should override those repository-spec errors.

`GET /tickers` uses the same object shape in both systems, but the semantics differ: live returns the complete supported universe while the faker derives its response from currently loaded files.

### Historical API

The current contract:

- Defaults to `302` redirect.
- Returns `{"url":"..."}` when `noredirect` is present.
- Supports gzip and announces a gzip-only transition starting 2026-04-30.
- Restricts history to a 90-day lookback.

The faker instead exposes `/download/{date}/{ticker}/...` routes that serve local files directly. Those are useful extensions but are not compatible replacements for `/v2/hist/...`.

The historical live probe hit `429` after the preceding audit requests, confirming rate limiting but leaving the signed-URL response unverified in this run.

### EOD report

The live EOD endpoint returned:

- `200`
- `Content-Type: application/zip`
- `Content-Disposition: attachment; filename=eod_report_SPY_2026-07-17.zip`
- `Content-Length: 81644081`

The body was not downloaded. The endpoint returns the newest completed report and may return the prior trading day's report before the current export completes.

### Futures conversion

The new endpoint returns:

```json
{
  "future_contract": "ESU6",
  "multiplier": 1,
  "additive": 39.619786804
}
```

Values are dynamic and update every 15 minutes during regular trading hours. Supported pairs are SPX/SPY→ES, NDX/QQQ→NQ, RUT/IWM→RTY, DIA→YM, GLD→GC, and USO→CL. An invalid SPX→NQ pair returned `400`.

A replay implementation needs captured conversion data or a date-aware configured fixture; inventing a live offset from historical option files would not be faithful.

## WebSocket findings

### Negotiation

Current API behavior:

1. `POST /v2/negotiate` receives an initial unprefixed group list.
2. The server auto-joins those groups and returns all authorized hub URLs.
3. `PATCH /v2/negotiate` replaces the complete group set without reconnecting.
4. Clients renegotiate when signed connection tokens expire.

Faker behavior:

1. `GET /negotiate` requires Basic auth.
2. It returns five local hub URLs plus a `blue` prefix.
3. Clients connect and send Azure `joinGroup` commands.
4. There is no PATCH operation or token-expiration lifecycle.

POST negotiation can close existing live connections for the same API slot. It was not invoked with the shared local key; verification requires a dedicated audit key or a window where interrupting that slot is safe.

### Hubs and explicit expiries

Current hubs:

- `classic`
- `state_gex`
- `state_greeks_zero`
- `state_greeks`
- `state_greeks_one`
- `orderflow`

The faker lacks `state_greeks`, so it cannot serve explicit-expiry Greek groups such as `SPX_state_gamma_20260717`. It also lacks expiry discovery and explicit-expiry streamers. Standard groups publish roughly every second; explicit-expiry groups publish roughly every five seconds and are realtime-only.

### Wire format

The faker and Python example protobuf files have matching packages, field numbers, and scalar types; differences are comments and Go package options. The faker already produces:

- Azure Web PubSub downstream messages
- `google.protobuf.Any`
- Zstd-compressed concrete protobuf payloads
- `proto.gex`, `proto.greek`, and `proto.orderflow` type URLs

Remaining live checks:

- Capture the actual `Any.type_url` strings.
- Compare decompressed bytes field by field.
- Confirm numeric scaling and truncation for negative and fractional values.
- Verify JSON subprotocol behavior.
- Capture disconnect/token-expiration behavior.

The Python example's unresolved native Zstd crash means captured production frames should be decoded in an isolated process during analysis.

## Faker extensions

These routes are useful but are not part of live parity:

- `/health`
- `/reset-cache`
- `/seek-to-timestamp`
- `/reload-date`
- `/available-dates`
- `/current-date`
- `/available-data/{date}`
- `/download/...`
- `/sync/stream`
- `/docs`, `/asyncapi`, and their specifications/assets

They should remain clearly documented as faker control-plane extensions. They should not replace or alter the matching public GexBot routes.

## Remediation order

1. Mount the compatibility API at `/v2`, add Bearer/User-Agent handling, and allow PATCH in CORS.
2. Replace legacy negotiation with POST/PATCH server-side subscriptions and add `state_greeks`.
3. Add categories, Quant tickers, expiries, and the live historical route.
4. Add EOD report and a fixture-backed futures conversion endpoint.
5. Match observed error statuses, media types, redirects, gzip behavior, and response semantics.
6. Add handler and WebSocket integration tests, then change matrix rows from mismatch/missing to matching.

## Continuous check

Run:

```bash
go test ./compatibility
```

The test fails when:

- A target operation is not classified.
- A classified faker route disappears.
- A missing route is added without being reviewed and reclassified.

`go test ./...` includes this check. The known-gap baseline prevents the existing incompatibilities from making every build red while still forcing deliberate review whenever the surface changes.
