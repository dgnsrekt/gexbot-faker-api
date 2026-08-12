---
type: Guide
title: The GEX Faker Studio
description: A tour of the embedded web UI at /studio and its seven screens — Server, Download, Data library, Live streams, Logs, Monitoring, and Settings.
tags: [studio, web-ui, screens, guide]
timestamp: 2026-08-11T00:00:00Z
---

# The GEX Faker Studio

**GEX Faker Studio** is a web UI embedded in the server at **`/studio`** — an
LM-Studio-style control panel for running the faker without curl or environment
variables. It is served by the same binary (no separate process) and reads/writes
through the server's control-plane endpoints.

Open it at **http://localhost:8080/studio**. The sidebar shows live server status
(running/stopped, the date being replayed, disk used) and the reachable base URL.

## The seven screens

### Server
The default screen: what your clients are talking to. Server status, the loaded
date, data/cache modes, and the base URL to hand to a client.

### Download data
Fetch historical market days from GexBot — a calendar plus a batch builder for
picking **dates**. Coverage (tickers, packages, categories) is **YAML-authoritative
and read-only**: it comes from the same downloader YAML the daemon uses
(`GEXBOT_DOWNLOADER_CONFIG`), shown with a "Configured by …" label, so manual and
scheduled downloads never diverge. Needs `GEXBOT_API_KEY` on the server and
**degrades gracefully** ("set GEXBOT_API_KEY") when it is unset. Downloads land as
EOD archives; see [download data](download-data.md).

### Data library
The EOD archives on this machine. Each date shows a state — **`archived`**,
**`ready`**, or **`loaded`** — with a **Materialize** or **Load** button. This is
where you turn a downloaded archive into a served date; see
[materialize & load](materialize-load.md). It also shows a coverage sparkline and
per-row deviation badge from the daemon's coverage checks. A **Load a span**
control (from/to dates) loads a contiguous multi-day range as one cross-day dataset
in one click — every day in the span badges as **loaded**, and replay/seek cross
day boundaries (see [multi-day range](materialize-load.md)).

### Live streams
The five WebSocket hubs with live client counts and active groups, plus a
**group-name builder**: pick a ticker and feed, copy the exact group name your
client subscribes to. See [WebSocket streaming](websockets.md).

### Logs
A live feed of server, downloader, and daemon logs. Backed by an SSE proxy that
queries **Loki** server-side (`LOKI_URL`); the browser never talks to Loki. Shows
a degrade message when `LOKI_URL` is unset (e.g. `go run`).

### Monitoring
Metrics from **Prometheus** (`PROMETHEUS_URL`), rendered as native time-series
panels — no Grafana. Request rate/latency, WebSocket clients, per-ticker snapshot
counts, and Prometheus alert-rule state. Degrades when Prometheus is unreachable.
See [docker & observability](docker-observability.md).

### Settings
Every environment variable in plain language — the effective server config,
explained. Read-only. A separate **Daemon** section (Schedule, Downloads, Packages,
Cleanup, Notifications) shows the daemon's sanitized effective config + runtime
state, proxied from the daemon over the compose network (`DAEMON_URL`); it shows
**"Daemon unavailable"** when the daemon is down, and never exposes secrets (no API
key or ntfy token).

## Remote access and auth

By default Docker binds the API to `127.0.0.1`, so `docker compose up` never
exposes the Studio to the LAN. For remote access set `HOST_BIND=0.0.0.0` **and**
`STUDIO_AUTH_TOKEN` (and front it with TLS). The token gates all `/studio` routes
behind HTTP Basic (any username, password = the token); empty = open (local dev).
See [configuration](configuration.md).
