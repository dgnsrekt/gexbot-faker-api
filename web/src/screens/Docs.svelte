<script lang="ts">
  let { baseUrl }: { baseUrl: string } = $props()

  // The three human/agent doc surfaces, all embedded in this server binary and served
  // offline (no external hosting). Each opens in a new tab.
  const docs: { href: string; glyph: string; title: string; tag: string; desc: string }[] = [
    {
      href: '/guides',
      glyph: '§',
      title: 'Guides',
      tag: 'handbook',
      desc: 'Concepts, quick start, and how-tos — the human-readable handbook, embedded offline.',
    },
    {
      href: '/docs',
      glyph: '{ }',
      title: 'REST API',
      tag: 'openapi',
      desc: 'Swagger reference for every endpoint — GexBot parity plus the faker control plane.',
    },
    {
      href: '/asyncapi',
      glyph: '≈',
      title: 'WebSocket API',
      tag: 'asyncapi',
      desc: 'AsyncAPI reference for the five streaming hubs and the Web PubSub protocol.',
    },
  ]

  // Raw machine-readable specs + agent files, for tooling.
  const specs: { href: string; label: string }[] = [
    { href: '/openapi.yaml', label: '/openapi.yaml' },
    { href: '/asyncapi.yaml', label: '/asyncapi.yaml' },
    { href: '/llms.txt', label: '/llms.txt' },
    { href: '/llms-full.txt', label: '/llms-full.txt' },
  ]
</script>

<div class="wrap">
  <div class="cards">
    {#each docs as d (d.href)}
      <a class="doccard" href={`${baseUrl}${d.href}`} target="_blank" rel="noreferrer">
        <div class="doccard-top">
          <span class="glyph mono">{d.glyph}</span>
          <span class="tag mono">{d.tag}</span>
        </div>
        <div class="doccard-title">{d.title}</div>
        <div class="doccard-desc">{d.desc}</div>
        <div class="doccard-open mono">open ↗</div>
      </a>
    {/each}
  </div>

  <div class="card raw">
    <div class="card-head">Raw specs &amp; agent files</div>
    <div class="raw-links">
      {#each specs as s (s.href)}
        <a class="raw-link mono" href={`${baseUrl}${s.href}`} target="_blank" rel="noreferrer"
          >{s.label} ↗</a
        >
      {/each}
    </div>
    <div class="raw-note">
      Everything here is served by this server — the guides and REST reference are embedded and
      work offline.
    </div>
  </div>
</div>

<style>
  .wrap {
    padding: 18px 20px;
    max-width: 1000px;
  }
  .cards {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
    gap: 12px;
    margin-bottom: 16px;
  }
  .doccard {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 16px;
    border: 1px solid var(--border);
    border-radius: 11px;
    background: var(--panel);
    text-decoration: none;
    color: inherit;
    transition:
      border-color 0.12s,
      box-shadow 0.12s,
      transform 0.12s;
  }
  .doccard:hover {
    border-color: var(--green-border);
    box-shadow:
      0 0 0 1px color-mix(in srgb, var(--green) 30%, transparent),
      0 10px 26px -14px rgba(0, 0, 0, 0.7);
    transform: translateY(-1px);
  }
  .doccard-top {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .glyph {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 30px;
    height: 30px;
    padding: 0 7px;
    border-radius: 8px;
    background: var(--green-bg);
    color: var(--green);
    border: 1px solid var(--green-border);
    font-size: 13px;
    font-weight: 600;
  }
  .tag {
    font-size: 11px;
    color: var(--muted-2);
    text-transform: lowercase;
  }
  .doccard-title {
    font-size: 15px;
    font-weight: 600;
    color: var(--text);
  }
  .doccard-desc {
    font-size: 12.5px;
    line-height: 1.5;
    color: var(--muted);
    flex: 1;
  }
  .doccard-open {
    font-size: 12px;
    color: var(--green-2);
  }
  .raw {
    padding: 14px 16px;
  }
  .raw-links {
    display: flex;
    flex-wrap: wrap;
    gap: 8px 16px;
    margin-top: 10px;
  }
  .raw-link {
    font-size: 12.5px;
    color: var(--green-2);
    text-decoration: none;
  }
  .raw-link:hover {
    color: var(--green);
    text-decoration: underline;
  }
  .raw-note {
    margin-top: 12px;
    font-size: 12px;
    color: var(--muted-2);
    line-height: 1.5;
  }
</style>
