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
      '/current-load': 'http://localhost:8080',
      '/dates': 'http://localhost:8080',
      '/available': 'http://localhost:8080',
      '/tickers': 'http://localhost:8080',
      '/load': 'http://localhost:8080',
      '/reset': 'http://localhost:8080',
      '/seek': 'http://localhost:8080',
    },
  },
})
