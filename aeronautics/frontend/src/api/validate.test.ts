import { describe, expect, test } from 'vitest'
import { ResponseShapeError, discovery, evaluation, failure, units } from './validate.ts'
import { unpowered } from '../testing/fixtures.ts'

// A response is untrusted data. These checks are what stands between a body
// that is not the shape this worksheet expects and a page that renders "NaN" or
// silently drops a requirement.

const good = {
  request: { session: 's', sequence: 1 },
  snapshot: 'snapshot',
  geometry: 'computed',
  aggregate: 'unmet',
  hasRequired: true,
  wing: null,
  checks: [
    {
      name: 'Stall ceiling',
      case: 'Level flight',
      subject: 'stall-speed',
      direction: 'maximum',
      priority: 'required',
      status: 'unmet',
      result: 'computed',
      bound: { value: 8, unit: 'm/s' },
      actual: { value: 10.5, unit: 'm/s' },
      margin: -0.3,
    },
  ],
  areaLower: { subject: 'wing-area', direction: 'minimum', known: true, partial: false, value: { value: 0.4, unit: 'm^2' } },
  areaUpper: { subject: 'wing-area', direction: 'maximum', known: false, partial: false },
  mass: {
    lower: { subject: 'mass', direction: 'minimum', known: false, partial: false },
    upper: { subject: 'mass', direction: 'maximum', known: false, partial: false },
    complete: false,
    empty: false,
  },
  massProperties: {
    datum: 'origin at the root leading edge',
    status: 'missing',
    detail: 'no component masses are listed',
    contributions: [],
    complete: false,
  },
  loads: [],
  conflicts: [],
  patterns: ['workflow.span-first'],
  definitionIssues: [],
  geometryIssues: [],
  configurationIssues: [],
  // The power half is a design that states no propulsion. It comes from the
  // shared fixture so that a contract change breaks it here too, rather than
  // leaving this one describing a response the service no longer sends.
  ...unpowered(),
}

describe('evaluations', () => {
  test('a well-formed response passes through with its numbers intact', () => {
    const checked = evaluation(good)
    expect(checked.checks[0]?.actual?.value).toBe(10.5)
    expect(checked.areaLower.value?.unit).toBe('m^2')
    expect(checked.patterns).toEqual(['workflow.span-first'])
  })

  test('a missing field is refused rather than read as undefined', () => {
    const withoutSnapshot: Record<string, unknown> = { ...good }
    delete withoutSnapshot['snapshot']
    expect(() => evaluation(withoutSnapshot)).toThrow(ResponseShapeError)
  })

  test('a number that is not one is refused, and so is a NaN', () => {
    expect(() =>
      evaluation({ ...good, checks: [{ ...good.checks[0], margin: 'a lot' }] }),
    ).toThrow(ResponseShapeError)
    expect(() =>
      evaluation({
        ...good,
        checks: [{ ...good.checks[0], actual: { value: Number.NaN, unit: 'm/s' } }],
      }),
    ).toThrow(ResponseShapeError)
  })

  test('a status that arrives as something other than text is refused', () => {
    expect(() => evaluation({ ...good, aggregate: 3 })).toThrow(ResponseShapeError)
    expect(() => evaluation({ ...good, checks: 'none' })).toThrow(ResponseShapeError)
  })

  test('the refusal names the field it is about', () => {
    try {
      evaluation({ ...good, hasRequired: 'yes' })
      expect.unreachable('a wrong type must be refused')
    } catch (error) {
      expect((error as Error).message).toContain('evaluation.hasRequired')
    }
  })
})

describe('the unit table', () => {
  test('a unit without a usable factor is refused', () => {
    // A wrong factor would silently misreport every number on the page, so it
    // is checked like any other field rather than trusted because it is small.
    expect(() =>
      units({ units: [{ symbol: 'm', dimension: 'length', factorToSi: 0, si: true }] }),
    ).toThrow(ResponseShapeError)
    expect(() =>
      units({ units: [{ symbol: 'm', dimension: 'length', factorToSi: -1, si: true }] }),
    ).toThrow(ResponseShapeError)
    expect(
      units({ units: [{ symbol: 'ft', dimension: 'length', factorToSi: 0.3048, si: false }] }),
    ).toEqual([{ symbol: 'ft', dimension: 'length', factorToSi: 0.3048, si: false }])
  })
})

describe('discovery and failures', () => {
  test('discovery keeps the equation revisions a saved draft is checked against', () => {
    const checked = discovery({
      contractVersion: 'v1',
      version: '0.0.0',
      equations: [{ id: 'lift.stall-speed', revision: '1' }],
    })
    expect(checked.equationRevisions).toEqual({ 'lift.stall-speed': '1' })
  })

  test('a failure body keeps its field issues', () => {
    const checked = failure({
      error: 'invalid',
      message: 'the request could not be read',
      issues: [{ field: 'design.mass', kind: 'missing', detail: 'supply the all-up mass' }],
    })
    expect(checked.issues[0]?.field).toBe('design.mass')
  })

  test('a failure body that is not one is refused', () => {
    expect(() => failure({ error: 'invalid' })).toThrow(ResponseShapeError)
  })
})
