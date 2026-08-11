<script lang="ts">
  import { onMount } from 'svelte'
  import { api, fmtInt, type Status, type EndpointDoc, type KeyEntry } from '../lib/api'
  import { copyText } from '../lib/clipboard'

  let { status, baseUrl }: { status: Status | null; baseUrl: string } = $props()

  let endpoints = $state<EndpointDoc[]>([])
  let keys = $state<KeyEntry[]>([])
  let copiedPath = $state<string | null>(null)
  let copiedSnippet = $state(false)
  let resetMsg = $state('')

  const devKey = 'gexbot-studio'

  async function refreshKeys() {
    try {
      keys = await api.keys()
    } catch {
      keys = []
    }
  }

  async function loadEndpoints() {
    try {
      endpoints = await api.endpoints()
    } catch {
      endpoints = []
    }
  }

  // onMount stays synchronous so its returned cleanup actually runs (an async
  // onMount returns a Promise, not a destroy fn).
  onMount(() => {
    loadEndpoints()
    refreshKeys()
    const t = setInterval(refreshKeys, 4000)
    return () => clearInterval(t)
  })

  function curlFor(e: EndpointDoc): string {
    const path = e.path.replace('{ticker}', 'SPX').replace('{aggregation}', 'gex_zero').replace('{type}', 'gex_zero').replace('{date}', status?.loaded_date ?? '2026-08-07')
    if (e.method === 'POST') {
      return `curl -X POST ${baseUrl}${path}`
    }
    const auth = /^\/(SPX|\{ticker\}|options|futures|hist)/.test(e.path) || e.path.startsWith('/{ticker}')
    return auth
      ? `curl -H "Authorization: Bearer ${devKey}" ${baseUrl}${path}`
      : `curl ${baseUrl}${path}`
  }

  async function copyCurl(e: EndpointDoc) {
    if (await copyText(curlFor(e))) {
      copiedPath = e.path
      setTimeout(() => (copiedPath = null), 1400)
    }
  }

  const snippet = $derived(
    `export GEXBOT_API_KEY=${devKey}\nBASE_URL="${baseUrl}"\n\ncurl -H "Authorization: Bearer $GEXBOT_API_KEY" \\\n  $BASE_URL/SPX/state/gex_zero`,
  )
  async function copySnippet() {
    if (await copyText(snippet)) {
      copiedSnippet = true
      setTimeout(() => (copiedSnippet = false), 1400)
    }
  }

  async function resetAll() {
    try {
      const r = await api.resetCache()
      resetMsg = `reset ${r.count}`
      refreshKeys()
    } catch (e) {
      resetMsg = 'failed'
    }
    setTimeout(() => (resetMsg = ''), 1600)
  }

  function pct(index: number): string {
    // We don't know stream length here; show index only unless huge.
    return fmtInt(index)
  }
</script>

<div class="wrap">
  <div class="card serving">
    <div class="serving-status" style="border-color:{status?.running ? 'var(--green-border)' : 'var(--border-2)'};background:{status?.running ? 'var(--green-bg)' : 'var(--input)'}">
      <span class="dot" style="background:{status?.running ? 'var(--green)' : 'var(--muted-2)'}"></span>
      <span class="statustext" style="color:{status?.running ? 'var(--green)' : 'var(--muted-2)'}">
        {status?.running ? 'Running' : 'Stopped'}
      </span>
    </div>
    <div class="serving-info">
      <div class="lbl">Serving</div>
      <div class="mono val">
        {status?.loaded_date ?? '—'} · {status?.cache_mode ?? ''} mode · {status?.data_mode ?? ''}
        {#if status?.is_reloading}<span class="reloading">· reloading…</span>{/if}
      </div>
    </div>
    <div class="flex-1"></div>
    <a class="btn" href={`${baseUrl}/docs`} target="_blank" rel="noreferrer">Open API docs</a>
    <a class="btn" href={`${baseUrl}/asyncapi`} target="_blank" rel="noreferrer">WebSocket docs</a>
  </div>

  <div class="grid">
    <div class="card">
      <div class="card-head">Endpoints</div>
      {#each endpoints as e (e.method + e.path)}
        <div class="ep">
          <span
            class="method mono"
            style="background:{e.method === 'GET' ? '#16241c' : '#1c2130'};color:{e.method === 'GET' ? 'var(--green)' : 'var(--blue)'}"
            >{e.method}</span
          >
          <div class="ep-mid">
            <div class="ep-path mono">{e.path}</div>
            <div class="ep-desc">{e.desc}</div>
          </div>
          <button class="curl mono" onclick={() => copyCurl(e)}
            >{copiedPath === e.path ? 'copied' : 'cURL'}</button
          >
        </div>
      {/each}
    </div>

    <div class="col">
      <div class="card pad">
        <div class="head-row">
          <div class="ttl">Client keys</div>
          <button class="btn small" onclick={resetAll}>{resetMsg || 'Reset all'}</button>
        </div>
        {#if keys.length === 0}
          <div class="empty">No clients have requested data yet.</div>
        {:else}
          {#each keys as k (k.key)}
            <div class="key">
              <span class="dot" style="background:var(--green)"></span>
              <div class="key-mid">
                <div class="key-name mono">{k.key}</div>
                <div class="key-pos">{k.streams.length} stream{k.streams.length === 1 ? '' : 's'} · lead {k.streams[0]?.data_key ?? ''}</div>
              </div>
              <div class="key-idx mono">{pct(k.streams[0]?.index ?? 0)}</div>
            </div>
          {/each}
        {/if}
      </div>

      <div class="card pad">
        <div class="ttl mb">Point a client here</div>
        <pre class="snippet mono">{snippet}</pre>
        <button class="btn full" onclick={copySnippet}
          >{copiedSnippet ? 'copied to clipboard' : 'Copy snippet'}</button
        >
      </div>
    </div>
  </div>
</div>

<style>
  .wrap {
    padding: 18px 20px 28px;
    display: flex;
    flex-direction: column;
    gap: 14px;
  }
  .flex-1 {
    flex: 1;
  }
  .serving {
    padding: 14px;
    display: flex;
    align-items: center;
    gap: 14px;
  }
  .serving-status {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 7px 12px;
    border-radius: 7px;
    border: 1px solid var(--border-2);
  }
  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
  }
  .statustext {
    font-size: 12px;
    font-weight: 600;
  }
  .serving-info .lbl {
    font-size: 11px;
    color: var(--muted-2);
  }
  .serving-info .val {
    font-size: 11.5px;
    margin-top: 2px;
  }
  .reloading {
    color: var(--amber);
  }
  .grid {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 300px;
    gap: 14px;
    align-items: start;
  }
  .col {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }
  .pad {
    padding: 14px;
  }
  .ep {
    display: grid;
    grid-template-columns: 52px minmax(0, 1fr) 56px;
    gap: 12px;
    align-items: center;
    padding: 9px 14px;
    border-bottom: 1px solid #191c20;
  }
  .method {
    font-size: 9.5px;
    padding: 2px 5px;
    border-radius: 4px;
    text-align: center;
  }
  .ep-mid {
    min-width: 0;
  }
  .ep-path {
    font-size: 11.5px;
    color: var(--text-2);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .ep-desc {
    font-size: 10.5px;
    color: var(--muted-2);
    margin-top: 2px;
  }
  .curl {
    font-family: var(--mono);
    font-size: 10.5px;
    padding: 3px 8px;
    border: 1px solid var(--border-2);
    border-radius: 5px;
    color: var(--muted-2);
    cursor: pointer;
    background: transparent;
  }
  .curl:hover {
    color: var(--text);
    border-color: #3a3e46;
  }
  .head-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 11px;
  }
  .ttl {
    font-size: 12.5px;
    font-weight: 600;
  }
  .mb {
    margin-bottom: 9px;
  }
  .btn.small {
    padding: 4px 9px;
  }
  .btn.full {
    width: 100%;
    text-align: center;
    margin-top: 9px;
  }
  .empty {
    font-size: 11px;
    color: var(--muted-2);
    padding: 6px 0;
  }
  .key {
    display: flex;
    align-items: center;
    gap: 9px;
    padding: 8px 0;
    border-bottom: 1px solid #191c20;
  }
  .key-mid {
    flex: 1;
    min-width: 0;
  }
  .key-name {
    font-size: 11px;
    color: var(--text-2);
  }
  .key-pos {
    font-size: 10px;
    color: var(--muted-2);
    margin-top: 2px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .key-idx {
    font-size: 10px;
    color: var(--dim);
  }
  .snippet {
    font-size: 10.5px;
    color: var(--muted);
    line-height: 1.7;
    background: var(--input);
    border: 1px solid var(--border-3);
    border-radius: 7px;
    padding: 10px;
    white-space: pre-wrap;
    margin: 0;
  }
</style>
