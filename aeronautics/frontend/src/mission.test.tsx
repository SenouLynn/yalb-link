import { screen, within } from '@testing-library/react'
import type { UserEvent } from '@testing-library/user-event'
import { beforeEach, describe, expect, test } from 'vitest'
import { memoryStorage } from './testing/storage.ts'
import {
  enter,
  enterIn,
  press,
  renderWorksheet,
  selectIn,
  setSelect,
} from './testing/harness.tsx'
import {
  energyFit,
  expand,
  fixtureAircraft,
  leg,
  reported,
  segmentPanel,
  valueBeside,
} from './testing/aircraft.tsx'

// A whole RC mission, against the real Go service. The aircraft, the legs and
// every figure asserted here are in calculator/testdata/electric-power-fixtures.md,
// which is independent of the book.
//
//   launch 12 s · climb 60 s · cruise 600 s · loiter 300 s ·
//   return 600 s into a 3 m/s headwind · recovery 30 s
//
// Two of those legs have no model behind them: no segment model estimates a
// launch or a recovery, so those carry figures the builder entered and say so.

let storage: Storage

beforeEach(() => {
  storage = memoryStorage()
})

/** addSegment writes one leg and opens its editor. */
async function addSegment(user: UserEvent, name: string, kind: string): Promise<HTMLElement> {
  await user.type(screen.getByLabelText('Add a segment'), name)
  await setSelect(user, 'Kind of segment to add', kind)
  await press(user, 'Add segment')
  const panel = segmentPanel(name)
  await expand(user, panel, name, '.segment-name')
  return panel
}

/** flown writes a leg the drag polar computes, at an airspeed and a duration. */
async function flown(
  user: UserEvent,
  name: string,
  kind: string,
  speed: string,
  seconds: string,
  options: { climb?: string; wind?: string } = {},
): Promise<void> {
  const panel = await addSegment(user, name, kind)
  await enterIn(user, panel, `Airspeed for ${name}`, speed)
  await enterIn(user, panel, `Duration of ${name}`, seconds)
  if (options.climb !== undefined) {
    // Every quantity comes back from the service in its SI unit, so the angle
    // field reads radians until it is told otherwise. Saying degrees here is
    // the same act a builder performs with the unit select beside the box.
    await selectIn(user, panel, `Unit for Flight-path angle for ${name}`, 'deg')
    await enterIn(user, panel, `Flight-path angle for ${name}`, options.climb)
  }
  if (options.wind !== undefined) {
    await enterIn(user, panel, `Wind along the track for ${name}`, options.wind)
  }
}

/** estimated writes a leg no model covers, carrying a figure the builder gave. */
async function estimated(
  user: UserEvent,
  name: string,
  kind: string,
  speed: string,
  seconds: string,
  watts: string,
): Promise<void> {
  const panel = await addSegment(user, name, kind)
  await selectIn(user, panel, `Where ${name}'s power comes from`, 'entered-estimate')
  await enterIn(user, panel, `Entered electrical power for ${name}`, watts)
  await enterIn(user, panel, `Where that figure for ${name} comes from`, 'synthetic fixture estimate')
  await enterIn(user, panel, `Airspeed for ${name}`, speed)
  await enterIn(user, panel, `Duration of ${name}`, seconds)
}

/** mission flies the six legs of the fixture, with the reserve stated first. */
async function mission(user: UserEvent): Promise<void> {
  await enter(user, 'Energy reserve', '0.2')
  await enter(user, 'Where the reserve comes from', 'synthetic fixture assumption')
  await estimated(user, 'launch', 'launch', '12', '12', '250')
  await flown(user, 'climb', 'climb', '14', '60', { climb: '10' })
  await flown(user, 'cruise', 'cruise', '16', '600')
  await flown(user, 'loiter', 'loiter', '16', '300')
  await flown(user, 'return', 'return', '16', '600', { wind: '-3' })
  await estimated(user, 'recovery', 'recovery', '10', '30', '20')
}

/** total reads one of the mission's reported totals. */
function total(term: string): string {
  return valueBeside(reported('Mission and energy'), term)
}

describe('an autopilot-equipped mission', () => {
  test('the six legs agree with the fixture and total to an energy the pack covers', async () => {
    const user = renderWorksheet(storage)
    await fixtureAircraft(user)
    await mission(user)

    // The 8 W of avionics is in every leg exactly once, the entered legs
    // included: an entered figure is the propulsion draw, so switching a leg
    // between the two models cannot change whether the autopilot was counted.
    expect(leg('launch')).toMatchObject({ power: '258 W', time: '12 s', energy: '3096 J' })
    expect(leg('climb')).toMatchObject({
      power: '173.613 W',
      time: '60 s',
      distance: '827.239 m',
      energy: '10416.8 J',
    })
    expect(leg('cruise')).toMatchObject({ power: '84.9105 W', distance: '9600 m', energy: '50946.3 J' })
    expect(leg('loiter')).toMatchObject({ power: '84.9105 W', distance: '4800 m', energy: '25473.1 J' })
    expect(leg('recovery')).toMatchObject({ power: '28 W', time: '30 s', energy: '840 J' })

    expect(total('Duration')).toBe('1602 s')
    expect(total('Ground distance')).toBe('23471.2 m')
    expect(total('Energy the mission needs')).toContain('141718 J')
    expect(energyFit()).toContain('yes')

    // The worst continuous moment of this mission is on a leg no model
    // computed, which is a fact about the evidence rather than about the
    // aircraft, and is why the entered legs are labelled in the table.
    expect(total('Largest continuous demand')).toBe('258 W')
    expect(within(legRow('launch')).getByText('entered estimate')).toBeInTheDocument()
    expect(within(legRow('cruise')).queryByText('entered estimate')).not.toBeInTheDocument()
  }, 240_000)

  test('the return leg spends the same energy whichever way the wind blows', async () => {
    const user = renderWorksheet(storage)
    await fixtureAircraft(user)
    await mission(user)

    // The aircraft flies the same airspeed for the same time; only the ground
    // track changes. That is why a return leg is stated with its own wind
    // rather than inferred from the outbound one.
    expect(leg('return')).toMatchObject({ distance: '7800 m', energy: '50946.3 J' })

    const panel = segmentPanel('return')
    await enterIn(user, panel, 'Wind along the track for return', '3')
    expect(leg('return')).toMatchObject({ distance: '11400 m', energy: '50946.3 J' })

    await enterIn(user, panel, 'Wind along the track for return', '0')
    expect(leg('return')).toMatchObject({ distance: '9600 m', energy: '50946.3 J' })
  }, 240_000)

  test('the reserve is held back once, against the total and never inside a leg', async () => {
    const user = renderWorksheet(storage)
    await fixtureAircraft(user)
    await mission(user)

    expect(total('Energy the mission needs')).toContain('141718 J')
    expect(total('Energy the pack offers')).toContain('170496 J')

    // Raising the reserve takes more of the same pack off the table. What the
    // mission needs is a property of the flight and does not move with it.
    await enter(user, 'Energy reserve', '0.5')
    expect(total('Energy the mission needs')).toContain('141718 J')
    expect(total('Energy the pack offers')).toContain('106560 J')
    expect(energyFit()).toContain('no')

    // The usable fraction is the pack's own property and is a separate
    // statement from the reserve, so they multiply rather than merging.
    await enter(user, 'Energy reserve', '0.2')
    await enter(user, 'Usable fraction', '1')
    expect(total('Energy the pack offers')).toContain('213120 J')
    expect(total('Energy the pack offers')).toContain('266400 J')
    expect(energyFit()).toContain('yes')
  }, 240_000)

  test('energy sufficiency is not flight feasibility, and a leg says which it answers', async () => {
    const user = renderWorksheet(storage)
    await fixtureAircraft(user)
    await mission(user)

    // No capability point is stated, so nothing is claimed about thrust on any
    // leg — including the ones whose energy is fully computed.
    expect(leg('cruise').thrust).toMatch(/names no capability point/)
    expect(leg('cruise').thrust).toMatch(/Its energy still counts/)
    expect(energyFit()).toContain('yes')
    expect(energyFit()).toMatch(/not flight feasibility/)
  }, 240_000)
})

/** legRow returns one leg's row in the mission table. */
function legRow(name: string): HTMLElement {
  const row = screen.getByRole('rowheader', { name: new RegExp(name) }).closest('tr')
  if (row === null) throw new Error(`no mission row for ${name}`)
  return row
}
