# SPEC — Multi-day range replay (cross-day seek) + control auth

**Status:** proposal for implementation (2026-08-11).
**Requested by:** gexsync (the TV Replay client that seeks this faker per ticker at a shared cursor).
**Priority:** Part A is the keystone. Part B is secondary (enables the gexsync remote-control).
**Client-side design context:** `~/Documents/projects/gexsync/DESIGN-continuous-multiday-replay.md`.

---

## Goal

Let a client replay historical GEX **continuously across a contiguous span of trading days**, per
ticker, without the client knowing which physical day-file backs any given timestamp. Today the
faker loads exactly one day; a seek to a timestamp outside it returns `400 "after all available
data"`, so replay dies the moment the chart cursor crosses a session boundary.

## Non-goals

- WebSocket / live streaming across days (REST replay only here).
- Changing data response bodies (byte-parity with live GEXbot must hold).
- Touching the downloader/daemon acquisition path.
- Auth changes beyond the control-plane gating in Part B.

## Baseline (current behavior — do not regress)

- **One day loaded at a time.** `POST /reload-date {date}` swaps it (resets positions).
  Data library flow: `archived → Materialize (unpack to JSONL) → Load`.
- **Seek is intra-day.** `POST /seek-to-timestamp {timestamp, key}` positions the per-`(ticker,
  package, key)` cursor within the loaded day. Before-data → clamp to row 0. After-data → `400`.
- **Cursor advances one row per data pull.** `CACHE_MODE=exhaust` → `404 "No more data"` at end;
  `rotation` → wraps to the *same day's* start. `ENDPOINT_CACHE_MODE=shared` = one cursor per
  `(ticker, package, key)`.
- **Data routes** `GET /{ticker}/{package}/{category}` require any non-empty Bearer (not validated).
  Control routes (`/reload-date`, `/available-dates`, `/current-date`, `/reset-cache`, `/tickers`,
  `/available-data/{date}`, `/seek-to-timestamp`) are open.

---

## Part A — Range load + cross-day seek (keystone)

### A1. `POST /load-range` — materialize + load a span into one cross-day index

Request (support both; `dates` wins if present):
```json
{ "from": "2026-08-06", "to": "2026-08-10" }
{ "dates": ["2026-08-06", "2026-08-07", "2026-08-10"] }
```
Behavior:
- For each requested date that is archived-but-not-materialized, **Materialize** it (reuse the Studio
  path). Then **Load all** into a *unified cross-day index*.
- **MUST use stream-mode** (disk offset index spanning days). RAM must stay bounded — days are
  200–540 MB / ~1–4 M rows; a 5-day range must not cost 5× RAM. Do not force `DATA_MODE=memory`.
- Idempotent: re-loading the same/overlapping range is a no-op for already-loaded days.
- Weekends/holidays inside a `from→to` span are simply absent (reflected in coverage, A4).

Response (sync when fast; otherwise return a job — see progress):
```json
{
  "loaded_range": { "from": "2026-08-06", "to": "2026-08-10" },
  "days": [ { "date": "2026-08-06", "status": "loaded", "tickers": ["AAPL","...","VIX"], "rows": 3851610 } ],
  "tickers_union": ["AAPL","...","VIX"],
  "tickers_intersection": ["QQQ","SPX","SPY"],
  "cache_mode": "exhaust"
}
```
**Progress:** materialize is slow. Either return `{ "job_id": "..." }` immediately with
`GET /load-range/status/{job_id}` (or an SSE stream), **reusing the existing `internal/downloadjob` +
`download.Manager.OnProgress` plumbing** the Studio download screen already uses. The client will show
a progress bar on the pill.

### A2. `POST /seek-to-timestamp` — make it range-aware

Same route + request shape (`{ timestamp, key }`; keep it key-scoped across all of that key's
endpoint cursors, as today). Now resolve the ts to `(day, row)` **anywhere in the loaded range**.

Return a small JSON body (today it's effectively 200-empty / 400):
```json
{ "requested_ts": 1786370149, "resolved_ts": 1786374000, "day": "2026-08-08",
  "in_gap": true, "clamped": "none", "reason": "gap→next-open" }
```
Resolution rules:
- **In session:** position at the first row at/after `ts` (existing intra-day semantics, now spanning
  days).
- **In an inter-session gap** (overnight/weekend/holiday): clamp to the **nearest row ≥ ts** (next
  session open). If none ahead in range, clamp to the **prior close**. Set `in_gap: true`.
- **Before range start:** clamp to range row 0, `clamped: "start"`.
- **After range end:** clamp to the last row, `clamped: "end"` **by default** (better replay UX than a
  hard 400); make the after-end policy a config flag (`clamp` | `error`).

### A3. Continuous cursor advance across days

In range mode, a data pull at the **last row of day N** advances into the **first row of day N+1**
(within the loaded range) instead of ending. Only at the range's final row does end-policy apply:
`exhaust` → `404`; `rotation` → wrap to range row 0. Single-day loads keep today's behavior exactly
(a single day is a range of one).

### A4. Coverage & discovery

- `GET /range-coverage?from=&to=` → per-day ticker sets + `union` + `intersection`, computed from
  archive metadata (`eod.ListArchives()`) so it works **before** load. Lets the client warn "IWM
  isn't covered on 08-07" before the user commits.
- `GET /current-range` → `{ "from", "to", "days": [{date,status}], "cache_mode" }`. (Or extend
  `/current-date` with a `loaded_range` field — keep `/current-date` working either way.)

### Code touchpoints (from CLAUDE.md)

- `api/openapi.yaml` → add routes → `just generate-gex-faker-api-spec`.
- `internal/server/handlers.go` → implement the new `StrictServerInterface` methods.
- `internal/data/loader.go` → `StreamLoader` gains a cross-day index; `IndexCache` gains cursor
  roll-over across day boundaries.
- `internal/downloadjob` + `download.Manager.OnProgress` → range-load progress.
- `internal/config` → after-range-end policy flag; keep `CACHE_MODE`/`ENDPOINT_CACHE_MODE` semantics.

---

## Part B — Control-plane auth for remote clients (secondary)

gexsync will **remote-control** the faker over the network (`llm2-studio.local`, not localhost):
`load-range` (with progress), `reload-date`, `reset-cache`, `current-range`, `range-coverage`, then
data GETs. These mutate faker state, so on a LAN they must be gated.

- **Reuse `STUDIO_AUTH_TOKEN`** (already gates `/studio` via HTTP Basic) to also gate the **mutating
  control routes** (`/load-range`, `/reload-date`, `/reset-cache`). Read-only discovery
  (`/available-dates`, `/current-range`, `/range-coverage`) may stay open. Data routes keep
  any-token behavior (unchanged).
- **Surface the token in the Studio Settings screen** (generate/show/rotate) so the user can copy it
  into gexsync. The client only *presents* the token (Bearer/Basic) — it does **not** register
  anything; the server owns its own secret. Empty token = open (localhost dev).

---

## Acceptance criteria (testable)

1. `load-range [D1..D3]`; seek to a ts in D2's session → returns a D2 row (classic **and** state);
   seek to a ts in D3 → D3 row. Data spot/majors match the standalone single-day load for that ts.
2. Play-through: pulling past D1's last row rolls into D2's first row with no 404.
3. Seek into an overnight gap → clamps to next open, `in_gap: true`, `resolved_ts` ≥ `requested_ts`.
4. A ticker present in D1 but absent in D2 → `range-coverage` reports it out of the intersection; a
   seek/pull for that ticker on D2 returns a **defined** response (documented error), not a crash.
5. Stream-mode RAM stays bounded loading a 5-day range (no 5× blow-up); `DATA_MODE=memory` not
   required.
6. **No regression:** a client that never calls `/load-range` (single `/reload-date` + intra-day
   seek) behaves exactly as before; data response bodies unchanged (byte-parity).
7. After-range-end policy honored: `clamp` returns the last row + `clamped:"end"`; `error` returns
   `400`.

## Open questions for the faker team

- Contiguous span vs explicit `dates` list as the primary shape? (Spec supports both; pick a default.)
- Progress transport: reuse the Studio download/logs SSE, or a poll endpoint?
- Should `/current-date` be extended or superseded by `/current-range`? (Backward-compat matters for
  existing gexsync — it reads `current_date`.)
- Per-key cursor semantics across a range under `ENDPOINT_CACHE_MODE=shared`: confirm one cursor per
  `(ticker, package, key)` still holds, now indexing into the cross-day span.

## How gexsync will drive this (client flow, informational)

1. On opening the replay panel: `GET /available-dates` + `/range-coverage` → render a date/range
   picker; warn on tickers outside the intersection for the chosen span.
2. User picks a span (or "last N days"): `POST /load-range` → show materialize progress on the pill.
3. TV Replay cursor moves → per pane, per ticker: `POST /seek-to-timestamp` → `GET
   /{ticker}/{package}/{category}`. `in_gap`/`clamped` drive a "between sessions" pill cue.
4. All control calls carry the token from gexsync's faker config (Part B) when set.
