import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import type { UserEvent } from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'
import { memoryStorage } from './testing/storage.ts'
import {
  buildBaseCandidate,
  componentPanel,
  enter,
  enterIn,
  inSection,
  open,
  press,
  renderWorksheet,
  section,
  selectIn,
  setSelect,
  settled,
} from './testing/harness.tsx'

// Visual mass placement, against the real Go service.
//
// The task's own fixture, independent of this system: 1.5 kg of airframe at
// x = 0.4 m and a 0.5 kg battery at x = 0.2 m total 2 kg and balance at 0.35 m.
// Moving the battery to 0.6 m gives 0.45 m with the loading unchanged; growing
// it to 1 kg gives 2.5 kg at 0.48 m and a stall speed higher by sqrt(2.5/2).

const CANVAS = { left: 0, top: 0, width: 320, height: 320 }

let storage: Storage

beforeEach(() => {
  storage = memoryStorage()
  // jsdom lays nothing out, so an SVG reports a zero-sized box and a drag would
  // have no coordinate system to map into. The box is stubbed to the canvas the
  // component draws at, which is what a browser would report.
  vi.spyOn(Element.prototype, 'getBoundingClientRect').mockImplementation(function (this: Element) {
    if (this.tagName.toLowerCase() !== 'svg') return new DOMRect(0, 0, 0, 0)
    return new DOMRect(CANVAS.left, CANVAS.top, CANVAS.width, CANVAS.height)
  })
})

afterEach(() => {
  vi.restoreAllMocks()
})

/** addComponent lists a component and gives it a mass, a basis and a position. */
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
  await enterIn(user, panel, 'Where the mass comes from', 'fixture value')
  // Every coordinate is stated, zero included: an unstated y is a missing field
  // rather than a claim that the component sits on the centerline.
  await enterIn(user, panel, 'Position x', x)
  await enterIn(user, panel, 'Position y', '0')
  await enterIn(user, panel, 'Position z', '0')
}

/** placeAt types one coordinate of a named component. */
async function placeAt(user: UserEvent, name: string, x: string): Promise<void> {
  await enterIn(user, componentPanel(name), 'Position x', x)
}

/** balance reads the reported centre of gravity out of the components panel. */
function balance(): string {
  return inSection('Components and balance').getByText(/^x /).textContent ?? ''
}

/** stallSpeed reads the stall-speed check out of the results panel. */
function stallSpeed(): string {
  const results = screen.getByRole('complementary', { name: /Results and requirements/ })
  const row = within(results).getByRole('rowheader', { name: /Stall ceiling/ }).closest('tr')
  if (row === null) throw new Error('no stall-speed row')
  const cells = within(row).getAllByRole('cell')
  return cells[2]?.textContent ?? ''
}

describe('mass placement', () => {
  test('the independent placement fixture balances, and moving a mass leaves the loading alone', async () => {
    const user = renderWorksheet(storage)
    await buildBaseCandidate(user)
    await enter(user, 'Highest acceptable stall speed', '8')

    await addComponent(user, 'airframe', 'airframe', '1.5', '0.4')
    await addComponent(user, 'battery', 'battery', '0.5', '0.2')
    await setSelect(user, 'Where the all-up mass comes from', 'components')

    expect(balance()).toContain('0.35')
    const before = stallSpeed()

    // Typing the coordinate and dragging to it are the same edit. This is the
    // typed one; the drag below produces the same command.
    await placeAt(user, 'battery', '0.6')
    expect(balance()).toContain('0.45')
    expect(stallSpeed()).toBe(before)
  }, 90_000)

  test('growing a component changes the loading as well as the balance', async () => {
    const user = renderWorksheet(storage)
    await buildBaseCandidate(user)
    await enter(user, 'Highest acceptable stall speed', '8')
    await addComponent(user, 'airframe', 'airframe', '1.5', '0.4')
    await addComponent(user, 'battery', 'battery', '1', '0.6')
    await setSelect(user, 'Where the all-up mass comes from', 'components')

    expect(balance()).toContain('0.48')
    // At 2.5 kg the stall speed is 10.5445013 * sqrt(1.25) = 11.7891109 m/s.
    expect(stallSpeed()).toContain('11.7891')
  }, 90_000)

  test('an unplaced component is reported as unplaced, never assumed to sit at the origin', async () => {
    const user = renderWorksheet(storage)
    await buildBaseCandidate(user)
    await addComponent(user, 'airframe', 'airframe', '1.5', '0.4')
    await addComponent(user, 'battery', 'battery', '0.5', '0.2')

    await user.type(screen.getByLabelText('Add a component'), 'payload')
    await press(user, 'Add component')
    const payload = componentPanel('payload')
    await user.click(within(payload).getByText('payload', { selector: '.component-name' }))
    await selectIn(user, payload, 'What it is', 'payload')
    await enterIn(user, payload, 'Mass', '0.5')
    await enterIn(user, payload, 'Where the mass comes from', 'estimate')

    // The two complete components still balance, at exactly the station they
    // give on their own: the third contributed nothing at all.
    expect(balance()).toContain('0.35')
    const panel = section('Components and balance')
    expect(within(panel).getByText(/not the balance of the whole aircraft/)).toBeInTheDocument()
  }, 90_000)

  test('a drag previews, commits as one undoable change, and can be cancelled', async () => {
    const user = renderWorksheet(storage)
    await buildBaseCandidate(user)
    await addComponent(user, 'airframe', 'airframe', '1.5', '0.4')
    await addComponent(user, 'battery', 'battery', '0.5', '0.2')
    await setSelect(user, 'Where the all-up mass comes from', 'components')
    expect(balance()).toContain('0.35')

    // Cancelling leaves nothing behind: the design still holds the old station.
    await dragBattery(160, 200)
    await waitFor(() => {
      expect(screen.getByText(/Placing battery at/)).toBeInTheDocument()
    })
    await user.click(screen.getByRole('button', { name: 'Cancel placement' }))
    expect(screen.queryByText(/Placing battery at/)).not.toBeInTheDocument()
    expect(balance()).toContain('0.35')

    // A completed drag commits one placement, and one undo takes it back.
    await dragBattery(160, 200)
    fireEvent.pointerUp(planCanvas())
    await settled()
    const moved = balance()
    expect(moved).not.toContain('0.35')

    await press(user, 'Undo')
    expect(balance()).toContain('0.35')
    await press(user, 'Redo')
    expect(balance()).toBe(moved)
  }, 90_000)

  test('a placement survives saving and reopening the draft', async () => {
    const user = renderWorksheet(storage)
    await buildBaseCandidate(user)
    await addComponent(user, 'airframe', 'airframe', '1.5', '0.4')
    await addComponent(user, 'battery', 'battery', '0.5', '0.6')
    await setSelect(user, 'Where the all-up mass comes from', 'components')
    expect(balance()).toContain('0.45')

    await open(user, 'Drafts')
    await user.type(screen.getByLabelText('Name'), 'placement')
    await press(user, 'Save')
    // Move it somewhere else, then reopen: the saved layout comes back and is
    // recalculated rather than restored from a cached number.
    await placeAt(user, 'battery', '0.2')
    expect(balance()).toContain('0.35')

    await press(user, 'Open')
    await settled()
    expect(balance()).toContain('0.45')
  }, 90_000)

  test('the coordinated views are drawn to one scale', async () => {
    const user = renderWorksheet(storage)
    await buildBaseCandidate(user)
    await addComponent(user, 'airframe', 'airframe', '1.5', '0.4')

    // A plan and a side view of the same aircraft at different scales are two
    // drawings of two aircraft. The same distance has to be the same number of
    // pixels in both, so the two canvases share one scale and differ only in
    // how much of each axis they need to fit.
    const canvases = section('Where the masses sit').querySelectorAll('.placement-canvas')
    expect(canvases).toHaveLength(2)
    const boxes = Array.from(canvases).map((canvas) => {
      const view = canvas.getAttribute('viewBox')?.split(' ') ?? []
      return { width: Number(view[2]), height: Number(view[3]) }
    })
    // The plan view spans the whole 1.2 m span across; the side view spans the
    // 0.2 m root chord. At one scale the first is the wider of the two.
    expect(boxes[0]?.width).toBeGreaterThan(boxes[1]?.width ?? 0)
  }, 90_000)

  test('the aerodynamic references are named as unknown rather than drawn', async () => {
    const user = renderWorksheet(storage)
    await buildBaseCandidate(user)
    const panel = section('Where the masses sit')
    expect(
      within(panel).getByText(/not a wing aerodynamic centre/),
    ).toBeInTheDocument()
    expect(within(panel).getByText(/^Unknown\. No model here produces any of them/)).toBeInTheDocument()
    // The lumped load is a magnitude with no line of action, and says so.
    expect(within(panel).getByText(/solves no line of action/)).toBeInTheDocument()
  }, 90_000)
})

/**
 * dragBattery presses the battery's marker and moves the pointer. Each element
 * is looked up again: React replaces the nodes on every answer, and a reference
 * held across one would be dragging a marker that is no longer on the page.
 */
async function dragBattery(clientX: number, clientY: number): Promise<void> {
  const plan = planCanvas()
  fireEvent.pointerDown(within(plan).getByRole('button', { name: 'Drag battery' }))
  fireEvent.pointerMove(planCanvas(), { clientX, clientY })
  await Promise.resolve()
}

/** planCanvas is the plan view inside the placement panel. */
function planCanvas(): HTMLElement {
  const panel = section('Where the masses sit')
  const canvas = panel.querySelector('.placement-canvas')
  if (!(canvas instanceof Element)) throw new Error('no placement canvas')
  return canvas as unknown as HTMLElement
}
