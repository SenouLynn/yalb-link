package calculator

import "math"

// RequirementSubject names the quantity a requirement bounds. Only quantities
// the implemented lift and geometry models actually produce are listed: a
// subject a model cannot evaluate would report unknown forever, which reads as
// a coverage claim this package cannot make.
type RequirementSubject uint8

const (
	// SubjectUnknown is the zero value and is never accepted.
	SubjectUnknown RequirementSubject = iota
	// SubjectStallSpeed bounds the stall speed in a named flight case. It is the
	// only per-case subject implemented, because it is the only result whose
	// value depends on the case.
	SubjectStallSpeed
	// SubjectWingArea bounds the plan-view reference area.
	SubjectWingArea
	// SubjectSpan bounds the plan-view span.
	SubjectSpan
	// SubjectMass bounds the all-up mass.
	SubjectMass
	// SubjectAspectRatio bounds the plan-view aspect ratio.
	SubjectAspectRatio
	// SubjectWingLoadingMass bounds mass-based wing loading, the form RC
	// builders usually quote. It carries no load factor.
	SubjectWingLoadingMass
	// SubjectElectricalPower bounds the largest electrical power any single
	// mission segment holds continuously. A required maximum on it is the power
	// ceiling a weight-first design can be decided by.
	SubjectElectricalPower
	// SubjectMissionEnergy bounds the energy the whole mission requires.
	SubjectMissionEnergy
	// SubjectMissionDuration bounds the mission's total time.
	SubjectMissionDuration
	// SubjectMissionRange bounds the mission's total ground distance.
	SubjectMissionRange
	// SubjectPropellerClearance bounds the propeller tip's clearance above the
	// ground line.
	SubjectPropellerClearance
)

var requirementSubjectNames = [...]string{
	SubjectUnknown:            "unknown",
	SubjectStallSpeed:         "stall speed",
	SubjectWingArea:           "wing area",
	SubjectSpan:               "span",
	SubjectMass:               "all-up mass",
	SubjectAspectRatio:        "aspect ratio",
	SubjectWingLoadingMass:    "mass wing loading",
	SubjectElectricalPower:    "electrical power",
	SubjectMissionEnergy:      "mission energy",
	SubjectMissionDuration:    "mission duration",
	SubjectMissionRange:       "mission range",
	SubjectPropellerClearance: "propeller clearance",
}

// String returns the subject's readable name.
func (s RequirementSubject) String() string {
	if int(s) < len(requirementSubjectNames) {
		return requirementSubjectNames[s]
	}
	return "unknown"
}

// Dimension returns the dimension a bound on this subject must carry.
func (s RequirementSubject) Dimension() Dimension {
	switch s {
	case SubjectStallSpeed:
		return DimSpeed
	case SubjectWingArea:
		return DimArea
	case SubjectSpan:
		return DimLength
	case SubjectMass:
		return DimMass
	case SubjectWingLoadingMass:
		return DimMassPerArea
	case SubjectElectricalPower:
		return DimPower
	case SubjectMissionEnergy:
		return DimEnergy
	case SubjectMissionDuration:
		return DimTime
	case SubjectMissionRange, SubjectPropellerClearance:
		return DimLength
	case SubjectAspectRatio, SubjectUnknown:
		return Dimensionless
	default:
		return Dimensionless
	}
}

// PerCase reports whether the subject's value depends on the flight case. A
// per-case requirement must name the cases it applies to; a design-level one
// must not, because there is only one value to bound.
func (s RequirementSubject) PerCase() bool { return s == SubjectStallSpeed }

// BoundDirection distinguishes a lower bound from an upper one. The two
// intersect differently — the largest lower bound and the smallest upper bound
// control — so which side a value is on is never inferred from context.
type BoundDirection uint8

const (
	// BoundDirectionUnknown is the zero value.
	BoundDirectionUnknown BoundDirection = iota
	// BoundLower is a minimum: the value must be at least the bound.
	BoundLower
	// BoundUpper is a maximum: the value must be at most the bound.
	BoundUpper
)

var boundDirectionNames = [...]string{
	BoundDirectionUnknown: "unknown",
	BoundLower:            "minimum",
	BoundUpper:            "maximum",
}

// String returns the direction's readable name.
func (b BoundDirection) String() string {
	if int(b) < len(boundDirectionNames) {
		return boundDirectionNames[b]
	}
	return "unknown"
}

// ResultStatus reports whether the model produced a number at all. It is kept
// separate from LimitStatus throughout: a computable candidate can violate a
// requirement, and an uncomputable one is neither met nor unmet.
type ResultStatus uint8

const (
	// ResultMissing means an input the value depends on was not supplied.
	ResultMissing ResultStatus = iota
	// ResultComputed means the model produced the value.
	ResultComputed
	// ResultInvalid means an input the value depends on cannot be true, so the
	// dependent calculation was blocked rather than approximated.
	ResultInvalid
	// ResultStale means the value was computed from an earlier revision of the
	// design and has not been recomputed.
	ResultStale
)

var resultStatusNames = [...]string{
	ResultMissing:  "missing",
	ResultComputed: "computed",
	ResultInvalid:  "invalid",
	ResultStale:    "stale",
}

// String returns the result status's readable name.
func (r ResultStatus) String() string {
	if int(r) < len(resultStatusNames) {
		return resultStatusNames[r]
	}
	return "missing"
}

// requirementTolerance is the relative slack a boundary comparison allows, so
// that a value sized to sit exactly on its bound is reported met rather than
// unmet by one float64 ulp. It is far tighter than any quoted requirement and
// far looser than the rounding of a single arithmetic step.
const requirementTolerance = 1e-9

// Requirement is a bound the candidate is judged against. It is deliberately
// not a driver: "span at most 1.4 m" does not set the span, and nothing in this
// package converts one into the other. Promoting a bound to a driver is an
// explicit edit the builder makes.
type Requirement struct {
	// Name identifies the requirement within a design.
	Name string
	// Basis states where the bound came from, for example a contest rule, a
	// doorway or a customer specification.
	Basis string
	// Cases names the flight cases a per-case requirement applies to. It must
	// be empty for a design-level subject and non-empty for a per-case one.
	Cases []string
	// Minimum is the lower bound. The zero Quantity means there is none.
	Minimum Quantity
	// Maximum is the upper bound. The zero Quantity means there is none.
	Maximum Quantity
	// Margin is a relative safety margin that tightens both bounds before they
	// are compared or intersected: a maximum becomes Maximum*(1-Margin) and a
	// minimum becomes Minimum*(1+Margin). Zero means the stated bound is used
	// as written.
	Margin float64
	// Subject names the quantity bounded.
	Subject RequirementSubject
	// Priority states whether the bound must hold.
	Priority Priority
}

func (r Requirement) clone() Requirement {
	c := r
	c.Cases = append([]string(nil), r.Cases...)
	return c
}

// effectiveMaximum applies Margin to the stated upper bound.
func (r Requirement) effectiveMaximum() Quantity {
	if !r.Maximum.supplied() {
		return Quantity{}
	}
	return Quantity{si: r.Maximum.si * (1 - r.Margin), dim: r.Maximum.dim}
}

// effectiveMinimum applies Margin to the stated lower bound.
func (r Requirement) effectiveMinimum() Quantity {
	if !r.Minimum.supplied() {
		return Quantity{}
	}
	return Quantity{si: r.Minimum.si * (1 + r.Margin), dim: r.Minimum.dim}
}

// appliesTo reports whether a per-case requirement covers the named case.
func (r Requirement) appliesTo(name string) bool {
	for _, c := range r.Cases {
		if c == name {
			return true
		}
	}
	return false
}

// validate checks a requirement against the design it belongs to, reporting
// every problem rather than the first.
func (r Requirement) validate(d Design, rs *resultSet) {
	field := "requirement." + r.Name
	if r.Name == "" {
		rs.add("requirement", IssueMissing, "name the requirement so its status can be reported against it")
	}
	if r.Subject == SubjectUnknown {
		rs.add(field, IssueMissing, "choose what the requirement bounds")
		return
	}
	if r.Priority == PriorityUnknown {
		rs.add(field, IssueMissing,
			"state whether the bound is required or preferred; a preferred bound never narrows "+
				"required feasibility")
	}
	if r.Basis == "" {
		rs.add(field, IssueMissing,
			"state where the bound came from, so an assumed requirement is visible alongside its status")
	}
	r.validateBounds(field, rs)
	r.validateCases(d, field, rs)
}

func (r Requirement) validateBounds(field string, rs *resultSet) {
	if !r.Minimum.supplied() && !r.Maximum.supplied() {
		rs.add(field, IssueMissing, "supply a minimum, a maximum or both")
	}
	want := r.Subject.Dimension()
	for _, bound := range []struct {
		name  string
		value Quantity
	}{{"minimum", r.Minimum}, {"maximum", r.Maximum}} {
		if !bound.value.supplied() {
			continue
		}
		if bound.value.dim != want {
			rs.add(field, IssueInvalid,
				"the "+bound.name+" bounds "+r.Subject.String()+", which is a "+want.String()+
					" value, but a "+bound.value.dim.String()+" value was supplied")
		}
	}
	if r.Margin < 0 || r.Margin >= 1 {
		rs.add(field, IssueInvalid,
			"the margin is a fraction of the bound between 0 and 1, received "+formatFloat(r.Margin))
		return
	}
	if r.Minimum.supplied() && r.Maximum.supplied() &&
		r.Minimum.dim == r.Maximum.dim && r.effectiveMinimum().si > r.effectiveMaximum().si {
		rs.add(field, IssueInvalid,
			"the requirement is empty after its margin: the minimum "+
				formatFloat(r.effectiveMinimum().si)+" exceeds the maximum "+
				formatFloat(r.effectiveMaximum().si))
	}
}

func (r Requirement) validateCases(d Design, field string, rs *resultSet) {
	if !r.Subject.PerCase() {
		if len(r.Cases) > 0 {
			rs.add(field, IssueInvalid,
				r.Subject.String()+" has one value for the whole design, so it takes no case list")
		}
		return
	}
	if len(r.Cases) == 0 {
		rs.add(field, IssueMissing,
			"name the flight cases this bound applies to; "+joinNames(d.caseNames())+
				" are defined, and applying it to all of them silently is a different requirement")
		return
	}
	for _, name := range r.Cases {
		if _, ok := d.Case(name); !ok {
			rs.add(field, IssueMissing, "names case "+name+", which the design does not define")
		}
	}
}

// RequirementCheck is one bound's outcome. A requirement carrying both a
// minimum and a maximum produces one check per side, so the two statuses are
// preserved separately rather than collapsed into a single verdict.
type RequirementCheck struct {
	// Name is the requirement's name.
	Name string
	// Case names the flight case, for a per-case subject only.
	Case string
	// Detail explains an unknown, invalid or unmet outcome.
	Detail string
	// Trace records the evaluation that produced Actual, when one ran.
	Trace Trace
	// Bound is the bound actually compared against, after any margin.
	Bound Quantity
	// Actual is the value it was compared with.
	Actual Quantity
	// Margin is the achieved relative room inside the bound, positive when the
	// check is met with room to spare and negative when it is violated. It is
	// zero when the outcome is not known.
	Margin float64
	// Subject names the bounded quantity.
	Subject RequirementSubject
	// Direction says which side of the bound this check is.
	Direction BoundDirection
	// Priority is the requirement's priority.
	Priority Priority
	// Status is met, unmet or unknown.
	Status LimitStatus
	// Result reports whether the model produced Actual at all.
	Result ResultStatus
	// Evidence grades the case evidence behind Actual, for a per-case subject.
	Evidence EvidenceQuality
}

// RequirementChecks is a set of outcomes in a stable order.
type RequirementChecks []RequirementCheck

// filter returns the checks the predicate accepts, in set order.
func (cs RequirementChecks) filter(keep func(*RequirementCheck) bool) RequirementChecks {
	out := make(RequirementChecks, 0, len(cs))
	for n := range cs {
		if keep(&cs[n]) {
			out = append(out, cs[n])
		}
	}
	return out
}

// WithPriority returns the checks carrying the given priority.
func (cs RequirementChecks) WithPriority(p Priority) RequirementChecks {
	return cs.filter(func(c *RequirementCheck) bool { return c.Priority == p })
}

// Named returns the checks belonging to the named requirement.
func (cs RequirementChecks) Named(name string) RequirementChecks {
	return cs.filter(func(c *RequirementCheck) bool { return c.Name == name })
}

// ForCase returns the checks evaluated in the named case.
func (cs RequirementChecks) ForCase(name string) RequirementChecks {
	return cs.filter(func(c *RequirementCheck) bool { return c.Case == name })
}

// Aggregate combines the required checks into one status. It is unmet if any
// required check is unmet, otherwise unknown if any is unknown, and met only
// when every required check is met. The second return is false when the
// required set is empty, which makes no feasibility claim at all: an empty set
// is not a passing one.
func (cs RequirementChecks) Aggregate() (LimitStatus, bool) {
	required := cs.WithPriority(PriorityRequired)
	if len(required) == 0 {
		return LimitUnknown, false
	}
	status := LimitMet
	for n := range required {
		switch required[n].Status {
		case LimitUnmet:
			return LimitUnmet, true
		case LimitUnknown:
			status = LimitUnknown
		case LimitMet:
		}
	}
	return status, true
}

// subjectReading is one subject's value together with why it is or is not
// available, so the requirement machinery never has to guess whether a zero
// Quantity means zero or means nothing was computed.
type subjectReading struct {
	Detail string
	Trace  Trace
	Value  Quantity
	Status ResultStatus
}

// read returns the value a requirement subject bounds, for the design as
// solved. A geometry failure blocks every dependent subject rather than
// producing an approximate number: mass, which no geometry feeds, still reads.
func (d Design) read(subject RequirementSubject, dc DesignCase, solved solvedWing) subjectReading {
	mass := d.massReading()
	if subject == SubjectMass {
		return mass
	}
	if subject.fromPowerModel() {
		return d.readPowerSubject(subject)
	}
	if solved.err != nil {
		return subjectReading{Status: solved.status, Detail: "the wing geometry did not solve: " + solved.err.Error()}
	}
	w := solved.wing
	switch subject {
	case SubjectSpan:
		return subjectReading{Status: ResultComputed, Value: w.Projected.Span}
	case SubjectWingArea:
		return subjectReading{Status: ResultComputed, Value: w.Projected.Area}
	case SubjectAspectRatio:
		return subjectReading{Status: ResultComputed, Value: ratio(w.ProjectedAspectRatio)}
	case SubjectWingLoadingMass:
		if mass.Status != ResultComputed {
			return mass
		}
		return reading(WingLoadingMass(mass.Value, w.Projected.Area))
	case SubjectStallSpeed:
		if mass.Status != ResultComputed {
			return mass
		}
		return reading(StallSpeed(dc.Case, mass.Value, w.Projected.Area))
	case SubjectMass, SubjectUnknown:
		return subjectReading{Status: ResultMissing, Detail: "no value is defined for " + subject.String()}
	default:
		return subjectReading{Status: ResultMissing, Detail: "no value is defined for " + subject.String()}
	}
}

// fromPowerModel reports whether the subject's value comes from the Task 09
// power and mission model rather than from the geometry and lift models. Those
// subjects need no solved wing of their own: the mission analysis solves it
// once and reports why it could not when it could not.
func (s RequirementSubject) fromPowerModel() bool {
	switch s {
	case SubjectElectricalPower, SubjectMissionEnergy, SubjectMissionDuration,
		SubjectMissionRange, SubjectPropellerClearance:
		return true
	default:
		return false
	}
}

// readPowerSubject reads a value the power and mission model produces.
func (d Design) readPowerSubject(subject RequirementSubject) subjectReading {
	if subject == SubjectPropellerClearance {
		if !d.Propulsion.Limits.PropellerDiameter.supplied() ||
			!d.Propulsion.Limits.PropellerHubHeight.supplied() {
			return subjectReading{
				Status: ResultMissing,
				Detail: "a tip clearance needs both the propeller diameter and the hub height " +
					"above the ground line",
			}
		}
		return reading(d.Propulsion.Limits.PropellerClearance())
	}
	mission := d.MissionAnalysis()
	if mission.Status != ResultComputed {
		return subjectReading{Status: mission.Status, Detail: mission.Detail}
	}
	if !mission.Complete {
		return subjectReading{
			Status: ResultMissing,
			Detail: "the mission covers only some of its segments, so its totals understate it " +
				"rather than describing it: " + mission.Detail,
		}
	}
	switch subject {
	case SubjectElectricalPower:
		return subjectReading{Status: ResultComputed, Value: mission.PeakContinuousPower}
	case SubjectMissionEnergy:
		return subjectReading{Status: ResultComputed, Value: mission.RequiredEnergy}
	case SubjectMissionDuration:
		return subjectReading{Status: ResultComputed, Value: mission.TotalDuration}
	case SubjectMissionRange:
		return subjectReading{Status: ResultComputed, Value: mission.TotalDistance}
	default:
		return subjectReading{Status: ResultMissing, Detail: "no value is defined for " + subject.String()}
	}
}

// reading converts an evaluation into a subject reading, mapping a rejected
// input onto missing or invalid rather than collapsing both into "no value".
func reading(r Result, err error) subjectReading {
	if err == nil {
		return subjectReading{Status: ResultComputed, Value: r.Value, Trace: r.Trace}
	}
	return subjectReading{Status: statusForError(err), Detail: err.Error()}
}

// statusForError maps an evaluation's issues onto a result status. An invalid
// input blocks the dependent calculation; a merely missing one leaves it
// uncomputed.
func statusForError(err error) ResultStatus {
	issues, ok := AsIssues(err)
	if !ok {
		return ResultInvalid
	}
	if issues.Kind(IssueInvalid) || issues.Kind(IssueUnsupported) {
		return ResultInvalid
	}
	return ResultMissing
}

// solvedWing carries one wing solve and why it failed, so every requirement in
// a pass shares a single solve rather than repeating it.
type solvedWing struct {
	err    error
	wing   Wing
	status ResultStatus
}

// failure returns why the wing did not solve, or the empty string when it did.
// Callers that report a failure rather than propagating it read this instead of
// the error, so that returning a described bound is not mistaken for swallowing
// an error.
func (s solvedWing) failure() string {
	if s.err == nil {
		return ""
	}
	return s.err.Error()
}

func (d Design) solveOnce() solvedWing {
	w, err := d.Solve()
	if err != nil {
		return solvedWing{err: err, status: statusForError(err)}
	}
	return solvedWing{wing: w}
}

// Assess evaluates every requirement against the design, one check per supplied
// bound side and, for a per-case subject, one per applicable case. The checks
// come back in requirement order, then case order, so the result never depends
// on map iteration.
func (d Design) Assess() RequirementChecks {
	solved := d.solveOnce()
	checks := make(RequirementChecks, 0, len(d.Requirements))
	for _, r := range d.Requirements {
		if r.Subject.PerCase() {
			for n := range d.Cases {
				if !r.appliesTo(d.Cases[n].Case.Name) {
					continue
				}
				checks = append(checks, d.checkBounds(r, d.Cases[n], solved)...)
			}
			continue
		}
		checks = append(checks, d.checkBounds(r, DesignCase{}, solved)...)
	}
	return checks
}

// checkBounds produces the checks for one requirement in one case.
func (d Design) checkBounds(r Requirement, dc DesignCase, solved solvedWing) RequirementChecks {
	value := d.read(r.Subject, dc, solved)
	checks := make(RequirementChecks, 0, 2)
	for _, side := range []struct {
		bound     Quantity
		direction BoundDirection
	}{
		{r.effectiveMinimum(), BoundLower},
		{r.effectiveMaximum(), BoundUpper},
	} {
		if !side.bound.supplied() {
			continue
		}
		checks = append(checks, buildCheck(r, dc, value, side.bound, side.direction))
	}
	return checks
}

// buildCheck compares one value against one bound and records the outcome.
func buildCheck(r Requirement, dc DesignCase, value subjectReading, bound Quantity,
	direction BoundDirection,
) RequirementCheck {
	check := RequirementCheck{
		Name:      r.Name,
		Case:      dc.Case.Name,
		Subject:   r.Subject,
		Direction: direction,
		Priority:  r.Priority,
		Bound:     bound,
		Actual:    value.Value,
		Trace:     value.Trace,
		Result:    value.Status,
		Evidence:  dc.CLmaxEvidence,
		Detail:    value.Detail,
	}
	if value.Status != ResultComputed {
		check.Status = LimitUnknown
		return check
	}
	slack := requirementTolerance * math.Abs(bound.si)
	if direction == BoundLower {
		check.Status = statusFor(value.Value.si >= bound.si-slack)
		check.Margin = relativeMargin(value.Value.si-bound.si, bound.si)
	} else {
		check.Status = statusFor(value.Value.si <= bound.si+slack)
		check.Margin = relativeMargin(bound.si-value.Value.si, bound.si)
	}
	if check.Status == LimitUnmet {
		check.Detail = value.Value.String() + " is outside the " + direction.String() + " of " + bound.String()
	}
	return check
}

func statusFor(ok bool) LimitStatus {
	if ok {
		return LimitMet
	}
	return LimitUnmet
}

// relativeMargin expresses the room left against the bound as a fraction of it.
func relativeMargin(room, bound float64) float64 {
	if bound == 0 {
		return 0
	}
	return room / math.Abs(bound)
}

// Validate reports every structural problem in the design definition: an
// unnamed configuration, a case without its priority, a requirement without a
// bound. It does not solve the wing and it is not a feasibility result.
func (d Design) Validate() error {
	rs := &resultSet{}
	if d.Name == "" {
		rs.add("design", IssueMissing, "name the design so its results can be reported against it")
	}
	d.validateMass(rs)
	d.validateComponents(rs)
	d.validateCases(rs)
	d.Polar.validate(rs)
	d.Propulsion.validate(rs)
	d.Battery.validate(d, rs)
	d.validateAuxiliary(rs)
	d.Mission.validate(d, rs)
	for _, r := range d.Requirements {
		r.validate(d, rs)
	}
	if len(rs.issues) > 0 {
		return rs.issues
	}
	return nil
}

// validateMass checks the mass the design is judged at, in the mode it chose.
// In the component mode the entered mass is not demanded and not read: the
// inventory is what the aircraft weighs, and its completeness is reported by
// the mass properties rather than refused here.
func (d Design) validateMass(rs *resultSet) {
	if d.MassMode.resolved() == MassModeComponents {
		return
	}
	if !d.Mass.supplied() {
		rs.add("mass", IssueMissing, "supply the all-up mass")
		return
	}
	if d.MassBasis == "" {
		rs.add("mass", IssueMissing,
			"state where the all-up mass came from, so a target mass is not read as a measured one")
	}
}

// validateAuxiliary checks the electrical loads. A load that names a component
// must name one the design lists, so that a device's mass and its draw stay
// attached to the same thing.
func (d Design) validateAuxiliary(rs *resultSet) {
	seen := make(map[string]bool, len(d.Auxiliary))
	for _, load := range d.Auxiliary {
		if seen[load.Name] && load.Name != "" {
			rs.add("auxiliary."+load.Name, IssueInvalid, "the design lists this load twice")
		}
		seen[load.Name] = true
		load.validate(rs)
		if load.Component == "" {
			continue
		}
		if _, ok := d.Component(load.Component); !ok {
			rs.add("auxiliary."+load.Name, IssueMissing,
				"names component "+load.Component+", which the design does not list; it lists "+
					joinNames(d.componentNames()))
		}
	}
}

func (d Design) validateCases(rs *resultSet) {
	seen := make(map[string]bool, len(d.Cases))
	for n := range d.Cases {
		c := &d.Cases[n]
		if c.Case.Name == "" {
			rs.add("case", IssueMissing, "name every flight case so a requirement can refer to it")
			continue
		}
		if seen[c.Case.Name] {
			rs.add("case."+c.Case.Name, IssueInvalid, "the design defines this case twice")
		}
		seen[c.Case.Name] = true
		if c.Priority == PriorityUnknown {
			rs.add("case."+c.Case.Name, IssueMissing,
				"state whether the case is required or preferred; only required cases contribute "+
					"to the intersected bounds")
		}
	}
}
