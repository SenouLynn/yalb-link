import { useState } from 'react'
import type { ReactNode } from 'react'
import type { MissionSegment } from '../api/contract.ts'
import { SEGMENT_KINDS, emptySegment, missionOf, propulsionOf } from '../state/power.ts'
import type { WorksheetApi } from '../useWorksheet.ts'
import { QuantityField, Section, TextField } from './fields.tsx'
import { ChoiceField } from './power.tsx'
import { anyIssue } from './issues.ts'

// The mission: an ordered list of legs, and the reserve that is applied once.
//
// A mission is a sequence rather than a set, so editing a leg and reordering
// the legs are different edits and the worksheet offers them separately. The
// same legs flown in a different order are a different mission, and the saved
// draft records the order for that reason.
//
// Nothing here adds a leg the builder did not ask for. An autopilot's own
// return-to-home setting is not a return leg: it establishes no speed, no wind
// and no distance, so a wind-aware return range only exists once someone writes
// the leg down.

const SEGMENT_MODELS = [
  { id: 'drag-polar', label: 'Computed from the drag polar' },
  { id: 'entered-estimate', label: 'A power figure you entered' },
] as const

const SEGMENT_TIMINGS = [
  { id: 'duration', label: 'A duration you set' },
  { id: 'ground-distance', label: 'A distance over the ground' },
] as const

export function MissionPanel(props: { api: WorksheetApi }): ReactNode {
  const { api } = props
  const mission = missionOf(api.design)
  const segments = mission.segments ?? []
  const cases = api.design.cases ?? []
  const capabilities = propulsionOf(api.design).capabilities ?? []
  const [name, setName] = useState('')
  const [kind, setKind] = useState<string>('cruise')

  function profile(changes: Partial<{ name: string; basis: string; reserveBasis: string; reserveFraction: number }>, clear: string[] = []): void {
    void api.run(
      [{
        kind: 'set-mission-profile',
        profile: {
          name: mission.name,
          basis: mission.basis,
          reserveBasis: mission.reserveBasis ?? '',
          reserveFraction: mission.reserveFraction,
          ...changes,
        },
      }],
      clear,
    )
  }

  function patch(segment: MissionSegment, changes: Partial<MissionSegment>, clear: string[] = []): void {
    void api.run([{ kind: 'set-mission-segment', segment: { ...segment, ...changes } }], clear)
  }

  return (
    <Section title="Mission" summary={`${String(segments.length)} segments`}>
      <p className="panel-note">
        The energy a mission needs is the sum of each leg's power over its own time, not one
        draw held for the whole flight. The reserve is applied once, to the pack's usable
        energy, and energy sufficiency is reported separately from whether each leg is
        flyable at all.
      </p>
      <TextField
        id="mission-name"
        label="Mission name"
        value={mission.name}
        api={api}
        onCommit={(value) => { profile({ name: value }, ['mission-name']) }}
      />
      <TextField
        id="mission-basis"
        label="Where the mission comes from"
        value={mission.basis}
        api={api}
        issue={anyIssue(api, 'mission.basis')}
        onCommit={(basis) => { profile({ basis }, ['mission-basis']) }}
      />
      <QuantityField
        id="mission-reserve"
        label="Energy reserve"
        dimension="ratio"
        value={mission.reserveFraction === 0 ? null : { value: mission.reserveFraction, unit: '1' }}
        role="driver"
        api={api}
        issue={anyIssue(api, 'mission.reserve')}
        hint="A fraction of the usable energy held back, applied once to the whole mission."
        onCommit={(target) => { profile({ reserveFraction: target?.value ?? 0 }, ['mission-reserve']) }}
      />
      <TextField
        id="mission-reserve-basis"
        label="Where the reserve comes from"
        value={mission.reserveBasis ?? ''}
        api={api}
        onCommit={(reserveBasis) => { profile({ reserveBasis }, ['mission-reserve-basis']) }}
      />

      {segments.map((segment, index) => (
        <details key={segment.name} className="segment">
          <summary>
            <span className="segment-name">{segment.name}</span>
            <span className="segment-kind"> {segment.kind}</span>
          </summary>
          <ChoiceField
            id={`segment-${segment.name}-case`}
            label={`Flight case for ${segment.name}`}
            value={segment.case}
            placeholder="Choose a flight case"
            choices={cases.map((entry) => ({ id: entry.name, label: entry.name }))}
            hint="The case carries the air density and the CLmax the leg is judged against."
            onChange={(caseName) => { patch(segment, { case: caseName }) }}
          />
          <ChoiceField
            id={`segment-${segment.name}-model`}
            label={`Where ${segment.name}'s power comes from`}
            value={segment.model}
            choices={SEGMENT_MODELS}
            hint="An entered estimate is evidence you supplied and is labelled as such; it is not a model result."
            onChange={(model) => { patch(segment, { model }) }}
          />
          <QuantityField
            id={`segment-${segment.name}-speed`}
            label={`Airspeed for ${segment.name}`}
            dimension="speed"
            value={segment.speed ?? null}
            role="driver"
            api={api}
            issue={anyIssue(api, `segment.${segment.name}.speed`)}
            onCommit={(target) => {
              patch(
                segment,
                { speed: target === null ? null : { value: target.value, unit: target.unit } },
                [`segment-${segment.name}-speed`],
              )
            }}
          />
          <QuantityField
            id={`segment-${segment.name}-climb`}
            label={`Flight-path angle for ${segment.name}`}
            dimension="angle"
            value={segment.climbAngle ?? null}
            role="driver"
            api={api}
            issue={anyIssue(api, `segment.${segment.name}.climb_angle`)}
            hint="Zero is level flight and is stated rather than left blank. A climb angle changes the force balance the leg is solved from."
            onCommit={(target) => {
              patch(
                segment,
                { climbAngle: target === null ? null : { value: target.value, unit: target.unit } },
                [`segment-${segment.name}-climb`],
              )
            }}
          />
          {segment.model === 'entered-estimate' && (
            <>
              <QuantityField
                id={`segment-${segment.name}-entered-power`}
                label={`Entered electrical power for ${segment.name}`}
                dimension="power"
                value={segment.enteredPower ?? null}
                role="driver"
                api={api}
                issue={anyIssue(api, `segment.${segment.name}.entered_power`)}
                onCommit={(target) => {
                  patch(
                    segment,
                    { enteredPower: target === null ? null : { value: target.value, unit: target.unit } },
                    [`segment-${segment.name}-entered-power`],
                  )
                }}
              />
              <TextField
                id={`segment-${segment.name}-entered-basis`}
                label={`Where that figure for ${segment.name} comes from`}
                value={segment.enteredBasis ?? ''}
                api={api}
                onCommit={(enteredBasis) => {
                  patch(segment, { enteredBasis }, [`segment-${segment.name}-entered-basis`])
                }}
              />
            </>
          )}
          <ChoiceField
            id={`segment-${segment.name}-timing`}
            label={`How long ${segment.name} lasts`}
            value={segment.timing}
            choices={SEGMENT_TIMINGS}
            onChange={(timing) => { patch(segment, { timing }) }}
          />
          {segment.timing === 'ground-distance' ? (
            <QuantityField
              id={`segment-${segment.name}-distance`}
              label={`Ground distance for ${segment.name}`}
              dimension="length"
              value={segment.distance ?? null}
              role="driver"
              api={api}
              issue={anyIssue(api, `segment.${segment.name}.distance`)}
              onCommit={(target) => {
                patch(
                  segment,
                  { distance: target === null ? null : { value: target.value, unit: target.unit } },
                  [`segment-${segment.name}-distance`],
                )
              }}
            />
          ) : (
            <QuantityField
              id={`segment-${segment.name}-duration`}
              label={`Duration of ${segment.name}`}
              dimension="time"
              value={segment.duration ?? null}
              role="driver"
              api={api}
              issue={anyIssue(api, `segment.${segment.name}.duration`)}
              onCommit={(target) => {
                patch(
                  segment,
                  { duration: target === null ? null : { value: target.value, unit: target.unit } },
                  [`segment-${segment.name}-duration`],
                )
              }}
            />
          )}
          <QuantityField
            id={`segment-${segment.name}-wind`}
            label={`Wind along the track for ${segment.name}`}
            dimension="speed"
            value={segment.windAlongTrack ?? null}
            role="driver"
            api={api}
            issue={anyIssue(api, `segment.${segment.name}.wind_along_track`)}
            hint="Positive is a tailwind. It changes the ground speed and so the distance and the time, and never the airspeed the power was computed at."
            onCommit={(target) => {
              patch(
                segment,
                { windAlongTrack: target === null ? null : { value: target.value, unit: target.unit } },
                [`segment-${segment.name}-wind`],
              )
            }}
          />
          <ChoiceField
            id={`segment-${segment.name}-capability`}
            label={`Capability point to check ${segment.name} against`}
            value={segment.capability ?? ''}
            placeholder="No thrust check for this leg"
            choices={capabilities.map((point) => ({ id: point.name, label: point.name }))}
            hint="Without one, the leg's energy can still be computed and its thrust availability stays unknown."
            onChange={(capability) => { patch(segment, { capability }) }}
          />
          <TextField
            id={`segment-${segment.name}-notes`}
            label={`Notes on ${segment.name}`}
            value={segment.notes ?? ''}
            api={api}
            onCommit={(notes) => { patch(segment, { notes }, [`segment-${segment.name}-notes`]) }}
          />
          <div className="action">
            <button
              type="button"
              className="inline"
              disabled={index === 0}
              onClick={() => {
                void api.run([{ kind: 'move-mission-segment', name: segment.name, index: index - 1 }])
              }}
            >
              Move {segment.name} earlier
            </button>
            <button
              type="button"
              className="inline"
              disabled={index === segments.length - 1}
              onClick={() => {
                void api.run([{ kind: 'move-mission-segment', name: segment.name, index: index + 1 }])
              }}
            >
              Move {segment.name} later
            </button>
            <button
              type="button"
              className="inline"
              onClick={() => { void api.run([{ kind: 'remove-mission-segment', name: segment.name }]) }}
            >
              Remove {segment.name}
            </button>
          </div>
        </details>
      ))}

      <div className="field">
        <label className="field-label" htmlFor="segment-new">Add a segment</label>
        <div className="field-entry">
          <input
            id="segment-new"
            className="field-input field-input-wide"
            autoComplete="off"
            value={name}
            onChange={(event) => { setName(event.target.value) }}
          />
          <select
            className="field-input"
            aria-label="Kind of segment to add"
            value={kind}
            onChange={(event) => { setKind(event.target.value) }}
          >
            {SEGMENT_KINDS.map((entry) => (
              <option key={entry.id} value={entry.id}>{entry.label}</option>
            ))}
          </select>
          <button
            type="button"
            className="inline"
            disabled={name.trim() === ''}
            onClick={() => {
              void api.run([{
                kind: 'set-mission-segment',
                segment: emptySegment(name.trim(), kind, cases[0]?.name ?? ''),
              }])
              setName('')
            }}
          >
            Add segment
          </button>
        </div>
        <p className="field-hint">
          A new leg is added at the end of the mission and can be moved from there. Nothing
          is added on your behalf: a return leg exists because you wrote one.
        </p>
      </div>
    </Section>
  )
}
