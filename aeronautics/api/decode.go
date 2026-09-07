package api

import (
	"math"
	"strconv"

	"yalb.aero/calculator"
)

// decoder turns wire values into core values, accumulating field issues rather
// than stopping at the first, so one bad request produces one complete answer
// about what is wrong with it.
type decoder struct {
	issues []Issue
}

func (d *decoder) add(field, kind, detail string) {
	d.issues = append(d.issues, Issue{Field: field, Kind: kind, Detail: detail})
}

func (d *decoder) take(issue *Issue) {
	if issue != nil {
		d.issues = append(d.issues, *issue)
	}
}

// quantity converts a wire quantity. An absent one yields the zero core
// Quantity, which every core field reads as "not supplied".
func (d *decoder) quantity(field string, q *Quantity) calculator.Quantity {
	if q == nil {
		return calculator.Quantity{}
	}
	unit, err := calculator.ParseUnit(q.Unit)
	if err != nil {
		d.add(field, "invalid",
			strconv.Quote(q.Unit)+" is not a supported unit symbol; the discovery document lists them")
		return calculator.Quantity{}
	}
	value, err := calculator.NewQuantity(q.Value, unit)
	if err != nil {
		d.add(field, "invalid", err.Error())
		return calculator.Quantity{}
	}
	return value
}

// number rejects a non-finite scalar before it reaches the core, which is the
// one thing JSON can carry that the core's own checks would otherwise have to
// re-report per equation.
func (d *decoder) number(field string, v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		d.add(field, "invalid", "must be a finite number")
		return 0
	}
	return v
}

// enumOrEmpty resolves an optional enum: an empty token leaves the core's zero
// value in place, which the core itself then reports as missing where it
// matters. This is what lets an unstated dihedral mode reach the core and be
// accepted at zero dihedral, where the two planes coincide.
func enumOrEmpty[T comparable](d *decoder, table enumTable[T], field, token string) T {
	var zero T
	if token == "" {
		return zero
	}
	value, issue := table.parse(field, token)
	d.take(issue)
	return value
}

// design converts a wire design.
func (d *decoder) design(design Design) calculator.Design {
	out := calculator.Design{
		Name:          design.Name,
		MassBasis:     design.MassBasis,
		Mass:          d.quantity("design.mass", design.Mass),
		Configuration: enumOrEmpty(d, configurations, "design.configuration", design.Configuration),
		MassMode:      enumOrEmpty(d, massModes, "design.massMode", design.MassMode),
		Wing:          d.wing(design.Wing),
	}
	if design.Tail != nil {
		out.Tail = d.tail(*design.Tail)
	}
	if len(design.Cases) > MaxCases {
		d.add("design.cases", "unsupported",
			"a design may carry at most "+strconv.Itoa(MaxCases)+" flight cases, received "+
				strconv.Itoa(len(design.Cases)))
		return out
	}
	if len(design.Requirements) > MaxRequirements {
		d.add("design.requirements", "unsupported",
			"a design may carry at most "+strconv.Itoa(MaxRequirements)+" requirements, received "+
				strconv.Itoa(len(design.Requirements)))
		return out
	}
	for n := range design.Cases {
		out.Cases = append(out.Cases, d.designCase("design.cases["+strconv.Itoa(n)+"]", design.Cases[n]))
	}
	for n := range design.Requirements {
		out.Requirements = append(out.Requirements,
			d.requirement("design.requirements["+strconv.Itoa(n)+"]", design.Requirements[n]))
	}
	if len(design.Components) > MaxComponents {
		d.add("design.components", "unsupported",
			"a design may carry at most "+strconv.Itoa(MaxComponents)+" components, received "+
				strconv.Itoa(len(design.Components)))
		return out
	}
	for n := range design.Components {
		out.Components = append(out.Components,
			d.component("design.components["+strconv.Itoa(n)+"]", design.Components[n]))
	}
	return out
}

// component converts one mass item. A coordinate that is absent stays absent:
// the core reports an unplaced component as unplaced rather than assuming an
// origin for it.
func (d *decoder) component(field string, c Component) calculator.MassItem {
	return calculator.MassItem{
		Name:     c.Name,
		Basis:    c.Basis,
		Role:     enumOrEmpty(d, componentRoles, field+".role", c.Role),
		Mass:     d.quantity(field+".mass", c.Mass),
		Position: d.position(field+".position", &c.Position),
	}
}

func (d *decoder) position(field string, p *Position) calculator.Point {
	if p == nil {
		return calculator.Point{}
	}
	return calculator.Point{
		X: d.quantity(field+".x", p.X),
		Y: d.quantity(field+".y", p.Y),
		Z: d.quantity(field+".z", p.Z),
	}
}

// sweepSettings converts a sweep request's settings. Whether the design holds
// the named driver, and whether the range and the sample count are usable, are
// the core's answers: this reads the shape and hands the rest over.
func (d *decoder) sweepSettings(field string, s SweepSettings) calculator.SweepSettings {
	driver, issue := parseSweepDriverKey(field+".driver", s.Driver)
	d.take(issue)
	return calculator.SweepSettings{
		Driver:  driver,
		From:    d.quantity(field+".from", &s.From),
		To:      d.quantity(field+".to", &s.To),
		Samples: s.Samples,
		Output: calculator.SweepOutput{
			Subject: enumOrEmpty(d, subjects, field+".output.subject", s.Output.Subject),
			Case:    s.Output.Case,
		},
	}
}

func (d *decoder) wing(wing Wing) calculator.WingDefinition {
	return calculator.WingDefinition{
		Name: wing.Name,
		Drivers: calculator.PlanformDrivers{
			Shape:       enumOrEmpty(d, shapes, "design.wing.shape", wing.Shape),
			Span:        d.quantity("design.wing.span", wing.Span),
			Area:        d.quantity("design.wing.area", wing.Area),
			RootChord:   d.quantity("design.wing.rootChord", wing.RootChord),
			AspectRatio: d.number("design.wing.aspectRatio", wing.AspectRatio),
			TaperRatio:  d.number("design.wing.taperRatio", wing.TaperRatio),
		},
		Sweep:          d.quantity("design.wing.sweep", wing.Sweep),
		SweepReference: d.number("design.wing.sweepReference", wing.SweepReference),
		Dihedral:       d.quantity("design.wing.dihedral", wing.Dihedral),
		Twist:          d.quantity("design.wing.twist", wing.Twist),
		Incidence:      d.quantity("design.wing.incidence", wing.Incidence),
		BodyWidth:      d.quantity("design.wing.bodyWidth", wing.BodyWidth),
		AreaBasis:      enumOrEmpty(d, areaBases, "design.wing.areaBasis", wing.AreaBasis),
		DihedralMode:   enumOrEmpty(d, dihedralModes, "design.wing.dihedralMode", wing.DihedralMode),
		RootAirfoil:    d.airfoil("design.wing.rootAirfoil", wing.RootAirfoil),
		TipAirfoil:     d.airfoil("design.wing.tipAirfoil", wing.TipAirfoil),
	}
}

func (d *decoder) airfoil(field string, airfoil *Airfoil) calculator.Airfoil {
	if airfoil == nil {
		return calculator.Airfoil{}
	}
	return calculator.Airfoil{
		Designation:    airfoil.Designation,
		Evidence:       airfoil.Evidence,
		ThicknessRatio: d.number(field+".thicknessRatio", airfoil.ThicknessRatio),
	}
}

func (d *decoder) tail(tail Tail) calculator.TailGeometry {
	out := calculator.TailGeometry{}
	if tail.Horizontal != nil {
		out.Horizontal = d.surface("design.tail.horizontal", *tail.Horizontal)
	}
	if tail.Vertical != nil {
		out.Vertical = d.surface("design.tail.vertical", *tail.Vertical)
	}
	if tail.VTail != nil {
		out.VTail = calculator.VTailPanels{
			PanelArea: d.quantity("design.tail.vtail.panelArea", tail.VTail.PanelArea),
			PanelSpan: d.quantity("design.tail.vtail.panelSpan", tail.VTail.PanelSpan),
			Cant:      d.quantity("design.tail.vtail.cant", tail.VTail.Cant),
			Arm:       d.quantity("design.tail.vtail.arm", tail.VTail.Arm),
		}
	}
	return out
}

func (d *decoder) surface(field string, surface Surface) calculator.TailSurface {
	return calculator.TailSurface{
		Area: d.quantity(field+".area", surface.Area),
		Span: d.quantity(field+".span", surface.Span),
		Arm:  d.quantity(field+".arm", surface.Arm),
	}
}

func (d *decoder) designCase(field string, c Case) calculator.DesignCase {
	return calculator.DesignCase{
		Case: calculator.FlightCase{
			Name:           c.Name,
			Configuration:  c.Configuration,
			DensityBasis:   c.DensityBasis,
			ViscosityBasis: c.ViscosityBasis,
			Density:        d.quantity(field+".density", c.Density),
			Viscosity:      d.quantity(field+".viscosity", c.Viscosity),
			LoadFactor:     d.number(field+".loadFactor", c.LoadFactor),
			CLmax:          d.clmax(field+".clmax", c.CLmax),
		},
		CLmaxEvidence: enumOrEmpty(d, evidenceGrades, field+".clmax.evidence", c.CLmax.Evidence),
		Priority:      enumOrEmpty(d, priorities, field+".priority", c.Priority),
	}
}

func (d *decoder) clmax(field string, c CLmax) calculator.LiftCoefficient {
	return calculator.LiftCoefficient{
		Basis: c.Basis,
		Max:   d.number(field+".max", c.Max),
		Scope: enumOrEmpty(d, coefficientScopes, field+".scope", c.Scope),
	}
}

func (d *decoder) requirement(field string, r Requirement) calculator.Requirement {
	return calculator.Requirement{
		Name:     r.Name,
		Basis:    r.Basis,
		Cases:    append([]string(nil), r.Cases...),
		Minimum:  d.quantity(field+".minimum", r.Minimum),
		Maximum:  d.quantity(field+".maximum", r.Maximum),
		Margin:   d.number(field+".margin", r.Margin),
		Subject:  enumOrEmpty(d, subjects, field+".subject", r.Subject),
		Priority: enumOrEmpty(d, priorities, field+".priority", r.Priority),
	}
}

func (d *decoder) scope(field string, scope *Scope) calculator.CaseScope {
	if scope == nil {
		d.add(field, "missing",
			"name the case scope: sizing against one case and against every required case are "+
				"different actions")
		return calculator.CaseScope{}
	}
	kind := enumOrEmpty(d, caseScopes, field+".kind", scope.Kind)
	if kind == calculator.CaseScopeSingle && scope.Case == "" {
		d.add(field+".case", "missing", "a single-case scope must name the case")
	}
	if kind == calculator.CaseScopeAllRequired && scope.Case != "" {
		d.add(field+".case", "invalid", "an all-required scope names no single case")
	}
	return calculator.CaseScope{Kind: kind, Name: scope.Case}
}

// request checks that a call carries an evaluation identity. It is required
// rather than defaulted: an unidentified result cannot be matched against the
// request a client is still waiting on, which is the whole point of carrying it.
func (d *decoder) request(r Request) Request {
	if r.Session == "" {
		d.add("request.session", "missing",
			"name the client session so a result can be matched to the request it answers")
	}
	if r.Sequence == 0 {
		d.add("request.sequence", "missing",
			"supply the session's request number; sequence 0 is the never-minted identity")
	}
	return r
}
