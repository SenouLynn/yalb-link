// Helpers over the wire design. Nothing here computes a physical value: the
// worksheet never edits a Design in place, it sends a command and receives the
// edited design back, so these are only for reading what the design says.

import type { Case, Design, Quantity, Requirement, Wing } from '../api/contract.ts'

/** The size drivers a planform can hold, named with the core's own keys. */
export const DRIVER_KEYS = {
  span: 'wing.span.projected',
  spanPanel: 'wing.span.panel',
  area: 'wing.area.reference',
  areaPanel: 'wing.area.panel',
  aspectRatio: 'wing.aspect_ratio.planform',
  rootChord: 'wing.chord.root',
} as const

export type DriverKey = (typeof DRIVER_KEYS)[keyof typeof DRIVER_KEYS]

/** JourneyId names an entry point into the same definition. */
export type JourneyId =
  | 'span-first'
  | 'mass-and-performance-first'
  | 'mass-and-size-first'
  | 'existing-design'
  | 'power-first'
  | 'mission-and-energy'

export interface JourneyDescription {
  readonly id: JourneyId
  readonly title: string
  readonly startsFrom: string
  readonly next: string
}

/**
 * JOURNEYS is what the entry choices say to a builder, in their language rather
 * than in the model's. The explanations differ per journey because the next
 * useful action does; none of them is a wizard, and every one edits the same
 * definition.
 */
export const JOURNEYS: readonly JourneyDescription[] = [
  {
    id: 'span-first',
    title: 'Span first',
    startsFrom: 'A span you have to fit — a doorway, a car boot, a contest rule.',
    next: 'Enter the span you have chosen to use, then an aspect ratio or a chord. '
      + 'A maximum span is a requirement, not a span: use “Use maximum” to adopt it.',
  },
  {
    id: 'mass-and-performance-first',
    title: 'Mass and performance first',
    startsFrom: 'A known all-up mass and a stall speed you need to hold.',
    next: 'Enter the mass and a stall-speed ceiling in each required case, then size '
      + 'the wing at that limit while holding the driver you care about.',
  },
  {
    id: 'mass-and-size-first',
    title: 'Mass and available size first',
    startsFrom: 'A wing you can already build or buy, and a mass to carry.',
    next: 'Enter the two size values that describe the wing and read the stall speed '
      + 'and the mass ceiling each required case allows.',
  },
  {
    id: 'existing-design',
    title: 'Existing design',
    startsFrom: 'An aircraft that already exists on paper or in the air.',
    next: 'Enter the whole definition, add the requirements it is judged against, '
      + 'and change one driver at a time to see what moves.',
  },
  {
    id: 'power-first',
    title: 'Power ceiling first',
    startsFrom: 'A mass you have to carry and an electrical power you must not exceed.',
    next: 'Enter the mass, the drag polar, the chain efficiency and the avionics draw, '
      + 'write the mission, then search a range of wings against the ceiling. A mass and '
      + 'a ceiling do not determine a wing: the answer is an interval, and choosing from '
      + 'it is your edit.',
  },
  {
    id: 'mission-and-energy',
    title: 'Mission and energy',
    startsFrom: 'An aircraft that exists, and the question of what it can do on one pack.',
    next: 'Enter the polar, the pack and the auxiliary draw, then write the mission leg '
      + 'by leg, return included. Energy sufficiency and flight feasibility are reported '
      + 'separately, because they are separate answers.',
  },
]

export function journey(id: JourneyId): JourneyDescription {
  const found = JOURNEYS.find((entry) => entry.id === id)
  if (!found) throw new Error(`no journey named ${id}`)
  return found
}

const ZERO_DEGREES: Quantity = { value: 0, unit: 'deg' }

/**
 * startingDesign is the definition a journey begins from. Every angle is stated
 * explicitly as zero, because the geometry model treats an unstated angle as a
 * missing field rather than as zero, and a later handling model cannot tell an
 * unswept wing from an unrecorded one.
 *
 * Nothing here is a default in the physical sense: the mass, the coefficient
 * and the requirements are all left for the builder, and the flight case
 * carries the ISA sea-level density with that stated as its basis.
 */
export function startingDesign(): Design {
  return {
    name: 'New design',
    configuration: 'conventional-tail',
    massBasis: '',
    mass: null,
    wing: {
      name: 'Main wing',
      shape: 'rectangle',
      span: null,
      area: null,
      rootChord: null,
      aspectRatio: 0,
      taperRatio: 0,
      sweep: ZERO_DEGREES,
      dihedral: ZERO_DEGREES,
      twist: ZERO_DEGREES,
      incidence: ZERO_DEGREES,
      bodyWidth: null,
      sweepReference: 0.25,
      areaBasis: 'reference-trapezoid',
      dihedralMode: '',
      rootAirfoil: null,
      tipAirfoil: null,
    },
    tail: null,
    cases: [levelCase()],
    requirements: [stallCeiling()],
  }
}

/** levelCase is the level-flight case every journey starts with. */
export function levelCase(): Case {
  return {
    name: 'Level flight',
    configuration: 'clean',
    densityBasis: 'ISA sea level',
    density: { value: 1.225, unit: 'kg/m^3' },
    viscosityBasis: '',
    viscosity: null,
    loadFactor: 1,
    priority: 'required',
    clmax: { max: 0, scope: 'aircraft', basis: '', evidence: 'assumed' },
  }
}

/** stallCeiling is the requirement the sizing action reads. */
export function stallCeiling(caseName = 'Level flight'): Requirement {
  return {
    name: 'Stall ceiling',
    subject: 'stall-speed',
    priority: 'required',
    basis: '',
    cases: [caseName],
    minimum: null,
    maximum: null,
    margin: 0,
  }
}

/** maximumSpan is the requirement a span limit is entered as. It is never a span. */
export function maximumSpan(): Requirement {
  return {
    name: 'Maximum span',
    subject: 'span',
    priority: 'required',
    basis: '',
    cases: [],
    minimum: null,
    maximum: null,
    margin: 0,
  }
}

/** maximumArea is the requirement a sheet-stock or storage limit is entered as. */
export function maximumArea(): Requirement {
  return {
    name: 'Maximum wing area',
    subject: 'wing-area',
    priority: 'required',
    basis: '',
    cases: [],
    minimum: null,
    maximum: null,
    margin: 0,
  }
}

/** driverOf reads a size driver's value, or null when it is not a driver. */
export function driverOf(wing: Wing, key: DriverKey): Quantity | null {
  switch (key) {
    case DRIVER_KEYS.span:
    case DRIVER_KEYS.spanPanel:
      return wing.span ?? null
    case DRIVER_KEYS.area:
    case DRIVER_KEYS.areaPanel:
      return wing.area ?? null
    case DRIVER_KEYS.rootChord:
      return wing.rootChord ?? null
    case DRIVER_KEYS.aspectRatio:
      return wing.aspectRatio === 0 ? null : { value: wing.aspectRatio, unit: '1' }
    default:
      return null
  }
}

/** requirementNamed finds a requirement by name. */
export function requirementNamed(design: Design, name: string): Requirement | null {
  return design.requirements?.find((requirement) => requirement.name === name) ?? null
}

/** caseNamed finds a flight case by name. */
export function caseNamed(design: Design, name: string): Case | null {
  return design.cases?.find((entry) => entry.name === name) ?? null
}
