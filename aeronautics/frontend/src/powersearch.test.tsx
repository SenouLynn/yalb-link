import { screen, within } from '@testing-library/react'
import { beforeEach, describe, expect, test } from 'vitest'
import { memoryStorage } from './testing/storage.ts'
import {
  enter,
  enterIn,
  press,
  renderWorksheet,
  setSelect,
} from './testing/harness.tsx'
import {
  expand,
  fixtureAircraft,
  segmentPanel,
} from './testing/aircraft.tsx'
import type { UserEvent } from '@testing-library/user-event'

// The bounded power search, against the real Go service.
//
// A mass and a power ceiling do not determine a wing. At the fixture's 16 m/s
// the demand is not monotonic in wing area — a small wing pays induced drag and
// a large one pays parasite drag — so the wings under a ceiling form an
// interval. With the fixture aircraft, a 90 W ceiling and areas sampled every
// 0.05 m^2 from 0.10 m^2:
//
//   0.10  outside the polar (CL 1.56)  the parabola would say 76.2 W
//   0.15  66.7 W   0.20  66.0 W   0.25  68.8 W   0.30  73.3 W
//   0.35  78.8 W   0.40  84.9 W
//   0.45  91.4 W   0.50  98.3 W   0.55 105.3 W   0.60 112.5 W
//
// The 0.10 m^2 wing is the case the whole envelope check exists for: its
// arithmetic comes in under the ceiling, and the aircraft cannot fly there.

let storage: Storage

beforeEach(() => {
  storage = memoryStorage()
})

/** searchable builds the fixture aircraft with one cruise leg to demand power. */
async function searchable(user: UserEvent): Promise<void> {
  await fixtureAircraft(user)
  await user.type(screen.getByLabelText('Add a segment'), 'cruise')
  await setSelect(user, 'Kind of segment to add', 'cruise')
  await press(user, 'Add segment')
  const panel = segmentPanel('cruise')
  await expand(user, panel, 'cruise', '.segment-name')
  await enterIn(user, panel, 'Airspeed for cruise', '16')
  await enterIn(user, panel, 'Duration of cruise', '600')
}

/** search asks for the fixture's range against a ceiling in watts. */
async function search(user: UserEvent, ceiling: string): Promise<void> {
  await setSelect(user, 'Vary', 'wing.area.reference')
  await enter(user, 'Lowest', '0.1')
  await enter(user, 'Highest', '0.6')
  await enter(user, 'Candidates', '11')
  await enter(user, 'Power ceiling', ceiling)
  await press(user, 'Search the range')
}

/** candidate reads one evaluated wing's row out of the search table. */
function candidate(area: string): { demand: string; outcome: string } {
  const table = screen.getByRole('table', { name: /Every evaluated candidate/ })
  const row = within(table).getByRole('rowheader', { name: `${area} m^2` }).closest('tr')
  if (row === null) throw new Error(`no search row for ${area}`)
  const cells = within(row).getAllByRole('cell')
  return { demand: cells[0]?.textContent ?? '', outcome: cells[1]?.textContent ?? '' }
}

/** intervals lists the runs of candidates the search reported. */
function intervals(): string[] {
  return screen
    .getAllByRole('listitem')
    .filter((item) => item.closest('ol')?.className === 'intervals')
    .map((item) => item.textContent ?? '')
}

describe('the bounded power search', () => {
  test('the wings under a ceiling are reported as an interval, not as one answer', async () => {
    const user = renderWorksheet(storage)
    await searchable(user)

    // Before anything is asked for, the panel says what the search still needs
    // rather than complaining about a ceiling nobody has typed.
    expect(screen.getByText(/still needs a power ceiling/)).toBeInTheDocument()

    await search(user, '90')

    expect(candidate('0.2').demand).toBe('66.0304 W')
    expect(candidate('0.4').demand).toBe('84.9105 W')
    expect(candidate('0.45').outcome).toBe('over the ceiling')
    expect(candidate('0.6').outcome).toBe('over the ceiling')

    // One run, bracketed by the evaluated candidates at its ends. Nothing here
    // picks a wing: adopting one is the ordinary driver edit, made from the
    // table deliberately.
    const runs = intervals()
    expect(runs).toHaveLength(1)
    expect(runs[0]).toContain('0.15 m^2 to 0.4 m^2')
  }, 180_000)

  test('a candidate the aircraft cannot fly never counts as fitting the ceiling', async () => {
    const user = renderWorksheet(storage)
    await searchable(user)
    await search(user, '90')

    // The parabolic polar would return 76.2 W at 0.10 m^2, comfortably under
    // the ceiling. It is refused instead, because holding 16 m/s on that wing
    // needs CL 1.56 — past both the case's CLmax and the range the polar is
    // claimed over — and a power figure for it would make the impossible look
    // affordable.
    expect(candidate('0.1').demand).toBe('—')
    expect(candidate('0.1').outcome).not.toContain('fits')
    expect(intervals()[0]).not.toContain('0.1 m^2 to')
  }, 180_000)

  test('a ceiling no evaluated wing meets reports no interval rather than the nearest wing', async () => {
    const user = renderWorksheet(storage)
    await searchable(user)
    await search(user, '50')

    expect(intervals()).toHaveLength(0)
    expect(screen.getByText(/no evaluated candidate is feasible/i)).toBeInTheDocument()
    expect(screen.getByText(/not about every wing/)).toBeInTheDocument()
  }, 180_000)

  test('an answer to a different range says so rather than being read as this one', async () => {
    const user = renderWorksheet(storage)
    await searchable(user)
    await search(user, '90')
    expect(intervals()).toHaveLength(1)

    // A curve belongs to a question. Moving the ceiling asks a different one,
    // and the answer on screen is not an answer to it.
    await enter(user, 'Power ceiling', '70')
    expect(screen.getByText(/The controls have moved on since this was searched/)).toBeInTheDocument()
  }, 180_000)
})
