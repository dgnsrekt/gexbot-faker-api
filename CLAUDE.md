# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

GEX Faker API is a Go server that replays historical options/derivatives data from the GexBot API. It serves as a mock API for development and testing, providing both REST endpoints and WebSocket streaming. The project also includes a CLI downloader for fetching historical data from the real GexBot API.

## Build and Development Commands

```bash
# Build
just build                          # Build downloader binary
just build-gexfakercli              # Build the gexfakercli agent CLI
just studio-build                   # Build the Studio web UI (web/ → web/dist, embedded)
just docs-build                     # Build the guides site (website/ → website/dist, embedded at /guides)
just demos-render                   # Render CLI demo GIFs (VHS) into website/public/demos
just build-gex-faker                # Build server binary (runs studio-build + docs-build + generates API code)

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
- `cmd/gexfakercli/main.go` - JSON-first agent CLI over the faker (client + self-installing skill + auto-setup)
- `cmd/synctickers/main.go` - Refresh `internal/config/tickers.json` from GexBot's `/tickers` (CI drift check)

### Control-plane auth & multi-day range
- **Mutating control routes** (`/load-range`, `/reload-date`, `/reset-cache`) are gated
  behind `STUDIO_AUTH_TOKEN` via `controlAuthMiddleware`/`isMutatingControlPath`
  (`internal/server/server.go`) — same Basic/Bearer check as the Studio; empty = open.
  Reads and `/seek-to-timestamp` stay open; data routes keep any-token auth.
- **Multi-day range replay**: `/load-range` (span → one cross-day dataset, async job),
  `/current-range`, `/range-coverage`, range-aware `/seek-to-timestamp`; after-range-end
  policy is `RANGE_END_POLICY` (`clamp`|`error`). Spec: `docs/SPEC-multiday-range-replay.md`.

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
- Screens: Download data (calendar + batch builder → fetch market days from GEXbot),
  Local server, Data library (Materialize archived dates in the background → Load ready
  ones), Live streams (hub stats + group-name builder), Settings (effective env vars),
  Logs (live feed from Loki). All screens are now live (no placeholders).
- **Download** (`internal/server/studio_download.go`): needs `GEXBOT_API_KEY` on the
  server (guarded — Download degrades with a "set GEXBOT_API_KEY" message when unset).
  The per-date orchestration lives in `internal/downloadjob` (extracted from the daemon,
  shared by both binaries); `download.Manager.OnProgress` feeds per-task progress.
  Downloaded dates land as EOD archives → the Library shows them `archived` → Materialize
  → Load. `/studio/api/calendar` powers the month grid (reuses `isMarketDay`).
- **Logs** (`internal/server/studio_logs.go`): `GET /studio/api/logs` is an SSE proxy —
  the server queries Loki (`LOKI_URL`, default `http://loki:3100`) over the compose
  network and streams parsed lines, so the browser never talks to Loki. If `LOKI_URL`
  is unset (e.g. `go run`), Logs shows a degrade message.
- **Monitoring** (`internal/server/studio_metrics.go`): `GET /studio/api/metrics/{query,range,alerts}`
  proxy PromQL to Prometheus (`PROMETHEUS_URL`, default `http://prometheus:9090`) — same
  server-side model as Logs; the browser never talks to Prometheus. The Monitoring screen
  renders panels natively (no Grafana). Degrades when `PROMETHEUS_URL` is unset/unreachable.
- **Observability stack**: **Prometheus + Loki + Alloy run in the DEFAULT compose stack**;
  Studio queries both server-side. Grafana, Uptime-Kuma, and the Caddy gateway were removed
  (issue #46) — Studio is the single pane; uptime monitoring is a fleet-level concern.
- **Auth / exposure**: Compose binds the API port to `127.0.0.1` by default
  (`HOST_BIND`), so `docker compose up` never exposes the Studio to the LAN. For
  remote access set `HOST_BIND=0.0.0.0` **and** `STUDIO_AUTH_TOKEN` (and front it
  with TLS — Basic/Bearer over plain HTTP is plaintext). `STUDIO_AUTH_TOKEN` gates
  all `/studio` routes behind HTTP Basic (any username, password = the token) via
  `studioAuthMiddleware`; empty = open (local dev). Log lines are redacted for
  signed-URL query strings at the source (`internal/redact`) and again in the logs
  proxy.
- When adding a Studio endpoint: add a handler in `studio_handlers.go`, register it
  in `RegisterStudioRoutes`, add a typed wrapper in `web/src/lib/api.ts`, then
  rebuild the UI. New backend accessors added for the UI: `ws.Hub.ClientCount()`,
  `data.IndexCache.AllPositions()`, `eod.ListArchives()`.

### Documentation (knowledge bundle + guides site)
- **`knowledge/`** is the OKF v0.1 Markdown **source of truth** for docs (humans +
  agents). Edit topics there; keep the OKF frontmatter (`type`/`title`/
  `description`/`tags`/`timestamp`) and update `knowledge/log.md`.
- **`llms.txt` / `llms-full.txt`** (repo root, committed, also served at
  `/llms.txt`) are **generated** from `knowledge/` by
  `website/scripts/gen-llms.mjs` — run `just docs-llms` after editing topics; do
  not hand-edit them.
- **`website/`** is an Astro Starlight site that renders `knowledge/`. A prebuild
  step (`sync-knowledge.mjs`) copies `knowledge/*.md` into `website/src/content/
  docs/` (strips the body H1, keeps frontmatter title). Two builds from one
  source via `DOCS_BASE`: `npm run build:embed` (base `/guides`, embedded via
  `website/embed.go` + served by `internal/server/guides.go` at `/guides`) and
  `npm run build:pages` (base `/gexbot-faker-api`, deployed to GitHub Pages by
  `.github/workflows/docs.yml`). `just docs-build` builds the embedded variant and
  is folded into `build-gex-faker`.

### Demos (VHS)
- **`demos/cli/*.tape`** are charmbracelet [VHS](https://github.com/charmbracelet/vhs)
  scripts that render CLI sessions to GIFs (theme matches the guides brand). Run
  **`just demos-render`** (needs `vhs` + `ttyd` + `ffmpeg` and a faker on `:8080`)
  → GIFs land in `website/public/demos/*.gif` (committed), served by the guides at
  `/guides/demos/` and embedded in the landing + README. See `demos/README.md`.

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
