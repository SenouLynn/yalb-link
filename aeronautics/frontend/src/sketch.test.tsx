import { screen, waitFor, within } from '@testing-library/react'
import { describe, expect, test } from 'vitest'
import {
  buildBaseCandidate,
  enter,
  inSection,
  open,
  press,
  renderWorksheet,
  rowFor,
  section,
  setSelect,
} from './testing/harness.tsx'

// The dimension and formula views, against the real Go service.
//
// A dimension on a drawing and a field in the worksheet are one thing seen
// twice. These tests hold that in both directions, and hold that the formula,
// the substitutions and the copied precision all come from the service.
//
// Expected values, computed independently: at b = 1.2 m and A = 6 on a
// rectangle, S = 1.44/6 = 0.24 m^2 and every chord is S/b = 0.2 m.

/** explanationBox is the panel that shows the selected value's relationship.
 * The same expression also appears in the parameter table, so a query that did
 * not say which it meant would match both. */
function explanationBox(): HTMLElement {
  const box = section('Dimensions and formulas').querySelector('.explanation')
  if (!(box instanceof HTMLElement)) throw new Error('nothing is selected')
  return box
}

describe('dimensions and formulas', () => {
  test('selecting a dimension selects its field, and selecting a field selects its dimension', async () => {
    const user = renderWorksheet()
    await buildBaseCandidate(user)

    const drawings = inSection('Dimensions and formulas')
    const spanDimension = drawings.getAllByRole('button', { name: /^span b,/ })[0]
    expect(spanDimension).toBeDefined()
    if (spanDimension === undefined) throw new Error('no span dimension is drawn')

    expect(spanDimension).toHaveAttribute('aria-pressed', 'false')
    await user.click(spanDimension)
    expect(spanDimension).toHaveAttribute('aria-pressed', 'true')

    // The field that drives it says so too, without either component knowing
    // about the other: both read the same parameter key.
    const fieldMark = screen.getByRole('button', {
      name: 'Show the drawing and formula for wing.span.projected',
    })
    expect(fieldMark).toHaveAttribute('aria-pressed', 'true')

    // And the other way round: pressing the field's mark selects the dimension.
    await user.click(fieldMark)
    expect(spanDimension).toHaveAttribute('aria-pressed', 'false')
    const areaMark = screen.getByRole('button', {
      name: 'Show the drawing and formula for wing.area.reference',
    })
    await user.click(areaMark)
    expect(areaMark).toHaveAttribute('aria-pressed', 'true')
  })

  test('a selected value shows the relationship and the values actually substituted', async () => {
    const user = renderWorksheet()
    await buildBaseCandidate(user)

    // The root chord is derived here: the design drives span and aspect ratio.
    const drawings = inSection('Dimensions and formulas')
    const rootChord = drawings.getAllByRole('button', { name: /^root chord c_root,/ })[0]
    if (rootChord === undefined) throw new Error('no root-chord dimension is drawn')
    await user.click(rootChord)

    const explanation = explanationBox()
    expect(within(explanation).getByText('c_root = 2 * S / (b * (1 + lambda))')).toBeInTheDocument()
    // The substitutions are the values the equation consumed, not a restatement
    // of the inputs the worksheet happens to hold.
    const substitutions = within(explanation).getByRole('table', { name: /Values substituted/ })
    expect(within(substitutions).getByRole('rowheader', { name: 'wing_area' })).toBeInTheDocument()
    expect(within(substitutions).getByRole('rowheader', { name: 'span' })).toBeInTheDocument()
    expect(within(substitutions).getByText('0.24 m^2')).toBeInTheDocument()
    expect(within(explanation).getByText(/Result:/)).toHaveTextContent('0.2 m')
  })

  test('the formulas are readable without the diagram, and the dependencies are followable', async () => {
    const user = renderWorksheet()
    await buildBaseCandidate(user)

    await open(user, 'Parameter table')
    const drawings = section('Dimensions and formulas')
    const table = within(drawings).getByRole('table', { name: /Named parameters/ })
    // Every relationship is in the table, so nothing on this page depends on
    // being able to read a drawing.
    expect(within(table).getByText('c_root = 2 * S / (b * (1 + lambda))')).toBeInTheDocument()

    await user.click(within(table).getByRole('button', { name: 'wing.chord.tip' }))
    expect(within(explanationBox()).getByText('c_tip = lambda * c_root')).toBeInTheDocument()
    // Following a dependency selects it, which is how a builder walks back from
    // a value to what it was computed from.
    await user.click(within(explanationBox()).getByRole('button', { name: 'wing.chord.root' }))
    expect(
      within(explanationBox()).getByText('c_root = 2 * S / (b * (1 + lambda))'),
    ).toBeInTheDocument()
  })

  test('a driver swap updates the dimensions, the roles and the dependency links', async () => {
    const user = renderWorksheet()
    await buildBaseCandidate(user)

    const before = inSection('Dimensions and formulas')
      .getAllByRole('button', { name: /^span b,/ })[0]
    expect(before?.getAttribute('aria-label')).toContain('1.2 m')

    // Give up the span and drive area and aspect ratio instead. The span is
    // then derived, and its dimension follows the parameter rather than keeping
    // the released driver's value.
    await user.click(within(rowFor('Wing area')).getByRole('button', { name: 'Use as input' }))
    await press(user, 'Give up wing.span.projected')
    await enter(user, 'Wing area', '0.6')

    const drawings = inSection('Dimensions and formulas')
    const span = drawings.getAllByRole('button', { name: /^span b,/ })[0]
    if (span === undefined) throw new Error('the span dimension vanished')
    // b = sqrt(A S) = sqrt(6 * 0.6) = 1.897366596 m.
    expect(span.getAttribute('aria-label')).toContain('1.89737')

    await user.click(span)
    const explanation = explanationBox()
    expect(within(explanation).getByText('b = sqrt(A * S)')).toBeInTheDocument()
    expect(within(explanation).getByText(/Depends on:/)).toBeInTheDocument()
  })

  test('copying the parameters preserves physical precision', async () => {
    const user = renderWorksheet()
    await buildBaseCandidate(user)
    await open(user, 'Parameter table')

    const copy = screen.getByLabelText('Copy as tab-separated values')
    if (!(copy instanceof HTMLTextAreaElement)) throw new Error('the copy box is not a textarea')
    const rows = copy.value.split('\n')
    const header = rows[0] ?? ''
    expect(header.split('\t')).toContain('formula')

    const mac = rows.find((row) => row.startsWith('wing.chord.mac'))
    if (mac === undefined) throw new Error('the MAC is not in the copied table')
    const value = mac.split('\t')[1] ?? ''
    // The rectangle's MAC is exactly the chord, 0.2 m, but the point of this
    // check is that the copied text round-trips to the same float64 the service
    // sent rather than to a rounded display value.
    expect(Number(value)).toBe(0.2)
    const aspect = rows.find((row) => row.startsWith('wing.aspect_ratio.projected'))
    if (aspect === undefined) throw new Error('the projected aspect ratio is not in the table')
    expect(String(Number(aspect.split('\t')[1]))).toBe(aspect.split('\t')[1])
  })

  test('panel dimensions are distinguished from projected ones in words', async () => {
    const user = renderWorksheet()
    await buildBaseCandidate(user)
    // The service answers in SI, so an angle field shows radians once the design
    // has been through the boundary once. The unit is chosen explicitly here
    // rather than assumed, which is what a builder does too.
    await user.selectOptions(screen.getByLabelText('Unit for Dihedral'), 'deg')
    await enter(user, 'Dihedral', '8')
    await waitFor(() => {
      expect(screen.getByLabelText(/What stays fixed as the dihedral changes/)).toBeInTheDocument()
    })
    // A dihedral without a held plane is a wing that cannot solve for a reason
    // nobody chose, so the choice is made before the drawing is read.
    await setSelect(user, 'What stays fixed as the dihedral changes', 'hold-projected')

    const drawings = inSection('Dimensions and formulas')
    const panel = drawings.getAllByRole('button', { name: /^panel half span,/ })[0]
    const projected = drawings.getAllByRole('button', { name: /^projected half span,/ })[0]
    if (panel === undefined || projected === undefined) {
      throw new Error('the front view does not dimension both planes')
    }
    // Which plane a dimension is in is written out, not left to a line style.
    expect(panel.getAttribute('aria-label')).toContain('panel as built')
    expect(projected.getAttribute('aria-label')).toContain('plan-view projection')
  })
})
