import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, test } from 'vitest'
import type { ApplyRequest, Design, EvaluateRequest, Evaluation, Request as RequestIdentity, UnitInfo } from './api/contract.ts'
import type { ApplyResult, Discovered, Transport } from './api/client.ts'
import { TransportError } from './api/client.ts'
import { startingDesign } from './state/design.ts'
import { noPowerSearch, unpowered } from './testing/fixtures.ts'
import { memoryStorage } from './testing/storage.ts'
import { Worksheet } from './Worksheet.tsx'

// These tests are about which answer is accepted, not about what is in it, so
// the transport is controllable rather than real. Timing is the whole point: a
// response that arrives after the builder has moved on must not land.
//
// The journey tests next door run against the real Go service, so nothing here
// stands in for the physics.

interface Deferred {
  readonly kind: 'evaluate' | 'apply'
  readonly identity: RequestIdentity
  readonly design: Design
  settle(evaluation: Evaluation, design?: Design): void
  fail(error: Error): void
}

/** deferredTransport hands out promises the test settles by hand. */
function deferredTransport(): { transport: Transport; queue: Deferred[] } {
  const queue: Deferred[] = []

  function enqueue(
    kind: 'evaluate' | 'apply',
    identity: RequestIdentity,
    design: Design,
  ): { promise: Promise<{ evaluation: Evaluation; design: Design }> } {
    let settle: (value: { evaluation: Evaluation; design: Design }) => void = () => undefined
    let reject: (error: Error) => void = () => undefined
    const promise = new Promise<{ evaluation: Evaluation; design: Design }>((res, rej) => {
      settle = res
      reject = rej
    })
    queue.push({
      kind,
      identity,
      design,
      settle: (evaluation, edited) => { settle({ evaluation, design: edited ?? design }) },
      fail: (error) => { reject(error) },
    })
    return { promise }
  }

  const units: UnitInfo[] = [
    { symbol: 'kg', dimension: 'mass', factorToSi: 1, si: true },
    { symbol: 'm', dimension: 'length', factorToSi: 1, si: true },
    { symbol: 'm^2', dimension: 'area', factorToSi: 1, si: true },
    { symbol: 'm/s', dimension: 'speed', factorToSi: 1, si: true },
    { symbol: 'deg', dimension: 'angle', factorToSi: Math.PI / 180, si: false },
    { symbol: 'rad', dimension: 'angle', factorToSi: 1, si: true },
    { symbol: '1', dimension: 'ratio', factorToSi: 1, si: true },
  ]
  const discovered: Discovered = {
    contractVersion: 'v1',
    version: '0.0.0',
    equationRevisions: { 'lift.stall-speed': '1' },
  }

  const transport: Transport = {
    discover: () => Promise.resolve(discovered),
    units: () => Promise.resolve(units),
    evaluate: async (request: EvaluateRequest) => {
      const { promise } = enqueue('evaluate', request.request, request.design)
      return (await promise).evaluation
    },
    apply: async (request: ApplyRequest): Promise<ApplyResult> => {
      const { promise } = enqueue('apply', request.request, request.design)
      return await promise
    },
    preview: () => Promise.reject(new TransportError('previews are not used here')),
    sweep: () => Promise.reject(new TransportError('sweeps are not used here')),
    powerSearch: noPowerSearch,
  }
  return { transport, queue }
}

function answerFor(identity: RequestIdentity): Evaluation {
  return {
    request: identity,
    snapshot: `snapshot-${String(identity.sequence)}`,
    geometry: 'missing',
    aggregate: 'unknown',
    hasRequired: false,
    wing: null,
    checks: [],
    areaLower: { subject: 'wing-area', direction: 'minimum', known: false, partial: false },
    areaUpper: { subject: 'wing-area', direction: 'maximum', known: false, partial: false },
    mass: {
      lower: { subject: 'mass', direction: 'minimum', known: false, partial: false },
      upper: { subject: 'mass', direction: 'maximum', known: false, partial: false },
      complete: false,
      empty: false,
    },
    massProperties: {
      datum: 'wing root',
      status: 'missing',
      contributions: [],
      total: null,
      cg: null,
      complete: false,
    },
    loads: [],
    conflicts: [],
    patterns: [],
    definitionIssues: [],
    geometryIssues: [],
    configurationIssues: [],
    ...unpowered(),
  }
}

/**
 * answeredWith is a design the fake boundary hands back. The mass is what these
 * tests read off the page, so every branch answers with a different one and a
 * discarded answer carries a mass no branch ever asked for. A helper that gave
 * them all the same mass would pass whether or not the answer landed.
 */
function answeredWith(mass: number): Design {
  return { ...startingDesign(), mass: { value: mass, unit: 'kg' } }
}

/** withSpan is a design whose span the boundary has accepted. */
function withSpan(metres: number): Design {
  const base = startingDesign()
  return { ...base, wing: { ...base.wing, span: { value: metres, unit: 'm' } } }
}

let storage: Storage

beforeEach(() => {
  storage = memoryStorage()
})

/** setUp renders the worksheet and settles its first evaluation. */
async function setUp(): Promise<{ queue: Deferred[]; user: ReturnType<typeof userEvent.setup> }> {
  const { transport, queue } = deferredTransport()
  render(<Worksheet transport={transport} session="ordering" storage={storage} />)
  const user = userEvent.setup()
  await waitFor(() => { expect(queue).toHaveLength(1) })
  const first = queue[0]
  if (!first) throw new Error('no first request')
  first.settle(answerFor(first.identity))
  await waitFor(() => { expect(screen.getByText('Current')).toBeInTheDocument() })
  return { queue, user }
}

/** shownMass is the mass the worksheet is currently showing, read off the page. */
function shownMass(): string {
  const mass = screen.getByLabelText('All-up mass')
  if (!(mass instanceof HTMLInputElement)) throw new Error('the mass field is not an input')
  return mass.value
}

/** takeLatest returns the most recent outstanding request. */
function latest(queue: Deferred[]): Deferred {
  const request = queue.at(-1)
  if (!request) throw new Error('no outstanding request')
  return request
}

describe('a response that arrives after the builder has moved on', () => {
  test('an edit, an undo and a different edit leave the late answer unwanted', async () => {
    const { queue, user } = await setUp()

    // Edit one: set the mass to 2 kg, and hold the response back.
    await user.clear(screen.getByLabelText('All-up mass'))
    await user.type(screen.getByLabelText('All-up mass'), '2{Enter}')
    await waitFor(() => { expect(queue).toHaveLength(2) })
    const branchA = latest(queue)
    branchA.settle(answerFor(branchA.identity), answeredWith(2))
    await waitFor(() => { expect(shownMass()).toBe('2') })

    // Edit two, held back this time.
    await user.clear(screen.getByLabelText('All-up mass'))
    await user.type(screen.getByLabelText('All-up mass'), '3{Enter}')
    await waitFor(() => { expect(queue).toHaveLength(3) })
    const outstanding = latest(queue)

    // Undo while it is still in flight. That retires its identity.
    await user.click(screen.getByRole('button', { name: 'Undo' }))
    // The undo asks for a fresh evaluation of the restored design.
    await waitFor(() => { expect(queue).toHaveLength(4) })
    const afterUndo = latest(queue)
    afterUndo.settle(answerFor(afterUndo.identity))
    await waitFor(() => { expect(screen.getByText('Current')).toBeInTheDocument() })

    // A different edit, settled.
    await user.clear(screen.getByLabelText('All-up mass'))
    await user.type(screen.getByLabelText('All-up mass'), '4{Enter}')
    await waitFor(() => { expect(queue).toHaveLength(5) })
    const branchB = latest(queue)
    branchB.settle(answerFor(branchB.identity), answeredWith(4))
    await waitFor(() => { expect(shownMass()).toBe('4') })

    // Now the abandoned answer finally arrives. It changes nothing: not the
    // design, not the result, not the status.
    outstanding.settle(answerFor(outstanding.identity), answeredWith(99))
    await new Promise((settle) => setTimeout(settle, 20))
    expect(shownMass()).toBe('4')
    expect(screen.getByText('Current')).toBeInTheDocument()
  })

  test('a redo retires an outstanding identity in the same way', async () => {
    const { queue, user } = await setUp()

    await user.clear(screen.getByLabelText('All-up mass'))
    await user.type(screen.getByLabelText('All-up mass'), '2{Enter}')
    await waitFor(() => { expect(queue).toHaveLength(2) })
    const edit = latest(queue)
    edit.settle(answerFor(edit.identity), answeredWith(2))
    await waitFor(() => { expect(shownMass()).toBe('2') })

    await user.click(screen.getByRole('button', { name: 'Undo' }))
    await waitFor(() => { expect(queue).toHaveLength(3) })
    const outstanding = latest(queue)

    await user.click(screen.getByRole('button', { name: 'Redo' }))
    await waitFor(() => { expect(queue).toHaveLength(4) })
    const afterRedo = latest(queue)

    // The pre-redo answer lands late and is discarded; the post-redo one is
    // accepted. The two are distinguished only by their identities.
    outstanding.settle(answerFor(outstanding.identity))
    afterRedo.settle(answerFor(afterRedo.identity))
    await waitFor(() => { expect(screen.getByText('Current')).toBeInTheDocument() })
    expect(shownMass()).toBe('2')
  })

  test('loading a draft retires an outstanding identity, and shows nothing cached', async () => {
    const { queue, user } = await setUp()

    // Save the design as it stands, then move it and hold the answer back.
    await user.type(screen.getByLabelText('Name'), 'saved')
    await user.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => { expect(screen.getByText(/Saved “saved”/)).toBeInTheDocument() })

    await user.clear(screen.getByLabelText('All-up mass'))
    await user.type(screen.getByLabelText('All-up mass'), '5{Enter}')
    await waitFor(() => { expect(queue).toHaveLength(2) })
    const outstanding = latest(queue)

    await user.click(screen.getByRole('button', { name: 'Open' }))
    await waitFor(() => {
      expect(screen.getByText(/nothing cached was shown as current/)).toBeInTheDocument()
    })
    // Loading asks for a fresh evaluation of the loaded design.
    await waitFor(() => { expect(queue).toHaveLength(3) })
    const afterLoad = latest(queue)

    outstanding.settle(answerFor(outstanding.identity), answeredWith(99))
    afterLoad.settle(answerFor(afterLoad.identity))
    await waitFor(() => { expect(screen.getByText('Current')).toBeInTheDocument() })
    expect(shownMass()).toBe('')
  })

  test('two answers arriving out of order leave only the later one showing', async () => {
    const { queue, user } = await setUp()

    await user.click(screen.getByRole('button', { name: 'Recalculate' }))
    await waitFor(() => { expect(queue).toHaveLength(2) })
    const first = latest(queue)
    await user.click(screen.getByRole('button', { name: 'Recalculate' }))
    await waitFor(() => { expect(queue).toHaveLength(3) })
    const second = latest(queue)

    second.settle(answerFor(second.identity))
    await waitFor(() => { expect(screen.getByText('Current')).toBeInTheDocument() })
    // The earlier request answers afterwards. It is not the outstanding one, so
    // it is ignored rather than overwriting a newer result with an older one.
    first.settle(answerFor(first.identity))
    await new Promise((settle) => setTimeout(settle, 20))
    expect(screen.getByText('Current')).toBeInTheDocument()
  })
})

describe('when the service cannot be reached', () => {
  test('the entries survive, the result stands, and retrying works', async () => {
    const { queue, user } = await setUp()

    // Blur commits, so moving from the span to the mass sends the span on its
    // way. That is the specified behaviour rather than an accident here, so the
    // span goes through the boundary before the mass is touched.
    await user.type(screen.getByLabelText('Span'), '1.2')
    await user.clear(screen.getByLabelText('All-up mass'))
    await waitFor(() => { expect(queue).toHaveLength(2) })
    const span = latest(queue)
    span.settle(answerFor(span.identity), withSpan(1.2))
    await waitFor(() => { expect(screen.getByLabelText('Span')).toHaveValue('1.2') })

    await user.type(screen.getByLabelText('All-up mass'), '2{Enter}')
    await waitFor(() => { expect(queue).toHaveLength(3) })
    latest(queue).fail(new TransportError('http://127.0.0.1:1/api/v1/apply could not be reached'))

    await waitFor(() => {
      expect(screen.getByText('The last request was refused')).toBeInTheDocument()
    })
    expect(screen.getByText(/try again once the calculation service is running/)).toBeInTheDocument()
    // Nothing was lost. The span that did get through still stands, and the
    // mass that was refused is still in its field: the design never took it, so
    // it is an entry waiting to be sent again rather than a value.
    expect(screen.getByLabelText('Span')).toHaveValue('1.2')
    expect(shownMass()).toBe('2')

    // Retrying goes through. Blurring the mass field to reach the button does
    // not resend the refused edit by itself; asking again is the deliberate act.
    await user.click(screen.getByRole('button', { name: 'Recalculate' }))
    await waitFor(() => { expect(queue).toHaveLength(4) })
    const retry = latest(queue)
    retry.settle(answerFor(retry.identity))
    await waitFor(() => { expect(screen.getByText('Current')).toBeInTheDocument() })
    expect(screen.getByLabelText('Span')).toHaveValue('1.2')
    expect(shownMass()).toBe('2')
  })

  test('a refused edit keeps the entry and explains itself against the field', async () => {
    const { queue, user } = await setUp()

    await user.clear(screen.getByLabelText('All-up mass'))
    await user.type(screen.getByLabelText('All-up mass'), '2{Enter}')
    await waitFor(() => { expect(queue).toHaveLength(2) })
    const { BoundaryError } = await import('./api/client.ts')
    latest(queue).fail(
      new BoundaryError('invalid', 'the request could not be read', [
        { field: 'mass', kind: 'invalid', detail: 'must be greater than zero' },
      ]),
    )

    // The complaint is attached to the field it is about, not only listed in
    // the summary, so it is readable where the correction has to be made.
    await waitFor(() => {
      expect(screen.getByLabelText('All-up mass')).toHaveAccessibleDescription(
        /must be greater than zero/,
      )
    })
    expect(screen.getByLabelText('All-up mass')).toBeInvalid()
    expect(screen.getByLabelText('All-up mass')).toHaveValue('2')
  })
})

describe('committing an entry', () => {
  test('leaving the field does not send the edit a second time', async () => {
    const { queue, user } = await setUp()

    await user.clear(screen.getByLabelText('All-up mass'))
    await user.type(screen.getByLabelText('All-up mass'), '2{Enter}')
    await waitFor(() => { expect(queue).toHaveLength(2) })

    // The answer is still on its way, so the entry is still in the field and
    // still differs from the committed value — it has to be, because a refused
    // edit must keep what was typed. Blurring the field to reach a control must
    // not repeat the request that is already carrying it.
    await user.click(screen.getByRole('button', { name: 'Recalculate' }))
    await waitFor(() => { expect(queue).toHaveLength(3) })
    expect(queue.filter((request) => request.kind === 'apply')).toHaveLength(1)
    expect(latest(queue).kind).toBe('evaluate')
  })

  test('pressing Enter again after a refusal asks again', async () => {
    const { queue, user } = await setUp()

    await user.clear(screen.getByLabelText('All-up mass'))
    await user.type(screen.getByLabelText('All-up mass'), '2{Enter}')
    await waitFor(() => { expect(queue).toHaveLength(2) })
    latest(queue).fail(new TransportError('http://127.0.0.1:1/api/v1/apply could not be reached'))
    await waitFor(() => {
      expect(screen.getByText('The last request was refused')).toBeInTheDocument()
    })

    // Enter is a deliberate act. The same text committed again is a retry, not
    // a no-op: the design never took the value, so there is still something to
    // send.
    await user.type(screen.getByLabelText('All-up mass'), '{Enter}')
    await waitFor(() => { expect(queue).toHaveLength(3) })
    expect(latest(queue).kind).toBe('apply')
  })
})

describe('while a request is outstanding', () => {
  test('the worksheet says it is calculating rather than showing a settled result', async () => {
    const { queue, user } = await setUp()
    expect(screen.getByText('Current')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Recalculate' }))
    await waitFor(() => {
      expect(screen.getByText('Calculating…')).toBeInTheDocument()
    })
    // Status is in words, not in colour: the headline itself changes.
    expect(screen.queryByText('Current')).not.toBeInTheDocument()

    const outstanding = latest(queue)
    outstanding.settle(answerFor(outstanding.identity))
    await waitFor(() => { expect(screen.getByText('Current')).toBeInTheDocument() })
  })
})
