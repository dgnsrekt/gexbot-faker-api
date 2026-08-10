<script lang="ts">
  import { onMount } from 'svelte'

  interface LogLine {
    time: string
    level: string
    service: string
    msg: string
  }

  const LEVELS = ['all', 'info', 'warn', 'error', 'debug'] as const
  type Level = (typeof LEVELS)[number]

  let lines = $state<LogLine[]>([])
  let filter = $state<Level>('all')
  let errorMsg = $state('')
  let connected = $state(false)

  const MAX = 1000

  const shown = $derived(
    filter === 'all' ? lines : lines.filter((l) => l.level.toLowerCase() === filter),
  )

  function levelColor(level: string): string {
    switch (level.toLowerCase()) {
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

  onMount(() => {
    const es = new EventSource('/studio/api/logs')
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
        lines.push(d as LogLine)
        if (lines.length > MAX) lines = lines.slice(-MAX)
      } catch {
        /* ignore malformed frame */
      }
    }
    es.onerror = () => {
      connected = false
    }
    return () => es.close()
  })
</script>

<div class="wrap">
  <div class="toolbar">
    {#each LEVELS as l (l)}
      <button class="chip mono" class:on={filter === l} onclick={() => (filter = l)}>{l}</button>
    {/each}
    <div class="flex-1"></div>
    <span class="conn mono" style="color:{connected ? 'var(--green)' : 'var(--muted-2)'}">
      {connected ? 'live' : 'connecting…'}
    </span>
    <button class="btn" onclick={() => (lines = [])}>Clear</button>
  </div>

  {#if errorMsg}
    <div class="notice">
      {errorMsg}
      <div class="hint mono">Run <code>just observability-up</code> (or ensure the Loki container is running), then reopen this screen.</div>
    </div>
  {/if}

  <div class="viewer mono">
    {#if shown.length === 0}
      <div class="empty">{errorMsg ? 'No log feed.' : 'Waiting for logs…'}</div>
    {:else}
      {#each shown as l, i (i + l.time + l.msg)}
        <div class="line">
          <span class="t">{l.time}</span>
          <span class="lvl" style="color:{levelColor(l.level)}">{l.level.toUpperCase()}</span>
          <span class="svc">{l.service}</span>
          <span class="msg" style="color:{levelColor(l.level) === 'var(--muted)' ? 'var(--text-2)' : levelColor(l.level)}">{l.msg}</span>
        </div>
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
  }
  .chip {
    font-size: 11px;
    padding: 4px 10px;
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
  .flex-1 {
    flex: 1;
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
    padding: 12px 14px;
    overflow: auto;
    font-size: 11px;
    line-height: 1.85;
  }
  .empty {
    color: var(--muted-2);
  }
  .line {
    display: flex;
    gap: 10px;
    white-space: pre-wrap;
    word-break: break-word;
  }
  .t {
    color: #4e535b;
    flex: none;
  }
  .lvl {
    flex: none;
    width: 46px;
  }
  .svc {
    color: var(--dim);
    flex: none;
    width: 96px;
  }
  .msg {
    flex: 1;
    min-width: 0;
  }
</style>
