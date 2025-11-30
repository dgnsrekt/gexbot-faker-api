# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

GEX Faker API is a Go server that replays historical options/derivatives data from the GexBot API. It serves as a mock API for development and testing, providing both REST endpoints and WebSocket streaming. The project also includes a CLI downloader for fetching historical data from the real GexBot API.

## Build and Development Commands

```bash
# Build
just build                          # Build downloader binary
just build-gex-faker                # Build server binary (auto-generates API code)

# Run
just serve-gex-faker                # Build and run server
PORT=8080 DATA_DATE=2025-11-24 go run ./cmd/server  # Run with env overrides

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

### Key Packages
- `internal/server/` - HTTP router, handlers, Swagger UI
- `internal/ws/` - WebSocket hubs, streamers, negotiate handler, protobuf encoding
- `internal/data/` - Data loading and caching
- `internal/config/` - Configuration loading and validation
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
