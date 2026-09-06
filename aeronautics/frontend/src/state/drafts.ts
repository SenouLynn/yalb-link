// Saved drafts: the versioned snapshot contract.
//
// A snapshot holds the authoritative inputs and nothing derived. Results are
// not stored as results: the equation revisions that produced them are stored
// instead, so a snapshot opened against a changed model is recomputed rather
// than believed. Unfinished editor text travels separately from committed
// values, because losing what someone was halfway through typing is a real loss.
//
// Storage I/O is the browser's. Physical validation is the Go boundary's: this
// module checks the shape and the version, and then the design is evaluated,
// which is what says whether it is a sound aircraft.

import type { Design } from '../api/contract.ts'
import type { JourneyId } from './design.ts'

/**
 * SCHEMA_VERSION is the snapshot format. It changes when a field is removed,
 * renamed or given a new meaning.
 *
 * There is exactly one version, so there is no migration to run and none is
 * claimed. MIGRATIONS is the mechanism a later version will use; an unknown
 * version is refused rather than guessed at, and refusing leaves the open draft
 * untouched. Tasks 07 to 09 extend this contract, and each addition either
 * keeps the version or arrives with its migration and a fixture of the schema
 * it migrates from.
 */
export const SCHEMA_VERSION = 1

export interface Snapshot {
  readonly schema: number
  /** contractVersion is the wire contract the design was written against. */
  readonly contractVersion: string
  /** modelVersion is the calculator version that evaluated it. */
  readonly modelVersion: string
  /**
   * equationRevisions are the revisions in force when the draft was saved. They
   * are what makes a cached result checkable rather than trusted.
   */
  readonly equationRevisions: Readonly<Record<string, string>>
  /** design is the authoritative input: drivers, roles, cases, requirements. */
  readonly design: Design
  /** drafts is unfinished editor text, kept apart from the committed values. */
  readonly drafts: Readonly<Record<string, string>>
  readonly journey: JourneyId | null
  readonly savedAt: string
}

export interface SnapshotContext {
  readonly contractVersion: string
  readonly modelVersion: string
  readonly equationRevisions: Readonly<Record<string, string>>
}

export function buildSnapshot(
  design: Design,
  drafts: Readonly<Record<string, string>>,
  journey: JourneyId | null,
  context: SnapshotContext,
  savedAt: string,
): Snapshot {
  return {
    schema: SCHEMA_VERSION,
    contractVersion: context.contractVersion,
    modelVersion: context.modelVersion,
    equationRevisions: { ...context.equationRevisions },
    design,
    drafts: { ...drafts },
    journey,
    savedAt,
  }
}

/** LoadFailure says why a snapshot was refused, in terms a builder can act on. */
export class LoadFailure extends Error {
  readonly reason: 'malformed' | 'unsupported-schema' | 'unsupported-contract'

  constructor(reason: 'malformed' | 'unsupported-schema' | 'unsupported-contract', message: string) {
    super(message)
    this.name = 'LoadFailure'
    this.reason = reason
  }
}

/**
 * MIGRATIONS maps a stored schema version onto a function that brings it up to
 * the next one. It is empty because there has only ever been one version. A
 * migration added later belongs here together with a fixture saved in the
 * schema it migrates from, so the path is exercised rather than assumed.
 */
export const MIGRATIONS: Readonly<Record<number, (stored: Record<string, unknown>) => Record<string, unknown>>> = {}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

/**
 * parseSnapshot reads stored text into a snapshot, or refuses it. It never
 * partially applies one: a caller that catches a LoadFailure still holds
 * exactly the draft it had.
 */
export function parseSnapshot(text: string, expectedContract: string): Snapshot {
  let decoded: unknown
  try {
    decoded = JSON.parse(text)
  } catch {
    throw new LoadFailure('malformed', 'the file is not valid JSON, so nothing was loaded')
  }
  if (!isRecord(decoded)) {
    throw new LoadFailure('malformed', 'the file does not hold a saved design, so nothing was loaded')
  }

  let stored = decoded
  let schema = stored['schema']
  while (typeof schema === 'number' && schema < SCHEMA_VERSION) {
    const migrate = MIGRATIONS[schema]
    if (!migrate) {
      throw new LoadFailure(
        'unsupported-schema',
        `this draft was saved in format ${String(schema)} and no migration to format `
          + `${String(SCHEMA_VERSION)} exists, so nothing was loaded`,
      )
    }
    stored = migrate(stored)
    schema = stored['schema']
  }
  if (schema !== SCHEMA_VERSION) {
    throw new LoadFailure(
      'unsupported-schema',
      `this draft was saved in format ${String(schema)}, which this worksheet does not read; `
        + `it reads format ${String(SCHEMA_VERSION)}, so nothing was loaded`,
    )
  }

  const contractVersion = stored['contractVersion']
  if (typeof contractVersion !== 'string') {
    throw new LoadFailure('malformed', 'the draft does not say which contract it was written against')
  }
  if (contractVersion !== expectedContract) {
    throw new LoadFailure(
      'unsupported-contract',
      `this draft was written against contract ${contractVersion} and the service serves `
        + `${expectedContract}, so nothing was loaded`,
    )
  }
  if (!isRecord(stored['design'])) {
    throw new LoadFailure('malformed', 'the draft holds no design, so nothing was loaded')
  }

  const drafts: Record<string, string> = {}
  if (isRecord(stored['drafts'])) {
    for (const [field, value] of Object.entries(stored['drafts'])) {
      if (typeof value === 'string') drafts[field] = value
    }
  }
  const revisions: Record<string, string> = {}
  if (isRecord(stored['equationRevisions'])) {
    for (const [id, revision] of Object.entries(stored['equationRevisions'])) {
      if (typeof revision === 'string') revisions[id] = revision
    }
  }
  const journey = stored['journey']

  return {
    schema: SCHEMA_VERSION,
    contractVersion,
    modelVersion: typeof stored['modelVersion'] === 'string' ? stored['modelVersion'] : '',
    equationRevisions: revisions,
    design: stored['design'] as unknown as Design,
    drafts,
    journey: typeof journey === 'string' ? (journey as JourneyId) : null,
    savedAt: typeof stored['savedAt'] === 'string' ? stored['savedAt'] : '',
  }
}

/**
 * modelChanged reports whether the equations behind a saved draft have moved
 * since it was written. It is why a snapshot stores revisions rather than
 * numbers: when this is true, anything cached against those revisions is
 * unknown until the design is evaluated again.
 */
export function modelChanged(
  snapshot: Snapshot,
  live: Readonly<Record<string, string>>,
): { changed: boolean; equations: string[] } {
  const equations: string[] = []
  for (const [id, revision] of Object.entries(snapshot.equationRevisions)) {
    const now = live[id]
    if (now === undefined || now !== revision) equations.push(id)
  }
  equations.sort()
  return { changed: equations.length > 0, equations }
}

const STORAGE_PREFIX = 'yalb.aero.draft.'

/** saveNamed writes a snapshot to browser storage under a builder-chosen name. */
export function saveNamed(storage: Storage, name: string, snapshot: Snapshot): void {
  storage.setItem(STORAGE_PREFIX + name, JSON.stringify(snapshot))
}

/** loadNamed reads a stored snapshot, or throws a LoadFailure. */
export function loadNamed(storage: Storage, name: string, expectedContract: string): Snapshot {
  const text = storage.getItem(STORAGE_PREFIX + name)
  if (text === null) {
    throw new LoadFailure('malformed', `no draft is saved under ${name}, so nothing was loaded`)
  }
  return parseSnapshot(text, expectedContract)
}

/** savedNames lists the drafts in storage, in a stable order. */
export function savedNames(storage: Storage): string[] {
  const names: string[] = []
  for (let n = 0; n < storage.length; n += 1) {
    const key = storage.key(n)
    if (key?.startsWith(STORAGE_PREFIX) === true) {
      names.push(key.slice(STORAGE_PREFIX.length))
    }
  }
  names.sort()
  return names
}
