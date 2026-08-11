---
type: Tutorial
title: Quick start — from clone to a live stream
description: The golden path to a running faker; clone, add a GexBot key, download a day with Docker, open the Studio, load the date, and watch a live WebSocket stream.
tags: [quick-start, tutorial, docker, studio, getting-started]
timestamp: 2026-08-11T00:00:00Z
---

# Quick start

Goal: a running faker replaying a real market day, with data visible in the
Studio — in a few minutes. This is the golden path; each step links to deeper
detail if you need it.

## Prerequisites

- [Go 1.24+](https://go.dev/doc/install) (`go version`)
- [just](https://github.com/casey/just#installation) (`just --version`)
- [Docker](https://docs.docker.com/get-docker/) (`docker --version`)
- A [GexBot API key](https://www.gexbot.com/pricing) with a **Quant Subscription**
  — required only to **download** history, not to replay it.

## 1. Clone and add your key

```bash
git clone git@github.com:dgnsrekt/gexbot-faker-api.git
cd gexbot-faker-api
cp gexbot.example.env .env
# edit .env: set GEXBOT_API_KEY=your_key
```

## 2. Download a day of data

```bash
just download-lookback 7   # last 7 market days (weekends/holidays skipped)
```

This stores compressed **EOD archives** under `data/eod/YYYY-MM-DD/`. See
[download data](download-data.md) for other ways to fetch (single date, custom
tickers, or the Studio's point-and-click screen).

## 3. Start the server

```bash
just up      # API server + daemon in Docker
just logs    # follow the logs
```

Or run locally without Docker: `just serve-gex-faker`.

## 4. Open the Studio and load the date

Open **http://localhost:8080/studio**.

- Go to **Data library**. A freshly downloaded date shows **`archived`** —
  click **Materialize** to unpack it, then **Load**. (Why two steps?
  See [materialize & load](materialize-load.md).)
- The header shows the server is now replaying that date.

## 5. Watch a live stream

Go to **Live streams**. Pick a ticker and a feed, copy the generated group name,
and connect a WebSocket client — or just watch the hub client counts light up as
consumers subscribe. See [WebSocket streaming](websockets.md) to wire a client.

## First win reached

You now have a faker replaying a real day. Next:

- Pull data from the command line or an agent → [gexfakercli](gexfakercli.md).
- Point an existing client at it → [point a client](point-a-client.md).
- Explore every screen → [Studio](studio.md).
