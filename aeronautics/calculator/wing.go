package calculator

import "math"

// AreaBasis records what a supplied area actually measures. Imported geometry
// must state it: reading an exposed area as a reference area silently shrinks
// the wing.
type AreaBasis uint8

const (
	// AreaBasisUnknown is the zero value and is never accepted.
	AreaBasisUnknown AreaBasis = iota
	// AreaBasisReferenceTrapezoid is the straight-tapered trapezoid carried
	// through the centerline, including the part inside any body.
	AreaBasisReferenceTrapezoid
	// AreaBasisExposedPanels is the area outside the body only.
	AreaBasisExposedPanels
)

var areaBasisNames = [...]string{
	AreaBasisUnknown:            "unknown",
	AreaBasisReferenceTrapezoid: "reference trapezoid through the centerline",
	AreaBasisExposedPanels:      "exposed panels outside the body",
}

// String returns the basis's readable name.
func (b AreaBasis) String() string {
	if int(b) < len(areaBasisNames) {
		return areaBasisNames[b]
	}
	return "unknown"
}

// DihedralMode states which dimensions a builder holds fixed as dihedral
// changes. Without it a dihedral change silently either shrinks the plan view
// or stretches the built panel, and the two are different aircraft.
type DihedralMode uint8

const (
	// DihedralModeUnknown is the zero value. It is only accepted at zero
	// dihedral, where the two planes coincide.
	DihedralModeUnknown DihedralMode = iota
	// DihedralHoldPanel treats the drivers as panel-plane construction
	// dimensions: the built panel keeps its size and the plan view foreshortens.
	DihedralHoldPanel
	// DihedralHoldProjected treats the drivers as plan-view dimensions: the
	// projected reference keeps its size and the built panel grows.
	DihedralHoldProjected
)

var dihedralModeNames = [...]string{
	DihedralModeUnknown:   "unknown",
	DihedralHoldPanel:     "hold panel dimensions",
	DihedralHoldProjected: "hold projected dimensions",
}

// String returns the mode's readable name.
func (m DihedralMode) String() string {
	if int(m) < len(dihedralModeNames) {
		return dihedralModeNames[m]
	}
	return "unknown"
}

// Airfoil records which section a surface uses and where that identity came
// from. It carries no polar: a planform does not define a section, and no
// method in this package derives lift or drag from a designation.
type Airfoil struct {
	// Designation names the section, for example "NACA 23012" or "SD7037".
	Designation string
	// Evidence states where the identity and any data came from, for example
	// "chosen from the UIUC database listing, no polar imported".
	Evidence string
	// ThicknessRatio is t/c where known, and zero where it is not stated.
	ThicknessRatio float64
}

func (a Airfoil) supplied() bool { return a != Airfoil{} }

// WingDefinition is the authoritative parametric definition of one wing. Angles
// are required rather than defaulted: an unstated sweep, dihedral, twist or
// incidence is a missing field, not zero, because a later handling model cannot
// tell an unswept wing from an unrecorded one.
type WingDefinition struct {
	// Name identifies the wing within a design.
	Name string
	// RootAirfoil and TipAirfoil are optional section identities.
	RootAirfoil Airfoil
	// TipAirfoil is the tip section identity.
	TipAirfoil Airfoil
	// Drivers are the plan-form size drivers, expressed in the plane named by
	// DihedralMode.
	Drivers PlanformDrivers
	// Sweep is the sweep angle at SweepReference, positive aft. Forward sweep is
	// a negative angle and is supported.
	Sweep Quantity
	// Dihedral is the uniform dihedral angle, positive tips up. Anhedral is a
	// negative angle and is supported.
	Dihedral Quantity
	// Twist is the geometric twist from root to tip. Washout is negative.
	Twist Quantity
	// Incidence is the root incidence relative to the fuselage reference line.
	Incidence Quantity
	// BodyWidth is the plan-view width of the body the wing passes through,
	// measured at the wing. It is optional; without it no exposed area is
	// reported.
	BodyWidth Quantity
	// SweepReference is the chord fraction Sweep is measured at: 0 at the
	// leading edge, 0.25 at the quarter chord.
	SweepReference float64
	// AreaBasis states what a supplied area measures.
	AreaBasis AreaBasis
	// DihedralMode states which dimensions stay fixed as dihedral changes.
	DihedralMode DihedralMode
}

// SpanwiseDimensions groups the dimensions that differ between the plan view
// and the panel plane. Chords, taper ratio and MAC are the same length in both:
// rotating a panel about the root chord line changes spanwise extent, not chord.
type SpanwiseDimensions struct {
	// Span is the tip-to-tip span in this plane.
	Span Quantity
	// Area is the reference area in this plane.
	Area Quantity
	// YMAC is the spanwise station of the MAC in this plane.
	YMAC Quantity
	// Half is half the span: the length of one panel in this plane, which is
	// what a sketch is usually driven by.
	Half Quantity
}

// Wing is a solved wing: its planform, the same wing in both the plan view and
// the panel plane, and the derived edges. It is geometry only. No stability,
// trim or control conclusion follows from any of these numbers.
type Wing struct {
	// Traces record every relationship evaluated, in evaluation order.
	Traces []Trace
	// Planform is the solved plan form in the plane the drivers were given in,
	// which Planform.Plane records. Projected and Panel below hold both planes
	// explicitly; prefer them over this field when the plane matters.
	Planform Planform
	// Definition echoes the drivers and choices this wing was solved from.
	Definition WingDefinition
	// Projected holds the plan-view dimensions.
	Projected SpanwiseDimensions
	// Panel holds the panel-plane construction dimensions.
	Panel SpanwiseDimensions
	// LeadingEdgeSweep is the plan-view sweep of the leading edge, positive aft.
	LeadingEdgeSweep Quantity
	// ExposedArea is the plan-view area outside the body. It is the zero
	// Quantity when no body width was supplied, which means "not reported".
	ExposedArea Quantity
	// TipLeadingEdgeOffset is the plan-view distance from the root leading edge
	// aft to the tip leading edge. Forward sweep makes it negative.
	TipLeadingEdgeOffset Quantity
	// TipRise is the height of the tip above the root chord line. Anhedral makes
	// it negative.
	TipRise Quantity
	// MACLeadingEdgeStation is how far aft of the root leading edge the MAC's
	// leading edge sits. It is what a later centre-of-gravity position would be
	// measured against; it is not itself a balance result.
	MACLeadingEdgeStation Quantity
	// ProjectedAspectRatio is b^2/S in the plan view, which is the aspect ratio
	// an aerodynamic model uses.
	ProjectedAspectRatio float64
}

// angle ranges the implemented geometry model covers. Beyond them the plan-view
// reference stops describing the surface usefully, so the input is reported as
// unsupported rather than as impossible.
const (
	maxSweepDegrees    = 60.0
	maxDihedralDegrees = 60.0
	maxTwistDegrees    = 15.0
)

// SolveWing solves a wing definition into its plan-view and panel-plane
// geometry. It reports every issue in a stage rather than only the first.
func SolveWing(def WingDefinition) (Wing, error) {
	rs := &resultSet{}
	def.validateChoices(rs)
	planform, err := SolvePlanform(def.Drivers)
	if err != nil {
		if issues, ok := AsIssues(err); ok {
			rs.issues = append(rs.issues, issues...)
		}
	}
	if len(rs.issues) > 0 {
		return Wing{}, rs.issues
	}
	rs.traces = append(rs.traces, planform.Traces...)

	// Record which plane the planform's own dimensions are in before anything
	// reads them. Holding panel dimensions fixed under a nonzero dihedral means
	// planform.Span is a construction length, not a plan-view span; at zero
	// dihedral the planes coincide and the distinction cannot change a result.
	if def.DihedralMode == DihedralHoldPanel && !def.Dihedral.IsZero() {
		planform.Plane = OutlinePanelSurface
	}

	w := Wing{Definition: def, Planform: planform}
	w.Projected, w.Panel = def.resolvePlanes(rs, planform)
	if len(rs.issues) > 0 {
		return Wing{}, rs.issues
	}
	w.ProjectedAspectRatio = rs.take(aspectRatio(w.Projected.Span, w.Projected.Area)).si
	w.LeadingEdgeSweep = rs.take(sweepAt(def.Sweep, def.SweepReference, 0,
		w.ProjectedAspectRatio, planform.TaperRatio))
	w.TipLeadingEdgeOffset = rs.take(tipLeadingEdgeOffset(w.Projected.Span, w.LeadingEdgeSweep))
	w.TipRise = rs.take(tipRise(w.Projected.Span, def.Dihedral))
	w.MACLeadingEdgeStation = rs.take(macLeadingEdgeStation(w.Projected.YMAC, w.LeadingEdgeSweep))
	w.ExposedArea = def.exposedArea(rs, w, planform)
	if len(rs.issues) > 0 {
		return Wing{}, rs.issues
	}
	w.Traces = rs.traces
	return w, nil
}

// validateChoices checks the choices that are recorded rather than evaluated.
func (def WingDefinition) validateChoices(rs *resultSet) {
	if def.AreaBasis != AreaBasisReferenceTrapezoid {
		rs.add("area_basis", basisIssue(def.AreaBasis),
			"supply the reference trapezoid area carried through the centerline; an exposed "+
				"area is a different measurement and converting one to the other is not implemented")
	}
	requireAngle(rs, "sweep", def.Sweep, maxSweepDegrees,
		"state the sweep angle, using 0 for an unswept wing at the stated reference")
	if def.SweepReference < 0 || def.SweepReference > 1 {
		rs.add("sweep_reference", IssueInvalid,
			"the sweep reference is a chord fraction between 0 and 1, received "+
				formatFloat(def.SweepReference))
	}
	requireAngle(rs, "dihedral", def.Dihedral, maxDihedralDegrees,
		"state the dihedral angle, using 0 for a flat wing")
	requireAngle(rs, "twist", def.Twist, maxTwistDegrees,
		"state the geometric twist, using 0 for an untwisted wing; washout is negative")
	requireAngle(rs, "incidence", def.Incidence, maxTwistDegrees,
		"state the root incidence, using 0 when the root chord lies on the fuselage reference line")
	def.RootAirfoil.validate(rs, "root_airfoil")
	def.TipAirfoil.validate(rs, "tip_airfoil")
}

func basisIssue(b AreaBasis) IssueKind {
	if b == AreaBasisUnknown {
		return IssueMissing
	}
	return IssueUnsupported
}

// requireAngle demands an explicitly supplied angle within the supported range.
func requireAngle(rs *resultSet, field string, q Quantity, maxDegrees float64, missing string) {
	if !q.supplied() {
		rs.add(field, IssueMissing, missing)
		return
	}
	if q.dim != DimAngle {
		rs.add(field, IssueInvalid, "expected an angle but received "+q.dim.String())
		return
	}
	limit := maxDegrees * math.Pi / 180
	if math.Abs(q.si) > limit {
		rs.add(field, IssueUnsupported,
			"the implemented geometry model covers up to "+formatFloat(maxDegrees)+
				" degrees either way; received "+formatFloat(q.si*180/math.Pi)+" degrees")
	}
}

func (a Airfoil) validate(rs *resultSet, field string) {
	if !a.supplied() {
		return
	}
	if a.Designation == "" {
		rs.add(field, IssueMissing, "name the section, or leave the airfoil unset")
	}
	if a.Evidence == "" {
		rs.add(field, IssueMissing,
			"state where the section identity and any data came from, so an assumed section is "+
				"visible alongside the geometry")
	}
	if a.ThicknessRatio < 0 || a.ThicknessRatio >= 1 {
		rs.add(field, IssueInvalid,
			"thickness ratio t/c must lie between 0 and 1, received "+formatFloat(a.ThicknessRatio))
	}
}

// resolvePlanes projects the solved planform into the plane it was not given in.
func (def WingDefinition) resolvePlanes(rs *resultSet, p Planform) (projected, panel SpanwiseDimensions) {
	mode := def.DihedralMode
	if mode == DihedralModeUnknown {
		if !def.Dihedral.IsZero() {
			rs.add("dihedral_mode", IssueMissing,
				"choose which dimensions stay fixed as dihedral changes: the built panel or the "+
					"plan-view reference")
			return projected, panel
		}
		// At zero dihedral the two planes coincide, so the choice cannot change
		// any result and is not demanded.
		mode = DihedralHoldProjected
	}
	given := SpanwiseDimensions{
		Span: p.Span, Area: p.Area, YMAC: p.YMAC, Half: rs.take(semiSpan(p.Span)),
	}
	switch mode {
	case DihedralHoldPanel:
		projected = SpanwiseDimensions{
			Span: rs.take(projectedSpan(p.Span, def.Dihedral)),
			Area: rs.take(projectedArea(p.Area, def.Dihedral)),
		}
		projected.YMAC = rs.take(macStation(projected.Span, p.TaperRatio))
		projected.Half = rs.take(semiSpan(projected.Span))
		return projected, given
	case DihedralHoldProjected:
		panel = SpanwiseDimensions{
			Span: rs.take(panelSpan(p.Span, def.Dihedral)),
			Area: rs.take(panelArea(p.Area, def.Dihedral)),
		}
		panel.YMAC = rs.take(macStation(panel.Span, p.TaperRatio))
		panel.Half = rs.take(semiSpan(panel.Span))
		return given, panel
	case DihedralModeUnknown:
		return projected, panel
	default:
		rs.add("dihedral_mode", IssueUnsupported, "unknown dihedral mode")
		return projected, panel
	}
}

// exposedArea reports the plan-view area outside the body, when a body width
// was supplied. Without one it returns the zero Quantity, which means the value
// is not reported rather than zero.
func (def WingDefinition) exposedArea(rs *resultSet, w Wing, p Planform) Quantity {
	if !def.BodyWidth.supplied() {
		return Quantity{}
	}
	return rs.take(exposedArea(w.Projected.Area, p.RootChord, p.TaperRatio,
		w.Projected.Span, def.BodyWidth))
}

func semiSpan(span Quantity) (Result, error) {
	e := newEvaluation(EqSemiSpan)
	b := e.quantity(portSpan.Name, span)
	return e.finish(b / 2)
}

func projectedSpan(panel, dihedral Quantity) (Result, error) {
	e := newEvaluation(EqProjectedSpan)
	b := e.quantity(portPanelSpan.Name, panel)
	g := e.signedQuantity(portDihedral.Name, dihedral)
	return e.finish(b * math.Cos(g))
}

func projectedArea(panel, dihedral Quantity) (Result, error) {
	e := newEvaluation(EqProjectedArea)
	s := e.quantity(portPanelArea.Name, panel)
	g := e.signedQuantity(portDihedral.Name, dihedral)
	return e.finish(s * math.Cos(g))
}

func panelSpan(projected, dihedral Quantity) (Result, error) {
	e := newEvaluation(EqPanelSpan)
	b := e.quantity(portSpan.Name, projected)
	g := e.signedQuantity(portDihedral.Name, dihedral)
	return e.finish(b / math.Cos(g))
}

func panelArea(projected, dihedral Quantity) (Result, error) {
	e := newEvaluation(EqPanelArea)
	s := e.quantity(portArea.Name, projected)
	g := e.signedQuantity(portDihedral.Name, dihedral)
	return e.finish(s / math.Cos(g))
}

// sweepAt converts a sweep angle from one chord fraction to another.
func sweepAt(sweep Quantity, reference, target, aspect, taper float64) (Result, error) {
	e := newEvaluation(EqSweepTransform)
	l := e.signedQuantity(portReferenceSweep.Name, sweep)
	m := e.scalarSigned(portReferenceFraction.Name, reference)
	n := e.scalarSigned(portTargetFraction.Name, target)
	a := e.scalar(portAspectRatio.Name, aspect)
	lam := e.scalar(portTaperRatio.Name, taper)
	if a == 0 {
		return e.finishSigned(0)
	}
	return e.finishSigned(math.Atan(math.Tan(l) - (4/a)*((n-m)*(1-lam)/(1+lam))))
}

func exposedArea(area, root Quantity, taper float64, span, bodyWidth Quantity) (Result, error) {
	e := newEvaluation(EqExposedArea)
	s := e.quantity(portArea.Name, area)
	c := e.quantity(portRootChord.Name, root)
	l := e.scalar(portTaperRatio.Name, taper)
	b := e.quantity(portSpan.Name, span)
	d := e.quantity(portBodyWidth.Name, bodyWidth)
	if b > 0 {
		e.requireWithin(portBodyWidth.Name, d, 0, b, portBodyWidth.Dimension.String(),
			"the body cannot be wider than the span "+formatFloat(b)+" m")
	}
	if b == 0 {
		return e.finish(0)
	}
	sideChord := c * (1 - (1-l)*d/b)
	return e.finish(s - d*(c+sideChord)/2)
}
