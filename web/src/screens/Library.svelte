<script lang="ts">
  import { onMount } from 'svelte'
  import { api, fmtBytes, fmtInt, type Status, type LibraryRow } from '../lib/api'

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
    busyDate = date
    error = ''
    try {
      await api.reloadDate(date) // fast: only offered on ready dates
      await refresh()
      onchanged()
    } catch (e) {
      error = `Load failed: ${e}`
    } finally {
      busyDate = null
    }
  }

  const totalSize = $derived(rows.reduce((a, r) => a + r.size_bytes, 0))
  const loadedRow = $derived(rows.find((r) => r.loaded))

  function pct(r: LibraryRow): string {
    if (!r.total) return '0%'
    return Math.round((r.materialized / r.total) * 100) + '%'
  }

  // --- Coverage: snapshots/ticker is comparable across dates, so a sustained
  // drop (e.g. a source thinning SPX/NDX sampling) is a real signal. Baseline is
  // the median of the most recent 20 days, so anomalies stand out against the
  // current normal rather than a mixed-regime average.
  function median(xs: number[]): number {
    const v = xs.filter((n) => n > 0).sort((a, b) => a - b)
    if (!v.length) return 0
    const m = Math.floor(v.length / 2)
    return v.length % 2 ? v[m] : Math.round((v[m - 1] + v[m]) / 2)
  }
  const baseline = $derived(median(rows.slice(0, 20).map((r) => r.snapshots)))
  function dev(r: LibraryRow): number {
    return baseline && r.snapshots ? (r.snapshots - baseline) / baseline : 0
  }
  function devClass(r: LibraryRow): string {
    const d = dev(r)
    if (Math.abs(d) < 0.08) return ''
    if (d <= -0.2) return 'dev big'
    return d < 0 ? 'dev' : 'dev up'
  }
  function devText(r: LibraryRow): string {
    const d = Math.round(dev(r) * 100)
    return (d > 0 ? '+' : '') + d + '%'
  }
  // Sparkline (chronological: oldest → newest) of snapshots/ticker.
  const SPARK_W = 240
  const SPARK_H = 30
  const sparkPoints = $derived.by(() => {
    const s = rows.map((r) => r.snapshots).reverse()
    if (s.length < 2) return ''
    const lo = Math.min(...s)
    const hi = Math.max(...s)
    const span = hi - lo || 1
    return s
      .map((v, i) => {
        const x = (i / (s.length - 1)) * SPARK_W
        const y = SPARK_H - ((v - lo) / span) * (SPARK_H - 4) - 2
        return `${x.toFixed(1)},${y.toFixed(1)}`
      })
      .join(' ')
  })
  const baseY = $derived.by(() => {
    const s = rows.map((r) => r.snapshots)
    const lo = Math.min(...s.filter((n) => n > 0))
    const hi = Math.max(...s)
    const span = hi - lo || 1
    return SPARK_H - ((baseline - lo) / span) * (SPARK_H - 4) - 2
  })
</script>

<div class="wrap">
  {#if loadedRow}
    <div class="banner">
      <span class="dot"></span>
      <div class="banner-text">
        Currently loaded: <span class="mono">{loadedRow.date}</span> ·
        {loadedRow.tickers.join(', ')} · {status?.files_loaded ?? 0} files
      </div>
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

  {#if baseline > 0 && rows.length >= 3}
    <div class="coverage">
      <div class="cov-head mono">
        COVERAGE
        <span class="dim">snapshots/ticker · median {fmtInt(baseline)} (~{Math.round((baseline / 23400) * 100)}% of a full 1/s session)</span>
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
              <span
                class={devClass(r)}
                title="{fmtInt(r.snapshots)} snapshots/ticker vs median {fmtInt(baseline)}"
              >{devText(r)}</span>
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
              <button class="btn load" disabled={busyDate !== null} onclick={() => load(r.date)}>
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
