import type { ReactNode } from 'react'
import type {
  ElectricalBudget,
  Evaluation,
  MissionResult,
  PowerFeasibility,
  Quantity,
  SegmentResult,
  ThrustCheck,
} from '../api/contract.ts'
import { displayNumber } from '../state/fields.ts'
import { Section } from './fields.tsx'

// The power answers, in the results column.
//
// Three things are kept apart here on purpose, because collapsing any two of
// them would make a partial answer look like a whole one.
//
// Whether a number could be computed at all is one question — computed,
// missing, invalid — and whether what it describes is acceptable is another —
// met, unmet, unknown. A mission whose energy fits is not thereby flyable:
// energy sufficiency and per-leg flight feasibility are separate statements and
// are shown as separate rows. And a budget that is missing one avionics figure
// is incomplete rather than small, so completeness is said in words next to
// every total that rests on it.

function quantityText(value: Quantity | null | undefined): string {
  if (!value) return 'not available'
  if (value.unit === '1') return displayNumber(value.value)
  return `${displayNumber(value.value)} ${value.unit}`
}

/** resultText says why a number is absent, in the core's own vocabulary. */
function resultText(status: string): string {
  switch (status) {
    case 'computed':
      return 'computed'
    case 'missing':
      return 'unknown — something it needs has not been supplied'
    case 'invalid':
      return 'unknown — an input it depends on cannot be true'
    case 'stale':
      return 'unknown — computed from an earlier revision'
    default:
      return status
  }
}

function limitText(status: string): string {
  switch (status) {
    case 'met':
      return 'within the limit'
    case 'unmet':
      return 'over the limit'
    default:
      return 'unknown — the limit or the demand is unstated'
  }
}

function ElectricalSection(props: { budget: ElectricalBudget }): ReactNode {
  const { budget } = props
  return (
    <Section title="Electrical budget" defaultOpen={false}>
      <dl className="bounds">
        <dt>Continuous auxiliary draw</dt>
        <dd>{quantityText(budget.continuous)}</dd>
        <dt>Peak auxiliary draw</dt>
        <dd>{quantityText(budget.peak)}</dd>
        <dt>Completeness</dt>
        <dd>
          {budget.complete
            ? 'every listed load states its draw'
            : 'incomplete — a listed load has no stated draw, so no complete electrical '
              + 'feasibility claim is made'}
          {(budget.detail ?? '') !== '' && ` ${budget.detail ?? ''}`}
        </dd>
      </dl>
      {budget.loads.length === 0 ? (
        <p>No electrical load is listed. Avionics, servos and the payload all draw power.</p>
      ) : (
        <table className="checks">
          <caption className="sr-only">Auxiliary loads</caption>
          <thead>
            <tr>
              <th scope="col">Load</th>
              <th scope="col">Side</th>
              <th scope="col">Continuous</th>
              <th scope="col">Peak</th>
            </tr>
          </thead>
          <tbody>
            {budget.loads.map((load) => (
              <tr key={load.name}>
                <th scope="row">{load.name}</th>
                <td>{load.side === 'load-side' ? 'load side' : 'pack side'}</td>
                <td>{load.known ? quantityText(load.continuous) : 'not stated'}</td>
                <td>{quantityText(load.peak)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      <p className="coverage">
        A load-side figure is what the device consumes and a pack-side one is what the pack
        supplies; they differ by the regulator's own losses, which is why each load says
        which it is.
      </p>
    </Section>
  )
}

function FeasibilitySection(props: { feasibility: PowerFeasibility }): ReactNode {
  const { feasibility } = props
  return (
    <Section title="Supply and ratings" defaultOpen={false}>
      <dl className="bounds">
        <dt>Continuous demand</dt>
        <dd>
          {quantityText(feasibility.continuousDemand)}
          {feasibility.status !== 'computed' && ` — ${resultText(feasibility.status)}`}
          {(feasibility.detail ?? '') !== '' && ` ${feasibility.detail ?? ''}`}
        </dd>
        <dt>Peak demand</dt>
        <dd>
          {quantityText(feasibility.peakDemand)}
          {(feasibility.peakDetail ?? '') !== '' && ` ${feasibility.peakDetail ?? ''}`}
        </dd>
      </dl>
      {feasibility.checks.length === 0 ? (
        <p>
          No component rating is stated, so nothing is checked. An unchecked limit is
          unknown rather than passed.
        </p>
      ) : (
        <table className="checks">
          <caption className="sr-only">Supply checks</caption>
          <thead>
            <tr>
              <th scope="col">Rating</th>
              <th scope="col">Limit</th>
              <th scope="col">Demand</th>
              <th scope="col">Status</th>
            </tr>
          </thead>
          <tbody>
            {feasibility.checks.map((check) => (
              <tr key={check.name}>
                <th scope="row">{check.name}</th>
                <td>{quantityText(check.limit)}</td>
                <td>{quantityText(check.actual)}</td>
                <td>{limitText(check.status)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </Section>
  )
}

function ThrustSection(props: { checks: readonly ThrustCheck[] }): ReactNode {
  if (props.checks.length === 0) return null
  return (
    <Section title="Thrust to weight" defaultOpen={false}>
      <table className="checks">
        <caption className="sr-only">Thrust-to-weight targets</caption>
        <thead>
          <tr>
            <th scope="col">Target</th>
            <th scope="col">Condition</th>
            <th scope="col">Wanted</th>
            <th scope="col">Available</th>
            <th scope="col">Status</th>
          </tr>
        </thead>
        <tbody>
          {props.checks.map((check) => (
            <tr key={check.name}>
              <th scope="row">
                {check.name}
                <span className="priority"> ({check.priority})</span>
              </th>
              <td>{check.condition === '' ? (check.capability ?? '—') : check.condition}</td>
              <td>{displayNumber(check.target)}</td>
              <td>{check.status === 'unknown' ? '—' : displayNumber(check.available)}</td>
              <td>
                {check.status === 'met' ? 'met' : check.status === 'unmet' ? 'unmet' : 'unknown'}
                {(check.detail ?? '') !== '' && ` — ${check.detail ?? ''}`}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      <p className="coverage">
        A target is answered only at the condition its capability point was measured at.
        Static thrust is not cruise thrust, and nothing here converts between them.
      </p>
    </Section>
  )
}

function segmentPowerText(segment: SegmentResult): string {
  if (segment.power.status !== 'computed') {
    return `${resultText(segment.power.status)}${(segment.power.detail ?? '') === '' ? '' : ` — ${segment.power.detail ?? ''}`}`
  }
  return quantityText(segment.power.electrical)
}

function availabilityText(segment: SegmentResult): string {
  const availability = segment.availability
  const detail = availability.detail ?? ''
  switch (availability.status) {
    case 'met':
      return 'thrust available'
    case 'unmet':
      return `not enough thrust — ${detail}`
    default:
      // An unknown thrust is not one fact but several — no point was named, the
      // point named does not exist, or the point was measured at a condition
      // this leg does not fly — and the core says which. Collapsing all three
      // into "unknown" would hide a static measurement being declined for a
      // cruise leg behind the same words as an empty field.
      return detail === '' ? 'thrust unknown' : `thrust unknown — ${detail}`
  }
}

function MissionSection(props: { mission: MissionResult }): ReactNode {
  const { mission } = props
  return (
    <Section title="Mission and energy">
      <dl className="bounds">
        <dt>Energy the mission needs</dt>
        <dd>
          {quantityText(mission.requiredEnergy)}
          {mission.status !== 'computed' && ` — ${resultText(mission.status)}`}
          {(mission.detail ?? '') !== '' && ` ${mission.detail ?? ''}`}
        </dd>
        <dt>Energy the pack offers</dt>
        <dd>
          {quantityText(mission.budget)}
          {mission.usableEnergy && (
            <> (usable {quantityText(mission.usableEnergy)}, less a{' '}
              {displayNumber(mission.reserveFraction * 100)}% reserve applied once)
            </>
          )}
        </dd>
        <dt>Does the energy fit</dt>
        <dd>
          {mission.energyStatus === 'met'
            ? 'yes, with the reserve held back'
            : mission.energyStatus === 'unmet'
              ? 'no — the mission needs more than the pack offers'
              : 'unknown — the pack or a segment is unstated'}
          . Energy sufficiency is not flight feasibility: each leg's own status is below.
        </dd>
        <dt>Duration</dt>
        <dd>{quantityText(mission.totalDuration)}</dd>
        <dt>Ground distance</dt>
        <dd>{quantityText(mission.totalDistance)}</dd>
        <dt>Largest continuous demand</dt>
        <dd>{quantityText(mission.peakContinuousPower)}</dd>
      </dl>
      {mission.segments.length === 0 ? (
        <p>
          No mission segment is written. Launch, climb, cruise, loiter, return and recovery
          are legs you state; nothing is added on your behalf.
        </p>
      ) : (
        <table className="checks">
          <caption className="sr-only">Mission segments</caption>
          <thead>
            <tr>
              <th scope="col">Segment</th>
              <th scope="col">Electrical power</th>
              <th scope="col">Time</th>
              <th scope="col">Distance</th>
              <th scope="col">Energy</th>
              <th scope="col">Thrust</th>
            </tr>
          </thead>
          <tbody>
            {mission.segments.map((segment) => (
              <tr key={segment.name}>
                <th scope="row">
                  {segment.name}
                  <span className="priority"> ({segment.kind})</span>
                  {segment.power.model === 'entered-estimate' && (
                    <span className="priority"> entered estimate</span>
                  )}
                </th>
                <td>{segmentPowerText(segment)}</td>
                <td>{quantityText(segment.duration)}</td>
                <td>{quantityText(segment.distance)}</td>
                <td>{quantityText(segment.energy)}</td>
                <td>{availabilityText(segment)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      <p className="coverage">
        A leg computed from the drag polar is only as good as the polar behind it, and one
        whose power was entered is evidence you supplied rather than a model result. No
        segment model estimates a launch or a recovery: those legs carry the figures you
        state for them.
      </p>
    </Section>
  )
}

/**
 * PowerResults is the whole power answer. It renders nothing at all when the
 * design states no power definition, so a wing-sizing worksheet is not filled
 * with empty tables about a propulsion system nobody has described yet.
 */
export function PowerResults(props: { evaluation: Evaluation; stated: boolean }): ReactNode {
  const { evaluation } = props
  if (!props.stated) return null
  return (
    <>
      <MissionSection mission={evaluation.mission} />
      <FeasibilitySection feasibility={evaluation.powerFeasibility} />
      <ElectricalSection budget={evaluation.electrical} />
      <ThrustSection checks={evaluation.thrustChecks} />
    </>
  )
}
