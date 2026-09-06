import { spawn } from 'node:child_process'
import { createServer } from 'node:net'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'
import type { TestProject } from 'vitest/node'

// The browser tests run against the real Go boundary rather than a recorded
// transcript. The physics has to come from the calculator for a journey test to
// mean anything, and a stub would drift from it the moment an equation changed.
//
// Request ordering is tested separately with a controllable transport, because
// there the point is which response is accepted, not what it contains.

const moduleRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..')

async function freePort(): Promise<number> {
  return new Promise((resolvePort, reject) => {
    const probe = createServer()
    probe.once('error', reject)
    probe.listen(0, '127.0.0.1', () => {
      const address = probe.address()
      if (address === null || typeof address === 'string') {
        probe.close()
        reject(new Error('could not obtain a port'))
        return
      }
      const { port } = address
      probe.close(() => { resolvePort(port) })
    })
  })
}

async function waitForReady(url: string): Promise<void> {
  const deadline = Date.now() + 90_000
  for (;;) {
    try {
      const response = await fetch(`${url}/api/v1/discovery`)
      if (response.ok) return
    } catch {
      // The server is still compiling or binding; keep trying until the
      // deadline, then fail with a message that says what was being waited on.
    }
    if (Date.now() > deadline) {
      throw new Error(`the aero server did not become ready on ${url}`)
    }
    await new Promise((sleep) => setTimeout(sleep, 200))
  }
}

export default async function setup(project: TestProject) {
  const port = await freePort()
  const baseUrl = `http://127.0.0.1:${String(port)}`
  // Detached with its own process group, because `go run` compiles to a second
  // binary and signalling only the parent would leave the server holding the
  // port. Every stream is ignored so no handle keeps the test runner alive.
  const server = spawn('go', ['run', './cmd/aero', 'serve', '--addr', `127.0.0.1:${String(port)}`], {
    cwd: moduleRoot,
    stdio: 'ignore',
    detached: true,
  })
  server.once('error', (error) => {
    throw new Error(`could not start the aero server: ${error.message}`)
  })
  server.unref()
  await waitForReady(baseUrl)
  project.provide('aeroBaseUrl', baseUrl)
  return () => {
    if (server.pid !== undefined) {
      try {
        process.kill(-server.pid, 'SIGKILL')
      } catch {
        // Already gone, which is the state the teardown wanted.
      }
    }
  }
}
