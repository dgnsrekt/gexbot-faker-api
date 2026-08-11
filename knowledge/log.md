# Knowledge base update log

## 2026-08-11 (review fixes)
* **Qualified the parity claim** in `overview` and `point-a-client`: the faker
  mirrors GexBot's *primary data routes and payload shapes*, not the whole API
  (`/tickers` returns loaded tickers; WS is the five replay hubs). Linked the
  compatibility matrix and audit.
* **Completed `configuration`**: added `ENDPOINT_CACHE_MODE`, a Docker/compose
  table (`HOST_BIND`/`HOST_PORT`/`DATA_HOST_DIR`/`PUID`/`PGID`/`PROMETHEUS_RETENTION`),
  the remaining daemon vars (`DAEMON_RUN_TIMEOUT_MINUTES`/`DAEMON_CONFIG_PATH`/
  `DAEMON_STATE_FILE`), the cleanup default (7), and `GEXBOT_DOWNLOADER_CONFIG`;
  softened "every variable" to not overclaim exhaustiveness.

## 2026-08-11 (initialization)
* **Created the OKF v0.1 knowledge bundle** for the GEX Faker API. Fifteen files:
  an `index.md` hub split into **Use it** / **Build with it**, this log, and
  thirteen topics — `overview`, `quick-start`, `studio`, `download-data`,
  `materialize-load`, `docker-observability`, `point-a-client`, `gexfakercli`,
  `rest-api`, `websockets`, `configuration`, `daemon`, and `troubleshooting`.
* Content condensed and Diátaxis-typed from `README.md`, `CLAUDE.md`,
  `WEBSOCKET.md`, `OBSERVABILITY.md`, and `cmd/gexfakercli/skill/SKILL.md`.
* The bundle is the single source of truth: `llms.txt` / `llms-full.txt` are
  generated from it, and the Astro Starlight site (`/guides` + GitHub Pages)
  renders it.
