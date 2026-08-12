<script lang="ts">
  import { onMount } from 'svelte'
  import { api, fmtBytes, fmtInt, isValidSpan, type Status, type LibraryRow } from '../lib/api'

  let { status, onchanged }: { status: Status | null; onchanged: () => void } = $props()

  let rows = $state<LibraryRow[]>([])
  let loading = $state(true)
  let busyDate = $state<string | null>(null) // a Load in progress
  let error = $state('')

  async function refresh() {
    try {
      rows = await api.library()
      error = ''
    } catch (e) {
      error = String(e)
    } finally {
      loading = false
    }
  }

  onMount(refresh)

  // Poll only while something is materializing, so a static library isn't hit
  // every few seconds (the library read stats/reads every archive manifest).
  const materializing = $derived(rows.some((r) => r.state === 'materializing'))
  $effect(() => {
    if (!materializing) return
    const t = setInterval(refresh, 2000)
    return () => clearInterval(t)
  })

  async function materialize(date: string) {
    error = ''
    try {
      await api.materialize(date)
      await refresh() // flips the row to "materializing"; the effect starts polling
    } catch (e) {
      error = `Materialize failed: ${e}`
    }
  }

  async function load(date: string) {
    if (anyBusy) return // don't queue a second load behind an in-flight one (backend serializes)
    busyDate = date
    error = ''
    try {
      await api.load(date) // async /load, polled to done; fast since only offered on ready dates
      await refresh()
      onchanged()
    } catch (e) {
      error = `Load failed: ${e}`
    } finally {
      busyDate = null
    }
  }

  // Span loader: load a contiguous range of days as one cross-day dataset (POST /load {from,to}).
  // The archived days in the range are materialized on demand; the loaded badges then span them all.
  let spanFrom = $state('')
  let spanTo = $state('')
  let spanBusy = $state(false)

  async function loadSpan() {
    if (!spanValid || anyBusy) return
    spanBusy = true
    error = ''
    try {
      await api.loadRange({ from: spanFrom, to: spanTo })
      await refresh()
      onchanged()
    } catch (e) {
      error = `Load span failed: ${e}`
    } finally {
      spanBusy = false
    }
  }

  const totalSize = $derived(rows.reduce((a, r) => a + r.size_bytes, 0))
  // The span-loader offers only the dates that actually exist on disk (sorted
  // chronologically), so you can't pick an empty day or scroll dead years.
  const availDates = $derived([...new Set(rows.map((r) => r.date))].sort())
  const dateMin = $derived(availDates[0] ?? '')
  const dateMax = $derived(availDates[availDates.length - 1] ?? '')
  // Newest-first for the dropdowns so the recent dates are at the top (no scrolling).
  const availDatesDesc = $derived([...availDates].reverse())
  // A span is valid only when both ends are set, ordered, and within the on-disk
  // inventory — so a reversed or out-of-range span can't be submitted.
  const spanValid = $derived(isValidSpan(spanFrom, spanTo, dateMin, dateMax))
  // Single-date and span loads share one busy gate: the backend serializes load
  // jobs, so a second load queued during the first would silently replace it.
  const anyBusy = $derived(spanBusy || busyDate !== null)
  // Every loaded date badges (issue #66); the banner summarizes the whole loaded span (chronological).
  const loadedRows = $derived(rows.filter((r) => r.loaded))
  const loadedSpanLabel = $derived.by(() => {
    const ds = loadedRows.map((r) => r.date).sort()
    if (ds.length <= 1) return ds[0] ?? ''
    return `${ds[0]} → ${ds[ds.length - 1]} (${ds.length} days)`
  })
  const loadedTickers = $derived([...new Set(loadedRows.flatMap((r) => r.tickers))].sort())

  function pct(r: LibraryRow): string {
    if (!r.total) return '0%'
    return Math.round((r.materialized / r.total) * 100) + '%'
  }

  // --- Coverage: the server sends a composition-stable index per date (~1.0 =
  // every ticker at its own snapshot norm; <1 = a real per-ticker drop). It is
  // already normalized per ticker, so changing the ticker set doesn't move it —
  // the deviation is simply (index − 1).
  const covRows = $derived(rows.filter((r) => r.coverage > 0))
  const haveCoverage = $derived(covRows.length >= 3)
  function dev(r: LibraryRow): number {
    return r.coverage > 0 ? r.coverage - 1 : 0
  }
  function devClass(r: LibraryRow): string {
    if (r.coverage <= 0) return ''
    const d = dev(r)
    if (Math.abs(d) < 0.08) return ''
    if (d <= -0.2) return 'dev big'
    return d < 0 ? 'dev' : 'dev up'
  }
  function devText(r: LibraryRow): string {
    const d = Math.round(dev(r) * 100)
    return (d > 0 ? '+' : '') + d + '%'
  }
  function covTitle(r: LibraryRow): string {
    const byTk = r.snapshots_by_ticker
    const parts = byTk
      ? Object.keys(byTk)
          .sort()
          .map((tk) => `${tk} ${fmtInt(byTk[tk])}`)
      : []
    return `coverage ${Math.round(r.coverage * 100)}% of each ticker's norm` +
      (parts.length ? ` · ${parts.join(', ')}` : '')
  }
  // Sparkline (chronological: oldest → newest) of the coverage index, scaled
  // around 1.0 so the baseline sits mid-height.
  const SPARK_W = 240
  const SPARK_H = 30
  const sparkPoints = $derived.by(() => {
    const s = rows.map((r) => r.coverage).reverse()
    if (s.filter((v) => v > 0).length < 2) return ''
    const vals = s.filter((v) => v > 0)
    const lo = Math.min(...vals, 0.9)
    const hi = Math.max(...vals, 1.1)
    const span = hi - lo || 1
    const pts: string[] = []
    s.forEach((v, i) => {
      if (v <= 0) return
      const x = (i / (s.length - 1)) * SPARK_W
      const y = SPARK_H - ((v - lo) / span) * (SPARK_H - 4) - 2
      pts.push(`${x.toFixed(1)},${y.toFixed(1)}`)
    })
    return pts.join(' ')
  })
  const baseY = $derived.by(() => {
    const vals = rows.map((r) => r.coverage).filter((v) => v > 0)
    if (!vals.length) return SPARK_H / 2
    const lo = Math.min(...vals, 0.9)
    const hi = Math.max(...vals, 1.1)
    const span = hi - lo || 1
    return SPARK_H - ((1 - lo) / span) * (SPARK_H - 4) - 2 // baseline at index 1.0
  })
</script>

<div class="wrap">
  {#if loadedRows.length > 0}
    <div class="banner">
      <span class="dot"></span>
      <div class="banner-text">
        Currently loaded: <span class="mono">{loadedSpanLabel}</span> ·
        {loadedTickers.join(', ')} · {status?.files_loaded ?? 0} files
      </div>
    </div>
  {/if}

  {#if !loading && rows.length > 0}
    <div class="span-loader">
      <span class="sl-label">Load a span</span>
      <select class="sl-select mono" bind:value={spanFrom} aria-label="span start">
        <option value="" disabled>from</option>
        {#each availDatesDesc as d (d)}<option value={d}>{d}</option>{/each}
      </select>
      <span class="sl-arrow">→</span>
      <select class="sl-select mono" bind:value={spanTo} aria-label="span end">
        <option value="" disabled>to</option>
        {#each availDatesDesc as d (d)}<option value={d}>{d}</option>{/each}
      </select>
      <button class="sl-btn" disabled={!spanValid || anyBusy} onclick={loadSpan}>
        {spanBusy ? 'Loading…' : 'Load span'}
      </button>
      <span class="sl-note">loads every archived day in the range as one cross-day dataset</span>
    </div>
  {/if}

  {#if error}
    <div class="error">{error}</div>
  {/if}

  <div class="legend">
    Daily downloads arrive as compressed <b>EOD archives</b> (<span class="mono">archived</span>):
    <b>Materialize</b> unpacks one to JSONL, then <b>Load</b> serves it. Dates fetched from the
    <b>Download</b> screen are materialized already and show <b>Load</b> directly.
  </div>

  {#if haveCoverage}
    <div class="coverage">
      <div class="cov-head mono">
        COVERAGE
        <span class="dim">each ticker vs its own median · 100% = normal</span>
      </div>
      <svg class="spark" viewBox="0 0 {SPARK_W} {SPARK_H}" preserveAspectRatio="none" aria-hidden="true">
        <line class="base" x1="0" x2={SPARK_W} y1={baseY} y2={baseY} />
        <polyline points={sparkPoints} />
      </svg>
    </div>
  {/if}

  <div class="card table">
    <div class="thead mono">
      <div>DATE</div>
      <div>TICKERS</div>
      <div>PACKAGES</div>
      <div>SIZE</div>
      <div>RECORDS</div>
      <div class="right">ACTIONS</div>
    </div>

    {#if loading}
      <div class="msg">Loading library…</div>
    {:else if rows.length === 0}
      <div class="msg">No EOD archives found in the data folder.</div>
    {:else}
      {#each rows as r (r.date)}
        <div class="row" class:loaded={r.loaded}>
          <div class="mono date" style="color:{r.loaded ? 'var(--green)' : 'var(--text-2)'}">
            {r.date}
          </div>
          <div class="mono tickers">{r.tickers.join(' ')}</div>
          <div class="pkgs">
            {#each r.packages as p (p)}<span class="pill">{p}</span>{/each}
          </div>
          <div class="mono dim">{fmtBytes(r.size_bytes)}</div>
          <div class="mono dim records">
            {fmtInt(r.records)}
            {#if devClass(r)}
              <span class={devClass(r)} title={covTitle(r)}>{devText(r)}</span>
            {/if}
          </div>
          <div class="actions">
            {#if r.status === 'corrupt'}
              <span class="badge corrupt">corrupt</span>
            {/if}

            {#if r.state === 'loaded'}
              <span class="badge loaded-badge">Loaded</span>
            {:else if r.state === 'materializing'}
              <div class="materializing" title="Unpacking EOD archives to JSONL">
                <div class="mbar"><div class="mfill" style="width:{pct(r)}"></div></div>
                <span class="mono mtext">Materializing {r.materialized}/{r.total}</span>
              </div>
            {:else if r.state === 'ready'}
              <button class="btn load" disabled={anyBusy} onclick={() => load(r.date)}>
                {busyDate === r.date ? 'Loading…' : 'Load'}
              </button>
            {:else}
              {#if r.job_error}
                <span class="failed mono" title={r.job_error}>failed</span>
              {/if}
              <button
                class="btn materialize"
                title={r.job_error ? `Retry: ${r.job_error}` : "Unpack this date's EOD archives so it can be loaded"}
                onclick={() => materialize(r.date)}
              >
                {r.job_error ? 'Retry' : 'Materialize'}
              </button>
            {/if}
          </div>
        </div>
      {/each}
    {/if}

    <div class="foot">
      <div>
        {rows.length} market day{rows.length === 1 ? '' : 's'} on disk · {fmtBytes(totalSize)}
      </div>
      <div class="flex-1"></div>
      <div class="mono path">~/gexbot-faker/data/eod</div>
    </div>
  </div>
</div>

<style>
  .wrap {
    padding: 18px 20px 24px;
    display: flex;
    flex-direction: column;
    gap: 14px;
  }
  .legend {
    font-size: 12px;
    line-height: 1.5;
    color: var(--text-2);
    border-left: 2px solid var(--border);
    padding: 2px 0 2px 10px;
  }
  .coverage {
    display: flex;
    align-items: center;
    gap: 16px;
    padding: 10px 14px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--panel, #111214);
  }
  .cov-head {
    font-size: 10.5px;
    letter-spacing: 0.04em;
    color: var(--text-1);
    white-space: nowrap;
  }
  .cov-head .dim {
    margin-left: 8px;
    color: var(--muted-2);
    letter-spacing: 0;
  }
  .spark {
    flex: 1;
    height: 30px;
    min-width: 120px;
  }
  .spark polyline {
    fill: none;
    stroke: var(--green);
    stroke-width: 1.4;
    vector-effect: non-scaling-stroke;
  }
  .spark .base {
    stroke: var(--border);
    stroke-width: 1;
    stroke-dasharray: 3 3;
    vector-effect: non-scaling-stroke;
  }
  .records {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .dev {
    font-size: 10px;
    padding: 1px 4px;
    border-radius: 4px;
    color: var(--amber, #e0b164);
    background: color-mix(in srgb, var(--amber, #e0b164) 14%, transparent);
  }
  .dev.up {
    color: var(--muted-2);
    background: color-mix(in srgb, var(--muted-2, #888) 14%, transparent);
  }
  .dev.big {
    color: var(--red, #e08080);
    background: color-mix(in srgb, var(--red, #e08080) 16%, transparent);
  }
  .legend b {
    color: var(--text-1);
    font-weight: 600;
  }
  .legend .mono {
    font-size: 11px;
    color: var(--amber, #e0b164);
  }
  .banner {
    border: 1px solid var(--green-border);
    background: #131a16;
    border-radius: 10px;
    padding: 12px 14px;
    display: flex;
    align-items: center;
    gap: 12px;
    font-size: 12.5px;
  }
  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--green);
    flex: none;
  }
  .span-loader {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
    border: 1px solid var(--border);
    background: var(--panel-2);
    border-radius: 10px;
    padding: 9px 12px;
  }
  .sl-label {
    font-size: 11.5px;
    color: var(--text-2);
    font-weight: 600;
  }
  .sl-select {
    background: var(--input);
    border: 1px solid var(--border-2);
    border-radius: 6px;
    color: var(--text-2);
    padding: 5px 8px;
    font-size: 11.5px;
    cursor: pointer;
  }
  .sl-select:focus {
    outline: none;
    border-color: var(--green-border);
  }
  .sl-arrow {
    color: var(--muted-2);
    font-size: 12px;
  }
  .sl-btn {
    padding: 6px 12px;
    border-radius: 6px;
    border: none;
    background: var(--green);
    color: var(--green-ink);
    font-size: 11.5px;
    font-weight: 600;
    cursor: pointer;
  }
  .sl-btn:disabled {
    background: #1b1e23;
    color: var(--dimmer);
    cursor: default;
  }
  .sl-note {
    font-size: 10.5px;
    color: var(--dim);
    margin-left: auto;
  }
  .error {
    border: 1px solid #4a2b2b;
    background: #1a1313;
    color: var(--red);
    border-radius: 8px;
    padding: 10px 14px;
    font-size: 12px;
  }
  .table {
    overflow: hidden;
  }
  .thead,
  .row {
    display: grid;
    grid-template-columns: 96px 120px minmax(80px, 1fr) 72px 84px 150px;
    gap: 12px;
    align-items: center;
    padding: 11px 14px;
  }
  .thead {
    border-bottom: 1px solid var(--border);
    font-size: 10.5px;
    color: var(--muted-2);
    padding: 10px 14px;
  }
  .row {
    border-bottom: 1px solid #191c20;
  }
  .row:hover {
    background: #16181c;
  }
  .row.loaded {
    background: #141a16;
  }
  .right {
    text-align: right;
  }
  .date {
    font-size: 11.5px;
  }
  .tickers {
    font-size: 11px;
    color: #9aa0a9;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .pkgs {
    display: flex;
    gap: 5px;
    flex-wrap: wrap;
  }
  .dim {
    font-size: 11px;
    color: #9aa0a9;
  }
  .actions {
    display: flex;
    gap: 6px;
    justify-content: flex-end;
    align-items: center;
  }
  .badge {
    font-size: 10px;
    padding: 2px 7px;
    border-radius: 4px;
    font-family: var(--mono);
  }
  .badge.corrupt {
    color: var(--red);
    border: 1px solid #4a2b2b;
  }
  .badge.loaded-badge {
    color: var(--green);
    border: 1px solid var(--green-border);
  }
  .btn.load {
    padding: 4px 12px;
    color: var(--text-2);
  }
  .btn.load:disabled {
    opacity: 0.5;
    cursor: default;
  }
  .failed {
    font-size: 10px;
    color: var(--red);
    border: 1px solid #4a2b2b;
    border-radius: 4px;
    padding: 2px 7px;
  }
  .btn.materialize {
    padding: 4px 10px;
    color: var(--amber);
    border-color: #4a3a20;
  }
  .btn.materialize:hover {
    color: #f0c682;
    border-color: #6a5528;
  }
  .materializing {
    display: flex;
    flex-direction: column;
    gap: 4px;
    align-items: flex-end;
    width: 100%;
  }
  .mbar {
    width: 100%;
    height: 5px;
    border-radius: 3px;
    background: #1e2126;
    overflow: hidden;
  }
  .mfill {
    height: 100%;
    background: var(--amber);
    transition: width 0.4s ease;
  }
  .mtext {
    font-size: 10px;
    color: var(--amber);
  }
  .msg {
    padding: 20px 14px;
    color: var(--muted-2);
    font-size: 12px;
  }
  .foot {
    display: flex;
    align-items: center;
    padding: 11px 14px;
    font-size: 11.5px;
    color: var(--muted-2);
  }
  .flex-1 {
    flex: 1;
  }
  .path {
    font-size: 10.5px;
    color: var(--dimmer);
  }
</style>
