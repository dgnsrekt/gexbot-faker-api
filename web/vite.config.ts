import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

// Served by the Go server under /studio, so assets must resolve relative to that
// base. During `npm run dev`, /studio/api/* is proxied to the running faker.
export default defineConfig({
  base: '/studio/',
  plugins: [svelte()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/studio/api': 'http://localhost:8080',
      // Control-plane + market-data endpoints the SPA calls directly.
      '/current-date': 'http://localhost:8080',
      '/available-dates': 'http://localhost:8080',
      '/available-data': 'http://localhost:8080',
      '/tickers': 'http://localhost:8080',
      '/reload-date': 'http://localhost:8080',
      '/reset-cache': 'http://localhost:8080',
      '/seek-to-timestamp': 'http://localhost:8080',
    },
  },
})
