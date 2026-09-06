import { describe, expect, test } from 'vitest'
import { CONTRACT_VERSION } from '../api/contract.ts'
import { memoryStorage } from '../testing/storage.ts'
import { startingDesign } from './design.ts'
import {
  LoadFailure,
  MIGRATIONS,
  SCHEMA_VERSION,
  buildSnapshot,
  loadNamed,
  modelChanged,
  parseSnapshot,
  saveNamed,
  savedNames,
} from './drafts.ts'

const context = {
  contractVersion: CONTRACT_VERSION,
  modelVersion: '0.0.0',
  equationRevisions: { 'lift.stall-speed': '1', 'geometry.aspect-ratio': '1' },
}

function snapshot(): ReturnType<typeof buildSnapshot> {
  const design = { ...startingDesign(), name: 'Trainer' }
  return buildSnapshot(design, { span: '1.' }, 'span-first', context, '2026-09-06T00:00:00Z')
}

describe('saving and reopening', () => {
  test('a draft round-trips its inputs, roles, requirements and unfinished text', () => {
    const storage = memoryStorage()
    saveNamed(storage, 'trainer', snapshot())
    expect(savedNames(storage)).toEqual(['trainer'])

    const loaded = loadNamed(storage, 'trainer', CONTRACT_VERSION)
    expect(loaded.schema).toBe(SCHEMA_VERSION)
    expect(loaded.design.name).toBe('Trainer')
    expect(loaded.design.wing.sweep).toEqual({ value: 0, unit: 'deg' })
    expect(loaded.design.requirements?.[0]?.subject).toBe('stall-speed')
    // Half-typed text survives separately from the committed values.
    expect(loaded.drafts).toEqual({ span: '1.' })
    expect(loaded.journey).toBe('span-first')
  })

  test('an incomplete draft saves and reopens like any other', () => {
    // A design with no mass, no coefficient and no bound is exactly what a
    // half-finished afternoon looks like. Refusing to save it would lose it.
    const storage = memoryStorage()
    const incomplete = buildSnapshot(startingDesign(), {}, null, context, 'now')
    saveNamed(storage, 'half done', incomplete)
    const loaded = loadNamed(storage, 'half done', CONTRACT_VERSION)
    expect(loaded.design.mass).toBeNull()
    expect(loaded.journey).toBeNull()
  })

  test('a draft records the equations behind it rather than the numbers', () => {
    const stored = snapshot()
    expect(stored.equationRevisions).toEqual(context.equationRevisions)
    expect(JSON.stringify(stored)).not.toContain('stallSpeed')
    expect(Object.keys(stored)).not.toContain('evaluation')
  })
})

describe('refusing what cannot be read', () => {
  test('a malformed file loads nothing and says so', () => {
    for (const text of ['not json', '[]', '"a string"', '{}']) {
      let failure: unknown
      try {
        parseSnapshot(text, CONTRACT_VERSION)
      } catch (error) {
        failure = error
      }
      expect(failure, text).toBeInstanceOf(LoadFailure)
      expect((failure as LoadFailure).message).toContain('nothing was loaded')
    }
  })

  test('an unsupported schema version is refused rather than guessed at', () => {
    const future = JSON.stringify({ ...snapshot(), schema: SCHEMA_VERSION + 1 })
    expect(() => parseSnapshot(future, CONTRACT_VERSION)).toThrow(LoadFailure)
    try {
      parseSnapshot(future, CONTRACT_VERSION)
    } catch (error) {
      expect((error as LoadFailure).reason).toBe('unsupported-schema')
      expect((error as LoadFailure).message).toContain('nothing was loaded')
    }
  })

  test('an older schema with no migration is refused, not partly applied', () => {
    // There is one schema version, so there is no migration; the loader says
    // that plainly instead of dropping the fields it does not recognise.
    expect(Object.keys(MIGRATIONS)).toHaveLength(0)
    const older = JSON.stringify({ ...snapshot(), schema: 0 })
    try {
      parseSnapshot(older, CONTRACT_VERSION)
      expect.unreachable('an older schema with no migration must be refused')
    } catch (error) {
      expect((error as LoadFailure).reason).toBe('unsupported-schema')
      expect((error as LoadFailure).message).toContain('no migration')
    }
  })

  test('a draft written against another contract is refused', () => {
    const other = JSON.stringify({ ...snapshot(), contractVersion: 'v0' })
    try {
      parseSnapshot(other, CONTRACT_VERSION)
      expect.unreachable('a different contract must be refused')
    } catch (error) {
      expect((error as LoadFailure).reason).toBe('unsupported-contract')
    }
  })

  test('a name that was never saved is refused without inventing one', () => {
    expect(() => loadNamed(memoryStorage(), 'absent', CONTRACT_VERSION)).toThrow(LoadFailure)
  })
})

describe('model revisions', () => {
  test('a draft saved under the same revisions is unchanged', () => {
    expect(modelChanged(snapshot(), context.equationRevisions)).toEqual({
      changed: false,
      equations: [],
    })
  })

  test('a moved or removed equation is reported, so nothing cached is believed', () => {
    const moved = modelChanged(snapshot(), {
      'lift.stall-speed': '2',
      'geometry.aspect-ratio': '1',
    })
    expect(moved).toEqual({ changed: true, equations: ['lift.stall-speed'] })

    const removed = modelChanged(snapshot(), { 'geometry.aspect-ratio': '1' })
    expect(removed).toEqual({ changed: true, equations: ['lift.stall-speed'] })
  })
})
