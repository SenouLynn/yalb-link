import { useState } from 'react'
import type { ReactNode } from 'react'
import type { AuxiliaryLoad, Battery, Capability, DragPolar, ThrustTarget } from '../api/contract.ts'
import {
  EVIDENCE_GRADES,
  batteryOf,
  emptyCapability,
  emptyLoad,
  polarOf,
  propulsionOf,
} from '../state/power.ts'
import type { WorksheetApi } from '../useWorksheet.ts'
import { QuantityField, Section, TextField } from './fields.tsx'
import { anyIssue, issueFor } from './issues.ts'

// The power definition's entry panels.
//
// Everything here is evidence the builder supplies. Nothing on this side of the
// wire estimates a drag coefficient, a chain efficiency or a thrust: the fields
// start empty, an empty field means unstated rather than zero, and every number
// is entered next to the basis that says where it came from. A power result
// that rests on an invented coefficient would look exactly like one that rests
// on a measurement, which is the reason the basis is not optional.

/** EvidenceSelect is how good a stated value is, in the core's three grades. */
function EvidenceSelect(props: {
  id: string
  label: string
  value: string
  onChange(evidence: string): void
}): ReactNode {
  return (
    <div className="field">
      <label className="field-label" htmlFor={props.id}>{props.label}</label>
      <div className="field-entry">
        <select
          id={props.id}
          className="field-input"
          value={props.value === '' ? 'assumed' : props.value}
          onChange={(event) => { props.onChange(event.target.value) }}
        >
          {EVIDENCE_GRADES.map((grade) => (
            <option key={grade} value={grade}>
              {grade.charAt(0).toUpperCase() + grade.slice(1)}
            </option>
          ))}
        </select>
      </div>
    </div>
  )
}

/** ChoiceField is one labelled select over a fixed vocabulary. */
export function ChoiceField(props: {
  id: string
  label: string
  value: string
  choices: readonly { readonly id: string; readonly label: string }[]
  hint?: string
  placeholder?: string
  onChange(value: string): void
}): ReactNode {
  return (
    <div className="field">
      <label className="field-label" htmlFor={props.id}>{props.label}</label>
      <div className="field-entry">
        <select
          id={props.id}
          className="field-input"
          value={props.value}
          onChange={(event) => { props.onChange(event.target.value) }}
        >
          {props.placeholder !== undefined && <option value="">{props.placeholder}</option>}
          {props.choices.map((choice) => (
            <option key={choice.id} value={choice.id}>{choice.label}</option>
          ))}
        </select>
      </div>
      {props.hint !== undefined && <p className="field-hint">{props.hint}</p>}
    </div>
  )
}

/**
 * PolarPanel is the aircraft drag polar: CD = CD0 + CL²/(pi*e*AR).
 *
 * The scope is fixed at aircraft level rather than offered as a choice. A 2D
 * section polar carries no induced, interference or trim drag and is not this
 * number; the core refuses one, and a select that let a builder pick it would
 * be offering an entry that cannot be accepted.
 */
export function PolarPanel(props: { api: WorksheetApi }): ReactNode {
  const { api } = props
  const polar = polarOf(api.design)
  const held = api.design.polar != null

  function patch(changes: Partial<DragPolar>, clear: string[] = []): void {
    void api.run([{ kind: 'set-drag-polar', polar: { ...polar, ...changes } }], clear)
  }

  return (
    <Section title="Drag polar" summary={held ? 'stated' : 'not stated'}>
      <p className="panel-note">
        CD = CD0 + CL² / (π·e·AR), and D = q·S·CD. Both coefficients are aircraft-level
        evidence you supply: nothing here estimates a zero-lift drag coefficient, and the
        span efficiency is not an airfoil's. Without this panel no segment can be computed
        from the polar.
      </p>
      <QuantityField
        id="polar-cd0"
        label="Zero-lift drag coefficient CD0"
        dimension="ratio"
        value={polar.cd0 === 0 ? null : { value: polar.cd0, unit: '1' }}
        role="driver"
        api={api}
        issue={anyIssue(api, 'polar.cd0')}
        onCommit={(target) => { patch({ cd0: target?.value ?? 0 }, ['polar-cd0']) }}
      />
      <TextField
        id="polar-cd0-basis"
        label="Where CD0 comes from"
        value={polar.cd0Basis}
        api={api}
        hint="A build-up, a similar aircraft, a wind-tunnel run or a guess — say which."
        issue={anyIssue(api, 'polar.cd0_basis')}
        onCommit={(basis) => { patch({ cd0Basis: basis }, ['polar-cd0-basis']) }}
      />
      <QuantityField
        id="polar-e"
        label="Span efficiency e"
        dimension="ratio"
        value={polar.oswaldEfficiency === 0 ? null : { value: polar.oswaldEfficiency, unit: '1' }}
        role="driver"
        api={api}
        issue={anyIssue(api, 'polar.oswald_efficiency')}
        onCommit={(target) => { patch({ oswaldEfficiency: target?.value ?? 0 }, ['polar-e']) }}
      />
      <TextField
        id="polar-e-basis"
        label="Where e comes from"
        value={polar.efficiencyBasis}
        api={api}
        issue={anyIssue(api, 'polar.efficiency_basis')}
        onCommit={(basis) => { patch({ efficiencyBasis: basis }, ['polar-e-basis']) }}
      />
      <QuantityField
        id="polar-cl-min"
        label="Lowest CL the polar is claimed over"
        dimension="ratio"
        value={{ value: polar.clValidMin, unit: '1' }}
        role="driver"
        api={api}
        issue={anyIssue(api, 'polar.cl_valid_min')}
        onCommit={(target) => { patch({ clValidMin: target?.value ?? 0 }, ['polar-cl-min']) }}
      />
      <QuantityField
        id="polar-cl-max"
        label="Highest CL the polar is claimed over"
        dimension="ratio"
        value={polar.clValidMax === 0 ? null : { value: polar.clValidMax, unit: '1' }}
        role="driver"
        api={api}
        issue={anyIssue(api, 'polar.cl_valid_max')}
        hint="Flight outside this range is reported as outside the model rather than computed. It is an attached-flow fit and says nothing about the stall."
        onCommit={(target) => { patch({ clValidMax: target?.value ?? 0 }, ['polar-cl-max']) }}
      />
      <EvidenceSelect
        id="polar-evidence"
        label="How good the polar is"
        value={polar.evidence ?? 'assumed'}
        onChange={(evidence) => { patch({ evidence }) }}
      />
      {held && (
        <button
          type="button"
          onClick={() => { void api.run([{ kind: 'set-drag-polar', polar: null }]) }}
        >
          Withdraw the polar
        </button>
      )}
    </Section>
  )
}

const CAPABILITY_KINDS = [
  { id: 'static', label: 'Static, on the bench' },
  { id: 'in-flight', label: 'In flight, at a stated speed' },
] as const

/**
 * PropulsionPanel holds the chain efficiency, the component ratings and the
 * measured capability points.
 *
 * A capability point is one condition and answers only questions at that
 * condition. Static thrust is not cruise thrust, and nothing here interpolates
 * between points: a Kv and a propeller diameter do not establish thrust across
 * an envelope, so a point that was not measured is simply absent.
 */
export function PropulsionPanel(props: { api: WorksheetApi }): ReactNode {
  const { api } = props
  const propulsion = propulsionOf(api.design)
  const capabilities = propulsion.capabilities ?? []
  const targets = propulsion.targets ?? []
  const limits = propulsion.limits ?? {}

  return (
    <Section title="Propulsion" summary={`${String(capabilities.length)} capability points`}>
      <p className="panel-note">
        The chain efficiency is the propeller, motor and speed controller together at one
        condition. It never turns a static thrust into a cruise figure, and dividing by it
        at zero speed would produce a static power out of nothing.
      </p>
      <QuantityField
        id="chain-efficiency"
        label="Propeller, motor and ESC efficiency"
        dimension="ratio"
        value={propulsion.efficiency === 0 ? null : { value: propulsion.efficiency, unit: '1' }}
        role="driver"
        api={api}
        issue={anyIssue(api, 'propulsion.efficiency')}
        onCommit={(target) => {
          void api.run(
            [{
              kind: 'set-propulsion-efficiency',
              efficiency: {
                total: target?.value ?? 0,
                basis: propulsion.efficiencyBasis ?? '',
                evidence: propulsion.efficiencyEvidence ?? 'assumed',
              },
            }],
            ['chain-efficiency'],
          )
        }}
      />
      <TextField
        id="chain-efficiency-basis"
        label="Where that efficiency comes from"
        value={propulsion.efficiencyBasis ?? ''}
        api={api}
        issue={anyIssue(api, 'propulsion.efficiency_basis')}
        onCommit={(basis) => {
          void api.run(
            [{
              kind: 'set-propulsion-efficiency',
              efficiency: {
                total: propulsion.efficiency,
                basis,
                evidence: propulsion.efficiencyEvidence ?? 'assumed',
              },
            }],
            ['chain-efficiency-basis'],
          )
        }}
      />
      <EvidenceSelect
        id="chain-efficiency-evidence"
        label="How good that is"
        value={propulsion.efficiencyEvidence ?? 'assumed'}
        onChange={(evidence) => {
          void api.run([{
            kind: 'set-propulsion-efficiency',
            efficiency: {
              total: propulsion.efficiency,
              basis: propulsion.efficiencyBasis ?? '',
              evidence,
            },
          }])
        }}
      />

      <details className="subsection">
        <summary>Component ratings</summary>
        <p className="panel-note">
          These are what a feasibility claim is checked against. A rating left unstated is
          not checked, and an unchecked limit is reported as unknown rather than as passed.
        </p>
        {([
          { id: 'maxContinuousPower', label: 'Continuous electrical power', dimension: 'power' },
          { id: 'maxPeakPower', label: 'Peak electrical power', dimension: 'power' },
          { id: 'maxRpm', label: 'Maximum RPM', dimension: 'rotation-rate' },
          { id: 'maxVoltage', label: 'Maximum voltage', dimension: 'voltage' },
          { id: 'propellerDiameter', label: 'Propeller diameter', dimension: 'length' },
          { id: 'propellerHubHeight', label: 'Hub height above the ground', dimension: 'length' },
        ] as const).map((rating) => (
          <QuantityField
            key={rating.id}
            id={`rating-${rating.id}`}
            label={rating.label}
            dimension={rating.dimension}
            value={limits[rating.id] ?? null}
            role="driver"
            api={api}
            issue={anyIssue(api, `propulsion.limits.${rating.id}`)}
            onCommit={(target) => {
              void api.run(
                [{
                  kind: 'set-propulsion-limits',
                  limits: {
                    ...limits,
                    [rating.id]: target === null ? null : { value: target.value, unit: target.unit },
                  },
                }],
                [`rating-${rating.id}`],
              )
            }}
          />
        ))}
        <TextField
          id="rating-basis"
          label="Where the ratings come from"
          value={limits.basis ?? ''}
          api={api}
          hint="A datasheet, a bench test or a manufacturer's claim."
          onCommit={(basis) => {
            void api.run([{ kind: 'set-propulsion-limits', limits: { ...limits, basis } }], ['rating-basis'])
          }}
        />
      </details>

      <CapabilityList api={api} capabilities={capabilities} />
      <ThrustTargetList api={api} targets={targets} capabilities={capabilities} />
    </Section>
  )
}

function CapabilityList(props: { api: WorksheetApi; capabilities: readonly Capability[] }): ReactNode {
  const { api, capabilities } = props
  const [name, setName] = useState('')

  function patch(point: Capability, changes: Partial<Capability>, clear: string[] = []): void {
    void api.run([{ kind: 'set-capability', capability: { ...point, ...changes } }], clear)
  }

  return (
    <details className="subsection" open>
      <summary>Measured capability</summary>
      <p className="panel-note">
        One point is one condition: a speed, an air density, a voltage and a throttle
        setting, with the thrust and the electrical power actually seen there. A static
        point and a cruise point are different measurements and are never substituted for
        one another.
      </p>
      {capabilities.map((point) => (
        <details key={point.name} className="capability">
          <summary><span className="capability-name">{point.name}</span></summary>
          <ChoiceField
            id={`capability-${point.name}-kind`}
            label={`Kind of point for ${point.name}`}
            value={point.kind}
            choices={CAPABILITY_KINDS}
            onChange={(kind) => { patch(point, { kind }) }}
          />
          {([
            { key: 'speed', label: 'Airspeed', dimension: 'speed' },
            { key: 'density', label: 'Air density', dimension: 'density' },
            { key: 'voltage', label: 'Pack voltage', dimension: 'voltage' },
            { key: 'rpm', label: 'RPM', dimension: 'rotation-rate' },
            { key: 'thrust', label: 'Thrust', dimension: 'force' },
            { key: 'electricalPower', label: 'Electrical power drawn', dimension: 'power' },
            { key: 'current', label: 'Current drawn', dimension: 'current' },
          ] as const).map((field) => (
            <QuantityField
              key={field.key}
              id={`capability-${point.name}-${field.key}`}
              label={`${field.label} at ${point.name}`}
              dimension={field.dimension}
              value={point[field.key] ?? null}
              role="driver"
              api={api}
              issue={anyIssue(api, `capability.${point.name}.${field.key}`)}
              onCommit={(target) => {
                patch(
                  point,
                  { [field.key]: target === null ? null : { value: target.value, unit: target.unit } },
                  [`capability-${point.name}-${field.key}`],
                )
              }}
            />
          ))}
          <QuantityField
            id={`capability-${point.name}-throttle`}
            label={`Throttle setting at ${point.name}`}
            dimension="ratio"
            value={point.throttle === 0 ? null : { value: point.throttle, unit: '1' }}
            role="driver"
            api={api}
            onCommit={(target) => {
              patch(point, { throttle: target?.value ?? 0 }, [`capability-${point.name}-throttle`])
            }}
          />
          <TextField
            id={`capability-${point.name}-density-basis`}
            label={`Where the density at ${point.name} comes from`}
            value={point.densityBasis ?? ''}
            api={api}
            onCommit={(densityBasis) => {
              patch(point, { densityBasis }, [`capability-${point.name}-density-basis`])
            }}
          />
          <TextField
            id={`capability-${point.name}-basis`}
            label={`Where ${point.name} comes from`}
            value={point.basis}
            api={api}
            hint="A bench run, a flight log or a manufacturer's chart."
            issue={anyIssue(api, `capability.${point.name}`)}
            onCommit={(basis) => { patch(point, { basis }, [`capability-${point.name}-basis`]) }}
          />
          <EvidenceSelect
            id={`capability-${point.name}-evidence`}
            label={`How good ${point.name} is`}
            value={point.evidence ?? 'assumed'}
            onChange={(evidence) => { patch(point, { evidence }) }}
          />
          <button
            type="button"
            onClick={() => { void api.run([{ kind: 'remove-capability', name: point.name }]) }}
          >
            Remove {point.name}
          </button>
        </details>
      ))}
      <div className="field">
        <label className="field-label" htmlFor="capability-new">Add a capability point</label>
        <div className="field-entry">
          <input
            id="capability-new"
            className="field-input field-input-wide"
            autoComplete="off"
            value={name}
            onChange={(event) => { setName(event.target.value) }}
          />
          <button
            type="button"
            className="inline"
            disabled={name.trim() === ''}
            onClick={() => {
              void api.run([{ kind: 'set-capability', capability: emptyCapability(name.trim(), 'in-flight') }])
              setName('')
            }}
          >
            In flight
          </button>
          <button
            type="button"
            className="inline"
            disabled={name.trim() === ''}
            onClick={() => {
              void api.run([{ kind: 'set-capability', capability: emptyCapability(name.trim(), 'static') }])
              setName('')
            }}
          >
            Static
          </button>
        </div>
        <p className="field-hint">
          The kind is chosen when the point is added, because it says what the measurement
          is rather than describing it afterwards.
        </p>
      </div>
    </details>
  )
}

/**
 * ThrustTargetList holds thrust-to-weight targets. A target is what you want; a
 * capability point is what the installation was measured to give. They are
 * entered separately and compared, rather than one standing in for the other.
 */
function ThrustTargetList(props: {
  api: WorksheetApi
  targets: readonly ThrustTarget[]
  capabilities: readonly Capability[]
}): ReactNode {
  const { api, targets, capabilities } = props
  const [name, setName] = useState('')

  function patch(target: ThrustTarget, changes: Partial<ThrustTarget>, clear: string[] = []): void {
    void api.run([{ kind: 'set-thrust-target', target: { ...target, ...changes } }], clear)
  }

  return (
    <details className="subsection">
      <summary>Thrust-to-weight targets</summary>
      <p className="panel-note">
        A target is stated against one capability point, because a thrust-to-weight ratio
        without its condition claims nothing: 0.8 static and 0.8 at cruise speed are
        different aircraft.
      </p>
      {targets.map((target) => (
        <div key={target.name} className="target">
          <h4>{target.name}</h4>
          <QuantityField
            id={`target-${target.name}-ratio`}
            label={`Thrust-to-weight for ${target.name}`}
            dimension="ratio"
            value={target.ratio === 0 ? null : { value: target.ratio, unit: '1' }}
            role="driver"
            api={api}
            issue={anyIssue(api, `thrust_target.${target.name}`)}
            onCommit={(entry) => {
              patch(target, { ratio: entry?.value ?? 0 }, [`target-${target.name}-ratio`])
            }}
          />
          <ChoiceField
            id={`target-${target.name}-capability`}
            label={`Condition for ${target.name}`}
            value={target.capability}
            placeholder="Choose a capability point"
            choices={capabilities.map((point) => ({ id: point.name, label: point.name }))}
            onChange={(capability) => { patch(target, { capability }) }}
          />
          <ChoiceField
            id={`target-${target.name}-priority`}
            label={`Priority for ${target.name}`}
            value={target.priority === '' ? 'required' : target.priority}
            choices={[{ id: 'required', label: 'Required' }, { id: 'preferred', label: 'Preferred' }]}
            onChange={(priority) => { patch(target, { priority }) }}
          />
          <TextField
            id={`target-${target.name}-basis`}
            label={`Where ${target.name} comes from`}
            value={target.basis}
            api={api}
            onCommit={(basis) => { patch(target, { basis }, [`target-${target.name}-basis`]) }}
          />
          <button
            type="button"
            onClick={() => { void api.run([{ kind: 'remove-thrust-target', name: target.name }]) }}
          >
            Remove {target.name}
          </button>
        </div>
      ))}
      <div className="field">
        <label className="field-label" htmlFor="target-new">Add a thrust target</label>
        <div className="field-entry">
          <input
            id="target-new"
            className="field-input field-input-wide"
            autoComplete="off"
            value={name}
            onChange={(event) => { setName(event.target.value) }}
          />
          <button
            type="button"
            className="inline"
            disabled={name.trim() === ''}
            onClick={() => {
              void api.run([{
                kind: 'set-thrust-target',
                target: { name: name.trim(), basis: '', capability: '', ratio: 0, priority: 'required' },
              }])
              setName('')
            }}
          >
            Add target
          </button>
        </div>
      </div>
    </details>
  )
}

/**
 * BatteryPanel is the flight pack. It carries no mass of its own: the pack's
 * mass is a listed component, named here, so that adding a battery cannot count
 * its mass twice.
 */
export function BatteryPanel(props: { api: WorksheetApi }): ReactNode {
  const { api } = props
  const battery = batteryOf(api.design)
  const held = api.design.battery != null
  const components = api.design.components ?? []
  const byCapacity = battery.mode !== 'entered-energy'

  function patch(changes: Partial<Battery>, clear: string[] = []): void {
    void api.run([{ kind: 'set-battery', battery: { ...battery, ...changes } }], clear)
  }

  return (
    <Section title="Flight pack" summary={held ? 'stated' : 'not stated'}>
      <p className="panel-note">
        The pack's energy is stated one way or the other, never both: a capacity and a
        nominal voltage, or an energy you measured. Its mass belongs to the component it
        names below, so it is never added twice.
      </p>
      <ChoiceField
        id="battery-mode"
        label="How the energy is stated"
        value={battery.mode}
        choices={[
          { id: 'capacity-and-voltage', label: 'Capacity and nominal voltage' },
          { id: 'entered-energy', label: 'Energy you measured' },
        ]}
        onChange={(mode) => { patch({ mode }) }}
      />
      {byCapacity ? (
        <>
          <QuantityField
            id="battery-capacity"
            label="Capacity"
            dimension="charge"
            value={battery.capacity ?? null}
            role="driver"
            api={api}
            issue={anyIssue(api, 'battery.capacity')}
            hint="A capacity in mAh is a charge, not an energy. It becomes one only against the nominal voltage."
            onCommit={(target) => {
              patch(
                { capacity: target === null ? null : { value: target.value, unit: target.unit } },
                ['battery-capacity'],
              )
            }}
          />
          <QuantityField
            id="battery-voltage"
            label="Nominal voltage"
            dimension="voltage"
            value={battery.nominalVoltage ?? null}
            role="driver"
            api={api}
            issue={anyIssue(api, 'battery.nominal_voltage')}
            onCommit={(target) => {
              patch(
                { nominalVoltage: target === null ? null : { value: target.value, unit: target.unit } },
                ['battery-voltage'],
              )
            }}
          />
        </>
      ) : (
        <QuantityField
          id="battery-energy"
          label="Energy"
          dimension="energy"
          value={battery.energy ?? null}
          role="driver"
          api={api}
          issue={anyIssue(api, 'battery.energy')}
          onCommit={(target) => {
            patch(
              { energy: target === null ? null : { value: target.value, unit: target.unit } },
              ['battery-energy'],
            )
          }}
        />
      )}
      <QuantityField
        id="battery-usable"
        label="Usable fraction"
        dimension="ratio"
        value={battery.usableFraction === 0 ? null : { value: battery.usableFraction, unit: '1' }}
        role="driver"
        api={api}
        issue={anyIssue(api, 'battery.usable_fraction')}
        hint="How much of the pack you are willing to take out of it. It is not the mission reserve, which is applied separately and once."
        onCommit={(target) => { patch({ usableFraction: target?.value ?? 0 }, ['battery-usable']) }}
      />
      <QuantityField
        id="battery-continuous-current"
        label="Continuous current rating"
        dimension="current"
        value={battery.continuousCurrentLimit ?? null}
        role="driver"
        api={api}
        onCommit={(target) => {
          patch(
            { continuousCurrentLimit: target === null ? null : { value: target.value, unit: target.unit } },
            ['battery-continuous-current'],
          )
        }}
      />
      <QuantityField
        id="battery-peak-current"
        label="Peak current rating"
        dimension="current"
        value={battery.peakCurrentLimit ?? null}
        role="driver"
        api={api}
        onCommit={(target) => {
          patch(
            { peakCurrentLimit: target === null ? null : { value: target.value, unit: target.unit } },
            ['battery-peak-current'],
          )
        }}
      />
      <ChoiceField
        id="battery-component"
        label="Which component is this pack"
        value={battery.component}
        placeholder="Choose a listed component"
        choices={components.map((component) => ({ id: component.name, label: component.name }))}
        hint="The pack's mass and position are that component's. Naming it here links the two without a second copy of the mass."
        onChange={(component) => { patch({ component }) }}
      />
      <TextField
        id="battery-basis"
        label="Where the pack figures come from"
        value={battery.basis}
        api={api}
        issue={anyIssue(api, 'battery.basis')}
        onCommit={(basis) => { patch({ basis }, ['battery-basis']) }}
      />
      <EvidenceSelect
        id="battery-evidence"
        label="How good the pack figures are"
        value={battery.evidence ?? 'assumed'}
        onChange={(evidence) => { patch({ evidence }) }}
      />
      {held && (
        <button
          type="button"
          onClick={() => { void api.run([{ kind: 'set-battery', battery: null }]) }}
        >
          Withdraw the pack
        </button>
      )}
    </Section>
  )
}

const LOAD_SIDES = [
  { id: 'pack-side', label: 'Pack side — what the pack supplies' },
  { id: 'load-side', label: 'Load side — what the device consumes' },
] as const

/**
 * AuxiliaryPanel is the avionics, servo, sensor and payload draw.
 *
 * Which side of the regulator a figure was taken on is asked rather than
 * assumed, because the two differ by the regulator's own losses, and adding a
 * load-side figure to a pack-side total counts those losses either zero times
 * or twice. A load with neither figure stated leaves the budget incomplete,
 * which is reported as incomplete rather than as a small number.
 */
export function AuxiliaryPanel(props: { api: WorksheetApi }): ReactNode {
  const { api } = props
  const loads = api.design.auxiliary ?? []
  const components = api.design.components ?? []
  const [name, setName] = useState('')

  function patch(load: AuxiliaryLoad, changes: Partial<AuxiliaryLoad>, clear: string[] = []): void {
    void api.run([{ kind: 'set-auxiliary-load', load: { ...load, ...changes } }], clear)
  }

  return (
    <Section title="Electrical loads" summary={`${String(loads.length)} listed`}>
      <p className="panel-note">
        Autopilot, receiver, telemetry, sensors, regulators, servos, wiring and payload.
        Continuous draw is what the mission's energy is spent on; peak draw is checked
        against the supply ratings and carries no energy of its own, because a peak with no
        stated duty cycle says nothing about how long it lasts.
      </p>
      {loads.map((load) => (
        <details key={load.name} className="load">
          <summary><span className="load-name">{load.name}</span></summary>
          <QuantityField
            id={`load-${load.name}-continuous`}
            label={`Continuous draw for ${load.name}`}
            dimension="power"
            value={load.continuous ?? null}
            role="driver"
            api={api}
            issue={anyIssue(api, `auxiliary.${load.name}.continuous`)}
            onCommit={(target) => {
              patch(
                load,
                { continuous: target === null ? null : { value: target.value, unit: target.unit } },
                [`load-${load.name}-continuous`],
              )
            }}
          />
          <QuantityField
            id={`load-${load.name}-peak`}
            label={`Peak draw for ${load.name}`}
            dimension="power"
            value={load.peak ?? null}
            role="driver"
            api={api}
            issue={anyIssue(api, `auxiliary.${load.name}.peak`)}
            onCommit={(target) => {
              patch(
                load,
                { peak: target === null ? null : { value: target.value, unit: target.unit } },
                [`load-${load.name}-peak`],
              )
            }}
          />
          <ChoiceField
            id={`load-${load.name}-side`}
            label={`Which side of the regulator for ${load.name}`}
            value={load.side}
            choices={LOAD_SIDES}
            onChange={(side) => { patch(load, { side }) }}
          />
          <QuantityField
            id={`load-${load.name}-regulator`}
            label={`Regulator efficiency for ${load.name}`}
            dimension="ratio"
            value={load.regulatorEfficiency === 0 ? null : { value: load.regulatorEfficiency, unit: '1' }}
            role="driver"
            api={api}
            issue={anyIssue(api, `auxiliary.${load.name}.regulator_efficiency`)}
            hint="Needed only for a load-side figure: it is what turns what the device consumes into what the pack supplies."
            onCommit={(target) => {
              patch(load, { regulatorEfficiency: target?.value ?? 0 }, [`load-${load.name}-regulator`])
            }}
          />
          <ChoiceField
            id={`load-${load.name}-component`}
            label={`Which component ${load.name} is`}
            value={load.component ?? ''}
            placeholder="Not linked to a component"
            choices={components.map((component) => ({ id: component.name, label: component.name }))}
            onChange={(component) => { patch(load, { component }) }}
          />
          <TextField
            id={`load-${load.name}-basis`}
            label={`Where ${load.name} comes from`}
            value={load.basis}
            api={api}
            issue={anyIssue(api, `auxiliary.${load.name}`)}
            onCommit={(basis) => { patch(load, { basis }, [`load-${load.name}-basis`]) }}
          />
          <EvidenceSelect
            id={`load-${load.name}-evidence`}
            label={`How good ${load.name} is`}
            value={load.evidence ?? 'assumed'}
            onChange={(evidence) => { patch(load, { evidence }) }}
          />
          <button
            type="button"
            onClick={() => { void api.run([{ kind: 'remove-auxiliary-load', name: load.name }]) }}
          >
            Remove {load.name}
          </button>
        </details>
      ))}
      <div className="field">
        <label className="field-label" htmlFor="load-new">Add an electrical load</label>
        <div className="field-entry">
          <input
            id="load-new"
            className="field-input field-input-wide"
            autoComplete="off"
            value={name}
            onChange={(event) => { setName(event.target.value) }}
          />
          <button
            type="button"
            className="inline"
            disabled={name.trim() === ''}
            onClick={() => {
              void api.run([{ kind: 'set-auxiliary-load', load: emptyLoad(name.trim()) }])
              setName('')
            }}
          >
            Add load
          </button>
        </div>
        {issueFor(api, 'auxiliary') !== undefined && (
          <p className="field-issue-text" role="status">{issueFor(api, 'auxiliary')}</p>
        )}
      </div>
    </Section>
  )
}
