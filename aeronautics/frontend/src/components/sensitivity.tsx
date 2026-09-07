import { useMemo, useState } from 'react'
import type { ReactNode } from 'react'
import type { Quantity, SweepResponse, SweepSample, SweepSettings } from '../api/contract.ts'
import { commitText, displayNumber } from '../state/fields.ts'
import {
  DEFAULT_SAMPLES,
  defaultRange,
  driverValue,
  settingsKey,
  sweepableDrivers,
} from '../state/sweep.ts'
import type { WorksheetApi } from '../useWorksheet.ts'
import { Section } from './fields.tsx'

// The sensitivity view.
//
// Every number plotted here came back from the service, which evaluated each
// candidate through the same workflow an ordinary result goes through. This
// file chooses the question and draws the answer.
//
// Three rules shape it. A curve is meaningless without its driver mode, so what
// was held fixed is stated in words next to the plot. A sample the model could
// not evaluate is a gap, and no line is drawn through it. And sampling commits
// nothing: selecting a candidate here changes what is described, never what the
// design holds.

/** OUTPUTS are the quantities a sweep can plot, with the units they carry. */
const OUTPUTS = [
  { subject: 'stall-speed', label: 'Stall speed', perCase: true },
  { subject: 'wing-area', label: 'Wing area', perCase: false },
  { subject: 'span', label: 'Span', perCase: false },
  { subject: 'aspect-ratio', label: 'Aspect ratio', perCase: false },
  { subject: 'mass-wing-loading', label: 'Wing loading, by mass', perCase: false },
  { subject: 'mass', label: 'All-up mass', perCase: false },
] as const

/** DRIVER_LABELS name the sweepable drivers for a reader. */
const DRIVER_LABELS: Readonly<Record<string, string>> = {
  'design.mass': 'All-up mass',
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

/** feasibilityText says in words whether a candidate meets its requirements. */
function feasibilityText(sample: SweepSample): string {
  if (sample.status !== 'computed') return 'not computed'
  if (!sample.hasRequired) return 'no required requirement'
  switch (sample.feasibility) {
    case 'met':
      return 'meets every required requirement'
    case 'unmet':
      return 'misses a required requirement'
    default:
      return 'feasibility unknown'
  }
}

/**
 * MARKERS give each outcome its own shape as well as its own place on the page.
 * Colour carries no information here: a plot that can only be read by seeing
 * its colours is not readable at all.
 */
const MARKERS: Readonly<Record<string, string>> = {
  met: '●',
  unmet: '✕',
  unknown: '◇',
}

function markerFor(sample: SweepSample): string {
  if (sample.status !== 'computed') return '·'
  if (!sample.hasRequired) return MARKERS['unknown'] ?? '◇'
  return MARKERS[sample.feasibility] ?? '◇'
}

interface PlotGeometry {
  readonly width: number
  readonly height: number
  x(value: number): number
  y(value: number): number
}

const PLOT_WIDTH = 420
const PLOT_HEIGHT = 220
const PLOT_MARGIN = 38

/**
 * plotGeometry maps values onto the canvas. It is presentation arithmetic: it
 * scales numbers the service produced onto pixels and computes nothing else.
 */
function plotGeometry(response: SweepResponse): PlotGeometry {
  const drivers = response.samples.map((sample) => sample.driver.value)
  const values: number[] = []
  for (const sample of response.samples) {
    if (sample.value) values.push(sample.value.value)
  }
  if (response.current.value) values.push(response.current.value.value)
  for (const bound of response.bounds) values.push(bound.value.value)
  const minX = Math.min(...drivers)
  const maxX = Math.max(...drivers)
  const minY = values.length === 0 ? 0 : Math.min(...values)
  const maxY = values.length === 0 ? 1 : Math.max(...values)
  const spanX = Math.max(maxX - minX, 1e-9)
  const spanY = Math.max(maxY - minY, Math.abs(maxY) * 1e-6, 1e-9)
  const usableWidth = PLOT_WIDTH - PLOT_MARGIN * 2
  const usableHeight = PLOT_HEIGHT - PLOT_MARGIN * 2
  return {
    width: PLOT_WIDTH,
    height: PLOT_HEIGHT,
    x: (value) => PLOT_MARGIN + ((value - minX) / spanX) * usableWidth,
    y: (value) => PLOT_HEIGHT - PLOT_MARGIN - ((value - minY) / spanY) * usableHeight,
  }
}

/**
 * segments splits the samples into runs of computed points. A gap is a gap: a
 * line joining the two sides of one would draw a curve through candidates the
 * model refused to evaluate.
 */
function segments(samples: readonly SweepSample[]): SweepSample[][] {
  const runs: SweepSample[][] = []
  let run: SweepSample[] = []
  for (const sample of samples) {
    if (sample.status === 'computed' && sample.value) {
      run.push(sample)
      continue
    }
    if (run.length > 0) runs.push(run)
    run = []
  }
  if (run.length > 0) runs.push(run)
  return runs
}

function Plot(props: {
  response: SweepResponse
  selected: number | null
  onSelect(index: number): void
}): ReactNode {
  const { response, selected } = props
  const geometry = plotGeometry(response)

  return (
    <svg
      className="sweep-plot"
      viewBox={`0 0 ${String(geometry.width)} ${String(geometry.height)}`}
      role="img"
      aria-label={`${labelFor(response.settings.driver)} against ${response.settings.output.subject}. ${response.detail}`}
    >
      {response.bounds.map((bound) => (
        <g key={`${bound.name}-${bound.direction}`} className="sweep-bound">
          <line
            x1={PLOT_MARGIN}
            y1={geometry.y(bound.value.value)}
            x2={geometry.width - PLOT_MARGIN}
            y2={geometry.y(bound.value.value)}
          />
          <text x={PLOT_MARGIN} y={geometry.y(bound.value.value) - 4}>
            {bound.name} ({bound.direction})
          </text>
        </g>
      ))}
      {segments(response.samples).map((run, n) => (
        <polyline
          key={`run-${String(n)}`}
          className="sweep-curve"
          fill="none"
          points={run
            .map((sample) =>
              `${String(geometry.x(sample.driver.value))},${String(geometry.y(sample.value?.value ?? 0))}`)
            .join(' ')}
        />
      ))}
      {response.samples.map((sample, n) =>
        sample.status === 'computed' && sample.value ? (
          <text
            key={`sample-${String(n)}`}
            className={`sweep-sample${selected === n ? ' selected' : ''}`}
            x={geometry.x(sample.driver.value)}
            y={geometry.y(sample.value.value)}
            textAnchor="middle"
            dominantBaseline="middle"
            onClick={() => { props.onSelect(n) }}
          >
            {markerFor(sample)}
          </text>
        ) : null,
      )}
      {response.current.value && (
        <g className="sweep-current">
          <line
            x1={geometry.x(response.current.driver.value)}
            y1={PLOT_MARGIN}
            x2={geometry.x(response.current.driver.value)}
            y2={geometry.height - PLOT_MARGIN}
          />
          <text x={geometry.x(response.current.driver.value)} y={PLOT_MARGIN - 6} textAnchor="middle">
            this design
          </text>
        </g>
      )}
    </svg>
  )
}

interface Draft {
  readonly driver: string
  readonly from: string
  readonly to: string
  readonly samples: string
  readonly subject: string
  readonly caseName: string
}

function buildSettings(draft: Draft, unit: string): SweepSettings | { error: string } {
  const from = commitText(draft.from)
  const to = commitText(draft.to)
  if (from.kind !== 'value') return { error: 'The start of the range is not a number.' }
  if (to.kind !== 'value') return { error: 'The end of the range is not a number.' }
  const samples = Number(draft.samples)
  if (!Number.isInteger(samples)) return { error: 'The sample count is not a whole number.' }
  return {
    driver: draft.driver,
    from: { value: from.value, unit },
    to: { value: to.value, unit },
    samples,
    output: { subject: draft.subject, case: draft.caseName },
  }
}

/**
 * SensitivityPanel lets a builder move one driver across a range and see what
 * follows. It is the one-driver subset the task documents: the book's matching
 * plot needs the power models Task 09 brings, and nothing here is presented as
 * one.
 */
export function SensitivityPanel(props: { api: WorksheetApi }): ReactNode {
  const { api } = props
  const design = api.design
  const evaluation = api.worksheet.current?.evaluation ?? null
  const held = evaluation?.wing?.drivers ?? []
  const drivers = sweepableDrivers(design, held)
  const cases = design.cases ?? []
  const firstDriver = drivers[0] ?? ''
  const firstCase = cases[0]?.name ?? ''

  const [draft, setDraft] = useState<Draft | null>(null)
  const [selected, setSelected] = useState<number | null>(null)

  const current = driverValue(design, draft?.driver ?? firstDriver)
  const unit = current?.unit ?? '1'
  const active: Draft = draft ?? {
    driver: firstDriver,
    from: current ? displayNumber(defaultRange(current).from.value) : '',
    to: current ? displayNumber(defaultRange(current).to.value) : '',
    samples: String(DEFAULT_SAMPLES),
    subject: 'stall-speed',
    caseName: firstCase,
  }
  const built = useMemo(() => buildSettings(active, unit), [active, unit])
  const swept = api.worksheet.swept
  const answersThis =
    swept !== null && !('error' in built) && settingsKey(swept.settings) === settingsKey(built)

  function update(change: Partial<Draft>): void {
    setDraft({ ...active, ...change })
    setSelected(null)
  }

  if (drivers.length === 0) {
    return (
      <Section title="What changes if…">
        <p>
          Nothing is being driven yet. Enter two size values, or an all-up mass, and one of
          them can be moved across a range to see what follows.
        </p>
      </Section>
    )
  }

  return (
    <Section title="What changes if…">
      <p className="panel-note">
        One driver moves; everything else stays as it is. A curve is only meaningful against
        what was held fixed, so that is stated with the answer. Sampling changes nothing:
        the design still holds the value it held before.
      </p>

      <div className="sweep-controls">
        <label className="field-label" htmlFor="sweep-driver">Move</label>
        <select
          id="sweep-driver"
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

        <label className="field-label" htmlFor="sweep-from">From</label>
        <input
          id="sweep-from"
          type="text"
          inputMode="decimal"
          value={active.from}
          onChange={(event) => { update({ from: event.target.value }) }}
        />
        <label className="field-label" htmlFor="sweep-to">To</label>
        <input
          id="sweep-to"
          type="text"
          inputMode="decimal"
          value={active.to}
          onChange={(event) => { update({ to: event.target.value }) }}
        />
        <span className="field-unit-fixed">{unit === '1' ? '' : unit}</span>

        <label className="field-label" htmlFor="sweep-samples">Samples</label>
        <input
          id="sweep-samples"
          type="text"
          inputMode="numeric"
          value={active.samples}
          onChange={(event) => { update({ samples: event.target.value }) }}
        />

        <label className="field-label" htmlFor="sweep-output">Show</label>
        <select
          id="sweep-output"
          value={active.subject}
          onChange={(event) => {
            const output = OUTPUTS.find((entry) => entry.subject === event.target.value)
            update({
              subject: event.target.value,
              caseName: output?.perCase === true ? active.caseName || firstCase : '',
            })
          }}
        >
          {OUTPUTS.map((output) => (
            <option key={output.subject} value={output.subject}>{output.label}</option>
          ))}
        </select>

        {OUTPUTS.find((entry) => entry.subject === active.subject)?.perCase === true && (
          <>
            <label className="field-label" htmlFor="sweep-case">In case</label>
            <select
              id="sweep-case"
              value={active.caseName}
              onChange={(event) => { update({ caseName: event.target.value }) }}
            >
              {cases.map((entry) => (
                <option key={entry.name} value={entry.name}>{entry.name}</option>
              ))}
            </select>
          </>
        )}

        {/*
          The button stays live while a sweep is outstanding, for the reason
          Recalculate does: nothing times a request out, so refusing a second
          ask would leave one call that never returns with no way to ask again.
          Correctness belongs to the identity check — a superseded answer is
          discarded because it is not the outstanding one — rather than to
          preventing the question.
        */}
        <button
          type="button"
          onClick={() => {
            if ('error' in built) return
            void api.runSweep(built)
          }}
        >
          Show the effect
        </button>
        {api.sweeping && (
          <span className="sweep-detail" role="status">Sampling…</span>
        )}
      </div>

      {'error' in built && <p className="field-issue-text" role="status">{built.error}</p>}

      {swept === null ? (
        <p>Nothing has been sampled yet.</p>
      ) : (
        <SweepAnswer
          api={api}
          response={swept.response}
          stale={api.staleSweep}
          answersThis={answersThis}
          selected={selected}
          onSelect={setSelected}
        />
      )}
    </Section>
  )
}

function SweepAnswer(props: {
  api: WorksheetApi
  response: SweepResponse
  stale: boolean
  answersThis: boolean
  selected: number | null
  onSelect(index: number | null): void
}): ReactNode {
  const { response, selected } = props
  const chosen = selected === null ? null : response.samples[selected] ?? null

  return (
    <div className="sweep-answer">
      {props.stale && (
        <p className="notice" role="status">
          This curve describes an earlier revision of the design. Ask again to bring it up to
          date; nothing on it has been applied.
        </p>
      )}
      {!props.answersThis && (
        <p className="notice" role="status">
          The controls have moved on since this was sampled. It still answers the question it
          was asked, which is not the one now selected.
        </p>
      )}
      <p className="sweep-detail">{response.detail}</p>
      {response.invariant && response.alsoChanged.length > 0 && (
        <p className="sweep-detail">
          Also changing across this range: {response.alsoChanged.join(', ')}.
        </p>
      )}
      <Plot response={response} selected={selected} onSelect={(n) => { props.onSelect(n) }} />
      <p className="legend">
        ● meets every required requirement · ✕ misses one · ◇ unknown · a break in the line is
        a candidate the model could not evaluate.
      </p>

      <table className="sweep-table">
        <caption>
          Every sampled candidate. The same answer as the plot, in a form that does not need
          one.
        </caption>
        <thead>
          <tr>
            <th scope="col">{labelFor(response.settings.driver)}</th>
            <th scope="col">{response.settings.output.subject}</th>
            <th scope="col">Outcome</th>
          </tr>
        </thead>
        <tbody
          role="listbox"
          aria-label="Sampled candidates"
          tabIndex={0}
          onKeyDown={(event) => {
            const last = response.samples.length - 1
            const at = selected ?? 0
            if (event.key === 'ArrowDown' || event.key === 'ArrowRight') {
              event.preventDefault()
              props.onSelect(Math.min(at + 1, last))
            }
            if (event.key === 'ArrowUp' || event.key === 'ArrowLeft') {
              event.preventDefault()
              props.onSelect(Math.max(at - 1, 0))
            }
            if (event.key === 'Home') {
              event.preventDefault()
              props.onSelect(0)
            }
            if (event.key === 'End') {
              event.preventDefault()
              props.onSelect(last)
            }
            if (event.key === 'Escape') {
              event.preventDefault()
              props.onSelect(null)
            }
          }}
        >
          {response.samples.map((sample, n) => (
            <tr
              key={`${String(sample.driver.value)}-${String(n)}`}
              role="option"
              aria-selected={selected === n}
              className={selected === n ? 'selected' : undefined}
              onClick={() => { props.onSelect(n) }}
            >
              <th scope="row">{quantityText(sample.driver)}</th>
              <td>{sample.status === 'computed' ? quantityText(sample.value) : '—'}</td>
              <td>
                {markerFor(sample)} {sample.status === 'computed'
                  ? feasibilityText(sample)
                  : `not computed — ${sample.detail ?? ''}`}
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      <dl className="before-after">
        <dt>This design</dt>
        <dd>
          {quantityText(response.current.driver)} → {quantityText(response.current.value)} (
          {feasibilityText(response.current)})
        </dd>
        <dt>Selected candidate</dt>
        <dd>
          {chosen === null
            ? 'none selected; choose a row or a marker, or use the arrow keys in the table.'
            : `${quantityText(chosen.driver)} → ${quantityText(chosen.value)} (${feasibilityText(chosen)})`}
        </dd>
      </dl>
      <UseThisCandidate api={props.api} driver={response.settings.driver} sample={chosen} />
      <p className="panel-note">
        Nothing on this plot has been applied. Adopting a candidate is the ordinary driver
        edit and nothing more: it goes through the same command, records the same kind of
        revision and undoes in one step.
      </p>
      <p className="panel-note">
        The samples are the values the model produced, not a range around them. No band is
        drawn: an assumed lift coefficient is an assumption whose consequences these
        candidates show, and shading it would present it as a statistical interval it is not.
      </p>
    </div>
  )
}

/**
 * UseThisCandidate adopts a sampled candidate. Selecting one on the plot
 * describes it; this is what commits it, as an explicit edit rather than a side
 * effect of looking at it.
 */
function UseThisCandidate(props: {
  api: WorksheetApi
  driver: string
  sample: SweepSample | null
}): ReactNode {
  const { api, sample } = props
  if (sample === null) return null
  return (
    <button
      type="button"
      onClick={() => {
        const value = { value: sample.driver.value, unit: sample.driver.unit }
        void api.runSweepCandidate(props.driver, value)
      }}
    >
      Use {quantityText(sample.driver)} as the {labelFor(props.driver).toLowerCase()}
    </button>
  )
}
