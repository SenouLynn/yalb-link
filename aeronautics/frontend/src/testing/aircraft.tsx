import { screen, within } from '@testing-library/react'
import type { UserEvent } from '@testing-library/user-event'
import { enter, enterIn, press, setSelect, settled } from './harness.tsx'

// The fixture aircraft the power and mission browser tests are written against.
//
// It is the one in calculator/testdata/electric-power-fixtures.md, which is
// independent of the book: the primary reference's powerplant chapter sizes a
// piston engine and its mission analysis burns fuel, so there is no book number
// an electric pack can be checked against. Every figure those tests assert was
// computed at 50 significant digits from the relations the core implements, and
// none of it is a recommended default for any real aircraft.
//
//   2.5 kg, 0.40 m^2, A = 8, ISA sea level, CD0 = 0.035, e = 0.85,
//   chain efficiency 0.55, 8 W of avionics on the pack side.
//
// It lives here rather than in either test file so that the two of them are
// describing one aircraft: a mission assertion and a power assertion that had
// drifted onto different airframes would agree with each other about nothing.

/** loadPanel returns one listed electrical load's editor, expanded. */
export function loadPanel(name: string): HTMLElement {
  const label = screen.getByText(name, { selector: '.load-name' })
  const panel = label.closest('details')
  if (panel === null) throw new Error(`no editor for the load ${name}`)
  return panel
}

/** segmentPanel returns one mission leg's editor. */
export function segmentPanel(name: string): HTMLElement {
  const label = screen.getByText(name, { selector: '.segment-name' })
  const panel = label.closest('details')
  if (panel === null) throw new Error(`no editor for the segment ${name}`)
  return panel
}

/** expand opens one of those editors, which start closed. */
export async function expand(
  user: UserEvent,
  panel: HTMLElement,
  name: string,
  selector: string,
): Promise<void> {
  await user.click(within(panel).getByText(name, { selector }))
}

/** airframe enters the fixture's mass, case and wing. */
export async function airframe(user: UserEvent): Promise<void> {
  await enter(user, 'All-up mass', '2.5')
  await enter(user, 'Where it comes from', 'synthetic fixture')
  await enter(user, 'Whole-aircraft CLmax', '1.2')
  await enter(user, 'Where the coefficient comes from', 'synthetic fixture assumption')
  await enter(user, 'Wing area', '0.4')
  await enter(user, 'Aspect ratio', '8')
}

/** polar enters the fixture's drag polar and the range it is claimed over. */
export async function polar(user: UserEvent): Promise<void> {
  await enter(user, 'Zero-lift drag coefficient CD0', '0.035')
  await enter(user, 'Where CD0 comes from', 'synthetic fixture assumption')
  await enter(user, 'Span efficiency e', '0.85')
  await enter(user, 'Where e comes from', 'synthetic fixture assumption')
  await enter(user, 'Lowest CL the polar is claimed over', '-0.2')
  await enter(user, 'Highest CL the polar is claimed over', '1.1')
}

/** chain enters the propeller, motor and ESC efficiency. */
export async function chain(user: UserEvent): Promise<void> {
  await enter(user, 'Propeller, motor and ESC efficiency', '0.55')
  await enter(user, 'Where that efficiency comes from', 'synthetic fixture assumption')
}

/**
 * avionics lists the fixture's 8 W of continuous pack-side draw, split across
 * the devices that actually draw it: 5 W of autopilot and 3 W of receiver and
 * servos. Both figures are pack-side, so no regulator efficiency enters them.
 */
export async function avionics(user: UserEvent): Promise<void> {
  await addLoad(user, 'autopilot', '5')
  await addLoad(user, 'receiver and servos', '3')
}

/**
 * fixtureAircraft is the whole aircraft: the airframe and case, the polar, the
 * chain efficiency, the avionics budget and the pack. It is everything a leg
 * needs before a mission is written on top of it.
 */
export async function fixtureAircraft(user: UserEvent): Promise<void> {
  await settled()
  await airframe(user)
  await polar(user)
  await chain(user)
  await avionics(user)
  await pack(user)
}

/** capabilityPanel returns one measured capability point's editor. */
export function capabilityPanel(name: string): HTMLElement {
  const label = screen.getByText(name, { selector: '.capability-name' })
  const panel = label.closest('details')
  if (panel === null) throw new Error(`no editor for the capability point ${name}`)
  return panel
}

/**
 * measured records one capability point: one condition, with the thrust and the
 * electrical power actually seen there. `kind` is the button that creates it,
 * and it is what makes a point static or in flight — a static point is at zero
 * airspeed by definition rather than by default.
 */
export async function measured(
  user: UserEvent,
  name: string,
  kind: 'Static' | 'In flight',
  thrust: string,
  watts: string,
  speed?: string,
): Promise<void> {
  await user.type(screen.getByLabelText('Add a capability point'), name)
  await press(user, kind)
  const panel = capabilityPanel(name)
  await expand(user, panel, name, '.capability-name')
  if (speed !== undefined) await enterIn(user, panel, `Airspeed at ${name}`, speed)
  await enterIn(user, panel, `Air density at ${name}`, '1.225')
  await enterIn(user, panel, `Where the density at ${name} comes from`, 'ISA sea level')
  await enterIn(user, panel, `Pack voltage at ${name}`, '14.8')
  await enterIn(user, panel, `Throttle setting at ${name}`, kind === 'Static' ? '1' : '0.6')
  await enterIn(user, panel, `Thrust at ${name}`, thrust)
  await enterIn(user, panel, `Electrical power drawn at ${name}`, watts)
  await enterIn(user, panel, `Where ${name} comes from`, 'synthetic fixture measurement')
}

/** addLoad lists one electrical load with a continuous pack-side draw. */
export async function addLoad(user: UserEvent, name: string, watts: string): Promise<void> {
  await user.type(screen.getByLabelText('Add an electrical load'), name)
  await press(user, 'Add load')
  const panel = loadPanel(name)
  await expand(user, panel, name, '.load-name')
  await enterIn(user, panel, `Continuous draw for ${name}`, watts)
  await enterIn(user, panel, `Where ${name} comes from`, 'synthetic fixture assumption')
}

/** pack enters the 5000 mAh, 14.8 V flight pack at 80% usable. */
export async function pack(user: UserEvent): Promise<void> {
  await setSelect(user, 'Unit for Capacity', 'mAh')
  await enter(user, 'Capacity', '5000')
  await enter(user, 'Nominal voltage', '14.8')
  await enter(user, 'Usable fraction', '0.8')
  await enter(user, 'Where the pack figures come from', 'synthetic fixture assumption')
}

/** leg reads one mission leg's row out of the mission table. */
export function leg(name: string): {
  power: string
  time: string
  distance: string
  energy: string
  thrust: string
} {
  const row = screen.getByRole('rowheader', { name: new RegExp(name) }).closest('tr')
  if (row === null) throw new Error(`no mission row for ${name}`)
  const cells = within(row).getAllByRole('cell').map((cell) => cell.textContent ?? '')
  return {
    power: cells[0] ?? '',
    time: cells[1] ?? '',
    distance: cells[2] ?? '',
    energy: cells[3] ?? '',
    thrust: cells[4] ?? '',
  }
}

/**
 * reported returns one panel of the results column. The lookup is scoped to
 * that column because the entry-point picker offers a journey called "Mission
 * and energy" as well: the question and the answer to it share a name, and a
 * query that did not say which it meant would match both.
 */
export function reported(title: string): HTMLElement {
  const results = screen.getByRole('complementary', { name: /Results and requirements/ })
  const heading = within(results).getByText(title)
  const panel = heading.closest('details')
  if (panel === null) throw new Error(`no results panel titled ${title}`)
  return panel
}

/** valueBeside reads the figure next to a labelled term inside one container.
 * The same word — "Completeness" — labels the mass inventory and the electrical
 * budget, and they are different answers. */
export function valueBeside(panel: HTMLElement, term: string): string {
  const dt = within(panel).getByText(term)
  const dd = dt.nextElementSibling
  if (!(dd instanceof HTMLElement)) throw new Error(`no value beside ${term}`)
  return dd.textContent ?? ''
}

/** energyFit is the mission's separate statement about energy sufficiency. */
export function energyFit(): string {
  return valueBeside(reported('Mission and energy'), 'Does the energy fit')
}

/** fieldIssue reads the complaint shown against one labelled field, if any. */
export function fieldIssue(label: string): string {
  const field = screen.getByLabelText(label).closest('.field')
  if (!(field instanceof HTMLElement)) throw new Error(`no field row for ${label}`)
  return within(field).queryByRole('status')?.textContent ?? ''
}

