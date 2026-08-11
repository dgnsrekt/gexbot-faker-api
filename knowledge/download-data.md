---
type: Guide
title: Downloading data
description: The two ways market data reaches the faker — the EOD archive path (daemon and CLI) and the individual /hist path (Studio) — plus the GexBot API key they require.
tags: [download, eod, hist, api-key, gexbot]
timestamp: 2026-08-11T00:00:00Z
---

# Downloading data

Downloading real history from GexBot needs a **`GEXBOT_API_KEY`** with a Quant
Subscription. (Replaying an already-downloaded day needs no key — see
[point a client](point-a-client.md).) Data reaches the faker two ways, and they
land in **different Library states**.

## Path 1 — EOD report (daemon default + CLI)

GexBot serves a pre-packed, per-ticker zip (`GET /v2/hist/eod/{ticker}`) whose
members are the gzipped per-category datasets. The faker stores it as-is as the
**canonical compressed archive** under `data/eod/YYYY-MM-DD/TICKER/`, plus a
locally-generated `…zip.manifest.json` sidecar. Because it is archive-only (no
JSONL yet), the date shows **`archived`** and must be **Materialized** before it
can be **Loaded**. This keeps disk usage lean.

Use the downloader CLI:

```bash
./bin/gexbot-downloader download 2025-11-14                     # single date
./bin/gexbot-downloader download 2025-11-01 2025-11-14          # date range
./bin/gexbot-downloader download --tickers SPX,NDX --packages state 2025-11-14
./bin/gexbot-downloader download --dry-run 2025-11-14           # preview
```

Or the just shortcut: `just download-lookback 7` (last 7 market days; weekends
and holidays are skipped automatically).

## Path 2 — individual `/hist` (Studio Download screen)

The Studio **Download** screen fetches per-category JSON from
`GET /v2/hist/{ticker}/{package}/{category}/{date}`, auto-converts it to JSONL,
and packs the same archive. The Studio worker **also writes the
`.eod-materialized` marker**, so the date shows **`ready` / Load` immediately** —
no separate Materialize step.

> The daemon's after-hours fallback uses this same `/hist` path but does **not**
> yet write the marker, so a daemon fallback leaves JSONL on disk while still
> showing `archived` / Materialize (tracked in
> [#38](https://github.com/dgnsrekt/gexbot-faker-api/issues/38)).

## What you can download

- **Tickers** — Indexes: SPX, NDX, RUT, VIX · ETFs: SPY, QQQ, IWM · Futures:
  ES_SPX, NQ_NDX.
- **Packages / categories** — `state` (gex_full/zero/one + delta/gamma/vanna/charm
  in zero/one), `classic` (gex_full/zero/one), `orderflow` (orderflow).

Select tickers, packages, and categories in `configs/custom.yaml` (copy from
`configs/default.yaml`) for the CLI/daemon, or in the Studio's batch builder.

Next: turn a downloaded archive into a served date →
[materialize & load](materialize-load.md).
