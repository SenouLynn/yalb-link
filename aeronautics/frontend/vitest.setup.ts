import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterEach } from 'vitest'

// Node ships its own experimental localStorage, which can shadow jsdom's and
// arrives without the Storage methods. The worksheet only ever uses the one it
// is handed, so the cleanup checks rather than assuming.
afterEach(() => {
  cleanup()
  if (typeof localStorage.clear === 'function') localStorage.clear()
})
