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
  AuxiliaryContribution,
  Bound,
  CaseLoad,
  Check,
  SketchCurve,
  SketchDimension,
  Discovery,
  ElectricalBudget,
  Evaluation,
  Explanation,
  Issue,
  MassContribution,
  MassProperties,
  MissionResult,
  Point,
  Position,
  PowerFeasibility,
  PowerSearchResponse,
  PowerSearchCandidate,
  PowerSearchInterval,
  Quantity,
  SegmentAvailability,
  SegmentPower,
  SegmentResult,
  SolvedWing,
  SupplyCheck,
  SweepResponse,
  SweepSample,
  ThrustCheck,
  Trace,
  UnitInfo,
  SketchView,
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

function loads(value: unknown, path: string): CaseLoad[] {
  return array(value, path).map((entry, n) => {
    const at = `${path}[${String(n)}]`
    const raw = record(entry, at)
    return {
      case: str(raw['case'], `${at}.case`),
      priority: str(raw['priority'], `${at}.priority`),
      status: str(raw['status'], `${at}.status`),
      detail: raw['detail'] === undefined ? '' : str(raw['detail'], `${at}.detail`),
      requiredLift: optional(raw['requiredLift'], `${at}.requiredLift`, quantity),
    }
  })
}

function point(value: unknown, path: string): Point {
  const raw = record(value, path)
  return {
    x: quantity(raw['x'], `${path}.x`),
    y: quantity(raw['y'], `${path}.y`),
    z: quantity(raw['z'], `${path}.z`),
  }
}

function position(value: unknown, path: string): Position {
  const raw = record(value, path)
  return {
    x: optional(raw['x'], `${path}.x`, quantity),
    y: optional(raw['y'], `${path}.y`, quantity),
    z: optional(raw['z'], `${path}.z`, quantity),
  }
}

function curves(value: unknown, path: string): SketchCurve[] {
  return array(value, path).map((entry, n) => {
    const at = `${path}[${String(n)}]`
    const raw = record(entry, at)
    return {
      label: str(raw['label'], `${at}.label`),
      role: str(raw['role'], `${at}.role`),
      points: array(raw['points'], `${at}.points`)
        .map((p, m) => point(p, `${at}.points[${String(m)}]`)),
      mirrored: bool(raw['mirrored'], `${at}.mirrored`),
      closed: bool(raw['closed'], `${at}.closed`),
    }
  })
}

function dimensions(value: unknown, path: string): SketchDimension[] {
  return array(value, path).map((entry, n) => {
    const at = `${path}[${String(n)}]`
    const raw = record(entry, at)
    return {
      key: str(raw['key'], `${at}.key`),
      label: str(raw['label'], `${at}.label`),
      detail: str(raw['detail'], `${at}.detail`),
      kind: str(raw['kind'], `${at}.kind`),
      plane: str(raw['plane'], `${at}.plane`),
      from: point(raw['from'], `${at}.from`),
      to: point(raw['to'], `${at}.to`),
      value: quantity(raw['value'], `${at}.value`),
    }
  })
}

function views(value: unknown, path: string): SketchView[] {
  return array(value, path).map((entry, n) => {
    const at = `${path}[${String(n)}]`
    const raw = record(entry, at)
    return {
      view: str(raw['view'], `${at}.view`),
      datum: str(raw['datum'], `${at}.datum`),
      across: str(raw['across'], `${at}.across`),
      up: str(raw['up'], `${at}.up`),
      curves: curves(raw['curves'], `${at}.curves`),
      dimensions: dimensions(raw['dimensions'], `${at}.dimensions`),
    }
  })
}

function substitutions(value: unknown, path: string): { name: string; value: Quantity }[] {
  return array(value, path).map((entry, n) => {
    const at = `${path}[${String(n)}]`
    const raw = record(entry, at)
    return { name: str(raw['name'], `${at}.name`), value: quantity(raw['value'], `${at}.value`) }
  })
}

function explanations(value: unknown, path: string): Explanation[] {
  return array(value, path).map((entry, n) => {
    const at = `${path}[${String(n)}]`
    const raw = record(entry, at)
    return {
      key: str(raw['key'], `${at}.key`),
      role: str(raw['role'], `${at}.role`),
      equationId: raw['equationId'] === undefined ? '' : str(raw['equationId'], `${at}.equationId`),
      revision: raw['revision'] === undefined ? '' : str(raw['revision'], `${at}.revision`),
      expression: raw['expression'] === undefined ? '' : str(raw['expression'], `${at}.expression`),
      detail: str(raw['detail'], `${at}.detail`),
      substitutions: substitutions(raw['substitutions'], `${at}.substitutions`),
      dependsOn: strings(raw['dependsOn'], `${at}.dependsOn`),
      value: quantity(raw['value'], `${at}.value`),
    }
  })
}

function contributions(value: unknown, path: string): MassContribution[] {
  return array(value, path).map((entry, n) => {
    const at = `${path}[${String(n)}]`
    const raw = record(entry, at)
    return {
      name: str(raw['name'], `${at}.name`),
      role: str(raw['role'], `${at}.role`),
      detail: raw['detail'] === undefined ? '' : str(raw['detail'], `${at}.detail`),
      moments: array(raw['moments'], `${at}.moments`)
        .map((m, i) => quantity(m, `${at}.moments[${String(i)}]`)),
      mass: optional(raw['mass'], `${at}.mass`, quantity),
      position: position(raw['position'], `${at}.position`),
      known: bool(raw['known'], `${at}.known`),
    }
  })
}

/**
 * massProperties validates the balance. The centre of gravity is only read when
 * the service sends one: a station that arrived without a computed status would
 * otherwise be shown as a result.
 */
export function massProperties(value: unknown, path: string): MassProperties {
  const raw = record(value, path)
  return {
    datum: str(raw['datum'], `${path}.datum`),
    status: str(raw['status'], `${path}.status`),
    detail: raw['detail'] === undefined ? '' : str(raw['detail'], `${path}.detail`),
    contributions: contributions(raw['contributions'], `${path}.contributions`),
    total: optional(raw['total'], `${path}.total`, quantity),
    cg: optional(raw['cg'], `${path}.cg`, point),
    complete: bool(raw['complete'], `${path}.complete`),
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
    views: views(raw['views'], `${path}.views`),
    explanations: explanations(raw['explanations'], `${path}.explanations`),
  }
}

function sweepSample(value: unknown, path: string): SweepSample {
  const raw = record(value, path)
  return {
    driver: quantity(raw['driver'], `${path}.driver`),
    value: optional(raw['value'], `${path}.value`, quantity),
    status: str(raw['status'], `${path}.status`),
    feasibility: str(raw['feasibility'], `${path}.feasibility`),
    detail: raw['detail'] === undefined ? '' : str(raw['detail'], `${path}.detail`),
    trace: null,
    hasRequired: bool(raw['hasRequired'], `${path}.hasRequired`),
  }
}

/**
 * sweep validates a sensitivity answer. The snapshot and the settings
 * fingerprint are checked like any other field, because they are what decides
 * whether the answer still belongs to the question being asked; a missing one
 * would silently make every answer look current.
 */
export function sweep(value: unknown, path = 'sweep'): SweepResponse {
  const raw = record(value, path)
  const request = record(raw['request'], `${path}.request`)
  const settings = record(raw['settings'], `${path}.settings`)
  const output = record(settings['output'], `${path}.settings.output`)
  return {
    request: {
      session: str(request['session'], `${path}.request.session`),
      sequence: num(request['sequence'], `${path}.request.sequence`),
    },
    settings: {
      driver: str(settings['driver'], `${path}.settings.driver`),
      output: {
        subject: str(output['subject'], `${path}.settings.output.subject`),
        case: output['case'] === undefined ? '' : str(output['case'], `${path}.settings.output.case`),
      },
      from: quantity(settings['from'], `${path}.settings.from`),
      to: quantity(settings['to'], `${path}.settings.to`),
      samples: num(settings['samples'], `${path}.settings.samples`),
    },
    settingsFingerprint: str(raw['settingsFingerprint'], `${path}.settingsFingerprint`),
    snapshot: str(raw['snapshot'], `${path}.snapshot`),
    solveMode: str(raw['solveMode'], `${path}.solveMode`),
    detail: str(raw['detail'], `${path}.detail`),
    heldFixed: strings(raw['heldFixed'], `${path}.heldFixed`),
    alsoChanged: strings(raw['alsoChanged'], `${path}.alsoChanged`),
    bounds: array(raw['bounds'], `${path}.bounds`).map((entry, n) => {
      const at = `${path}.bounds[${String(n)}]`
      const b = record(entry, at)
      return {
        name: str(b['name'], `${at}.name`),
        direction: str(b['direction'], `${at}.direction`),
        priority: str(b['priority'], `${at}.priority`),
        value: quantity(b['value'], `${at}.value`),
      }
    }),
    samples: array(raw['samples'], `${path}.samples`)
      .map((entry, n) => sweepSample(entry, `${path}.samples[${String(n)}]`)),
    current: sweepSample(raw['current'], `${path}.current`),
    invariant: bool(raw['invariant'], `${path}.invariant`),
  }
}

// The Task 09 answers: the electrical budget, the mission, the supply and
// thrust checks, and the bounded power search.
//
// These carry more optional quantities than anything before them, because a
// power answer is routinely partial: a segment with no capability named still
// reports its drag, and a budget missing one avionics figure still lists the
// loads it does know. `text` reads the strings the service omits when empty,
// and `optional` reads the quantities it omits when unknown; neither is ever
// turned into a zero.

/** text reads a string the service omits rather than sends empty. */
function text(value: unknown, path: string): string {
  return value === undefined || value === null ? '' : str(value, path)
}

function trace(value: unknown, path: string): Trace {
  const raw = record(value, path)
  return {
    equationId: str(raw['equationId'], `${path}.equationId`),
    revision: str(raw['revision'], `${path}.revision`),
    expression: str(raw['expression'], `${path}.expression`),
    substitutions: substitutions(raw['substitutions'], `${path}.substitutions`),
    result: quantity(raw['result'], `${path}.result`),
  }
}

function traces(value: unknown, path: string): Trace[] {
  if (value === undefined || value === null) return []
  return array(value, path).map((entry, n) => trace(entry, `${path}[${String(n)}]`))
}

function auxiliaryContributions(value: unknown, path: string): AuxiliaryContribution[] {
  if (value === undefined || value === null) return []
  return array(value, path).map((entry, n) => {
    const at = `${path}[${String(n)}]`
    const raw = record(entry, at)
    return {
      name: str(raw['name'], `${at}.name`),
      component: text(raw['component'], `${at}.component`),
      side: str(raw['side'], `${at}.side`),
      evidence: text(raw['evidence'], `${at}.evidence`),
      detail: text(raw['detail'], `${at}.detail`),
      continuous: optional(raw['continuous'], `${at}.continuous`, quantity),
      peak: optional(raw['peak'], `${at}.peak`, quantity),
      known: bool(raw['known'], `${at}.known`),
    }
  })
}

/**
 * electricalBudget validates the auxiliary demand. `complete` is read as the
 * boolean it is rather than inferred from the totals: a budget can carry a
 * continuous figure and still be incomplete, which is exactly the case a
 * missing servo draw produces, and inferring it here would present that as a
 * whole answer.
 */
export function electricalBudget(value: unknown, path: string): ElectricalBudget {
  const raw = record(value, path)
  return {
    status: str(raw['status'], `${path}.status`),
    detail: text(raw['detail'], `${path}.detail`),
    evidence: text(raw['evidence'], `${path}.evidence`),
    loads: auxiliaryContributions(raw['loads'], `${path}.loads`),
    continuous: optional(raw['continuous'], `${path}.continuous`, quantity),
    peak: optional(raw['peak'], `${path}.peak`, quantity),
    complete: bool(raw['complete'], `${path}.complete`),
  }
}

function segmentPower(value: unknown, path: string): SegmentPower {
  const raw = record(value, path)
  return {
    model: str(raw['model'], `${path}.model`),
    status: str(raw['status'], `${path}.status`),
    detail: text(raw['detail'], `${path}.detail`),
    evidence: text(raw['evidence'], `${path}.evidence`),
    traces: traces(raw['traces'], `${path}.traces`),
    dynamicPressure: optional(raw['dynamicPressure'], `${path}.dynamicPressure`, quantity),
    lift: optional(raw['lift'], `${path}.lift`, quantity),
    drag: optional(raw['drag'], `${path}.drag`, quantity),
    thrust: optional(raw['thrust'], `${path}.thrust`, quantity),
    propulsive: optional(raw['propulsive'], `${path}.propulsive`, quantity),
    propulsiveElectrical: optional(raw['propulsiveElectrical'], `${path}.propulsiveElectrical`, quantity),
    electrical: optional(raw['electrical'], `${path}.electrical`, quantity),
    liftCoefficient: num(raw['liftCoefficient'], `${path}.liftCoefficient`),
    dragCoefficient: num(raw['dragCoefficient'], `${path}.dragCoefficient`),
    liftToDrag: num(raw['liftToDrag'], `${path}.liftToDrag`),
    chainEfficiency: num(raw['chainEfficiency'], `${path}.chainEfficiency`),
  }
}

function segmentAvailability(value: unknown, path: string): SegmentAvailability {
  const raw = record(value, path)
  return {
    status: str(raw['status'], `${path}.status`),
    capability: text(raw['capability'], `${path}.capability`),
    detail: text(raw['detail'], `${path}.detail`),
    evidence: text(raw['evidence'], `${path}.evidence`),
    availableThrust: optional(raw['availableThrust'], `${path}.availableThrust`, quantity),
    trace: raw['trace'] === undefined || raw['trace'] === null ? null : trace(raw['trace'], `${path}.trace`),
    margin: num(raw['margin'], `${path}.margin`),
  }
}

function segmentResults(value: unknown, path: string): SegmentResult[] {
  if (value === undefined || value === null) return []
  return array(value, path).map((entry, n) => {
    const at = `${path}[${String(n)}]`
    const raw = record(entry, at)
    return {
      name: str(raw['name'], `${at}.name`),
      case: text(raw['case'], `${at}.case`),
      kind: str(raw['kind'], `${at}.kind`),
      status: str(raw['status'], `${at}.status`),
      detail: text(raw['detail'], `${at}.detail`),
      traces: traces(raw['traces'], `${at}.traces`),
      groundSpeed: optional(raw['groundSpeed'], `${at}.groundSpeed`, quantity),
      duration: optional(raw['duration'], `${at}.duration`, quantity),
      distance: optional(raw['distance'], `${at}.distance`, quantity),
      energy: optional(raw['energy'], `${at}.energy`, quantity),
      availability: segmentAvailability(raw['availability'], `${at}.availability`),
      power: segmentPower(raw['power'], `${at}.power`),
    }
  })
}

/**
 * missionResult validates the energy budget. Energy sufficiency and the
 * segments' own status are separate fields and are validated separately,
 * because they are separate answers: a mission whose energy fits is not
 * thereby flyable.
 */
export function missionResult(value: unknown, path: string): MissionResult {
  const raw = record(value, path)
  return {
    status: str(raw['status'], `${path}.status`),
    energyStatus: str(raw['energyStatus'], `${path}.energyStatus`),
    detail: text(raw['detail'], `${path}.detail`),
    segments: segmentResults(raw['segments'], `${path}.segments`),
    traces: traces(raw['traces'], `${path}.traces`),
    requiredEnergy: optional(raw['requiredEnergy'], `${path}.requiredEnergy`, quantity),
    usableEnergy: optional(raw['usableEnergy'], `${path}.usableEnergy`, quantity),
    budget: optional(raw['budget'], `${path}.budget`, quantity),
    totalDuration: optional(raw['totalDuration'], `${path}.totalDuration`, quantity),
    totalDistance: optional(raw['totalDistance'], `${path}.totalDistance`, quantity),
    peakContinuousPower: optional(raw['peakContinuousPower'], `${path}.peakContinuousPower`, quantity),
    reserveFraction: num(raw['reserveFraction'], `${path}.reserveFraction`),
    complete: bool(raw['complete'], `${path}.complete`),
  }
}

function supplyChecks(value: unknown, path: string): SupplyCheck[] {
  if (value === undefined || value === null) return []
  return array(value, path).map((entry, n) => {
    const at = `${path}[${String(n)}]`
    const raw = record(entry, at)
    return {
      name: str(raw['name'], `${at}.name`),
      status: str(raw['status'], `${at}.status`),
      detail: text(raw['detail'], `${at}.detail`),
      limit: optional(raw['limit'], `${at}.limit`, quantity),
      actual: optional(raw['actual'], `${at}.actual`, quantity),
      margin: num(raw['margin'], `${at}.margin`),
    }
  })
}

/** powerFeasibility validates the demand against the component ratings. */
export function powerFeasibility(value: unknown, path: string): PowerFeasibility {
  const raw = record(value, path)
  return {
    status: str(raw['status'], `${path}.status`),
    detail: text(raw['detail'], `${path}.detail`),
    peakDetail: text(raw['peakDetail'], `${path}.peakDetail`),
    checks: supplyChecks(raw['checks'], `${path}.checks`),
    continuousDemand: optional(raw['continuousDemand'], `${path}.continuousDemand`, quantity),
    peakDemand: optional(raw['peakDemand'], `${path}.peakDemand`, quantity),
  }
}

/** thrustChecks validates the thrust-to-weight targets against capability. */
export function thrustChecks(value: unknown, path: string): ThrustCheck[] {
  if (value === undefined || value === null) return []
  return array(value, path).map((entry, n) => {
    const at = `${path}[${String(n)}]`
    const raw = record(entry, at)
    return {
      name: str(raw['name'], `${at}.name`),
      capability: text(raw['capability'], `${at}.capability`),
      condition: text(raw['condition'], `${at}.condition`),
      detail: text(raw['detail'], `${at}.detail`),
      priority: str(raw['priority'], `${at}.priority`),
      evidence: text(raw['evidence'], `${at}.evidence`),
      status: str(raw['status'], `${at}.status`),
      trace: raw['trace'] === undefined || raw['trace'] === null ? null : trace(raw['trace'], `${at}.trace`),
      available: num(raw['available'], `${at}.available`),
      target: num(raw['target'], `${at}.target`),
      margin: num(raw['margin'], `${at}.margin`),
    }
  })
}

function powerSearchCandidates(value: unknown, path: string): PowerSearchCandidate[] {
  return array(value, path).map((entry, n) => {
    const at = `${path}[${String(n)}]`
    const raw = record(entry, at)
    return {
      driver: quantity(raw['driver'], `${at}.driver`),
      demand: optional(raw['demand'], `${at}.demand`, quantity),
      status: str(raw['status'], `${at}.status`),
      feasibility: str(raw['feasibility'], `${at}.feasibility`),
      detail: text(raw['detail'], `${at}.detail`),
      margin: num(raw['margin'], `${at}.margin`),
      hasRequired: bool(raw['hasRequired'], `${at}.hasRequired`),
      withinCeiling: bool(raw['withinCeiling'], `${at}.withinCeiling`),
      feasible: bool(raw['feasible'], `${at}.feasible`),
    }
  })
}

function powerSearchIntervals(value: unknown, path: string): PowerSearchInterval[] {
  return array(value, path).map((entry, n) => {
    const at = `${path}[${String(n)}]`
    const raw = record(entry, at)
    return {
      first: quantity(raw['first'], `${at}.first`),
      last: quantity(raw['last'], `${at}.last`),
      belowFirst: optional(raw['belowFirst'], `${at}.belowFirst`, quantity),
      aboveLast: optional(raw['aboveLast'], `${at}.aboveLast`, quantity),
      detail: str(raw['detail'], `${at}.detail`),
      openLow: bool(raw['openLow'], `${at}.openLow`),
      openHigh: bool(raw['openHigh'], `${at}.openHigh`),
    }
  })
}

/**
 * powerSearch validates a bounded search answer. Like a sweep it carries the
 * snapshot and the settings fingerprint, because a search answers one question
 * over one design and an answer to a different range is not an answer to this
 * one. `found` and `unique` are read rather than derived from the candidate
 * list: they are the service's own statement about whether it found anything
 * and whether the answer is one interval, and recomputing them here would be a
 * second opinion about a question only the core has the standing to settle.
 */
export function powerSearch(value: unknown, path = 'powerSearch'): PowerSearchResponse {
  const raw = record(value, path)
  const request = record(raw['request'], `${path}.request`)
  const settings = record(raw['settings'], `${path}.settings`)
  return {
    request: {
      session: str(request['session'], `${path}.request.session`),
      sequence: num(request['sequence'], `${path}.request.sequence`),
    },
    settings: {
      driver: str(settings['driver'], `${path}.settings.driver`),
      from: quantity(settings['from'], `${path}.settings.from`),
      to: quantity(settings['to'], `${path}.settings.to`),
      ceiling: quantity(settings['ceiling'], `${path}.settings.ceiling`),
      samples: num(settings['samples'], `${path}.settings.samples`),
    },
    settingsFingerprint: str(raw['settingsFingerprint'], `${path}.settingsFingerprint`),
    snapshot: str(raw['snapshot'], `${path}.snapshot`),
    solveMode: str(raw['solveMode'], `${path}.solveMode`),
    detail: str(raw['detail'], `${path}.detail`),
    heldFixed: strings(raw['heldFixed'], `${path}.heldFixed`),
    candidates: powerSearchCandidates(raw['candidates'], `${path}.candidates`),
    intervals: powerSearchIntervals(raw['intervals'], `${path}.intervals`),
    unique: bool(raw['unique'], `${path}.unique`),
    found: bool(raw['found'], `${path}.found`),
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
    massProperties: massProperties(raw['massProperties'], `${path}.massProperties`),
    loads: loads(raw['loads'], `${path}.loads`),
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
    electrical: electricalBudget(raw['electrical'], `${path}.electrical`),
    mission: missionResult(raw['mission'], `${path}.mission`),
    powerFeasibility: powerFeasibility(raw['powerFeasibility'], `${path}.powerFeasibility`),
    thrustChecks: thrustChecks(raw['thrustChecks'], `${path}.thrustChecks`),
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
