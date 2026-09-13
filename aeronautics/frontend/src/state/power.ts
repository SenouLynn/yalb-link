// The power definition's own state: what a new entry starts as, and the
// bounded search's freshness rules.
//
// Nothing here is a physical default. Every template below is deliberately
// empty apart from the identity a command needs — a name, and the kind that
// says what sort of thing it is — because a CD0, a chain efficiency or a
// reserve that the worksheet supplied would be a number with nothing behind
// it, and the service would then report it as evidence the builder gave.
// Missing values come back as missing, which is the answer.

import type {
  AuxiliaryLoad,
  Battery,
  Capability,
  Design,
  DragPolar,
  Mission,
  MissionSegment,
  PowerSearchResponse,
  PowerSearchSettings,
  Propulsion,
} from '../api/contract.ts'

/** SEGMENT_KINDS are the legs an RC mission is written in. */
export const SEGMENT_KINDS = [
  { id: 'launch', label: 'Launch' },
  { id: 'climb', label: 'Climb' },
  { id: 'cruise', label: 'Cruise' },
  { id: 'loiter', label: 'Loiter' },
  { id: 'return', label: 'Return' },
  { id: 'recovery', label: 'Recovery' },
  { id: 'other', label: 'Other' },
] as const

/** EVIDENCE_GRADES is how good a stated value is, in the core's own words. */
export const EVIDENCE_GRADES = ['assumed', 'measured', 'simulated'] as const

/** emptyPolar is a polar with nothing in it but its aircraft-level scope. */
export function emptyPolar(): DragPolar {
  return {
    cd0: 0,
    cd0Basis: '',
    oswaldEfficiency: 0,
    efficiencyBasis: '',
    configuration: 'clean',
    clValidMin: 0,
    clValidMax: 0,
    scope: 'aircraft',
    evidence: 'assumed',
  }
}

/** emptyBattery is a pack with no capacity, so nothing is claimed about it. */
export function emptyBattery(): Battery {
  return {
    basis: '',
    component: '',
    mode: 'capacity-and-voltage',
    capacity: null,
    nominalVoltage: null,
    energy: null,
    usableFraction: 0,
    continuousCurrentLimit: null,
    peakCurrentLimit: null,
    evidence: 'assumed',
  }
}

/** emptyCapability is one named operating point with nothing measured yet. */
export function emptyCapability(name: string, kind: 'static' | 'in-flight'): Capability {
  return {
    name,
    basis: '',
    kind,
    // A static point is at zero speed by definition, and that is a statement
    // about the point rather than a default: it is what makes it static.
    speed: kind === 'static' ? { value: 0, unit: 'm/s' } : null,
    density: null,
    densityBasis: '',
    voltage: null,
    rpm: null,
    throttle: 0,
    thrust: null,
    electricalPower: null,
    current: null,
    evidence: 'assumed',
  }
}

/** emptyLoad is one named electrical draw with neither figure stated. */
export function emptyLoad(name: string): AuxiliaryLoad {
  return {
    name,
    basis: '',
    component: '',
    continuous: null,
    peak: null,
    regulatorEfficiency: 0,
    side: 'pack-side',
    evidence: 'assumed',
  }
}

/**
 * emptySegment is one named leg. The model and the timing are stated because
 * they say what kind of answer the segment can give at all — a leg computed
 * from the polar and one whose power was typed in are different evidence — and
 * a leg with neither would be a segment nothing could report on.
 */
export function emptySegment(name: string, kind: string, caseName: string): MissionSegment {
  return {
    name,
    case: caseName,
    kind,
    model: 'drag-polar',
    timing: 'duration',
    speed: null,
    climbAngle: { value: 0, unit: 'deg' },
    windAlongTrack: { value: 0, unit: 'm/s' },
    duration: null,
    distance: null,
    enteredPower: null,
    enteredBasis: '',
    enteredEvidence: 'assumed',
    efficiency: 0,
    efficiencyBasis: '',
    capability: '',
    notes: '',
  }
}

/** emptyMission is a profile with no reserve and no legs. */
export function emptyMission(): Mission {
  return { name: 'Mission', basis: '', reserveFraction: 0, reserveBasis: '', segments: [] }
}

/** polarOf reads the design's polar, or an empty one to edit into. */
export function polarOf(design: Design): DragPolar {
  return design.polar ?? emptyPolar()
}

/** propulsionOf reads the propulsion definition, or an empty one. */
export function propulsionOf(design: Design): Propulsion {
  return design.propulsion ?? { efficiency: 0, efficiencyBasis: '', efficiencyEvidence: 'assumed' }
}

/** batteryOf reads the flight pack, or an empty one to edit into. */
export function batteryOf(design: Design): Battery {
  return design.battery ?? emptyBattery()
}

/** missionOf reads the mission, or an empty profile. */
export function missionOf(design: Design): Mission {
  return design.mission ?? emptyMission()
}

/**
 * SEARCH_DEFAULT_SAMPLES is a readable scan that stays cheap. Every candidate
 * is a whole evaluation, so this is a real cost rather than a plotted point.
 */
export const SEARCH_DEFAULT_SAMPLES = 13

/** Searching is an outstanding power search. */
export interface Searching {
  readonly settings: PowerSearchSettings
  readonly sequence: number
  /** revision is the design revision the request was issued against. */
  readonly revision: number
}

/** Searched is an accepted search answer and the question it answers. */
export interface Searched {
  readonly response: PowerSearchResponse
  readonly settings: PowerSearchSettings
  /** revision is the design revision the answer describes. */
  readonly revision: number
}

/**
 * searchKey renders the settings so two client-side questions can be compared.
 * Like the sweep's key it is not the service's fingerprint; it is used to check
 * that the answer the service returns describes the settings that were sent.
 */
export function searchKey(settings: PowerSearchSettings): string {
  const { driver, from, to, ceiling, samples } = settings
  return [
    driver,
    `${String(from.value)}${from.unit}`,
    `${String(to.value)}${to.unit}`,
    `${String(ceiling.value)}${ceiling.unit}`,
    String(samples),
  ].join('|')
}

/**
 * answersSearch reports whether a response answers the question now being
 * asked. It is the sweep's rule exactly: the outstanding identity, the design
 * the answer was computed over, and the settings it answers must all agree, so
 * a search asked before an edit cannot reappear as current after one.
 */
export function answersSearch(
  response: PowerSearchResponse,
  asked: Searching,
  snapshot: string,
  settings: PowerSearchSettings,
): boolean {
  if (response.request.sequence !== asked.sequence) return false
  if (response.snapshot !== snapshot) return false
  return searchKey(response.settings) === searchKey(settings)
}

/**
 * searchableDrivers lists what a power search may move: the size drivers the
 * design currently holds. The all-up mass is not among them, unlike a sweep's:
 * moving the mass changes what the aircraft is rather than how big its wing is,
 * and the power-first journey starts from a mass the builder already knows.
 */
export function searchableDrivers(held: readonly string[]): string[] {
  return [...held]
}
