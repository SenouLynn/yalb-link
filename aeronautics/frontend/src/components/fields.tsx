import { useRef, useState } from 'react'
import type { ReactNode } from 'react'
import type { Quantity, UnitInfo } from '../api/contract.ts'
import { commitText, displayNumber, isEditingState } from '../state/fields.ts'
import type { WorksheetApi } from '../useWorksheet.ts'

/**
 * Section is a collapsible group. It is open by default: a worksheet that hides
 * its own contents on arrival makes the builder hunt for the field they came
 * for.
 */
export function Section(props: {
  title: string
  summary?: string
  children: ReactNode
  defaultOpen?: boolean
}): ReactNode {
  return (
    <details className="section" open={props.defaultOpen ?? true}>
      <summary>
        <span className="section-title">{props.title}</span>
        {props.summary !== undefined && <span className="section-summary">{props.summary}</span>}
      </summary>
      <div className="section-body">{props.children}</div>
    </details>
  )
}

/** unitsFor lists the units of one dimension, the SI one first. */
export function unitsFor(units: readonly UnitInfo[], dimension: string): UnitInfo[] {
  return units
    .filter((unit) => unit.dimension === dimension)
    .sort((a, b) => (a.si === b.si ? a.symbol.localeCompare(b.symbol) : a.si ? -1 : 1))
}

/**
 * inUnit renders a stored value in a chosen unit. The factors come from the
 * service's own table, so this is arithmetic on an authoritative conversion
 * rather than a second copy of one; it changes what is shown and never what is
 * stored or evaluated.
 */
export function inUnit(value: Quantity, symbol: string, units: readonly UnitInfo[]): number | null {
  const from = units.find((unit) => unit.symbol === value.unit)
  const to = units.find((unit) => unit.symbol === symbol)
  if (!from || !to) return null
  return (value.value * from.factorToSi) / to.factorToSi
}

export interface CommitTarget {
  /** value is the number the builder committed, in the chosen unit. */
  value: number
  unit: string
}

export interface QuantityFieldProps {
  id: string
  label: string
  dimension: string
  value: Quantity | null
  api: WorksheetApi
  /** role labels the value as something the builder chose or something derived. */
  role?: 'driver' | 'derived'
  /** hint explains the field, and for a derived value names what it follows from. */
  hint?: string
  /** issue is a problem the service reported against this field. */
  issue?: string | undefined
  /** actions are the buttons that belong to the row, such as “Use as input”. */
  actions?: ReactNode
  readOnly?: boolean
  onCommit?: (target: CommitTarget | null) => void
}

/**
 * QuantityField is one number and its unit.
 *
 * The text being typed and the committed value are separate. "", "-" and "1."
 * are on the way to a number, not errors, so nothing complains mid-keystroke.
 * Enter and blur commit; Escape restores the committed value; an invalid commit
 * keeps the text and says why, so nothing typed is lost. Clearing a field
 * withdraws the value and never means zero.
 */
export function QuantityField(props: QuantityFieldProps): ReactNode {
  const { api, id, value, dimension } = props
  const choices = unitsFor(api.units, dimension)
  const fallback = choices[0]?.symbol ?? value?.unit ?? ''
  const [unit, setUnit] = useState<string | null>(null)
  const [localIssue, setLocalIssue] = useState<string | null>(null)
  const shownUnit = unit ?? value?.unit ?? fallback

  const committedText =
    value === null
      ? ''
      : (() => {
          const converted = inUnit(value, shownUnit, api.units)
          return converted === null ? displayNumber(value.value) : displayNumber(converted)
        })()
  const draft = api.worksheet.drafts[id]
  const text = draft ?? committedText
  const editing = draft !== undefined

  // A commit is a request, and the draft outlives it: the text has to stay
  // until the answer comes back, because a refused edit must keep what was
  // typed. So between pressing Enter and the response arriving, the field still
  // holds text that differs from the committed value — and blurring it, by
  // clicking Undo or any other control, would send the same edit a second time.
  // The text last sent is remembered until it changes.
  //
  // Enter is exempt. It is a deliberate act, so pressing it again after a
  // refusal retries rather than doing nothing.
  const sent = useRef<string | null>(null)

  function commit(deliberate: boolean): void {
    if (draft === undefined) return
    if (draft === committedText) {
      api.discardDraft(id)
      setLocalIssue(null)
      return
    }
    if (!deliberate && draft === sent.current) return
    const commitment = commitText(draft)
    if (commitment.kind === 'invalid') {
      setLocalIssue(`${commitment.reason}. The entry is still here; correct it or press Escape.`)
      return
    }
    setLocalIssue(null)
    sent.current = draft
    props.onCommit?.(
      commitment.kind === 'cleared' ? null : { value: commitment.value, unit: shownUnit },
    )
  }

  const issue = localIssue ?? props.issue ?? null
  const describedBy = [
    props.hint === undefined ? null : `${id}-hint`,
    issue === null ? null : `${id}-issue`,
  ].filter((entry): entry is string => entry !== null)

  return (
    <div className={`field${issue === null ? '' : ' field-issue'}`}>
      <label className="field-label" htmlFor={id}>
        {props.label}
      </label>
      <div className="field-entry">
        <input
          id={id}
          className="field-input"
          type="text"
          inputMode="decimal"
          autoComplete="off"
          readOnly={props.readOnly ?? false}
          value={text}
          aria-describedby={describedBy.length === 0 ? undefined : describedBy.join(' ')}
          aria-invalid={issue !== null}
          onChange={(event) => {
            setLocalIssue(null)
            sent.current = null
            api.setDraft(id, event.target.value)
          }}
          onBlur={() => { commit(false) }}
          onKeyDown={(event) => {
            if (event.key === 'Enter') {
              event.preventDefault()
              commit(true)
            }
            if (event.key === 'Escape') {
              event.preventDefault()
              setLocalIssue(null)
              sent.current = null
              api.discardDraft(id)
            }
          }}
        />
        {choices.length > 1 ? (
          <select
            className="field-unit"
            aria-label={`Unit for ${props.label}`}
            value={shownUnit}
            onChange={(event) => {
              // Changing the unit changes what is shown and nothing else: the
              // stored value is untouched, so no request is made and no result
              // becomes stale.
              setUnit(event.target.value)
              api.discardDraft(id)
            }}
          >
            {choices.map((choice) => (
              <option key={choice.symbol} value={choice.symbol}>
                {choice.symbol}
              </option>
            ))}
          </select>
        ) : (
          // The dimensionless unit is written "1", which reads as part of the
          // number rather than as a unit, so a ratio shows none.
          shownUnit !== '1' && <span className="field-unit-fixed">{shownUnit}</span>
        )}
        {props.role !== undefined && (
          <span className={`role role-${props.role}`}>{props.role === 'driver' ? 'input' : 'derived'}</span>
        )}
        {props.actions}
      </div>
      {props.hint !== undefined && (
        <p className="field-hint" id={`${id}-hint`}>
          {props.hint}
        </p>
      )}
      {issue !== null && (
        <p className="field-issue-text" id={`${id}-issue`} role="status">
          {issue}
        </p>
      )}
      {editing && isEditingState(text) && issue === null && (
        <p className="field-hint">Still being typed; press Enter to use it.</p>
      )}
    </div>
  )
}

/** TextField is a plain text entry, used for the bases every value must state. */
export function TextField(props: {
  id: string
  label: string
  value: string
  api: WorksheetApi
  hint?: string
  issue?: string | undefined
  onCommit(value: string): void
}): ReactNode {
  const { api, id } = props
  const draft = api.worksheet.drafts[id]
  const text = draft ?? props.value
  // The same rule as QuantityField: blur must not resend the text Enter has
  // already sent while that request is still outstanding.
  const sent = useRef<string | null>(null)

  function commit(deliberate: boolean): void {
    if (draft === undefined || draft === props.value) return
    if (!deliberate && draft === sent.current) return
    sent.current = draft
    props.onCommit(draft)
  }

  return (
    <div className={`field${props.issue === undefined ? '' : ' field-issue'}`}>
      <label className="field-label" htmlFor={id}>
        {props.label}
      </label>
      <div className="field-entry">
        <input
          id={id}
          className="field-input field-input-wide"
          type="text"
          autoComplete="off"
          value={text}
          aria-invalid={props.issue !== undefined}
          aria-describedby={props.hint === undefined ? undefined : `${id}-hint`}
          onChange={(event) => {
            sent.current = null
            api.setDraft(id, event.target.value)
          }}
          onBlur={() => { commit(false) }}
          onKeyDown={(event) => {
            if (event.key === 'Enter') {
              event.preventDefault()
              commit(true)
            }
            if (event.key === 'Escape') {
              event.preventDefault()
              sent.current = null
              api.discardDraft(id)
            }
          }}
        />
      </div>
      {props.hint !== undefined && (
        <p className="field-hint" id={`${id}-hint`}>
          {props.hint}
        </p>
      )}
      {props.issue !== undefined && (
        <p className="field-issue-text" role="status">
          {props.issue}
        </p>
      )}
    </div>
  )
}
