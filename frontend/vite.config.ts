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
  },
  test: {
    // Domain resolvers are pure functions — no DOM, no jsdom cost.
    environment: 'node',
    include: ['src/**/*.test.ts'],
  },
})
