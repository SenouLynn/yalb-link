import { useCallback, useRef, useState } from 'react'
import type { PointerEvent as ReactPointerEvent, ReactNode } from 'react'
import type {
  Component,
  Evaluation,
  MassProperties,
  Quantity,
  SketchView,
} from '../api/contract.ts'
import { displayNumber } from '../state/fields.ts'
import type { Box, Plane } from '../state/views.ts'
import { canvasFor, extent, fittedScale, include, polylinePoints, project } from '../state/views.ts'
import type { WorksheetApi } from '../useWorksheet.ts'
import { QuantityField, Section, TextField } from './fields.tsx'

// The visual mass placement.
//
// The drawing maps screen coordinates to design coordinates and back, and that
// is the only arithmetic in this file. Every mass, every station and the centre
// of gravity itself came from the service; a drag asks Go what the placement
// would do, and only a completed drag commits it, as one undoable change.
//
// The references are labelled apart on purpose. A mechanical centre of gravity,
// a wing aerodynamic centre, an aircraft neutral point and a centre of pressure
// are four different things, and only the first is implemented. The quarter-MAC
// marker is a geometric reference and is labelled as one; it is not a centre of
// lift, and calling it one would be a stability claim nothing here supports.

const ROLES = [
  { value: 'airframe', label: 'Airframe' },
  { value: 'battery', label: 'Battery' },
  { value: 'motor', label: 'Motor' },
  { value: 'avionics', label: 'Avionics' },
  { value: 'payload', label: 'Payload' },
  { value: 'other', label: 'Other' },
] as const

/** ROLE_MARKS give each role a shape as well as a place, so no reading of this
 * drawing depends on telling two colours apart. */
const ROLE_MARKS: Readonly<Record<string, string>> = {
  airframe: '■',
  battery: '▬',
  motor: '▲',
  avionics: '◆',
  payload: '⬟',
  other: '○',
}

function metres(value: number): Quantity {
  return { value, unit: 'm' }
}

function coordinate(value: Quantity | null): string {
  return value === null ? 'not placed' : `${displayNumber(value.value)} ${value.unit}`
}

/** placed reports whether every coordinate of a component's position is stated. */
function placed(component: Component): boolean {
  const { x, y, z } = component.position
  return Boolean(x) && Boolean(y) && Boolean(z)
}

/** positionOf reads a component's position as a plain triple, or null when it
 * has not been placed. */
function positionOf(component: Component): { x: number; y: number; z: number } | null {
  const { x, y, z } = component.position
  if (!x || !y || !z) return null
  return { x: x.value, y: y.value, z: z.value }
}

/**
 * ComponentsPanel is the mass inventory: what the aircraft is made of, what
 * each piece weighs, where that figure came from and where it sits.
 */
export function ComponentsPanel(props: { api: WorksheetApi }): ReactNode {
  const { api } = props
  const design = api.design
  const components = design.components ?? []
  const mode = design.massMode ?? 'entered'
  const properties = api.worksheet.current?.evaluation.massProperties ?? null
  const [adding, setAdding] = useState('')

  return (
    <Section title="Components and balance" summary={`${String(components.length)} listed`}>
      <p className="panel-note">
        A component with no mass, or with no complete position, contributes nothing and is
        never assumed to sit at the origin. The balance says which one is still missing.
      </p>

      <div className="field">
        <label className="field-label" htmlFor="mass-mode">Where the all-up mass comes from</label>
        <div className="field-entry">
          <select
            id="mass-mode"
            className="field-input"
            value={mode}
            onChange={(event) => {
              void api.run([{ kind: 'set-mass-mode', mode: event.target.value }])
            }}
          >
            <option value="entered">The entered figure</option>
            <option value="components">The component total</option>
          </select>
        </div>
        <p className="field-hint">
          {mode === 'components'
            ? 'The components are the aircraft. An inventory that is still missing a mass or a '
              + 'position establishes no all-up mass, so the sizing results wait for it.'
            : 'The entered all-up mass is what the design is judged at. The components are still '
              + 'balanced below; adopting their total is a separate, deliberate change.'}
        </p>
      </div>

      {components.map((component) => (
        <ComponentEditor key={component.name} api={api} component={component} />
      ))}

      <div className="field">
        <label className="field-label" htmlFor="component-new">Add a component</label>
        <div className="field-entry">
          <input
            id="component-new"
            className="field-input field-input-wide"
            type="text"
            autoComplete="off"
            value={adding}
            onChange={(event) => { setAdding(event.target.value) }}
          />
          <button
            type="button"
            disabled={adding.trim() === ''}
            onClick={() => {
              const name = adding.trim()
              if (name === '') return
              setAdding('')
              void api.run([{
                kind: 'set-component',
                component: {
                  name,
                  role: 'payload',
                  basis: '',
                  mass: null,
                  position: { x: null, y: null, z: null },
                },
              }])
            }}
          >
            Add
          </button>
        </div>
      </div>

      {properties !== null && <BalanceSummary properties={properties} />}
    </Section>
  )
}

function BalanceSummary(props: { properties: MassProperties }): ReactNode {
  const { properties } = props
  if (properties.status !== 'computed' || !properties.cg || !properties.total) {
    return (
      <p className="panel-note" role="status">
        No centre of gravity yet. {properties.detail}
      </p>
    )
  }
  return (
    <dl className="balance">
      <dt>Total of the listed masses</dt>
      <dd>{displayNumber(properties.total.value)} {properties.total.unit}</dd>
      <dt>Mechanical centre of gravity</dt>
      <dd>
        x {displayNumber(properties.cg.x.value)} m, y {displayNumber(properties.cg.y.value)} m,
        z {displayNumber(properties.cg.z.value)} m
      </dd>
      <dt>Datum</dt>
      <dd>{properties.datum}</dd>
      {!properties.complete && (
        <>
          <dt>Completeness</dt>
          <dd role="status">{properties.detail}</dd>
        </>
      )}
    </dl>
  )
}

function ComponentEditor(props: { api: WorksheetApi; component: Component }): ReactNode {
  const { api, component } = props
  const id = `component-${component.name.replace(/\s+/g, '-').toLowerCase()}`

  function place(axis: 'x' | 'y' | 'z', value: number | null): void {
    const position = component.position
    const next = {
      x: axis === 'x' ? (value === null ? null : metres(value)) : position.x ?? null,
      y: axis === 'y' ? (value === null ? null : metres(value)) : position.y ?? null,
      z: axis === 'z' ? (value === null ? null : metres(value)) : position.z ?? null,
    }
    // A placement carries the whole position, and the core refuses one with a
    // coordinate missing: a component that is only half located is not placed.
    // So a coordinate typed into a component that is already placed is the same
    // command a completed drag sends, and a coordinate typed into one that is
    // still being filled in records the component instead. Both are one
    // undoable edit; only the first claims the component has a position.
    const complete = Boolean(next.x) && Boolean(next.y) && Boolean(next.z)
    void api.run(
      [
        complete
          ? { kind: 'place-component', name: component.name, position: next }
          : { kind: 'set-component', component: { ...component, position: next } },
      ],
      [`${id}-${axis}`],
    )
  }

  return (
    <details className="subsection">
      <summary>
        {ROLE_MARKS[component.role] ?? '○'}{' '}
        <span className="component-name">{component.name}</span>
        <span className="section-summary">
          {' '}
          {component.mass ? `${displayNumber(component.mass.value)} ${component.mass.unit}` : 'no mass'}
          {' · '}
          {placed(component) ? `x ${coordinate(component.position.x ?? null)}` : 'not placed'}
        </span>
      </summary>
      <div className="field">
        <label className="field-label" htmlFor={`${id}-role`}>What it is</label>
        <div className="field-entry">
          <select
            id={`${id}-role`}
            className="field-input"
            value={component.role}
            onChange={(event) => {
              void api.run([{
                kind: 'set-component',
                component: { ...component, role: event.target.value },
              }])
            }}
          >
            {ROLES.map((role) => (
              <option key={role.value} value={role.value}>{role.label}</option>
            ))}
          </select>
        </div>
      </div>
      <QuantityField
        id={`${id}-mass`}
        label="Mass"
        dimension="mass"
        value={component.mass ?? null}
        role="driver"
        api={api}
        hint="Changing a mass changes the aircraft: the total, the wing loading and the stall speed all move with it."
        onCommit={(target) => {
          void api.run(
            [{
              kind: 'set-component',
              component: {
                ...component,
                mass: target === null ? null : { value: target.value, unit: target.unit },
              },
            }],
            [`${id}-mass`],
          )
        }}
      />
      <TextField
        id={`${id}-basis`}
        label="Where the mass comes from"
        value={component.basis}
        api={api}
        hint="An estimate and a weighing are different things."
        onCommit={(basis) => {
          void api.run([{ kind: 'set-component', component: { ...component, basis } }], [`${id}-basis`])
        }}
      />
      {(['x', 'y', 'z'] as const).map((axis) => (
        <QuantityField
          key={axis}
          id={`${id}-${axis}`}
          label={`Position ${axis}`}
          dimension="length"
          value={component.position[axis] ?? null}
          role="driver"
          api={api}
          hint={
            axis === 'x'
              ? 'Positive aft of the root leading edge. Moving a fixed mass changes the balance and leaves the loading alone.'
              : axis === 'y'
                ? 'Positive towards the right tip. Zero is on the centerline, and it has to be said.'
                : 'Positive up from the root chord line.'
          }
          onCommit={(target) => { place(axis, target === null ? null : target.value) }}
        />
      ))}
      <button
        type="button"
        onClick={() => { void api.run([{ kind: 'remove-component', name: component.name }]) }}
      >
        Remove {component.name}
      </button>
    </details>
  )
}

/** Placement is one component at one position: what a preview is asked about. */
interface Placement {
  readonly name: string
  readonly x: number
  readonly y: number
  readonly z: number
}

/** Ghost is a placement being dragged: where it is now, and what Go says it
 * would do. */
interface Ghost extends Placement {
  readonly preview: Evaluation | null
}

/**
 * samePlacement reports whether a held drag is still exactly where a preview
 * was asked about. An answer for anywhere else belongs to a position the
 * builder has already left, and showing it would put an older aircraft's
 * numbers under a newer one's coordinates.
 */
function samePlacement(held: Ghost | null, asked: Placement): held is Ghost {
  if (held === null) return false
  return held.name === asked.name && held.x === asked.x && held.y === asked.y && held.z === asked.z
}

/** CANVAS_SIZE and CANVAS_MARGIN size the placement drawings. They are named
 * because the two views must be measured the same way to share a scale. */
const CANVAS_SIZE = 320
const CANVAS_MARGIN = 28

/**
 * placementExtent measures what a placement view has to fit: the wing geometry
 * and every placed component, because a battery outside the outline still has
 * to be visible where it actually is.
 */
function placementExtent(view: SketchView, components: readonly Component[]): Box {
  let box: Box = extent(view)
  for (const component of components) {
    const position = positionOf(component)
    if (position === null) continue
    box = include(box, project(
      { x: metres(position.x), y: metres(position.y), z: metres(position.z) },
      view,
    ))
  }
  return box
}

/** viewFor picks one of the service's views by name. */
function viewFor(views: readonly SketchView[], name: string): SketchView | null {
  return views.find((view) => view.view === name) ?? null
}

/**
 * PlacementView draws one view with the wing outline, the component markers and
 * the centre of gravity, and lets a marker be dragged.
 */
function PlacementView(props: {
  view: SketchView
  components: readonly Component[]
  properties: MassProperties | null
  ghost: Ghost | null
  quarterMAC: { x: number; y: number } | null
  /** scale is shared between the coordinated views, so the same distance is the
   * same number of pixels in both. */
  scale: number
  onDragStart(name: string): void
  onDragMove(plane: Plane): void
  onDragEnd(): void
  onCancel(): void
}): ReactNode {
  const { view, components, properties, ghost } = props
  const svg = useRef<SVGSVGElement | null>(null)
  const canvas = canvasFor(placementExtent(view, components), view, CANVAS_SIZE, CANVAS_MARGIN, props.scale)

  const toDesign = useCallback(
    (event: ReactPointerEvent): Plane | null => {
      const element = svg.current
      if (element === null) return null
      const rect = element.getBoundingClientRect()
      if (rect.width === 0 || rect.height === 0) return null
      const x = ((event.clientX - rect.left) / rect.width) * canvas.width
      const y = ((event.clientY - rect.top) / rect.height) * canvas.height
      return canvas.toDesign(x, y)
    },
    [canvas],
  )

  function markerAt(component: Component): { x: number; y: number } | null {
    const dragging = ghost !== null && ghost.name === component.name
    const position = dragging
      ? { x: ghost.x, y: ghost.y, z: ghost.z }
      : positionOf(component)
    if (position === null) return null
    return canvas.toScreen(project(
      { x: metres(position.x), y: metres(position.y), z: metres(position.z) },
      view,
    ))
  }

  const cg = properties?.status === 'computed' && properties.cg
    ? canvas.toScreen(project(properties.cg, view))
    : null

  return (
    <figure className="view">
      <figcaption>{view.view}</figcaption>
      <svg
        ref={svg}
        className="view-canvas placement-canvas"
        viewBox={`0 0 ${String(canvas.width)} ${String(canvas.height)}`}
        role="img"
        aria-label={`${view.view} with component placements. ${view.datum}`}
        onPointerMove={(event) => {
          if (ghost === null) return
          const plane = toDesign(event)
          if (plane !== null) props.onDragMove(plane)
        }}
        onPointerUp={() => { if (ghost !== null) props.onDragEnd() }}
        onPointerLeave={() => { if (ghost !== null) props.onCancel() }}
      >
        {view.curves.map((curve, n) => (
          <g key={`${curve.role}-${String(n)}`} className={`curve curve-${curve.role}`}>
            <polyline points={polylinePoints(curve, view, canvas)} fill="none" />
            {curve.mirrored && (
              <polyline points={polylinePoints(curve, view, canvas, true)} fill="none" />
            )}
          </g>
        ))}
        {props.quarterMAC !== null && (
          <g className="reference-marker">
            <circle
              cx={canvas.toScreen(project(
                { x: metres(props.quarterMAC.x), y: metres(props.quarterMAC.y), z: metres(0) },
                view,
              )).x}
              cy={canvas.toScreen(project(
                { x: metres(props.quarterMAC.x), y: metres(props.quarterMAC.y), z: metres(0) },
                view,
              )).y}
              r={5}
            />
          </g>
        )}
        {cg !== null && (
          <g className="cg-marker" aria-hidden="true">
            <circle cx={cg.x} cy={cg.y} r={7} />
            <line x1={cg.x - 9} y1={cg.y} x2={cg.x + 9} y2={cg.y} />
            <line x1={cg.x} y1={cg.y - 9} x2={cg.x} y2={cg.y + 9} />
          </g>
        )}
        {components.map((component) => {
          const at = markerAt(component)
          if (at === null) return null
          return (
            <text
              key={component.name}
              className={`component-marker${ghost?.name === component.name ? ' dragging' : ''}`}
              x={at.x}
              y={at.y}
              textAnchor="middle"
              dominantBaseline="middle"
              role="button"
              tabIndex={0}
              aria-label={`Drag ${component.name}`}
              onPointerDown={() => { props.onDragStart(component.name) }}
            >
              {ROLE_MARKS[component.role] ?? '○'}
            </text>
          )
        })}
      </svg>
    </figure>
  )
}

/**
 * PlacementPanel is the coordinated plan and side view of where the masses sit.
 *
 * A drag is a preview until it is finished. While it is in progress the numbers
 * beside it come from a Go preview of that placement, and the design still
 * holds the old one; releasing commits a single placement change, and cancelling
 * leaves nothing behind.
 */
export function PlacementPanel(props: { api: WorksheetApi }): ReactNode {
  const { api } = props
  const design = api.design
  const components = design.components ?? []
  const evaluation = api.worksheet.current?.evaluation ?? null
  const wing = evaluation?.wing ?? null
  const properties = evaluation?.massProperties ?? null
  const [ghost, setGhost] = useState<Ghost | null>(null)

  // A drag asks Go what the placement would do, and a pointer produces far more
  // positions than there will ever be answers. One request is in flight at a
  // time and only the newest position waits its turn, so the work stays bounded
  // however fast the pointer moves.
  //
  // An answer is applied only when the marker is still exactly where it was
  // when the question was asked. That is what stops a delayed preview from
  // overwriting a later placement: an answer for a position the builder has
  // already left is dropped rather than shown under the newer coordinates.
  const wanted = useRef<Placement | null>(null)
  const inFlight = useRef(false)
  const pump = useRef<() => void>(() => undefined)

  pump.current = () => {
    if (inFlight.current) return
    const asked = wanted.current
    if (asked === null) return
    wanted.current = null
    inFlight.current = true
    void api
      .previewCommand({
        kind: 'place-component',
        name: asked.name,
        position: { x: metres(asked.x), y: metres(asked.y), z: metres(asked.z) },
      })
      .then((preview) => {
        inFlight.current = false
        if (preview !== null) {
          setGhost((held) => (samePlacement(held, asked) ? { ...held, preview } : held))
        }
        pump.current()
      })
  }

  const requestPreview = useCallback((asked: Placement) => {
    wanted.current = asked
    pump.current()
  }, [])

  if (wing === null) {
    return (
      <Section title="Where the masses sit">
        <p>
          The wing has not solved, so there is no outline to place components against. The
          masses and their coordinates are still editable above, and the balance is still
          reported.
        </p>
      </Section>
    )
  }

  const plan = viewFor(wing.views, 'plan-view')
  const side = viewFor(wing.views, 'side-view')
  const quarterMAC = quarterMACPoint(wing.parameters)
  // One scale for both drawings: a plan and a side view of the same aircraft at
  // different scales are two drawings of two aircraft, and a component that
  // looks further aft in one than in the other is a drawing that lies. The
  // tighter of the two fits, so nothing is cropped.
  const scale = Math.min(
    ...[plan, side]
      .filter((view): view is SketchView => view !== null)
      .map((view) => fittedScale(placementExtent(view, components), CANVAS_SIZE, CANVAS_MARGIN)),
  )

  function startDrag(name: string): void {
    const component = components.find((entry) => entry.name === name)
    const position = component ? positionOf(component) : null
    if (position === null) return
    setGhost({ name, ...position, preview: null })
  }

  function moveDrag(plane: Plane, view: SketchView): void {
    setGhost((held) => {
      if (held === null) return held
      const next = { ...held, ...axisUpdate(view, plane) }
      requestPreview({ name: next.name, x: next.x, y: next.y, z: next.z })
      return next
    })
  }

  function endDrag(): void {
    const held = ghost
    setGhost(null)
    if (held === null) return
    void api.run([{
      kind: 'place-component',
      name: held.name,
      position: { x: metres(held.x), y: metres(held.y), z: metres(held.z) },
    }])
  }

  return (
    <Section title="Where the masses sit">
      <p className="panel-note">
        Drag a marker, or type its coordinates above; the two do the same thing. While a drag
        is in progress the numbers below come from a preview and nothing has been changed.
        Releasing records one placement, which undoes in one step.
      </p>
      <div className="views">
        {plan !== null && (
          <PlacementView
            view={plan}
            components={components}
            properties={properties}
            ghost={ghost}
            quarterMAC={quarterMAC}
            scale={scale}
            onDragStart={startDrag}
            onDragMove={(plane) => { moveDrag(plane, plan) }}
            onDragEnd={endDrag}
            onCancel={() => { setGhost(null) }}
          />
        )}
        {side !== null && (
          <PlacementView
            view={side}
            components={components}
            properties={properties}
            ghost={ghost}
            quarterMAC={quarterMAC}
            scale={scale}
            onDragStart={startDrag}
            onDragMove={(plane) => { moveDrag(plane, side) }}
            onDragEnd={endDrag}
            onCancel={() => { setGhost(null) }}
          />
        )}
      </div>

      {ghost !== null && (
        <div className="drag-readout" role="status">
          <p>
            Placing {ghost.name} at x {displayNumber(ghost.x)} m, y {displayNumber(ghost.y)} m,
            z {displayNumber(ghost.z)} m. Nothing has been changed yet.
          </p>
          <BeforeAfter before={properties} after={ghost.preview?.massProperties ?? null} />
          <button type="button" onClick={() => { setGhost(null) }}>Cancel placement</button>
        </div>
      )}

      {evaluation !== null && <References evaluation={evaluation} quarterMAC={quarterMAC} />}
    </Section>
  )
}

/** axisUpdate turns a position in one view's two axes into a coordinate change.
 * The third axis is left exactly as it was: a plan view says nothing about
 * height, and moving a marker in it must not silently invent one. */
function axisUpdate(view: SketchView, plane: Plane): Partial<Record<'x' | 'y' | 'z', number>> {
  const update: Partial<Record<'x' | 'y' | 'z', number>> = {}
  if (view.across === 'x' || view.across === 'y' || view.across === 'z') {
    update[view.across] = plane.across
  }
  if (view.up === 'x' || view.up === 'y' || view.up === 'z') {
    update[view.up] = plane.up
  }
  return update
}

function BeforeAfter(props: { before: MassProperties | null; after: MassProperties | null }): ReactNode {
  const before = props.before?.status === 'computed' ? props.before.cg : null
  const after = props.after?.status === 'computed' ? props.after.cg : null
  return (
    <dl className="before-after">
      <dt>Centre of gravity now</dt>
      <dd>{before ? `x ${displayNumber(before.x.value)} m` : 'not available'}</dd>
      <dt>If this placement is kept</dt>
      <dd>{after ? `x ${displayNumber(after.x.value)} m` : 'waiting for the service'}</dd>
      <dt>All-up mass</dt>
      <dd>unchanged — moving a fixed mass changes neither the total nor the wing loading</dd>
    </dl>
  )
}

/** quarterMACPoint locates the quarter-chord point of the mean aerodynamic
 * chord from the parameters the service reported. It is a geometric reference
 * and nothing else. */
function quarterMACPoint(
  parameters: readonly { key: string; value: Quantity }[],
): { x: number; y: number } | null {
  const read = (key: string): number | null =>
    parameters.find((parameter) => parameter.key === key)?.value.value ?? null
  const leading = read('wing.station.mac_leading_edge')
  const chord = read('wing.chord.mac')
  const station = read('wing.station.mac')
  if (leading === null || chord === null || station === null) return null
  return { x: leading + chord / 4, y: station }
}

function References(props: {
  evaluation: Evaluation
  quarterMAC: { x: number; y: number } | null
}): ReactNode {
  const { evaluation } = props
  return (
    <details className="subsection" open>
      <summary>What the markers mean</summary>
      <dl className="references">
        <dt>Mechanical centre of gravity (the cross)</dt>
        <dd>
          The mass-weighted mean of the listed components, in the datum above. Evidence: the
          masses and positions entered, and nothing else.
        </dd>
        <dt>Quarter of the mean aerodynamic chord (the small circle)</dt>
        <dd>
          {props.quarterMAC === null
            ? 'Not available until the wing solves.'
            : 'A geometric reference on the reference planform. It is not a wing aerodynamic '
              + 'centre, not an aircraft neutral point and not a centre of pressure, and it is '
              + 'not a “centre of lift”: no implemented model supports that reading.'}
        </dd>
        <dt>Wing aerodynamic centre, aircraft neutral point, centre of pressure</dt>
        <dd>Unknown. No model here produces any of them, so none is drawn.</dd>
        <dt>Static margin and trim</dt>
        <dd>
          Unknown. Where the centre of gravity sits relative to any of the markers above
          carries no handling conclusion until Task 08 supplies a supported model.
        </dd>
        <dt>Required lift</dt>
        <dd>
          <LoadList evaluation={evaluation} />
        </dd>
      </dl>
    </details>
  )
}

function LoadList(props: { evaluation: Evaluation }): ReactNode {
  const loads = props.evaluation.loads
  if (loads.length === 0) return <>No flight case is defined, so no load follows.</>
  return (
    <ul>
      {loads.map((load) => (
        <li key={load.case}>
          {load.case}:{' '}
          {load.status === 'computed' && load.requiredLift
            ? `${displayNumber(load.requiredLift.value)} ${load.requiredLift.unit}`
            : `unknown — ${load.detail ?? ''}`}
          {' '}— a magnitude only. The lumped model solves no line of action, so no arrow is
          drawn for it.
        </li>
      ))}
    </ul>
  )
}

