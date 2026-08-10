<script lang="ts">
  import { onMount } from 'svelte'
  import { fmtBytes } from '../lib/api'

  interface LogLine {
    time: string
    level: string
    service: string
    msg: string
    caller?: string
    fields?: Record<string, unknown>
  }
  type Row = LogLine & { _id: number }

  const LEVELS = ['all', 'info', 'warn', 'error', 'debug'] as const
  const SERVICES = [
    { id: 'all', label: 'all' },
    { id: 'gex-faker-api', label: 'api' },
    { id: 'gex-daemon', label: 'daemon' },
  ]
  const RANGES = ['5m', '1h', '6h'] as const

  let rows = $state<Row[]>([])
  let level = $state<(typeof LEVELS)[number]>('all')
  let service = $state('all')
  let range = $state<(typeof RANGES)[number]>('5m')
  let search = $state('')
  let hideAccess = $state(true)
  let expanded = $state<Set<number>>(new Set())
  let connected = $state(false)
  let errorMsg = $state('')
  let volume = $state<{ t: number; v: number }[]>([])

  const MAX = 1500
  let nextId = 0
  let es: EventSource | null = null
  let searchTimer: ReturnType<typeof setTimeout> | undefined

  // service + hideAccess are applied server-side (in the Loki query, so a sparse
  // service like the daemon isn't crowded out of the backfill); only the level
  // filter is client-side.
  const shown = $derived(
    level === 'all' ? rows : rows.filter((r) => r.level.toLowerCase() === level),
  )
  const errPerMin = $derived(volume.reduce((a, p) => a + p.v, 0))

  function connect() {
    es?.close()
    rows = []
    expanded = new Set()
    connected = false
    const qs = new URLSearchParams({ range, search, service, hide_access: hideAccess ? '1' : '0' })
    es = new EventSource(`/studio/api/logs?${qs}`)
    es.onopen = () => {
      connected = true
      errorMsg = ''
    }
    es.onmessage = (e) => {
      try {
        const d = JSON.parse(e.data)
        if (d.error) {
          errorMsg = d.error
          return
        }
        rows.push({ ...(d as LogLine), _id: nextId++ })
        if (rows.length > MAX) rows = rows.slice(-MAX)
      } catch {
        /* ignore */
      }
    }
    es.onerror = () => (connected = false)
  }

  async function fetchVolume() {
    try {
      const r = await fetch(`/studio/api/logs/volume?range=${range}`)
      const d = await r.json()
      volume = d.points ?? []
    } catch {
      volume = []
    }
  }

  function setRange(r: (typeof RANGES)[number]) {
    range = r
    connect()
    fetchVolume()
  }
  function setService(s: string) {
    service = s
    connect()
  }
  function toggleAccess() {
    hideAccess = !hideAccess
    connect()
  }
  function onSearch(v: string) {
    search = v
    clearTimeout(searchTimer)
    searchTimer = setTimeout(connect, 350)
  }
  function toggle(id: number) {
    const s = new Set(expanded)
    s.has(id) ? s.delete(id) : s.add(id)
    expanded = s
  }

  onMount(() => {
    connect()
    fetchVolume()
    return () => es?.close()
  })

  function levelColor(l: string): string {
    switch (l.toLowerCase()) {
      case 'error':
      case 'fatal':
      case 'panic':
        return 'var(--red)'
      case 'warn':
      case 'warning':
        return 'var(--amber)'
      case 'debug':
        return 'var(--dim)'
      default:
        return 'var(--muted)'
    }
  }
  function statusColor(s: number): string {
    if (s >= 500) return 'var(--red)'
    if (s >= 400) return 'var(--amber)'
    if (s >= 200 && s < 300) return 'var(--green)'
    return 'var(--muted)'
  }
  function fmtDur(s: unknown): string {
    if (typeof s !== 'number') return ''
    if (s < 0.001) return `${(s * 1e6).toFixed(0)}µs`
    if (s < 1) return `${(s * 1000).toFixed(1)}ms`
    return `${s.toFixed(2)}s`
  }
  function short(v: unknown): string {
    const s = typeof v === 'object' ? JSON.stringify(v) : String(v)
    return s.length > 60 ? s.slice(0, 60) + '…' : s
  }
  // Non-request fields as compact key=value tags.
  function tags(f?: Record<string, unknown>): [string, string][] {
    if (!f) return []
    return Object.entries(f).map(([k, v]) => [k, short(v)])
  }
</script>

<div class="wrap">
  <div class="toolbar">
    <div class="chips">
      {#each LEVELS as l (l)}
        <button class="chip mono" class:on={level === l} onclick={() => (level = l)}>{l}</button>
      {/each}
    </div>
    <span class="sep"></span>
    <div class="chips">
      {#each SERVICES as s (s.id)}
        <button class="chip mono" class:on={service === s.id} onclick={() => setService(s.id)}
          >{s.label}</button
        >
      {/each}
    </div>
    <span class="sep"></span>
    <div class="chips">
      {#each RANGES as r (r)}
        <button class="chip mono" class:on={range === r} onclick={() => setRange(r)}>{r}</button>
      {/each}
    </div>
    <input
      class="search mono"
      placeholder="search Loki…"
      value={search}
      oninput={(e) => onSearch((e.target as HTMLInputElement).value)}
    />
    <label class="toggle"><input type="checkbox" checked={hideAccess} onchange={toggleAccess} /> hide access logs</label>
    <div class="flex-1"></div>
    {#if volume.length}
      <div class="spark" title={`${errPerMin} error/warn lines in ${range}`}>
        {#each volume as p (p.t)}
          <span
            class="bar"
            style="height:{Math.max(2, Math.min(18, p.v * 4))}px;background:{p.v > 0
              ? 'var(--amber)'
              : '#2a2d33'}"
          ></span>
        {/each}
        <span class="spark-label mono">{errPerMin} err/warn</span>
      </div>
    {/if}
    <span class="conn mono" style="color:{connected ? 'var(--green)' : 'var(--muted-2)'}"
      >{connected ? 'live' : 'connecting…'}</span
    >
    <button class="btn" onclick={() => (rows = [])}>Clear</button>
  </div>

  {#if errorMsg}
    <div class="notice">
      {errorMsg}
      <div class="hint mono">
        Run <code>just observability-up</code> (or ensure the Loki container is running), then reopen
        this screen.
      </div>
    </div>
  {/if}

  <div class="viewer mono">
    {#if shown.length === 0}
      <div class="empty">{errorMsg ? 'No log feed.' : 'Waiting for logs…'}</div>
    {:else}
      {#each shown as r (r._id)}
        <div
          class="line"
          onclick={() => toggle(r._id)}
          onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && toggle(r._id)}
          role="button"
          tabindex="-1"
        >
          <span class="t">{r.time}</span>
          <span class="lvl" style="color:{levelColor(r.level)}">{r.level.toUpperCase()}</span>
          <span class="svc">{r.service.replace('gex-', '')}</span>
          <span class="body">
            {#if r.msg === 'request completed' && r.fields}
              <span class="method">{r.fields.method}</span>
              <span class="route">{r.fields.route}</span>
              <span class="arrow">→</span>
              <span style="color:{statusColor(Number(r.fields.status))}">{r.fields.status}</span>
              <span class="meta">· {fmtDur(r.fields.duration)} · {fmtBytes(Number(r.fields.response_bytes))}</span>
            {:else}
              <span class="msg" style="color:{levelColor(r.level) === 'var(--muted)' ? 'var(--text-2)' : levelColor(r.level)}">{r.msg}</span>
              {#each tags(r.fields) as [k, v] (k)}
                <span class="tag"><span class="k">{k}</span>=<span class="v">{v}</span></span>
              {/each}
            {/if}
          </span>
          {#if r.caller}<span class="caller">{r.caller}</span>{/if}
        </div>
        {#if expanded.has(r._id) && r.fields}
          <pre class="detail">{JSON.stringify(r.fields, null, 2)}</pre>
        {/if}
      {/each}
    {/if}
  </div>
</div>

<style>
  .wrap {
    padding: 18px 20px 28px;
    display: flex;
    flex-direction: column;
    gap: 12px;
    height: 100%;
  }
  .toolbar {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
  }
  .chips {
    display: flex;
    gap: 6px;
  }
  .sep {
    width: 1px;
    height: 16px;
    background: var(--border-2);
  }
  .chip {
    font-size: 11px;
    padding: 4px 9px;
    border-radius: 6px;
    cursor: pointer;
    border: 1px solid var(--border-2);
    background: var(--input);
    color: var(--muted);
  }
  .chip.on {
    border-color: #3a3e46;
    background: #1f242a;
    color: #fff;
  }
  .search {
    font-size: 11px;
    padding: 5px 9px;
    border-radius: 6px;
    border: 1px solid var(--border-2);
    background: var(--input);
    color: var(--text);
    width: 150px;
  }
  .toggle {
    font-size: 11px;
    color: var(--muted-2);
    display: flex;
    align-items: center;
    gap: 5px;
    cursor: pointer;
  }
  .flex-1 {
    flex: 1;
  }
  .spark {
    display: flex;
    align-items: flex-end;
    gap: 1px;
    height: 20px;
    padding: 0 8px;
  }
  .bar {
    width: 2px;
    border-radius: 1px;
  }
  .spark-label {
    align-self: center;
    font-size: 9.5px;
    color: var(--muted-2);
    margin-left: 6px;
  }
  .conn {
    font-size: 10.5px;
  }
  .notice {
    border: 1px solid #4a3a20;
    background: #1a1710;
    color: var(--amber);
    border-radius: 8px;
    padding: 10px 14px;
    font-size: 12px;
  }
  .notice .hint {
    color: var(--muted-2);
    font-size: 10.5px;
    margin-top: 6px;
  }
  .notice code {
    color: var(--green-2);
  }
  .viewer {
    flex: 1;
    min-height: 300px;
    border: 1px solid var(--border);
    border-radius: 10px;
    background: var(--bg-log);
    padding: 10px 14px;
    overflow: auto;
    font-size: 11px;
    line-height: 1.8;
  }
  .empty {
    color: var(--muted-2);
  }
  .line {
    display: flex;
    gap: 10px;
    align-items: baseline;
    white-space: nowrap;
    cursor: pointer;
    border-radius: 3px;
  }
  .line:hover {
    background: #14161a;
  }
  .t {
    color: #4e535b;
    flex: none;
  }
  .lvl {
    flex: none;
    width: 42px;
  }
  .svc {
    color: var(--dim);
    flex: none;
    width: 58px;
  }
  .body {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .method {
    color: var(--blue);
  }
  .route {
    color: var(--text-2);
  }
  .arrow,
  .meta {
    color: var(--dim);
  }
  .msg {
    color: var(--text-2);
  }
  .tag {
    color: var(--dim);
    margin-left: 8px;
  }
  .tag .k {
    color: var(--muted-2);
  }
  .tag .v {
    color: #9aa0a9;
  }
  .caller {
    flex: none;
    color: #3f444c;
    font-size: 10px;
  }
  .detail {
    margin: 2px 0 6px 62px;
    padding: 8px 10px;
    background: #101215;
    border: 1px solid var(--border);
    border-radius: 6px;
    color: var(--muted);
    font-size: 10.5px;
    white-space: pre-wrap;
    overflow-x: auto;
  }
</style>
