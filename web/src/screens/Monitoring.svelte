<script lang="ts">
  import { onMount } from 'svelte'
  import {
    api,
    promScalar,
    promToSeries,
    activeAlerts,
    fmtBytes,
    type Series,
    type PromAlert,
  } from '../lib/api'
  import MetricChart from '../lib/MetricChart.svelte'

  type Tone = 'ok' | 'warn' | 'bad' | ''
  type Tile = { label: string; value: string; tone: Tone }

  let tiles = $state<Tile[]>([])
  let resourceTiles = $state<Tile[]>([])
  let jobTiles = $state<Tile[]>([])

  // The long server-side async jobs (faker_jobs_* metrics). Known, bounded kinds.
  const JOB_KINDS: [string, string][] = [
    ['materialize', 'Materialize'],
    ['range_load', 'Range load'],
    ['studio_download', 'Download'],
  ]
  let degraded = $state('') // non-empty → Prometheus unset/unreachable
  let loading = $state(true)

  // Chart series (range queries).
  let snapshots = $state<Series[]>([])
  let reqRate = $state<Series[]>([])
  let latency = $state<Series[]>([])
  let wsConns = $state<Series[]>([])
  let alerts = $state<PromAlert[]>([])
  let alertsErr = $state(false) // alerts endpoint failed → unknown, not "all clear"

  const fmtPerSec = (v: number) => v.toFixed(2) + '/s'
  const fmtMs = (v: number) => Math.round(v * 1000) + 'ms'
  const fmtInt = (v: number) => String(Math.round(v))

  const alertName = (a: PromAlert) => a.labels.alertname || 'alert'
  const alertSummary = (a: PromAlert) => a.annotations.summary || alertName(a)
  const alertSeverity = (a: PromAlert) => a.labels.severity || 'warning'
  function alertSince(a: PromAlert): string {
    if (!a.activeAt) return ''
    const s = (Date.now() - new Date(a.activeAt).getTime()) / 1000
    if (s < 90) return 'just now'
    if (s < 5400) return `${Math.round(s / 60)}m`
    if (s < 172800) return `${(s / 3600).toFixed(1)}h`
    return `${Math.round(s / 86400)}d`
  }

  function fmtAge(s: number): string {
    if (s < 0) return '—'
    if (s < 90) return `${Math.round(s)}s ago`
    if (s < 5400) return `${Math.round(s / 60)}m ago`
    if (s < 172800) return `${(s / 3600).toFixed(1)}h ago`
    return `${Math.round(s / 86400)}d ago`
  }
  function fmtIn(s: number): string {
    if (s <= 0) return 'due now'
    if (s < 5400) return `in ${Math.round(s / 60)}m`
    return `in ${(s / 3600).toFixed(1)}h`
  }
  function fmtUptime(s: number): string {
    if (s < 3600) return `${Math.round(s / 60)}m`
    if (s < 172800) return `${Math.floor(s / 3600)}h ${Math.round((s % 3600) / 60)}m`
    return `${Math.floor(s / 86400)}d ${Math.round((s % 86400) / 3600)}h`
  }
  const GiB = 1024 * 1024 * 1024

  async function load() {
    try {
      const [lastAge, nextIn, ok, fail, overdueQ] = await Promise.all([
        api.metricsQuery('time() - faker_daemon_last_success_timestamp_seconds{job="faker-daemon"}'),
        api.metricsQuery('faker_daemon_next_run_timestamp_seconds{job="faker-daemon"} - time()'),
        api.metricsQuery('sum(faker_daemon_download_runs_total{result="success"})'),
        api.metricsQuery('sum(faker_daemon_download_runs_total{result!="success"})'),
        api.metricsQuery('faker_daemon_download_overdue{job="faker-daemon"}'),
      ])
      const bad = [lastAge, nextIn, ok, fail, overdueQ].find((r) => r.status === 'error')
      if (bad) {
        degraded = bad.error || 'Prometheus is unavailable.'
        tiles = []
        return
      }
      degraded = ''
      const age = promScalar(lastAge)
      const nxt = promScalar(nextIn)
      const oks = promScalar(ok)
      const fails = promScalar(fail)
      const overdue = promScalar(overdueQ)
      tiles = [
        { label: 'Last successful download', value: age === null ? 'no data yet' : fmtAge(age), tone: '' },
        // Market-aware freshness: the daemon owns the NYSE calendar and sets this to 1 only
        // when it's behind the latest market date whose scheduled time has passed — so this
        // tile stays green over weekends/holidays (no false alarm).
        {
          label: 'Data freshness',
          value: overdue === null ? '—' : overdue >= 1 ? 'overdue' : 'up to date',
          tone: overdue === null ? '' : overdue >= 1 ? 'bad' : 'ok',
        },
        { label: 'Next scheduled run', value: nxt === null ? '—' : fmtIn(nxt), tone: '' },
        {
          label: 'Failed runs',
          value: fails === null ? '0' : String(fails),
          tone: fails && fails > 0 ? 'bad' : 'ok',
        },
        { label: 'Successful runs', value: oks === null ? '—' : String(oks), tone: '' },
      ]

      // Charts (range queries). Snapshots trend over a week; the API/WS series
      // over the last hour. Each is isolated: a failing range query must only
      // empty its own panel — never blank the daemon tiles or the other charts,
      // and never leave the panel showing a prior refresh's stale data.
      const chart = (
        q: string,
        minutes: number,
        step: number,
        set: (s: Series[]) => void,
        label?: string,
        fallback?: string,
      ) =>
        api
          .metricsRange(q, minutes, step)
          .then((r) => set(promToSeries(r, label, fallback)))
          .catch(() => set([]))
      // Resource USE (server process + data volume), isolated like the charts so a
      // failure only empties this panel. Disk free is the one with a health tone — the
      // repo's characteristic failure mode.
      const resources = Promise.all([
        api.metricsQuery('faker_data_volume_free_bytes{job="faker-api"}'),
        api.metricsQuery('faker_data_volume_total_bytes{job="faker-api"}'),
        api.metricsQuery('process_resident_memory_bytes{job="faker-api"}'),
        api.metricsQuery('go_goroutines{job="faker-api"}'),
        api.metricsQuery('process_open_fds{job="faker-api"}'),
        api.metricsQuery('time() - process_start_time_seconds{job="faker-api"}'),
      ])
        .then(([free, total, rss, gor, fds, up]) => {
          const f = promScalar(free)
          const tt = promScalar(total)
          const usedPct = f !== null && tt ? Math.round((1 - f / tt) * 100) : null
          const r = promScalar(rss)
          const g = promScalar(gor)
          const fd = promScalar(fds)
          const u = promScalar(up)
          resourceTiles = [
            {
              label: 'Data volume free',
              value: f === null ? '—' : fmtBytes(f),
              tone: f === null ? '' : f < 3 * GiB ? 'bad' : f < 10 * GiB ? 'warn' : 'ok',
            },
            { label: 'Data volume used', value: usedPct === null ? '—' : `${usedPct}%`, tone: '' },
            { label: 'Memory (RSS)', value: r === null ? '—' : fmtBytes(r), tone: '' },
            { label: 'Goroutines', value: g === null ? '—' : fmtInt(g), tone: '' },
            { label: 'Open files', value: fd === null ? '—' : fmtInt(fd), tone: '' },
            { label: 'Uptime', value: u === null ? '—' : fmtUptime(u), tone: '' },
          ]
        })
        .catch(() => (resourceTiles = []))
      // Background jobs: in-progress by kind + recent errors — the "is anything stuck?"
      // view. Isolated so a query failure only empties this panel.
      const jobs = Promise.all([
        ...JOB_KINDS.map(([k]) =>
          api.metricsQuery(`faker_jobs_in_progress{kind="${k}",job="faker-api"}`),
        ),
        api.metricsQuery('sum(increase(faker_jobs_total{result="error",job="faker-api"}[1h]))'),
      ])
        .then((rs) => {
          const inprog = JOB_KINDS.map(([, label], i) => ({ label, n: promScalar(rs[i]) ?? 0 }))
          const active = inprog.reduce((a, b) => a + b.n, 0)
          const errs = promScalar(rs[JOB_KINDS.length]) ?? 0
          jobTiles = [
            { label: 'Active jobs', value: fmtInt(active), tone: active > 0 ? 'warn' : 'ok' },
            ...inprog.map((x) => ({
              label: `${x.label} running`,
              value: fmtInt(x.n),
              tone: (x.n > 0 ? 'warn' : '') as Tone,
            })),
            { label: 'Job errors (1h)', value: fmtInt(errs), tone: errs > 0 ? 'bad' : 'ok' },
          ]
        })
        .catch(() => (jobTiles = []))
      await Promise.all([
        resources,
        jobs,
        chart('faker_daemon_snapshots', 7 * 24 * 60, 3600, (s) => (snapshots = s), 'ticker'),
        chart('sum(rate(faker_http_requests_total[5m]))', 60, 60, (s) => (reqRate = s), undefined, 'requests'),
        chart('histogram_quantile(0.95, sum(rate(faker_http_request_duration_seconds_bucket[5m])) by (le))', 60, 60, (s) => (latency = s), undefined, 'p95'),
        chart('faker_ws_connections', 60, 60, (s) => (wsConns = s), 'hub'),
        // Active alerts from the Prometheus rule set — isolated like the charts.
        // A fetch failure is "unknown", NOT "no alerts": don't paint a false
        // all-clear when the alerts endpoint alone is unavailable.
        api
          .metricsAlerts()
          .then((r) => {
            if (r.status === 'error') throw new Error(r.error || 'alerts unavailable')
            alerts = activeAlerts(r)
            alertsErr = false
          })
          .catch(() => (alertsErr = true)),
      ])
    } catch (e) {
      degraded = String(e)
    } finally {
      loading = false
    }
  }

  // onMount stays synchronous (returns the interval cleanup); load() runs async.
  onMount(() => {
    load()
    const t = setInterval(load, 15000)
    return () => clearInterval(t)
  })
</script>

<div class="wrap">
  <div class="legend">
    Metrics are queried from <b>Prometheus</b> server-side (the browser never talks to it) and
    rendered here — the same pattern as Logs. No Grafana required.
  </div>

  {#if loading && !tiles.length && !degraded}
    <div class="msg">Loading metrics…</div>
  {:else if degraded}
    <div class="degrade">
      {degraded}
      <div class="dim">
        The daemon and API export Prometheus metrics; bring up Prometheus (observability stack) to
        chart them here.
      </div>
    </div>
  {:else}
    <div class="section mono">ALERTS</div>
    {#if alertsErr}
      <div class="warn-banner">
        <span class="dot"></span> Alert state unavailable — the Prometheus alerts endpoint didn't respond.
      </div>
    {:else if alerts.length === 0}
      <div class="ok-banner">
        <span class="dot"></span> No active alerts.
      </div>
    {:else}
      <div class="alerts">
        {#each alerts as a (alertName(a) + JSON.stringify(a.labels))}
          <div class="alert {a.state === 'firing' ? alertSeverity(a) : 'pending'}">
            <div class="a-top">
              <span class="a-state mono">{a.state === 'firing' ? alertSeverity(a).toUpperCase() : 'PENDING'}</span>
              <span class="a-name mono">{alertName(a)}</span>
              {#if alertSince(a)}<span class="a-since">for {alertSince(a)}</span>{/if}
            </div>
            <div class="a-summary">{alertSummary(a)}</div>
          </div>
        {/each}
      </div>
    {/if}

    <div class="section mono">DAEMON</div>
    <div class="tiles">
      {#each tiles as t (t.label)}
        <div class="tile">
          <div class="tval {t.tone}">{t.value}</div>
          <div class="tlabel">{t.label}</div>
        </div>
      {/each}
    </div>

    {#if jobTiles.length}
      <div class="section mono">JOBS</div>
      <div class="tiles">
        {#each jobTiles as t (t.label)}
          <div class="tile">
            <div class="tval {t.tone}">{t.value}</div>
            <div class="tlabel">{t.label}</div>
          </div>
        {/each}
      </div>
    {/if}

    {#if resourceTiles.length}
      <div class="section mono">RESOURCES</div>
      <div class="tiles">
        {#each resourceTiles as t (t.label)}
          <div class="tile">
            <div class="tval {t.tone}">{t.value}</div>
            <div class="tlabel">{t.label}</div>
          </div>
        {/each}
      </div>
    {/if}

    <div class="section mono">COVERAGE</div>
    <MetricChart title="Snapshots per ticker" hint="7 days" series={snapshots} height={170} />

    <div class="section mono">API &amp; STREAMS</div>
    <div class="grid2">
      <MetricChart title="Request rate" hint="last hour" series={reqRate} format={fmtPerSec} />
      <MetricChart title="p95 latency" hint="last hour" series={latency} format={fmtMs} />
      <MetricChart title="WebSocket connections" hint="last hour" series={wsConns} format={fmtInt} />
    </div>
  {/if}
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
  .legend b {
    color: var(--text-1);
    font-weight: 600;
  }
  .msg,
  .degrade {
    font-size: 13px;
    color: var(--text-2);
    padding: 16px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--panel, #111214);
  }
  .degrade .dim {
    margin-top: 6px;
    font-size: 12px;
    color: var(--muted-2);
  }
  .section {
    font-size: 10.5px;
    letter-spacing: 0.06em;
    color: var(--muted-2);
  }
  .tiles {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
    gap: 12px;
  }
  .grid2 {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
    gap: 12px;
  }
  .ok-banner {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 13px;
    color: var(--text-2);
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--panel, #111214);
    padding: 12px 14px;
  }
  .ok-banner .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--green);
  }
  .warn-banner {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 13px;
    color: var(--text-2);
    border: 1px solid var(--border);
    border-left: 3px solid var(--amber, #e0b164);
    border-radius: 8px;
    background: var(--panel, #111214);
    padding: 12px 14px;
  }
  .warn-banner .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--amber, #e0b164);
  }
  .alerts {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .alert {
    border: 1px solid var(--border);
    border-left-width: 3px;
    border-radius: 8px;
    background: var(--panel, #111214);
    padding: 10px 14px;
  }
  .alert.critical {
    border-left-color: var(--red, #e08080);
  }
  .alert.warning {
    border-left-color: var(--amber, #e0b164);
  }
  .alert.pending {
    border-left-color: var(--muted-2);
  }
  .a-top {
    display: flex;
    align-items: baseline;
    gap: 10px;
  }
  .a-state {
    font-size: 10px;
    letter-spacing: 0.04em;
  }
  .alert.critical .a-state {
    color: var(--red, #e08080);
  }
  .alert.warning .a-state {
    color: var(--amber, #e0b164);
  }
  .alert.pending .a-state {
    color: var(--muted-2);
  }
  .a-name {
    font-size: 12px;
    color: var(--text-1);
  }
  .a-since {
    font-size: 11px;
    color: var(--muted-2);
    margin-left: auto;
  }
  .a-summary {
    margin-top: 3px;
    font-size: 12px;
    color: var(--text-2);
  }
  .tile {
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--panel, #111214);
    padding: 14px 16px;
  }
  .tval {
    font-size: 22px;
    font-weight: 600;
    color: var(--text-1);
    font-variant-numeric: tabular-nums;
  }
  .tval.ok {
    color: var(--green);
  }
  .tval.warn {
    color: var(--amber, #e0b164);
  }
  .tval.bad {
    color: var(--red, #e08080);
  }
  .tlabel {
    margin-top: 4px;
    font-size: 12px;
    color: var(--text-2);
  }
</style>
