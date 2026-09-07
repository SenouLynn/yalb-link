package calculator

// The dimension and formula views.
//
// These are the geometry a worksheet draws and the explanation behind each
// dimension it draws. Nothing here computes a new number: every coordinate and
// every value comes from a solved Wing, so a drawing and the parameter table
// beside it cannot disagree, and a frontend that maps a click back to a field
// never has to know an equation.

// ViewKind names one orthographic view. The three are separate because the
// dimensions that mean something differ between them: a dihedral angle is
// invisible in the plan view, and a span is a foreshortened line in the side
// one.
type ViewKind uint8

const (
	// ViewUnknown is the zero value.
	ViewUnknown ViewKind = iota
	// ViewPlan looks down the z axis: x aft, y towards the right tip.
	ViewPlan
	// ViewFront looks along the x axis: y towards the right tip, z up.
	ViewFront
	// ViewSide looks along the y axis: x aft, z up.
	ViewSide
)

var viewKindNames = [...]string{
	ViewUnknown: "unknown",
	ViewPlan:    "plan view",
	ViewFront:   "front view",
	ViewSide:    "side view",
}

// String returns the view's readable name.
func (v ViewKind) String() string {
	if int(v) < len(viewKindNames) {
		return viewKindNames[v]
	}
	return "unknown"
}

// Axes names the two datum axes the view shows, in the order across then up.
func (v ViewKind) Axes() (across, up string) {
	switch v {
	case ViewPlan:
		return "y", "x"
	case ViewFront:
		return "y", "z"
	case ViewSide:
		return "x", "z"
	case ViewUnknown:
		return "", ""
	default:
		return "", ""
	}
}

// SketchRole says what a drawn curve is, so a worksheet can style an outline
// differently from a construction line without inferring it from the geometry.
type SketchRole uint8

const (
	// SketchRoleUnknown is the zero value.
	SketchRoleUnknown SketchRole = iota
	// SketchOutline is a real edge of the surface.
	SketchOutline
	// SketchCenterline is the symmetry plane.
	SketchCenterline
	// SketchAxis is a datum axis through the sketch origin.
	SketchAxis
	// SketchConstruction is a line that exists to be dimensioned against and is
	// not an edge: a sweep reference line, a projection of a panel.
	SketchConstruction
	// SketchReference marks a named geometric reference such as the mean
	// aerodynamic chord. It is geometry, not an aerodynamic claim.
	SketchReference
)

var sketchRoleNames = [...]string{
	SketchRoleUnknown:  "unknown",
	SketchOutline:      "outline",
	SketchCenterline:   "centerline",
	SketchAxis:         "axis",
	SketchConstruction: "construction",
	SketchReference:    "reference",
}

// String returns the role's readable name.
func (r SketchRole) String() string {
	if int(r) < len(sketchRoleNames) {
		return sketchRoleNames[r]
	}
	return "unknown"
}

// DimensionKind separates a length from an angle, because the two are drawn and
// labelled differently and carry different units.
type DimensionKind uint8

const (
	// DimensionKindUnknown is the zero value.
	DimensionKindUnknown DimensionKind = iota
	// DimensionLinear measures the distance between two points.
	DimensionLinear
	// DimensionAngular measures the angle of the line between two points.
	DimensionAngular
)

var dimensionKindNames = [...]string{
	DimensionKindUnknown: "unknown",
	DimensionLinear:      "linear",
	DimensionAngular:     "angular",
}

// String returns the kind's readable name.
func (k DimensionKind) String() string {
	if int(k) < len(dimensionKindNames) {
		return dimensionKindNames[k]
	}
	return "unknown"
}

// SketchCurve is one drawn line in a view.
type SketchCurve struct {
	// Label names the curve for a reader and for a screen reader.
	Label string
	// Points are its vertices in DatumWingRoot, in order.
	Points []Point
	// Role says what the curve is.
	Role SketchRole
	// Mirrored reports that the left panel is this curve's mirror in y. It is
	// stated rather than drawn twice, so a consumer cannot mistake the mirror
	// for a second, independently solved panel.
	Mirrored bool
	// Closed reports that the last point joins the first.
	Closed bool
}

// SketchDimension is one dimension on a view, tied to the parameter it measures.
//
// Key is what makes the drawing and the worksheet one thing rather than two:
// selecting a dimension selects its field, and selecting a field highlights its
// dimension, because both are the same stable parameter key.
type SketchDimension struct {
	// Key names the parameter this dimension measures.
	Key ParameterKey
	// Label is what the dimension is called on the drawing.
	Label string
	// Detail says what the dimension measures and, where the two differ, which
	// plane it is measured in.
	Detail string
	// From and To are the dimension's endpoints in DatumWingRoot.
	From Point
	// To is the dimension's second endpoint.
	To Point
	// Value is the dimensioned value.
	Value Quantity
	// Kind separates a length from an angle.
	Kind DimensionKind
	// Plane records whether the dimension is a plan-view projection or a
	// dimension of the panel as built. A builder cutting a panel needs the
	// second, and a doorway constrains the first.
	Plane OutlinePlane
}

// SketchView is one orthographic view: what to draw and what to dimension.
type SketchView struct {
	// Datum names the coordinate convention every point here uses.
	Datum string
	// Across names the datum axis that runs left to right in this view.
	Across string
	// Up names the datum axis that runs up the page in this view.
	Up string
	// Curves are the lines to draw.
	Curves []SketchCurve
	// Dimensions are the dimensions to annotate.
	Dimensions []SketchDimension
	// View names the view.
	View ViewKind
}

// Views returns the plan, front and side views of the solved wing, with their
// construction geometry and their dimensions.
//
// The origin, the axes and the symmetry plane are drawn rather than implied: a
// sketch that does not show what its dimensions are measured from is not a
// sketch anyone can rebuild from.
func (w Wing) Views() []SketchView {
	return []SketchView{w.planView(), w.frontView(), w.sideView()}
}

// length builds a length quantity, for the construction points a view needs
// that are not themselves reported parameters.
func length(v float64) Quantity { return Quantity{si: v, dim: DimLength} }

func point(x, y, z float64) Point {
	return Point{X: length(x), Y: length(y), Z: length(z)}
}

// newView starts a view with its datum and axes filled in.
func newView(kind ViewKind) SketchView {
	across, up := kind.Axes()
	return SketchView{View: kind, Datum: DatumWingRoot, Across: across, Up: up}
}

func (v *SketchView) curve(role SketchRole, label string, mirrored, closed bool, points ...Point) {
	v.Curves = append(v.Curves, SketchCurve{
		Role: role, Label: label, Mirrored: mirrored, Closed: closed, Points: points,
	})
}

func (v *SketchView) linear(key ParameterKey, label, detail string, plane OutlinePlane, value Quantity, from, to Point) {
	v.Dimensions = append(v.Dimensions, SketchDimension{
		Key: key, Label: label, Detail: detail, Kind: DimensionLinear,
		Plane: plane, Value: value, From: from, To: to,
	})
}

func (v *SketchView) angular(key ParameterKey, label, detail string, value Quantity, from, to Point) {
	v.Dimensions = append(v.Dimensions, SketchDimension{
		Key: key, Label: label, Detail: detail, Kind: DimensionAngular,
		Plane: OutlinePlanView, Value: value, From: from, To: to,
	})
}

// planView draws the projected planform: the outline a doorway constrains and
// the reference area is measured on.
func (w Wing) planView() SketchView {
	view := newView(ViewPlan)
	semi := w.Projected.Half.si
	root := w.Planform.RootChord.si
	tip := w.Planform.TipChord.si
	tipLE := w.TipLeadingEdgeOffset.si
	yMAC := w.Projected.YMAC.si
	macLE := w.MACLeadingEdgeStation.si
	mac := w.Planform.MAC.si
	fraction := w.Definition.SweepReference

	view.curve(SketchOutline, "right panel, plan view", true, true,
		point(0, 0, 0), point(tipLE, semi, 0), point(tipLE+tip, semi, 0), point(root, 0, 0))
	view.curve(SketchCenterline, "centerline and symmetry plane, y = 0", false, false,
		point(0, 0, 0), point(root, 0, 0))
	view.curve(SketchAxis, "y axis through the sketch origin, positive to the right tip", false, false,
		point(0, -semi, 0), point(0, semi, 0))
	view.curve(SketchConstruction, "sweep reference line at chord fraction "+formatFloat(fraction),
		true, false, point(fraction*root, 0, 0), point(tipLE+fraction*tip, semi, 0))
	view.curve(SketchReference, "mean aerodynamic chord, a geometric reference", false, false,
		point(macLE, yMAC, 0), point(macLE+mac, yMAC, 0))

	view.linear(ParamSpanProjected, "span b",
		"tip to tip, projected onto the plan view", OutlinePlanView, w.Projected.Span,
		point(0, -semi, 0), point(0, semi, 0))
	view.linear(ParamSemiSpanProjected, "half span b/2",
		"centerline to the right tip, projected", OutlinePlanView, w.Projected.Half,
		point(0, 0, 0), point(0, semi, 0))
	view.linear(ParamChordRoot, "root chord c_root",
		"the centerline chord of the reference trapezoid", OutlinePlanView, w.Planform.RootChord,
		point(0, 0, 0), point(root, 0, 0))
	view.linear(ParamChordTip, "tip chord c_tip",
		"the chord at the tip", OutlinePlanView, w.Planform.TipChord,
		point(tipLE, semi, 0), point(tipLE+tip, semi, 0))
	view.linear(ParamTipLEOffset, "tip leading-edge offset",
		"how far aft of the root leading edge the tip leading edge sits", OutlinePlanView,
		w.TipLeadingEdgeOffset, point(0, semi, 0), point(tipLE, semi, 0))
	view.linear(ParamStationMAC, "MAC station y_mac",
		"the spanwise station the mean aerodynamic chord sits at", OutlinePlanView, w.Projected.YMAC,
		point(0, 0, 0), point(0, yMAC, 0))
	view.linear(ParamStationMACLeading, "MAC leading edge x_le_mac",
		"how far aft of the root leading edge the MAC's own leading edge sits", OutlinePlanView,
		w.MACLeadingEdgeStation, point(0, yMAC, 0), point(macLE, yMAC, 0))
	view.linear(ParamChordMAC, "mean aerodynamic chord",
		"a geometric reference length; it is not a centre of lift", OutlinePlanView, w.Planform.MAC,
		point(macLE, yMAC, 0), point(macLE+mac, yMAC, 0))
	view.angular(ParamSweepLeadingEdge, "leading-edge sweep",
		"the plan-view angle of the leading edge, positive aft", w.LeadingEdgeSweep,
		point(0, 0, 0), point(tipLE, semi, 0))
	if w.Definition.BodyWidth.supplied() {
		half := w.Definition.BodyWidth.si / 2
		view.curve(SketchConstruction, "body sides at the wing", false, false,
			point(0, -half, 0), point(0, half, 0))
		view.linear(ParamBodyWidth, "body width at the wing",
			"the plan-view width of the body the wing passes through", OutlinePlanView,
			w.Definition.BodyWidth, point(0, -half, 0), point(0, half, 0))
	}
	return view
}

// frontView is where the two planes stop coinciding. It draws the panel as
// built against its own plan-view projection, so a builder can see that a
// panel-plane half span and a projected one are different lengths.
func (w Wing) frontView() SketchView {
	view := newView(ViewFront)
	semi := w.Projected.Half.si
	rise := w.TipRise.si

	view.curve(SketchOutline, "right panel as built, seen from the front", true, false,
		point(0, 0, 0), point(0, semi, rise))
	view.curve(SketchConstruction, "the same panel projected onto the plan view", true, false,
		point(0, 0, 0), point(0, semi, 0))
	view.curve(SketchAxis, "z axis through the sketch origin, positive up", false, false,
		point(0, 0, 0), point(0, 0, rise))

	view.linear(ParamSemiSpanProjected, "projected half span",
		"the plan-view length a doorway or a contest rule constrains", OutlinePlanView,
		w.Projected.Half, point(0, 0, 0), point(0, semi, 0))
	view.linear(ParamSemiSpanPanel, "panel half span",
		"the construction length of the panel as built, which is what a rib jig is cut to",
		OutlinePanelSurface, w.Panel.Half, point(0, 0, 0), point(0, semi, rise))
	view.linear(ParamTipRise, "tip rise",
		"how far the tip sits above the root chord line", OutlinePanelSurface, w.TipRise,
		point(0, semi, 0), point(0, semi, rise))
	view.angular(ParamDihedral, "dihedral",
		"uniform along the panel, applied about the root chord line", w.Definition.Dihedral,
		point(0, 0, 0), point(0, semi, rise))
	return view
}

// sideView carries the angles a plan view cannot show.
func (w Wing) sideView() SketchView {
	view := newView(ViewSide)
	root := w.Planform.RootChord.si
	tip := w.Planform.TipChord.si
	tipLE := w.TipLeadingEdgeOffset.si
	semi := w.Projected.Half.si
	rise := w.TipRise.si

	view.curve(SketchOutline, "root chord line", false, false,
		point(0, 0, 0), point(root, 0, 0))
	view.curve(SketchConstruction, "tip chord line, at the tip station", true, false,
		point(tipLE, semi, rise), point(tipLE+tip, semi, rise))
	view.curve(SketchAxis, "x axis through the sketch origin, positive aft", false, false,
		point(0, 0, 0), point(root, 0, 0))

	view.linear(ParamChordRoot, "root chord c_root",
		"the centerline chord of the reference trapezoid", OutlinePlanView, w.Planform.RootChord,
		point(0, 0, 0), point(root, 0, 0))
	view.linear(ParamTipLEOffset, "tip leading-edge offset",
		"how far aft of the root leading edge the tip leading edge sits", OutlinePlanView,
		w.TipLeadingEdgeOffset, point(0, 0, 0), point(tipLE, 0, 0))
	view.angular(ParamIncidence, "root incidence",
		"the root chord against the fuselage reference line", w.Definition.Incidence,
		point(0, 0, 0), point(root, 0, 0))
	view.angular(ParamTwist, "geometric twist",
		"root to tip; washout is negative", w.Definition.Twist,
		point(tipLE, semi, rise), point(tipLE+tip, semi, rise))
	return view
}

// Explanation is why one parameter holds the value it does: the relationship,
// the revision that produced it, the values actually substituted and the
// result. It is what a builder sees when they select a dimension or a field.
type Explanation struct {
	// Key names the parameter.
	Key ParameterKey
	// EquationID identifies the relationship, for a derived value.
	EquationID string
	// Revision is that equation's revision at the time of the solve.
	Revision string
	// Expression is the symbolic relationship, for example c_root = 2S/(b(1+lambda)).
	Expression string
	// Detail says in words where the value came from.
	Detail string
	// Substitutions are the values actually used, in the order the equation
	// consumed them. It is empty for a driver: nothing was substituted.
	Substitutions []Substitution
	// DependsOn lists the parameters the value was computed from.
	DependsOn []ParameterKey
	// Value is the parameter's value.
	Value Quantity
	// Unit is the unit Value is expressed in.
	Unit Unit
	// Role separates a value the builder chose from one this package computed.
	Role ParameterRole
}

// Explain returns the relationship behind one parameter. It reports false for a
// key the solved wing does not define.
func (w Wing) Explain(key ParameterKey) (Explanation, bool) {
	parameters := w.Parameters()
	parameter, ok := parameters.Get(key)
	if !ok {
		return Explanation{}, false
	}
	return w.explainParameter(parameter), true
}

// Explanations returns the relationship behind every parameter, in parameter
// order, so a worksheet can build its whole formula view from one call.
func (w Wing) Explanations() []Explanation {
	parameters := w.Parameters()
	out := make([]Explanation, 0, len(parameters))
	for _, parameter := range parameters {
		out = append(out, w.explainParameter(parameter))
	}
	return out
}

func (w Wing) explainParameter(parameter Parameter) Explanation {
	explanation := Explanation{
		Key:        parameter.Key,
		Role:       parameter.Role,
		Value:      parameter.Value,
		Unit:       parameter.Unit,
		EquationID: parameter.EquationID,
		Revision:   parameter.Revision,
		DependsOn:  append([]ParameterKey(nil), parameter.DependsOn...),
	}
	if parameter.Role == RoleDriver {
		explanation.Detail = "entered by the builder; nothing was substituted to produce it"
		return explanation
	}
	if equation, err := Lookup(parameter.EquationID); err == nil {
		explanation.Expression = equation.Expression
	}
	if trace, ok := w.traceFor(parameter); ok {
		explanation.Substitutions = append([]Substitution(nil), trace.Substitutions...)
		explanation.Expression = trace.Expression
		explanation.Revision = trace.Revision
	}
	explanation.Detail = "computed by " + parameter.EquationID
	if len(explanation.Substitutions) == 0 {
		explanation.Detail += "; the evaluation's substitutions were not recorded for this value"
	}
	return explanation
}

// traceFor finds the evaluation that produced a parameter. A wing evaluates
// some relationships more than once — a semi-span in each plane, an aspect ratio
// in each — so the equation ID alone does not identify one; the result value
// picks the right evaluation out of them. Where two evaluations of one equation
// agree, the planes coincide and either trace describes the value correctly.
//
// The match is exact rather than nearest. A trace whose result is not this
// parameter's value is some other evaluation's, and showing its substitutions
// under this parameter's name would be a wrong explanation rather than a
// partial one; the explanation says so instead.
func (w Wing) traceFor(parameter Parameter) (Trace, bool) {
	for n := range w.Traces {
		trace := w.Traces[n]
		if trace.EquationID == parameter.EquationID && trace.Result == parameter.Value {
			return trace, true
		}
	}
	return Trace{}, false
}
