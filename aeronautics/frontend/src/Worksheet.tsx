import type { ReactNode } from 'react'
import type { Transport } from './api/client.ts'
import { useWorksheet } from './useWorksheet.ts'
import { ResultsPanel } from './components/ResultsPanel.tsx'
import {
  CasesPanel,
  ConfigurationPanel,
  DraftsPanel,
  JourneyPanel,
  MassPanel,
  RequirementsPanel,
  WingPanel,
} from './components/panels.tsx'

/**
 * Worksheet is the whole page: entry choices and inputs on the left, the
 * results and requirements always visible on the right.
 *
 * Nothing here computes. Every edit becomes a curated command sent to the Go
 * boundary, and every number shown came back from it.
 */
export function Worksheet(props: { transport: Transport; session: string; storage?: Storage }): ReactNode {
  const api = useWorksheet(props.transport, props.session, props.storage ?? localStorage)

  return (
    <div className="worksheet">
      <header className="worksheet-header">
        <h1>RC wing sizing</h1>
        <div className="history" role="group" aria-label="History">
          <button type="button" onClick={() => { api.undo() }} disabled={!api.canUndo}>
            Undo
          </button>
          <button type="button" onClick={() => { api.redo() }} disabled={!api.canRedo}>
            Redo
          </button>
          {/*
            Recalculate stays live while a request is outstanding. Freshness is
            decided by the request identity, not by preventing a second ask: a
            late answer is discarded because it is not the outstanding one. And
            nothing here times a request out, so disabling the button would let
            one call that never returns leave the worksheet with no way to ask
            again. The headline says "Calculating…" meanwhile.
          */}
          <button type="button" onClick={() => { void api.evaluate() }}>
            Recalculate
          </button>
        </div>
      </header>

      <div className="columns">
        <div className="inputs">
          <JourneyPanel api={api} />
          <ConfigurationPanel api={api} />
          <MassPanel api={api} />
          <WingPanel api={api} />
          <CasesPanel api={api} />
          <RequirementsPanel api={api} />
          <DraftsPanel api={api} />
        </div>
        <ResultsPanel api={api} />
      </div>
    </div>
  )
}
