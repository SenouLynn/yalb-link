// The sensitivity sweep's own state.
//
// A sweep is a question about candidates the design does not hold, so it never
// commits anything and never joins the design history. What it does need is the
// same freshness discipline every other answer has, and one more clause: a
// sweep answer belongs to a *question*, so it is matched against the settings
// as well as against the design it was computed from. Changing the range or the
// plotted output makes an outstanding answer someone else's.

import type { Design, Quantity, SweepResponse, SweepSettings } from '../api/contract.ts'

/** SWEEP_DRIVER_KEYS are the values a sweep may move, named with the core's keys. */
export const SWEEP_DRIVER_KEYS = {
  mass: 'design.mass',
  span: 'wing.span.projected',
  spanPanel: 'wing.span.panel',
  area: 'wing.area.reference',
  areaPanel: 'wing.area.panel',
  aspectRatio: 'wing.aspect_ratio.planform',
  rootChord: 'wing.chord.root',
} as const

/** DEFAULT_SAMPLES is a readable curve that is cheap to evaluate. */
export const DEFAULT_SAMPLES = 17

/**
 * fingerprint renders the settings the way the service does, so an answer can
 * be matched against the question without comparing fields one at a time.
 *
 * It is deliberately not the service's own string: it is a local key used only
 * to compare two client-side settings objects. The authoritative match is the
 * fingerprint the service returns, which `answers` below checks against the
 * settings that were actually sent.
 */
export function settingsKey(settings: SweepSettings): string {
  const { driver, from, to, samples, output } = settings
  return [
    driver,
    `${String(from.value)}${from.unit}`,
    `${String(to.value)}${to.unit}`,
    String(samples),
    output.subject,
    output.case,
  ].join('|')
}

/** Sweeping is an outstanding sweep request. */
export interface Sweeping {
  readonly settings: SweepSettings
  readonly sequence: number
  /** revision is the design revision the request was issued against. */
  readonly revision: number
}

/** Swept is an accepted sweep answer and what it answers. */
export interface Swept {
  readonly response: SweepResponse
  readonly settings: SweepSettings
  /** revision is the design revision the answer describes. */
  readonly revision: number
}

/**
 * answers reports whether a response is the answer to the question now being
 * asked. Three things must agree: the outstanding request identity, the design
 * the answer was computed from, and the settings it answers.
 *
 * The identity clause is what keeps an obsolete curve obsolete. Every design
 * change retires the outstanding request — undo and redo included — so a sweep
 * asked for before an edit is refused afterwards even when the history walks
 * back to the exact design it was computed from. Matching on the inputs alone
 * would let it reappear as current on a branch the builder had already left.
 *
 * The settings clause is the sweep's own: a different range or a different
 * plotted output is a different question, and an answer to one is not an answer
 * to the other even over identical inputs.
 */
export function answers(
  response: SweepResponse,
  asked: Sweeping,
  snapshot: string,
  settings: SweepSettings,
): boolean {
  if (response.request.sequence !== asked.sequence) return false
  if (response.snapshot !== snapshot) return false
  return settingsKey(response.settings) === settingsKey(settings)
}

/** driverValue reads a sweepable driver's current value from the design. */
export function driverValue(design: Design, key: string): Quantity | null {
  switch (key) {
    case SWEEP_DRIVER_KEYS.mass:
      return design.mass ?? null
    case SWEEP_DRIVER_KEYS.span:
    case SWEEP_DRIVER_KEYS.spanPanel:
      return design.wing.span ?? null
    case SWEEP_DRIVER_KEYS.area:
    case SWEEP_DRIVER_KEYS.areaPanel:
      return design.wing.area ?? null
    case SWEEP_DRIVER_KEYS.aspectRatio:
      return design.wing.aspectRatio === 0 ? null : { value: design.wing.aspectRatio, unit: '1' }
    case SWEEP_DRIVER_KEYS.rootChord:
      return design.wing.rootChord ?? null
    default:
      return null
  }
}

/**
 * sweepableDrivers lists what this design can actually sweep: the size drivers
 * it currently holds, plus the entered all-up mass when that is where the mass
 * comes from. A derived value is left out rather than offered and then refused,
 * because promoting it releases another driver and is the builder's decision.
 */
export function sweepableDrivers(design: Design, held: readonly string[]): string[] {
  const drivers = [...held]
  if ((design.massMode ?? 'entered') !== 'components' && design.mass) {
    drivers.unshift(SWEEP_DRIVER_KEYS.mass)
  }
  return drivers
}

/**
 * defaultRange is a range around the value the design already holds. It is a
 * starting point for the controls, not a recommendation: both ends are shown
 * and editable, and nothing is swept until the builder asks for it.
 */
export function defaultRange(value: Quantity): { from: Quantity; to: Quantity } {
  return {
    from: { value: value.value * 0.6, unit: value.unit },
    to: { value: value.value * 1.6, unit: value.unit },
  }
}
