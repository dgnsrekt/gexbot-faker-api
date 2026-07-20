# GexBot compatibility evidence

`upstream-openapi.yaml` is the target market-data contract used by the compatibility audit.

- Base: `nfa-llc/gexbot-openapi` commit `fb026f49af3e370ccf2fbd6a57092b284065e4ed`
- Source SHA-256: `b2c249a0a995b590a446c395ceb54688792ebf962236d7e6ccb939d7224871d5`
- Live docs: <https://www.gexbot.com/apidocs>
- Live docs bundle checked: `/static/js/main.e7ad3422.js`
- Bundle SHA-256: `6f9e261a3e7845c0b8cbaa5aae0c6eaf36a6611a4a6439b92f7a1facbb9a2cdb`
- Snapshot date: 2026-07-19

The live documentation is ahead of the repository specification. This snapshot adds:

- `GET /hist/eod/{ticker}`
- `GET /futures/conversion`

Research is intentionally excluded from faker parity. Faker health, playback, download, documentation, and synchronization routes are extensions and do not count against parity.
