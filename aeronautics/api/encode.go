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
		Datum:      calculator.DatumWingRoot,
		Drivers:    []string{},
		Parameters: []Parameter{},
		Outline:    []Point{},
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
