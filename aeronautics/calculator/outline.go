package calculator

import "math"

// DatumWingRoot names the coordinate convention every wing coordinate in this
// package uses. Coordinates are meaningless without it, so it travels with the
// outline and with every positional parameter.
const DatumWingRoot = "origin at the reference planform's root leading edge on the centerline; " +
	"x positive aft along the root chord line, y positive towards the right tip, z positive up"

// OutlinePlane distinguishes a plan-view projection from the surface the panel
// is actually built on. A projected outline is not a cutting template.
type OutlinePlane uint8

const (
	// OutlinePlaneUnknown is the zero value and is never returned by a solve.
	OutlinePlaneUnknown OutlinePlane = iota
	// OutlinePlanView is the projection onto the horizontal plane, z = 0.
	OutlinePlanView
	// OutlinePanelSurface is the panel as built, lifted by the dihedral angle.
	OutlinePanelSurface
)

var outlinePlaneNames = [...]string{
	OutlinePlaneUnknown: "unknown",
	OutlinePlanView:     "plan view projection",
	OutlinePanelSurface: "panel surface as built",
}

// String returns the plane's readable name.
func (p OutlinePlane) String() string {
	if int(p) < len(outlinePlaneNames) {
		return outlinePlaneNames[p]
	}
	return "unknown"
}

// Point is a coordinate in the wing datum.
type Point struct {
	X Quantity
	Y Quantity
	Z Quantity
}

// Outline is the closed corner sequence of the right panel: root leading edge,
// tip leading edge, tip trailing edge, root trailing edge. The left panel is
// its mirror in y. It reconstructs the solved planform's span, area and chords;
// it is not a manufacturing drawing, and it describes no airfoil section, so it
// is not a 3D loft either.
type Outline struct {
	// Datum names the coordinate convention. It is always DatumWingRoot.
	Datum string
	// Points are the corners in order, starting at the root leading edge.
	Points []Point
	// Plane records whether these are projected or as-built coordinates.
	Plane OutlinePlane
}

// Outline returns the right panel's corner coordinates in the requested plane.
func (w Wing) Outline(plane OutlinePlane) (Outline, error) {
	semi := w.Projected.Half
	if !semi.supplied() || semi.si <= 0 {
		return Outline{}, Issues{{
			Field:  portSpan.Name,
			Kind:   IssueMissing,
			Detail: "an outline needs a solved span",
		}}
	}
	tipZ := Quantity{dim: DimLength}
	switch plane {
	case OutlinePlanView:
	case OutlinePanelSurface:
		tipZ = w.TipRise
	case OutlinePlaneUnknown:
		return Outline{}, Issues{{
			Field:  "plane",
			Kind:   IssueMissing,
			Detail: "choose the plan view or the panel surface: a projected outline is not a cutting template",
		}}
	default:
		return Outline{}, Issues{{
			Field:  "plane",
			Kind:   IssueUnsupported,
			Detail: "only the plan view and the panel surface are implemented",
		}}
	}
	tipLE := w.TipLeadingEdgeOffset
	length := func(v float64) Quantity { return Quantity{si: v, dim: DimLength} }
	return Outline{
		Plane: plane,
		Datum: DatumWingRoot,
		Points: []Point{
			{X: length(0), Y: length(0), Z: length(0)},
			{X: tipLE, Y: semi, Z: tipZ},
			{X: length(tipLE.si + w.Planform.TipChord.si), Y: semi, Z: tipZ},
			{X: w.Planform.RootChord, Y: length(0), Z: length(0)},
		},
	}, nil
}

// tipLeadingEdgeOffset returns the plan-view offset from the root leading edge
// to the tip leading edge.
func tipLeadingEdgeOffset(span, leadingEdgeSweep Quantity) (Result, error) {
	e := newEvaluation(EqTipLeadingEdgeOffset)
	b := e.quantity(portSpan.Name, span)
	l := e.signedQuantity(portLeadingEdgeSweep.Name, leadingEdgeSweep)
	return e.finishSigned(b / 2 * math.Tan(l))
}

// macLeadingEdgeStation returns how far aft of the root leading edge the MAC's
// own leading edge sits.
func macLeadingEdgeStation(macStation, leadingEdgeSweep Quantity) (Result, error) {
	e := newEvaluation(EqMACLeadingEdgeStation)
	y := e.signedQuantity(portStation.Name, macStation)
	l := e.signedQuantity(portLeadingEdgeSweep.Name, leadingEdgeSweep)
	return e.finishSigned(y * math.Tan(l))
}

// tipRise returns the height of the tip above the root chord line. It is zero
// for a flat wing and negative for anhedral.
func tipRise(projectedSpan, dihedral Quantity) (Result, error) {
	e := newEvaluation(EqTipRise)
	b := e.quantity(portSpan.Name, projectedSpan)
	g := e.signedQuantity(portDihedral.Name, dihedral)
	return e.finishSigned(b / 2 * math.Tan(g))
}
