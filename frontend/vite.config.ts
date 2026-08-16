import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      // Generated Connect clients import as '@/gen/gcs/v1/...'.
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  test: {
    // Domain resolvers are pure functions — no DOM, no jsdom cost.
    environment: 'node',
    include: ['src/**/*.test.ts'],
  },
})
