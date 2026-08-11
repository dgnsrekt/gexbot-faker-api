<script lang="ts">
  import type { Series } from './api'

  let {
    title,
    series = [],
    format = (v: number) => (Math.abs(v) >= 1000 ? (v / 1000).toFixed(1) + 'k' : String(Math.round(v * 100) / 100)),
    height = 150,
    hint = '',
  }: {
    title: string
    series?: Series[]
    format?: (v: number) => string
    height?: number
    hint?: string
  } = $props()

  const W = 600
  const H = 200
  const PADL = 46 // left padding for y labels
  const PADR = 8
  const PADT = 8
  const PADB = 18 // bottom padding for x labels

  // Distinct, theme-friendly series colors.
  const COLORS = ['#6fcf97', '#7aa2f7', '#e0b164', '#e08080', '#b48ead', '#7fd1c9', '#d19a66']

  const flat = $derived(series.flatMap((s) => s.points))
  const hasData = $derived(flat.length > 0)
  const tMin = $derived(hasData ? Math.min(...flat.map((p) => p[0])) : 0)
  const tMax = $derived(hasData ? Math.max(...flat.map((p) => p[0])) : 1)
  const vMinRaw = $derived(hasData ? Math.min(...flat.map((p) => p[1])) : 0)
  const vMaxRaw = $derived(hasData ? Math.max(...flat.map((p) => p[1])) : 1)
  // Pad the value range a touch; keep 0 as the floor when data is non-negative.
  const vMin = $derived(vMinRaw >= 0 ? 0 : vMinRaw - (vMaxRaw - vMinRaw) * 0.05)
  const vMax = $derived(vMaxRaw + (vMaxRaw - vMin) * 0.08 || 1)

  function x(t: number): number {
    const span = tMax - tMin || 1
    return PADL + ((t - tMin) / span) * (W - PADL - PADR)
  }
  function y(v: number): number {
    const span = vMax - vMin || 1
    return PADT + (1 - (v - vMin) / span) * (H - PADT - PADB)
  }
  function path(pts: [number, number][]): string {
    return pts.map((p, i) => `${i ? 'L' : 'M'}${x(p[0]).toFixed(1)},${y(p[1]).toFixed(1)}`).join(' ')
  }

  function fmtTime(t: number): string {
    const d = new Date(t * 1000)
    const p = (n: number) => String(n).padStart(2, '0')
    return tMax - tMin > 2 * 86400
      ? `${p(d.getMonth() + 1)}/${p(d.getDate())}`
      : `${p(d.getHours())}:${p(d.getMinutes())}`
  }
  const yTicks = $derived([vMax, (vMax + vMin) / 2, vMin])
  function lastVal(s: Series): number | null {
    return s.points.length ? s.points[s.points.length - 1][1] : null
  }
</script>

<div class="chart">
  <div class="head mono">
    {title}{#if hint}<span class="hint">{hint}</span>{/if}
  </div>
  {#if !hasData}
    <div class="empty">no data in range</div>
  {:else}
    <svg viewBox="0 0 {W} {H}" style="height:{height}px" preserveAspectRatio="none">
      {#each yTicks as t (t)}
        <line class="grid" x1={PADL} x2={W - PADR} y1={y(t)} y2={y(t)} />
        <text class="ylab" x={PADL - 6} y={y(t) + 3} text-anchor="end">{format(t)}</text>
      {/each}
      <text class="xlab" x={PADL} y={H - 5} text-anchor="start">{fmtTime(tMin)}</text>
      <text class="xlab" x={W - PADR} y={H - 5} text-anchor="end">{fmtTime(tMax)}</text>
      {#each series as s, i (s.name)}
        <path class="line" d={path(s.points)} style="stroke:{COLORS[i % COLORS.length]}" />
      {/each}
    </svg>
    <div class="legend">
      {#each series as s, i (s.name)}
        <span class="item">
          <span class="dot" style="background:{COLORS[i % COLORS.length]}"></span>
          {s.name}{#if lastVal(s) !== null}<span class="v mono">{format(lastVal(s)!)}</span>{/if}
        </span>
      {/each}
    </div>
  {/if}
</div>

<style>
  .chart {
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--panel, #111214);
    padding: 12px 14px;
  }
  .head {
    font-size: 11px;
    letter-spacing: 0.04em;
    color: var(--text-1);
    margin-bottom: 8px;
  }
  .head .hint {
    margin-left: 8px;
    color: var(--muted-2);
    letter-spacing: 0;
  }
  .empty {
    font-size: 12px;
    color: var(--muted-2);
    padding: 24px 0;
    text-align: center;
  }
  svg {
    width: 100%;
    display: block;
  }
  .grid {
    stroke: var(--border);
    stroke-width: 1;
    vector-effect: non-scaling-stroke;
  }
  .line {
    fill: none;
    stroke-width: 1.4;
    vector-effect: non-scaling-stroke;
  }
  .ylab,
  .xlab {
    fill: var(--muted-2);
    font-size: 9px;
    font-family: ui-monospace, monospace;
  }
  .legend {
    display: flex;
    flex-wrap: wrap;
    gap: 4px 14px;
    margin-top: 8px;
    font-size: 11px;
    color: var(--text-2);
  }
  .item {
    display: inline-flex;
    align-items: center;
    gap: 5px;
  }
  .dot {
    width: 8px;
    height: 8px;
    border-radius: 2px;
    display: inline-block;
  }
  .v {
    color: var(--text-1);
    font-size: 10px;
  }
</style>
