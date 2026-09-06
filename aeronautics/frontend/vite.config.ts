import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

// The dev server proxies /api to the Go boundary so that a local worksheet is
// two commands and no CORS configuration: `go run ./cmd/aero serve` in the
// module root, `pnpm dev` here.
//
// The proxy is a development convenience only. A deployment needs the Go
// service running alongside these assets: a static host serves the page but
// does not execute the calculation core, so nothing here would answer.
const apiTarget = process.env['AERO_API'] ?? 'http://127.0.0.1:8081'

export default defineConfig({
  plugins: [react()],
  server: {
    host: '127.0.0.1',
    port: 5173,
    strictPort: true,
    proxy: { '/api': { target: apiTarget, changeOrigin: false } },
  },
  test: {
    environment: 'node',
    include: ['src/**/*.test.ts', 'src/**/*.test.tsx'],
  },
})
