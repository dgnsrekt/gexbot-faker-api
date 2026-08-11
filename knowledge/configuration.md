---
type: Reference
title: Configuration
description: Every environment variable that configures the faker — the server (port, data, cache, WebSocket, sync, auth, observability), the daemon schedule, ntfy push, and the downloader YAML config.
tags: [configuration, environment, env-vars, settings, reference]
timestamp: 2026-08-11T00:00:00Z
---

# Configuration

Everything is configured by environment variables (the Studio **Settings** screen
shows the effective values in plain language). Copy `gexbot.example.env` to `.env`
to start.

## Server

| Variable | Default | Description |
| --- | --- | --- |
| `PORT` | 8080 | HTTP server port |
| `DATA_DIR` | ./data | Data directory path |
| `DATA_DATE` | latest | Date to load (`YYYY-MM-DD` or `latest`) |
| `DATA_MODE` | memory | `memory` (fast) or `stream` (low RAM) |
| `CACHE_MODE` | exhaust | `exhaust` (404 at end) or `rotation` (loop) |
| `WS_ENABLED` | true | Enable WebSocket streaming |
| `WS_STREAM_INTERVAL` | 1s | Broadcast interval |
| `WS_GROUP_PREFIX` | blue | Prefix for WebSocket group names |
| `SYNC_BROADCAST_SYSTEM_ENABLED` | false | Enable the SSE sync-broadcast endpoint |
| `SYNC_BROADCAST_SYSTEM_ID` | hostname | Broadcaster identifier |
| `SYNC_BROADCAST_SYSTEM_INTERVAL` | 1s | Position broadcast interval |

## Exposure, auth & observability

| Variable | Default | Description |
| --- | --- | --- |
| `HOST_BIND` | 127.0.0.1 | Docker host bind address (`0.0.0.0` for LAN) |
| `HOST_PORT` | 8080 | Docker host port |
| `STUDIO_AUTH_TOKEN` | *(empty)* | HTTP Basic gate for `/studio` (empty = open) |
| `LOKI_URL` | http://loki:3100 | Loki endpoint for the Logs screen |
| `PROMETHEUS_URL` | http://prometheus:9090 | Prometheus endpoint for Monitoring |

## Daemon (scheduled downloads)

| Variable | Default | Description |
| --- | --- | --- |
| `DAEMON_SCHEDULE_HOUR` | 17 | First EOD attempt hour (ET) |
| `DAEMON_SCHEDULE_MINUTE` | 5 | First EOD attempt minute |
| `DAEMON_TIMEZONE` | America/New_York | Timezone |
| `DAEMON_RUN_ON_STARTUP` | true | Check/download on start |

## TTL cleanup (optional)

| Variable | Default | Description |
| --- | --- | --- |
| `GEXBOT_OUTPUT_AUTO_CLEANUP` | false | Evict idle materialized JSONL back to archive |
| `GEXBOT_OUTPUT_CLEANUP_AFTER_DAYS` | — | Idle window before eviction |

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
`configs/custom.yaml` (copy from `configs/default.yaml`):

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
