# Observability

Observability runs in the **default** Compose stack — `just up` (or `docker
compose up -d`) starts **Prometheus**, **Loki**, and **Grafana Alloy** alongside
the API and daemon. There is no separate profile, gateway, or extra login: the
GEX Faker **Studio** is the single pane of glass. It queries Prometheus and Loki
**server-side** over the private Compose network and renders everything natively,
so the browser never talks to those backends and there are no host-facing
monitoring ports to expose.

## What runs

| Component | Role |
| --- | --- |
| **Prometheus** | Scrapes API (`gex-faker-api:9090`) and daemon (`gex-daemon:9091`) metrics; retention `PROMETHEUS_RETENTION` (default `30d`). |
| **Loki** | Aggregates container logs. |
| **Grafana Alloy** | Tails containers labeled `observability.logs=true` and ships their logs to Loki. |

None are published to the host — Studio reaches them internally.

## In Studio

- **Logs** — live log feed proxied from Loki (`LOKI_URL`, default `http://loki:3100`).
- **Monitoring** — metric panels proxied from Prometheus (`PROMETHEUS_URL`, default
  `http://prometheus:9090`). Both degrade with a clear message if the backend is
  unset or unreachable (e.g. under `go run` without the stack).

Reach the Studio at `http://<host>:${HOST_PORT:-8080}/studio` (gate it with
`STUDIO_AUTH_TOKEN` and set `HOST_BIND=0.0.0.0` for LAN access — see the README).

## Uptime monitoring

Uptime/health monitoring is intentionally **not** part of this project — it's a
fleet-level concern. Point a single fleet Uptime-Kuma (or similar) at the faker's
open `GET /health` endpoint.

## Notes

- When running Compose from a Git worktree, set `DATA_HOST_DIR` to the checkout
  that owns the dataset (e.g. `DATA_HOST_DIR=../gexbot-faker-api/data`). Default `./data`.
- Grafana, Uptime-Kuma, and the Caddy gateway were removed (issue #46); Grafana can
  be re-added later purely for ad-hoc PromQL exploration if a need arises.
