package calculator

import "math"

// PlanformShape names the supported plan-view shapes. A shape is part of the
// design definition rather than something inferred from which fields happen to
// be filled in: a rectangle and a trapezoid have different independent values.
type PlanformShape uint8

const (
	// ShapeUnknown is the zero value and is never solved.
	ShapeUnknown PlanformShape = iota
	// ShapeRectangle is a constant-chord wing, the taper ratio 1 limit of the
	// trapezoid.
	ShapeRectangle
	// ShapeTrapezoid is a straight-tapered wing, symmetric about the centerline.
	ShapeTrapezoid
)

var planformShapeNames = [...]string{
	ShapeUnknown:   "unknown",
	ShapeRectangle: "rectangle",
	ShapeTrapezoid: "symmetric trapezoid",
}

// String returns the shape's readable name.
func (s PlanformShape) String() string {
	if int(s) < len(planformShapeNames) {
		return planformShapeNames[s]
	}
	return "unknown"
}

// SolveMode names the driver pair a planform was solved from. There is no
// "any three fields editable" mode: a planform has exactly two independent size
// values, plus the taper ratio for a trapezoid, and each supported pair is a
// named path with its own equations.
type SolveMode uint8

const (
	// SolveModeUnknown is the zero value, reported when no supported pair matched.
	SolveModeUnknown SolveMode = iota
	// SolveFromSpanAndArea derives aspect ratio and the chords.
	SolveFromSpanAndArea
	// SolveFromSpanAndAspectRatio derives area and the chords.
	SolveFromSpanAndAspectRatio
	// SolveFromAreaAndAspectRatio is the chapter's planform-sizing path, b = sqrt(A S).
	SolveFromAreaAndAspectRatio
	// SolveFromSpanAndRootChord derives area from the taper relation.
	SolveFromSpanAndRootChord
	// SolveFromAreaAndRootChord derives span from the taper relation.
	SolveFromAreaAndRootChord
	// SolveFromAspectRatioAndRootChord derives span, then area.
	SolveFromAspectRatioAndRootChord
)

var solveModeNames = [...]string{
	SolveModeUnknown:                 "unknown",
	SolveFromSpanAndArea:             "span and area",
	SolveFromSpanAndAspectRatio:      "span and aspect ratio",
	SolveFromAreaAndAspectRatio:      "area and aspect ratio",
	SolveFromSpanAndRootChord:        "span and root chord",
	SolveFromAreaAndRootChord:        "area and root chord",
	SolveFromAspectRatioAndRootChord: "aspect ratio and root chord",
}

// String names the driver pair, for example "area and aspect ratio".
func (m SolveMode) String() string {
	if int(m) < len(solveModeNames) {
		return solveModeNames[m]
	}
	return "unknown"
}

// driver bits identify which size values a caller supplied. Exactly two must be
// present; the pair selects the solve mode.
const (
	driverSpan = 1 << iota
	driverArea
	driverAspectRatio
	driverRootChord
)

var solveModeForDrivers = map[int]SolveMode{
	driverSpan | driverArea:             SolveFromSpanAndArea,
	driverSpan | driverAspectRatio:      SolveFromSpanAndAspectRatio,
	driverArea | driverAspectRatio:      SolveFromAreaAndAspectRatio,
	driverSpan | driverRootChord:        SolveFromSpanAndRootChord,
	driverArea | driverRootChord:        SolveFromAreaAndRootChord,
	driverAspectRatio | driverRootChord: SolveFromAspectRatioAndRootChord,
}

// PlanformDrivers is what a builder actually chose. Exactly two of Span, Area,
// AspectRatio and RootChord are drivers; everything else is derived from them.
// Supplying a third is reported rather than silently preferred, because
// choosing which value is authoritative is the builder's decision.
//
// The zero value of each field means "not supplied". A supplied zero is a
// different answer and is rejected as invalid rather than treated as absent.
type PlanformDrivers struct {
	// Span is the projected, tip-to-tip span b of the reference planform.
	Span Quantity
	// Area is the reference area S, the trapezoid carried through the centerline.
	Area Quantity
	// RootChord is the centerline chord of the reference trapezoid. For a
	// rectangle it is the constant chord.
	RootChord Quantity
	// AspectRatio is A = b^2/S.
	AspectRatio float64
	// TaperRatio is lambda = c_tip/c_root. It is required for a trapezoid, where
	// reverse taper (lambda > 1) is supported. For a rectangle it must be 1 or
	// left unset.
	TaperRatio float64
	// Shape selects the supported plan-view shape.
	Shape PlanformShape
}

// Planform is a solved plan-view geometry. Every value is a plan-view
// projection of the reference trapezoid; panel construction lengths come from
// the wing, which is where dihedral lives.
type Planform struct {
	// Traces record every relationship evaluated, in evaluation order.
	Traces []Trace
	// Span is the projected tip-to-tip span.
	Span Quantity
	// Area is the reference area, including the part inside any body.
	Area Quantity
	// RootChord is the centerline chord of the reference trapezoid.
	RootChord Quantity
	// TipChord is the chord at the tip.
	TipChord Quantity
	// MeanChord is the geometric mean chord S/b. Under taper it is not the MAC.
	MeanChord Quantity
	// MAC is the mean aerodynamic chord.
	MAC Quantity
	// YMAC is the spanwise station of the MAC, from the centerline.
	YMAC Quantity
	// AspectRatio is A = b^2/S.
	AspectRatio float64
	// TaperRatio is lambda = c_tip/c_root.
	TaperRatio float64
	// Shape and Mode record what was solved and from which drivers.
	Shape PlanformShape
	// Mode names the driver pair this planform was solved from.
	Mode SolveMode
}

// resultSet accumulates the traces and issues of a multi-equation solve, so a
// caller receives every problem in one pass rather than only the first.
type resultSet struct {
	traces []Trace
	issues Issues
}

// take records a sub-evaluation. A failed evaluation contributes its issues and
// yields the zero Quantity, which downstream evaluations report as missing;
// the accumulated issues suppress the whole result at the end.
func (rs *resultSet) take(r Result, err error) Quantity {
	if err != nil {
		if issues, ok := AsIssues(err); ok {
			rs.issues = append(rs.issues, issues...)
		} else {
			rs.issues = append(rs.issues, Issue{Field: "planform", Kind: IssueInvalid, Detail: err.Error()})
		}
		return Quantity{}
	}
	rs.traces = append(rs.traces, r.Trace)
	return r.Value
}

func (rs *resultSet) add(field string, kind IssueKind, detail string) {
	rs.issues = append(rs.issues, Issue{Field: field, Kind: kind, Detail: detail})
}

// SolvePlanform solves the plan-view geometry from exactly two size drivers,
// plus the taper ratio for a trapezoid. It reports every issue found in a stage
// of the solve rather than only the first, and stops before deriving anything
// from a value that was already rejected, so a rejected span is not also
// reported as a missing one. No partial planform is returned when any issue was
// found.
func SolvePlanform(d PlanformDrivers) (Planform, error) {
	rs := &resultSet{}
	taper := d.resolveTaper(rs)
	mode := d.resolveMode(rs, taper)
	if len(rs.issues) > 0 {
		return Planform{}, rs.issues
	}
	span, area := d.solveSize(rs, mode, taper)
	if len(rs.issues) > 0 {
		return Planform{}, rs.issues
	}
	p := Planform{
		Shape:      d.Shape,
		Mode:       mode,
		Span:       span,
		Area:       area,
		TaperRatio: taper,
	}
	p.completeFrom(d, rs, taper)
	if len(rs.issues) > 0 {
		return Planform{}, rs.issues
	}
	p.Traces = rs.traces
	return p, nil
}

// resolveTaper returns the taper ratio implied by the shape and the drivers.
func (d PlanformDrivers) resolveTaper(rs *resultSet) float64 {
	switch d.Shape {
	case ShapeRectangle:
		if d.TaperRatio != 0 && d.TaperRatio != 1 {
			rs.add("taper_ratio", IssueInvalid,
				"a rectangle has taper ratio 1; choose the trapezoid shape to taper the wing")
			return 0
		}
		return 1
	case ShapeTrapezoid:
		return resolveTrapezoidTaper(d.TaperRatio, rs)
	case ShapeUnknown:
		rs.add("shape", IssueMissing,
			"choose a planform shape: a rectangle and a trapezoid do not have the same independent values")
		return 0
	default:
		rs.add("shape", IssueUnsupported,
			"only a rectangle and a symmetric trapezoid are implemented")
		return 0
	}
}

func resolveTrapezoidTaper(taper float64, rs *resultSet) float64 {
	switch {
	case taper == 0:
		rs.add("taper_ratio", IssueMissing,
			"a trapezoid needs a taper ratio lambda = c_tip/c_root; a pointed tip (lambda = 0) "+
				"is not supported by this model")
		return 0
	case !isFinite(taper) || taper < 0:
		rs.add("taper_ratio", IssueInvalid,
			"taper ratio must be a positive finite number, received "+formatFloat(taper))
		return 0
	default:
		return taper
	}
}

// suppliedDrivers reports which size drivers were filled in.
func (d PlanformDrivers) suppliedDrivers() (mask int, names []string) {
	for _, candidate := range []struct {
		name string
		bit  int
		on   bool
	}{
		{portSpan.Name, driverSpan, d.Span.supplied()},
		{portArea.Name, driverArea, d.Area.supplied()},
		{portAspectRatio.Name, driverAspectRatio, d.AspectRatio != 0},
		{portRootChord.Name, driverRootChord, d.RootChord.supplied()},
	} {
		if candidate.on {
			mask |= candidate.bit
			names = append(names, candidate.name)
		}
	}
	return mask, names
}

// resolveMode selects the named solve path, or explains why the driver set is
// not one. An over-determined set is reported as redundant when the extra
// values agree and as conflicting when they do not, so the two are never
// confused with each other.
func (d PlanformDrivers) resolveMode(rs *resultSet, taper float64) SolveMode {
	mask, names := d.suppliedDrivers()
	if len(names) == 2 {
		mode, ok := solveModeForDrivers[mask]
		if !ok {
			rs.add("drivers", IssueUnsupported,
				"no solve path is implemented for "+joinNames(names))
			return SolveModeUnknown
		}
		return mode
	}
	if len(names) < 2 {
		rs.add("drivers", IssueMissing,
			"a planform needs exactly two of span, wing_area, aspect_ratio and root_chord as "+
				"drivers; supplied "+joinNames(names))
		return SolveModeUnknown
	}
	d.reportOverDetermined(rs, names, taper)
	return SolveModeUnknown
}

func joinNames(names []string) string {
	if len(names) == 0 {
		return "none"
	}
	out := names[0]
	for _, n := range names[1:] {
		out += ", " + n
	}
	return out
}

// driverConsistency is the relative agreement required before an
// over-determined driver set is called redundant rather than conflicting. It is
// loose enough to accept values a builder typed back from a rounded display and
// tight enough that a real disagreement is never called redundant.
const driverConsistency = 1e-6

// reportOverDetermined distinguishes redundant drivers from conflicting ones.
// It solves from the first supported pair and compares the rest against it.
// Which pair that is has to be arbitrary — with three mutually inconsistent
// values nothing in the numbers says which one the builder meant — so a
// conflict is reported once against the whole driver set, naming the reference
// pair it was measured against, rather than blaming one field the solver had no
// way of identifying.
func (d PlanformDrivers) reportOverDetermined(rs *resultSet, names []string, taper float64) {
	span, area, pair, ok := d.independentPair(taper)
	if !ok {
		rs.add("drivers", IssueUnsupported, "over-determined: "+joinNames(names)+" were all supplied")
		return
	}
	conflicts := d.conflicts(span, area, taper)
	if len(conflicts) == 0 {
		rs.add("drivers", IssueUnsupported,
			"over-determined but consistent: "+joinNames(names)+" all agree, so nothing is wrong "+
				"with the numbers; choose exactly two as drivers and let the rest be derived")
		return
	}
	detail := "the supplied drivers cannot all be true; taking " + pair + " as the reference pair"
	for _, c := range conflicts {
		detail += ", " + c
	}
	rs.add("drivers", IssueInvalid, detail)
}

// independentPair recovers span and area from whichever supported pair of
// drivers is present, without evaluating or tracing: it exists only to test the
// remaining drivers for agreement.
func (d PlanformDrivers) independentPair(taper float64) (span, area float64, pair string, ok bool) {
	switch {
	case d.Span.supplied() && d.Area.supplied():
		return d.Span.si, d.Area.si, "span and wing_area", true
	case d.Span.supplied() && d.AspectRatio != 0:
		return d.Span.si, d.Span.si * d.Span.si / d.AspectRatio, "span and aspect_ratio", true
	case d.Area.supplied() && d.AspectRatio != 0:
		return math.Sqrt(d.AspectRatio * d.Area.si), d.Area.si, "wing_area and aspect_ratio", true
	case d.Span.supplied() && d.RootChord.supplied():
		return d.Span.si, d.Span.si * d.RootChord.si * (1 + taper) / 2, "span and root_chord", true
	case d.Area.supplied() && d.RootChord.supplied():
		return 2 * d.Area.si / (d.RootChord.si * (1 + taper)), d.Area.si, "wing_area and root_chord", true
	default:
		return 0, 0, "", false
	}
}

// conflicts lists the supplied drivers that disagree with the span and area
// recovered from the reference pair.
func (d PlanformDrivers) conflicts(span, area, taper float64) []string {
	var found []string
	check := func(field string, supplied, implied float64, on bool) {
		if !on || agrees(supplied, implied) {
			return
		}
		found = append(found, field+" was supplied as "+formatFloat(supplied)+
			" but is implied to be "+formatFloat(implied))
	}
	check(portSpan.Name, d.Span.si, span, d.Span.supplied())
	check(portArea.Name, d.Area.si, area, d.Area.supplied())
	check(portAspectRatio.Name, d.AspectRatio, span*span/area, d.AspectRatio != 0)
	check(portRootChord.Name, d.RootChord.si, 2*area/(span*(1+taper)), d.RootChord.supplied())
	return found
}

func agrees(got, want float64) bool {
	if want == 0 {
		return got == 0
	}
	return math.Abs(got-want) <= driverConsistency*math.Abs(want)
}

// solveSize evaluates the named path down to span and area, recording a trace
// for each relationship it uses.
func (d PlanformDrivers) solveSize(rs *resultSet, mode SolveMode, taper float64) (span, area Quantity) {
	aspect := ratio(d.AspectRatio)
	switch mode {
	case SolveFromSpanAndArea:
		return d.Span, d.Area
	case SolveFromSpanAndAspectRatio:
		return d.Span, rs.take(areaFromSpanAndAspect(d.Span, aspect))
	case SolveFromAreaAndAspectRatio:
		return rs.take(spanFromAreaAndAspect(d.Area, aspect)), d.Area
	case SolveFromSpanAndRootChord:
		return d.Span, rs.take(areaFromSpanAndRootChord(d.Span, d.RootChord, taper))
	case SolveFromAreaAndRootChord:
		return rs.take(spanFromAreaAndRootChord(d.Area, d.RootChord, taper)), d.Area
	case SolveFromAspectRatioAndRootChord:
		b := rs.take(spanFromAspectAndRootChord(aspect, d.RootChord, taper))
		return b, rs.take(areaFromSpanAndAspect(b, aspect))
	case SolveModeUnknown:
		return Quantity{}, Quantity{}
	default:
		return Quantity{}, Quantity{}
	}
}

// completeFrom derives every remaining reference dimension. A value that is a
// driver keeps the builder's number; the rest are evaluated and traced.
func (p *Planform) completeFrom(d PlanformDrivers, rs *resultSet, taper float64) {
	if d.AspectRatio != 0 {
		p.AspectRatio = d.AspectRatio
	} else {
		p.AspectRatio = rs.take(aspectRatio(p.Span, p.Area)).si
	}
	if d.RootChord.supplied() {
		p.RootChord = d.RootChord
	} else {
		p.RootChord = rs.take(rootChord(p.Area, p.Span, taper))
	}
	if len(rs.issues) > 0 {
		return
	}
	p.TipChord = rs.take(tipChord(p.RootChord, taper))
	p.MeanChord = rs.take(meanChord(p.Area, p.Span))
	p.MAC = rs.take(meanAerodynamicChord(p.RootChord, taper))
	p.YMAC = rs.take(macStation(p.Span, taper))
}

// ratio wraps a dimensionless driver so it can be handed to an evaluation port.
func ratio(v float64) Quantity { return Quantity{si: v, dim: Dimensionless} }

func aspectRatio(span, area Quantity) (Result, error) {
	e := newEvaluation(EqAspectRatio)
	b := e.quantity(portSpan.Name, span)
	s := e.quantity(portArea.Name, area)
	return e.finish(b * b / s)
}

func spanFromAreaAndAspect(area, aspect Quantity) (Result, error) {
	e := newEvaluation(EqSpanFromAreaAndAspect)
	s := e.quantity(portArea.Name, area)
	a := e.scalar(portAspectRatio.Name, aspect.si)
	return e.finish(math.Sqrt(a * s))
}

func areaFromSpanAndAspect(span, aspect Quantity) (Result, error) {
	e := newEvaluation(EqAreaFromSpanAndAspect)
	b := e.quantity(portSpan.Name, span)
	a := e.scalar(portAspectRatio.Name, aspect.si)
	return e.finish(b * b / a)
}

func spanFromAreaAndRootChord(area, root Quantity, taper float64) (Result, error) {
	e := newEvaluation(EqSpanFromAreaAndRootChord)
	s := e.quantity(portArea.Name, area)
	c := e.quantity(portRootChord.Name, root)
	l := e.scalar(portTaperRatio.Name, taper)
	return e.finish(2 * s / (c * (1 + l)))
}

func areaFromSpanAndRootChord(span, root Quantity, taper float64) (Result, error) {
	e := newEvaluation(EqAreaFromSpanAndRootChord)
	b := e.quantity(portSpan.Name, span)
	c := e.quantity(portRootChord.Name, root)
	l := e.scalar(portTaperRatio.Name, taper)
	return e.finish(b * c * (1 + l) / 2)
}

func spanFromAspectAndRootChord(aspect, root Quantity, taper float64) (Result, error) {
	e := newEvaluation(EqSpanFromAspectAndRootChord)
	a := e.scalar(portAspectRatio.Name, aspect.si)
	c := e.quantity(portRootChord.Name, root)
	l := e.scalar(portTaperRatio.Name, taper)
	return e.finish(a * c * (1 + l) / 2)
}

func rootChord(area, span Quantity, taper float64) (Result, error) {
	e := newEvaluation(EqRootChord)
	s := e.quantity(portArea.Name, area)
	b := e.quantity(portSpan.Name, span)
	l := e.scalar(portTaperRatio.Name, taper)
	return e.finish(2 * s / (b * (1 + l)))
}

func tipChord(root Quantity, taper float64) (Result, error) {
	e := newEvaluation(EqTipChord)
	c := e.quantity(portRootChord.Name, root)
	l := e.scalar(portTaperRatio.Name, taper)
	return e.finish(l * c)
}

func meanChord(area, span Quantity) (Result, error) {
	e := newEvaluation(EqMeanChord)
	s := e.quantity(portArea.Name, area)
	b := e.quantity(portSpan.Name, span)
	return e.finish(s / b)
}

func meanAerodynamicChord(root Quantity, taper float64) (Result, error) {
	e := newEvaluation(EqMeanAerodynamicChord)
	c := e.quantity(portRootChord.Name, root)
	l := e.scalar(portTaperRatio.Name, taper)
	return e.finish((2.0 / 3.0) * c * (1 + l + l*l) / (1 + l))
}

func macStation(span Quantity, taper float64) (Result, error) {
	e := newEvaluation(EqMACStation)
	b := e.quantity(portSpan.Name, span)
	l := e.scalar(portTaperRatio.Name, taper)
	return e.finish(b * (1 + 2*l) / (6 * (1 + l)))
}

// ChordAt returns the chord at a spanwise station measured from the centerline,
// with its trace. The station must lie within the semi-span.
func (p Planform) ChordAt(station Quantity) (Result, error) {
	e := newEvaluation(EqChordAtStation)
	c := e.quantity(portRootChord.Name, p.RootChord)
	l := e.scalar(portTaperRatio.Name, p.TaperRatio)
	b := e.quantity(portSpan.Name, p.Span)
	y := e.signedQuantity(portStation.Name, station)
	if b > 0 {
		e.requireWithin(portStation.Name, y, 0, b/2, portStation.Dimension.String(),
			"the station must lie between the centerline and the semi-span "+formatFloat(b/2)+" m")
	}
	return e.finish(c * (1 - (1-l)*2*y/b))
}

// SemiSpan returns half the projected span, the plan-view length of one panel.
func (p Planform) SemiSpan() Quantity {
	return Quantity{si: p.Span.si / 2, dim: DimLength}
}
