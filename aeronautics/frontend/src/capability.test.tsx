import { screen, within } from '@testing-library/react'
import type { UserEvent } from '@testing-library/user-event'
import { beforeEach, describe, expect, test } from 'vitest'
import { memoryStorage } from './testing/storage.ts'
import { enter, enterIn, open, press, renderWorksheet, selectIn, setSelect, settled } from './testing/harness.tsx'
import {
  addLoad,
  airframe,
  chain,
  expand,
  leg,
  measured,
  polar,
  reported,
  segmentPanel,
} from './testing/aircraft.tsx'

// Measured capability and thrust-to-weight targets, against the real Go service.
//
// A capability point is one condition and answers only questions at that
// condition. Two points are recorded for the fixture aircraft:
//
//   bench run   static, full throttle      20 N,  300 W
//   cruise run  in flight at 16 m/s        3 N,   160 W
//
// The aircraft weighs 24.516625 N, so those are thrust-to-weight ratios of
// 0.815773 and 0.122366. Nothing converts one into the other: that needs a
// propeller operating-point model with real blade data, which is not here.

let storage: Storage

beforeEach(() => {
  storage = memoryStorage()
})

/**
 * capable is what a thrust-to-weight target needs and no more: the mass the
 * ratio is taken against, and the two measured points. No pack and no mission,
 * because a target is a statement about thrust at a condition and says nothing
 * about how long the aircraft can hold it.
 */
async function capable(user: UserEvent): Promise<void> {
  await settled()
  await airframe(user)
  await measured(user, 'bench run', 'Static', '20', '300')
  await measured(user, 'cruise run', 'In flight', '3', '160', '16')
}

/** flying adds what a leg needs to establish a required thrust of its own. */
async function flying(user: UserEvent): Promise<void> {
  await polar(user)
  await chain(user)
  // An aircraft with no avionics at all says so, by listing a load of zero
  // watts with that as its basis. An empty list is a budget nobody has written
  // yet, and no complete electrical figure follows from one.
  await addLoad(user, 'nothing aboard', '0')
  await user.type(screen.getByLabelText('Add a segment'), 'cruise')
  await setSelect(user, 'Kind of segment to add', 'cruise')
  await press(user, 'Add segment')
  const panel = segmentPanel('cruise')
  await expand(user, panel, 'cruise', '.segment-name')
  await enterIn(user, panel, 'Airspeed for cruise', '16')
  await enterIn(user, panel, 'Duration of cruise', '600')
}

/** target states one thrust-to-weight target against one capability point. */
async function target(
  user: UserEvent,
  name: string,
  ratio: string,
  condition: string,
  priority: string,
): Promise<void> {
  await user.type(screen.getByLabelText('Add a thrust target'), name)
  await press(user, 'Add target')
  await enter(user, `Thrust-to-weight for ${name}`, ratio)
  if (condition !== '') await setSelect(user, `Condition for ${name}`, condition)
  await setSelect(user, `Priority for ${name}`, priority)
  await enter(user, `Where ${name} comes from`, 'synthetic fixture assumption')
}

/** checked reads one target's row out of the thrust results table. */
function checked(name: string): { wanted: string; available: string; status: string } {
  const table = within(reported('Thrust to weight')).getByRole('table', {
    name: /Thrust-to-weight targets/,
  })
  const row = within(table).getByRole('rowheader', { name: new RegExp(name) }).closest('tr')
  if (row === null) throw new Error(`no thrust row for ${name}`)
  const cells = within(row).getAllByRole('cell')
  return {
    wanted: cells[1]?.textContent ?? '',
    available: cells[2]?.textContent ?? '',
    status: cells[3]?.textContent ?? '',
  }
}

describe('measured capability', () => {
  test('a static point and a cruise point are never substituted for one another', async () => {
    const user = renderWorksheet(storage)
    await capable(user)
    await target(user, 'launch push', '0.8', 'bench run', 'required')
    await target(user, 'cruise margin', '0.8', 'cruise run', 'required')
    await open(user, 'Thrust to weight')

    // The same aircraft, the same motor, the same propeller, two conditions.
    // 0.8 is met on the bench and nowhere near met at 16 m/s, and the bench
    // figure is not borrowed to answer the cruise question.
    expect(checked('launch push')).toMatchObject({ available: '0.815773', status: 'met' })
    expect(checked('cruise margin').available).toBe('0.122366')
    expect(checked('cruise margin').status).toContain('unmet')
    expect(screen.getByText(/Static thrust is not cruise thrust/)).toBeInTheDocument()
  }, 180_000)

  test('a target with no condition is unknown rather than unmet', async () => {
    const user = renderWorksheet(storage)
    await capable(user)
    await target(user, 'somewhere', '0.8', '', 'required')
    await open(user, 'Thrust to weight')

    // A ratio with no condition behind it is not a failed check: it is a claim
    // nobody has stated the condition for, and saying "unmet" would report the
    // aircraft as falling short of something never asked of it.
    expect(checked('somewhere').status).toContain('unknown')
    expect(checked('somewhere').available).toBe('—')

    await setSelect(user, 'Condition for somewhere', 'bench run')
    expect(checked('somewhere').status).toBe('met')
  }, 180_000)

  test('a preferred target is assessed and says so, beside a required one', async () => {
    const user = renderWorksheet(storage)
    await capable(user)
    await target(user, 'cruise margin', '0.8', 'cruise run', 'required')
    await target(user, 'spirited climb', '2', 'bench run', 'preferred')
    await open(user, 'Thrust to weight')

    expect(checked('spirited climb').status).toContain('unmet')
    const table = within(reported('Thrust to weight')).getByRole('table', {
      name: /Thrust-to-weight targets/,
    })
    expect(within(table).getByRole('rowheader', { name: /spirited climb/ })).toHaveTextContent('(preferred)')
    expect(within(table).getByRole('rowheader', { name: /cruise margin/ })).toHaveTextContent('(required)')
  }, 180_000)

  test('a leg is answered only by a point taken at the condition it flies', async () => {
    const user = renderWorksheet(storage)
    await capable(user)
    await flying(user)
    const panel = segmentPanel('cruise')

    // The bench measured 20 N, eight times what the cruise needs. It is still
    // no answer about 16 m/s, and the refusal says why rather than rounding the
    // question off to the measurement that happens to exist.
    await selectIn(user, panel, 'Capability point to check cruise against', 'bench run')
    expect(leg('cruise').thrust).toMatch(/static measurement at zero airspeed/)

    await selectIn(user, panel, 'Capability point to check cruise against', 'cruise run')
    expect(leg('cruise').thrust).toBe('thrust available')

    // 2.6438 N is needed and the point delivers 3 N. Halve the point and the
    // same leg is refused, with both figures named.
    await enterIn(user, capabilityPanelFor('cruise run'), 'Thrust at cruise run', '1.5')
    expect(leg('cruise').thrust).toMatch(/not enough thrust/)
    expect(leg('cruise').thrust).toMatch(/delivers 1.5 N at that condition/)
  }, 180_000)
})

/** capabilityPanelFor is the point's editor, already open from `measured`. */
function capabilityPanelFor(name: string): HTMLElement {
  const label = screen.getByText(name, { selector: '.capability-name' })
  const panel = label.closest('details')
  if (panel === null) throw new Error(`no editor for the capability point ${name}`)
  return panel
}
