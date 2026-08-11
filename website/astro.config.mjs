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
      // English (root) + Spanish. Untranslated pages fall back to English with a
      // notice; the header gets a language switcher automatically.
      defaultLocale: 'root',
      locales: {
        root: { label: 'English', lang: 'en' },
        es: { label: 'Español', lang: 'es' },
      },
      // GEX Faker Studio brand system: the Reflection mark + Space Grotesk
      // wordmark, green accent on the near-black Studio palette, the stripe
      // texture, JetBrains Mono, mono-glyph cards. Fonts are self-hosted
      // (@fontsource) so the embedded /guides build stays offline.
      customCss: [
        '@fontsource/space-grotesk/400.css',
        '@fontsource/space-grotesk/500.css',
        '@fontsource/space-grotesk/700.css',
        '@fontsource/jetbrains-mono/400.css',
        '@fontsource/jetbrains-mono/500.css',
        '@fontsource/jetbrains-mono/700.css',
        './src/styles/theme.css',
      ],
      favicon: '/favicon.svg',
      components: {
        // The Reflection mark + wordmark lockup, and per-topic title glyphs.
        SiteTitle: './src/components/SiteTitle.astro',
        PageTitle: './src/components/PageTitle.astro',
      },
      social: [
        { icon: 'github', label: 'GitHub', href: 'https://github.com/dgnsrekt/gexbot-faker-api' },
      ],
      // IA mirrors knowledge/index.md: Use it (GUI / getting data flowing) vs
      // Build with it (clients / API / streaming). Slugs map to knowledge/<slug>.md.
      sidebar: [
        { label: 'Use it', translations: { es: 'Úsalo' }, items: [
          'overview', 'quick-start', 'studio', 'download-data', 'materialize-load', 'docker-observability',
        ] },
        { label: 'Build with it', translations: { es: 'Desarrolla con él' }, items: [
          'point-a-client', 'gexfakercli', 'rest-api', 'websockets', 'configuration', 'daemon',
        ] },
        { label: 'Help', translations: { es: 'Ayuda' }, items: ['troubleshooting'] },
      ],
    }),
  ],
})
