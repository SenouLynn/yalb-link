// The transport between the worksheet and the Go boundary.
//
// It is an interface, not a bare fetch call, for one reason that matters: the
// request-ordering tests need to control when a response arrives, and the
// journey tests need the real physics. Both use the same worksheet.

import type {
  ApplyRequest,
  UnitInfo,
  Command,
  Design,
  Evaluation,
  EvaluateRequest,
  Issue,
  PreviewRequest,
  Request as RequestIdentity,
  SweepRequest,
  SweepResponse,
} from './contract.ts'
import { CONTRACT_VERSION } from './contract.ts'
import * as validate from './validate.ts'

/** BoundaryError is a call the service refused, with its field issues. */
export class BoundaryError extends Error {
  readonly kind: string
  readonly issues: Issue[]

  constructor(kind: string, message: string, issues: Issue[]) {
    super(message)
    this.name = 'BoundaryError'
    this.kind = kind
    this.issues = issues
  }
}

/** TransportError is a call that never reached the service, or never returned. */
export class TransportError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'TransportError'
  }
}

export interface ApplyResult {
  design: Design
  evaluation: Evaluation
}

export interface Discovered {
  contractVersion: string
  version: string
  equationRevisions: Record<string, string>
}

export interface Transport {
  discover(signal?: AbortSignal): Promise<Discovered>
  units(signal?: AbortSignal): Promise<UnitInfo[]>
  evaluate(request: EvaluateRequest, signal?: AbortSignal): Promise<Evaluation>
  apply(request: ApplyRequest, signal?: AbortSignal): Promise<ApplyResult>
  preview(request: PreviewRequest, signal?: AbortSignal): Promise<Evaluation>
  sweep(request: SweepRequest, signal?: AbortSignal): Promise<SweepResponse>
}

/** The versioned path prefix the Go service serves, matching its own. */
export const API_PREFIX = `/api/${CONTRACT_VERSION}`

async function readBody(response: Response, url: string): Promise<unknown> {
  const text = await response.text()
  try {
    return JSON.parse(text) as unknown
  } catch {
    throw new TransportError(`${url} answered ${String(response.status)} with a body that is not JSON`)
  }
}

async function call(base: string, path: string, body: unknown, signal?: AbortSignal): Promise<unknown> {
  const url = `${base}${API_PREFIX}${path}`
  let response: Response
  try {
    response = await fetch(url, {
      method: body === undefined ? 'GET' : 'POST',
      headers: body === undefined ? {} : { 'Content-Type': 'application/json' },
      body: body === undefined ? null : JSON.stringify(body),
      ...(signal ? { signal } : {}),
    })
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') throw error
    throw new TransportError(
      `${url} could not be reached: ${error instanceof Error ? error.message : String(error)}`,
    )
  }
  const decoded = await readBody(response, url)
  if (!response.ok) {
    const failure = validate.failure(decoded)
    throw new BoundaryError(failure.error, failure.message, failure.issues)
  }
  return decoded
}

/** httpTransport talks to a running aero service. */
export function httpTransport(base = ''): Transport {
  return {
    async discover(signal) {
      return validate.discovery(await call(base, '/discovery', undefined, signal))
    },
    async units(signal) {
      return validate.units(await call(base, '/units', undefined, signal))
    },
    async evaluate(request, signal) {
      return validate.evaluation(await call(base, '/evaluate', request, signal))
    },
    async apply(request, signal) {
      const applied = validate.applyResponse(await call(base, '/apply', request, signal))
      return { design: applied.design as Design, evaluation: applied.evaluation }
    },
    async preview(request, signal) {
      const raw = await call(base, '/preview', request, signal)
      if (typeof raw !== 'object' || raw === null) {
        throw new TransportError('the preview response is not an object')
      }
      return validate.evaluation((raw as Record<string, unknown>)['after'], 'preview.after')
    },
    async sweep(request, signal) {
      return validate.sweep(await call(base, '/sweep', request, signal))
    },
  }
}

/** evaluateRequest builds an evaluation call for one design under one identity. */
export function evaluateRequest(request: RequestIdentity, design: Design): EvaluateRequest {
  return { request, design }
}

/** applyRequest builds an apply call for one design and one command. */
export function applyRequest(
  request: RequestIdentity,
  design: Design,
  commands: Command[],
): ApplyRequest {
  return { request, design, commands }
}
