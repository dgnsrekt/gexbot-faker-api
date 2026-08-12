// Typed fetch wrappers for the GEX Faker Studio control-plane. Paths are absolute
// from the server root (the SPA is served at /studio but the API lives at the
// root, e.g. /studio/api/* and /load).

export interface Status {
  running: boolean
  loaded_date: string // span anchor day (single day for a single-day load)
  loaded_dates: string[] // the full loaded span, chronological (issue #66)
  loaded_at: string
  files_loaded: number
  tickers: string[]
  data_mode: string
  cache_mode: string
  endpoint_cache_mode: string
  is_reloading: boolean
  disk_bytes: number
  group_prefix: string
  ws_enabled: boolean
  port: string
}

export interface SettingRow {
  label: string
  help: string
  env: string
  value: string
}
export interface SettingGroup {
  title: string
  sub: string
  rows: SettingRow[]
}

export interface HubStat {
  name: string
  clients: number
  active_groups: string[]
  interval: string
}

export type LibraryState = 'loaded' | 'ready' | 'archived' | 'materializing'

export interface LibraryRow {
  date: string
  tickers: string[]
  packages: string[]
  size_bytes: number
  records: number
  coverage: number // composition-stable coverage index (~1.0 = every ticker at its own snapshot norm; <1 = drop; 0 = unknown)
  snapshots_by_ticker?: Record<string, number> // per-ticker intraday snapshot counts
  status: string // "ok" | "corrupt" (archive integrity)
  materialized: number // tickers with JSONL on disk
  total: number // archived tickers for the date
  loaded: boolean
  state: LibraryState
  job_error?: string // last background-materialize failure for this date, if any
}

export interface MaterializeJob {
  date: string
  state: 'running' | 'done' | 'error'
  error?: string
}

// LoadJob is the async POST /load response, polled via GET /load/status/{job_id}.
export interface LoadJob {
  job_id: string
  dates?: string[]
  state: 'queued' | 'running' | 'done' | 'error'
  done?: number
  total?: number
  error?: string
}

export interface CalendarDay {
  date: string
  day: number
  weekday: number // 0=Sun..6=Sat
  market_day: boolean
  holiday: boolean
  state: string // loaded|ready|archived|missing|"" (non-market)
}

export interface DownloadPackage {
  name: string
  categories: string[]
}

// DownloadOptions is the effective, YAML-authoritative download coverage the
// Download screen renders read-only (the user chooses only dates).
export interface DownloadOptions {
  enabled: boolean
  config_path: string
  tickers: string[]
  packages: DownloadPackage[]
  message?: string // why downloads are disabled, when enabled=false
}

// DaemonStatus is the sanitized daemon status the Studio proxies from the daemon's
// internal /status (never any secret). config unavailable → {available:false}.
export interface DaemonStatus {
  ready: boolean
  in_progress: boolean
  last_result?: string
  last_error?: string
  last_run_at?: string
  last_downloaded_date?: string
  config_path: string
  output_dir: string
  tickers: string[]
  packages: DownloadPackage[]
  schedule: {
    hour: number
    minute: number
    timezone: string
    run_on_startup: boolean
    run_timeout_minutes: number
  }
  cleanup: { enabled: boolean; retention_days: number }
  notifications: { enabled: boolean; server: string; topic: string; priority: string; tags: string }
}

export interface DownloadJob {
  date: string
  state: 'queued' | 'running' | 'done' | 'partial' | 'error'
  done: number
  total: number
  success: number
  skipped: number
  failed: number
  not_found: number
  error?: string
}

export interface KeyStream {
  data_key: string
  index: number
}
export interface KeyEntry {
  key: string
  streams: KeyStream[]
}

export interface EndpointDoc {
  method: string
  path: string
  desc: string
}

export interface TickersResp {
  stocks: string[]
  indexes: string[]
  futures: string[]
}

async function getJSON<T>(path: string): Promise<T> {
  const res = await fetch(path, { headers: { Accept: 'application/json' } })
  if (!res.ok) throw new Error(`${path} → ${res.status}`)
  return res.json() as Promise<T>
}

async function postJSON<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `${path} → ${res.status}`)
  }
  return res.json() as Promise<T>
}

// pollLoad polls /load/status to completion so a POST /load behaves synchronously for the caller
// (a single day finishes fast; a span materializes then loads). A still-running job past the budget
// is a timeout, not success — surfaced so the caller doesn't clear its busy state as though loaded.
async function pollLoad(job: LoadJob): Promise<LoadJob> {
  let j = job
  for (let i = 0; i < 240 && j.state !== 'done' && j.state !== 'error'; i++) {
    await new Promise((r) => setTimeout(r, 500))
    j = await getJSON<LoadJob>(`/load/status/${job.job_id}`)
  }
  if (j.state === 'error') throw new Error(j.error || 'load failed')
  if (j.state !== 'done') throw new Error('load did not finish in time (still running server-side)')
  return j
}

export const api = {
  status: () => getJSON<Status>('/studio/api/status'),
  config: () => getJSON<SettingGroup[]>('/studio/api/config'),
  hubs: () => getJSON<HubStat[]>('/studio/api/hubs'),
  library: () => getJSON<LibraryRow[]>('/studio/api/library'),
  keys: () => getJSON<KeyEntry[]>('/studio/api/keys'),
  endpoints: () => getJSON<EndpointDoc[]>('/studio/api/endpoints'),
  tickers: () => getJSON<TickersResp>('/tickers'),
  categories: (pkg: string) => getJSON<string[]>(`/${pkg}/categories`),
  // load kicks off the async POST /load for a single day and polls to completion, so the Library
  // "Load" button behaves synchronously (a single-day load finishes fast).
  load: async (date: string): Promise<LoadJob> => pollLoad(await postJSON<LoadJob>('/load', { date })),
  // loadRange loads a contiguous span ({from,to}) or an explicit {dates} list as one cross-day
  // dataset, polling to completion — the Studio "Load span" control. The loaded badges then span
  // every day in the range (issue #66).
  loadRange: async (body: { from?: string; to?: string; dates?: string[] }): Promise<LoadJob> =>
    pollLoad(await postJSON<LoadJob>('/load', body)),
  reset: () => postJSON<{ count: number }>('/reset', {}),
  materialize: (date: string) => postJSON<MaterializeJob>('/studio/api/materialize', { date }),
  downloadOptions: () => getJSON<DownloadOptions>('/studio/api/download/options'),
  calendar: (month: string) => getJSON<{ month: string; days: CalendarDay[] }>(`/studio/api/calendar?month=${month}`),
  // Coverage is the server's YAML authority — only dates are sent.
  download: (dates: string[]) => postJSON<DownloadJob[]>('/studio/api/download', { dates }),
  downloadStatus: () => getJSON<DownloadJob[]>('/studio/api/download'),
  daemon: () => getJSON<DaemonStatus>('/studio/api/daemon'),
  metricsQuery: (query: string) =>
    getJSON<PromResponse>(`/studio/api/metrics/query?query=${encodeURIComponent(query)}`),
  // metricsRange runs a range query over the last `minutes`, sampled every `step` seconds.
  metricsRange: (query: string, minutes: number, step: number) => {
    const end = Math.floor(Date.now() / 1000)
    const start = end - minutes * 60
    const qs = `query=${encodeURIComponent(query)}&start=${start}&end=${end}&step=${step}`
    return getJSON<PromResponse>(`/studio/api/metrics/range?${qs}`)
  },
  // metricsAlerts returns Prometheus's active alerts (from the rule set).
  metricsAlerts: () => getJSON<PromAlertsResponse>('/studio/api/metrics/alerts'),
}

// Prometheus /api/v1/alerts.
export interface PromAlert {
  labels: Record<string, string>
  annotations: Record<string, string>
  state: 'firing' | 'pending' | 'inactive'
  activeAt?: string
}
export interface PromAlertsResponse {
  status: 'success' | 'error'
  error?: string
  data?: { alerts: PromAlert[] }
}

// activeAlerts returns firing/pending alerts, firing first.
export function activeAlerts(r: PromAlertsResponse): PromAlert[] {
  if (r.status !== 'success' || !r.data?.alerts) return []
  return r.data.alerts
    .filter((a) => a.state === 'firing' || a.state === 'pending')
    .sort((a, b) => (a.state === b.state ? 0 : a.state === 'firing' ? -1 : 1))
}

// PromResponse mirrors Prometheus's query API envelope (proxied server-side).
export interface PromResponse {
  status: 'success' | 'error'
  error?: string
  data?: { resultType: string; result: PromSample[] }
}
export interface PromSample {
  metric: Record<string, string>
  value?: [number, string] // instant query: [unixSeconds, sampleValue]
  values?: [number, string][] // range query: [[unixSeconds, sampleValue], ...]
}

// promScalar returns the single sample value of an instant query, or null.
export function promScalar(r: PromResponse): number | null {
  const v = r.data?.result?.[0]?.value?.[1]
  return v === undefined ? null : Number(v)
}

// A chart series: a label plus [unixSeconds, value] points.
export interface Series {
  name: string
  points: [number, number][]
}

// promToSeries turns a range-query response into chart series, labeling each by
// the given metric label (e.g. "ticker") or falling back to a static name.
// Prometheus serializes gaps as "NaN"/"+Inf" strings (e.g. histogram_quantile
// over an idle window); those are dropped so one bad point can't poison the
// chart's min/max scaling, and a series left with no finite points is omitted.
export function promToSeries(r: PromResponse, label?: string, fallback = ''): Series[] {
  if (r.status !== 'success' || !r.data?.result) return []
  return r.data.result
    .map((s) => ({
      name: (label && s.metric[label]) || fallback || Object.values(s.metric).join(' ') || 'value',
      points: (s.values || [])
        .map(([t, v]) => [t, Number(v)] as [number, number])
        .filter((p) => Number.isFinite(p[1])),
    }))
    .filter((s) => s.points.length > 0)
}

export function fmtBytes(n: number): string {
  if (!n) return '—'
  const u = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let v = n
  while (v >= 1024 && i < u.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(v >= 100 || i === 0 ? 0 : 1)} ${u[i]}`
}

export function fmtInt(n: number): string {
  return n.toLocaleString('en-US')
}

// fmtLoadedSpan renders what's loaded: a single day as itself, a multi-day span as
// "first → last". Prefers loaded_dates (the full span, issue #66), falling back to the
// scalar loaded_date for older servers.
export function fmtLoadedSpan(s: Status | null | undefined): string {
  const dates = s?.loaded_dates
  if (dates && dates.length > 0) {
    return dates.length === 1 ? dates[0] : `${dates[0]} → ${dates[dates.length - 1]}`
  }
  return s?.loaded_date ?? '—'
}
