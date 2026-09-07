package calculator

import "sort"

// Priority separates a bound that must hold from one that is only preferred.
// A preferred requirement is assessed and reported like any other, but it never
// tightens required feasibility: a candidate that misses a preference is still
// feasible, and letting a wish narrow the feasible set would quietly promote it
// into a rule.
type Priority uint8

const (
	// PriorityUnknown is the zero value and is never accepted. Whether a bound
	// must hold changes what feasible means, so it is stated rather than
	// defaulted.
	PriorityUnknown Priority = iota
	// PriorityRequired must hold for the candidate to be feasible.
	PriorityRequired
	// PriorityPreferred is assessed and reported but never narrows the required
	// feasible set.
	PriorityPreferred
)

var priorityNames = [...]string{
	PriorityUnknown:   "unknown",
	PriorityRequired:  "required",
	PriorityPreferred: "preferred",
}

// String returns the priority's readable name.
func (p Priority) String() string {
	if int(p) < len(priorityNames) {
		return priorityNames[p]
	}
	return "unknown"
}

// EvidenceQuality grades how a case's maximum lift coefficient was established.
// It is reported alongside a requirement's outcome because an assumed CLmax and
// a measured one can produce the same number while meaning different things.
//
// This lives on DesignCase rather than on LiftCoefficient because it is a
// workflow judgement about a result, not an input any lift equation consumes.
// FlightCase already demands that every coefficient state its basis; this
// grades that basis without changing what Task 02 evaluates.
type EvidenceQuality uint8

const (
	// EvidenceUnstated is the zero value. A result resting on it is reported,
	// but its evidence quality is not claimed.
	EvidenceUnstated EvidenceQuality = iota
	// EvidenceAssumed is a value chosen for initial sizing, with no measurement
	// or simulation behind it.
	EvidenceAssumed
	// EvidenceMeasured comes from a wind tunnel or a flight test.
	EvidenceMeasured
	// EvidenceSimulated comes from a panel or CFD method, with its own
	// validation limits.
	EvidenceSimulated
)

var evidenceQualityNames = [...]string{
	EvidenceUnstated:  "unstated",
	EvidenceAssumed:   "assumed",
	EvidenceMeasured:  "measured",
	EvidenceSimulated: "simulated",
}

// String returns the evidence quality's readable name.
func (q EvidenceQuality) String() string {
	if int(q) < len(evidenceQualityNames) {
		return evidenceQualityNames[q]
	}
	return "unstated"
}

// DesignCase is a flight case as the workflow engine uses it: the case itself,
// whether it must hold, and how good the evidence behind its CLmax is.
type DesignCase struct {
	// Case is the flight condition, with its own density and CLmax provenance.
	Case FlightCase
	// CLmaxEvidence grades the evidence behind the case's CLmax.
	CLmaxEvidence EvidenceQuality
	// Priority states whether the case must hold. Only required cases
	// contribute to the intersected required bounds.
	Priority Priority
}

// Design is the authoritative parametric definition of one candidate: named
// inputs, the drivers the builder chose, the flight cases it must hold, and the
// requirements it is judged against. Nothing else in this package owns a second
// authoritative copy of a value that appears here.
//
// A Design is a plain value. Every edit goes through a Command so that history,
// driver roles and evaluation identity stay consistent; nothing mutates a
// Design in place.
//
// Not yet present, and deliberately so: power and mission cases (Task 09), and
// any handling requirement (Task 08). Only the implemented lift, geometry and
// mass-properties constraints are expressible here.
type Design struct {
	// Name identifies the candidate.
	Name string
	// MassBasis states where Mass came from, for example "target all-up mass"
	// or "measured airframe plus payload".
	MassBasis string
	// Cases are the flight conditions this candidate is judged in.
	Cases []DesignCase
	// Requirements are the bounds it is judged against.
	Requirements []Requirement
	// Components are the placed masses the design is built from. They are
	// balanced whatever the mass mode; whether they also set the all-up mass is
	// what MassMode selects.
	Components []MassItem
	// Wing is the wing definition, whose Drivers hold the active solve mode.
	Wing WingDefinition
	// Tail is the tail description the configuration calls for.
	Tail TailGeometry
	// Mass is the entered all-up mass. It is what the design is judged at in
	// MassModeEntered and is left alone, not overwritten, in MassModeComponents.
	Mass Quantity
	// Configuration names the layout.
	Configuration Configuration
	// MassMode selects whether the all-up mass is the entered one or the
	// component total.
	MassMode MassMode
}

// clone returns a deep copy. The slices are the only shared state a Design
// carries, and history keeps one snapshot per edit, so a shallow copy would let
// an edit reach back into an earlier revision.
func (d Design) clone() Design {
	c := d
	c.Cases = append([]DesignCase(nil), d.Cases...)
	c.Requirements = make([]Requirement, len(d.Requirements))
	for n := range d.Requirements {
		c.Requirements[n] = d.Requirements[n].clone()
	}
	c.Components = append([]MassItem(nil), d.Components...)
	return c
}

// Airframe returns the design's airframe, so that the configuration and tail
// checks implemented in Task 03 apply to a Design without a second definition
// of what an airframe is.
func (d Design) Airframe() Airframe {
	return Airframe{
		Name:          d.Name,
		Wing:          d.Wing,
		Tail:          d.Tail,
		Configuration: d.Configuration,
	}
}

// Case returns the design case with the given name.
func (d Design) Case(name string) (DesignCase, bool) {
	for n := range d.Cases {
		if d.Cases[n].Case.Name == name {
			return d.Cases[n], true
		}
	}
	return DesignCase{}, false
}

// Requirement returns the requirement with the given name.
func (d Design) Requirement(name string) (Requirement, bool) {
	for _, r := range d.Requirements {
		if r.Name == name {
			return r, true
		}
	}
	return Requirement{}, false
}

// Solve solves the wing geometry from the design's drivers.
func (d Design) Solve() (Wing, error) {
	return SolveWing(d.Wing)
}

// sizeDriverKeyOrder is the fixed order size drivers are reported in, so that
// a driver set never depends on which field a caller happened to fill first.
var sizeDriverKeyOrder = [...]ParameterKey{
	ParamSpanProjected, ParamAreaReference, ParamAspectRatio, ParamChordRoot,
}

// planeSizeKey maps a plane-neutral size key onto the plane the design's
// drivers are actually given in. Span and area exist in both planes; aspect
// ratio and root chord do not differ between them.
func (d Design) planeSizeKey(key ParameterKey) ParameterKey {
	spanKey, areaKey, _, _ := planeKeys(d.Wing.DihedralMode)
	switch key {
	case ParamSpanProjected, ParamSpanPanel:
		return spanKey
	case ParamAreaReference, ParamAreaPanel:
		return areaKey
	default:
		return key
	}
}

// DriverKeys returns the size drivers the design currently holds, in a stable
// order and named in the plane they were given in. It is what a worksheet shows
// as editable-as-a-driver, and what a promotion has to release one of.
func (d Design) DriverKeys() []ParameterKey {
	keys := make([]ParameterKey, 0, len(sizeDriverKeyOrder))
	for _, key := range sizeDriverKeyOrder {
		if d.holdsDriver(key) {
			keys = append(keys, d.planeSizeKey(key))
		}
	}
	return keys
}

// holdsDriver reports whether the plane-neutral size key is currently a driver.
func (d Design) holdsDriver(key ParameterKey) bool {
	drivers := d.Wing.Drivers
	switch key {
	case ParamSpanProjected, ParamSpanPanel:
		return drivers.Span.supplied()
	case ParamAreaReference, ParamAreaPanel:
		return drivers.Area.supplied()
	case ParamAspectRatio:
		return drivers.AspectRatio != 0
	case ParamChordRoot:
		return drivers.RootChord.supplied()
	default:
		return false
	}
}

// canonicalSizeKey maps either plane's span or area key onto the plane-neutral
// key the driver machinery works in, so a caller may name a driver in either
// plane without the two behaving differently.
func canonicalSizeKey(key ParameterKey) (ParameterKey, bool) {
	switch key {
	case ParamSpanProjected, ParamSpanPanel:
		return ParamSpanProjected, true
	case ParamAreaReference, ParamAreaPanel:
		return ParamAreaReference, true
	case ParamAspectRatio, ParamChordRoot:
		return key, true
	default:
		return "", false
	}
}

// setSizeDriver writes a size driver's value, or clears it when value is the
// zero Quantity. It is the single place the driver fields are written, so a
// promotion and a plain edit cannot disagree about what a driver is.
func (d *Design) setSizeDriver(key ParameterKey, value Quantity) {
	switch key {
	case ParamSpanProjected, ParamSpanPanel:
		d.Wing.Drivers.Span = value
	case ParamAreaReference, ParamAreaPanel:
		d.Wing.Drivers.Area = value
	case ParamAspectRatio:
		d.Wing.Drivers.AspectRatio = value.si
	case ParamChordRoot:
		d.Wing.Drivers.RootChord = value
	default:
		// Unreachable through the exported API: every caller passes a key that
		// canonicalSizeKey has already accepted. Ignoring anything else keeps a
		// non-size key from being written into a size driver.
	}
}

// sizeDriverDimension is the dimension a size driver's value must carry.
func sizeDriverDimension(key ParameterKey) Dimension {
	switch key {
	case ParamSpanProjected, ParamSpanPanel, ParamChordRoot:
		return DimLength
	case ParamAreaReference, ParamAreaPanel:
		return DimArea
	default:
		return Dimensionless
	}
}

// ValidSwaps lists the drivers that could be released to make promote a driver.
// Promotion is atomic: exactly one existing driver is released in the same
// edit, and which one is the builder's choice. A planform holds exactly two
// size drivers and every pair is a supported solve path, so a promotion is
// normally ambiguous and this is the offer a worksheet presents.
func (d Design) ValidSwaps(promote ParameterKey) ([]ParameterKey, error) {
	canonical, ok := canonicalSizeKey(promote)
	if !ok {
		return nil, Issues{{
			Field:  string(promote),
			Kind:   IssueUnsupported,
			Detail: "only span, wing area, aspect ratio and root chord are size drivers",
		}}
	}
	if d.holdsDriver(canonical) {
		return nil, Issues{{
			Field:  string(promote),
			Kind:   IssueInvalid,
			Detail: "this value is already a driver; edit it instead of promoting it",
		}}
	}
	held := d.DriverKeys()
	if len(held) == 0 {
		return nil, Issues{{
			Field:  string(promote),
			Kind:   IssueMissing,
			Detail: "the design holds no size drivers to release",
		}}
	}
	swaps := make([]ParameterKey, 0, len(held))
	for _, candidate := range held {
		trial := d.clone()
		trial.setSizeDriver(candidate, Quantity{})
		trial.setSizeDriver(canonical, probeValue(canonical))
		if _, err := trial.Wing.Drivers.solvePath(); err == nil {
			swaps = append(swaps, candidate)
		}
	}
	if len(swaps) == 0 {
		return nil, Issues{{
			Field:  string(promote),
			Kind:   IssueUnsupported,
			Detail: "no supported solve path results from promoting " + string(promote),
		}}
	}
	return swaps, nil
}

// probeValue is a placeholder used only to ask whether a driver set would have
// a supported solve path. It never reaches a result: ValidSwaps discards the
// trial design, and the real value arrives with the promotion.
func probeValue(key ParameterKey) Quantity {
	return Quantity{si: 1, dim: sizeDriverDimension(key)}
}

// solvePath reports which named solve path a driver set selects, without
// evaluating anything. It exists so a promotion can be checked for a supported
// path before it is offered, rather than after it has already been applied.
func (p PlanformDrivers) solvePath() (SolveMode, error) {
	rs := &resultSet{}
	taper := p.resolveTaper(rs)
	mode := p.resolveMode(rs, taper)
	if len(rs.issues) > 0 {
		return SolveModeUnknown, rs.issues
	}
	return mode, nil
}

// SolveMode reports the named driver pair the design currently solves from.
func (d Design) SolveMode() (SolveMode, error) {
	return d.Wing.Drivers.solvePath()
}

// caseNames returns every case name in the design, sorted, so a message that
// lists them does not depend on entry order.
func (d Design) caseNames() []string {
	names := make([]string, 0, len(d.Cases))
	for n := range d.Cases {
		names = append(names, d.Cases[n].Case.Name)
	}
	sort.Strings(names)
	return names
}
