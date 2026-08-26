import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      // Generated protobuf types import as '@/gen/gcs/v1/...'.
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    // Bind 0.0.0.0 and fix the port: inside the compose container the default
    // loopback bind would leave the published port answering nothing, and a
    // dev server that silently moves to 3001 breaks the published mapping.
    host: true,
    port: 3000,
    strictPort: true,
    proxy: {
      // The app always fetches a relative /api URL, so the backend needs no
      // CORS policy and the client needs no configured backend address. Only
      // this proxy knows where the backend actually is, and it differs between
      // the host (localhost) and Compose (the service name).
      '/api': {
        target: process.env['GCS_BACKEND_URL'] ?? 'http://localhost:8080',
        // Command CSRF checks compare Origin with the browser's original Host.
        changeOrigin: false,
        // Server-sent events must not be buffered or they arrive in bursts.
        ws: false,
        configure: (proxy) => {
          proxy.on('proxyRes', (proxyRes) => {
            proxyRes.headers['cache-control'] = 'no-cache';
          });
        },
      },
    },
  },
  test: {
    // Domain logic is pure functions and components are rendered to static
    // markup with react-dom/server — no DOM, no jsdom cost.
    environment: 'node',
    include: ['src/**/*.test.ts', 'src/**/*.test.tsx'],
  },
})
