import { useMemo, useState } from 'react'
import type { ReactNode } from 'react'
import type {
  PowerSearchCandidate,
  PowerSearchResponse,
  PowerSearchSettings,
  Quantity,
} from '../api/contract.ts'
import { commitText, displayNumber } from '../state/fields.ts'
import { SEARCH_DEFAULT_SAMPLES, searchKey, searchableDrivers } from '../state/power.ts'
import { defaultRange, driverValue } from '../state/sweep.ts'
import type { WorksheetApi } from '../useWorksheet.ts'
import { Section } from './fields.tsx'

// The bounded power search.
//
// This is the power-first journey's answer, and its shape is the point. A mass
// and an electrical power ceiling do not determine a wing: the demand is not
// monotonic in wing area at a fixed speed, so the wings that fit a ceiling
// generally form an interval and can form several. What is reported is
// therefore every evaluated candidate and the runs they fall into, with the
// evaluated candidates that bracket each run's ends — never a single wing
// presented as the answer.
//
// Nothing here is a solver. No iterate converges, so there is no iterate that
// could be shown as a converged design, and the interval ends are brackets
// between two candidates that were both evaluated rather than a boundary
// anything solved for.

const DRIVER_LABELS: Readonly<Record<string, string>> = {
  'wing.span.projected': 'Span',
  'wing.span.panel': 'Span, panel plane',
  'wing.area.reference': 'Wing area',
  'wing.area.panel': 'Wing area, panel plane',
  'wing.aspect_ratio.planform': 'Aspect ratio',
  'wing.chord.root': 'Root chord',
}

function labelFor(key: string): string {
  return DRIVER_LABELS[key] ?? key
}

function quantityText(value: Quantity | null | undefined): string {
  if (!value) return 'not available'
  if (value.unit === '1') return displayNumber(value.value)
  return `${displayNumber(value.value)} ${value.unit}`
}

/**
 * outcomeText says what a candidate is in words. Fitting the ceiling and
 * meeting the design's own required requirements are separate facts and stay
 * separate here: a wing that fits the ceiling and stalls too fast is not a
 * smaller answer, it is the wrong one, and saying only "no" would hide which
 * of the two went wrong.
 */
function outcomeText(candidate: PowerSearchCandidate): string {
  if (candidate.status !== 'computed') {
    return `demand not computed — ${candidate.detail ?? 'something it needs is missing'}`
  }
  if (!candidate.withinCeiling) return 'over the ceiling'
  if (candidate.feasible) return 'fits the ceiling and meets every required requirement'
  if (!candidate.hasRequired) return 'fits the ceiling; no required requirement is set'
  return `fits the ceiling but its required requirements are ${candidate.feasibility}`
}

interface Draft {
  readonly driver: string
  readonly from: string
  readonly to: string
  readonly ceiling: string
  readonly ceilingUnit: string
  readonly samples: string
}

/**
 * Unbuilt is a search that cannot be asked for yet. `stated` separates the two
 * reasons: a field nobody has filled in is a prompt for what the search needs,
 * while text that will not read as a number is a refusal of something the
 * builder did type. The panel opens with no ceiling entered, so reporting that
 * as bad input would greet every builder with an error about a field they have
 * not touched.
 */
interface Unbuilt {
  readonly error: string
  readonly stated: boolean
}

function buildSettings(draft: Draft, unit: string): PowerSearchSettings | Unbuilt {
  const from = commitText(draft.from)
  const to = commitText(draft.to)
  const ceiling = commitText(draft.ceiling)
  const missing = [
    from.kind === 'cleared' ? 'the start of the range' : '',
    to.kind === 'cleared' ? 'the end of the range' : '',
    ceiling.kind === 'cleared' ? 'a power ceiling to judge the candidates against' : '',
  ].filter((entry) => entry !== '')
  if (missing.length > 0) {
    return { error: `This search still needs ${missing.join(' and ')}.`, stated: false }
  }
  if (from.kind !== 'value') return { error: 'The start of the range is not a number.', stated: true }
  if (to.kind !== 'value') return { error: 'The end of the range is not a number.', stated: true }
  if (ceiling.kind !== 'value') return { error: 'The power ceiling is not a number.', stated: true }
  const samples = Number(draft.samples)
  if (!Number.isInteger(samples)) {
    return { error: 'The candidate count is not a whole number.', stated: true }
  }
  return {
    driver: draft.driver,
    from: { value: from.value, unit },
    to: { value: to.value, unit },
    ceiling: { value: ceiling.value, unit: draft.ceilingUnit },
    samples,
  }
}

/**
 * PowerSearchPanel searches a range of wings against an electrical power
 * ceiling. It commits nothing: adopting a candidate is the ordinary driver
 * edit, made deliberately from the table.
 */
export function PowerSearchPanel(props: { api: WorksheetApi }): ReactNode {
  const { api } = props
  const design = api.design
  const evaluation = api.worksheet.current?.evaluation ?? null
  const drivers = searchableDrivers(evaluation?.wing?.drivers ?? [])
  const firstDriver = drivers[0] ?? ''
  const [draft, setDraft] = useState<Draft | null>(null)

  const current = driverValue(design, draft?.driver ?? firstDriver)
  const unit = current?.unit ?? '1'
  const active: Draft = draft ?? {
    driver: firstDriver,
    from: current ? displayNumber(defaultRange(current).from.value) : '',
    to: current ? displayNumber(defaultRange(current).to.value) : '',
    ceiling: '',
    ceilingUnit: 'W',
    samples: String(SEARCH_DEFAULT_SAMPLES),
  }
  const built = useMemo(() => buildSettings(active, unit), [active, unit])
  const searched = api.worksheet.searched
  const answersThis =
    searched !== null && !('error' in built) && searchKey(searched.settings) === searchKey(built)

  function update(change: Partial<Draft>): void {
    setDraft({ ...active, ...change })
  }

  if (drivers.length === 0) {
    return (
      <Section title="Which wings fit the power ceiling?">
        <p>
          Nothing is being driven yet. Enter two size values, and one of them can be searched
          across a range against a power ceiling.
        </p>
      </Section>
    )
  }

  return (
    <Section title="Which wings fit the power ceiling?">
      <p className="panel-note">
        A mass and a power ceiling do not determine a wing. At a fixed speed a small wing
        pays induced drag and a large one pays parasite drag, so the wings that fit a
        ceiling form an interval — sometimes more than one — and this reports them rather
        than choosing between them. Each candidate is an ordinary design evaluated the
        ordinary way; the mission is what demands the power, so there has to be one.
      </p>

      <div className="sweep-controls">
        <label className="field-label" htmlFor="search-driver">Vary</label>
        <select
          id="search-driver"
          value={active.driver}
          onChange={(event) => {
            const next = driverValue(design, event.target.value)
            update({
              driver: event.target.value,
              from: next ? displayNumber(defaultRange(next).from.value) : '',
              to: next ? displayNumber(defaultRange(next).to.value) : '',
            })
          }}
        >
          {drivers.map((key) => (
            <option key={key} value={key}>{labelFor(key)}</option>
          ))}
        </select>

        <label className="field-label" htmlFor="search-from">Lowest</label>
        <input
          id="search-from"
          type="text"
          inputMode="decimal"
          value={active.from}
          onChange={(event) => { update({ from: event.target.value }) }}
        />
        <label className="field-label" htmlFor="search-to">Highest</label>
        <input
          id="search-to"
          type="text"
          inputMode="decimal"
          value={active.to}
          onChange={(event) => { update({ to: event.target.value }) }}
        />
        <span className="field-unit-fixed">{unit === '1' ? '' : unit}</span>

        <label className="field-label" htmlFor="search-ceiling">Power ceiling</label>
        <input
          id="search-ceiling"
          type="text"
          inputMode="decimal"
          value={active.ceiling}
          onChange={(event) => { update({ ceiling: event.target.value }) }}
        />
        <select
          aria-label="Unit for the power ceiling"
          value={active.ceilingUnit}
          onChange={(event) => { update({ ceilingUnit: event.target.value }) }}
        >
          <option value="W">W</option>
          <option value="kW">kW</option>
        </select>

        <label className="field-label" htmlFor="search-samples">Candidates</label>
        <input
          id="search-samples"
          type="text"
          inputMode="numeric"
          value={active.samples}
          onChange={(event) => { update({ samples: event.target.value }) }}
        />

        <button
          type="button"
          onClick={() => {
            if ('error' in built) return
            void api.runPowerSearch(built)
          }}
        >
          Search the range
        </button>
        {api.searching && <span className="sweep-detail" role="status">Searching…</span>}
      </div>

      {'error' in built && (
        <p className={built.stated ? 'field-issue-text' : 'field-hint'} role="status">
          {built.error}
        </p>
      )}

      {searched === null ? (
        <p>Nothing has been searched yet.</p>
      ) : (
        <SearchAnswer
          api={api}
          response={searched.response}
          stale={api.staleSearch}
          answersThis={answersThis}
        />
      )}
    </Section>
  )
}

function SearchAnswer(props: {
  api: WorksheetApi
  response: PowerSearchResponse
  stale: boolean
  answersThis: boolean
}): ReactNode {
  const { api, response } = props
  const driver = response.settings.driver

  return (
    <div className="search-answer">
      {props.stale && (
        <p className="notice" role="status">
          This search describes an earlier revision of the design. Ask again to bring it up
          to date; nothing in it has been applied.
        </p>
      )}
      {!props.answersThis && (
        <p className="notice" role="status">
          The controls have moved on since this was searched. It still answers the question
          it was asked, which is not the one now selected.
        </p>
      )}
      <p className="sweep-detail">{response.detail}</p>
      <p className="sweep-detail">
        Solved from {response.solveMode}. Held fixed: {response.heldFixed.length === 0
          ? 'nothing else is stated'
          : response.heldFixed.join(', ')}.
      </p>

      {response.intervals.length === 0 ? (
        <p>
          No evaluated candidate is feasible, so there is no interval to report. That is a
          statement about this range at this resolution and not about every wing.
        </p>
      ) : (
        <ol className="intervals">
          {response.intervals.map((interval) => (
            <li key={`${String(interval.first.value)}-${String(interval.last.value)}`}>
              <strong>
                {quantityText(interval.first)} to {quantityText(interval.last)}
              </strong>
              <p className="detail">{interval.detail}</p>
            </li>
          ))}
        </ol>
      )}

      <table className="sweep-table">
        <caption>
          Every evaluated candidate, with the largest electrical power any one mission
          segment holds continuously.
        </caption>
        <thead>
          <tr>
            <th scope="col">{labelFor(driver)}</th>
            <th scope="col">Continuous demand</th>
            <th scope="col">Outcome</th>
            <th scope="col">Adopt</th>
          </tr>
        </thead>
        <tbody>
          {response.candidates.map((candidate, n) => (
            <tr key={`${String(candidate.driver.value)}-${String(n)}`}>
              <th scope="row">{quantityText(candidate.driver)}</th>
              <td>{candidate.status === 'computed' ? quantityText(candidate.demand) : '—'}</td>
              <td>{outcomeText(candidate)}</td>
              <td>
                <button
                  type="button"
                  className="inline"
                  onClick={() => {
                    void api.run([{
                      kind: 'set-driver',
                      key: driver,
                      value: { value: candidate.driver.value, unit: candidate.driver.unit },
                    }])
                  }}
                >
                  Use {quantityText(candidate.driver)}
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      <p className="panel-note">
        Adopting a candidate is the ordinary driver edit: the same command, the same kind of
        revision, undone in one step. Every candidate offered here was evaluated; none of
        them was solved for, and a feasible wing could lie between two of them or outside
        the range entirely.
      </p>
    </div>
  )
}
