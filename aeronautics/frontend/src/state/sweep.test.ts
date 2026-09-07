import { describe, expect, test } from 'vitest'
import type { Design, Quantity, SweepResponse, SweepSettings } from '../api/contract.ts'
import { startingDesign } from './design.ts'
import { answers, defaultRange, driverValue, settingsKey, sweepableDrivers } from './sweep.ts'
import type { Sweeping } from './sweep.ts'

// The sweep's own freshness rules, isolated from the transport.
//
// A sweep answer belongs to a question, and the question has three parts: which
// request asked it, which design it was asked about, and what it asked for.

const settings: SweepSettings = {
  driver: 'wing.aspect_ratio.planform',
  output: { subject: 'stall-speed', case: 'Level flight' },
  from: { value: 4, unit: '1' },
  to: { value: 10, unit: '1' },
  samples: 5,
}

function asked(sequence: number, revision = 0): Sweeping {
  return { settings, sequence, revision }
}

function response(sequence: number, snapshot = 'inputs', over = settings): SweepResponse {
  return {
    request: { session: 'test', sequence },
    settings: over,
    settingsFingerprint: 'service-side',
    snapshot,
    solveMode: 'span-and-aspect-ratio',
    detail: 'the planform solves from span and aspect ratio',
    heldFixed: ['wing.span.projected'],
    alsoChanged: [],
    bounds: [],
    samples: [],
    current: {
      driver: { value: 6, unit: '1' },
      value: { value: 10.5, unit: 'm/s' },
      status: 'computed',
      feasibility: 'met',
      trace: null,
      hasRequired: true,
    },
    invariant: false,
  }
}

describe('sweep freshness', () => {
  test('an answer to the outstanding request over the current design is accepted', () => {
    expect(answers(response(3), asked(3), 'inputs', settings)).toBe(true)
  })

  test('an answer to a superseded request is not', () => {
    expect(answers(response(2), asked(3), 'inputs', settings)).toBe(false)
  })

  test('an answer computed from other inputs is not', () => {
    expect(answers(response(3, 'other inputs'), asked(3), 'inputs', settings)).toBe(false)
  })

  test('a different range is a different question', () => {
    const wider: SweepSettings = { ...settings, to: { value: 12, unit: '1' } }
    expect(answers(response(3, 'inputs', wider), asked(3), 'inputs', settings)).toBe(false)
    expect(settingsKey(wider)).not.toBe(settingsKey(settings))
  })

  test('a different plotted output is a different question', () => {
    const other: SweepSettings = { ...settings, output: { subject: 'wing-area', case: '' } }
    expect(answers(response(3, 'inputs', other), asked(3), 'inputs', settings)).toBe(false)
  })

  test('the same range in a different unit is a different question', () => {
    // 4 to 10 is not 4 to 10 when one is metres and the other is centimetres,
    // and the fingerprint says so rather than comparing bare numbers.
    const centimetres: SweepSettings = {
      ...settings,
      from: { value: 4, unit: 'cm' },
      to: { value: 10, unit: 'cm' },
    }
    expect(settingsKey(centimetres)).not.toBe(settingsKey(settings))
  })
})

describe('what a design can sweep', () => {
  function withMass(design: Design, mass: Quantity | null): Design {
    return { ...design, mass }
  }

  test('the entered mass is sweepable, and the component total is not', () => {
    const design = withMass(startingDesign(), { value: 2, unit: 'kg' })
    expect(sweepableDrivers(design, ['wing.span.projected'])).toEqual([
      'design.mass',
      'wing.span.projected',
    ])
    const fromComponents: Design = { ...design, massMode: 'components' }
    // A mass that follows from an inventory is a result, not a driver: sweeping
    // it would be sweeping something the builder does not set.
    expect(sweepableDrivers(fromComponents, ['wing.span.projected'])).toEqual([
      'wing.span.projected',
    ])
  })

  test('a design with no mass offers only its size drivers', () => {
    expect(sweepableDrivers(withMass(startingDesign(), null), ['wing.area.reference']))
      .toEqual(['wing.area.reference'])
  })

  test('a driver value is read in the unit the design holds it in', () => {
    const design = withMass(startingDesign(), { value: 4.4, unit: 'lb' })
    expect(driverValue(design, 'design.mass')).toEqual({ value: 4.4, unit: 'lb' })
    expect(driverValue(design, 'wing.chord.mac')).toBeNull()
  })

  test('a default range brackets the value the design holds, in its own unit', () => {
    const range = defaultRange({ value: 10, unit: 'm' })
    expect(range.from.unit).toBe('m')
    expect(range.from.value).toBeLessThan(10)
    expect(range.to.value).toBeGreaterThan(10)
  })
})
