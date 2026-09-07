package api

import "yalb.aero/calculator"

// encodeQuantity renders a core quantity in its dimension's SI unit, with the
// symbol alongside. Responses are always SI: display conversion is a
// presentation decision, and a boundary that guessed a display unit would be
// making it on the client's behalf.
func encodeQuantity(q calculator.Quantity) *Quantity {
	if q == (calculator.Quantity{}) {
		return nil
	}
	unit := q.Dimension().SIUnit()
	value, err := q.In(unit)
	if err != nil {
		return nil
	}
	return &Quantity{Value: value, Unit: unit.Symbol()}
}

// requireQuantity renders a quantity that is always present, such as a bound a
// check was compared against.
func requireQuantity(q calculator.Quantity) Quantity {
	if encoded := encodeQuantity(q); encoded != nil {
		return *encoded
	}
	return Quantity{Unit: q.Dimension().SIUnit().Symbol()}
}

// encodeIssues renders a core error's field issues. A non-Issues error becomes
// a single issue rather than a bare string, so the shape of a failure does not
// depend on which layer produced it.
// The empty slice rather than nil matters: the contract declares these fields
// as arrays, and a nil slice marshals to null, which a client that trusted the
// declared type would then iterate over. TestArrayFieldsAreNeverNull holds it.
func encodeIssues(err error) []Issue {
	if err == nil {
		return []Issue{}
	}
	issues, ok := calculator.AsIssues(err)
	if !ok {
		return []Issue{{Field: "design", Kind: "invalid", Detail: err.Error()}}
	}
	out := make([]Issue, 0, len(issues))
	for _, issue := range issues {
		out = append(out, Issue{
			Field:  issue.Field,
			Kind:   issueKinds.format(issue.Kind),
			Detail: issue.Detail,
		})
	}
	return out
}

func encodeTrace(t calculator.Trace) *Trace {
	if t.EquationID == "" {
		return nil
	}
	subs := make([]Substitution, 0, len(t.Substitutions))
	for _, s := range t.Substitutions {
		subs = append(subs, Substitution{Name: s.Name, Value: requireQuantity(s.Value)})
	}
	return &Trace{
		EquationID:    t.EquationID,
		Revision:      t.Revision,
		Expression:    t.Expression,
		Substitutions: subs,
		Result:        requireQuantity(t.Result),
	}
}

// encodeDesign renders a core design back onto the wire, so that a client can
// take the design an apply call produced and send it back unchanged.
func encodeDesign(d calculator.Design) Design {
	out := Design{
		Name:          d.Name,
		Configuration: configurations.format(d.Configuration),
		MassBasis:     d.MassBasis,
		MassMode:      massModes.format(d.MassMode),
		Mass:          encodeQuantity(d.Mass),
		Wing:          encodeWing(d.Wing),
	}
	if tail := encodeTail(d.Tail); tail != nil {
		out.Tail = tail
	}
	for n := range d.Cases {
		out.Cases = append(out.Cases, encodeCase(d.Cases[n]))
	}
	for n := range d.Requirements {
		out.Requirements = append(out.Requirements, encodeRequirement(d.Requirements[n]))
	}
	for n := range d.Components {
		out.Components = append(out.Components, encodeComponent(d.Components[n]))
	}
	return out
}

func encodeWing(w calculator.WingDefinition) Wing {
	out := Wing{
		Name:           w.Name,
		Shape:          shapes.format(w.Drivers.Shape),
		Span:           encodeQuantity(w.Drivers.Span),
		Area:           encodeQuantity(w.Drivers.Area),
		RootChord:      encodeQuantity(w.Drivers.RootChord),
		AspectRatio:    w.Drivers.AspectRatio,
		TaperRatio:     w.Drivers.TaperRatio,
		Sweep:          encodeQuantity(w.Sweep),
		Dihedral:       encodeQuantity(w.Dihedral),
		Twist:          encodeQuantity(w.Twist),
		Incidence:      encodeQuantity(w.Incidence),
		BodyWidth:      encodeQuantity(w.BodyWidth),
		SweepReference: w.SweepReference,
		AreaBasis:      areaBases.format(w.AreaBasis),
		DihedralMode:   dihedralModes.format(w.DihedralMode),
	}
	if w.RootAirfoil != (calculator.Airfoil{}) {
		out.RootAirfoil = &Airfoil{
			Designation:    w.RootAirfoil.Designation,
			Evidence:       w.RootAirfoil.Evidence,
			ThicknessRatio: w.RootAirfoil.ThicknessRatio,
		}
	}
	if w.TipAirfoil != (calculator.Airfoil{}) {
		out.TipAirfoil = &Airfoil{
			Designation:    w.TipAirfoil.Designation,
			Evidence:       w.TipAirfoil.Evidence,
			ThicknessRatio: w.TipAirfoil.ThicknessRatio,
		}
	}
	return out
}

func encodeTail(t calculator.TailGeometry) *Tail {
	if t == (calculator.TailGeometry{}) {
		return nil
	}
	out := &Tail{}
	if t.Horizontal != (calculator.TailSurface{}) {
		out.Horizontal = encodeSurface(t.Horizontal)
	}
	if t.Vertical != (calculator.TailSurface{}) {
		out.Vertical = encodeSurface(t.Vertical)
	}
	if t.VTail != (calculator.VTailPanels{}) {
		out.VTail = &VTail{
			PanelArea: encodeQuantity(t.VTail.PanelArea),
			PanelSpan: encodeQuantity(t.VTail.PanelSpan),
			Cant:      encodeQuantity(t.VTail.Cant),
			Arm:       encodeQuantity(t.VTail.Arm),
		}
	}
	return out
}

func encodeSurface(s calculator.TailSurface) *Surface {
	return &Surface{
		Area: encodeQuantity(s.Area),
		Span: encodeQuantity(s.Span),
		Arm:  encodeQuantity(s.Arm),
	}
}

// encodePosition renders a component's position. An absent coordinate stays
// absent: it means the component has not been placed on that axis, which is a
// different answer from a coordinate of zero.
func encodePosition(p calculator.Point) Position {
	return Position{X: encodeQuantity(p.X), Y: encodeQuantity(p.Y), Z: encodeQuantity(p.Z)}
}

// requirePoint renders a point every coordinate of which is present, such as a
// vertex of a drawn curve.
func requirePoint(p calculator.Point) Point {
	return Point{X: requireQuantity(p.X), Y: requireQuantity(p.Y), Z: requireQuantity(p.Z)}
}

func encodeComponent(item calculator.MassItem) Component {
	return Component{
		Name:     item.Name,
		Role:     componentRoles.format(item.Role),
		Basis:    item.Basis,
		Mass:     encodeQuantity(item.Mass),
		Position: encodePosition(item.Position),
	}
}

func encodeMassProperties(mp calculator.MassProperties) MassProperties {
	out := MassProperties{
		Datum:         mp.Datum,
		Status:        resultStatuses.format(mp.Status),
		Detail:        mp.Detail,
		Complete:      mp.Complete,
		Total:         encodeQuantity(mp.Total),
		Contributions: []MassContribution{},
	}
	if mp.Status == calculator.ResultComputed {
		cg := requirePoint(mp.CG)
		out.CG = &cg
	}
	for n := range mp.Contributions {
		c := &mp.Contributions[n]
		contribution := MassContribution{
			Name:     c.Name,
			Role:     componentRoles.format(c.Role),
			Detail:   c.Detail,
			Known:    c.Known,
			Mass:     encodeQuantity(c.Mass),
			Position: encodePosition(c.Position),
			Moments:  []Quantity{},
		}
		for _, moment := range c.Moments {
			contribution.Moments = append(contribution.Moments, requireQuantity(moment))
		}
		out.Contributions = append(out.Contributions, contribution)
	}
	return out
}

func encodeLoads(loads []calculator.CaseLoad) []CaseLoad {
	out := make([]CaseLoad, 0, len(loads))
	for n := range loads {
		load := &loads[n]
		out = append(out, CaseLoad{
			Case:         load.Case,
			Priority:     priorities.format(load.Priority),
			Status:       resultStatuses.format(load.Status),
			Detail:       load.Detail,
			RequiredLift: encodeQuantity(load.RequiredLift),
		})
	}
	return out
}

func encodeCase(c calculator.DesignCase) Case {
	return Case{
		Name:           c.Case.Name,
		Configuration:  c.Case.Configuration,
		DensityBasis:   c.Case.DensityBasis,
		ViscosityBasis: c.Case.ViscosityBasis,
		Priority:       priorities.format(c.Priority),
		Density:        encodeQuantity(c.Case.Density),
		Viscosity:      encodeQuantity(c.Case.Viscosity),
		LoadFactor:     c.Case.LoadFactor,
		CLmax: CLmax{
			Max:      c.Case.CLmax.Max,
			Scope:    coefficientScopes.format(c.Case.CLmax.Scope),
			Basis:    c.Case.CLmax.Basis,
			Evidence: evidenceGrades.format(c.CLmaxEvidence),
		},
	}
}

func encodeRequirement(r calculator.Requirement) Requirement {
	return Requirement{
		Name:     r.Name,
		Subject:  subjects.format(r.Subject),
		Priority: priorities.format(r.Priority),
		Basis:    r.Basis,
		Cases:    append([]string(nil), r.Cases...),
		Minimum:  encodeQuantity(r.Minimum),
		Maximum:  encodeQuantity(r.Maximum),
		Margin:   r.Margin,
	}
}

// encodeEvaluation renders one complete assessment.
func encodeEvaluation(e calculator.Evaluation, request Request) Evaluation {
	out := Evaluation{
		Request:             request,
		Snapshot:            e.Snapshot,
		Geometry:            resultStatuses.format(e.Geometry),
		Aggregate:           limitStatuses.format(e.Aggregate),
		HasRequired:         e.HasRequired,
		Checks:              encodeChecks(e.Checks),
		AreaLower:           encodeBound(e.AreaLower),
		AreaUpper:           encodeBound(e.AreaUpper),
		Mass:                encodeMassRange(e.Mass),
		MassProperties:      encodeMassProperties(e.MassProperties),
		Loads:               encodeLoads(e.Loads),
		Conflicts:           encodeConflicts(e.Conflicts),
		Patterns:            patternIDsFor(e.Design),
		DefinitionIssues:    encodeIssues(e.DefinitionIssues),
		GeometryIssues:      encodeIssues(e.GeometryIssues),
		ConfigurationIssues: encodeIssues(e.ConfigurationIssues),
	}
	if e.Geometry == calculator.ResultComputed {
		out.Wing = encodeSolvedWing(e.Design, e.Wing)
	}
	return out
}

func patternIDsFor(d calculator.Design) []string {
	patterns := calculator.PatternsFor(d)
	ids := make([]string, 0, len(patterns))
	for n := range patterns {
		ids = append(ids, patterns[n].ID)
	}
	return ids
}

func encodeSolvedWing(d calculator.Design, w calculator.Wing) *SolvedWing {
	out := &SolvedWing{
		Datum:        calculator.DatumWingRoot,
		Drivers:      []string{},
		Parameters:   []Parameter{},
		Outline:      []Point{},
		Views:        encodeViews(w),
		Explanations: encodeExplanations(w),
	}
	if mode, err := d.SolveMode(); err == nil {
		out.SolveMode = solveModes.format(mode)
	}
	for _, key := range d.DriverKeys() {
		out.Drivers = append(out.Drivers, string(key))
	}
	for _, p := range w.Parameters() {
		out.Parameters = append(out.Parameters, Parameter{
			Key:        string(p.Key),
			Role:       parameterRoles.format(p.Role),
			Datum:      p.Datum,
			EquationID: p.EquationID,
			Revision:   p.Revision,
			DependsOn:  encodeKeys(p.DependsOn),
			Value:      requireQuantity(p.Value),
		})
	}
	if outline, err := w.Outline(calculator.OutlinePlanView); err == nil {
		for _, point := range outline.Points {
			out.Outline = append(out.Outline, Point{
				X: requireQuantity(point.X),
				Y: requireQuantity(point.Y),
				Z: requireQuantity(point.Z),
			})
		}
	}
	return out
}

// encodeViews renders the dimensioned plan, front and side views. Every
// dimension keeps the parameter key it measures, which is what lets a worksheet
// map a click on a drawing onto a field and back again.
func encodeViews(w calculator.Wing) []SketchView {
	views := w.Views()
	out := make([]SketchView, 0, len(views))
	for n := range views {
		view := &views[n]
		encoded := SketchView{
			View:       viewKinds.format(view.View),
			Datum:      view.Datum,
			Across:     view.Across,
			Up:         view.Up,
			Curves:     []SketchCurve{},
			Dimensions: []SketchDimension{},
		}
		for c := range view.Curves {
			curve := &view.Curves[c]
			points := make([]Point, 0, len(curve.Points))
			for _, p := range curve.Points {
				points = append(points, requirePoint(p))
			}
			encoded.Curves = append(encoded.Curves, SketchCurve{
				Label:    curve.Label,
				Role:     sketchRoles.format(curve.Role),
				Points:   points,
				Mirrored: curve.Mirrored,
				Closed:   curve.Closed,
			})
		}
		for d := range view.Dimensions {
			dimension := &view.Dimensions[d]
			encoded.Dimensions = append(encoded.Dimensions, SketchDimension{
				Key:    string(dimension.Key),
				Label:  dimension.Label,
				Detail: dimension.Detail,
				Kind:   dimensionKinds.format(dimension.Kind),
				Plane:  outlinePlanes.format(dimension.Plane),
				From:   requirePoint(dimension.From),
				To:     requirePoint(dimension.To),
				Value:  requireQuantity(dimension.Value),
			})
		}
		out = append(out, encoded)
	}
	return out
}

func encodeExplanations(w calculator.Wing) []Explanation {
	explanations := w.Explanations()
	out := make([]Explanation, 0, len(explanations))
	for n := range explanations {
		e := &explanations[n]
		out = append(out, Explanation{
			Key:           string(e.Key),
			Role:          parameterRoles.format(e.Role),
			EquationID:    e.EquationID,
			Revision:      e.Revision,
			Expression:    e.Expression,
			Detail:        e.Detail,
			Substitutions: encodeSubstitutions(e.Substitutions),
			DependsOn:     encodeKeys(e.DependsOn),
			Value:         requireQuantity(e.Value),
		})
	}
	return out
}

func encodeSubstitutions(subs []calculator.Substitution) []Substitution {
	out := make([]Substitution, 0, len(subs))
	for _, s := range subs {
		out = append(out, Substitution{Name: s.Name, Value: requireQuantity(s.Value)})
	}
	return out
}

// encodeSweep renders one sensitivity sweep.
func encodeSweep(result calculator.SweepResult, request Request) SweepResponse {
	out := SweepResponse{
		Request:             request,
		Settings:            encodeSweepSettings(result.Settings),
		SettingsFingerprint: result.SettingsFingerprint,
		Snapshot:            result.Snapshot,
		SolveMode:           solveModes.format(result.Mode),
		Detail:              result.Detail,
		HeldFixed:           encodeKeys(result.HeldFixed),
		AlsoChanged:         encodeKeys(result.AlsoChanged),
		Bounds:              []SweepBound{},
		Samples:             make([]SweepSample, 0, len(result.Samples)),
		Current:             encodeSweepSample(result.Current),
		Invariant:           result.Invariant,
	}
	for n := range result.Bounds {
		b := &result.Bounds[n]
		out.Bounds = append(out.Bounds, SweepBound{
			Name:      b.Name,
			Direction: directions.format(b.Direction),
			Priority:  priorities.format(b.Priority),
			Value:     requireQuantity(b.Value),
		})
	}
	for n := range result.Samples {
		out.Samples = append(out.Samples, encodeSweepSample(result.Samples[n]))
	}
	return out
}

func encodeSweepSettings(s calculator.SweepSettings) SweepSettings {
	return SweepSettings{
		Driver:  string(s.Driver),
		Output:  SweepOutput{Subject: subjects.format(s.Output.Subject), Case: s.Output.Case},
		From:    requireQuantity(s.From),
		To:      requireQuantity(s.To),
		Samples: s.Samples,
	}
}

func encodeSweepSample(sample calculator.SweepSample) SweepSample {
	return SweepSample{
		Driver:      requireQuantity(sample.Driver),
		Value:       encodeQuantity(sample.Value),
		Status:      resultStatuses.format(sample.Status),
		Feasibility: limitStatuses.format(sample.Feasibility),
		Detail:      sample.Detail,
		Trace:       encodeTrace(sample.Trace),
		HasRequired: sample.HasRequired,
	}
}

func encodeKeys(keys []calculator.ParameterKey) []string {
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, string(key))
	}
	return out
}

func encodeChecks(checks calculator.RequirementChecks) []Check {
	out := make([]Check, 0, len(checks))
	for n := range checks {
		c := &checks[n]
		out = append(out, Check{
			Name:      c.Name,
			Case:      c.Case,
			Subject:   subjects.format(c.Subject),
			Direction: directions.format(c.Direction),
			Priority:  priorities.format(c.Priority),
			Status:    limitStatuses.format(c.Status),
			Result:    resultStatuses.format(c.Result),
			Evidence:  evidenceGrades.format(c.Evidence),
			Detail:    c.Detail,
			Bound:     requireQuantity(c.Bound),
			Actual:    encodeQuantity(c.Actual),
			Trace:     encodeTrace(c.Trace),
			Margin:    c.Margin,
		})
	}
	return out
}

func encodeBound(b calculator.SizingBound) Bound {
	out := Bound{
		Subject:     subjects.format(b.Subject),
		Direction:   directions.format(b.Direction),
		Detail:      b.Detail,
		Controlling: append([]string(nil), b.Controlling...),
		Value:       encodeQuantity(b.Value),
		Known:       b.Known,
		Partial:     b.Partial,
	}
	for n := range b.Contributions {
		c := &b.Contributions[n]
		out.Contributions = append(out.Contributions, Contribution{
			Source: c.Source,
			Detail: c.Detail,
			Value:  encodeQuantity(c.Value),
			Trace:  encodeTrace(c.Trace),
			Known:  c.Known,
		})
	}
	return out
}

func encodeMassRange(m calculator.MassInterval) MassRange {
	return MassRange{
		Detail:   m.Detail,
		Lower:    encodeBound(m.Lower),
		Upper:    encodeBound(m.Upper),
		Complete: m.Complete,
		Empty:    m.Empty,
	}
}

func encodeConflicts(conflicts []calculator.Conflict) []Conflict {
	out := make([]Conflict, 0, len(conflicts))
	for n := range conflicts {
		c := &conflicts[n]
		conflict := Conflict{
			Summary:      c.Summary,
			Detail:       c.Detail,
			Group:        append([]string{}, c.Group...),
			Alternatives: []Alternative{},
		}
		for _, alternative := range c.Alternatives {
			command, ok := encodeCommand(alternative.Command)
			if !ok {
				// An alternative whose command cannot cross the boundary would
				// read as advice with no action behind it. Saying so is better
				// than presenting half of it.
				conflict.Alternatives = append(conflict.Alternatives, Alternative{
					Name: alternative.Name,
					Description: alternative.Description +
						" (this alternative has no wire form in contract " + ContractVersion +
						"; apply it through the core)",
				})
				continue
			}
			conflict.Alternatives = append(conflict.Alternatives, Alternative{
				Name:        alternative.Name,
				Description: alternative.Description,
				Command:     command,
			})
		}
		out = append(out, conflict)
	}
	return out
}

func encodeChanges(changes []calculator.RequirementChange) []Change {
	out := make([]Change, 0, len(changes))
	for n := range changes {
		c := &changes[n]
		out = append(out, Change{
			Name:      c.Name,
			Case:      c.Case,
			Subject:   subjects.format(c.Subject),
			Direction: directions.format(c.Direction),
			From:      limitStatuses.format(c.From),
			To:        limitStatuses.format(c.To),
			Detail:    c.Detail,
			Appeared:  c.Appeared,
			Vanished:  c.Vanished,
		})
	}
	return out
}

func encodeEquation(eq calculator.Equation) Equation {
	out := Equation{
		ID:          eq.ID,
		Revision:    eq.Revision,
		Expression:  eq.Expression,
		Output:      encodePort(eq.Output),
		Assumptions: append([]string(nil), eq.Assumptions...),
		Source: Source{
			Kind:             sourceKinds.format(eq.Source.Kind),
			Title:            eq.Source.Title,
			URL:              eq.Source.URL,
			Section:          eq.Source.Section,
			Accessed:         eq.Source.Accessed,
			UpstreamRevision: eq.Source.UpstreamRevision,
			OriginalUnits:    eq.Source.OriginalUnits,
			Adaptation:       eq.Source.Adaptation,
		},
	}
	for _, port := range eq.Inputs {
		out.Inputs = append(out.Inputs, encodePort(port))
	}
	return out
}

func encodePort(p calculator.Port) Port {
	return Port{
		Name:        p.Name,
		Description: p.Description,
		Dimension:   dimensions.format(p.Dimension),
		Unit:        p.Dimension.SIUnit().Symbol(),
	}
}

func encodePattern(p calculator.Pattern) Pattern {
	return Pattern{
		ID:             p.ID,
		Name:           p.Name,
		Journey:        journeys.format(p.Journey),
		Rationale:      p.Rationale,
		Outcome:        p.Outcome,
		RequiredInputs: append([]string(nil), p.RequiredInputs...),
		ActiveDrivers:  encodeKeys(p.ActiveDrivers),
		ValidityLimits: append([]string(nil), p.ValidityLimits...),
		Supported:      p.Supported,
	}
}
