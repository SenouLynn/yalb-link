import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, test, vi } from 'vitest'
import type {
  ApplyRequest,
  Component,
  Design,
  EvaluateRequest,
  Evaluation,
  MassProperties,
  PreviewRequest,
  Request as RequestIdentity,
  SweepRequest,
  SweepResponse,
  UnitInfo,
} from './api/contract.ts'
import type { ApplyResult, Discovered, Transport } from './api/client.ts'
import { startingDesign } from './state/design.ts'
import { memoryStorage } from './testing/storage.ts'
import { Worksheet } from './Worksheet.tsx'

// Which answer is accepted, for the two request kinds Task 07 adds.
//
// The transport is controllable rather than real, because timing is the whole
// point: a sweep that answers a question the builder has moved on from must not
// replace the plot, and a preview for a position they have already left must
// not describe the one they are on. The journey tests next door run against the
// real Go service, so nothing here stands in for the physics.

interface Held<T> {
  settle(value: T): void
}

interface Fake {
  transport: Transport
  sweeps: (Held<SweepResponse> & { request: SweepRequest })[]
  previews: (Held<Evaluation | null> & { request: PreviewRequest })[]
}

const UNITS: UnitInfo[] = [
  { symbol: 'kg', dimension: 'mass', factorToSi: 1, si: true },
  { symbol: 'm', dimension: 'length', factorToSi: 1, si: true },
  { symbol: 'm^2', dimension: 'area', factorToSi: 1, si: true },
  { symbol: 'm/s', dimension: 'speed', factorToSi: 1, si: true },
  { symbol: 'rad', dimension: 'angle', factorToSi: 1, si: true },
  { symbol: 'deg', dimension: 'angle', factorToSi: Math.PI / 180, si: false },
  { symbol: '1', dimension: 'ratio', factorToSi: 1, si: true },
]

const DISCOVERED: Discovered = {
  contractVersion: 'v1',
  version: '0.0.0',
  equationRevisions: { 'lift.stall-speed': '1' },
}

/** balanceAt is a mass-properties answer with the centre of gravity at x. */
function balanceAt(x: number): MassProperties {
  return {
    datum: 'origin at the root leading edge',
    status: 'computed',
    detail: '',
    contributions: [],
    total: { value: 2, unit: 'kg' },
    cg: { x: { value: x, unit: 'm' }, y: { value: 0, unit: 'm' }, z: { value: 0, unit: 'm' } },
    complete: true,
  }
}

/** solvedWing is the least a drawing needs: one outline and one dimension. */
function solvedWing(): NonNullable<Evaluation['wing']> {
  const origin = { x: { value: 0, unit: 'm' }, y: { value: 0, unit: 'm' }, z: { value: 0, unit: 'm' } }
  const tip = { x: { value: 0, unit: 'm' }, y: { value: 0.6, unit: 'm' }, z: { value: 0, unit: 'm' } }
  return {
    datum: 'origin at the root leading edge',
    solveMode: 'span-and-aspect-ratio',
    drivers: ['wing.span.projected', 'wing.aspect_ratio.planform'],
    parameters: [{
      key: 'wing.span.projected',
      role: 'driver',
      datum: '',
      equationId: '',
      revision: '',
      dependsOn: [],
      value: { value: 1.2, unit: 'm' },
    }],
    outline: [],
    explanations: [{
      key: 'wing.span.projected',
      role: 'driver',
      equationId: '',
      revision: '',
      expression: '',
      detail: 'entered by the builder',
      substitutions: [],
      dependsOn: [],
      value: { value: 1.2, unit: 'm' },
    }],
    views: [{
      view: 'plan-view',
      datum: 'origin at the root leading edge',
      across: 'y',
      up: 'x',
      curves: [{ label: 'right panel', role: 'outline', points: [origin, tip], mirrored: true, closed: false }],
      dimensions: [{
        key: 'wing.span.projected',
        label: 'span b',
        detail: 'tip to tip',
        kind: 'linear',
        plane: 'plan-view',
        from: origin,
        to: tip,
        value: { value: 1.2, unit: 'm' },
      }],
    }],
  }
}

function evaluationFor(identity: RequestIdentity, cg: number): Evaluation {
  return {
    request: identity,
    snapshot: 'inputs',
    geometry: 'computed',
    aggregate: 'unknown',
    hasRequired: false,
    wing: solvedWing(),
    checks: [],
    areaLower: { subject: 'wing-area', direction: 'minimum', known: false, partial: false },
    areaUpper: { subject: 'wing-area', direction: 'maximum', known: false, partial: false },
    mass: {
      lower: { subject: 'mass', direction: 'minimum', known: false, partial: false },
      upper: { subject: 'mass', direction: 'maximum', known: false, partial: false },
      complete: false,
      empty: false,
    },
    massProperties: balanceAt(cg),
    loads: [],
    conflicts: [],
    patterns: [],
    definitionIssues: [],
    geometryIssues: [],
    configurationIssues: [],
  }
}

/** placedDesign carries the two-component fixture the drag moves. */
function placedDesign(batteryAt: number): Design {
  const base = startingDesign()
  const component = (name: string, role: string, mass: number, x: number): Component => ({
    name,
    role,
    basis: 'fixture',
    mass: { value: mass, unit: 'kg' },
    position: {
      x: { value: x, unit: 'm' },
      y: { value: 0, unit: 'm' },
      z: { value: 0, unit: 'm' },
    },
  })
  return {
    ...base,
    massMode: 'components',
    components: [
      component('airframe', 'airframe', 1.5, 0.4),
      component('battery', 'battery', 0.5, batteryAt),
    ],
    wing: { ...base.wing, span: { value: 1.2, unit: 'm' }, aspectRatio: 6 },
  }
}

function sweepAnswer(identity: RequestIdentity, request: SweepRequest, first: number): SweepResponse {
  return {
    request: identity,
    settings: request.settings,
    settingsFingerprint: 'service-side',
    snapshot: 'inputs',
    solveMode: 'span-and-aspect-ratio',
    detail: 'the planform solves from span and aspect ratio',
    heldFixed: ['wing.span.projected'],
    alsoChanged: [],
    bounds: [],
    samples: [{
      driver: { value: first, unit: '1' },
      value: { value: first, unit: 'm/s' },
      status: 'computed',
      feasibility: 'unknown',
      trace: null,
      hasRequired: false,
    }],
    current: {
      driver: { value: 6, unit: '1' },
      value: { value: 10.5, unit: 'm/s' },
      status: 'computed',
      feasibility: 'unknown',
      trace: null,
      hasRequired: false,
    },
    invariant: false,
  }
}

/**
 * fakeService answers evaluations and applies immediately, because those are
 * not what these tests are about, and hands sweeps and previews to the test to
 * settle by hand.
 */
function fakeService(design: Design): Fake {
  const sweeps: Fake['sweeps'] = []
  const previews: Fake['previews'] = []
  let current = design
  const transport: Transport = {
    discover: () => Promise.resolve(DISCOVERED),
    units: () => Promise.resolve(UNITS),
    evaluate: (request: EvaluateRequest) =>
      Promise.resolve(evaluationFor(request.request, 0.35)),
    apply: (request: ApplyRequest): Promise<ApplyResult> => {
      // The apply answers with the design the command asked for, so a committed
      // placement is visible on the page.
      const command = request.commands[0]
      if (command?.kind === 'place-component' && command.position?.x) {
        current = placedDesign(command.position.x.value)
      }
      return Promise.resolve({
        design: current,
        evaluation: evaluationFor(request.request, 0.99),
      })
    },
    preview: (request: PreviewRequest) =>
      new Promise<Evaluation>((resolve) => {
        previews.push({
          request,
          settle: (value) => { if (value !== null) resolve(value) },
        })
      }),
    sweep: (request: SweepRequest) =>
      new Promise<SweepResponse>((resolve) => {
        sweeps.push({ request, settle: resolve })
      }),
  }
  return { transport, sweeps, previews }
}

let storage: Storage

beforeEach(() => {
  storage = memoryStorage()
  vi.spyOn(Element.prototype, 'getBoundingClientRect').mockImplementation(function (this: Element) {
    if (this.tagName.toLowerCase() !== 'svg') return new DOMRect(0, 0, 0, 0)
    return new DOMRect(0, 0, 320, 320)
  })
})

/**
 * setUp renders the worksheet and gets the two-component fixture into it. The
 * fake boundary answers every apply with that design, so one ordinary edit is
 * enough: what a command means is the service's answer, and here the service is
 * the test.
 */
async function setUp(): Promise<{ fake: Fake; user: ReturnType<typeof userEvent.setup> }> {
  const fake = fakeService(placedDesign(0.2))
  render(<Worksheet transport={fake.transport} session="task07-ordering" storage={storage} />)
  const user = userEvent.setup()
  await waitFor(() => { expect(screen.getByText('Current')).toBeInTheDocument() })

  await user.type(screen.getByLabelText('Add a component'), 'seed')
  await user.click(screen.getByRole('button', { name: 'Add' }))
  await waitFor(() => {
    expect(screen.getByText('battery', { selector: '.component-name' })).toBeInTheDocument()
  })
  return { fake, user }
}

function sweepPanel(): HTMLElement {
  const panel = screen.getByText('What changes if…').closest('details')
  if (panel === null) throw new Error('no sensitivity panel')
  return panel
}

async function askForASweep(user: ReturnType<typeof userEvent.setup>): Promise<void> {
  await user.click(within(sweepPanel()).getByRole('button', { name: /Show the effect/ }))
}

describe('a sweep answer that arrives after the builder has moved on', () => {
  test('an edit, an undo and a different edit leave the late answer unwanted', async () => {
    const { fake, user } = await setUp()
    await askForASweep(user)
    await waitFor(() => { expect(fake.sweeps).toHaveLength(1) })
    const asked = fake.sweeps[0]
    if (asked === undefined) throw new Error('no sweep was requested')

    // Edit, undo back to the design the sweep was asked about, then edit again.
    await user.clear(screen.getByLabelText('All-up mass'))
    await user.type(screen.getByLabelText('All-up mass'), '3{Enter}')
    await waitFor(() => { expect(screen.getByText('Current')).toBeInTheDocument() })
    await user.click(screen.getByRole('button', { name: 'Undo' }))
    await waitFor(() => { expect(screen.getByText('Current')).toBeInTheDocument() })

    // The answer finally arrives. The design is back where it started, and the
    // curve is still not shown: the identity was retired by the edit and
    // nothing restores it.
    asked.settle(sweepAnswer(asked.request.request, asked.request, 4))
    await Promise.resolve()
    expect(within(sweepPanel()).queryByRole('listbox', { name: 'Sampled candidates' }))
      .not.toBeInTheDocument()
  })

  test('an answer to a superseded question does not replace the plot', async () => {
    const { fake, user } = await setUp()
    await askForASweep(user)
    await waitFor(() => { expect(fake.sweeps).toHaveLength(1) })

    // Ask again. The first answer is now for a question nobody is asking.
    await askForASweep(user)
    await waitFor(() => { expect(fake.sweeps).toHaveLength(2) })
    const [first, second] = fake.sweeps
    if (first === undefined || second === undefined) throw new Error('two sweeps were expected')

    second.settle(sweepAnswer(second.request.request, second.request, 8))
    await waitFor(() => {
      expect(within(sweepPanel()).getByRole('listbox', { name: 'Sampled candidates' }))
        .toBeInTheDocument()
    })
    first.settle(sweepAnswer(first.request.request, first.request, 4))
    await Promise.resolve()

    // The plot still shows the second answer's sample, not the first's.
    const listbox = within(sweepPanel()).getByRole('listbox', { name: 'Sampled candidates' })
    const options = within(listbox).getAllByRole('option')
    expect(options[0]?.textContent).toContain('8')
    expect(options[0]?.textContent).not.toContain('4')
  })

  test('changing the range says the curve answers a different question', async () => {
    const { fake, user } = await setUp()
    await askForASweep(user)
    await waitFor(() => { expect(fake.sweeps).toHaveLength(1) })
    const asked = fake.sweeps[0]
    if (asked === undefined) throw new Error('no sweep was requested')
    asked.settle(sweepAnswer(asked.request.request, asked.request, 4))
    await waitFor(() => {
      expect(within(sweepPanel()).getByRole('listbox', { name: 'Sampled candidates' }))
        .toBeInTheDocument()
    })
    expect(within(sweepPanel()).queryByText(/controls have moved on/)).not.toBeInTheDocument()

    const to = within(sweepPanel()).getByLabelText('To')
    await user.clear(to)
    await user.type(to, '12')
    // The curve is still readable and is no longer presented as the answer to
    // what the controls now ask.
    expect(within(sweepPanel()).getByText(/controls have moved on/)).toBeInTheDocument()
  })
})

describe('a preview that arrives after the placement has moved on', () => {
  function planCanvas(): HTMLElement {
    const panel = screen.getByText('Where the masses sit').closest('details')
    const canvas = panel?.querySelector('.placement-canvas')
    if (!(canvas instanceof Element)) throw new Error('no placement canvas')
    return canvas as unknown as HTMLElement
  }

  test('a delayed preview cannot describe a position the builder has left', async () => {
    const { fake } = await setUp()

    fireEvent.pointerDown(within(planCanvas()).getByRole('button', { name: 'Drag battery' }))
    fireEvent.pointerMove(planCanvas(), { clientX: 120, clientY: 120 })
    await waitFor(() => { expect(fake.previews).toHaveLength(1) })
    const stale = fake.previews[0]
    if (stale === undefined) throw new Error('no preview was requested')

    // Move on before the answer arrives.
    fireEvent.pointerMove(planCanvas(), { clientX: 200, clientY: 220 })
    const readout = screen.getByText(/Placing battery at/).textContent ?? ''

    // The late answer describes the position that has been left, so it is
    // dropped: the readout still waits for the one being asked about.
    stale.settle(evaluationFor({ session: 'task07-ordering', sequence: 99 }, 0.777))
    await Promise.resolve()
    expect(screen.getByText(/Placing battery at/).textContent).toBe(readout)
    expect(screen.queryByText(/0\.777/)).not.toBeInTheDocument()
  })

  test('a preview that arrives after the drag was committed changes nothing', async () => {
    const { fake } = await setUp()

    fireEvent.pointerDown(within(planCanvas()).getByRole('button', { name: 'Drag battery' }))
    fireEvent.pointerMove(planCanvas(), { clientX: 120, clientY: 120 })
    await waitFor(() => { expect(fake.previews).toHaveLength(1) })
    const held = fake.previews[0]
    if (held === undefined) throw new Error('no preview was requested')

    fireEvent.pointerUp(planCanvas())
    await waitFor(() => {
      expect(screen.queryByText(/Placing battery at/)).not.toBeInTheDocument()
    })

    held.settle(evaluationFor({ session: 'task07-ordering', sequence: 99 }, 0.777))
    await Promise.resolve()
    // The committed placement stands and no preview readout reappears.
    expect(screen.queryByText(/Placing battery at/)).not.toBeInTheDocument()
    expect(screen.queryByText(/0\.777/)).not.toBeInTheDocument()
  })
})
