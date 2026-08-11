<script lang="ts">
  import { onMount } from 'svelte'
  import { api, fmtBytes, fmtLoadedSpan, type Status } from './lib/api'
  import { copyText } from './lib/clipboard'
  import Server from './screens/Server.svelte'
  import Library from './screens/Library.svelte'
  import Streams from './screens/Streams.svelte'
  import Settings from './screens/Settings.svelte'
  import Logs from './screens/Logs.svelte'
  import Monitoring from './screens/Monitoring.svelte'
  import Download from './screens/Download.svelte'

  type ScreenId = 'download' | 'library' | 'server' | 'streams' | 'logs' | 'monitoring' | 'settings'

  const nav: { id: ScreenId; label: string; glyph: string }[] = [
    { id: 'download', label: 'Download data', glyph: '↓' },
    { id: 'library', label: 'Data library', glyph: '▤' },
    { id: 'server', label: 'Local server', glyph: '◉' },
    { id: 'streams', label: 'Live streams', glyph: '≈' },
    { id: 'logs', label: 'Logs', glyph: '≡' },
    { id: 'monitoring', label: 'Monitoring', glyph: '▦' },
    { id: 'settings', label: 'Settings', glyph: '⚙' },
  ]
  const titles: Record<ScreenId, [string, string]> = {
    download: ['Download data', 'Fetch historical market days from GexBot'],
    library: ['Data library', 'EOD archives on this machine'],
    server: ['Local server', 'What your clients are talking to'],
    streams: ['Live streams', 'Five WebSocket hubs'],
    logs: ['Logs', 'Server, downloader and daemon'],
    monitoring: ['Monitoring', 'Metrics from Prometheus'],
    settings: ['Settings', 'Every environment variable, in plain language'],
  }

  // Screen from the URL hash so deep-links / reloads work: /studio/#/library
  function screenFromHash(): ScreenId {
    const h = location.hash.replace(/^#\/?/, '') as ScreenId
    return nav.some((n) => n.id === h) ? h : 'server'
  }
  let screen = $state<ScreenId>(screenFromHash())
  function go(id: ScreenId) {
    screen = id
    location.hash = `#/${id}`
  }

  let status = $state<Status | null>(null)
  let copied = $state(false)

  async function refreshStatus() {
    try {
      status = await api.status()
    } catch {
      status = null
    }
  }

  // The origin the browser used to reach Studio *is* the API base — this keeps
  // copied URLs, docs links and cURL commands pointing at the real server
  // (preserving host, scheme, and any reverse-proxy port) rather than localhost.
  const baseUrl = window.location.origin

  async function copyBase() {
    if (await copyText(baseUrl)) {
      copied = true
      setTimeout(() => (copied = false), 1400)
    }
  }

  onMount(() => {
    refreshStatus()
    const t = setInterval(refreshStatus, 5000)
    const onHash = () => (screen = screenFromHash())
    addEventListener('hashchange', onHash)
    return () => {
      clearInterval(t)
      removeEventListener('hashchange', onHash)
    }
  })
</script>

<div class="app">
  <aside class="sidebar">
    <div class="brand">
      <div class="brand-dot"></div>
      <div class="brand-name">GEX Faker Studio</div>
    </div>

    <nav class="nav">
      {#each nav as item (item.id)}
        <button class="nav-item" class:active={screen === item.id} onclick={() => go(item.id)}>
          <span class="nav-glyph mono">{item.glyph}</span>
          <span class="nav-label">{item.label}</span>
        </button>
      {/each}
    </nav>

    <div class="flex-1"></div>

    <div class="status-box">
      <div class="status-line">
        <span
          class="status-pulse"
          style="background:{status?.running ? 'var(--green)' : 'var(--muted-2)'}"
        ></span>
        <span
          class="status-label"
          style="color:{status?.running ? 'var(--green)' : 'var(--muted-2)'}"
        >
          {status ? (status.running ? 'Running' : 'Stopped') : 'Connecting…'}
        </span>
      </div>
      <div class="status-kv">
        <span>replaying</span><span class="mono val">{fmtLoadedSpan(status)}</span>
      </div>
      <div class="status-kv">
        <span>on disk</span><span class="mono val">{status ? fmtBytes(status.disk_bytes) : '—'}</span
        >
      </div>
    </div>
    <div class="datapath mono">~/gexbot-faker/data</div>
  </aside>

  <main class="main">
    <header class="topbar">
      <div class="topbar-title">{titles[screen][0]}</div>
      <div class="topbar-sub">{titles[screen][1]}</div>
      <div class="flex-1"></div>
      <div class="reach">
        <span class="reach-label">Reachable at</span>
        <span class="reach-url mono">{baseUrl || '—'}</span>
        <button class="btn" onclick={copyBase}>{copied ? 'copied' : 'copy'}</button>
      </div>
    </header>

    <div class="screen">
      {#if screen === 'server'}
        <Server {status} {baseUrl} />
      {:else if screen === 'library'}
        <Library {status} onchanged={refreshStatus} />
      {:else if screen === 'streams'}
        <Streams {status} />
      {:else if screen === 'settings'}
        <Settings />
      {:else if screen === 'download'}
        <Download onchanged={refreshStatus} />
      {:else if screen === 'logs'}
        <Logs />
      {:else if screen === 'monitoring'}
        <Monitoring />
      {/if}
    </div>
  </main>
</div>

<style>
  .app {
    display: flex;
    height: 100vh;
    overflow: hidden;
  }
  .sidebar {
    width: 236px;
    flex: none;
    display: flex;
    flex-direction: column;
    background: var(--panel);
    border-right: 1px solid var(--border);
  }
  .brand {
    height: 46px;
    display: flex;
    align-items: center;
    gap: 9px;
    padding: 0 14px;
    border-bottom: 1px solid var(--border);
  }
  .brand-dot {
    width: 9px;
    height: 9px;
    border-radius: 2px;
    background: var(--green);
  }
  .brand-name {
    font-size: 12.5px;
    font-weight: 600;
    letter-spacing: 0.02em;
  }
  .nav {
    padding: 10px 8px;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .nav-item {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 7px 10px;
    border-radius: 6px;
    cursor: pointer;
    background: transparent;
    border: none;
    color: #a2a7b0;
    text-align: left;
    width: 100%;
  }
  .nav-item:hover {
    background: #1c1f24;
  }
  .nav-item.active {
    background: #1f242a;
    color: #fff;
  }
  .nav-glyph {
    width: 18px;
    text-align: center;
    font-size: 12px;
    opacity: 0.85;
  }
  .nav-label {
    flex: 1;
    font-size: 12.5px;
    font-weight: 500;
  }
  .flex-1 {
    flex: 1;
  }
  .status-box {
    margin: 8px;
    padding: 10px 11px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--panel-2);
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .status-line {
    display: flex;
    align-items: center;
    gap: 7px;
  }
  .status-pulse {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    animation: pulseDot 2s ease-in-out infinite;
  }
  .status-label {
    font-size: 11.5px;
    font-weight: 600;
  }
  .status-kv {
    display: flex;
    justify-content: space-between;
    font-family: var(--mono);
    font-size: 10.5px;
    color: var(--muted);
  }
  .status-kv .val {
    color: var(--text-2);
  }
  .datapath {
    padding: 0 14px 12px;
    font-size: 10px;
    color: var(--dimmer);
  }
  .main {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }
  .topbar {
    height: 46px;
    flex: none;
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 0 16px;
    border-bottom: 1px solid var(--border);
    background: var(--panel);
  }
  .topbar-title {
    font-size: 13px;
    font-weight: 600;
  }
  .topbar-sub {
    font-size: 11.5px;
    color: var(--muted-2);
  }
  .reach {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .reach-label {
    font-size: 11px;
    color: var(--muted-2);
  }
  .reach-url {
    font-size: 11.5px;
    background: var(--input);
    border: 1px solid var(--border-2);
    border-radius: 6px;
    padding: 4px 9px;
    color: var(--text-2);
  }
  .screen {
    flex: 1;
    min-height: 0;
    overflow: auto;
  }
</style>
