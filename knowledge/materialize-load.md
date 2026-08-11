---
type: Explanation
title: The materialize and load lifecycle
description: Why a date you just downloaded shows Materialize instead of Load — the archived to ready to loaded lifecycle, the three Library states, and why the compressed archive is the durable source of truth.
tags: [materialize, load, lifecycle, archived, ready, loaded, jsonl]
timestamp: 2026-08-11T00:00:00Z
---

# The materialize & load lifecycle

This is the single most common point of confusion: **a date you just downloaded
often shows "Materialize", not "Load".** Here is why, and what each state means.

## Two on-disk forms

A market day exists on disk in one or both of two forms:

1. **EOD archive** — a compressed per-ticker zip under
   `data/eod/YYYY-MM-DD/TICKER/`. Compact, durable, the **canonical source of
   truth**. This is what the daemon and downloader CLI produce.
2. **Materialized JSONL** — uncompressed per-category files under
   `data/YYYY-MM-DD/TICKER/PACKAGE/CATEGORY.jsonl`, with an `.eod-materialized`
   marker. This is what the server actually **replays**. JSONL is uncompressed
   because stream playback needs record offsets that gzip can't provide
   efficiently.

**Materialize** = unpack the archive's gzipped members into JSONL and write the
marker. **Load** = start serving that JSONL over the API.

## The three Library states

| State | Button | Meaning |
| --- | --- | --- |
| `archived` | **Materialize** | Only the EOD archive is on disk (typical for daemon/CLI downloads) — unpack it first |
| `ready` | **Load** | JSONL is materialized on disk — loading is instant |
| `loaded` | *Loaded* | Currently being served by the API |

So the flow is **archived → (Materialize) → ready → (Load) → loaded**. A date
downloaded via the CLI or daemon starts at `archived`; a date downloaded via the
Studio's `/hist` screen skips ahead to `ready` because that path writes the
marker for you (see [download data](download-data.md)).

## Materialize-on-demand

You don't always have to click Materialize. The server **materializes a date on
demand** when it loads one that isn't unpacked yet — including via
`POST /reload-date` and `gexfakercli load <date>`. Materialize in the Library is
just a way to do the unpack ahead of time (and in the background) so a later Load
is instant.

## TTL cleanup (optional, off by default)

If `GEXBOT_OUTPUT_AUTO_CLEANUP=true` (window `GEXBOT_OUTPUT_CLEANUP_AFTER_DAYS`),
the daemon evicts idle materialized JSONL back to archive-only after the window,
reclaiming disk. The archive is untouched — a date can always be re-materialized.
This is **disabled by default**.

## Why the archive is the source of truth

The zip is small and canonical; JSONL is a derived, regenerable replay cache.
Keeping the archive as the durable form (and materializing on demand) is what lets
the faker hold hundreds of days of history without the disk cost of keeping every
day unpacked.

If a date won't load or shows the wrong state, see
[troubleshooting](troubleshooting.md).
