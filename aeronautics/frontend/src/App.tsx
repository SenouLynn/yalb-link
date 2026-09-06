import type { ReactNode } from 'react'
import { useMemo } from 'react'
import { httpTransport } from './api/client.ts'
import { Worksheet } from './Worksheet.tsx'

/**
 * App wires the worksheet to the running service. The base URL is empty because
 * the dev server proxies /api to it and a deployment serves both from one
 * origin; a static host on its own answers nothing, because it does not run the
 * calculation core.
 */
export function App(): ReactNode {
  const transport = useMemo(() => httpTransport(''), [])
  const session = useMemo(() => `worksheet-${String(Date.now())}`, [])
  return <Worksheet transport={transport} session={session} />
}
