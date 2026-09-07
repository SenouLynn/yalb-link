import { beforeEach, describe, expect, test } from 'vitest'
import type { Design, Evaluation, Request as RequestIdentity } from '../api/contract.ts'
import { startingDesign } from './design.ts'
import type { Worksheet } from './worksheet.ts'
import type { SweepResponse, SweepSettings } from '../api/contract.ts'
import { canRedo, canUndo, design, initialWorksheet, reduce, staleResult, staleSweep } from './worksheet.ts'

// A stand-in evaluation. The reducer never reads a number out of one; what it
// decides is whether an answer is still wanted, and that turns on the identity.
function answer(request: RequestIdentity, snapshot = 'snapshot'): Evaluation {
  return {
    request,
    snapshot,
    geometry: 'computed',
    aggregate: 'unmet',
    hasRequired: true,
    wing: null,
    checks: [],
    areaLower: { subject: 'wing-area', direction: 'minimum', known: false, partial: false },
    areaUpper: { subject: 'wing-area', direction: 'maximum', known: false, partial: false },
    mass: {
      lower: { subject: 'mass', direction: 'minimum', known: false, partial: false },
      upper: { subject: 'mass', direction: 'maximum', known: false, partial: false },
      complete: false,
      empty: false,
    },
    massProperties: {
      datum: 'wing root',
      status: 'missing',
      contributions: [],
      total: null,
      cg: null,
      complete: false,
    },
    loads: [],
    conflicts: [],
    patterns: [],
    definitionIssues: [],
    geometryIssues: [],
    configurationIssues: [],
  }
}

function named(name: string): Design {
  return { ...startingDesign(), name }
}

// start returns a worksheet with one accepted result, which is the state most
// of these checks are about.
function start(): { worksheet: Worksheet; identity: RequestIdentity } {
  let worksheet = initialWorksheet('s')
  worksheet = reduce(worksheet, { type: 'request-started', kind: 'evaluate' })
  const identity = { session: 's', sequence: 1 }
  worksheet = reduce(worksheet, { type: 'evaluated', evaluation: answer(identity) })
  return { worksheet, identity }
}

describe('request identity', () => {
  test('a result is accepted only while its request is the outstanding one', () => {
    const { worksheet } = start()
    expect(worksheet.current).not.toBeNull()
    expect(worksheet.pending).toBeNull()

    // A second answer to the same, now-settled request changes nothing.
    const again = reduce(worksheet, {
      type: 'evaluated',
      evaluation: answer({ session: 's', sequence: 1 }),
    })
    expect(again).toBe(worksheet)
  })

  test('an answer to an abandoned request is ignored', () => {
    let worksheet = initialWorksheet('s')
    worksheet = reduce(worksheet, { type: 'request-started', kind: 'evaluate' })
    worksheet = reduce(worksheet, { type: 'request-started', kind: 'evaluate' })
    // Only the later request is outstanding.
    const late = reduce(worksheet, {
      type: 'evaluated',
      evaluation: answer({ session: 's', sequence: 1 }),
    })
    expect(late.current).toBeNull()

    const current = reduce(worksheet, {
      type: 'evaluated',
      evaluation: answer({ session: 's', sequence: 2 }),
    })
    expect(current.current).not.toBeNull()
  })

  test('every design change retires the outstanding identity', () => {
    let worksheet = initialWorksheet('s')
    worksheet = reduce(worksheet, { type: 'request-started', kind: 'apply' })
    const outstanding = { session: 's', sequence: 1 }
    worksheet = reduce(worksheet, {
      type: 'applied',
      design: named('edited'),
      evaluation: answer(outstanding),
    })
    expect(worksheet.pending).toBeNull()
    expect(design(worksheet).name).toBe('edited')

    // The very same answer, arriving again after the design moved on, is not
    // accepted a second time.
    const repeat = reduce(worksheet, {
      type: 'applied',
      design: named('edited twice'),
      evaluation: answer(outstanding),
    })
    expect(design(repeat).name).toBe('edited')
  })

  test('undoing back to the exact design an answer describes does not revive it', () => {
    // This is the case an input comparison cannot catch. The design is edited,
    // an evaluation of the edited design is asked for, and then the edit is
    // undone. The outstanding answer describes a design the worksheet held a
    // moment ago and holds again after a redo — and it is still not wanted,
    // because the request it belonged to was retired by the undo.
    let worksheet = initialWorksheet('s')
    worksheet = reduce(worksheet, { type: 'request-started', kind: 'apply' })
    worksheet = reduce(worksheet, {
      type: 'applied',
      design: named('branch'),
      evaluation: answer({ session: 's', sequence: 1 }),
    })
    expect(design(worksheet).name).toBe('branch')

    worksheet = reduce(worksheet, { type: 'request-started', kind: 'evaluate' })
    const outstanding = { session: 's', sequence: 2 }
    worksheet = reduce(worksheet, { type: 'undo' })
    expect(design(worksheet).name).toBe('New design')
    expect(worksheet.pending).toBeNull()

    const revived = reduce(worksheet, { type: 'evaluated', evaluation: answer(outstanding) })
    expect(revived.current).toBe(worksheet.current)
    expect(revived.pending).toBeNull()

    // Redoing back to the branch does not make it acceptable either.
    const redone = reduce(worksheet, { type: 'redo' })
    expect(design(redone).name).toBe('branch')
    const stillRefused = reduce(redone, { type: 'evaluated', evaluation: answer(outstanding) })
    expect(stillRefused.current).toBe(redone.current)
  })

  test('the sequence only increases, across undo, redo and loading', () => {
    let worksheet = initialWorksheet('s')
    const sequences: number[] = []
    for (const action of [
      { type: 'request-started', kind: 'evaluate' },
      { type: 'undo' },
      { type: 'request-started', kind: 'evaluate' },
      { type: 'redo' },
      { type: 'request-started', kind: 'evaluate' },
      { type: 'draft-loaded', design: named('loaded'), drafts: {}, journey: null },
      { type: 'request-started', kind: 'evaluate' },
    ] as const) {
      worksheet = reduce(worksheet, action)
      if (action.type === 'request-started') sequences.push(worksheet.nextSequence)
    }
    expect(sequences).toEqual([1, 2, 3, 4])
    expect(new Set(sequences).size).toBe(sequences.length)
  })
})

describe('history', () => {
  test('undo restores the earlier design and leaves the later one to redo', () => {
    let worksheet = initialWorksheet('s')
    worksheet = reduce(worksheet, { type: 'request-started', kind: 'apply' })
    worksheet = reduce(worksheet, {
      type: 'applied',
      design: named('second'),
      evaluation: answer({ session: 's', sequence: 1 }),
    })
    expect(canUndo(worksheet)).toBe(true)
    expect(canRedo(worksheet)).toBe(false)

    worksheet = reduce(worksheet, { type: 'undo' })
    expect(design(worksheet).name).toBe('New design')
    expect(canRedo(worksheet)).toBe(true)

    worksheet = reduce(worksheet, { type: 'redo' })
    expect(design(worksheet).name).toBe('second')
  })

  test('a result that describes an earlier revision is reported as one', () => {
    const { worksheet } = start()
    expect(staleResult(worksheet)).toBe(false)
    const moved = reduce(worksheet, { type: 'undo' })
    // There is nothing to undo, so nothing moved.
    expect(staleResult(moved)).toBe(false)

    let edited = reduce(worksheet, { type: 'request-started', kind: 'apply' })
    edited = reduce(edited, {
      type: 'applied',
      design: named('second'),
      evaluation: answer({ session: 's', sequence: 2 }),
    })
    const undone = reduce(edited, { type: 'undo' })
    expect(staleResult(undone)).toBe(true)
  })
})

describe('drafts and failures', () => {
  test('unfinished text is kept apart from the design and survives a failure', () => {
    let worksheet = initialWorksheet('s')
    worksheet = reduce(worksheet, { type: 'draft-changed', field: 'span', text: '1.' })
    worksheet = reduce(worksheet, { type: 'request-started', kind: 'apply' })
    worksheet = reduce(worksheet, {
      type: 'failed',
      failure: { message: 'refused', issues: [], transport: true },
    })
    expect(worksheet.drafts['span']).toBe('1.')
    expect(worksheet.pending).toBeNull()
    expect(worksheet.failure?.transport).toBe(true)
    // A failure changes no design and loses no result.
    expect(design(worksheet).name).toBe('New design')
  })

  test('discarding a draft removes only that field', () => {
    let worksheet = initialWorksheet('s')
    worksheet = reduce(worksheet, { type: 'draft-changed', field: 'span', text: '1.4' })
    worksheet = reduce(worksheet, { type: 'draft-changed', field: 'area', text: '0.2' })
    worksheet = reduce(worksheet, { type: 'draft-discarded', field: 'span' })
    expect(worksheet.drafts).toEqual({ area: '0.2' })
  })

  test('loading a draft restores its text and shows no cached result as current', () => {
    const { worksheet } = start()
    const loaded = reduce(worksheet, {
      type: 'draft-loaded',
      design: named('loaded'),
      drafts: { span: '1.4' },
      journey: 'span-first',
    })
    expect(design(loaded).name).toBe('loaded')
    expect(loaded.drafts).toEqual({ span: '1.4' })
    expect(loaded.journey).toBe('span-first')
    expect(loaded.current).toBeNull()
    expect(loaded.previous).toBeNull()
    // The previous design stays undoable, so loading is not destructive.
    expect(canUndo(loaded)).toBe(true)
  })
})

// The sweep's freshness inside the reducer. A sweep is never committed and
// never joins the history, so what it needs from the reducer is exactly one
// thing: to know whose question a late answer belongs to.

const sweepSettings: SweepSettings = {
  driver: 'wing.aspect_ratio.planform',
  output: { subject: 'stall-speed', case: 'Level flight' },
  from: { value: 4, unit: '1' },
  to: { value: 10, unit: '1' },
  samples: 5,
}

function sweepAnswer(sequence: number, snapshot: string): SweepResponse {
  return {
    request: { session: 'test', sequence },
    settings: sweepSettings,
    settingsFingerprint: 'service-side',
    snapshot,
    solveMode: 'span-and-aspect-ratio',
    detail: 'held fixed: the span',
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

describe('sweeps', () => {
  let worksheet: Worksheet

  beforeEach(() => {
    worksheet = initialWorksheet('sweeps')
  })

  test('an answer to the outstanding sweep is shown', () => {
    worksheet = reduce(worksheet, { type: 'sweep-started', settings: sweepSettings, sequence: 1 })
    worksheet = reduce(worksheet, { type: 'swept', response: sweepAnswer(1, 'inputs'), snapshot: 'inputs' })
    expect(worksheet.swept?.response.request.sequence).toBe(1)
    expect(worksheet.sweeping).toBeNull()
    expect(staleSweep(worksheet)).toBe(false)
  })

  test('a design change retires the outstanding sweep, and undo does not restore it', () => {
    worksheet = reduce(worksheet, { type: 'sweep-started', settings: sweepSettings, sequence: 1 })
    // An edit while the sweep is in flight.
    worksheet = reduce(worksheet, { type: 'request-started', kind: 'apply' })
    worksheet = reduce(worksheet, {
      type: 'applied',
      design: { ...startingDesign(), name: 'edited' },
      evaluation: answer(worksheet.pending?.identity ?? { session: 'sweeps', sequence: 0 }),
    })
    worksheet = reduce(worksheet, { type: 'undo' })

    // The design is back where the sweep was asked about, and the late answer
    // is still not wanted: the identity was retired by the edit and nothing
    // restores it.
    worksheet = reduce(worksheet, { type: 'swept', response: sweepAnswer(1, 'inputs'), snapshot: 'inputs' })
    expect(worksheet.swept).toBeNull()
  })

  test('a plotted curve is marked as describing an earlier revision rather than dropped', () => {
    worksheet = reduce(worksheet, { type: 'sweep-started', settings: sweepSettings, sequence: 1 })
    worksheet = reduce(worksheet, { type: 'swept', response: sweepAnswer(1, 'inputs'), snapshot: 'inputs' })
    worksheet = reduce(worksheet, { type: 'undo' })
    // With no earlier revision the undo does nothing, so make a real change.
    worksheet = reduce(worksheet, { type: 'request-started', kind: 'apply' })
    worksheet = reduce(worksheet, {
      type: 'applied',
      design: { ...startingDesign(), name: 'edited' },
      evaluation: answer(worksheet.pending?.identity ?? { session: 'sweeps', sequence: 0 }),
    })
    expect(worksheet.swept).not.toBeNull()
    expect(staleSweep(worksheet)).toBe(true)
  })

  test('loading a draft drops the curve rather than marking it stale', () => {
    worksheet = reduce(worksheet, { type: 'sweep-started', settings: sweepSettings, sequence: 1 })
    worksheet = reduce(worksheet, { type: 'swept', response: sweepAnswer(1, 'inputs'), snapshot: 'inputs' })
    worksheet = reduce(worksheet, {
      type: 'draft-loaded',
      design: startingDesign(),
      drafts: {},
      journey: null,
    })
    // A loaded draft is a different design, so a curve plotted for the previous
    // one is not an earlier revision of it.
    expect(worksheet.swept).toBeNull()
  })

  test('every request kind advances the one identity counter', () => {
    // The hook mints identities from its own counter and the reducer keeps a
    // matching one. A kind that took an identity without moving the reducer's
    // counter would put the two out of step, and every answer after it would be
    // discarded as somebody else's.
    worksheet = reduce(worksheet, { type: 'request-started', kind: 'evaluate' })
    expect(worksheet.nextSequence).toBe(1)
    worksheet = reduce(worksheet, { type: 'sweep-started', settings: sweepSettings, sequence: 2 })
    expect(worksheet.nextSequence).toBe(2)
    worksheet = reduce(worksheet, { type: 'preview-started', sequence: 3 })
    expect(worksheet.nextSequence).toBe(3)
    worksheet = reduce(worksheet, { type: 'request-started', kind: 'apply' })
    expect(worksheet.pending?.identity.sequence).toBe(4)
  })

  test('selecting a parameter is remembered, and selecting nothing clears it', () => {
    worksheet = reduce(worksheet, { type: 'parameter-selected', key: 'wing.chord.root' })
    expect(worksheet.selection).toBe('wing.chord.root')
    worksheet = reduce(worksheet, { type: 'parameter-selected', key: null })
    expect(worksheet.selection).toBeNull()
  })
})
