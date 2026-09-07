package calculator

// ParameterKey is a stable, transport-neutral name for one value in a design
// definition. The keys are the contract a saved definition, an API, an MCP
// sidecar and a CAD export all share, so they never encode a display label, a
// unit or a CAD expression syntax.
type ParameterKey string

// The supported wing parameter keys.
const (
	ParamSpanProjected     ParameterKey = "wing.span.projected"
	ParamSpanPanel         ParameterKey = "wing.span.panel"
	ParamAreaReference     ParameterKey = "wing.area.reference"
	ParamAreaPanel         ParameterKey = "wing.area.panel"
	ParamAreaExposed       ParameterKey = "wing.area.exposed"
	ParamSemiSpanProjected ParameterKey = "wing.semispan.projected"
	ParamSemiSpanPanel     ParameterKey = "wing.semispan.panel"
	// ParamAspectRatio is the aspect ratio in the plane the drivers were given
	// in, which is the plane the planform was solved in.
	ParamAspectRatio ParameterKey = "wing.aspect_ratio.planform"
	// ParamAspectRatioProjected is the plan-view aspect ratio, which is the one
	// an aerodynamic model uses. Under dihedral it is not the planform value.
	ParamAspectRatioProjected ParameterKey = "wing.aspect_ratio.projected"
	ParamTaperRatio           ParameterKey = "wing.taper_ratio"
	ParamChordRoot            ParameterKey = "wing.chord.root"
	ParamChordTip             ParameterKey = "wing.chord.tip"
	ParamChordMeanGeo         ParameterKey = "wing.chord.mean_geometric"
	ParamChordMAC             ParameterKey = "wing.chord.mac"
	ParamStationMAC           ParameterKey = "wing.station.mac"
	ParamSweepReference       ParameterKey = "wing.sweep.reference"
	ParamSweepLeadingEdge     ParameterKey = "wing.sweep.leading_edge"
	ParamDihedral             ParameterKey = "wing.dihedral"
	ParamTwist                ParameterKey = "wing.twist"
	ParamIncidence            ParameterKey = "wing.incidence"
	ParamTipLEOffset          ParameterKey = "wing.tip.leading_edge_offset"
	ParamTipRise              ParameterKey = "wing.tip.rise"
	ParamStationMACLeading    ParameterKey = "wing.station.mac_leading_edge"
	ParamBodyWidth            ParameterKey = "body.width_at_wing"
	// ParamAllUpMass names the entered all-up mass. It is not a wing parameter
	// and never appears in a solved wing's parameter set; it is named here
	// because it is a value the builder enters and a sensitivity sweep can move.
	ParamAllUpMass ParameterKey = "design.mass"
)

// ParameterRole separates what the builder chose from what follows from that
// choice. Changing the solve mode changes the roles, not the key names.
type ParameterRole uint8

const (
	// RoleUnknown is the zero value.
	RoleUnknown ParameterRole = iota
	// RoleDriver is a value the builder supplied.
	RoleDriver
	// RoleDerived is a value this package computed, and carries the equation
	// and revision that produced it.
	RoleDerived
)

var parameterRoleNames = [...]string{
	RoleUnknown: "unknown",
	RoleDriver:  "driver",
	RoleDerived: "derived",
}

// String returns the role's readable name.
func (r ParameterRole) String() string {
	if int(r) < len(parameterRoleNames) {
		return parameterRoleNames[r]
	}
	return "unknown"
}

// Parameter is one named value in a design definition, with everything needed
// to reproduce it: its role, its unit, the datum any coordinate is measured
// from, and for a derived value the equation revision and the parameters it was
// computed from.
type Parameter struct {
	// Key is the stable name.
	Key ParameterKey
	// EquationID identifies the relationship behind a derived value.
	EquationID string
	// Revision is that equation's revision at the time of the solve.
	Revision string
	// Datum names the coordinate convention, for a positional value only.
	Datum string
	// DependsOn lists the parameters a derived value was computed from. It is
	// empty for a driver.
	DependsOn []ParameterKey
	// Value is the value itself, held in SI.
	Value Quantity
	// Unit is the unit Value is expressed in, stated rather than implied.
	Unit Unit
	// Role separates a driver from a derived value.
	Role ParameterRole
}

// ParameterSet is one design definition's parameters, in a stable order.
type ParameterSet []Parameter

// Get returns the parameter with the given key.
func (ps ParameterSet) Get(key ParameterKey) (Parameter, bool) {
	for _, p := range ps {
		if p.Key == key {
			return p, true
		}
	}
	return Parameter{}, false
}

// WithRole returns the keys holding the given role, in set order.
func (ps ParameterSet) WithRole(role ParameterRole) []ParameterKey {
	keys := make([]ParameterKey, 0, len(ps))
	for _, p := range ps {
		if p.Role == role {
			keys = append(keys, p.Key)
		}
	}
	return keys
}

// EvaluationOrder returns the parameters in an order where every dependency
// comes before the value that uses it. It fails when a dependency is not in the
// set, and when the dependencies form a cycle, which is what makes "the graph
// is acyclic" a checked property rather than an intention.
func (ps ParameterSet) EvaluationOrder() ([]ParameterKey, error) {
	remaining := make(map[ParameterKey][]ParameterKey, len(ps))
	for _, p := range ps {
		if _, duplicate := remaining[p.Key]; duplicate {
			return nil, Issues{{
				Field:  string(p.Key),
				Kind:   IssueInvalid,
				Detail: "the parameter set defines this key twice",
			}}
		}
		remaining[p.Key] = p.DependsOn
	}
	for _, p := range ps {
		for _, dep := range p.DependsOn {
			if _, ok := remaining[dep]; !ok {
				return nil, Issues{{
					Field:  string(p.Key),
					Kind:   IssueMissing,
					Detail: "depends on " + string(dep) + ", which is not in the parameter set",
				}}
			}
		}
	}
	ordered := make([]ParameterKey, 0, len(ps))
	placed := make(map[ParameterKey]bool, len(ps))
	for len(ordered) < len(ps) {
		progressed := false
		for _, p := range ps {
			if placed[p.Key] || !allPlaced(placed, p.DependsOn) {
				continue
			}
			placed[p.Key] = true
			ordered = append(ordered, p.Key)
			progressed = true
		}
		if !progressed {
			return nil, Issues{{
				Field:  "parameters",
				Kind:   IssueInvalid,
				Detail: "the dependency graph has a cycle involving " + unplacedKeys(ps, placed),
			}}
		}
	}
	return ordered, nil
}

func allPlaced(placed map[ParameterKey]bool, deps []ParameterKey) bool {
	for _, dep := range deps {
		if !placed[dep] {
			return false
		}
	}
	return true
}

func unplacedKeys(ps ParameterSet, placed map[ParameterKey]bool) string {
	names := ""
	for _, p := range ps {
		if placed[p.Key] {
			continue
		}
		if names != "" {
			names += ", "
		}
		names += string(p.Key)
	}
	return names
}

// parameterBuilder assembles a wing's parameter set, keeping the roles and the
// dependency edges in one place so that they cannot disagree with each other.
type parameterBuilder struct {
	params    ParameterSet
	spanKey   ParameterKey
	areaKey   ParameterKey
	otherSpan ParameterKey
	otherArea ParameterKey
	wing      Wing
}

func (b *parameterBuilder) driver(key ParameterKey, value Quantity) {
	b.params = append(b.params, Parameter{
		Key:   key,
		Role:  RoleDriver,
		Value: value,
		Unit:  value.Dimension().SIUnit(),
	})
}

func (b *parameterBuilder) derived(key ParameterKey, value Quantity, id string, deps ...ParameterKey) {
	revision := ""
	if eq, err := Lookup(id); err == nil {
		revision = eq.Revision
	}
	b.params = append(b.params, Parameter{
		Key:        key,
		Role:       RoleDerived,
		Value:      value,
		Unit:       value.Dimension().SIUnit(),
		EquationID: id,
		Revision:   revision,
		DependsOn:  deps,
	})
}

// located records a parameter that only means something against a datum.
func (b *parameterBuilder) located(key ParameterKey, value Quantity, id string, deps ...ParameterKey) {
	b.derived(key, value, id, deps...)
	b.params[len(b.params)-1].Datum = DatumWingRoot
}

// Parameters exports the wing as a dependency graph of named parameters. The
// keys are stable across solve modes; the roles and the edges are not, because
// they record what the builder actually chose.
func (w Wing) Parameters() ParameterSet {
	b := &parameterBuilder{wing: w}
	b.spanKey, b.areaKey, b.otherSpan, b.otherArea = planeKeys(w.Definition.DihedralMode)
	b.addSizeDrivers()
	b.addPlanformChords()
	b.addOtherPlane()
	b.addAnglesAndConstruction()
	return b.params
}

// planeKeys names the plane the drivers were given in and the plane derived
// from it. At zero dihedral the two coincide numerically, but they stay
// separate keys so an importer never has to guess which one a number was.
func planeKeys(mode DihedralMode) (spanKey, areaKey, otherSpan, otherArea ParameterKey) {
	if mode == DihedralHoldPanel {
		return ParamSpanPanel, ParamAreaPanel, ParamSpanProjected, ParamAreaReference
	}
	return ParamSpanProjected, ParamAreaReference, ParamSpanPanel, ParamAreaPanel
}

// sizeDriverKeys names which two size values the solve mode treats as drivers.
func sizeDriverKeys(mode SolveMode, spanKey, areaKey ParameterKey) (first, second ParameterKey) {
	switch mode {
	case SolveFromSpanAndArea:
		return spanKey, areaKey
	case SolveFromSpanAndAspectRatio:
		return spanKey, ParamAspectRatio
	case SolveFromAreaAndAspectRatio:
		return areaKey, ParamAspectRatio
	case SolveFromSpanAndRootChord:
		return spanKey, ParamChordRoot
	case SolveFromAreaAndRootChord:
		return areaKey, ParamChordRoot
	case SolveFromAspectRatioAndRootChord:
		return ParamAspectRatio, ParamChordRoot
	case SolveModeUnknown:
		return "", ""
	default:
		return "", ""
	}
}

// sizeEquations names the relationship behind each derived size value for the
// mode, so a parameter's edges match the equation that actually ran.
func sizeEquations(mode SolveMode) (spanFrom, areaFrom string) {
	switch mode {
	case SolveFromSpanAndArea:
		return "", ""
	case SolveFromSpanAndAspectRatio:
		return "", EqAreaFromSpanAndAspect
	case SolveFromAreaAndAspectRatio:
		return EqSpanFromAreaAndAspect, ""
	case SolveFromSpanAndRootChord:
		return "", EqAreaFromSpanAndRootChord
	case SolveFromAreaAndRootChord:
		return EqSpanFromAreaAndRootChord, ""
	case SolveFromAspectRatioAndRootChord:
		return EqSpanFromAspectAndRootChord, EqAreaFromSpanAndAspect
	case SolveModeUnknown:
		return "", ""
	default:
		return "", ""
	}
}

func (b *parameterBuilder) addSizeDrivers() {
	p := b.wing.Planform
	first, second := sizeDriverKeys(p.Mode, b.spanKey, b.areaKey)
	isDriver := func(key ParameterKey) bool { return key == first || key == second }

	b.driver(ParamTaperRatio, ratio(p.TaperRatio))
	spanFrom, areaFrom := sizeEquations(p.Mode)
	if isDriver(b.spanKey) {
		b.driver(b.spanKey, p.Span)
	} else {
		b.derived(b.spanKey, p.Span, spanFrom, dependenciesFor(spanFrom, b.spanKey, b.areaKey)...)
	}
	if isDriver(b.areaKey) {
		b.driver(b.areaKey, p.Area)
	} else {
		b.derived(b.areaKey, p.Area, areaFrom, dependenciesFor(areaFrom, b.spanKey, b.areaKey)...)
	}
	if isDriver(ParamAspectRatio) {
		b.driver(ParamAspectRatio, ratio(p.AspectRatio))
	} else {
		b.derived(ParamAspectRatio, ratio(p.AspectRatio), EqAspectRatio, b.spanKey, b.areaKey)
	}
	if isDriver(ParamChordRoot) {
		b.driver(ParamChordRoot, p.RootChord)
	} else {
		b.derived(ParamChordRoot, p.RootChord, EqRootChord, b.areaKey, b.spanKey, ParamTaperRatio)
	}
}

// dependenciesFor lists the inputs of a size equation as parameter keys.
func dependenciesFor(id string, spanKey, areaKey ParameterKey) []ParameterKey {
	switch id {
	case EqSpanFromAreaAndAspect:
		return []ParameterKey{areaKey, ParamAspectRatio}
	case EqSpanFromAreaAndRootChord:
		return []ParameterKey{areaKey, ParamChordRoot, ParamTaperRatio}
	case EqSpanFromAspectAndRootChord:
		return []ParameterKey{ParamAspectRatio, ParamChordRoot, ParamTaperRatio}
	case EqAreaFromSpanAndAspect:
		return []ParameterKey{spanKey, ParamAspectRatio}
	case EqAreaFromSpanAndRootChord:
		return []ParameterKey{spanKey, ParamChordRoot, ParamTaperRatio}
	default:
		return nil
	}
}

func (b *parameterBuilder) addPlanformChords() {
	p := b.wing.Planform
	b.derived(ParamChordTip, p.TipChord, EqTipChord, ParamChordRoot, ParamTaperRatio)
	b.derived(ParamChordMeanGeo, p.MeanChord, EqMeanChord, b.areaKey, b.spanKey)
	b.derived(ParamChordMAC, p.MAC, EqMeanAerodynamicChord, ParamChordRoot, ParamTaperRatio)
}

func (b *parameterBuilder) addOtherPlane() {
	w := b.wing
	projected := w.Projected
	panelSide := w.Panel
	if w.Definition.DihedralMode == DihedralHoldPanel {
		b.derived(b.otherSpan, projected.Span, EqProjectedSpan, b.spanKey, ParamDihedral)
		b.derived(b.otherArea, projected.Area, EqProjectedArea, b.areaKey, ParamDihedral)
	} else {
		b.derived(b.otherSpan, panelSide.Span, EqPanelSpan, b.spanKey, ParamDihedral)
		b.derived(b.otherArea, panelSide.Area, EqPanelArea, b.areaKey, ParamDihedral)
	}
	b.located(ParamStationMAC, projected.YMAC, EqMACStation, ParamSpanProjected, ParamTaperRatio)
	b.derived(ParamAspectRatioProjected, ratio(w.ProjectedAspectRatio), EqAspectRatio,
		ParamSpanProjected, ParamAreaReference)
	b.derived(ParamSemiSpanProjected, projected.Half, EqSemiSpan, ParamSpanProjected)
	b.derived(ParamSemiSpanPanel, panelSide.Half, EqSemiSpan, ParamSpanPanel)
}

func (b *parameterBuilder) addAnglesAndConstruction() {
	def := b.wing.Definition
	b.driver(ParamSweepReference, def.Sweep)
	b.driver(ParamDihedral, def.Dihedral)
	b.driver(ParamTwist, def.Twist)
	b.driver(ParamIncidence, def.Incidence)
	b.derived(ParamSweepLeadingEdge, b.wing.LeadingEdgeSweep, EqSweepTransform,
		ParamSweepReference, ParamAspectRatioProjected, ParamTaperRatio)
	b.located(ParamTipLEOffset, b.wing.TipLeadingEdgeOffset, EqTipLeadingEdgeOffset,
		ParamSpanProjected, ParamSweepLeadingEdge)
	b.located(ParamTipRise, b.wing.TipRise, EqTipRise, ParamSpanProjected, ParamDihedral)
	b.located(ParamStationMACLeading, b.wing.MACLeadingEdgeStation, EqMACLeadingEdgeStation,
		ParamStationMAC, ParamSweepLeadingEdge)
	if def.BodyWidth.supplied() {
		b.driver(ParamBodyWidth, def.BodyWidth)
		b.derived(ParamAreaExposed, b.wing.ExposedArea, EqExposedArea,
			ParamAreaReference, ParamChordRoot, ParamTaperRatio, ParamSpanProjected, ParamBodyWidth)
	}
}
