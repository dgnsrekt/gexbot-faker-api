// Sync the repo-root knowledge/ OKF bundle into Starlight's content dir.
//
// knowledge/ is the single source of truth (OKF: frontmatter title + a body H1,
// readable standalone). Starlight wants the title in frontmatter and no body H1
// (it renders the title heading itself). This script bridges the two: it keeps
// title/description, strips the leading body H1, synthesizes frontmatter for the
// hub/log files that have none, and rewrites sibling `.md` links to relative form
// so Astro resolves them with the configured base.
import { readdir, readFile, writeFile, mkdir, rm } from 'node:fs/promises'
import path from 'node:path'

const SRC = path.resolve('../knowledge')
const OUT = path.resolve('src/content/docs')

// Per-topic hero glyphs, reusing the GEX Faker Studio's nav icon language. Kept
// here (not in the OKF frontmatter) so knowledge/ stays a clean plain-Markdown
// source; injected into each synced doc's frontmatter and rendered by the
// PageTitle override. Mirrors the landing's card glyphs.
const GLYPHS = {
  overview: '⬡',
  'quick-start': '▸',
  studio: '▣',
  'download-data': '↓',
  'materialize-load': '⇡',
  'docker-observability': '▦',
  'point-a-client': '⇆',
  gexfakercli: '❯',
  'rest-api': '⧉',
  websockets: '≈',
  configuration: '⚙',
  daemon: '⟳',
  troubleshooting: '⚠',
  log: '≡',
}

function splitFrontmatter(text) {
  const m = text.match(/^---\n([\s\S]*?)\n---\n?/)
  if (!m) return { fm: null, body: text }
  return { fm: m[1], body: text.slice(m[0].length) }
}
function fmField(fm, key) {
  if (!fm) return null
  const m = fm.match(new RegExp(`^${key}:\\s*(.*)$`, 'm'))
  return m ? m[1].trim() : null
}
function firstH1(body) {
  const m = body.match(/^#\s+(.+)$/m)
  return m ? m[1].trim() : null
}
function stripLeadingH1(body) {
  return body.replace(/^\s*#\s+.+\n+/, '')
}

// Transform one knowledge/*.md into a Starlight doc: keep title/description,
// strip the body H1, inject the per-slug glyph, and make sibling .md links
// relative. index.md is the OKF hub / hand-authored landing, so it is skipped.
async function syncDir(srcDir, outDir) {
  await mkdir(outDir, { recursive: true })
  for (const f of await readdir(outDir).catch(() => [])) {
    if (f.endsWith('.md')) await rm(path.join(outDir, f))
  }
  const files = (await readdir(srcDir))
    .filter((f) => f.endsWith('.md') && f !== 'index.md')
  for (const f of files) {
    const raw = await readFile(path.join(srcDir, f), 'utf8')
    const { fm, body } = splitFrontmatter(raw)
    const title = fmField(fm, 'title') || firstH1(body) || f.replace(/\.md$/, '')
    const description = fmField(fm, 'description')

    let out = stripLeadingH1(body).trimStart()
    out = out.replace(/\]\(([a-z0-9][a-z0-9-]*\.md)\)/g, '](./$1)')

    const front = [`title: ${JSON.stringify(title)}`]
    if (description) front.push(`description: ${JSON.stringify(description)}`)
    const glyph = GLYPHS[f.replace(/\.md$/, '')]
    if (glyph) front.push(`glyph: ${JSON.stringify(glyph)}`)
    await writeFile(path.join(outDir, f), `---\n${front.join('\n')}\n---\n\n${out}`)
  }
  return files.length
}

// English (root) + the Spanish locale (knowledge/es/ -> src/content/docs/es/).
// The es slugs match the English ones, so Starlight pairs them across locales.
const enCount = await syncDir(SRC, OUT)
let esCount = 0
if (await readdir(path.join(SRC, 'es')).then(() => true, () => false)) {
  esCount = await syncDir(path.join(SRC, 'es'), path.join(OUT, 'es'))
}
console.log(`synced ${enCount} en + ${esCount} es docs -> src/content/docs`)
