// Runtime validation of responses from the Go boundary.
//
// The generated contract in ./contract.ts is types only: it describes what a
// response is expected to look like and vanishes at run time. A response is
// untrusted data — a proxy, a stale deployment or a different service can all
// put something else on the wire — so every field the worksheet reads is
// checked here before anything is allowed to depend on it.
//
// The checks cover what the worksheet actually consumes. A field nothing reads
// is not validated, because a validator that claims more coverage than it has
// is worse than an honest one; adding a read means adding its check.

import type {
  Bound,
  Check,
  Discovery,
  Evaluation,
  Issue,
  Quantity,
  SolvedWing,
  UnitInfo,
} from './contract.ts'

export class ResponseShapeError extends Error {
  constructor(path: string, detail: string) {
    super(`the response is not the shape this worksheet expects: ${path} ${detail}`)
    this.name = 'ResponseShapeError'
  }
}

function fail(path: string, detail: string): never {
  throw new ResponseShapeError(path, detail)
}

function record(value: unknown, path: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    fail(path, 'is not an object')
  }
  return value as Record<string, unknown>
}

function str(value: unknown, path: string): string {
  if (typeof value !== 'string') fail(path, 'is not a string')
  return value
}

function bool(value: unknown, path: string): boolean {
  if (typeof value !== 'boolean') fail(path, 'is not a boolean')
  return value
}

// num rejects NaN and the infinities as well as non-numbers. JSON can carry
// neither, but a hand-written client or a proxy that rewrites bodies can, and a
// NaN that reaches a field would render as "NaN" rather than as an error.
function num(value: unknown, path: string): number {
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    fail(path, 'is not a finite number')
  }
  return value
}

function array(value: unknown, path: string): unknown[] {
  if (!Array.isArray(value)) fail(path, 'is not an array')
  return value
}

function optional<T>(value: unknown, path: string, read: (v: unknown, p: string) => T): T | null {
  if (value === undefined || value === null) return null
  return read(value, path)
}

function strings(value: unknown, path: string): string[] {
  return array(value, path).map((entry, n) => str(entry, `${path}[${String(n)}]`))
}

export function quantity(value: unknown, path: string): Quantity {
  const raw = record(value, path)
  return { value: num(raw['value'], `${path}.value`), unit: str(raw['unit'], `${path}.unit`) }
}

function issues(value: unknown, path: string): Issue[] {
  return array(value, path).map((entry, n) => {
    const raw = record(entry, `${path}[${String(n)}]`)
    return {
      field: str(raw['field'], `${path}[${String(n)}].field`),
      kind: str(raw['kind'], `${path}[${String(n)}].kind`),
      detail: str(raw['detail'], `${path}[${String(n)}].detail`),
    }
  })
}

function checks(value: unknown, path: string): Check[] {
  return array(value, path).map((entry, n) => {
    const at = `${path}[${String(n)}]`
    const raw = record(entry, at)
    return {
      name: str(raw['name'], `${at}.name`),
      case: raw['case'] === undefined ? '' : str(raw['case'], `${at}.case`),
      subject: str(raw['subject'], `${at}.subject`),
      direction: str(raw['direction'], `${at}.direction`),
      priority: str(raw['priority'], `${at}.priority`),
      status: str(raw['status'], `${at}.status`),
      result: str(raw['result'], `${at}.result`),
      evidence: raw['evidence'] === undefined ? '' : str(raw['evidence'], `${at}.evidence`),
      detail: raw['detail'] === undefined ? '' : str(raw['detail'], `${at}.detail`),
      bound: quantity(raw['bound'], `${at}.bound`),
      actual: optional(raw['actual'], `${at}.actual`, quantity),
      margin: num(raw['margin'], `${at}.margin`),
    }
  })
}

function bound(value: unknown, path: string): Bound {
  const raw = record(value, path)
  return {
    subject: str(raw['subject'], `${path}.subject`),
    direction: str(raw['direction'], `${path}.direction`),
    detail: raw['detail'] === undefined ? '' : str(raw['detail'], `${path}.detail`),
    controlling: raw['controlling'] === undefined ? [] : strings(raw['controlling'], `${path}.controlling`),
    value: optional(raw['value'], `${path}.value`, quantity),
    known: bool(raw['known'], `${path}.known`),
    partial: bool(raw['partial'], `${path}.partial`),
  }
}

function solvedWing(value: unknown, path: string): SolvedWing {
  const raw = record(value, path)
  const parameters = array(raw['parameters'], `${path}.parameters`).map((entry, n) => {
    const at = `${path}.parameters[${String(n)}]`
    const p = record(entry, at)
    return {
      key: str(p['key'], `${at}.key`),
      role: str(p['role'], `${at}.role`),
      datum: p['datum'] === undefined ? '' : str(p['datum'], `${at}.datum`),
      equationId: p['equationId'] === undefined ? '' : str(p['equationId'], `${at}.equationId`),
      revision: p['revision'] === undefined ? '' : str(p['revision'], `${at}.revision`),
      dependsOn: p['dependsOn'] === undefined ? [] : strings(p['dependsOn'], `${at}.dependsOn`),
      value: quantity(p['value'], `${at}.value`),
    }
  })
  return {
    datum: str(raw['datum'], `${path}.datum`),
    solveMode: str(raw['solveMode'], `${path}.solveMode`),
    drivers: strings(raw['drivers'], `${path}.drivers`),
    parameters,
    outline: [],
  }
}

export function evaluation(value: unknown, path = 'evaluation'): Evaluation {
  const raw = record(value, path)
  const request = record(raw['request'], `${path}.request`)
  return {
    request: {
      session: str(request['session'], `${path}.request.session`),
      sequence: num(request['sequence'], `${path}.request.sequence`),
    },
    snapshot: str(raw['snapshot'], `${path}.snapshot`),
    geometry: str(raw['geometry'], `${path}.geometry`),
    aggregate: str(raw['aggregate'], `${path}.aggregate`),
    hasRequired: bool(raw['hasRequired'], `${path}.hasRequired`),
    wing: raw['wing'] === undefined || raw['wing'] === null
      ? null
      : solvedWing(raw['wing'], `${path}.wing`),
    checks: checks(raw['checks'], `${path}.checks`),
    areaLower: bound(raw['areaLower'], `${path}.areaLower`),
    areaUpper: bound(raw['areaUpper'], `${path}.areaUpper`),
    mass: {
      detail: (() => {
        const mass = record(raw['mass'], `${path}.mass`)
        return mass['detail'] === undefined ? '' : str(mass['detail'], `${path}.mass.detail`)
      })(),
      lower: bound(record(raw['mass'], `${path}.mass`)['lower'], `${path}.mass.lower`),
      upper: bound(record(raw['mass'], `${path}.mass`)['upper'], `${path}.mass.upper`),
      complete: bool(record(raw['mass'], `${path}.mass`)['complete'], `${path}.mass.complete`),
      empty: bool(record(raw['mass'], `${path}.mass`)['empty'], `${path}.mass.empty`),
    },
    conflicts: array(raw['conflicts'], `${path}.conflicts`).map((entry, n) => {
      const at = `${path}.conflicts[${String(n)}]`
      const c = record(entry, at)
      return {
        summary: str(c['summary'], `${at}.summary`),
        detail: str(c['detail'], `${at}.detail`),
        group: strings(c['group'], `${at}.group`),
        alternatives: array(c['alternatives'], `${at}.alternatives`).map((alt, m) => {
          const altAt = `${at}.alternatives[${String(m)}]`
          const a = record(alt, altAt)
          return {
            name: str(a['name'], `${altAt}.name`),
            description: str(a['description'], `${altAt}.description`),
            command: record(a['command'], `${altAt}.command`) as never,
          }
        }),
      }
    }),
    patterns: strings(raw['patterns'], `${path}.patterns`),
    definitionIssues: issues(raw['definitionIssues'], `${path}.definitionIssues`),
    geometryIssues: issues(raw['geometryIssues'], `${path}.geometryIssues`),
    configurationIssues: issues(raw['configurationIssues'], `${path}.configurationIssues`),
  }
}

export function applyResponse(value: unknown): { design: unknown; evaluation: Evaluation } {
  const raw = record(value, 'apply')
  return {
    design: record(raw['design'], 'apply.design'),
    evaluation: evaluation(raw['evaluation'], 'apply.evaluation'),
  }
}

// discovery validates only what the worksheet reads from it: the contract and
// model versions, and the equation identities and revisions a saved draft is
// checked against.
export function discovery(value: unknown): Pick<Discovery, 'contractVersion' | 'version'> & {
  equationRevisions: Record<string, string>
} {
  const raw = record(value, 'discovery')
  const equationRevisions: Record<string, string> = {}
  for (const [n, entry] of array(raw['equations'], 'discovery.equations').entries()) {
    const at = `discovery.equations[${String(n)}]`
    const equation = record(entry, at)
    equationRevisions[str(equation['id'], `${at}.id`)] = str(equation['revision'], `${at}.revision`)
  }
  return {
    contractVersion: str(raw['contractVersion'], 'discovery.contractVersion'),
    version: str(raw['version'], 'discovery.version'),
    equationRevisions,
  }
}

/**
 * units validates the unit table, including the exact SI factor a field uses to
 * render a stored value in a chosen unit. A wrong factor here would silently
 * misreport every number on the page, so it is checked like any other field.
 */
export function units(value: unknown): UnitInfo[] {
  const raw = record(value, 'units')
  return array(raw['units'], 'units.units').map((entry, n) => {
    const at = `units.units[${String(n)}]`
    const unit = record(entry, at)
    const factor = num(unit['factorToSi'], `${at}.factorToSi`)
    if (factor <= 0) fail(`${at}.factorToSi`, 'is not a positive factor')
    return {
      symbol: str(unit['symbol'], `${at}.symbol`),
      dimension: str(unit['dimension'], `${at}.dimension`),
      factorToSi: factor,
      si: bool(unit['si'], `${at}.si`),
    }
  })
}

export interface BoundaryFailure {
  error: string
  message: string
  issues: Issue[]
}

export function failure(value: unknown): BoundaryFailure {
  const raw = record(value, 'failure')
  return {
    error: str(raw['error'], 'failure.error'),
    message: str(raw['message'], 'failure.message'),
    issues: raw['issues'] === undefined ? [] : issues(raw['issues'], 'failure.issues'),
  }
}
