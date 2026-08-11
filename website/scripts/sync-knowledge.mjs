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

await mkdir(OUT, { recursive: true })
for (const f of await readdir(OUT).catch(() => [])) {
  if (f.endsWith('.md')) await rm(path.join(OUT, f))
}

// knowledge/index.md is the OKF hub for agents/llms; the site home is instead a
// hand-authored card-grid landing (src/content/docs/index.mdx), so skip it here.
const files = (await readdir(SRC)).filter((f) => f.endsWith('.md') && f !== 'index.md')
for (const f of files) {
  const raw = await readFile(path.join(SRC, f), 'utf8')
  const { fm, body } = splitFrontmatter(raw)
  const title = fmField(fm, 'title') || firstH1(body) || f.replace(/\.md$/, '')
  const description = fmField(fm, 'description')

  let out = stripLeadingH1(body).trimStart()
  // sibling `](name.md)` -> `](./name.md)` so Astro rewrites to the final URL.
  out = out.replace(/\]\(([a-z0-9][a-z0-9-]*\.md)\)/g, '](./$1)')

  const front = [`title: ${JSON.stringify(title)}`]
  if (description) front.push(`description: ${JSON.stringify(description)}`)
  await writeFile(path.join(OUT, f), `---\n${front.join('\n')}\n---\n\n${out}`)
}
console.log(`synced ${files.length} docs -> src/content/docs`)
