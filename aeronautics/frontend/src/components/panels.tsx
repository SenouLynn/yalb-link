import { useState } from 'react'
import type { ReactNode } from 'react'
import type { Case, Command, Quantity, Requirement } from '../api/contract.ts'
import { DRIVER_KEYS, JOURNEYS, maximumArea, maximumSpan, requirementNamed } from '../state/design.ts'
import { displayNumber } from '../state/fields.ts'
import type { WorksheetApi } from '../useWorksheet.ts'
import { QuantityField, Section, TextField, inUnit } from './fields.tsx'

/** issueFor finds the service's complaint about one field, if there is one. */
function issueFor(api: WorksheetApi, field: string): string | undefined {
  const failure = api.worksheet.failure
  if (failure === null) return undefined
  const issue = failure.issues.find((entry) => entry.field === field || entry.field.endsWith(`.${field}`))
  return issue?.detail
}

/** definitionIssue finds a structural complaint the evaluation reported. */
function definitionIssue(api: WorksheetApi, field: string): string | undefined {
  const current = api.worksheet.current
  if (current === null) return undefined
  const all = [
    ...current.evaluation.definitionIssues,
    ...current.evaluation.geometryIssues,
  ]
  return all.find((issue) => issue.field === field)?.detail
}

export function JourneyPanel(props: { api: WorksheetApi }): ReactNode {
  const { api } = props
  const chosen = api.worksheet.journey
  const description = JOURNEYS.find((entry) => entry.id === chosen)
  return (
    <Section title="Where are you starting from?">
      <div className="journeys" role="group" aria-label="Entry point">
        {JOURNEYS.map((entry) => (
          <button
            key={entry.id}
            type="button"
            className={`journey${entry.id === chosen ? ' journey-chosen' : ''}`}
            aria-pressed={entry.id === chosen}
            onClick={() => { api.selectJourney(entry.id) }}
          >
            <span className="journey-title">{entry.title}</span>
            <span className="journey-from">{entry.startsFrom}</span>
          </button>
        ))}
      </div>
      {description === undefined ? (
        <p>
          Pick the one that matches what you already know. They all edit the same design, so
          nothing is lost by changing your mind, and none of them is a wizard you have to
          finish.
        </p>
      ) : (
        <p className="journey-next">{description.next}</p>
      )}
    </Section>
  )
}

export function MassPanel(props: { api: WorksheetApi }): ReactNode {
  const { api, api: { design } } = props
  return (
    <Section title="Mass">
      <QuantityField
        id="mass"
        label="All-up mass"
        dimension="mass"
        value={design.mass ?? null}
        role="driver"
        api={api}
        issue={issueFor(api, 'mass') ?? definitionIssue(api, 'mass')}
        hint="Everything that leaves the ground: airframe, battery, avionics and payload."
        onCommit={(target) => {
          if (target === null) {
            void api.run([{ kind: 'set-mass', mass: null, basis: design.massBasis }], ['mass'])
            return
          }
          void api.run(
            [{ kind: 'set-mass', mass: { value: target.value, unit: target.unit }, basis: design.massBasis }],
            ['mass'],
          )
        }}
      />
      <TextField
        id="mass-basis"
        label="Where it comes from"
        value={design.massBasis}
        api={api}
        hint="A target and a measurement are different things, so the mass says which it is."
        issue={issueFor(api, 'mass')}
        onCommit={(basis) => {
          void api.run([{ kind: 'set-mass', mass: design.mass ?? null, basis }], ['mass-basis'])
        }}
      />
    </Section>
  )
}

const SIZE_FIELDS = [
  { key: DRIVER_KEYS.span, id: 'span', label: 'Span', dimension: 'length' },
  { key: DRIVER_KEYS.area, id: 'area', label: 'Wing area', dimension: 'area' },
  { key: DRIVER_KEYS.aspectRatio, id: 'aspect-ratio', label: 'Aspect ratio', dimension: 'ratio' },
  { key: DRIVER_KEYS.rootChord, id: 'root-chord', label: 'Root chord', dimension: 'length' },
] as const

/**
 * SelectionMark is the other half of the dimension-to-field link. A dimension
 * on a drawing and the field that drives it are one thing seen twice, so the
 * field says when it is the selected one and can select itself.
 *
 * The selection lives in the worksheet state rather than in either component,
 * which is what lets the drawing and the worksheet sit in different columns and
 * still agree.
 */
function SelectionMark(props: { api: WorksheetApi; parameterKey: string }): ReactNode {
  const { api, parameterKey } = props
  const selected = api.selection === parameterKey
  return (
    <button
      type="button"
      className={`inline selection-mark${selected ? ' selected' : ''}`}
      aria-pressed={selected}
      aria-label={`Show the drawing and formula for ${parameterKey}`}
      onClick={() => { api.select(selected ? null : parameterKey) }}
    >
      {selected ? 'On the drawing ▸' : 'On the drawing'}
    </button>
  )
}

const PLANFORM_DRIVERS = 2

export function WingPanel(props: { api: WorksheetApi }): ReactNode {
  const { api, api: { design } } = props
  const evaluation = api.worksheet.current?.evaluation ?? null
  const solved = evaluation?.wing ?? null
  // Which values are inputs is read from the design rather than from the last
  // result: the design is authoritative and is there even when the wing has not
  // solved, which is exactly when the first two values are being entered.
  const held = SIZE_FIELDS.filter((field) => driverValue(api, field.key) !== null)

  function solvedValue(key: string): Quantity | null {
    return solved?.parameters.find((parameter) => parameter.key === key)?.value ?? null
  }

  return (
    <Section
      title="Wing"
      summary={solved === null ? 'not solved' : `solved from ${solved.solveMode}`}
    >
      <p className="panel-note">
        A planform holds exactly two of these. The other two follow from them, stay readable
        and copyable, and say what they were computed from. “Use as input” hands one of them
        the role and gives up another in the same edit.
      </p>
      {SIZE_FIELDS.map((field) => {
        const driver = driverValue(api, field.key)
        const isDriver = driver !== null
        // Below two drivers there is an empty slot to fill, so every field
        // accepts an entry; at two, the others are derived and are promoted
        // deliberately instead.
        const editable = isDriver || held.length < PLANFORM_DRIVERS
        const value = isDriver ? driver : solvedValue(field.key)
        return (
          <QuantityField
            key={field.id}
            id={field.id}
            label={field.label}
            dimension={field.dimension}
            value={value}
            role={isDriver ? 'driver' : 'derived'}
            api={api}
            readOnly={!editable}
            issue={issueFor(api, field.key) ?? issueFor(api, field.id)}
            {...(isDriver ? {} : hintProp(derivedHint(api, field.key)))}
            actions={
              <>
                <SelectionMark api={api} parameterKey={field.key} />
                {!isDriver && !editable && (
                  <UseAsInput api={api} promote={field.key} value={value} unit={field.dimension} />
                )}
              </>
            }
            onCommit={(target) => {
              // An empty field withdraws the value rather than meaning zero,
              // so the command carries no value at all.
              void api.run(
                [
                  target === null
                    ? { kind: 'set-driver', key: field.key }
                    : {
                        kind: 'set-driver',
                        key: field.key,
                        value: { value: target.value, unit: target.unit },
                      },
                ],
                [field.id],
              )
            }}
          />
        )
      })}
      <QuantityField
        id="body-width"
        label="Body width at the wing"
        dimension="length"
        value={design.wing.bodyWidth ?? null}
        role="driver"
        api={api}
        issue={issueFor(api, 'body_width')}
        hint="Optional. With it the exposed area outside the body is reported, and the body sides are drawn on the plan view; without it neither is claimed."
        onCommit={(target) => {
          void api.run(
            [{
              kind: 'set-body-width',
              value: target === null ? null : { value: target.value, unit: target.unit },
            }],
            ['body-width'],
          )
        }}
      />
      <ShapeControls api={api} />
      <AngleControls api={api} />
    </Section>
  )
}

function driverValue(api: WorksheetApi, key: string): Quantity | null {
  const wing = api.design.wing
  switch (key) {
    case DRIVER_KEYS.span:
      return wing.span ?? null
    case DRIVER_KEYS.area:
      return wing.area ?? null
    case DRIVER_KEYS.rootChord:
      return wing.rootChord ?? null
    case DRIVER_KEYS.aspectRatio:
      return wing.aspectRatio === 0 ? null : { value: wing.aspectRatio, unit: '1' }
    default:
      return null
  }
}

// hintProp spreads a hint only when there is one. exactOptionalPropertyTypes
// distinguishes an absent property from one set to undefined, and the field
// component takes the first.
function hintProp(hint: string | undefined): { hint?: string } {
  return hint === undefined ? {} : { hint }
}

function derivedHint(api: WorksheetApi, key: string): string | undefined {
  const parameter = api.worksheet.current?.evaluation.wing?.parameters.find((p) => p.key === key)
  if (parameter?.role !== 'derived') return undefined
  const from = (parameter.dependsOn ?? []).join(', ')
  const equationId = parameter.equationId ?? ''
  const equation = equationId === '' ? 'the solve' : equationId
  const revision = parameter.revision ?? 'unrecorded'
  return from === ''
    ? `Follows from ${equation}.`
    : `Follows from ${equation} (revision ${revision}), using ${from}.`
}

/**
 * UseAsInput promotes a derived value to a driver. The swap is the Task 04
 * command, so the release is chosen deliberately rather than inferred: with two
 * drivers held, either could go, and the worksheet asks which.
 */
function UseAsInput(props: {
  api: WorksheetApi
  promote: string
  value: Quantity | null
  unit: string
}): ReactNode {
  const { api } = props
  const [asking, setAsking] = useState(false)
  const held = api.worksheet.current?.evaluation.wing?.drivers ?? []
  if (props.value === null) return null

  if (!asking) {
    return (
      <button type="button" className="inline" onClick={() => { setAsking(true) }}>
        Use as input
      </button>
    )
  }
  return (
    <span className="swap" role="group" aria-label="Choose which input to give up">
      <span>Give up:</span>
      {held.map((release) => (
        <button
          key={release}
          type="button"
          className="inline"
          // The bare key is what a builder is choosing between, but it is not a
          // name on its own: the parameter table names the same keys, and a
          // button called only "wing.area.reference" does not say what pressing
          // it would do.
          aria-label={`Give up ${release}`}
          onClick={() => {
            setAsking(false)
            const value = props.value
            if (value === null) return
            void api.run([
              {
                kind: 'promote-driver',
                promote: props.promote,
                release,
                value: { value: value.value, unit: value.unit },
              },
            ])
          }}
        >
          {release}
        </button>
      ))}
      <button type="button" className="inline" onClick={() => { setAsking(false) }}>
        Cancel
      </button>
    </span>
  )
}

function ShapeControls(props: { api: WorksheetApi }): ReactNode {
  const { api, api: { design } } = props
  const wing = design.wing
  return (
    <div className="field">
      <span className="field-label" id="shape-label">Plan shape</span>
      <div className="field-entry" role="group" aria-labelledby="shape-label">
        <select
          className="field-input"
          aria-label="Plan shape"
          value={wing.shape}
          onChange={(event) => {
            const shape = event.target.value
            void api.run([
              { kind: 'set-planform-shape', shape, ratio: shape === 'rectangle' ? 1 : 0.5 },
            ])
          }}
        >
          <option value="rectangle">Rectangle</option>
          <option value="trapezoid">Tapered trapezoid</option>
        </select>
      </div>
      {wing.shape === 'trapezoid' && (
        <QuantityField
          id="taper-ratio"
          label="Taper ratio"
          dimension="ratio"
          value={wing.taperRatio === 0 ? null : { value: wing.taperRatio, unit: '1' }}
          role="driver"
          api={api}
          issue={issueFor(api, 'taper_ratio')}
          hint="Tip chord over root chord. A pointed tip is not supported."
          onCommit={(target) => {
            if (target === null) return
            void api.run(
              [{ kind: 'set-planform-shape', shape: 'trapezoid', ratio: target.value }],
              ['taper-ratio'],
            )
          }}
        />
      )}
    </div>
  )
}

const ANGLES = [
  { id: 'sweep', label: 'Sweep at the quarter chord' },
  { id: 'dihedral', label: 'Dihedral' },
  { id: 'twist', label: 'Twist, root to tip' },
  { id: 'incidence', label: 'Root incidence' },
] as const

function AngleControls(props: { api: WorksheetApi }): ReactNode {
  const { api, api: { design } } = props
  const wing = design.wing

  function angle(id: (typeof ANGLES)[number]['id']): Quantity | null {
    switch (id) {
      case 'sweep':
        return wing.sweep ?? null
      case 'dihedral':
        return wing.dihedral ?? null
      case 'twist':
        return wing.twist ?? null
      default:
        return wing.incidence ?? null
    }
  }

  function commit(id: string, value: Quantity | null): void {
    const next = {
      sweep: id === 'sweep' ? value : wing.sweep ?? null,
      dihedral: id === 'dihedral' ? value : wing.dihedral ?? null,
      twist: id === 'twist' ? value : wing.twist ?? null,
      incidence: id === 'incidence' ? value : wing.incidence ?? null,
      sweepReference: wing.sweepReference,
      dihedralMode: wing.dihedralMode ?? '',
    }
    void api.run([{ kind: 'set-wing-angles', angles: next }], [id])
  }

  return (
    <details className="subsection">
      <summary>Angles</summary>
      <p className="panel-note">
        Every angle is stated, with zero spelled out. A later handling model cannot tell an
        unswept wing from an unrecorded one, so there is no such thing as leaving one blank.
      </p>
      {ANGLES.map((entry) => (
        <QuantityField
          key={entry.id}
          id={entry.id}
          label={entry.label}
          dimension="angle"
          value={angle(entry.id)}
          role="driver"
          api={api}
          issue={issueFor(api, entry.id) ?? definitionIssue(api, entry.id)}
          onCommit={(target) => {
            commit(entry.id, target === null ? null : { value: target.value, unit: target.unit })
          }}
        />
      ))}
      {(wing.dihedral?.value ?? 0) !== 0 && (
        <div className="field">
          <label className="field-label" htmlFor="dihedral-mode">
            What stays fixed as the dihedral changes
          </label>
          <div className="field-entry">
            <select
              id="dihedral-mode"
              className="field-input"
              value={wing.dihedralMode ?? ''}
              onChange={(event) => {
                void api.run([
                  {
                    kind: 'set-wing-angles',
                    angles: {
                      sweep: wing.sweep ?? null,
                      dihedral: wing.dihedral ?? null,
                      twist: wing.twist ?? null,
                      incidence: wing.incidence ?? null,
                      sweepReference: wing.sweepReference,
                      dihedralMode: event.target.value,
                    },
                  },
                ])
              }}
            >
              <option value="">Choose one</option>
              <option value="hold-panel">The panel you build</option>
              <option value="hold-projected">The plan-view reference</option>
            </select>
          </div>
          <p className="field-hint">
            The two are different aircraft: holding the panel foreshortens the plan view, and
            holding the plan view stretches the panel.
          </p>
        </div>
      )}
    </details>
  )
}

export function ConfigurationPanel(props: { api: WorksheetApi }): ReactNode {
  const { api, api: { design } } = props
  return (
    <Section title="Layout">
      <div className="field">
        <label className="field-label" htmlFor="configuration">Configuration</label>
        <div className="field-entry">
          <select
            id="configuration"
            className="field-input"
            value={design.configuration}
            onChange={(event) => {
              const configuration = event.target.value
              // A flying wing carries no tail, so the description goes in the
              // same edit rather than leaving the design in a state that is
              // neither layout.
              void api.run([
                configuration === 'flying-wing'
                  ? { kind: 'set-configuration', configuration }
                  : { kind: 'set-configuration', configuration, tail: design.tail ?? {} },
              ])
            }}
          >
            <option value="conventional-tail">Conventional tail</option>
            <option value="v-tail">V-tail</option>
            <option value="flying-wing">Flying wing</option>
          </select>
        </div>
      </div>
      <p className="panel-note">
        Handling is unknown for all three. Nothing here predicts trim, static margin or
        control authority, and an incomplete tail does not stop the wing being sized.
      </p>
    </Section>
  )
}

export function CasesPanel(props: { api: WorksheetApi }): ReactNode {
  const { api, api: { design } } = props
  const cases = design.cases ?? []

  function replace(original: Case, changes: Partial<Case>): Command {
    return { kind: 'set-case', case: { ...original, ...changes } }
  }

  return (
    <Section title="Flight cases">
      <p className="panel-note">
        A result without its condition is not a result. Each case states the air it assumes
        and where its lift coefficient came from; only required cases narrow the bounds.
      </p>
      {cases.map((entry) => (
        <div key={entry.name} className="case">
          <h3>{entry.name}</h3>
          <QuantityField
            id={`case-${entry.name}-clmax`}
            label="Whole-aircraft CLmax"
            dimension="ratio"
            value={entry.clmax.max === 0 ? null : { value: entry.clmax.max, unit: '1' }}
            role="driver"
            api={api}
            issue={issueFor(api, 'clmax') ?? definitionIssue(api, 'clmax')}
            hint="A 2D section clmax is refused here: it is not the same number."
            onCommit={(target) => {
              void api.run(
                [replace(entry, { clmax: { ...entry.clmax, max: target?.value ?? 0 } })],
                [`case-${entry.name}-clmax`],
              )
            }}
          />
          <TextField
            id={`case-${entry.name}-clmax-basis`}
            label="Where the coefficient comes from"
            value={entry.clmax.basis}
            api={api}
            issue={issueFor(api, 'clmax')}
            onCommit={(basis) => {
              void api.run(
                [replace(entry, { clmax: { ...entry.clmax, basis } })],
                [`case-${entry.name}-clmax-basis`],
              )
            }}
          />
          <div className="field">
            <label className="field-label" htmlFor={`case-${entry.name}-evidence`}>
              How good that is
            </label>
            <div className="field-entry">
              <select
                id={`case-${entry.name}-evidence`}
                className="field-input"
                value={entry.clmax.evidence ?? 'assumed'}
                onChange={(event) => {
                  void api.run([replace(entry, { clmax: { ...entry.clmax, evidence: event.target.value } })])
                }}
              >
                <option value="assumed">Assumed</option>
                <option value="measured">Measured</option>
                <option value="simulated">Simulated</option>
              </select>
            </div>
          </div>
          <QuantityField
            id={`case-${entry.name}-n`}
            label="Load factor"
            dimension="ratio"
            value={{ value: entry.loadFactor, unit: '1' }}
            role="driver"
            api={api}
            onCommit={(target) => {
              void api.run(
                [replace(entry, { loadFactor: target?.value ?? 0 })],
                [`case-${entry.name}-n`],
              )
            }}
          />
          <div className="field">
            <label className="field-label" htmlFor={`case-${entry.name}-priority`}>
              Priority for {entry.name}
            </label>
            <div className="field-entry">
              <select
                id={`case-${entry.name}-priority`}
                className="field-input"
                value={entry.priority}
                onChange={(event) => {
                  void api.run([
                    { kind: 'set-case-priority', name: entry.name, priority: event.target.value },
                  ])
                }}
              >
                <option value="required">Required</option>
                <option value="preferred">Preferred</option>
              </select>
            </div>
            <p className="field-hint">
              A preferred case is still assessed. It stops narrowing the required bounds.
            </p>
          </div>
        </div>
      ))}
      <button
        type="button"
        onClick={() => {
          const name = `Manoeuvre ${String((design.cases?.length ?? 0) + 1)}`
          const template = cases[0]
          if (!template) return
          void api.run([
            { kind: 'set-case', case: { ...template, name, loadFactor: 2 } },
            { kind: 'set-requirement', requirement: withCase(design.requirements ?? [], name) },
          ])
        }}
      >
        Add a manoeuvre case at n = 2
      </button>
    </Section>
  )
}

function withCase(requirements: readonly Requirement[], name: string): Requirement {
  const stall = requirements.find((requirement) => requirement.subject === 'stall-speed')
  if (!stall) {
    return {
      name: 'Stall ceiling', subject: 'stall-speed', priority: 'required', basis: '',
      cases: [name], minimum: null, maximum: null, margin: 0,
    }
  }
  return { ...stall, cases: [...(stall.cases ?? []), name] }
}

export function RequirementsPanel(props: { api: WorksheetApi }): ReactNode {
  const { api, api: { design } } = props
  const stall = requirementNamed(design, 'Stall ceiling')
  const spanLimit = requirementNamed(design, 'Maximum span')
  const areaLimit = requirementNamed(design, 'Maximum wing area')
  const firstCase = (design.cases ?? [])[0]

  return (
    <Section title="Requirements">
      <p className="panel-note">
        A requirement is a bound, not a value. A maximum span does not set the span; using
        the whole allowance is a decision you make.
      </p>

      {stall !== null && (
        <>
          <QuantityField
            id="stall-ceiling"
            label="Highest acceptable stall speed"
            dimension="speed"
            value={stall.maximum ?? null}
            api={api}
            issue={issueFor(api, 'requirement.Stall ceiling')}
            hint={`Applies to ${(stall.cases ?? []).join(', ')}.`}
            onCommit={(target) => {
              void api.run(
                [{
                  kind: 'set-requirement',
                  requirement: {
                    ...stall,
                    maximum: target === null ? null : { value: target.value, unit: target.unit },
                  },
                }],
                ['stall-ceiling'],
              )
            }}
          />
          <TextField
            id="stall-basis"
            label="Where that comes from"
            value={stall.basis}
            api={api}
            onCommit={(basis) => {
              void api.run(
                [{ kind: 'set-requirement', requirement: { ...stall, basis } }],
                ['stall-basis'],
              )
            }}
          />
          <div className="field">
            <label className="field-label" htmlFor="stall-priority">
              Priority for the stall ceiling
            </label>
            <div className="field-entry">
              <select
                id="stall-priority"
                className="field-input"
                value={stall.priority}
                onChange={(event) => {
                  void api.run([
                    { kind: 'set-requirement-priority', name: stall.name, priority: event.target.value },
                  ])
                }}
              >
                <option value="required">Required</option>
                <option value="preferred">Preferred</option>
              </select>
            </div>
          </div>
          <SizeAtStallLimit api={api} caseName={firstCase?.name ?? ''} />
        </>
      )}

      <QuantityField
        id="max-area"
        label="Maximum wing area"
        dimension="area"
        value={areaLimit?.maximum ?? null}
        api={api}
        hint="Sheet stock, a storage box or a transport case. Like the span limit, it bounds the wing rather than setting it."
        onCommit={(target) => {
          const base = areaLimit ?? maximumArea()
          void api.run(
            [{
              kind: 'set-requirement',
              requirement: {
                ...base,
                basis: base.basis === '' ? 'a size limit outside aerodynamics' : base.basis,
                maximum: target === null ? null : { value: target.value, unit: target.unit },
              },
            }],
            ['max-area'],
          )
        }}
      />

      <QuantityField
        id="max-span"
        label="Maximum span"
        dimension="length"
        value={spanLimit?.maximum ?? null}
        api={api}
        hint="A doorway, a car boot or a contest rule. It stays separate from the span you use."
        actions={<UseMaximumSpan api={api} limit={spanLimit?.maximum ?? null} />}
        onCommit={(target) => {
          const base = spanLimit ?? maximumSpan()
          void api.run(
            [{
              kind: 'set-requirement',
              requirement: {
                ...base,
                basis: base.basis === '' ? 'a size limit outside aerodynamics' : base.basis,
                maximum: target === null ? null : { value: target.value, unit: target.unit },
              },
            }],
            ['max-span'],
          )
        }}
      />
    </Section>
  )
}

/**
 * UseMaximumSpan copies the limit into the span, once, when asked. Nothing
 * recopies it on later edits: the boundary the builder chose to sit on is a
 * decision, and a decision that reasserts itself is not one.
 */
function UseMaximumSpan(props: { api: WorksheetApi; limit: Quantity | null }): ReactNode {
  const { api, limit } = props
  if (limit === null) return null
  const shown = inUnit(limit, limit.unit, api.units) ?? limit.value
  return (
    <button
      type="button"
      className="inline"
      onClick={() => {
        void api.run([
          { kind: 'set-driver', key: DRIVER_KEYS.span, value: { value: limit.value, unit: limit.unit } },
        ])
      }}
    >
      Use maximum ({displayNumber(shown)} {limit.unit})
    </button>
  )
}

function SizeAtStallLimit(props: { api: WorksheetApi; caseName: string }): ReactNode {
  const { api } = props
  const held = api.worksheet.current?.evaluation.wing?.drivers ?? []
  const holdable = held.filter((driver) => driver !== DRIVER_KEYS.area && driver !== DRIVER_KEYS.areaPanel)
  if (holdable.length === 0) return null
  return (
    <div className="action">
      <span>Size the wing at the stall limit, keeping:</span>
      {holdable.map((hold) => (
        <span key={hold} className="action-pair">
          <button
            type="button"
            className="inline"
            onClick={() => {
              void api.run([
                { kind: 'size-at-stall-limit', hold, scope: { kind: 'all-required' } },
              ])
            }}
          >
            {hold}, all required cases
          </button>
          {props.caseName !== '' && (
            <button
              type="button"
              className="inline"
              onClick={() => {
                void api.run([
                  {
                    kind: 'size-at-stall-limit',
                    hold,
                    scope: { kind: 'single', case: props.caseName },
                  },
                ])
              }}
            >
              {hold}, {props.caseName} only
            </button>
          )}
        </span>
      ))}
      <p className="field-hint">
        Sizing against one case is a different action from sizing against all of them, so the
        scope is named rather than assumed. Every requirement is reassessed either way.
      </p>
    </div>
  )
}

export function DraftsPanel(props: { api: WorksheetApi }): ReactNode {
  const { api } = props
  const [name, setName] = useState('')
  return (
    <Section title="Drafts" defaultOpen={false}>
      <div className="field">
        <label className="field-label" htmlFor="draft-name">Name</label>
        <div className="field-entry">
          <input
            id="draft-name"
            className="field-input field-input-wide"
            value={name}
            autoComplete="off"
            onChange={(event) => { setName(event.target.value) }}
          />
          <button
            type="button"
            className="inline"
            disabled={name.trim() === ''}
            onClick={() => { api.saveDraft(name.trim()) }}
          >
            Save
          </button>
        </div>
        <p className="field-hint">
          A draft holds the inputs, the roles, the requirements and anything half-typed. It
          records which equations produced its numbers rather than the numbers themselves, so
          reopening it recalculates instead of believing a cached answer.
        </p>
      </div>
      {api.draftNames.length > 0 && (
        <ul className="drafts">
          {api.draftNames.map((saved) => (
            <li key={saved}>
              {saved}{' '}
              <button type="button" className="inline" onClick={() => { api.loadDraft(saved) }}>
                Open
              </button>
            </li>
          ))}
        </ul>
      )}
    </Section>
  )
}
