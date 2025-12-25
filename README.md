# GEX Faker API

Mock API server that replays historical GexBot market data with per-API-key sequential playback. Includes REST API, WebSocket streaming, CLI downloader, and scheduled download daemon.

<img width="2752" height="1536" alt="image" src="https://github.com/user-attachments/assets/ae1314af-56b9-4bd3-b030-1e44e79205d6" />

https://github.com/user-attachments/assets/e7b82f7a-311c-493f-bd48-32e7ba572196

NOTE: This only downloads and replays GexBot market data.

## Features

- REST API with Swagger UI documentation at `/docs`
- WebSocket streaming (5 hubs, Azure Web PubSub compatible)
- Per-API-key playback position tracking
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

**Key behavior**: Each API key maintains independent playback position. Data advances on each request.

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

### Daemon Service

Automated daily downloads with market day awareness.

| Variable                 | Default          | Description             |
| ------------------------ | ---------------- | ----------------------- |
| `DAEMON_SCHEDULE_HOUR`   | 20               | Hour to run (0-23)      |
| `DAEMON_SCHEDULE_MINUTE` | 0                | Minute to run           |
| `DAEMON_TIMEZONE`        | America/New_York | Timezone                |
| `DAEMON_RUN_ON_STARTUP`  | true             | Check/download on start |

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

| Variable             | Default | Description                                 |
| -------------------- | ------- | ------------------------------------------- |
| `PORT`               | 8080    | HTTP server port                            |
| `DATA_DIR`           | ./data  | Data directory path                         |
| `DATA_DATE`          | latest  | Date to load (YYYY-MM-DD or "latest")       |
| `DATA_MODE`          | memory  | `memory` (fast) or `stream` (low RAM)       |
| `CACHE_MODE`         | exhaust | `exhaust` (404 at end) or `rotation` (loop) |
| `WS_ENABLED`         | true    | Enable WebSocket streaming                  |
| `WS_STREAM_INTERVAL` | 1s      | Broadcast interval                          |
| `WS_GROUP_PREFIX`    | blue    | Prefix for WebSocket group names            |

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
└── 2025-11-14/
    └── SPX/
        ├── classic/
        │   └── gex_zero.jsonl
        ├── state/
        │   ├── gex_zero.jsonl
        │   └── delta_zero.jsonl
        └── orderflow/
            └── orderflow.jsonl
```

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
