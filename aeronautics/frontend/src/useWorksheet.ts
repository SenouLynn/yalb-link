// The hook that binds the worksheet state to the Go boundary.
//
// It owns the request lifecycle and nothing else. Every edit becomes a curated
// command; the boundary answers with the edited design and its evaluation; the
// reducer decides whether that answer is still wanted. No component here
// decides what an edit means physically.

import { useCallback, useEffect, useMemo, useReducer, useRef, useState } from 'react'
import type { Command, Design, Issue, UnitInfo } from './api/contract.ts'
import { CONTRACT_VERSION } from './api/contract.ts'
import type { Discovered, Transport } from './api/client.ts'
import { BoundaryError, TransportError } from './api/client.ts'
import type { JourneyId } from './state/design.ts'
import type { Failure, Worksheet } from './state/worksheet.ts'
import {
  canRedo,
  canUndo,
  design as currentDesign,
  initialWorksheet,
  nextIdentity,
  reduce,
  staleResult,
} from './state/worksheet.ts'
import type { Snapshot, SnapshotContext } from './state/drafts.ts'
import { LoadFailure, buildSnapshot, loadNamed, modelChanged, saveNamed, savedNames } from './state/drafts.ts'

export interface WorksheetApi {
  readonly worksheet: Worksheet
  readonly design: Design
  readonly units: readonly UnitInfo[]
  readonly discovery: Discovered | null
  readonly pending: boolean
  readonly stale: boolean
  readonly canUndo: boolean
  readonly canRedo: boolean
  readonly draftNames: readonly string[]
  readonly notice: string | null
  selectJourney(journey: JourneyId): void
  setDraft(field: string, text: string): void
  discardDraft(field: string): void
  run(commands: Command[], clearFields?: string[]): Promise<boolean>
  evaluate(): Promise<void>
  undo(): void
  redo(): void
  saveDraft(name: string): void
  loadDraft(name: string): void
}

function asFailure(error: unknown): Failure {
  if (error instanceof BoundaryError) {
    return { message: error.message, issues: error.issues, transport: false }
  }
  if (error instanceof TransportError) {
    return {
      message: `${error.message}. Nothing was changed; the entries are still here, so try again `
        + 'once the calculation service is running.',
      issues: [] as Issue[],
      transport: true,
    }
  }
  return {
    message: error instanceof Error ? error.message : String(error),
    issues: [] as Issue[],
    transport: false,
  }
}

export function useWorksheet(
  transport: Transport,
  session: string,
  // Storage is injected so a test can hand in an ordinary in-memory one. The
  // worksheet only ever uses the store it is given, which is also what keeps
  // draft persistence out of the calculation path entirely.
  storage: Storage = localStorage,
): WorksheetApi {
  const [worksheet, dispatch] = useReducer(reduce, session, (s) => initialWorksheet(s))
  const [discovery, setDiscovery] = useState<Discovered | null>(null)
  const [units, setUnits] = useState<readonly UnitInfo[]>([])
  const [notice, setNotice] = useState<string | null>(null)

  // A request has to carry its identity out to the transport before the reducer
  // ever sees a response, so the identity is minted here and the reducer mints
  // the matching one when it records the request. The two counters agree
  // because every mint below is paired with exactly one 'request-started'
  // dispatch, in that order; nothing else starts a request. Adding a second
  // caller that does one without the other would put them out of step, and a
  // pending identity that never matches an answer discards every response.
  const sequence = useRef(0)
  const mintIdentity = useCallback(() => {
    sequence.current += 1
    return { session, sequence: sequence.current }
  }, [session])

  useEffect(() => {
    const controller = new AbortController()
    void (async () => {
      try {
        const discovered = await transport.discover(controller.signal)
        setDiscovery(discovered)
        setUnits(await transport.units(controller.signal))
      } catch (error) {
        if (controller.signal.aborted) return
        dispatch({ type: 'failed', failure: asFailure(error) })
      }
    })()
    return () => { controller.abort() }
  }, [transport])

  const design = currentDesign(worksheet)

  const evaluate = useCallback(async () => {
    const identity = mintIdentity()
    dispatch({ type: 'request-started', kind: 'evaluate' })
    try {
      const evaluation = await transport.evaluate({ request: identity, design })
      dispatch({ type: 'evaluated', evaluation })
    } catch (error) {
      dispatch({ type: 'failed', failure: asFailure(error) })
    }
  }, [transport, design, mintIdentity])

  const run = useCallback(
    async (commands: Command[], clearFields: string[] = []): Promise<boolean> => {
      const identity = mintIdentity()
      dispatch({ type: 'request-started', kind: 'apply' })
      try {
        const applied = await transport.apply({ request: identity, design, commands })
        dispatch({ type: 'applied', design: applied.design, evaluation: applied.evaluation })
        for (const field of clearFields) dispatch({ type: 'draft-discarded', field })
        setNotice(null)
        return true
      } catch (error) {
        // The draft text stays exactly as it was: a refused edit must not throw
        // away what the builder typed, and retrying is then one keystroke.
        dispatch({ type: 'failed', failure: asFailure(error) })
        return false
      }
    },
    [transport, design, mintIdentity],
  )

  // A design change with no result for it asks for one. Undo and redo do not go
  // through the boundary, so this is what makes their results current rather
  // than leaving the previous revision's answer on screen.
  const wantsEvaluation =
    worksheet.pending === null
    && worksheet.failure === null
    && (worksheet.current === null || staleResult(worksheet))
  useEffect(() => {
    if (!wantsEvaluation) return
    void evaluate()
  }, [wantsEvaluation, evaluate])

  const context: SnapshotContext | null = useMemo(
    () =>
      discovery
        ? {
            contractVersion: discovery.contractVersion,
            modelVersion: discovery.version,
            equationRevisions: discovery.equationRevisions,
          }
        : null,
    [discovery],
  )

  const [draftNames, setDraftNames] = useState<readonly string[]>(() => savedNames(storage))

  const saveDraft = useCallback(
    (name: string) => {
      if (!context) {
        setNotice('The calculation service has not been reached yet, so there is no model '
          + 'revision to record with the draft. Nothing was saved.')
        return
      }
      const snapshot: Snapshot = buildSnapshot(
        design,
        worksheet.drafts,
        worksheet.journey,
        context,
        new Date().toISOString(),
      )
      saveNamed(storage, name, snapshot)
      setDraftNames(savedNames(storage))
      setNotice(`Saved “${name}”. Unfinished entries were saved with it, separately from the `
        + 'committed values.')
    },
    [context, design, worksheet.drafts, worksheet.journey, storage],
  )

  const loadDraft = useCallback(
    (name: string) => {
      try {
        const snapshot = loadNamed(storage, name, CONTRACT_VERSION)
        const drifted = discovery
          ? modelChanged(snapshot, discovery.equationRevisions)
          : { changed: false, equations: [] as string[] }
        dispatch({
          type: 'draft-loaded',
          design: snapshot.design,
          drafts: { ...snapshot.drafts },
          journey: snapshot.journey,
        })
        setNotice(
          drifted.changed
            ? `Loaded “${name}”. The model has moved since it was saved (${drifted.equations.join(', ')}), `
              + 'so it is being evaluated again; nothing cached was shown.'
            : `Loaded “${name}”. It is being evaluated again; nothing cached was shown as current.`,
        )
      } catch (error) {
        // A refused load changes nothing at all. The open draft, its unfinished
        // text and the current result are exactly where they were.
        setNotice(
          error instanceof LoadFailure
            ? `${error.message} The design you had open is untouched.`
            : `“${name}” could not be read, so nothing was loaded. The design you had open is untouched.`,
        )
      }
    },
    [discovery, storage],
  )

  return {
    worksheet,
    design,
    units,
    discovery,
    pending: worksheet.pending !== null,
    stale: staleResult(worksheet),
    canUndo: canUndo(worksheet),
    canRedo: canRedo(worksheet),
    draftNames,
    notice,
    selectJourney: (journey) => { dispatch({ type: 'journey-selected', journey }) },
    setDraft: (field, text) => { dispatch({ type: 'draft-changed', field, text }) },
    discardDraft: (field) => { dispatch({ type: 'draft-discarded', field }) },
    run,
    evaluate,
    undo: () => { dispatch({ type: 'undo' }) },
    redo: () => { dispatch({ type: 'redo' }) },
    saveDraft,
    loadDraft,
  }
}

// nextIdentity is re-exported so a test can reason about the sequence without
// reaching into the reducer's internals.
export { nextIdentity }
