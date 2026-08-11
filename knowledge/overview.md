---
type: Overview
title: What the GEX Faker is
description: A Go server that replays historical GexBot options/GEX market data over REST and WebSocket, mirroring the real API's primary data routes, plus a downloader, a scheduling daemon, a web UI, and an agent CLI.
tags: [overview, faker, gexbot, rest, websocket, replay]
timestamp: 2026-08-11T00:00:00Z
---

# What the GEX Faker is

The **GEX Faker API** is a Go server that **replays historical GexBot market
data** — options gamma exposure (GEX), Greek profiles, and orderflow — over REST
and WebSocket routes that mirror the real [GexBot](https://www.gexbot.com) API's
primary data surface. Point a client, dashboard, or trading tool at the faker
instead of production and it replays a recorded market day, snapshot by snapshot,
with no live API quota and fully deterministic output.

> **Parity, honestly.** The core market-data routes and payload shapes match
> production, so a client built against those works against GexBot. It is not a
> total mirror: some endpoints differ (e.g. `/tickers` returns the *loaded*
> tickers, not the live supported universe), and the WebSocket hubs are the
> faker's five replay hubs. Known differences are tracked in
> [`compatibility/matrix.json`](https://github.com/dgnsrekt/gexbot-faker-api/blob/main/compatibility/matrix.json)
> and the [live compatibility audit](https://github.com/dgnsrekt/gexbot-faker-api/blob/main/docs/GEXBOT_LIVE_COMPATIBILITY_AUDIT.md).

## Why it exists

- **Develop and test offline.** Build against a stable, repeatable data feed
  instead of the live market. The same day replays identically every run.
- **No production key needed to *serve*.** Downloading real history needs a
  GexBot Quant key, but once a day is on disk the faker replays it for any client
  and accepts **any** token (see [point a client](point-a-client.md)).
- **Parity with production.** The REST paths, WebSocket hubs, and payload shapes
  mirror the real API, so a client that works against the faker works against
  GexBot.

## The pieces

| Piece | What it is |
| --- | --- |
| **API server** (`cmd/server`) | Serves REST + WebSocket + the Studio UI; replays a loaded date |
| **Studio** | Embedded web UI at `/studio` for downloading, loading, and watching data without curl |
| **Downloader** (`cmd/downloader`) | CLI that fetches historical days from GexBot into local archives |
| **Daemon** (`cmd/daemon`) | Long-running scheduler that auto-downloads each market day |
| **gexfakercli** (`cmd/gexfakercli`) | JSON-first CLI made for LLM agents to drive the faker |

## How data flows

1. **Download** a market day from GexBot (CLI, daemon, or the Studio) → it lands
   as a compressed **EOD archive** on disk.
2. **Materialize** the archive → per-category JSONL files the server can replay.
3. **Load** a date → the server serves its snapshots over REST and WebSocket.
4. A client **pulls** data; each API key walks the day's snapshots in order.

The download → materialize → load lifecycle is the one thing worth understanding
up front — see [materialize & load](materialize-load.md). To get running now, go
to the [quick start](quick-start.md).
