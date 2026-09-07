// Projecting the service's three-dimensional geometry onto a flat drawing.
//
// This is presentation arithmetic and nothing else: it turns a point in the
// design's datum into a position on an SVG canvas, and a position on the canvas
// back into a point in the datum. It contains no physical relationship, no
// conversion factor and no rule about what a dimension means — every number it
// consumes came from the service, and every number it produces is a pixel.

import type { Point, SketchCurve, SketchDimension, SketchView } from '../api/contract.ts'

/** Plane is a point reduced to the two axes one view shows. */
export interface Plane {
  readonly across: number
  readonly up: number
}

/** Box is the extent of a view's geometry in its own two axes. */
export interface Box {
  readonly minAcross: number
  readonly maxAcross: number
  readonly minUp: number
  readonly maxUp: number
}

/** axisOf reads one named datum axis off a point. */
export function axisOf(point: Point, axis: string): number {
  switch (axis) {
    case 'x':
      return point.x.value
    case 'y':
      return point.y.value
    case 'z':
      return point.z.value
    default:
      return 0
  }
}

/** project reduces a datum point to the two axes a view shows. */
export function project(point: Point, view: { across: string; up: string }): Plane {
  return { across: axisOf(point, view.across), up: axisOf(point, view.up) }
}

/**
 * pointsDown reports whether the view's "up" axis is drawn down the page.
 *
 * A plan view runs the aircraft's x axis, which is positive aft, and a reader
 * expects the nose at the top; a front or side view runs z, which is positive
 * up and belongs upwards. That is a drawing convention, so it is decided here
 * rather than by the service.
 */
export function pointsDown(view: { up: string }): boolean {
  return view.up === 'x'
}

const EMPTY_BOX: Box = { minAcross: 0, maxAcross: 1, minUp: 0, maxUp: 1 }

/** extent measures everything a view draws, so the drawing fits its content. */
export function extent(view: SketchView): Box {
  const points: Plane[] = []
  for (const curve of view.curves) {
    for (const point of curve.points) {
      const plane = project(point, view)
      points.push(plane)
      // A mirrored curve is drawn on both sides, so both sides must fit.
      if (curve.mirrored) points.push({ across: -plane.across, up: plane.up })
    }
  }
  for (const dimension of view.dimensions) {
    points.push(project(dimension.from, view), project(dimension.to, view))
  }
  if (points.length === 0) return EMPTY_BOX
  const across = points.map((p) => p.across)
  const up = points.map((p) => p.up)
  return {
    minAcross: Math.min(...across),
    maxAcross: Math.max(...across),
    minUp: Math.min(...up),
    maxUp: Math.max(...up),
  }
}

/** include grows a box to contain a point, so a marker outside the wing still fits. */
export function include(box: Box, plane: Plane): Box {
  return {
    minAcross: Math.min(box.minAcross, plane.across),
    maxAcross: Math.max(box.maxAcross, plane.across),
    minUp: Math.min(box.minUp, plane.up),
    maxUp: Math.max(box.maxUp, plane.up),
  }
}

/**
 * Canvas maps between design coordinates and the SVG user space. One scale is
 * used for both axes so that the drawing stays to scale: a wing drawn with
 * different horizontal and vertical scales is not the wing.
 */
export interface Canvas {
  readonly width: number
  readonly height: number
  readonly scale: number
  toScreen(plane: Plane): { x: number; y: number }
  toDesign(x: number, y: number): Plane
}

/**
 * fittedScale is the pixels-per-metre that fits a box into the canvas. It is
 * separate from canvasFor so that two coordinated views can share one scale:
 * a plan and a side view of the same aircraft drawn at different scales are two
 * drawings of two aircraft, and a component that looks further aft in one than
 * in the other is a drawing that lies.
 */
export function fittedScale(box: Box, size = 320, margin = 28): number {
  const spanAcross = Math.max(box.maxAcross - box.minAcross, 1e-6)
  const spanUp = Math.max(box.maxUp - box.minUp, 1e-6)
  const usable = Math.max(size - margin * 2, 1)
  return Math.min(usable / spanAcross, usable / spanUp)
}

/**
 * canvasFor builds the mapping for one view's extent, with a margin around it.
 * Passing a scale overrides the fitted one, which is how coordinated views are
 * drawn to a common scale.
 */
export function canvasFor(
  box: Box,
  view: { up: string },
  size = 320,
  margin = 28,
  shared?: number,
): Canvas {
  const spanAcross = Math.max(box.maxAcross - box.minAcross, 1e-6)
  const spanUp = Math.max(box.maxUp - box.minUp, 1e-6)
  const scale = shared ?? fittedScale(box, size, margin)
  const width = spanAcross * scale + margin * 2
  const height = spanUp * scale + margin * 2
  const down = pointsDown(view)
  return {
    width,
    height,
    scale,
    toScreen(plane) {
      const x = margin + (plane.across - box.minAcross) * scale
      const fromTop = down ? plane.up - box.minUp : box.maxUp - plane.up
      return { x, y: margin + fromTop * scale }
    },
    toDesign(x, y) {
      const across = box.minAcross + (x - margin) / scale
      const fromTop = (y - margin) / scale
      const up = down ? box.minUp + fromTop : box.maxUp - fromTop
      return { across, up }
    },
  }
}

/** polylinePoints renders a curve as an SVG points attribute. */
export function polylinePoints(
  curve: SketchCurve,
  view: SketchView,
  canvas: Canvas,
  mirror = false,
): string {
  return curve.points
    .map((point) => {
      const plane = project(point, view)
      const screen = canvas.toScreen(mirror ? { across: -plane.across, up: plane.up } : plane)
      return `${String(screen.x)},${String(screen.y)}`
    })
    .join(' ')
}

/** dimensionEnds gives a dimension's two endpoints in screen coordinates. */
export function dimensionEnds(
  dimension: SketchDimension,
  view: SketchView,
  canvas: Canvas,
): { from: { x: number; y: number }; to: { x: number; y: number } } {
  return {
    from: canvas.toScreen(project(dimension.from, view)),
    to: canvas.toScreen(project(dimension.to, view)),
  }
}
