# GEX Faker knowledge base

OKF v0.1 bundle for the GEX Faker API — a mock server that replays historical
GexBot options/GEX market data over REST and WebSocket, with a web UI (Studio)
and an agent CLI (`gexfakercli`). Start here, or point an agent at this directory
to answer setup, usage, and integration questions from the source itself.

## Use it

* [Overview](overview.md) — what the faker is, what it replays, and the pieces at a glance
* [Quick start](quick-start.md) — from clone to a live stream in the Studio (the golden path)
* [Studio](studio.md) — the seven screens of the web UI and what each one does
* [Download data](download-data.md) — the two ways data arrives (EOD archive vs `/hist`) and the API key
* [Materialize & load](materialize-load.md) — the archived → ready → loaded lifecycle (the #1 confusion)
* [Docker & observability](docker-observability.md) — the compose stack, Prometheus/Loki, and the Monitoring screen

## Build with it

* [Point a client at the faker](point-a-client.md) — base URL, header auth, and per-key playback
* [gexfakercli](gexfakercli.md) — the JSON-first agent CLI (`setup`, `describe`, data pulls, cursor)
* [REST API](rest-api.md) — the endpoint surface, Swagger UI, and the OpenAPI spec
* [WebSocket streaming](websockets.md) — the five hubs, `/negotiate`, group naming, and the frame format
* [Configuration](configuration.md) — every server and downloader environment variable
* [Daemon](daemon.md) — scheduled downloads, coverage alerts, and ntfy push notifications

## When something is wrong

* [Troubleshooting](troubleshooting.md) — no data, `archived` vs `ready`, auth 400s, exhausted cursors
