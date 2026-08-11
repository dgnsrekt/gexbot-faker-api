// @ts-check
import { defineConfig } from 'astro/config'
import starlight from '@astrojs/starlight'

// One source, two builds. The embedded build (served by the faker at /guides)
// uses base=/guides; the GitHub Pages build uses base=/gexbot-faker-api. The
// build:embed / build:pages npm scripts set DOCS_BASE accordingly.
const base = process.env.DOCS_BASE ?? '/guides'

// `site` is only meaningful for the public GitHub Pages build (canonical URLs +
// sitemap). The embedded /guides build leaves it unset so it doesn't emit
// canonicals pointing at a Pages path that doesn't match its own base.
const site = process.env.DOCS_SITE || undefined

export default defineConfig({
  site,
  base,
  trailingSlash: 'ignore',
  integrations: [
    starlight({
      title: 'GEX Faker',
      description:
        'Mock server that replays historical GexBot options/GEX market data over REST and WebSocket, with a web UI (Studio) and an agent CLI (gexfakercli).',
      // LM Studio-flavored palette (violet accent + near-black dark theme).
      customCss: ['./src/styles/theme.css'],
      social: [
        { icon: 'github', label: 'GitHub', href: 'https://github.com/dgnsrekt/gexbot-faker-api' },
      ],
      // IA mirrors knowledge/index.md: Use it (GUI / getting data flowing) vs
      // Build with it (clients / API / streaming). Slugs map to knowledge/<slug>.md.
      sidebar: [
        { label: 'Use it', items: [
          'overview', 'quick-start', 'studio', 'download-data', 'materialize-load', 'docker-observability',
        ] },
        { label: 'Build with it', items: [
          'point-a-client', 'gexfakercli', 'rest-api', 'websockets', 'configuration', 'daemon',
        ] },
        { label: 'Help', items: ['troubleshooting'] },
      ],
    }),
  ],
})
