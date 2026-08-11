---
type: Reference
title: Configuration
description: The environment variables that configure the faker — the server (port, data, cache, WebSocket, sync, auth, observability), Docker/compose, the daemon, ntfy push, TTL cleanup, and the downloader YAML config.
tags: [configuration, environment, env-vars, settings, reference]
timestamp: 2026-08-11T00:00:00Z
---

# Configuration

The faker is configured by environment variables. This page covers the ones you
are most likely to set; the Studio **Settings** screen shows the *effective*
server config in plain language, and `gexbot.example.env` is the annotated
starting point (`cp gexbot.example.env .env`).

## Server

| Variable | Default | Description |
| --- | --- | --- |
| `PORT` | 8080 | HTTP server port |
| `DATA_DIR` | ./data | Data directory path |
| `DATA_DATE` | latest | Date to load (`YYYY-MM-DD` or `latest`) |
| `DATA_MODE` | memory | `memory` (fast) or `stream` (low RAM) |
| `CACHE_MODE` | exhaust | `exhaust` (404 at end) or `rotation` (loop) |
| `ENDPOINT_CACHE_MODE` | shared | `shared` (one cursor per ticker/pkg per key) or `independent` (per category) |
| `RANGE_END_POLICY` | clamp | Multi-day range: at the span's end, `clamp` (last row) or `error` (HTTP 400) |
| `WS_ENABLED` | true | Enable WebSocket streaming |
| `WS_STREAM_INTERVAL` | 1s | Broadcast interval |
| `WS_GROUP_PREFIX` | blue | Prefix for WebSocket group names |
| `SYNC_BROADCAST_SYSTEM_ENABLED` | false | Enable the SSE sync-broadcast endpoint |
| `SYNC_BROADCAST_SYSTEM_ID` | hostname | Broadcaster identifier |
| `SYNC_BROADCAST_SYSTEM_INTERVAL` | 1s | Position broadcast interval |

## Docker / compose

| Variable | Default | Description |
| --- | --- | --- |
| `HOST_BIND` | 127.0.0.1 | Host bind address (`0.0.0.0` for LAN) |
| `HOST_PORT` | 8080 | Host port |
| `DATA_HOST_DIR` | ./data | Host directory mounted at `/app/data` |
| `PUID` / `PGID` | 1000 | User/group the containers run as |
| `PROMETHEUS_RETENTION` | 30d | Prometheus metrics retention |

## Auth & observability

| Variable | Default | Description |
| --- | --- | --- |
| `STUDIO_AUTH_TOKEN` | *(empty)* | HTTP Basic/Bearer gate for `/studio` **and** the mutating control routes (`reload-date`/`reset-cache`/`load-range`); empty = open |
| `LOKI_URL` | http://loki:3100 | Loki endpoint for the Logs screen |
| `PROMETHEUS_URL` | http://prometheus:9090 | Prometheus endpoint for Monitoring |

## Daemon (scheduled downloads)

| Variable | Default | Description |
| --- | --- | --- |
| `DAEMON_SCHEDULE_HOUR` | 17 | First EOD attempt hour (ET) |
| `DAEMON_SCHEDULE_MINUTE` | 5 | First EOD attempt minute |
| `DAEMON_TIMEZONE` | America/New_York | Timezone |
| `DAEMON_RUN_ON_STARTUP` | true | Check/download on start |
| `DAEMON_RUN_TIMEOUT_MINUTES` | 45 | Max duration of one download run |
| `DAEMON_CONFIG_PATH` | /app/configs/default.yaml | Downloader config the daemon uses |
| `DAEMON_STATE_FILE` | /app/data/.daemon-state | Where the daemon records progress |

## TTL cleanup (optional)

| Variable | Default | Description |
| --- | --- | --- |
| `GEXBOT_OUTPUT_AUTO_CLEANUP` | false | Evict idle materialized JSONL back to archive |
| `GEXBOT_OUTPUT_CLEANUP_AFTER_DAYS` | 7 | Idle days before eviction (must be ≥1 when cleanup is on; `gexbot.example.env` uses 7) |

## Push notifications (ntfy)

| Variable | Default | Description |
| --- | --- | --- |
| `NTFY_ENABLED` | false | Enable push notifications |
| `NTFY_SERVER` | https://ntfy.sh | ntfy server URL |
| `NTFY_TOPIC` | *(required)* | Topic name |
| `NTFY_PRIORITY` | default | min / low / default / high / urgent |
| `NTFY_TAGS` | package | Comma-separated emoji tags |
| `NTFY_TOKEN` | *(optional)* | Access token for private topics |

## Downloader (`GEXBOT_API_KEY` + YAML)

Downloading needs `GEXBOT_API_KEY`. Tickers/packages/categories and tuning live in
a YAML config selected by `GEXBOT_DOWNLOADER_CONFIG` (or the downloader's
`-c/--config` flag); copy `configs/default.yaml` to `configs/custom.yaml`:

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

Never print or commit secret values (`GEXBOT_API_KEY`, `STUDIO_AUTH_TOKEN`,
`NTFY_TOKEN`). See [daemon](daemon.md) for scheduling and [download data](download-data.md)
for the download paths.
