// Generate llms.txt + llms-full.txt from the knowledge/ OKF bundle.
//
// - llms.txt: an llmstxt.org-style map (Use it / Build with it) linking each topic
//   to its canonical GitHub source, so the links are correct from anywhere.
// - llms-full.txt: every topic concatenated for direct ingestion into an agent.
//
// Both are written to the repo root (committed, served by the faker at /llms.txt
// and /llms-full.txt) and into website/public/ (served by the docs site).
import { readFile, writeFile, mkdir } from 'node:fs/promises'
import path from 'node:path'

const SRC = path.resolve('../knowledge')
const GH = 'https://github.com/dgnsrekt/gexbot-faker-api/blob/main'

const GROUPS = [
  ['Use it', ['overview', 'quick-start', 'studio', 'download-data', 'materialize-load', 'docker-observability']],
  ['Build with it', ['point-a-client', 'gexfakercli', 'rest-api', 'websockets', 'configuration', 'daemon']],
  ['Troubleshooting', ['troubleshooting']],
]

const fm = (t) => (t.match(/^---\n([\s\S]*?)\n---\n?/) || [, ''])[1]
const field = (f, k) => ((f.match(new RegExp(`^${k}:\\s*(.*)$`, 'm')) || [, ''])[1] || '').trim()
const body = (t) => t.replace(/^---\n[\s\S]*?\n---\n?/, '')
const read = (name) => readFile(path.join(SRC, `${name}.md`), 'utf8')

// --- llms.txt ---
let map =
  '# GEX Faker API\n\n' +
  '> Mock server that replays historical GexBot options/GEX market data over REST and WebSocket, ' +
  'with a web UI (Studio) and an agent CLI (gexfakercli). Point an agent at this map — or at ' +
  'knowledge/index.md — to answer setup, usage, and integration questions from the source.\n'
for (const [label, names] of GROUPS) {
  map += `\n## ${label}\n`
  for (const n of names) {
    const t = await read(n)
    map += `- [${field(fm(t), 'title')}](${GH}/knowledge/${n}.md): ${field(fm(t), 'description')}\n`
  }
}
map +=
  '\n## Optional\n' +
  `- [OpenAPI spec](${GH}/api/openapi.yaml): the REST surface (served live at /openapi.yaml, Swagger UI at /docs)\n` +
  `- [Full knowledge base](${GH}/llms-full.txt): every topic concatenated for direct ingestion\n`

// --- llms-full.txt ---
const order = ['index', ...GROUPS.flatMap((g) => g[1])]
let full =
  '# GEX Faker API — full knowledge base\n\n' +
  `Generated from knowledge/*.md. Source of truth: ${GH}/knowledge/\n`
for (const n of order) {
  full += `\n\n${'='.repeat(72)}\n\n${body(await read(n)).trim()}\n`
}

await mkdir('public', { recursive: true })
for (const [p, c] of [
  ['../llms.txt', map],
  ['../llms-full.txt', full],
  ['public/llms.txt', map],
  ['public/llms-full.txt', full],
]) {
  await writeFile(path.resolve(p), c)
}
console.log('generated llms.txt + llms-full.txt (repo root + website/public)')
