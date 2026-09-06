import { describe, expect, test } from 'vitest'
import type { Design, Evaluation, Request as RequestIdentity } from '../api/contract.ts'
import { startingDesign } from './design.ts'
import type { Worksheet } from './worksheet.ts'
import { canRedo, canUndo, design, initialWorksheet, reduce, staleResult } from './worksheet.ts'

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
