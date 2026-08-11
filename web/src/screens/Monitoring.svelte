<script lang="ts">
  import { onMount } from 'svelte'
  import { api, promScalar } from '../lib/api'

  type Tone = 'ok' | 'warn' | 'bad' | ''
  type Tile = { label: string; value: string; tone: Tone }

  let tiles = $state<Tile[]>([])
  let degraded = $state('') // non-empty → Prometheus unset/unreachable
  let loading = $state(true)

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
        {
          label: 'Last successful download',
          value: age === null ? 'no data yet' : fmtAge(age),
          tone: age !== null && age > 26 * 3600 ? 'warn' : 'ok',
        },
        { label: 'Next scheduled run', value: nxt === null ? '—' : fmtIn(nxt), tone: '' },
        { label: 'Successful runs', value: oks === null ? '—' : String(oks), tone: '' },
        {
          label: 'Failed runs',
          value: fails === null ? '0' : String(fails),
          tone: fails && fails > 0 ? 'bad' : 'ok',
        },
      ]
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
