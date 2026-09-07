import { screen, waitFor, within } from '@testing-library/react'
import { describe, expect, test } from 'vitest'
import {
  buildBaseCandidate,
  enter,
  press,
  renderWorksheet,
  rowFor,
  section,
  setSelect,
} from './testing/harness.tsx'

// The sensitivity view, against the real Go service.
//
// Expected values, computed independently at the fixture's conditions
// (m = 2 kg, rho = 1.225 kg/m^3, CLmax = 1.2, n = 1, b = 1.2 m):
//
//   S  = b^2/A,  Vs = sqrt(2 n m g / (rho S CLmax))
//   A = 4  -> S = 0.36  m^2 -> Vs =  8.6095493 m/s
//   A = 6  -> S = 0.24  m^2 -> Vs = 10.5445013 m/s
//   A = 10 -> S = 0.144 m^2 -> Vs = 13.6128927 m/s

function sweepPanel(): HTMLElement {
  return section('What changes if…')
}

/**
 * rows returns the sampled candidates as [driver, output, outcome] triples. The
 * sample rows are options rather than rows: the table body is a listbox so the
 * samples can be walked with the arrow keys.
 */
function rows(): string[][] {
  const list = within(sweepPanel()).getByRole('listbox', { name: 'Sampled candidates' })
  return within(list)
    .getAllByRole('option')
    .map((row) => Array.from(row.children).map((cell) => cell.textContent ?? ''))
}

async function sweep(
  user: ReturnType<typeof renderWorksheet>,
  from: string,
  to: string,
  samples: string,
): Promise<void> {
  const panel = sweepPanel()
  const fromField = within(panel).getByLabelText('From')
  await user.clear(fromField)
  await user.type(fromField, from)
  const toField = within(panel).getByLabelText('To')
  await user.clear(toField)
  await user.type(toField, to)
  const sampleField = within(panel).getByLabelText('Samples')
  await user.clear(sampleField)
  await user.type(sampleField, samples)
  await user.click(within(sweepPanel()).getByRole('button', { name: /Show the effect/ }))
  await waitFor(() => {
    expect(within(sweepPanel()).getByRole('listbox', { name: 'Sampled candidates' }))
      .toBeInTheDocument()
  })
}

describe('sensitivity', () => {
  test('a swept driver produces the same numbers a direct evaluation gives', async () => {
    const user = renderWorksheet()
    await buildBaseCandidate(user)
    await enter(user, 'Highest acceptable stall speed', '8')

    await setSelect(user, 'Move', 'wing.aspect_ratio.planform')
    await sweep(user, '4', '10', '4')

    const sampled = rows()
    expect(sampled).toHaveLength(4)
    expect(sampled[0]?.[0]).toBe('4')
    expect(sampled[0]?.[1]).toContain('8.60955')
    expect(sampled[3]?.[0]).toBe('10')
    expect(sampled[3]?.[1]).toContain('13.6129')

    // Higher aspect ratio at a fixed span is a smaller wing, so the stall speed
    // rises. The answer says what it held fixed rather than leaving that to be
    // inferred from the shape of the curve.
    expect(within(sweepPanel()).getByText(/wing\.span\.projected/)).toBeInTheDocument()
    expect(within(sweepPanel()).getByText(/span and aspect ratio/)).toBeInTheDocument()

    // The marker is the design itself, evaluated rather than interpolated: it
    // sits at the aspect ratio the design holds and reports that candidate's
    // stall speed.
    const marker = within(sweepPanel()).getByText('This design').nextElementSibling
    expect(marker?.textContent).toContain('6')
    expect(marker?.textContent).toContain('10.5445')
    // Nothing was applied: the design still holds the aspect ratio it held.
    expect(screen.getByLabelText('Aspect ratio')).toHaveValue('6')
  }, 90_000)

  test('every candidate is judged against the requirement, and the boundary is shown', async () => {
    const user = renderWorksheet()
    await buildBaseCandidate(user)
    await enter(user, 'Highest acceptable stall speed', '9')

    await setSelect(user, 'Move', 'wing.aspect_ratio.planform')
    await sweep(user, '4', '10', '4')

    const sampled = rows()
    // At A = 4 the stall speed is 8.61 m/s and the ceiling is met; by A = 10 it
    // is 13.6 m/s and it is not.
    expect(sampled[0]?.[2]).toContain('meets every required requirement')
    expect(sampled[3]?.[2]).toContain('misses a required requirement')
    // The boundary itself is drawn and named.
    expect(within(sweepPanel()).getByText(/Stall ceiling \(maximum\)/)).toBeInTheDocument()
  }, 90_000)

  test('a span sweep at a fixed area leaves the stall speed alone and says what did move', async () => {
    const user = renderWorksheet()
    await buildBaseCandidate(user)
    await enter(user, 'Highest acceptable stall speed', '8')

    // Drive span and area, so the aspect ratio and the chords follow.
    await user.click(within(rowFor('Wing area')).getByRole('button', { name: 'Use as input' }))
    await press(user, 'Give up wing.aspect_ratio.planform')
    await enter(user, 'Wing area', '0.24')

    await setSelect(user, 'Move', 'wing.span.projected')
    await sweep(user, '0.8', '2', '4')

    const sampled = rows()
    for (const row of sampled) {
      expect(row[1]).toContain('10.5445')
    }
    // Not "the span does nothing": this output does not see it, and the answer
    // names what did change.
    expect(within(sweepPanel()).getByText(/does not move across this range/)).toBeInTheDocument()
    expect(
      within(sweepPanel()).getByText(/Also changing across this range/),
    ).toHaveTextContent('wing.aspect_ratio.planform')
  }, 90_000)

  test('the samples can be walked with the keyboard, and show before and after', async () => {
    const user = renderWorksheet()
    await buildBaseCandidate(user)
    await setSelect(user, 'Move', 'wing.aspect_ratio.planform')
    await sweep(user, '4', '10', '4')

    const table = within(sweepPanel()).getByRole('listbox', { name: 'Sampled candidates' })
    table.focus()
    await user.keyboard('{ArrowDown}')
    const options = within(table).getAllByRole('option')
    expect(options[1]).toHaveAttribute('aria-selected', 'true')

    const comparison = within(sweepPanel()).getByText('Selected candidate').nextElementSibling
    expect(comparison?.textContent).toContain('6')
    await user.keyboard('{End}')
    expect(within(table).getAllByRole('option')[3]).toHaveAttribute('aria-selected', 'true')
  }, 90_000)

  test('a candidate the model cannot evaluate is a gap, not a point', async () => {
    const user = renderWorksheet()
    await buildBaseCandidate(user)
    // A body cannot be wider than the span, so the short end of this range is
    // not a design the geometry model covers.
    await enter(user, 'Body width at the wing', '0.8')

    await setSelect(user, 'Move', 'wing.span.projected')
    await sweep(user, '0.4', '2', '4')

    const sampled = rows()
    const gaps = sampled.filter((row) => row[1] === '—')
    expect(gaps.length).toBeGreaterThan(0)
    expect(sampled.filter((row) => row[1] !== '—').length).toBeGreaterThan(0)
    for (const gap of gaps) {
      expect(gap[2]).toContain('not computed')
    }
  }, 90_000)

  test('adopting a candidate is an explicit, undoable edit and nothing else', async () => {
    const user = renderWorksheet()
    await buildBaseCandidate(user)
    await setSelect(user, 'Move', 'wing.aspect_ratio.planform')
    await sweep(user, '4', '10', '4')

    // Selecting a candidate describes it. The design is untouched.
    const listbox = within(sweepPanel()).getByRole('listbox', { name: 'Sampled candidates' })
    listbox.focus()
    await user.keyboard('{End}')
    expect(screen.getByLabelText('Aspect ratio')).toHaveValue('6')

    // Adopting it is a separate act, and it is the ordinary driver edit.
    await press(user, /Use 10 as the aspect ratio/)
    expect(screen.getByLabelText('Aspect ratio')).toHaveValue('10')
    await press(user, 'Undo')
    expect(screen.getByLabelText('Aspect ratio')).toHaveValue('6')
  }, 90_000)

  test('nothing on the plot can only be read by seeing a colour', async () => {
    const user = renderWorksheet()
    await buildBaseCandidate(user)
    await enter(user, 'Highest acceptable stall speed', '9')
    await setSelect(user, 'Move', 'wing.aspect_ratio.planform')
    await sweep(user, '4', '10', '4')

    // Every outcome is written out beside the marker that carries it, and the
    // legend says what each shape means.
    expect(within(sweepPanel()).getAllByText(/meets every required requirement/).length)
      .toBeGreaterThan(0)
    for (const row of rows()) {
      expect(row[2]).toMatch(/meets|misses|unknown|not computed/)
    }

    // The plot itself carries the same answer for a reader who cannot see it at
    // all: what is being plotted against what, and what was held fixed.
    const plot = within(sweepPanel()).getByRole('img')
    const described = plot.getAttribute('aria-label') ?? ''
    expect(described).toContain('Aspect ratio')
    expect(described).toContain('stall-speed')
    expect(described).toContain('wing.span.projected')
  }, 90_000)

  test('an edit retires the curve rather than silently replacing it', async () => {
    const user = renderWorksheet()
    await buildBaseCandidate(user)
    await setSelect(user, 'Move', 'wing.aspect_ratio.planform')
    await sweep(user, '4', '10', '4')
    expect(within(sweepPanel()).queryByText(/describes an earlier revision/)).not.toBeInTheDocument()

    await enter(user, 'All-up mass', '3')
    expect(within(sweepPanel()).getByText(/describes an earlier revision/)).toBeInTheDocument()
    // It is still there to read: the curve is marked, not thrown away.
    expect(rows()).toHaveLength(4)
  }, 90_000)
})
