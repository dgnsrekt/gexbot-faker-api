<script lang="ts">
  import { onMount } from 'svelte'
  import { api, type SettingGroup, type DaemonStatus } from '../lib/api'

  let groups = $state<SettingGroup[]>([])
  let loading = $state(true)
  // Daemon settings come from a separate proxied endpoint; it can be unavailable
  // (daemon down / DAEMON_URL unset) without breaking the API settings above.
  let daemon = $state<DaemonStatus | null>(null)
  let daemonErr = $state('')

  onMount(async () => {
    // API config and daemon status are independent; load them in parallel so one
    // being unavailable never blanks the other.
    const [cfg, dmn] = await Promise.allSettled([api.config(), api.daemon()])
    groups = cfg.status === 'fulfilled' ? cfg.value : []
    if (dmn.status === 'fulfilled') daemon = dmn.value
    else daemonErr = 'Daemon unavailable — the gex-daemon container is down or DAEMON_URL is unset.'
    loading = false
  })

  const onOff = (b: boolean) => (b ? 'on' : 'off')

  // daemonGroups renders the sanitized daemon status as labeled setting groups,
  // clearly separate from the API-process settings above.
  const daemonGroups = $derived.by((): SettingGroup[] => {
    const d = daemon
    if (!d) return []
    return [
      {
        title: 'Daemon · Schedule',
        sub: 'When the scheduled EOD download runs',
        rows: [
          { label: 'Time', help: 'Daily first-attempt time (ET by default).', env: 'DAEMON_SCHEDULE_HOUR/MINUTE', value: `${String(d.schedule.hour).padStart(2, '0')}:${String(d.schedule.minute).padStart(2, '0')}` },
          { label: 'Timezone', help: '', env: 'DAEMON_TIMEZONE', value: d.schedule.timezone },
          { label: 'Run on startup', help: 'Check for missed days when the daemon starts.', env: 'DAEMON_RUN_ON_STARTUP', value: onOff(d.schedule.run_on_startup) },
          { label: 'Per-run timeout', help: '', env: 'DAEMON_RUN_TIMEOUT_MINUTES', value: `${d.schedule.run_timeout_minutes} min` },
        ],
      },
      {
        title: 'Daemon · Downloads',
        sub: 'Effective coverage + last run',
        rows: [
          { label: 'Config path', help: 'The downloader YAML that governs coverage.', env: 'DAEMON_CONFIG_PATH', value: d.config_path },
          { label: 'Output directory', help: '', env: 'output.directory', value: d.output_dir },
          { label: 'Tickers', help: `${d.tickers.length} configured.`, env: 'tickers', value: d.tickers.join(' ') },
          { label: 'Last downloaded date', help: '', env: '', value: d.last_downloaded_date || '—' },
          { label: 'In progress', help: '', env: '', value: onOff(d.in_progress) },
          { label: 'Last result', help: d.last_error ? `Error: ${d.last_error}` : '', env: '', value: d.last_result || '—' },
          { label: 'Ready', help: 'Init complete and scheduling.', env: '', value: onOff(d.ready) },
        ],
      },
      {
        title: 'Daemon · Packages',
        sub: 'Enabled packages and their categories',
        rows: d.packages.map((p) => ({ label: p.name, help: `${p.categories.length} categories`, env: '', value: p.categories.join(' · ') })),
      },
      {
        title: 'Daemon · Cleanup',
        sub: 'TTL eviction of materialized JSONL (archives untouched)',
        rows: [
          { label: 'Enabled', help: '', env: 'GEXBOT_OUTPUT_AUTO_CLEANUP', value: onOff(d.cleanup.enabled) },
          { label: 'Retention', help: 'Days of inactivity before eviction.', env: 'GEXBOT_OUTPUT_CLEANUP_AFTER_DAYS', value: `${d.cleanup.retention_days} days` },
        ],
      },
      {
        title: 'Daemon · Notifications',
        sub: 'ntfy push (token never shown)',
        rows: [
          { label: 'Enabled', help: '', env: 'NTFY_ENABLED', value: onOff(d.notifications.enabled) },
          { label: 'Server', help: '', env: 'NTFY_SERVER', value: d.notifications.server },
          { label: 'Topic', help: '', env: 'NTFY_TOPIC', value: d.notifications.topic || '—' },
          { label: 'Priority', help: '', env: 'NTFY_PRIORITY', value: d.notifications.priority },
          { label: 'Tags', help: '', env: 'NTFY_TAGS', value: d.notifications.tags },
        ],
      },
    ]
  })
</script>

{#snippet groupCard(g: SettingGroup)}
  <div class="card">
    <div class="ghead">
      <div class="gtitle">{g.title}</div>
      <div class="gsub">{g.sub}</div>
    </div>
    {#each g.rows as r, i (r.label + i)}
      <div class="row">
        <div class="left">
          <div class="label">{r.label}</div>
          {#if r.help}<div class="help">{r.help}</div>{/if}
          {#if r.env}<div class="env mono">{r.env}</div>{/if}
        </div>
        <div class="value mono">{r.value || '—'}</div>
      </div>
    {/each}
  </div>
{/snippet}

<div class="wrap">
  <div class="note">Read-only. These are the server's current effective settings; change them via
    environment variables and restart. The <b>Daemon</b> sections come from the separate daemon
    process — change them in its downloader YAML / env.</div>

  {#if loading}
    <div class="msg">Loading settings…</div>
  {/if}

  <div class="section-head">API process</div>
  {#each groups as g (g.title)}
    {@render groupCard(g)}
  {/each}

  <div class="section-head">Daemon</div>
  {#if daemonErr}
    <div class="notice">{daemonErr}</div>
  {:else if daemon}
    {#each daemonGroups as g (g.title)}
      {@render groupCard(g)}
    {/each}
  {/if}
</div>

<style>
  .wrap {
    padding: 18px 20px 32px;
    max-width: 860px;
    display: flex;
    flex-direction: column;
    gap: 14px;
  }
  .note {
    font-size: 11.5px;
    color: var(--muted-2);
    line-height: 1.5;
  }
  .msg {
    color: var(--muted-2);
    font-size: 12px;
  }
  .section-head {
    font-size: 11px;
    font-weight: 600;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--muted-2);
    margin: 6px 0 -4px;
  }
  .notice {
    border: 1px solid #4a3a20;
    background: #1a1710;
    color: var(--amber);
    border-radius: 8px;
    padding: 10px 14px;
    font-size: 11.5px;
  }
  .ghead {
    padding: 11px 14px;
    border-bottom: 1px solid var(--border);
  }
  .gtitle {
    font-size: 12.5px;
    font-weight: 600;
  }
  .gsub {
    font-size: 11px;
    color: var(--muted-2);
    margin-top: 3px;
  }
  .row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 220px;
    gap: 16px;
    align-items: center;
    padding: 11px 14px;
    border-bottom: 1px solid #191c20;
  }
  .label {
    font-size: 12px;
    color: var(--text-2);
  }
  .help {
    font-size: 10.5px;
    color: var(--muted-2);
    margin-top: 3px;
    line-height: 1.5;
  }
  .env {
    font-size: 9.5px;
    color: var(--dimmer);
    margin-top: 4px;
  }
  .value {
    font-size: 11.5px;
    border: 1px solid var(--border-2);
    background: var(--input);
    border-radius: 7px;
    padding: 7px 10px;
    color: var(--text-2);
    justify-self: stretch;
    text-align: right;
  }
</style>
