import { screen, within } from '@testing-library/react'
import type { UserEvent } from '@testing-library/user-event'
import { beforeEach, describe, expect, test } from 'vitest'
import { memoryStorage } from './testing/storage.ts'
import {
  componentPanel,
  enter,
  enterIn,
  open,
  press,
  renderWorksheet,
  section,
  selectIn,
  setSelect,
  settled,
} from './testing/harness.tsx'
import {
  energyFit,
  expand,
  fieldIssue,
  fixtureAircraft,
  leg,
  loadPanel,
  polar,
  reported,
  segmentPanel,
  valueBeside,
} from './testing/aircraft.tsx'

// The power-first journey, against the real Go service. The aircraft, and the
// figures every assertion here is checked against, are in testing/aircraft.tsx.

let storage: Storage

beforeEach(() => {
  storage = memoryStorage()
})

/** cruise writes the 600 s leg at 16 m/s and the 20% reserve. */
async function cruise(user: UserEvent): Promise<void> {
  await enter(user, 'Energy reserve', '0.2')
  await enter(user, 'Where the reserve comes from', 'synthetic fixture assumption')
  await user.type(screen.getByLabelText('Add a segment'), 'cruise')
  await setSelect(user, 'Kind of segment to add', 'cruise')
  await press(user, 'Add segment')
  const panel = segmentPanel('cruise')
  await expand(user, panel, 'cruise', '.segment-name')
  await enterIn(user, panel, 'Airspeed for cruise', '16')
  await enterIn(user, panel, 'Duration of cruise', '600')
}

/** ratingRow returns one supply check's row. */
function ratingRow(name: string): HTMLElement {
  const row = within(reported('Supply and ratings')).getByRole('rowheader', { name }).closest('tr')
  if (row === null) throw new Error(`no supply check for ${name}`)
  return row
}

/** loadRow returns one listed load's row in the electrical budget. */
function loadRow(name: string): HTMLElement {
  const row = within(reported('Electrical budget')).getByRole('rowheader', { name }).closest('tr')
  if (row === null) throw new Error(`no budget row for ${name}`)
  return row
}

/** listedTotal reads the component inventory's total, which is the all-up mass
 * once the inventory is adopted. */
function listedTotal(): string {
  return valueBeside(section('Components and balance'), 'Total of the listed masses')
}

/** balance reads the reported centre of gravity. */
function balance(): string {
  return valueBeside(section('Components and balance'), 'Mechanical centre of gravity')
}

/**
 * stallSpeed reads the stall-speed check. It stands in for the wing loading
 * here: at a fixed area the stall speed is what a change in mass moves, so it
 * is the loading seen through the requirement the builder actually set.
 */
function stallSpeed(): string {
  const results = screen.getByRole('complementary', { name: /Results and requirements/ })
  const row = within(results).getByRole('rowheader', { name: /Stall ceiling/ }).closest('tr')
  if (row === null) throw new Error('no stall-speed row')
  return within(row).getAllByRole('cell')[2]?.textContent ?? ''
}

/** addComponent lists a component with a mass, a basis and a position. */
async function addComponent(
  user: UserEvent,
  name: string,
  role: string,
  mass: string,
  x: string,
): Promise<void> {
  await user.type(screen.getByLabelText('Add a component'), name)
  await press(user, 'Add component')
  const panel = componentPanel(name)
  await user.click(within(panel).getByText(name, { selector: '.component-name' }))
  await selectIn(user, panel, 'What it is', role)
  await enterIn(user, panel, 'Mass', mass)
  await enterIn(user, panel, 'Where the mass comes from', 'synthetic fixture')
  await enterIn(user, panel, 'Position x', x)
  await enterIn(user, panel, 'Position y', '0')
  await enterIn(user, panel, 'Position z', '0')
}

describe('the power-first journey', () => {
  test('the fixture cruise agrees with the independently computed figures', async () => {
    const user = renderWorksheet(storage)
    await fixtureAircraft(user)
    await cruise(user)

    // 84.91046324557467 W at the pack: the polar's drag at 16 m/s through the
    // chain efficiency, plus the avionics draw, which is added once.
    const cruised = leg('cruise')
    expect(cruised.power).toBe('84.9105 W')
    expect(cruised.time).toBe('600 s')
    expect(cruised.distance).toBe('9600 m')
    expect(cruised.energy).toBe('50946.3 J')

    // E_usable = 266400 J x 0.8, and the 20% reserve is taken once, off that.
    expect(valueBeside(reported('Mission and energy'), 'Energy the pack offers')).toContain('170496 J')
    expect(valueBeside(reported('Mission and energy'), 'Energy the pack offers')).toContain('213120 J')
    expect(energyFit()).toContain('yes')
  }, 120_000)

  test('a leg the polar does not cover is never presented as power-feasible', async () => {
    const user = renderWorksheet(storage)
    await fixtureAircraft(user)
    await cruise(user)

    // 8 m/s needs CL = 1.56, past the 1.1 the polar is claimed over. The
    // parabola would still return a number there; it would not describe the
    // aircraft, so no power for the leg follows and no energy from it either.
    await enterIn(user, segmentPanel('cruise'), 'Airspeed for cruise', '8')

    // Both refusals are stated, because they are different facts: the aircraft
    // cannot hold the condition at all, and the polar was never claimed there.
    const refused = leg('cruise')
    expect(refused.power).toMatch(/above the case's CLmax of 1.2/)
    expect(refused.power).toMatch(/outside the range the polar is claimed over/)
    expect(refused.energy).toBe('not available')
    expect(energyFit()).toContain('unknown')

    // The ground track is not a power question, so it is still reported; what
    // does not follow is an energy for a leg the aircraft cannot fly.
    expect(refused.distance).toBe('4800 m')
  }, 120_000)

  test('the energy fits while a peak the pack cannot supply does not', async () => {
    const user = renderWorksheet(storage)
    await fixtureAircraft(user)
    await cruise(user)

    // A peak carries no energy of its own — it has no stated duty cycle — so
    // adding one leaves the mission's energy exactly where it was and changes
    // only what the supply is asked for at the worst moment.
    await enterIn(user, loadPanel('receiver and servos'), 'Peak draw for receiver and servos', '40')
    await enter(user, 'Continuous current rating', '20')
    await enter(user, 'Peak current rating', '6')
    await open(user, 'Supply and ratings')

    expect(leg('cruise').energy).toBe('50946.3 J')
    expect(energyFit()).toContain('yes')

    // 76.9105 W of propulsion plus the 40 W peak, at 14.8 V, is 7.90 A.
    expect(within(ratingRow('pack continuous current')).getByText('within the limit')).toBeInTheDocument()
    expect(within(ratingRow('pack peak current')).getByText('over the limit')).toBeInTheDocument()
    expect(within(reported('Supply and ratings')).getByText(/every auxiliary/)).toBeInTheDocument()
  }, 120_000)

  test('a listed load with no stated draw blocks the completeness claim', async () => {
    const user = renderWorksheet(storage)
    await fixtureAircraft(user)
    await cruise(user)
    await open(user, 'Electrical budget')
    expect(valueBeside(reported('Electrical budget'), 'Completeness')).toContain('every listed load states its draw')

    // The payload is named but not measured. The mission's energy is not
    // thereby wrong, but no complete electrical feasibility claim rests on it.
    await user.type(screen.getByLabelText('Add an electrical load'), 'payload')
    await press(user, 'Add load')
    expect(valueBeside(reported('Electrical budget'), 'Completeness')).toContain('incomplete')
    expect(within(loadRow('payload')).getByText('not stated')).toBeInTheDocument()
  }, 120_000)

  test('withdrawing the polar withdraws the number, and restoring it brings it back', async () => {
    const user = renderWorksheet(storage)
    await fixtureAircraft(user)
    await cruise(user)
    expect(leg('cruise').power).toBe('84.9105 W')

    // Losing the provenance is not losing the arithmetic: the drag relation
    // still has every number it needs. What is lost is the claim that those
    // numbers describe this aircraft, and that is said on the box that is now
    // empty rather than on the coefficient beside it.
    await enter(user, 'Where CD0 comes from', '')
    expect(leg('cruise').power).toBe('84.9105 W')
    expect(fieldIssue('Where CD0 comes from')).toMatch(/state where CD0 came from/)
    await enter(user, 'Where CD0 comes from', 'synthetic fixture assumption')
    expect(fieldIssue('Where CD0 comes from')).toBe('')

    // Withdrawing the polar itself is a different matter: a leg computed from a
    // polar the design no longer states has no power and no energy.
    await press(user, 'Withdraw the polar')
    expect(leg('cruise').power).toMatch(/the design states none/)
    expect(leg('cruise').energy).toBe('not available')
    expect(energyFit()).toContain('unknown')

    await polar(user)
    expect(leg('cruise').power).toBe('84.9105 W')
    expect(energyFit()).toContain('yes')
  }, 150_000)
})

describe('the two mass modes', () => {
  test('adopting the components counts the pack once and leaves the cruise where it was', async () => {
    const user = renderWorksheet(storage)
    await fixtureAircraft(user)
    await cruise(user)
    await enter(user, 'Highest acceptable stall speed', '12')

    await addComponent(user, 'airframe', 'airframe', '2', '0.4')
    await addComponent(user, 'battery', 'battery', '0.5', '0.2')
    await setSelect(user, 'Which component is this pack', 'battery')

    // The pack's mass belongs to the component it names. Adopting the inventory
    // must therefore total 2.5 kg and not 3 kg, and every downstream number
    // stays exactly where the entered total put it.
    const loading = stallSpeed()
    await setSelect(user, 'Where the all-up mass comes from', 'components')
    expect(listedTotal()).toContain('2.5 kg')
    expect(stallSpeed()).toBe(loading)
    expect(leg('cruise').power).toBe('84.9105 W')

    // Growing the pack is not moving it: the mass, the balance, the loading and
    // the power the cruise needs all move together.
    const balanced = balance()
    await enterIn(user, componentPanel('battery'), 'Mass', '0.8')
    expect(listedTotal()).toContain('2.8 kg')
    expect(balance()).not.toBe(balanced)
    expect(stallSpeed()).not.toBe(loading)
    expect(Number(leg('cruise').power.replace(' W', ''))).toBeGreaterThan(84.9105)
  }, 150_000)

  test('a power design survives saving and reopening the draft', async () => {
    const user = renderWorksheet(storage)
    await fixtureAircraft(user)
    await cruise(user)

    await open(user, 'Drafts')
    await user.type(screen.getByLabelText('Name'), 'power')
    await press(user, 'Save')

    // Move the leg somewhere else, then reopen. The saved mission comes back
    // and is recalculated rather than restored from a cached number.
    await enterIn(user, segmentPanel('cruise'), 'Airspeed for cruise', '20')
    expect(leg('cruise').power).not.toBe('84.9105 W')

    await press(user, 'Open')
    await settled()
    expect(leg('cruise').power).toBe('84.9105 W')
    expect(valueBeside(reported('Mission and energy'), 'Energy the pack offers')).toContain('170496 J')
  }, 150_000)
})
