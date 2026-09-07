// The worksheet's single canonical state.
//
// Two rules shape everything here.
//
// The design is never edited in the browser. Every change is a curated command
// sent to the Go boundary, which returns the edited design and its evaluation
// together. That is what keeps one authoritative definition and keeps
// aerodynamic decisions out of the UI: there is no local code that could
// disagree with the core about what a driver swap does.
//
// Every design change retires the outstanding evaluation identity. A response
// is accepted only when it answers the request that is still outstanding, so a
// slow answer cannot land on a branch the builder has already left — including
// after an undo back to the exact design the answer was computed from.

import type {
  Design,
  Evaluation,
  Issue,
  Request as RequestIdentity,
  SweepSettings,
} from '../api/contract.ts'
import type { Sweeping, Swept } from './sweep.ts'
import { answers } from './sweep.ts'
import type { JourneyId } from './design.ts'
import { startingDesign } from './design.ts'

/** Accepted is a result the worksheet is showing, with the revision it describes. */
export interface Accepted {
  readonly evaluation: Evaluation
  /** revision is the design revision the result was computed from. */
  readonly revision: number
}

/** Pending is an outstanding request. */
export interface Pending {
  readonly identity: RequestIdentity
  readonly kind: 'evaluate' | 'apply'
  /** revision is the design revision the request was issued against. */
  readonly revision: number
}

export interface Failure {
  readonly message: string
  readonly issues: readonly Issue[]
  /** transport is true when the service could not be reached at all. */
  readonly transport: boolean
}

export interface Worksheet {
  readonly session: string
  readonly nextSequence: number
  readonly history: readonly Design[]
  readonly cursor: number
  /** revision increases on every committed design change, undo included. */
  readonly revision: number
  /** drafts is unfinished editor text, keyed by field, kept apart from values. */
  readonly drafts: Readonly<Record<string, string>>
  readonly pending: Pending | null
  readonly current: Accepted | null
  /** previous is the result before the last accepted one, kept for comparison. */
  readonly previous: Accepted | null
  readonly journey: JourneyId | null
  readonly failure: Failure | null
  /**
   * selection is the parameter a builder has selected, by the core's own key.
   * It is what makes a dimension on a drawing and a field in the worksheet one
   * thing: both read this, so selecting either highlights the other.
   */
  readonly selection: string | null
  /** sweeping is the outstanding sensitivity request, if there is one. */
  readonly sweeping: Sweeping | null
  /** swept is the sensitivity answer currently being shown. */
  readonly swept: Swept | null
}

export function initialWorksheet(session: string, design = startingDesign()): Worksheet {
  return {
    session,
    nextSequence: 0,
    history: [design],
    cursor: 0,
    revision: 0,
    drafts: {},
    pending: null,
    current: null,
    previous: null,
    journey: null,
    failure: null,
    selection: null,
    sweeping: null,
    swept: null,
  }
}

export function design(worksheet: Worksheet): Design {
  const found = worksheet.history[worksheet.cursor]
  if (!found) throw new Error('the worksheet history is empty')
  return found
}

export function canUndo(worksheet: Worksheet): boolean {
  return worksheet.cursor > 0
}

export function canRedo(worksheet: Worksheet): boolean {
  return worksheet.cursor < worksheet.history.length - 1
}

/** staleResult reports whether the shown result describes an earlier revision. */
export function staleResult(worksheet: Worksheet): boolean {
  return worksheet.current !== null && worksheet.current.revision !== worksheet.revision
}

/**
 * staleSweep reports whether the plotted curve describes an earlier revision.
 * It is a separate question from staleResult: an edit retires both, but
 * changing the range or the plotted output retires only the sweep.
 */
export function staleSweep(worksheet: Worksheet): boolean {
  return worksheet.swept !== null && worksheet.swept.revision !== worksheet.revision
}

export type Action =
  | { readonly type: 'journey-selected'; readonly journey: JourneyId }
  | { readonly type: 'draft-changed'; readonly field: string; readonly text: string }
  | { readonly type: 'draft-discarded'; readonly field: string }
  | { readonly type: 'request-started'; readonly kind: 'evaluate' | 'apply' }
  | { readonly type: 'applied'; readonly design: Design; readonly evaluation: Evaluation }
  | { readonly type: 'evaluated'; readonly evaluation: Evaluation }
  | { readonly type: 'failed'; readonly failure: Failure }
  | { readonly type: 'undo' }
  | { readonly type: 'redo' }
  | { readonly type: 'draft-loaded'; readonly design: Design; readonly drafts: Record<string, string>; readonly journey: JourneyId | null }
  | { readonly type: 'parameter-selected'; readonly key: string | null }
  | { readonly type: 'sweep-started'; readonly settings: SweepSettings; readonly sequence: number }
  | { readonly type: 'preview-started'; readonly sequence: number }
  | { readonly type: 'swept'; readonly response: import('../api/contract.ts').SweepResponse; readonly snapshot: string }
  | { readonly type: 'sweep-discarded' }

/**
 * nextIdentity mints the identity the next request will carry. The sequence
 * only ever increases: undo, redo and loading a draft all restore a design and
 * none of them restores an identity.
 */
export function nextIdentity(worksheet: Worksheet): { identity: RequestIdentity; worksheet: Worksheet } {
  const sequence = worksheet.nextSequence + 1
  return {
    identity: { session: worksheet.session, sequence },
    worksheet: { ...worksheet, nextSequence: sequence },
  }
}

/** sameIdentity compares two request identities. */
export function sameIdentity(a: RequestIdentity | undefined, b: RequestIdentity | undefined): boolean {
  if (!a || !b) return false
  return a.session === b.session && a.sequence === b.sequence
}

// commitDesign records a new design revision. It retires the outstanding
// identity, because the request that identity belongs to was asked about a
// design the worksheet no longer holds.
function commitDesign(worksheet: Worksheet, next: Design): Worksheet {
  return {
    ...worksheet,
    history: [...worksheet.history.slice(0, worksheet.cursor + 1), next],
    cursor: worksheet.cursor + 1,
    revision: worksheet.revision + 1,
    pending: null,
    failure: null,
    // A design change retires an outstanding sweep for the same reason it
    // retires an outstanding evaluation: the answer on its way describes a
    // branch the builder has left. The plotted curve stays on screen, marked as
    // describing an earlier revision, rather than vanishing mid-read.
    sweeping: null,
  }
}

export function reduce(worksheet: Worksheet, action: Action): Worksheet {
  switch (action.type) {
    case 'journey-selected':
      return { ...worksheet, journey: action.journey }

    case 'draft-changed':
      return { ...worksheet, drafts: { ...worksheet.drafts, [action.field]: action.text } }

    case 'draft-discarded': {
      const drafts = Object.fromEntries(
        Object.entries(worksheet.drafts).filter(([field]) => field !== action.field),
      )
      return { ...worksheet, drafts }
    }

    case 'request-started': {
      const { identity, worksheet: minted } = nextIdentity(worksheet)
      return {
        ...minted,
        pending: { identity, kind: action.kind, revision: worksheet.revision },
        failure: null,
      }
    }

    case 'applied': {
      // An apply answers with the edited design and its evaluation together, so
      // the two are consistent by construction. It is still checked against the
      // outstanding identity: a response to an abandoned edit must not silently
      // replace the design the builder is now looking at.
      if (!sameIdentity(worksheet.pending?.identity, action.evaluation.request)) {
        return worksheet
      }
      const committed = commitDesign(worksheet, action.design)
      return {
        ...committed,
        previous: worksheet.current,
        current: { evaluation: action.evaluation, revision: committed.revision },
      }
    }

    case 'evaluated': {
      const pending = worksheet.pending
      if (pending === null || !sameIdentity(pending.identity, action.evaluation.request)) {
        return worksheet
      }
      return {
        ...worksheet,
        pending: null,
        previous: worksheet.current,
        current: { evaluation: action.evaluation, revision: pending.revision },
        failure: null,
      }
    }

    case 'failed':
      // A failure clears the outstanding request and leaves the current result
      // and every draft exactly as they were. A refused call is not a reason to
      // lose what the builder typed.
      return { ...worksheet, pending: null, failure: action.failure }

    case 'undo':
      if (!canUndo(worksheet)) return worksheet
      return {
        ...worksheet,
        cursor: worksheet.cursor - 1,
        revision: worksheet.revision + 1,
        pending: null,
        failure: null,
        sweeping: null,
      }

    case 'redo':
      if (!canRedo(worksheet)) return worksheet
      return {
        ...worksheet,
        cursor: worksheet.cursor + 1,
        revision: worksheet.revision + 1,
        pending: null,
        failure: null,
        sweeping: null,
      }

    case 'parameter-selected':
      return { ...worksheet, selection: action.key }

    // Every request that leaves the worksheet takes an identity out of the same
    // stream, so every one of them has to move the counter — including the two
    // that record nothing else. A request whose identity was minted outside the
    // counter would put the two out of step, and a pending identity that never
    // matches an answer discards every response after it.
    case 'sweep-started':
      return {
        ...worksheet,
        nextSequence: action.sequence,
        sweeping: {
          settings: action.settings,
          sequence: action.sequence,
          revision: worksheet.revision,
        },
        failure: null,
      }

    case 'preview-started':
      // A preview describes a design the worksheet does not hold, so nothing
      // but the counter changes: there is no pending state to record and no
      // result to accept.
      return { ...worksheet, nextSequence: action.sequence }

    case 'swept': {
      // A sweep answer is accepted only when it answers the outstanding
      // request, over the inputs the design still holds, for the settings still
      // selected. An answer that fails any of the three is somebody else's.
      const asked = worksheet.sweeping
      if (asked === null || !answers(action.response, asked, action.snapshot, asked.settings)) {
        return worksheet
      }
      return {
        ...worksheet,
        sweeping: null,
        swept: {
          response: action.response,
          settings: asked.settings,
          revision: asked.revision,
        },
      }
    }

    case 'sweep-discarded':
      return { ...worksheet, sweeping: null, swept: null }

    case 'draft-loaded': {
      const committed = commitDesign(worksheet, action.design)
      return {
        ...committed,
        drafts: action.drafts,
        journey: action.journey,
        // A loaded draft's cached result, if it had one, is not restored as
        // current. It was computed somewhere else, possibly under a different
        // model revision, and showing it would present a stale output as a
        // current one.
        previous: null,
        current: null,
        // A loaded draft is a different design, so a curve plotted for the
        // previous one is not an earlier revision of this one; it is an answer
        // about something else and is dropped rather than shown as stale.
        swept: null,
      }
    }

    default:
      return worksheet
  }
}
