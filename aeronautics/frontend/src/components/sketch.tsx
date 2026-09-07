import type { ReactNode } from 'react'
import type { Explanation, Parameter, SketchDimension, SketchView } from '../api/contract.ts'
import { displayNumber } from '../state/fields.ts'
import { canvasFor, dimensionEnds, extent, polylinePoints } from '../state/views.ts'
import type { WorksheetApi } from '../useWorksheet.ts'
import { Section } from './fields.tsx'

// The dimension and formula views.
//
// Nothing here computes a coordinate, a length or an angle: every point, every
// dimension and every formula came back from the service, and this file turns
// them into lines on a canvas. The one piece of arithmetic is the projection
// from design coordinates to pixels, which lives in state/views.ts.
//
// A dimension and a worksheet field are the same thing seen twice. Both carry
// the core's stable parameter key, so selecting either highlights the other,
// and the selection lives in the worksheet state rather than in this component.

/** planeLabel says which plane a dimension is measured in, in words. */
function planeLabel(plane: string): string {
  return plane === 'panel-surface' ? 'panel as built' : 'plan-view projection'
}

/**
 * quantityText renders a value for a label. It rounds for display only: the
 * copyable parameter table below carries the physical precision.
 */
function quantityText(value: { value: number; unit: string }): string {
  if (value.unit === '1') return displayNumber(value.value)
  if (value.unit === 'rad') return `${displayNumber((value.value * 180) / Math.PI)}°`
  return `${displayNumber(value.value)} ${value.unit}`
}

function dimensionName(dimension: SketchDimension): string {
  return `${dimension.label}, ${quantityText(dimension.value)}, ${planeLabel(dimension.plane)}`
}

/** ViewDrawing draws one orthographic view and its dimensions. */
function ViewDrawing(props: { view: SketchView; api: WorksheetApi }): ReactNode {
  const { view, api } = props
  const box = extent(view)
  const canvas = canvasFor(box, view)

  return (
    <figure className="view">
      <figcaption>
        {view.view}
        <span className="view-axes">
          {' '}
          — across: {view.across}, up: {view.up}
        </span>
      </figcaption>
      <svg
        className="view-canvas"
        viewBox={`0 0 ${String(canvas.width)} ${String(canvas.height)}`}
        role="img"
        aria-label={`${view.view}. ${view.datum}`}
      >
        {view.curves.map((curve, n) => (
          <g key={`${curve.role}-${String(n)}`} className={`curve curve-${curve.role}`}>
            <polyline
              points={polylinePoints(curve, view, canvas)}
              fill="none"
              {...(curve.closed ? { className: 'closed' } : {})}
            />
            {curve.mirrored && (
              <polyline points={polylinePoints(curve, view, canvas, true)} fill="none" />
            )}
          </g>
        ))}
        {view.dimensions.map((dimension) => (
          <DimensionMark
            key={`${view.view}-${dimension.key}`}
            dimension={dimension}
            view={view}
            canvas={canvas}
            api={api}
          />
        ))}
      </svg>
    </figure>
  )
}

function DimensionMark(props: {
  dimension: SketchDimension
  view: SketchView
  canvas: ReturnType<typeof canvasFor>
  api: WorksheetApi
}): ReactNode {
  const { dimension, view, canvas, api } = props
  const { from, to } = dimensionEnds(dimension, view, canvas)
  const selected = api.selection === dimension.key
  const midX = (from.x + to.x) / 2
  const midY = (from.y + to.y) / 2

  return (
    <g
      className={`dimension dimension-${dimension.kind} plane-${dimension.plane}${selected ? ' selected' : ''}`}
      role="button"
      tabIndex={0}
      aria-pressed={selected}
      aria-label={dimensionName(dimension)}
      onClick={() => { api.select(selected ? null : dimension.key) }}
      onKeyDown={(event) => {
        if (event.key !== 'Enter' && event.key !== ' ') return
        event.preventDefault()
        api.select(selected ? null : dimension.key)
      }}
    >
      <line x1={from.x} y1={from.y} x2={to.x} y2={to.y} />
      <circle cx={from.x} cy={from.y} r={selected ? 4 : 2.5} />
      <circle cx={to.x} cy={to.y} r={selected ? 4 : 2.5} />
      <text x={midX} y={midY - 4}>
        {dimension.label}
      </text>
    </g>
  )
}

/**
 * ExplanationView is what a selected dimension or field says: the relationship,
 * the revision, the values actually substituted and the result. Every one of
 * them came from the service; nothing here reconstructs a formula.
 */
function ExplanationView(props: { explanation: Explanation; api: WorksheetApi }): ReactNode {
  const { explanation, api } = props
  return (
    <div className="explanation">
      <h4>{explanation.key}</h4>
      <p>{explanation.detail}</p>
      {explanation.role === 'derived' && (
        <>
          <p className="expression">
            <code>{explanation.expression}</code>
            {explanation.revision !== '' && (
              <span className="revision"> · revision {explanation.revision}</span>
            )}
          </p>
          {explanation.substitutions.length > 0 && (
            <table className="substitutions">
              <caption>Values substituted</caption>
              <thead>
                <tr>
                  <th scope="col">Input</th>
                  <th scope="col">Value</th>
                </tr>
              </thead>
              <tbody>
                {explanation.substitutions.map((substitution) => (
                  <tr key={substitution.name}>
                    <th scope="row">{substitution.name}</th>
                    <td>{quantityText(substitution.value)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
          {explanation.dependsOn.length > 0 && (
            <p className="depends">
              Depends on:{' '}
              {explanation.dependsOn.map((key) => (
                <button
                  key={key}
                  type="button"
                  className="inline"
                  onClick={() => { api.select(key) }}
                >
                  {key}
                </button>
              ))}
            </p>
          )}
        </>
      )}
      <p className="result">
        Result: <strong>{quantityText(explanation.value)}</strong>
      </p>
    </div>
  )
}

/**
 * copyText puts the parameter table on the clipboard where there is one. The
 * clipboard is absent outside a secure context and a permission can be refused,
 * and the text is on the page either way, so a failure costs nothing a builder
 * cannot work around by selecting the box beside the button.
 */
async function copyText(text: string): Promise<void> {
  const clipboard = navigator.clipboard as Clipboard | undefined
  if (clipboard === undefined) return
  try {
    await clipboard.writeText(text)
  } catch {
    // Refused. The text box still holds it.
  }
}

/** tabSeparated renders the parameter table at full physical precision. */
function tabSeparated(parameters: readonly Parameter[], explanations: readonly Explanation[]): string {
  const formulas = new Map(explanations.map((entry) => [entry.key, entry]))
  const header = ['name', 'value', 'unit', 'role', 'formula', 'equation', 'revision'].join('\t')
  const rows = parameters.map((parameter) => {
    const explanation = formulas.get(parameter.key)
    return [
      parameter.key,
      // String() is the shortest text that reads back as the same float64, so
      // copying loses nothing. The displayed table rounds; this does not.
      String(parameter.value.value),
      parameter.value.unit,
      parameter.role,
      explanation?.expression ?? '',
      parameter.equationId ?? '',
      parameter.revision ?? '',
    ].join('\t')
  })
  return [header, ...rows].join('\n')
}

/**
 * ParameterTable is the copyable handoff. It is a generic geometry table, not a
 * Fusion-ready one: Task 11 is where the expressions and the units are verified
 * against a real CAD parameter set, and until then nothing here claims they can
 * be pasted in.
 */
function ParameterTable(props: {
  parameters: readonly Parameter[]
  explanations: readonly Explanation[]
  api: WorksheetApi
}): ReactNode {
  const { parameters, explanations, api } = props
  const text = tabSeparated(parameters, explanations)
  return (
    <details className="subsection">
      <summary>Parameter table</summary>
      <p className="panel-note">
        A generic geometry handoff. The values below are shown rounded and copied at full
        precision. Task 11 adds the verified Fusion-ready form; until then these are
        explanatory relationships, not driving constraints, and a maximum span stays a
        requirement rather than a sketch dimension.
      </p>
      <table className="parameters">
        <caption className="sr-only">Named parameters, roles and formulas</caption>
        <thead>
          <tr>
            <th scope="col">Name</th>
            <th scope="col">Value</th>
            <th scope="col">Role</th>
            <th scope="col">Formula</th>
          </tr>
        </thead>
        <tbody>
          {parameters.map((parameter) => {
            const explanation = explanations.find((entry) => entry.key === parameter.key)
            const selected = api.selection === parameter.key
            return (
              <tr key={parameter.key} className={selected ? 'selected' : undefined}>
                <th scope="row">
                  <button
                    type="button"
                    className="inline"
                    aria-pressed={selected}
                    onClick={() => { api.select(selected ? null : parameter.key) }}
                  >
                    {parameter.key}
                  </button>
                </th>
                <td>{quantityText(parameter.value)}</td>
                <td>{parameter.role === 'driver' ? 'input' : 'derived'}</td>
                <td>
                  <code>{explanation?.expression ?? 'entered'}</code>
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
      <label className="field-label" htmlFor="parameter-copy">
        Copy as tab-separated values
      </label>
      <textarea id="parameter-copy" className="copy-area" readOnly rows={4} value={text} />
      <button type="button" onClick={() => { void copyText(text) }}>
        Copy parameters
      </button>
    </details>
  )
}

/**
 * SketchPanel is the dimensioned drawing and the formulas behind it.
 *
 * It is a view of the design definition, not a second copy of it: selecting a
 * dimension selects the parameter, which is the same key the worksheet field
 * carries, and changing a driver redraws both.
 */
export function SketchPanel(props: { api: WorksheetApi }): ReactNode {
  const { api } = props
  const evaluation = api.worksheet.current?.evaluation ?? null
  const wing = evaluation?.wing ?? null

  if (wing === null) {
    return (
      <Section title="Dimensions and formulas">
        <p>
          The wing has not solved, so there is nothing dimensioned to draw. The relationships
          are still listed in the geometry results once two size values are entered.
        </p>
      </Section>
    )
  }

  const selected = wing.explanations.find((entry) => entry.key === api.selection) ?? null

  return (
    <Section title="Dimensions and formulas">
      <p className="panel-note">
        Origin, axes and the centerline are drawn, because a sketch that does not show what
        its dimensions are measured from cannot be rebuilt from. Selecting a dimension shows
        the relationship behind it and highlights the field that drives it. Panel dimensions
        are drawn dashed and labelled as the panel as built; everything else is a plan-view
        projection.
      </p>
      <div className="views">
        {wing.views.map((view) => (
          <ViewDrawing key={view.view} view={view} api={api} />
        ))}
      </div>
      {selected === null ? (
        <p className="panel-note">
          Nothing is selected. Choose a dimension on a drawing, or a name in the parameter
          table, to see what it follows from. The same information is in the table without
          the diagram.
        </p>
      ) : (
        <ExplanationView explanation={selected} api={api} />
      )}
      <ParameterTable parameters={wing.parameters} explanations={wing.explanations} api={api} />
    </Section>
  )
}
