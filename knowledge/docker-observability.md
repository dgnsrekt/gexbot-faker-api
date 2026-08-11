---
type: Guide
title: Docker and observability
description: The default docker compose stack (API, daemon, Prometheus, Loki, Alloy), how port binding and Studio auth work, and how the Studio Logs and Monitoring screens query Loki and Prometheus server-side.
tags: [docker, compose, prometheus, loki, alloy, observability, monitoring]
timestamp: 2026-08-11T00:00:00Z
---

# Docker & observability

## The compose stack

`docker compose up -d` (or `just up`) starts the **default stack**:

| Service | Role |
| --- | --- |
| `gex-faker-api` | The API server + Studio |
| `gex-daemon` | Scheduled downloads + coverage checks |
| `prometheus` | Scrapes metrics from the API (`:9090`) and daemon (`:9091`) |
| `loki` | Stores logs |
| `alloy` | Ships container logs into Loki |

Prometheus, Loki, and Alloy run in the **default** stack so the Studio's Logs and
Monitoring screens work out of the box. Grafana, Uptime-Kuma, and a Caddy gateway
were removed (issue #46) — the Studio is the single pane of glass.

Common recipes: `just up`, `just down`, `just logs`, `just api-logs`,
`just daemon-logs`, `just restart-api`, `just restart-daemon`.

## Port binding and exposure

The API port is bound to loopback by default:

```
"${HOST_BIND:-127.0.0.1}:${HOST_PORT:-8080}:8080"
```

So `docker compose up` never exposes the Studio to the LAN. For remote access set
**`HOST_BIND=0.0.0.0`** and **`STUDIO_AUTH_TOKEN`** (and front it with TLS —
Basic/Bearer over plain HTTP is plaintext). The container always listens on
`8080` internally.

## Server-side telemetry proxies

The browser never talks to Loki or Prometheus directly — the server proxies both,
so no extra ports need exposing and secrets stay server-side:

- **Logs** — `GET /studio/api/logs` is an SSE proxy that queries Loki
  (`LOKI_URL`, default `http://loki:3100`) and streams parsed lines. Log lines are
  redacted for signed-URL query strings at the source (`internal/redact`) and
  again in the proxy.
- **Monitoring** — `GET /studio/api/metrics/{query,range,alerts}` proxy PromQL to
  Prometheus (`PROMETHEUS_URL`, default `http://prometheus:9090`). The Monitoring
  screen renders the panels natively.

Both screens **degrade gracefully** with a message when their backend URL is unset
or unreachable (e.g. running the server via `go run` without the stack).

## Coverage metrics

The daemon exports per-ticker coverage to Prometheus —
`faker_daemon_snapshots{ticker}` (latest snapshot count) and
`faker_daemon_coverage_findings_total{kind}` — surfaced on the Monitoring screen
and as a sparkline in the Data Library. See [daemon](daemon.md) for the alerting
side.
