import { defineCollection } from 'astro:content'
import { z } from 'astro:schema'
import { docsLoader } from '@astrojs/starlight/loaders'
import { docsSchema } from '@astrojs/starlight/schema'

// Docs content is synced from the repo-root knowledge/ OKF bundle by
// scripts/sync-knowledge.mjs. Extend Starlight's schema to accept the OKF
// frontmatter fields (type/tags/timestamp) so validation passes.
export const collections = {
  docs: defineCollection({
    loader: docsLoader(),
    schema: docsSchema({
      extend: z.object({
        type: z.string().optional(),
        tags: z.array(z.string()).optional(),
        timestamp: z.string().optional(),
        // A mono glyph shown before the page title (injected per slug by
        // scripts/sync-knowledge.mjs; reuses the Studio's nav icon language).
        glyph: z.string().optional(),
      }),
    }),
  }),
}
