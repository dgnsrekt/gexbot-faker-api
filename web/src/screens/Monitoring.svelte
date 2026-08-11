<script lang="ts">
  import { onMount } from 'svelte'
  import { api, promScalar, promToSeries, type Series } from '../lib/api'
  import MetricChart from '../lib/MetricChart.svelte'

  type Tone = 'ok' | 'warn' | 'bad' | ''
  type Tile = { label: string; value: string; tone: Tone }

  let tiles = $state<Tile[]>([])
  let degraded = $state('') // non-empty → Prometheus unset/unreachable
  let loading = $state(true)

  // Chart series (range queries).
  let snapshots = $state<Series[]>([])
  let reqRate = $state<Series[]>([])
  let latency = $state<Series[]>([])
  let wsConns = $state<Series[]>([])

  const fmtPerSec = (v: number) => v.toFixed(2) + '/s'
  const fmtMs = (v: number) => Math.round(v * 1000) + 'ms'
  const fmtInt = (v: number) => String(Math.round(v))

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

  async function load() {
    try {
      const [lastAge, nextIn, ok, fail] = await Promise.all([
        api.metricsQuery('time() - faker_daemon_last_success_timestamp_seconds{job="faker-daemon"}'),
        api.metricsQuery('faker_daemon_next_run_timestamp_seconds{job="faker-daemon"} - time()'),
        api.metricsQuery('sum(faker_daemon_download_runs_total{result="success"})'),
        api.metricsQuery('sum(faker_daemon_download_runs_total{result!="success"})'),
      ])
      const bad = [lastAge, nextIn, ok, fail].find((r) => r.status === 'error')
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
      tiles = [
        // Shown informationally (no health tone): a wall-clock threshold would
        // false-alarm over weekends/holidays when no market-day download is due.
        // A market-schedule-aware "download overdue" signal belongs server-side
        // (the daemon knows its schedule + the market calendar) — a later slice.
        { label: 'Last successful download', value: age === null ? 'no data yet' : fmtAge(age), tone: '' },
        { label: 'Next scheduled run', value: nxt === null ? '—' : fmtIn(nxt), tone: '' },
        { label: 'Successful runs', value: oks === null ? '—' : String(oks), tone: '' },
        {
          label: 'Failed runs',
          value: fails === null ? '0' : String(fails),
          tone: fails && fails > 0 ? 'bad' : 'ok',
        },
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
      await Promise.all([
        chart('faker_daemon_snapshots', 7 * 24 * 60, 3600, (s) => (snapshots = s), 'ticker'),
        chart('sum(rate(faker_http_requests_total[5m]))', 60, 60, (s) => (reqRate = s), undefined, 'requests'),
        chart('histogram_quantile(0.95, sum(rate(faker_http_request_duration_seconds_bucket[5m])) by (le))', 60, 60, (s) => (latency = s), undefined, 'p95'),
        chart('faker_ws_connections', 60, 60, (s) => (wsConns = s), 'hub'),
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
    <div class="section mono">DAEMON</div>
    <div class="tiles">
      {#each tiles as t (t.label)}
        <div class="tile">
          <div class="tval {t.tone}">{t.value}</div>
          <div class="tlabel">{t.label}</div>
        </div>
      {/each}
    </div>

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
