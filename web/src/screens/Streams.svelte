<script lang="ts">
  import { onMount } from 'svelte'
  import { api, type Status, type HubStat } from '../lib/api'
  import { copyText } from '../lib/clipboard'

  let { status }: { status: Status | null } = $props()

  let hubs = $state<HubStat[]>([])
  let tickers = $state<string[]>(['SPX', 'NDX', 'SPY'])
  let pickTicker = $state('SPX')
  let pickFeed = $state(0)
  let copied = $state(false)

  const hubDesc: Record<string, string> = {
    orderflow: 'DEX, GEX, convexity, vanna, charm',
    classic: 'Classic GEX chain',
    state_gex: 'State GEX profiles',
    state_greeks_zero: 'Delta, gamma, vanna, charm — 0DTE',
    state_greeks_one: 'Delta, gamma, vanna, charm — 1DTE+',
  }

  // Group-name suffix segments: blue_{ticker}_{hub}_{cat}, matching the per-hub
  // validators (classic → classic_gex_*, state gex → state_gex_*, greeks →
  // state_{greek}_{zero|one}, orderflow → orderflow_orderflow).
  const feeds = [
    { label: 'classic · gex_zero', hub: 'classic', cat: 'gex_zero' },
    { label: 'classic · gex_full', hub: 'classic', cat: 'gex_full' },
    { label: 'classic · gex_one', hub: 'classic', cat: 'gex_one' },
    { label: 'state · gex_zero', hub: 'state', cat: 'gex_zero' },
    { label: 'state · gamma_zero', hub: 'state', cat: 'gamma_zero' },
    { label: 'state · delta_zero', hub: 'state', cat: 'delta_zero' },
    { label: 'state · vanna_one', hub: 'state', cat: 'vanna_one' },
    { label: 'state · charm_one', hub: 'state', cat: 'charm_one' },
    { label: 'orderflow', hub: 'orderflow', cat: 'orderflow' },
  ]

  async function refresh() {
    try {
      hubs = await api.hubs()
    } catch {
      hubs = []
    }
  }

  async function loadTickers() {
    try {
      const t = await api.tickers()
      const all = [...t.indexes, ...t.stocks, ...t.futures]
      if (all.length) tickers = all
    } catch {
      /* keep defaults */
    }
  }

  // Keep onMount synchronous so the returned cleanup runs (see Server.svelte).
  onMount(() => {
    loadTickers()
    refresh()
    const iv = setInterval(refresh, 3000)
    return () => clearInterval(iv)
  })

  const prefix = $derived(status?.group_prefix ?? 'blue')
  const groupName = $derived(
    `${prefix}_${pickTicker}_${feeds[pickFeed].hub}_${feeds[pickFeed].cat}`,
  )

  async function copyGroup() {
    if (await copyText(groupName)) {
      copied = true
      setTimeout(() => (copied = false), 1400)
    }
  }

  const wsFacts = $derived([
    { k: 'Negotiate', v: '/negotiate' },
    { k: 'Subprotocol', v: 'protobuf.webpubsub.azure.v1' },
    { k: 'Encoding', v: 'protobuf + zstd' },
    { k: 'Broadcast interval', v: hubs[0]?.interval ?? status?.data_mode ?? '1s' },
    { k: 'Group prefix', v: prefix },
  ])
</script>

<div class="wrap">
  <div class="hubs">
    {#each hubs as h (h.name)}
      <div class="card hub">
        <div class="hub-top">
          <span
            class="dot"
            style="background:{h.clients > 0 ? 'var(--green)' : '#3a3e46'}"
          ></span>
          <div class="hub-mid">
            <div class="mono name">{h.name}</div>
            <div class="desc">{hubDesc[h.name] ?? ''}</div>
          </div>
          <div class="mono clients">{h.clients} client{h.clients === 1 ? '' : 's'}</div>
          <div class="mono rate" style="color:{h.clients > 0 ? 'var(--green)' : 'var(--dim)'}">
            {h.clients > 0 ? h.interval : 'idle'}
          </div>
        </div>
        {#if h.active_groups.length}
          <div class="groups">
            {#each h.active_groups as g (g)}<span class="pill">{g}</span>{/each}
          </div>
        {/if}
      </div>
    {/each}
    {#if hubs.length === 0}
      <div class="card empty">WebSocket hubs are disabled or unreachable.</div>
    {/if}
  </div>

  <div class="side">
    <div class="card pad">
      <div class="ttl">Group name builder</div>
      <div class="sub">Pick a ticker and a feed; copy the group name your client subscribes to.</div>

      <div class="lbl">Ticker</div>
      <div class="chips">
        {#each tickers as t (t)}
          <button
            class="chip mono"
            class:on={pickTicker === t}
            onclick={() => (pickTicker = t)}>{t}</button
          >
        {/each}
      </div>

      <div class="lbl">Feed</div>
      <div class="chips">
        {#each feeds as f, i (f.label)}
          <button class="chip mono" class:on={pickFeed === i} onclick={() => (pickFeed = i)}
            >{f.label}</button
          >
        {/each}
      </div>

      <div class="group mono">{groupName}</div>
      <button class="btn full" onclick={copyGroup}>{copied ? 'copied to clipboard' : 'Copy group name'}</button>
    </div>

    <div class="card pad">
      <div class="ttl mb">Connection</div>
      {#each wsFacts as f (f.k)}
        <div class="fact">
          <span class="fk">{f.k}</span><span class="fv mono">{f.v}</span>
        </div>
      {/each}
    </div>
  </div>
</div>

<style>
  .wrap {
    padding: 18px 20px 28px;
    display: grid;
    grid-template-columns: minmax(0, 1fr) 340px;
    gap: 16px;
    align-items: start;
  }
  .hubs {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .hub {
    padding: 13px 14px;
  }
  .hub-top {
    display: flex;
    align-items: flex-start;
    gap: 10px;
  }
  .dot {
    width: 7px;
    height: 7px;
    flex: none;
    margin-top: 5px;
    border-radius: 50%;
  }
  .hub-mid {
    flex: 1;
    min-width: 0;
  }
  .name {
    font-size: 12px;
  }
  .desc {
    font-size: 11px;
    color: var(--muted-2);
    margin-top: 3px;
  }
  .clients {
    font-size: 10.5px;
    color: var(--dim);
    flex: none;
  }
  .rate {
    font-size: 10.5px;
    flex: none;
  }
  .groups {
    display: flex;
    gap: 5px;
    flex-wrap: wrap;
    margin-top: 10px;
  }
  .empty {
    padding: 16px;
    color: var(--muted-2);
    font-size: 12px;
  }
  .side {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }
  .pad {
    padding: 14px;
  }
  .ttl {
    font-size: 12.5px;
    font-weight: 600;
  }
  .mb {
    margin-bottom: 10px;
  }
  .sub {
    font-size: 11.5px;
    color: var(--muted);
    line-height: 1.5;
    margin: 4px 0 12px;
  }
  .lbl {
    font-size: 11px;
    color: var(--muted-2);
    margin-bottom: 6px;
  }
  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin-bottom: 12px;
  }
  .chip {
    padding: 4px 9px;
    border-radius: 6px;
    font-size: 11px;
    cursor: pointer;
    border: 1px solid var(--border-2);
    background: var(--input);
    color: var(--muted);
  }
  .chip.on {
    border-color: var(--green);
    background: var(--green-bg);
    color: var(--green);
  }
  .group {
    font-size: 11.5px;
    background: var(--input);
    border: 1px solid var(--border-3);
    border-radius: 7px;
    padding: 10px;
    color: var(--green-2);
    word-break: break-all;
  }
  .btn.full {
    width: 100%;
    text-align: center;
    margin-top: 9px;
  }
  .fact {
    display: flex;
    justify-content: space-between;
    gap: 12px;
    padding: 5px 0;
    font-size: 11.5px;
  }
  .fk {
    color: var(--muted);
  }
  .fv {
    color: var(--text-2);
    text-align: right;
  }
</style>
