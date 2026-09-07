import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { UserEvent } from '@testing-library/user-event'
import { beforeEach, describe, expect, inject, test } from 'vitest'
import { httpTransport } from './api/client.ts'
import { memoryStorage } from './testing/storage.ts'
import { Worksheet } from './Worksheet.tsx'

// These run against the real Go service, started once for the suite. The
// physics has to come from the calculator for a journey test to mean anything,
// and a stub would drift from it the moment an equation changed.
//
// The expected numbers below were computed independently, not read back from
// this system:
//
//   S     = b^2/A = 1.44/6                             = 0.24 m^2
//   Vs    = sqrt(2 n m g/(rho S CLmax))                = 10.5445013 m/s
//   S_min = 2 n m g/(rho Vs_limit^2 CLmax) at 8 m/s    = 0.4169494 m^2
//   m_max = rho Vs^2 S CLmax/(2 n g) at S = 0.24 m^2   = 1.1512188 kg
//   n = 2 doubles S_min and halves m_max.
//
// with g = 9.80665, rho = 1.225, CLmax = 1.2, m = 2 kg.

let storage: Storage

beforeEach(() => {
  storage = memoryStorage()
})

function renderWorksheet(): UserEvent {
  const base = inject('aeroBaseUrl')
  render(<Worksheet transport={httpTransport(base)} session="browser-test" storage={storage} />)
  return userEvent.setup()
}

/** settled waits for the worksheet to be showing a current result. */
async function settled(): Promise<void> {
  // The headline says one of "Calculating…", "Showing an earlier revision",
  // "The last request was refused" or "Current". Waiting for the last of those
  // is what makes every assertion below about a result that describes the
  // design as it now stands.
  await waitFor(
    () => {
      expect(screen.getByText('Current')).toBeInTheDocument()
    },
    { timeout: 15_000 },
  )
}

/** enter commits a value into a field and waits for the result to settle. */
async function enter(user: UserEvent, label: string, text: string): Promise<void> {
  const field = screen.getByLabelText(label)
  await user.clear(field)
  await user.type(field, `${text}{Enter}`)
  await settled()
}

/** setSelect chooses an option and waits. */
async function setSelect(user: UserEvent, label: string, value: string): Promise<void> {
  await user.selectOptions(screen.getByLabelText(label), value)
  await settled()
}

/** press clicks a button by its accessible name and waits. */
async function press(user: UserEvent, name: string | RegExp): Promise<void> {
  await user.click(screen.getByRole('button', { name }))
  await settled()
}

/**
 * results scopes a query to the results panel. The same parameter keys appear
 * in the dimension views' own table, so a query that did not say which table it
 * meant would match both and stop meaning anything.
 */
function results(): HTMLElement {
  return screen.getByRole('complementary', { name: /Results and requirements/ })
}

/** checkRow returns the requirement row for a named requirement. */
function checkRow(name: string): HTMLElement {
  const row = within(results()).getByRole('rowheader', { name: new RegExp(name) }).closest('tr')
  if (row === null) throw new Error(`no requirement row for ${name}`)
  return row
}

/** parameterValue reads a solved parameter out of the geometry table. */
function parameterValue(key: string): string {
  const row = within(results()).getByRole('rowheader', { name: key }).closest('tr')
  if (row === null) throw new Error(`no parameter row for ${key}`)
  const cells = within(row).getAllByRole('cell')
  return cells[0]?.textContent ?? ''
}

/** rowFor returns the field row a labelled control belongs to. */
function rowFor(label: string): HTMLElement {
  const row = screen.getByLabelText(label).closest('.field')
  if (!(row instanceof HTMLElement)) throw new Error(`no field row for ${label}`)
  return row
}

/** buildBaseCandidate enters the shared fixture: 2 kg, CLmax 1.2, span and AR. */
async function buildBaseCandidate(user: UserEvent): Promise<void> {
  await enter(user, 'All-up mass', '2')
  await enter(user, 'Where it comes from', 'target all-up mass')
  await enter(user, 'Whole-aircraft CLmax', '1.2')
  await enter(user, 'Where the coefficient comes from', 'assumed for initial sizing')
  await enter(user, 'Span', '1.2')
  await enter(user, 'Aspect ratio', '6')
}

describe('the span-first journey', () => {
  test('a span and an aspect ratio give the area, the stall speed and the required area', async () => {
    const user = renderWorksheet()
    await settled()

    await press(user, /Span first/)
    expect(screen.getByText(/Enter the span you have chosen to use/)).toBeInTheDocument()

    await buildBaseCandidate(user)
    expect(parameterValue('wing.area.reference')).toBe('0.24 m^2')

    // The wing area is derived, so it is readable and says what it followed
    // from rather than being editable.
    const area = screen.getByLabelText('Wing area')
    expect(area).toHaveAttribute('readonly')
    expect(screen.getByText(/Follows from geometry.area-from-span-aspect/)).toBeInTheDocument()

    await enter(user, 'Highest acceptable stall speed', '8')
    const stall = checkRow('Stall ceiling')
    expect(within(stall).getByText('unmet')).toBeInTheDocument()
    expect(within(stall).getByText('10.5445 m/s')).toBeInTheDocument()
    expect(screen.getByText(/At least one required requirement is unmet/)).toBeInTheDocument()
    expect(screen.getByText(/minimum 0.416949 m\^2/)).toBeInTheDocument()
  }, 60_000)

  test('a maximum span stays separate from the span, and is adopted only on request', async () => {
    const user = renderWorksheet()
    await settled()
    await buildBaseCandidate(user)

    await enter(user, 'Maximum span', '1.4')
    // Entering the limit does not move the span.
    expect(parameterValue('wing.span.projected')).toBe('1.2 m')

    await press(user, /Use maximum/)
    expect(parameterValue('wing.span.projected')).toBe('1.4 m')

    // And it stays put: editing something else does not recopy the maximum.
    await enter(user, 'All-up mass', '1.8')
    expect(parameterValue('wing.span.projected')).toBe('1.4 m')
  }, 60_000)

  test('sizing at the stall limit puts the wing on the boundary, keeping the span', async () => {
    const user = renderWorksheet()
    await settled()
    await buildBaseCandidate(user)
    await enter(user, 'Highest acceptable stall speed', '8')

    await press(user, /wing.span.projected, all required cases/)
    expect(parameterValue('wing.area.reference')).toBe('0.416949 m^2')
    expect(parameterValue('wing.span.projected')).toBe('1.2 m')
    expect(parameterValue('wing.aspect_ratio.planform')).toBe('3.45366')

    const stall = checkRow('Stall ceiling')
    expect(within(stall).getByText('met')).toBeInTheDocument()
    expect(within(stall).getByText('8 m/s')).toBeInTheDocument()
    expect(screen.getByText(/Every required requirement is met/)).toBeInTheDocument()
  }, 60_000)
})

describe('the weight-first journeys', () => {
  test('mass and performance first: the wing follows from the mass and the ceiling', async () => {
    const user = renderWorksheet()
    await settled()
    await press(user, /Mass and performance first/)

    await enter(user, 'All-up mass', '2')
    await enter(user, 'Where it comes from', 'target all-up mass')
    await enter(user, 'Whole-aircraft CLmax', '1.2')
    await enter(user, 'Where the coefficient comes from', 'assumed for initial sizing')
    await enter(user, 'Highest acceptable stall speed', '8')

    // With no geometry yet, the area the cases demand is already known: it
    // needs the mass and the case, not a wing.
    expect(screen.getByText(/minimum 0.416949 m\^2/)).toBeInTheDocument()

    await enter(user, 'Span', '1.2')
    await enter(user, 'Aspect ratio', '6')
    await press(user, /wing.span.projected, all required cases/)
    expect(parameterValue('wing.area.reference')).toBe('0.416949 m^2')
  }, 60_000)

  test('mass and size first: the wing is given and the mass ceiling follows', async () => {
    const user = renderWorksheet()
    await settled()
    await press(user, /Mass and available size first/)

    await enter(user, 'All-up mass', '2')
    await enter(user, 'Where it comes from', 'target all-up mass')
    await enter(user, 'Whole-aircraft CLmax', '1.2')
    await enter(user, 'Where the coefficient comes from', 'assumed for initial sizing')
    await enter(user, 'Span', '1.2')
    await enter(user, 'Wing area', '0.24')
    await enter(user, 'Highest acceptable stall speed', '8')

    expect(parameterValue('wing.aspect_ratio.planform')).toBe('6')
    const stall = checkRow('Stall ceiling')
    expect(within(stall).getByText('10.5445 m/s')).toBeInTheDocument()
    // A ceiling bounds the mass from above and justifies no lower bound at all.
    expect(screen.getByText(/an upper mass bound alone is not a mass range/i)).toBeInTheDocument()
  }, 60_000)

  test('the same candidate reached two ways gives the same outputs', async () => {
    const readings: string[][] = []
    for (const route of ['span and aspect ratio', 'span and area'] as const) {
      storage = memoryStorage()
      const user = renderWorksheet()
      await settled()
      await enter(user, 'All-up mass', '2')
      await enter(user, 'Where it comes from', 'target all-up mass')
      await enter(user, 'Whole-aircraft CLmax', '1.2')
      await enter(user, 'Where the coefficient comes from', 'assumed for initial sizing')
      await enter(user, 'Span', '1.2')
      if (route === 'span and aspect ratio') {
        await enter(user, 'Aspect ratio', '6')
      } else {
        await enter(user, 'Wing area', '0.24')
      }
      await enter(user, 'Highest acceptable stall speed', '8')
      readings.push([
        parameterValue('wing.area.reference'),
        parameterValue('wing.aspect_ratio.planform'),
        parameterValue('wing.chord.root'),
        within(checkRow('Stall ceiling')).getAllByRole('cell')[2]?.textContent ?? '',
      ])
      cleanup()
    }
    expect(readings[0]).toEqual(readings[1])
  }, 120_000)
})

describe('editing', () => {
  test('unfinished text is left alone, Escape restores, and an empty field is not zero', async () => {
    const user = renderWorksheet()
    await settled()
    await buildBaseCandidate(user)

    const span = screen.getByLabelText('Span')
    await user.clear(span)
    await user.type(span, '1.')
    // Half a number is an editing state, not an error.
    expect(span).toHaveValue('1.')
    expect(screen.getByText('Still being typed; press Enter to use it.')).toBeInTheDocument()
    expect(screen.queryByText(/is not a number/)).not.toBeInTheDocument()

    await user.keyboard('{Escape}')
    expect(span).toHaveValue('1.2')

    await user.clear(span)
    await user.type(span, '-{Enter}')
    // An invalid commit keeps what was typed and says why.
    expect(span).toHaveValue('-')
    expect(screen.getByText(/is not a number/)).toBeInTheDocument()
    expect(parameterValue('wing.span.projected')).toBe('1.2 m')

    await user.clear(span)
    await user.type(span, '{Enter}')
    // Clearing withdraws the value; it never becomes zero.
    await waitFor(() => {
      expect(screen.getByText(/a planform needs exactly two/i)).toBeInTheDocument()
    })
  }, 60_000)

  test('editing is possible with the keyboard alone, and focus survives a refusal', async () => {
    const user = renderWorksheet()
    await settled()

    const mass = screen.getByLabelText('All-up mass')
    mass.focus()
    await user.keyboard('2{Enter}')
    await settled()

    // Tab reaches the unit, then the basis, without a pointer anywhere.
    await user.tab()
    expect(screen.getByLabelText('Unit for All-up mass')).toHaveFocus()

    const span = screen.getByLabelText('Span')
    span.focus()
    await user.keyboard('nonsense{Enter}')
    expect(span).toHaveFocus()
    expect(screen.getByText(/is not a number/)).toBeInTheDocument()
  }, 60_000)

  test('choosing a unit changes what is shown and not what is stored', async () => {
    const user = renderWorksheet()
    await settled()
    await buildBaseCandidate(user)
    expect(parameterValue('wing.span.projected')).toBe('1.2 m')

    await user.selectOptions(screen.getByLabelText('Unit for Span'), 'mm')
    expect(screen.getByLabelText('Span')).toHaveValue('1200')
    // The stored value is untouched, so the result is not stale and nothing
    // was recalculated.
    expect(screen.getByText('Current')).toBeInTheDocument()
    expect(parameterValue('wing.span.projected')).toBe('1.2 m')

    // And entering in that unit is converted by the service, not by the page.
    await enter(user, 'Span', '1400')
    expect(parameterValue('wing.span.projected')).toBe('1.4 m')
  }, 60_000)

  test('a derived value can be promoted, giving up a named input', async () => {
    const user = renderWorksheet()
    await settled()
    await buildBaseCandidate(user)

    const areaRow = rowFor('Wing area')
    await user.click(within(areaRow).getByRole('button', { name: 'Use as input' }))
    // The swap is deliberate: with two inputs held, the worksheet asks which
    // one is given up rather than choosing.
    expect(screen.getByText('Give up:')).toBeInTheDocument()
    await press(user, 'Give up wing.aspect_ratio.planform')

    expect(screen.getByLabelText('Wing area')).not.toHaveAttribute('readonly')
    expect(screen.getByLabelText('Aspect ratio')).toHaveAttribute('readonly')
    expect(parameterValue('wing.area.reference')).toBe('0.24 m^2')
  }, 60_000)

  test('undo and redo restore the whole state, roles included', async () => {
    const user = renderWorksheet()
    await settled()
    await buildBaseCandidate(user)
    await enter(user, 'Highest acceptable stall speed', '8')
    await press(user, /wing.span.projected, all required cases/)
    expect(parameterValue('wing.area.reference')).toBe('0.416949 m^2')

    await press(user, 'Undo')
    expect(parameterValue('wing.area.reference')).toBe('0.24 m^2')
    expect(screen.getByLabelText('Aspect ratio')).not.toHaveAttribute('readonly')
    expect(screen.getByLabelText('Wing area')).toHaveAttribute('readonly')

    await press(user, 'Redo')
    expect(parameterValue('wing.area.reference')).toBe('0.416949 m^2')
    expect(screen.getByLabelText('Wing area')).not.toHaveAttribute('readonly')
  }, 60_000)
})

describe('requirements and cases', () => {
  test('a second required case controls the bound, and preferring it steps back', async () => {
    const user = renderWorksheet()
    await settled()
    await buildBaseCandidate(user)
    await enter(user, 'Highest acceptable stall speed', '8')
    expect(screen.getByText(/minimum 0.416949 m\^2/)).toBeInTheDocument()

    await press(user, /Add a manoeuvre case at n = 2/)
    // The n = 2 case demands twice the area and is named as what controls it.
    expect(screen.getByText(/minimum 0.833899 m\^2 — set by Manoeuvre 2/)).toBeInTheDocument()

    await setSelect(user, 'Priority for Manoeuvre 2', 'preferred')
    expect(screen.getByText(/minimum 0.416949 m\^2 — set by Level flight/)).toBeInTheDocument()
    // The preferred case is still assessed; it has simply stopped narrowing.
    const rows = screen.getAllByRole('rowheader', { name: /Stall ceiling/ })
    expect(rows).toHaveLength(2)
  }, 90_000)

  test('withdrawing a coefficient makes its check unknown and the bound partial', async () => {
    const user = renderWorksheet()
    await settled()
    await buildBaseCandidate(user)
    await enter(user, 'Highest acceptable stall speed', '8')

    const clmax = screen.getByLabelText('Whole-aircraft CLmax')
    await user.clear(clmax)
    await user.type(clmax, '{Enter}')
    await settled()

    const stall = checkRow('Stall ceiling')
    expect(within(stall).getByText(/unknown/)).toBeInTheDocument()
    expect(screen.getByText(/not bounded|partial/i)).toBeInTheDocument()
  }, 60_000)

  test('an infeasible wing is explained with the alternatives that would resolve it', async () => {
    const user = renderWorksheet()
    await settled()
    await buildBaseCandidate(user)
    await enter(user, 'Highest acceptable stall speed', '8')
    // The wing must be at least 0.4169494 m^2 to hold the ceiling, and the
    // sheet stock allows 0.3 m^2. Both are required, and they cannot both hold.
    await enter(user, 'Maximum wing area', '0.3')

    await waitFor(() => {
      expect(screen.getByText(/the wing area must be at least/)).toBeInTheDocument()
    })
    // The group is a known conflicting one, and the worksheet says so rather
    // than claiming it is the smallest.
    expect(screen.getByText(/known conflicting group, not a minimal one/)).toBeInTheDocument()
    expect(screen.getByText(/mass at 1.15121881/)).toBeInTheDocument()
  }, 90_000)
})

describe('layout and coverage', () => {
  test('every configuration can be chosen and none reports a handling result', async () => {
    const user = renderWorksheet()
    await settled()
    await buildBaseCandidate(user)

    for (const configuration of ['v-tail', 'flying-wing', 'conventional-tail']) {
      await setSelect(user, 'Configuration', configuration)
      // Whatever the layout, the wing still sizes.
      expect(parameterValue('wing.area.reference')).toBe('0.24 m^2')
      expect(screen.getByText(/Handling is unknown for every configuration/)).toBeInTheDocument()
    }

    // A conventional layout with no tail described is incomplete, and says so
    // without withholding the wing.
    expect(screen.getByText(/a conventional tail needs a horizontal surface/i)).toBeInTheDocument()
    expect(screen.getByText(/does not affect the wing sizing/)).toBeInTheDocument()
  }, 90_000)
})

describe('drafts', () => {
  test('a draft saves and reopens with its inputs, roles and unfinished text', async () => {
    const user = renderWorksheet()
    await settled()
    await buildBaseCandidate(user)
    await enter(user, 'Highest acceptable stall speed', '8')

    // Leave one field holding text that cannot be committed. Blurring it will
    // not turn it into a value, so it is exactly the unfinished entry that has
    // to survive the round trip separately from the span itself.
    const span = screen.getByLabelText('Span')
    await user.clear(span)
    await user.type(span, '1.2.3')

    await user.type(screen.getByLabelText('Name'), 'trainer')
    await user.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => {
      expect(screen.getByText(/Saved “trainer”/)).toBeInTheDocument()
    })

    // Move the design somewhere else, then reopen.
    await enter(user, 'All-up mass', '3')
    await press(user, 'Open')
    await waitFor(() => {
      expect(screen.getByText(/nothing cached was shown as current/)).toBeInTheDocument()
    })
    await settled()
    expect(screen.getByLabelText('Span')).toHaveValue('1.2.3')
    expect(parameterValue('wing.span.projected')).toBe('1.2 m')
    expect(within(checkRow('Stall ceiling')).getByText('unmet')).toBeInTheDocument()
  }, 90_000)

  test('a draft this worksheet cannot read leaves the open design untouched', async () => {
    const user = renderWorksheet()
    await settled()
    await buildBaseCandidate(user)

    storage.setItem('yalb.aero.draft.broken', '{"schema":99}')
    // The panel reads its list from storage when a save happens, so save one
    // first to make the broken entry reachable.
    await user.type(screen.getByLabelText('Name'), 'good')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    const broken = screen.getAllByRole('listitem').find((item) => item.textContent?.startsWith('broken'))
    expect(broken).toBeDefined()
    if (broken === undefined) throw new Error('the broken draft is not listed')
    await user.click(within(broken).getByRole('button', { name: 'Open' }))

    await waitFor(() => {
      expect(screen.getByText(/format 99/)).toBeInTheDocument()
    })
    expect(screen.getByText(/The design you had open is untouched/)).toBeInTheDocument()
    expect(parameterValue('wing.span.projected')).toBe('1.2 m')
  }, 90_000)
})
