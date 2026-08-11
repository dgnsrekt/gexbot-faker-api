// Typed fetch wrappers for the GEX Faker Studio control-plane. Paths are absolute
// from the server root (the SPA is served at /studio but the API lives at the
// root, e.g. /studio/api/* and /reload-date).

export interface Status {
  running: boolean
  loaded_date: string
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
  snapshots: number // avg per-ticker intraday snapshot count (coverage signal; ~23.4k = full 1/s session)
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

export interface CalendarDay {
  date: string
  day: number
  weekday: number // 0=Sun..6=Sat
  market_day: boolean
  holiday: boolean
  state: string // loaded|ready|archived|missing|"" (non-market)
}

export interface DownloadOptions {
  enabled: boolean
  tickers: string[]
  packages: { name: string; categories: string[] }[]
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

export const api = {
  status: () => getJSON<Status>('/studio/api/status'),
  config: () => getJSON<SettingGroup[]>('/studio/api/config'),
  hubs: () => getJSON<HubStat[]>('/studio/api/hubs'),
  library: () => getJSON<LibraryRow[]>('/studio/api/library'),
  keys: () => getJSON<KeyEntry[]>('/studio/api/keys'),
  endpoints: () => getJSON<EndpointDoc[]>('/studio/api/endpoints'),
  tickers: () => getJSON<TickersResp>('/tickers'),
  categories: (pkg: string) => getJSON<string[]>(`/${pkg}/categories`),
  reloadDate: (date: string) => postJSON<{ new_date: string }>('/reload-date', { date }),
  resetCache: () => postJSON<{ count: number }>('/reset-cache', {}),
  materialize: (date: string) => postJSON<MaterializeJob>('/studio/api/materialize', { date }),
  downloadOptions: () => getJSON<DownloadOptions>('/studio/api/download/options'),
  calendar: (month: string) => getJSON<{ month: string; days: CalendarDay[] }>(`/studio/api/calendar?month=${month}`),
  download: (dates: string[], tickers: string[], packages: string[]) =>
    postJSON<DownloadJob[]>('/studio/api/download', { dates, tickers, packages }),
  downloadStatus: () => getJSON<DownloadJob[]>('/studio/api/download'),
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
