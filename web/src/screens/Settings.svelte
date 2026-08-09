<script lang="ts">
  import { onMount } from 'svelte'
  import { api, type SettingGroup } from '../lib/api'

  let groups = $state<SettingGroup[]>([])
  let loading = $state(true)

  onMount(async () => {
    try {
      groups = await api.config()
    } catch {
      groups = []
    } finally {
      loading = false
    }
  })
</script>

<div class="wrap">
  <div class="note">Read-only. These are the server's current effective settings; change them via
    environment variables and restart.</div>

  {#if loading}
    <div class="msg">Loading settings…</div>
  {/if}

  {#each groups as g (g.title)}
    <div class="card">
      <div class="ghead">
        <div class="gtitle">{g.title}</div>
        <div class="gsub">{g.sub}</div>
      </div>
      {#each g.rows as r (r.env)}
        <div class="row">
          <div class="left">
            <div class="label">{r.label}</div>
            {#if r.help}<div class="help">{r.help}</div>{/if}
            <div class="env mono">{r.env}</div>
          </div>
          <div class="value mono">{r.value || '—'}</div>
        </div>
      {/each}
    </div>
  {/each}
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
