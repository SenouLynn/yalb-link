import type { ReactNode } from 'react'
import type { Bound, Check, Evaluation, Parameter } from '../api/contract.ts'
import { displayNumber } from '../state/fields.ts'
import type { WorksheetApi } from '../useWorksheet.ts'
import { Section } from './fields.tsx'
import { PowerResults } from './PowerResults.tsx'

// Status is said in words. Colour carries no information here: a status that
// can only be read by seeing it is not readable at all.

// The dimensionless unit is written "1", which reads as part of the number
// rather than as a unit. A ratio is shown without it.
function quantityText(value: { value: number; unit: string } | null | undefined): string {
  if (!value) return 'not available'
  if (value.unit === '1') return displayNumber(value.value)
  return `${displayNumber(value.value)} ${value.unit}`
}

function checkStatusText(check: Check): string {
  if (check.result !== 'computed') {
    switch (check.result) {
      case 'missing':
        return 'unknown — something it needs has not been supplied'
      case 'invalid':
        return 'unknown — an input it depends on cannot be true'
      case 'stale':
        return 'unknown — computed from an earlier revision'
      default:
        return 'unknown'
    }
  }
  return check.status
}

function boundText(bound: Bound): string {
  const detail = bound.detail ?? ''
  if (!bound.known) {
    return detail === '' ? 'not bounded' : detail
  }
  const controlling = bound.controlling ?? []
  const by = controlling.length === 0 ? '' : ` — set by ${controlling.join(' and ')}`
  const partial =
    bound.partial
      ? '. This bound is partial: a required case could not contribute, so it does not '
        + 'establish a complete feasible range.'
      : ''
  return `${bound.direction} ${quantityText(bound.value)}${by}${partial}`
}

function aggregateText(evaluation: Evaluation): string {
  if (!evaluation.hasRequired) {
    return 'No required requirement is set, so no feasibility claim is made. '
      + 'An empty set is not a passing one.'
  }
  switch (evaluation.aggregate) {
    case 'met':
      return 'Every required requirement is met.'
    case 'unmet':
      return 'At least one required requirement is unmet.'
    default:
      return 'At least one required requirement is unknown, and none is unmet.'
  }
}

/** ResultsPanel is the always-visible summary of what the candidate does. */
export function ResultsPanel(props: { api: WorksheetApi }): ReactNode {
  const { api } = props
  const current = api.worksheet.current
  const failure = api.worksheet.failure

  return (
    <aside className="results" aria-label="Results and requirements">
      <div className="results-status" role="status" aria-live="polite">
        <strong>{statusHeadline(api)}</strong>
        {api.stale && current !== null && (
          <p>
            These numbers describe revision {current.revision} of the design. The current
            revision is {api.worksheet.revision}.
          </p>
        )}
        {api.notice !== null && <p className="notice">{api.notice}</p>}
        {failure !== null && (
          <div className="failure">
            <p>{failure.message}</p>
            {failure.issues.length > 0 && (
              <ul>
                {failure.issues.map((issue) => (
                  <li key={`${issue.field}:${issue.detail}`}>
                    <code>{issue.field}</code>: {issue.kind} — {issue.detail}
                  </li>
                ))}
              </ul>
            )}
          </div>
        )}
      </div>

      {current === null ? (
        <p>No result yet.</p>
      ) : (
        <ResultsBody
          evaluation={current.evaluation}
          statesPower={statesPower(api)}
          {...(api.worksheet.previous ? { previousRevision: api.worksheet.previous.revision } : {})}
        />
      )}
    </aside>
  )
}

/**
 * statesPower reports whether the design describes a propulsion system at all.
 * It reads the definition rather than the result: a design with a mission and
 * no polar has a power answer to show — the one that says what is missing —
 * whereas a wing-sizing worksheet that has never mentioned a motor has none.
 */
function statesPower(api: WorksheetApi): boolean {
  const design = api.design
  return design.polar != null
    || design.battery != null
    || design.propulsion != null
    || (design.mission?.segments ?? []).length > 0
    || (design.auxiliary ?? []).length > 0
}

function statusHeadline(api: WorksheetApi): string {
  if (api.pending) return 'Calculating…'
  if (api.worksheet.failure !== null) return 'The last request was refused'
  if (api.worksheet.current === null) return 'Nothing calculated yet'
  if (api.stale) return 'Showing an earlier revision'
  return 'Current'
}

function ResultsBody(props: {
  evaluation: Evaluation
  statesPower: boolean
  previousRevision?: number | undefined
}): ReactNode {
  const { evaluation } = props
  return (
    <>
      <Section title="Requirements">
        <p>{aggregateText(evaluation)}</p>
        {evaluation.checks.length === 0 ? (
          <p>No requirement is being checked.</p>
        ) : (
          <table className="checks">
            <caption className="sr-only">Requirement outcomes</caption>
            <thead>
              <tr>
                <th scope="col">Requirement</th>
                <th scope="col">Case</th>
                <th scope="col">Bound</th>
                <th scope="col">Actual</th>
                <th scope="col">Status</th>
              </tr>
            </thead>
            <tbody>
              {evaluation.checks.map((check) => (
                <tr key={`${check.name}:${check.case ?? ''}:${check.direction}`}>
                  <th scope="row">
                    {check.name}
                    <span className="priority"> ({check.priority})</span>
                  </th>
                  <td>{check.case === '' || check.case === undefined ? '—' : check.case}</td>
                  <td>
                    {check.direction} {quantityText(check.bound)}
                  </td>
                  <td>{quantityText(check.actual)}</td>
                  <td>{checkStatusText(check)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Section>

      <Section title="Bounds">
        <dl className="bounds">
          <dt>Wing area, from below</dt>
          <dd>{boundText(evaluation.areaLower)}</dd>
          <dt>Wing area, from above</dt>
          <dd>{boundText(evaluation.areaUpper)}</dd>
          <dt>All-up mass</dt>
          <dd>
            {evaluation.mass.complete
              ? `between ${quantityText(evaluation.mass.lower.value)} and `
                + quantityText(evaluation.mass.upper.value)
              : ((evaluation.mass.detail ?? '') === '' ? 'not bounded' : evaluation.mass.detail)}
            {evaluation.mass.empty && ' No mass satisfies every required constraint.'}
          </dd>
        </dl>
      </Section>

      {evaluation.conflicts.length > 0 && (
        <Section title="Conflicts">
          {evaluation.conflicts.map((conflict) => (
            <div key={conflict.summary} className="conflict">
              <p>{conflict.summary}</p>
              <p className="detail">{conflict.detail}</p>
              <ul>
                {conflict.alternatives.map((alternative) => (
                  <li key={alternative.name}>
                    <strong>{alternative.name}</strong> — {alternative.description}
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </Section>
      )}

      <PowerResults evaluation={evaluation} stated={props.statesPower} />

      <Section title="Geometry" defaultOpen={false}>
        {evaluation.wing === null || evaluation.wing === undefined ? (
          <>
            <p>The wing did not solve, so no geometry is reported.</p>
            <ul>
              {evaluation.geometryIssues.map((issue) => (
                <li key={`${issue.field}:${issue.detail}`}>
                  <code>{issue.field}</code>: {issue.kind} — {issue.detail}
                </li>
              ))}
            </ul>
          </>
        ) : (
          <>
            <p>
              Solved from {evaluation.wing.solveMode}. Datum: {evaluation.wing.datum}
            </p>
            <table className="parameters">
              <caption className="sr-only">Solved parameters</caption>
              <thead>
                <tr>
                  <th scope="col">Parameter</th>
                  <th scope="col">Value</th>
                  <th scope="col">Role</th>
                  <th scope="col">From</th>
                </tr>
              </thead>
              <tbody>
                {evaluation.wing.parameters.map((parameter) => (
                  <ParameterRow key={parameter.key} parameter={parameter} />
                ))}
              </tbody>
            </table>
          </>
        )}
      </Section>

      {evaluation.configurationIssues.length > 0 && (
        <Section title="Airframe description">
          <ul>
            {evaluation.configurationIssues.map((issue) => (
              <li key={`${issue.field}:${issue.detail}`}>
                <code>{issue.field}</code>: {issue.kind} — {issue.detail}
              </li>
            ))}
          </ul>
          <p>This does not affect the wing sizing above.</p>
        </Section>
      )}

      <p className="coverage">
        Handling is unknown for every configuration: no trim, static-margin, control or
        structural model is implemented, and none of these numbers is one. The power and
        mission results above are energy and force balances at conditions you stated; they
        say nothing about whether the aircraft is controllable.
      </p>
      {props.previousRevision !== undefined && (
        <p className="coverage">A previous result, for revision {props.previousRevision}, is kept for comparison.</p>
      )}
    </>
  )
}

function ParameterRow(props: { parameter: Parameter }): ReactNode {
  const { parameter } = props
  const dependsOn = parameter.dependsOn ?? []
  return (
    <tr>
      <th scope="row">{parameter.key}</th>
      <td>{quantityText(parameter.value)}</td>
      <td>{parameter.role === 'driver' ? 'input' : 'derived'}</td>
      <td>
        {parameter.role === 'driver' ? (
          'entered'
        ) : (
          <details>
            <summary>{parameter.equationId === '' ? 'derived' : parameter.equationId}</summary>
            <p>
              revision {parameter.revision === '' ? 'unrecorded' : parameter.revision}
              {dependsOn.length > 0 && <> · from {dependsOn.join(', ')}</>}
            </p>
          </details>
        )}
      </td>
    </tr>
  )
}
