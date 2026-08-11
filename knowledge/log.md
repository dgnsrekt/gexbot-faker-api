# Knowledge base update log

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
