---
type: Guide
title: The download daemon
description: The long-running scheduler that downloads each market day automatically, its EOD-with-hist-fallback strategy, the coverage-regression alerts it emits, and ntfy push notifications.
tags: [daemon, scheduler, eod, coverage, alerts, ntfy]
timestamp: 2026-08-11T00:00:00Z
---

# The download daemon

The daemon (`cmd/daemon`) is a long-running service that **downloads each market
day automatically**, so a deployed faker always has fresh data without manual
runs. It ships in the default docker stack as `gex-daemon`.

## Scheduling

Each market day it attempts the **EOD report** download starting at
`DAEMON_SCHEDULE_HOUR:MINUTE` (default 17:05 ET), retrying every five minutes.
**After 8:00 PM ET** it falls back to individual `/hist` downloads. It is
market-day aware (skips weekends and holidays) and, with `DAEMON_RUN_ON_STARTUP`,
checks/downloads on boot. See [configuration](configuration.md) for the schedule
variables.

> The after-hours `/hist` fallback writes JSONL but not the `.eod-materialized`
> marker, so those dates still show `archived` / Materialize (issue #38). See
> [materialize & load](materialize-load.md).

## Coverage alerts

After each successful download the data is checked for **coverage regressions**
(`internal/coverage`); findings are logged, and a `high`-priority ntfy alert fires
when notifications are enabled:

- **Snapshot drop** — a ticker's intraday snapshot count falls >10% below its
  20-day median (what a silent source change looks like — e.g. a feed thinning
  SPX/NDX sampling).
- **Session shape** — the day opens later than ~09:30 ET, closes earlier than
  ~16:00 ET, or has an intraday gap over 120s (a truncated feed / outage).

The snapshot metric is also exported to Prometheus
(`faker_daemon_snapshots{ticker}`, `faker_daemon_coverage_findings_total{kind}`)
and surfaced in the Studio Data Library (sparkline + deviation badge) and the
Monitoring screen — see [docker & observability](docker-observability.md).

## Push notifications (ntfy)

Both the daemon and the CLI downloader can push to [ntfy.sh](https://ntfy.sh) on
completion, failure, or a coverage finding. Enable with `NTFY_ENABLED=true` and a
`NTFY_TOPIC`; subscribe at `https://ntfy.sh/<topic>` or in the ntfy app. Full
variable list in [configuration](configuration.md).
