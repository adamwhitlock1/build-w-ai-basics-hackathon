import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    // 5173 is used by another app on this machine; fail loudly rather than
    // silently drifting to a different port.
    port: 5174,
    strictPort: true,
    proxy: {
      // The Go API. Same-origin in the browser means no CORS and, later,
      // session cookies just work.
      '/api': { target: 'http://127.0.0.1:8080' },
    },
  },
})
