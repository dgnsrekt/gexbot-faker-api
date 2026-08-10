# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

GEX Faker API is a Go server that replays historical options/derivatives data from the GexBot API. It serves as a mock API for development and testing, providing both REST endpoints and WebSocket streaming. The project also includes a CLI downloader for fetching historical data from the real GexBot API.

## Build and Development Commands

```bash
# Build
just build                          # Build downloader binary
just studio-build                   # Build the Studio web UI (web/ → web/dist, embedded)
just build-gex-faker                # Build server binary (runs studio-build + generates API code)

# Run
just serve-gex-faker                # Build and run server
PORT=8080 DATA_DATE=2025-11-24 go run ./cmd/server  # Run with env overrides (uses last-built web/dist)

# Code Generation
just generate-gex-faker-api-spec    # Generate Go code from OpenAPI spec
just generate-protos                # Generate protobuf code for WebSocket

# Tests and Lint
just test                           # Run all tests
just lint                           # Run golangci-lint
go test -v ./internal/config/...    # Run single package tests
```

## Environment Variables (Server)

| Variable | Default | Description |
|----------|---------|-------------|
| PORT | 8080 | HTTP server port |
| DATA_DIR | ./data | Directory containing JSONL data files |
| DATA_DATE | latest | Date folder to load (YYYY-MM-DD or "latest") |
| DATA_MODE | memory | Data loading mode: "memory" or "stream" |
| CACHE_MODE | exhaust | Playback behavior: "exhaust" (stop at end) or "rotation" (loop) |
| WS_ENABLED | true | Enable WebSocket streaming |
| WS_STREAM_INTERVAL | 1s | Interval between WebSocket broadcasts |

## Architecture

### Entry Points
- `cmd/server/main.go` - REST API + WebSocket server
- `cmd/downloader/main.go` - CLI for downloading historical data from GexBot API
- `cmd/daemon/main.go` - Background service for scheduled downloads

### Code Generation Pipeline
1. `api/openapi.yaml` - OpenAPI 3.0 spec defines all endpoints
2. `api/generate.go` - Contains `//go:generate` directive for oapi-codegen
3. `internal/api/generated/server.gen.go` - Auto-generated strict server interface

**When adding/modifying endpoints:**
1. Edit `api/openapi.yaml`
2. Run `just generate-gex-faker-api-spec`
3. Implement the new method in `internal/server/handlers.go` (must satisfy `StrictServerInterface`)

### WebSocket Architecture
Five WebSocket hubs stream different data types:
- `orderflow` - Order flow metrics
- `classic` - Classic GEX data
- `state_gex` - State GEX profiles
- `state_greeks_zero` - Greek profiles (0DTE: delta_zero, gamma_zero, etc.)
- `state_greeks_one` - Greek profiles (1DTE+: delta_one, gamma_one, etc.)

Protocol: Azure Web PubSub-compatible with Zstandard-compressed Protobufs.

**Group naming convention:** `blue_{TICKER}_{hub_type}_{category}` (e.g., `blue_SPX_classic_gex_zero`)

### Data Loading
- Data stored as JSONL files in `data/{date}/{ticker}/{package}/{category}.jsonl`
- `DataLoader` interface (`internal/data/loader.go`) provides random access
- Two modes: `MemoryLoader` (loads all to RAM) or `StreamLoader` (reads from disk)
- `IndexCache` tracks per-API-key playback positions

### GEX Faker Studio (web UI)
An LM-Studio-style web UI served by the same server at `/studio`, for managing the
faker without curl/env-vars.
- `web/` - Vite + Svelte 5 + TypeScript SPA. `npm run build` (or `just studio-build`)
  writes `web/dist`, which is embedded via `//go:embed` in `web/embed.go`
  (`web.Dist`) and served by `internal/server/studio.go`. A checked-in
  `web/dist/.gitkeep` keeps the Go package compilable before a build; `web/dist`
  is otherwise gitignored (build it, don't commit it). Docker builds it in a Node
  stage (see `Dockerfile`).
- Read-only control-plane JSON at `/studio/api/{status,config,hubs,library,keys,endpoints}`
  (`internal/server/studio_handlers.go`); the SPA also calls the existing open
  control endpoints (`/reload-date`, `/reset-cache`, `/current-date`, `/tickers`, ...).
- Screens: Local server, Data library (Materialize archived dates in the background →
  Load ready ones), Live streams (hub stats + group-name builder), Settings (effective
  env vars), Logs (live feed from Loki). Download is the remaining Phase-2 placeholder.
- **Logs** (`internal/server/studio_logs.go`): `GET /studio/api/logs` is an SSE proxy —
  the server queries Loki (`LOKI_URL`, default `http://loki:3100`) over the compose
  network and streams parsed lines, so the browser never talks to Loki. **Loki + Alloy
  run in the DEFAULT compose stack** (not the `observability` profile); Grafana/
  Prometheus/Uptime-Kuma/Caddy stay opt-in. If `LOKI_URL` is unset (e.g. `go run`), Logs
  shows a "start the observability stack" message.
- **Auth**: `STUDIO_AUTH_TOKEN` gates all `/studio` routes behind HTTP Basic (any
  username, password = the token) via `studioAuthMiddleware`. Empty = open (local
  dev). Set it whenever the API port is reachable beyond a trusted host — the
  Studio exposes control endpoints and container logs. Log lines are redacted for
  signed-URL query strings at the source (`internal/redact`) and again in the logs
  proxy.
- When adding a Studio endpoint: add a handler in `studio_handlers.go`, register it
  in `RegisterStudioRoutes`, add a typed wrapper in `web/src/lib/api.ts`, then
  rebuild the UI. New backend accessors added for the UI: `ws.Hub.ClientCount()`,
  `data.IndexCache.AllPositions()`, `eod.ListArchives()`.

### Key Packages
- `internal/server/` - HTTP router, handlers, Swagger UI, Studio UI + endpoints
- `internal/ws/` - WebSocket hubs, streamers, negotiate handler, protobuf encoding
- `internal/data/` - Data loading and caching
- `internal/config/` - Configuration loading and validation
- `web/` - GEX Faker Studio SPA (embedded)
- `proto/` - Protobuf definitions for WebSocket messages

## Data Packages and Categories

| Package | Categories |
|---------|------------|
| state | gex_full, gex_zero, gex_one, delta_zero, gamma_zero, delta_one, gamma_one, vanna_zero, charm_zero, vanna_one, charm_one |
| classic | gex_full, gex_zero, gex_one |
| orderflow | orderflow |

## Docker

```bash
just up           # Build and start all containers
just down         # Stop containers
just logs         # Follow all logs
just api-logs     # Follow API logs only
```
