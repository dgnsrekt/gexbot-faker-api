# GEX Faker API

Mock API server that replays historical GexBot market data with per-API-key sequential playback. Includes REST API, WebSocket streaming, CLI downloader, and scheduled download daemon.

<img width="2752" height="1536" alt="image" src="https://github.com/user-attachments/assets/ae1314af-56b9-4bd3-b030-1e44e79205d6" />

https://github.com/user-attachments/assets/e7b82f7a-311c-493f-bd48-32e7ba572196

NOTE: This only downloads and replays GexBot market data.

Optional authenticated monitoring is documented in [OBSERVABILITY.md](OBSERVABILITY.md).

## Features

- REST API with Swagger UI documentation at `/docs`
- WebSocket streaming (5 hubs, Azure Web PubSub compatible)
- Per-API-key playback position tracking
- Hot reload data dates without server restart
- Sync Broadcast System for external service synchronization
- CLI for downloading historical data from GexBot
- Daemon for scheduled automatic downloads
- Push notifications via ntfy.sh on download completion
- Docker deployment ready

## Quick Start

### Prerequisites

- [Go 1.24+](https://go.dev/doc/install) - verify with `go version`
- [just](https://github.com/casey/just#installation) command runner - verify with `just --version`
- [Docker](https://docs.docker.com/get-docker/) - verify with `docker --version`
- [GexBot API key](https://www.gexbot.com/pricing) with **Quant Subscription** (required for downloading data)

### 1. Clone and Configure Environment

```bash
git clone git@github.com:dgnsrekt/gexbot-faker-api.git
cd gexbot-faker-api

# Copy example env and add your GexBot API key
cp gexbot.example.env .env
# Edit .env and set GEXBOT_API_KEY=your_api_key_here
```

### 2. Create Download Config

Create a custom config to select your tickers, packages, and categories:

```bash
cp configs/default.yaml configs/custom.yaml
```

Edit `configs/custom.yaml` to customize:
- **tickers**: Enable/disable tickers (SPX, NDX, SPY, etc.)
- **packages**: Enable/disable data packages (state, classic, orderflow)
- **categories**: Enable/disable specific data types within each package

### 3. Download Initial Data

```bash
# Download last 7 days of market data (adjust number as needed)
just download-lookback 7
```

> **Note**: The downloader automatically skips weekends and market holidays.

### 4. Start the API

**With Docker (Recommended):**

```bash
just up        # Start API server and daemon
just logs      # View logs
```

**Or run locally:**

```bash
just serve-gex-faker              # Build and run server
open http://localhost:8080/docs   # Access API docs
```

## Components

### API Server

REST API serving historical GEX data with sequential playback per API key.

**Endpoints** (see `/docs` for full reference):

- `/{ticker}/classic/{aggregation}` - Classic GEX chain data
- `/{ticker}/state/{type}` - State GEX profiles and Greeks
- `/{ticker}/orderflow/orderflow` - Orderflow metrics
- `/available-data/{date}` - Discover available data for a date
- `/download/{date}/{ticker}/links` - Get all download links for a date/ticker
- `/download/{date}/{ticker}/classic/{aggregation}` - Download classic data
- `/download/{date}/{ticker}/state/{type}` - Download state data
- `/download/{date}/{ticker}/orderflow` - Download orderflow data
- `/negotiate` - WebSocket connection URLs
- `/health`, `/tickers`, `/available-dates` - Server info
- `/reload-date` - Hot reload data for a different date
- `/reset-cache` - Reset playback positions
- `/seek-to-timestamp` - Seek positions to a specific timestamp

**Key behavior**: Each API key maintains independent playback position. Data advances on each request.

### Hot Reload

Switch data dates at runtime without restarting the server:

```bash
# Reload to a different date
curl -X POST http://localhost:8080/reload-date \
  -H "Content-Type: application/json" \
  -d '{"date": "2025-12-04"}'
```

**Response:**
```json
{
  "status": "success",
  "previous_date": "2025-11-28",
  "new_date": "2025-12-04",
  "loaded_at": "2025-12-27T15:30:00Z",
  "files_loaded": 45
}
```

**Behavior:**
- Validates the date exists before unloading current data
- Pauses WebSocket streaming during reload
- Resets all cache positions to 0 for clean playback
- Returns 400 for invalid/missing dates, 409 if reload already in progress

### Cache Management

Control playback positions for testing and synchronization.

**Reset positions:**
```bash
# Reset all positions for an API key
curl -X POST "http://localhost:8080/reset-cache?key=mykey"

# Reset ALL positions (all keys)
curl -X POST http://localhost:8080/reset-cache
```

**Seek to timestamp:**
```bash
# Seek all positions for an API key to a specific timestamp
curl -X POST http://localhost:8080/seek-to-timestamp \
  -H "Content-Type: application/json" \
  -d '{"timestamp": 1767191500, "key": "mykey"}'
```

**Response:**
```json
{
  "status": "success",
  "message": "Seeked 45 positions to timestamp 1767191500",
  "positions_set": 45,
  "details": [
    {"data_key": "SPX/classic/gex_zero", "index": 98, "timestamp": 1767191500}
  ]
}
```

**Use case:** Sync with external services by seeking to a broadcast timestamp from the Sync Broadcast System.

### WebSocket Streaming

Real-time data streaming via 5 specialized hubs:

| Hub                 | Data Type                                 |
| ------------------- | ----------------------------------------- |
| `orderflow`         | DEX, GEX, convexity, vanna, charm metrics |
| `classic`           | Classic GEX chain                         |
| `state_gex`         | State GEX profiles                        |
| `state_greeks_zero` | Greeks (0DTE)                             |
| `state_greeks_one`  | Greeks (1DTE+)                            |

See [WEBSOCKET.md](WEBSOCKET.md) for protocol details.

### Sync Broadcast System

SSE-based market time broadcast for synchronizing external services with the faker's playback position. External services subscribe to receive position updates and can seek their own data to match.

**Endpoint:** `GET /sync/stream?key={api-key}`

**Enable via environment:**
```bash
SYNC_BROADCAST_SYSTEM_ENABLED=true
```

**Example subscription:**
```bash
curl -N "http://localhost:8080/sync/stream?key=my-api-key"
```

**Response (SSE stream):**
```
event: snapshot
id: 1
data: {"broadcaster_id":"gexbot-faker","data_date":"2025-12-05","cache_mode":"exhaust","timestamp":1766870060074,"sequence":1,"positions":[{"cache_key":"SPX/classic/my-a****","index":42,"data_length":23375,"data_timestamp":1764945003,"exhausted":false}]}

event: batch
id: 2
data: {...positions updated every interval...}
```

**Position fields:**
- `cache_key`: Cache key path with masked API key (REST or WebSocket format)
- `index`: Current playback position
- `data_length`: Total records available
- `data_timestamp`: Unix timestamp from the data at current position
- `exhausted`: True if position has reached end (exhaust mode only)

**Use case:** Other faker-like services with timestamp-indexed data can subscribe and seek their own data to match the broadcaster's market time.

### Python WebSocket Client

Use [quant-python-sockets](https://github.com/nfa-llc/quant-python-sockets) to connect to the WebSocket feeds.

**To use with faker API**, change `main.py` line 36:

```python
# Original (production):
BASE_URL = "https://api.gexbot.com"

# Change to (faker):
BASE_URL = "http://localhost:8080"
```

Then run:

```bash
export GEXBOT_API_KEY=test123  # Faker accepts any key
python main.py
```

### Downloader CLI

Download historical data from the GexBot API.

```bash
# Single date
./bin/gexbot-downloader download 2025-11-14

# Date range
./bin/gexbot-downloader download 2025-11-01 2025-11-14

# Custom tickers/packages
./bin/gexbot-downloader download --tickers SPX,NDX --packages state 2025-11-14

# Preview (dry run)
./bin/gexbot-downloader download --dry-run 2025-11-14
```

EOD reports are retained as the canonical compressed archive under
`data/eod/YYYY-MM-DD/TICKER/`. JSONL is materialized only when the server
needs a date.

```bash
# Convert and verify existing JSONL; --prune removes it after verification
./bin/gexbot-downloader eod pack 2026-07-17 --ticker SPY --prune

# Verify every archive or restore a date for replay
./bin/gexbot-downloader eod verify all
./bin/gexbot-downloader eod materialize 2026-07-17
```

### Daemon Service

Automated EOD downloads with market-day awareness. Reports are retried every
five minutes; after 8:00 PM ET the daemon falls back to individual downloads.

| Variable                 | Default          | Description             |
| ------------------------ | ---------------- | ----------------------- |
| `DAEMON_SCHEDULE_HOUR`   | 17               | First EOD attempt (ET)   |
| `DAEMON_SCHEDULE_MINUTE` | 5                | First EOD attempt minute |
| `DAEMON_TIMEZONE`        | America/New_York | Timezone                |
| `DAEMON_RUN_ON_STARTUP`  | true             | Check/download on start |

### Download paths & the data lifecycle

Data reaches the replayer two different ways, and they land in **different
states** in the Studio Data Library. Understanding this explains why a date you
just downloaded may show **Materialize** instead of **Load**:

1. **EOD report** — the daemon's default, and the CLI. GexBot serves a
   pre-packed, per-ticker zip (`GET /v2/hist/eod/{ticker}`) whose members are the
   gzipped per-category datasets. It is stored as-is as the **canonical
   compressed archive** under `data/eod/YYYY-MM-DD/TICKER/`, and the downloader
   generates a `…zip.manifest.json` sidecar **locally** when it verifies the
   archive (`eod.Verify`) — the manifest is ours, not part of GexBot's ZIP.
   Because it is archive-only (no JSONL yet), a freshly EOD-downloaded date shows
   **`archived`** and must be **Materialized** (unpacked to JSONL) before it can
   be **Loaded**. This keeps disk usage lean.

2. **Individual `/hist` download** — the Studio "Download" screen. Fetches
   per-category JSON from `GET /v2/hist/{ticker}/{package}/{category}/{date}`,
   auto-converts it to JSONL (`auto_convert_to_jsonl`), then packs the same
   archive. The Studio worker also writes the `.eod-materialized` marker, so the
   date shows **`ready` / Load** immediately — no separate Materialize step.

   > The daemon's after-hours fallback uses this **same** `/hist` path, but it
   > does **not** yet write the marker — so a daemon fallback leaves JSONL on
   > disk while still showing **`archived` / Materialize** (tracked in
   > [#38](https://github.com/dgnsrekt/gexbot-faker-api/issues/38)). Only the
   > Studio download currently becomes `ready` automatically.

**Materialize** unpacks an archive's gzipped members back into
`data/YYYY-MM-DD/TICKER/PACKAGE/CATEGORY.jsonl` and writes an
`.eod-materialized` marker. The server materializes on demand when it loads a
date. If TTL cleanup is enabled (`GEXBOT_OUTPUT_AUTO_CLEANUP=true`, window
`GEXBOT_OUTPUT_CLEANUP_AFTER_DAYS`; **disabled by default**), the daemon later
evicts idle materialized JSONL back to archive-only. The archive is always the
durable source of truth.

| Library state | Button | Meaning |
| ------------- | ------ | ------- |
| `archived`    | **Materialize** | Only the EOD archive is on disk (typical for daemon downloads) — unpack it first |
| `ready`       | **Load**        | JSONL is materialized on disk — loading is instant |
| `loaded`      | *Loaded*        | Currently being served by the API |

> **`/hist` gzip transition:** GexBot is moving the historical endpoint to serve
> **only** gzip-compressed payloads (`Content-Encoding: gzip`). This is
> transport compression of the *same* per-category JSON — **not** a new pack
> format, and unrelated to the EOD zip above. The downloader is already
> compatible: it leaves Go's transparent gzip on and never sets `Accept-Encoding`
> itself, so responses are negotiated and decompressed automatically
> (`internal/api/client.go`).

### Coverage alerts

After each successful daemon download the data is checked for **coverage
regressions** (`internal/coverage`) and a `high`-priority ntfy alert is sent if
anything looks off:

- **Snapshot drop** — a ticker's intraday snapshot count (one row per snapshot,
  read from the manifest) falls >10% below its 20-day median. This is what a
  silent source change looks like (e.g. a feed thinning SPX/NDX sampling).
- **Session shape** — the day opens later than ~09:30 ET, closes earlier than
  ~16:00 ET, or has an intraday gap over 120s (a truncated feed / outage).

Findings are always logged; the ntfy alert only fires when notifications are
enabled. The same snapshot metric is surfaced visually in the Studio Data
Library (coverage sparkline + per-row deviation badge) and exported to
Prometheus by the daemon — `faker_daemon_snapshots{ticker}` (latest per-ticker
snapshot count) and `faker_daemon_coverage_findings_total{kind}`. The Studio
**Monitoring** screen queries Prometheus server-side (`PROMETHEUS_URL`) and
renders the panels natively — no Grafana needed.

### Push Notifications (ntfy)

Both the daemon and CLI downloader support push notifications via [ntfy.sh](https://ntfy.sh) when downloads complete or fail.

| Variable        | Default           | Description                          |
| --------------- | ----------------- | ------------------------------------ |
| `NTFY_ENABLED`  | false             | Enable push notifications            |
| `NTFY_SERVER`   | https://ntfy.sh   | ntfy server URL (supports self-hosted) |
| `NTFY_TOPIC`    | *(required)*      | Topic name for notifications         |
| `NTFY_PRIORITY` | default           | Priority: min, low, default, high, urgent |
| `NTFY_TAGS`     | package           | Comma-separated emoji tags           |
| `NTFY_TOKEN`    | *(optional)*      | Access token for private topics      |

**Quick setup:**

```bash
# In .env
NTFY_ENABLED=true
NTFY_TOPIC=my-gexbot-downloads
```

Subscribe to notifications at `https://ntfy.sh/my-gexbot-downloads` or use the ntfy app.

## Configuration

### Server Environment Variables

| Variable                         | Default  | Description                                 |
| -------------------------------- | -------- | ------------------------------------------- |
| `PORT`                           | 8080     | HTTP server port                            |
| `DATA_DIR`                       | ./data   | Data directory path                         |
| `DATA_DATE`                      | latest   | Date to load (YYYY-MM-DD or "latest")       |
| `DATA_MODE`                      | memory   | `memory` (fast) or `stream` (low RAM)       |
| `CACHE_MODE`                     | exhaust  | `exhaust` (404 at end) or `rotation` (loop) |
| `WS_ENABLED`                     | true     | Enable WebSocket streaming                  |
| `WS_STREAM_INTERVAL`             | 1s       | Broadcast interval                          |
| `WS_GROUP_PREFIX`                | blue     | Prefix for WebSocket group names            |
| `SYNC_BROADCAST_SYSTEM_ENABLED`  | false    | Enable SSE sync broadcast endpoint          |
| `SYNC_BROADCAST_SYSTEM_ID`       | hostname | Broadcaster identifier                      |
| `SYNC_BROADCAST_SYSTEM_INTERVAL` | 1s       | Position broadcast interval                 |

### Downloader Configuration

Create `configs/default.yaml` or set `GEXBOT_API_KEY`:

```yaml
api:
  api_key: "${GEXBOT_API_KEY}"
  timeout_sec: 300
  retry_count: 3

download:
  workers: 3
  rate_per_second: 2
  resume_enabled: true

output:
  directory: "data"
  auto_convert_to_jsonl: true
```

## Data Reference

### Packages and Categories

| Package   | Categories                                                                                                              |
| --------- | ----------------------------------------------------------------------------------------------------------------------- |
| state     | gex_full, gex_zero, gex_one, delta_zero, gamma_zero, delta_one, gamma_one, vanna_zero, vanna_one, charm_zero, charm_one |
| classic   | gex_full, gex_zero, gex_one                                                                                             |
| orderflow | orderflow                                                                                                               |

### Tickers

Indexes: SPX, NDX, RUT, VIX
ETFs: SPY, QQQ, IWM
Futures: ES_SPX, NQ_NDX

### Data Directory Structure

```
data/
├── eod/
│   └── 2025-11-14/
│       └── SPX/
│           ├── eod_report_SPX_2025-11-14.zip
│           └── eod_report_SPX_2025-11-14.zip.manifest.json
└── 2025-11-14/                 # materialized replay cache
    └── SPX/
        ├── classic/gex_zero.jsonl
        ├── state/delta_zero.jsonl
        └── orderflow/orderflow.jsonl
```

The ZIP is the canonical archive. Its members use GEXBot's
`TICKER/package/category/date_ticker_package_category.json.gz` layout and
contain JSON arrays. JSONL remains uncompressed because stream playback needs
record offsets that gzip cannot provide efficiently.

## Development

```bash
# Build
just build                          # Downloader
just build-gex-faker                # Server

# Code generation
just generate-gex-faker-api-spec    # OpenAPI → Go
just generate-protos                # Protobuf → Go

# Test and lint
just test
just lint

# Docker
just up                             # Start containers
just down                           # Stop containers
just logs                           # View logs
```

## License

See [LICENSE](LICENSE) file.

## Contact Information

Telegram = Twitter = Tradingview = Discord = @dgnsrekt

Email = dgnsrekt@pm.me

Note: It may take me a few days to reply.
