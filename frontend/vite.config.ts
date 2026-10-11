import tailwindcss from '@tailwindcss/vite'
import { tanstackRouter } from '@tanstack/router-plugin/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// Both default to the normal local setup. The overrides exist so a test run
// can point a second dev server at a fake API without touching your own.
const apiTarget = process.env.PORCHLIGHT_API ?? 'http://127.0.0.1:8080'
const webPort = Number(process.env.PORCHLIGHT_WEB_PORT ?? 5174)

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    // Must come before react(): it turns the files in src/routes into
    // src/routeTree.gen.ts and splits each route's component into its own chunk.
    tanstackRouter({ target: 'react', autoCodeSplitting: true }),
    react(),
    tailwindcss(),
  ],
  server: {
    // 5173 is used by another app on this machine; fail loudly rather than
    // silently drifting to a different port.
    port: webPort,
    strictPort: true,
    proxy: {
      // The Go API. Same-origin in the browser means no CORS, and the
      // session cookie is simply sent with every /api request.
      '/api': { target: apiTarget },
    },
  },
})
