# Knowledge base update log

## 2026-08-11 (API consolidation + parity labeling)
* **Consolidated + renamed the faker control plane** to match the CLI's vocabulary
  (issue continuation of #66): the two loaders `reload-date` + `load-range` unified
  into one async **`POST /load`** (`{date}`|`{from,to}`|`{dates[]}` → `job_id`, poll
  `/load/status/{id}`); the two reads `current-date` + `current-range` into
  **`GET /current-load`**; and a full rename — `available-dates`→`/dates`,
  `available-data/{date}`→`/available/{date}`, `range-coverage`→`/coverage`,
  `reset-cache`→`/reset`, `seek-to-timestamp`→`/seek`. Old paths removed. The gated
  mutating set is now `load` + `reset`; `seek` and the reads stay open.
* **Made GexBot parity explicit.** `rest-api` now splits into **Real-GexBot parity**
  (a GexBot client works unchanged) vs the **Faker control plane** (faker-only);
  Swagger tags every operation the same way; `websockets` gained a parity banner
  (the protocol mirrors production, only prefix/cadence are faker knobs);
  `gexfakercli describe` carries a `gexbot_parity` flag per endpoint.
* **Fixed #66** (Studio showed only the span-start date): Studio now reads the full
  loaded span (`loaded_dates`), badges every loaded date, and shows the span in the
  status/settings. Spanish mirrors updated to match.

## 2026-08-11 (range replay + control auth)
* **Corrected the auth model.** `rest-api` and `point-a-client` no longer claim
  control routes are open: the mutating ones (`reload-date`/`reset-cache`/
  `load-range`) are gated behind `STUDIO_AUTH_TOKEN` when the server sets one (401
  otherwise); reads and `seek-to-timestamp` stay open (features #63).
* **Documented multi-day range replay** (#60/#64): the range endpoints
  (`/load-range`, `/current-range`, `/range-coverage`, `/load-range/status`) and the
  range-aware `seek` in `rest-api`; the `coverage`/`load-range`/`current-range`
  commands + `--token`/`GEXFAKER_TOKEN` in `gexfakercli` (mirroring the embedded
  SKILL.md); the `RANGE_END_POLICY` config; and a cross-day note in
  `materialize-load` + `point-a-client`. Spanish mirrors updated to match.

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
