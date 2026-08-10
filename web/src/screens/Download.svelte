<script lang="ts">
  import { onMount } from 'svelte'
  import { api, type CalendarDay, type DownloadJob } from '../lib/api'

  let { onchanged }: { onchanged: () => void } = $props()

  let enabled = $state(true)
  let tickers = $state<string[]>([])
  let packages = $state<string[]>([])
  let selTickers = $state<Set<string>>(new Set())
  let selPackages = $state<Set<string>>(new Set())

  let mode = $state<'calendar' | 'batch'>('calendar')

  // Calendar
  let month = $state('') // YYYY-MM
  let days = $state<CalendarDay[]>([])
  let selDates = $state<Set<string>>(new Set())

  // Batch
  let from = $state('')
  let to = $state('')

  let jobs = $state<DownloadJob[]>([])
  let error = $state('')
  let loading = $state(true)

  const WEEKDAYS = ['MON', 'TUE', 'WED', 'THU', 'FRI']

  function ym(d: Date): string {
    return `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, '0')}`
  }

  async function loadCalendar() {
    try {
      const r = await api.calendar(month)
      days = r.days
    } catch {
      days = []
    }
  }

  async function init() {
    try {
      const o = await api.downloadOptions()
      enabled = o.enabled
      tickers = o.tickers
      packages = o.packages.map((p) => p.name)
      selTickers = new Set(tickers.slice(0, 3))
      selPackages = new Set(packages.filter((p) => p === 'state' || p === 'classic'))
    } catch {
      enabled = false
    }
    const s = await api.status().catch(() => null)
    month = s?.loaded_date ? s.loaded_date.slice(0, 7) : ym(new Date())
    from = s?.loaded_date ?? ''
    to = s?.loaded_date ?? ''
    await loadCalendar()
    await refreshJobs()
    loading = false
  }

  // onMount stays synchronous so its cleanup actually runs.
  onMount(() => {
    init()
    return () => clearInterval(poll)
  })

  // Weekday-only (Mon–Fri) grid cells with a leading offset.
  const grid = $derived.by(() => {
    const wk = days.filter((d) => d.weekday >= 1 && d.weekday <= 5)
    if (wk.length === 0) return { blanks: 0, cells: [] as CalendarDay[] }
    const blanks = wk[0].weekday - 1 // Mon=1 → 0
    return { blanks, cells: wk }
  })

  function shiftMonth(delta: number) {
    const [y, m] = month.split('-').map(Number)
    const d = new Date(Date.UTC(y, m - 1 + delta, 1))
    month = ym(d)
    loadCalendar()
  }

  function toggleDate(d: CalendarDay) {
    if (!d.market_day) return
    const s = new Set(selDates)
    s.has(d.date) ? s.delete(d.date) : s.add(d.date)
    selDates = s
  }
  function selectMissing() {
    selDates = new Set(days.filter((d) => d.state === 'missing').map((d) => d.date))
  }
  function toggleSet(set: Set<string>, k: string): Set<string> {
    const s = new Set(set)
    s.has(k) ? s.delete(k) : s.add(k)
    return s
  }

  // Batch range → weekdays (server drops holidays).
  const batchDates = $derived.by(() => {
    if (!from || !to || from > to) return [] as string[]
    const out: string[] = []
    let d = new Date(from + 'T00:00:00Z')
    const end = new Date(to + 'T00:00:00Z')
    let guard = 0
    while (d <= end && guard++ < 500) {
      const wd = d.getUTCDay()
      if (wd >= 1 && wd <= 5) out.push(d.toISOString().slice(0, 10))
      d = new Date(d.getTime() + 86400000)
    }
    return out
  })

  const chosenDates = $derived(mode === 'calendar' ? [...selDates].sort() : batchDates)
  const canDownload = $derived(
    enabled && chosenDates.length > 0 && selTickers.size > 0 && selPackages.size > 0,
  )

  let poll: ReturnType<typeof setInterval>
  const busy = $derived(jobs.some((j) => j.state === 'queued' || j.state === 'running'))

  async function refreshJobs() {
    try {
      jobs = await api.downloadStatus()
    } catch {
      /* keep */
    }
  }

  function ensurePolling() {
    clearInterval(poll)
    poll = setInterval(async () => {
      await refreshJobs()
      await loadCalendar()
      onchanged() // library/status may have new archives
      if (!jobs.some((j) => j.state === 'queued' || j.state === 'running')) clearInterval(poll)
    }, 2000)
  }

  async function startDownload() {
    if (!canDownload) return
    error = ''
    try {
      await api.download(chosenDates, [...selTickers], [...selPackages])
      selDates = new Set()
      await refreshJobs()
      ensurePolling()
    } catch (e) {
      error = `Download failed: ${e}`
    }
  }

  function dayColor(d: CalendarDay): string {
    if (!d.market_day) return '#2a2d33'
    switch (d.state) {
      case 'loaded':
      case 'ready':
        return 'var(--green)'
      case 'archived':
        return 'var(--blue)'
      default:
        return '#3a3e46' // missing
    }
  }
  function jobColor(s: string): string {
    if (s === 'done') return 'var(--green)'
    if (s === 'running' || s === 'partial') return 'var(--amber)'
    if (s === 'error') return 'var(--red)'
    return '#3a3e46'
  }
</script>

<div class="wrap">
  {#if !enabled}
    <div class="notice">
      Downloads are disabled — the server has no <code class="mono">GEXBOT_API_KEY</code>. Set it on
      the <code class="mono">gex-faker-api</code> container (and restart) to fetch market days from GEXbot.
    </div>
  {/if}
  {#if error}<div class="err">{error}</div>{/if}

  <div class="head">
    <div class="lead">
      <div class="title">Get market days onto disk</div>
      <div class="sub">
        Pick days to fetch from GEXbot. Weekends and holidays are skipped. Each day lands as an EOD
        archive — materialize + load it from the Data library.
      </div>
    </div>
    <div class="flex-1"></div>
    <div class="toggle">
      <button class="tg" class:on={mode === 'calendar'} onclick={() => (mode = 'calendar')}>Calendar</button>
      <button class="tg" class:on={mode === 'batch'} onclick={() => (mode = 'batch')}>Batch builder</button>
    </div>
  </div>

  <div class="grid2">
    <!-- left: picker -->
    <div class="card picker">
      {#if mode === 'calendar'}
        <div class="cal-head">
          <button class="nav" onclick={() => shiftMonth(-1)}>‹</button>
          <div class="month mono">{month}</div>
          <button class="nav" onclick={() => shiftMonth(1)}>›</button>
          <div class="flex-1"></div>
          <button class="mini" onclick={selectMissing}>Select missing</button>
          <button class="mini" onclick={() => (selDates = new Set())}>Clear</button>
        </div>
        <div class="cal">
          {#each WEEKDAYS as w (w)}<div class="wd mono">{w}</div>{/each}
          {#each Array(grid.blanks) as _blank, i (i)}<div class="blank"></div>{/each}
          {#each grid.cells as d (d.date)}
            <div
              class="cell"
              class:sel={selDates.has(d.date)}
              class:clickable={d.market_day}
              onclick={() => toggleDate(d)}
              role="button"
              tabindex="-1"
              onkeydown={(e) => e.key === 'Enter' && toggleDate(d)}
            >
              <div class="cell-top">
                <span class="num mono" style="color:{d.market_day ? 'var(--text-2)' : '#4e535b'}">{d.day}</span>
                <span class="dot" style="background:{dayColor(d)}"></span>
              </div>
              <div class="cell-state mono" style="color:{d.market_day ? 'var(--muted-2)' : '#4e535b'}">
                {d.market_day ? d.state || 'missing' : 'holiday'}
              </div>
            </div>
          {/each}
        </div>
        <div class="legend">
          <span><i style="background:#3a3e46"></i>missing</span>
          <span><i style="background:var(--blue)"></i>on disk</span>
          <span><i style="background:var(--green)"></i>ready/loaded</span>
          <span><i style="background:#2a2d33"></i>closed</span>
        </div>
      {:else}
        <div class="batch">
          <div class="lbl">Date range</div>
          <div class="range">
            <label>from <input type="date" bind:value={from} class="mono" /></label>
            <label>to <input type="date" bind:value={to} class="mono" /></label>
          </div>
          <div class="rnote mono">{batchDates.length} weekday{batchDates.length === 1 ? '' : 's'} · holidays skipped on the server</div>
        </div>
      {/if}
    </div>

    <!-- right: selection + action -->
    <div class="col">
      <div class="card pad">
        <div class="lbl">Tickers</div>
        <div class="chips">
          {#each tickers as t (t)}
            <button class="chip mono" class:on={selTickers.has(t)} onclick={() => (selTickers = toggleSet(selTickers, t))}>{t}</button>
          {/each}
        </div>
        <div class="lbl mt">Packages</div>
        <div class="chips">
          {#each packages as p (p)}
            <button class="chip mono" class:on={selPackages.has(p)} onclick={() => (selPackages = toggleSet(selPackages, p))}>{p}</button>
          {/each}
        </div>
      </div>

      <div class="card pad">
        <div class="row"><span>Days</span><span class="mono">{chosenDates.length}</span></div>
        <div class="row"><span>Tickers × packages</span><span class="mono">{selTickers.size} × {selPackages.size}</span></div>
        <button class="dl" class:ready={canDownload} disabled={!canDownload} onclick={startDownload}>
          {chosenDates.length ? `Download ${chosenDates.length} day${chosenDates.length === 1 ? '' : 's'}` : 'Pick day(s)'}
        </button>
      </div>
    </div>
  </div>

  {#if jobs.length}
    <div class="card queue">
      <div class="qhead">
        <div class="qtitle">Download queue</div>
        <div class="qsub mono">{busy ? 'downloading…' : 'idle'}</div>
      </div>
      {#each [...jobs].sort((a, b) => (a.date < b.date ? 1 : -1)) as j (j.date)}
        <div class="qrow">
          <div class="mono qdate">{j.date}</div>
          <div class="bar"><div class="fill" style="width:{j.total ? Math.round((j.done / j.total) * 100) : (j.state === 'done' ? 100 : 0)}%;background:{jobColor(j.state)}"></div></div>
          <div class="mono qstate" style="color:{jobColor(j.state)}">{j.state}{j.total ? ` ${j.done}/${j.total}` : ''}</div>
          <div class="mono qres">{j.state === 'done' || j.state === 'partial' ? `${j.success} ok · ${j.skipped} skip · ${j.not_found} n/f${j.failed ? ` · ${j.failed} fail` : ''}` : j.error ?? ''}</div>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .wrap {
    padding: 18px 20px 28px;
    display: flex;
    flex-direction: column;
    gap: 14px;
  }
  .notice {
    border: 1px solid #4a3a20;
    background: #1a1710;
    color: var(--amber);
    border-radius: 8px;
    padding: 10px 14px;
    font-size: 12px;
  }
  .notice code {
    color: var(--green-2);
  }
  .err {
    border: 1px solid #4a2b2b;
    background: #1a1313;
    color: var(--red);
    border-radius: 8px;
    padding: 10px 14px;
    font-size: 12px;
  }
  .head {
    display: flex;
    align-items: flex-start;
    gap: 12px;
  }
  .title {
    font-size: 14px;
    font-weight: 600;
    margin-bottom: 4px;
  }
  .sub {
    font-size: 12px;
    color: var(--muted);
    line-height: 1.5;
    max-width: 620px;
  }
  .flex-1 {
    flex: 1;
  }
  .toggle {
    display: flex;
    border: 1px solid var(--border-2);
    border-radius: 7px;
    overflow: hidden;
    flex: none;
  }
  .tg {
    padding: 6px 12px;
    font-size: 11.5px;
    background: var(--input);
    color: var(--muted);
    border: none;
    cursor: pointer;
  }
  .tg.on {
    background: #2a2f36;
    color: #fff;
  }
  .grid2 {
    display: grid;
    grid-template-columns: minmax(340px, 1fr) minmax(260px, 320px);
    gap: 16px;
    align-items: start;
  }
  .picker {
    overflow: hidden;
  }
  .cal-head {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 12px;
    border-bottom: 1px solid var(--border);
  }
  .nav {
    background: var(--input);
    border: 1px solid var(--border-2);
    color: var(--muted);
    border-radius: 5px;
    width: 24px;
    height: 24px;
    cursor: pointer;
  }
  .month {
    font-size: 12.5px;
    font-weight: 600;
  }
  .mini {
    font-size: 10.5px;
    padding: 4px 8px;
    border: 1px solid var(--border-2);
    border-radius: 5px;
    color: #9aa0a9;
    background: transparent;
    cursor: pointer;
  }
  .cal {
    display: grid;
    grid-template-columns: repeat(5, minmax(0, 1fr));
    gap: 1px;
    background: #1a1d21;
    padding: 1px;
  }
  .wd {
    background: var(--panel-2);
    padding: 7px 10px;
    font-size: 10.5px;
    color: var(--muted-2);
  }
  .blank {
    background: #101215;
    min-height: 62px;
  }
  .cell {
    background: var(--panel-2);
    padding: 8px 9px;
    min-height: 62px;
    display: flex;
    flex-direction: column;
    gap: 5px;
    cursor: default;
  }
  .cell.clickable {
    cursor: pointer;
  }
  .cell.clickable:hover {
    background: #16181c;
  }
  .cell.sel {
    background: #16211b;
    box-shadow: inset 2px 0 0 var(--green);
  }
  .cell-top {
    display: flex;
    align-items: center;
  }
  .num {
    font-size: 12px;
    flex: 1;
  }
  .dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
  }
  .cell-state {
    font-size: 10px;
  }
  .legend {
    display: flex;
    gap: 14px;
    padding: 9px 12px;
    border-top: 1px solid var(--border);
    font-size: 10.5px;
    color: var(--muted-2);
  }
  .legend span {
    display: flex;
    align-items: center;
    gap: 5px;
  }
  .legend i {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    display: inline-block;
  }
  .batch {
    padding: 14px;
  }
  .range {
    display: flex;
    gap: 10px;
  }
  .range label {
    font-size: 11px;
    color: var(--muted-2);
    display: flex;
    flex-direction: column;
    gap: 5px;
  }
  .range input {
    background: var(--input);
    border: 1px solid var(--border-2);
    border-radius: 6px;
    color: var(--text-2);
    padding: 6px 8px;
    font-size: 11.5px;
  }
  .rnote {
    margin-top: 10px;
    font-size: 10.5px;
    color: var(--dim);
  }
  .col {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }
  .pad {
    padding: 14px;
  }
  .lbl {
    font-size: 11px;
    color: var(--muted-2);
    margin-bottom: 7px;
  }
  .mt {
    margin-top: 14px;
  }
  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }
  .chip {
    padding: 4px 9px;
    border-radius: 6px;
    font-size: 11px;
    cursor: pointer;
    border: 1px solid var(--border-2);
    background: var(--input);
    color: var(--muted);
  }
  .chip.on {
    border-color: var(--green);
    background: var(--green-bg);
    color: var(--green);
  }
  .row {
    display: flex;
    justify-content: space-between;
    font-size: 12px;
    color: var(--muted);
    margin-bottom: 8px;
  }
  .dl {
    width: 100%;
    text-align: center;
    padding: 9px;
    border-radius: 7px;
    background: #1b1e23;
    color: var(--dimmer);
    border: none;
    font-size: 12.5px;
    font-weight: 600;
    cursor: default;
    margin-top: 4px;
  }
  .dl.ready {
    background: var(--green);
    color: var(--green-ink);
    cursor: pointer;
  }
  .queue {
    overflow: hidden;
  }
  .qhead {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 11px 14px;
    border-bottom: 1px solid var(--border);
  }
  .qtitle {
    font-size: 12.5px;
    font-weight: 600;
  }
  .qsub {
    font-size: 11px;
    color: var(--muted-2);
  }
  .qrow {
    display: grid;
    grid-template-columns: 96px minmax(0, 1fr) 110px 140px;
    gap: 12px;
    align-items: center;
    padding: 10px 14px;
    border-bottom: 1px solid #191c20;
  }
  .qdate {
    font-size: 11.5px;
  }
  .bar {
    height: 6px;
    border-radius: 3px;
    background: #1e2126;
    overflow: hidden;
  }
  .fill {
    height: 100%;
    border-radius: 3px;
    transition: width 0.4s ease;
  }
  .qstate {
    font-size: 11px;
  }
  .qres {
    font-size: 10.5px;
    color: var(--dim);
    text-align: right;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
