package calculator

import (
	"math"
	"sort"
)

// CaseScopeKind names which cases an action or an intersected bound covers.
// It is stated rather than defaulted: "size at the stall limit" against one
// case and against every required case are different actions with different
// answers, and reading one as the other silently relaxes a constraint.
type CaseScopeKind uint8

const (
	// CaseScopeUnknown is the zero value and is never accepted.
	CaseScopeUnknown CaseScopeKind = iota
	// CaseScopeAllRequired covers every required case in the design.
	CaseScopeAllRequired
	// CaseScopeSingle covers one named case, whatever its priority.
	CaseScopeSingle
)

var caseScopeKindNames = [...]string{
	CaseScopeUnknown:     "unknown",
	CaseScopeAllRequired: "all required cases",
	CaseScopeSingle:      "single case",
}

// String returns the scope kind's readable name.
func (k CaseScopeKind) String() string {
	if int(k) < len(caseScopeKindNames) {
		return caseScopeKindNames[k]
	}
	return "unknown"
}

// CaseScope names the cases a bound or an action covers.
type CaseScope struct {
	// Name is the case, for CaseScopeSingle only.
	Name string
	// Kind selects all required cases or one named case.
	Kind CaseScopeKind
}

// AllRequiredCases scopes an action to every required case in the design.
func AllRequiredCases() CaseScope { return CaseScope{Kind: CaseScopeAllRequired} }

// SingleCase scopes an action to one named case, whatever its priority.
func SingleCase(name string) CaseScope { return CaseScope{Kind: CaseScopeSingle, Name: name} }

// String describes the scope, for a message that has to name it.
func (s CaseScope) String() string {
	if s.Kind == CaseScopeSingle {
		return "case " + s.Name
	}
	return s.Kind.String()
}

// BoundContribution is one case's or one requirement's contribution to an
// intersected bound, with the evaluation that produced it.
type BoundContribution struct {
	// Source names the case or requirement the contribution came from.
	Source string
	// Detail explains a contribution that could not be evaluated.
	Detail string
	// Trace records the evaluation, when one ran.
	Trace Trace
	// Value is the bound this source implies.
	Value Quantity
	// Known reports whether Value was established. An unknown contribution makes
	// the intersected bound partial rather than silently dropping out of it.
	Known bool
}

// SizingBound is the intersection of every applicable contribution: the largest
// lower bound, or the smallest upper bound. It reports which sources control
// it, including ties, and whether any applicable source failed to contribute.
type SizingBound struct {
	// Detail explains an unknown or partial bound.
	Detail string
	// Contributions are every source considered, in a stable order.
	Contributions []BoundContribution
	// Controlling names the sources that set Value. More than one means a tie.
	Controlling []string
	// Value is the intersected bound.
	Value Quantity
	// Subject names the bounded quantity.
	Subject RequirementSubject
	// Direction says which side of the value the bound is.
	Direction BoundDirection
	// Partial reports that at least one applicable source could not contribute,
	// so Value bounds the design but does not establish a complete feasible
	// interval or overall compliance.
	Partial bool
	// Known reports whether any source contributed a value at all.
	Known bool
}

// boundTie is the relative agreement within which two contributions are
// reported as tied controlling sources rather than as one narrowly winning. It
// matches the tolerance a requirement check uses at its boundary, so a bound
// and the check against it cannot disagree about a tie.
const boundTie = requirementTolerance

// intersect reduces the contributions to a single bound.
func (b *SizingBound) intersect() {
	best := math.NaN()
	for n := range b.Contributions {
		if !b.Contributions[n].Known {
			b.Partial = true
			continue
		}
		if value := b.Contributions[n].Value.si; math.IsNaN(best) || better(b.Direction, value, best) {
			best = value
		}
	}
	if math.IsNaN(best) {
		b.Known = false
		return
	}
	b.Known = true
	b.Value = Quantity{si: best, dim: b.Subject.Dimension()}
	for n := range b.Contributions {
		c := &b.Contributions[n]
		if c.Known && math.Abs(c.Value.si-best) <= boundTie*math.Abs(best) {
			b.Controlling = append(b.Controlling, c.Source)
		}
	}
}

// better reports whether candidate tightens the bound relative to current.
func better(direction BoundDirection, candidate, current float64) bool {
	if direction == BoundLower {
		return candidate > current
	}
	return candidate < current
}

// stallCeilings returns the required stall-speed ceilings that apply in scope,
// paired with their case, in case order. Only a required ceiling in a required
// case contributes to a required bound: a preferred case and a preferred
// requirement are both assessed, and neither narrows required feasibility.
func (d Design) stallCeilings(scope CaseScope) ([]stallCeiling, error) {
	if err := scope.validate(d); err != nil {
		return nil, err
	}
	var found []stallCeiling
	for c := range d.Cases {
		dc := &d.Cases[c]
		if !scope.covers(dc.Priority, dc.Case.Name) {
			continue
		}
		for n := range d.Requirements {
			r := d.Requirements[n]
			if r.Subject != SubjectStallSpeed || !r.appliesTo(dc.Case.Name) {
				continue
			}
			if scope.Kind == CaseScopeAllRequired && r.Priority != PriorityRequired {
				continue
			}
			if !r.effectiveMaximum().supplied() {
				continue
			}
			found = append(found, stallCeiling{designCase: *dc, requirement: r})
		}
	}
	return found, nil
}

// stallCeiling pairs a case with the stall-speed ceiling required in it.
type stallCeiling struct {
	requirement Requirement
	designCase  DesignCase
}

// covers reports whether the scope includes a case with this priority and name.
func (s CaseScope) covers(priority Priority, name string) bool {
	if s.Kind == CaseScopeSingle {
		return name == s.Name
	}
	return priority == PriorityRequired
}

// validate rejects a scope the design cannot answer.
func (s CaseScope) validate(d Design) error {
	switch s.Kind {
	case CaseScopeAllRequired:
		return nil
	case CaseScopeSingle:
		if _, ok := d.Case(s.Name); !ok {
			return Issues{{
				Field:  "case_scope",
				Kind:   IssueMissing,
				Detail: "the design defines no case named " + s.Name + "; it defines " + joinNames(d.caseNames()),
			}}
		}
		return nil
	case CaseScopeUnknown:
		return Issues{{
			Field: "case_scope",
			Kind:  IssueMissing,
			Detail: "name the case scope: sizing against one case and against every required case " +
				"are different actions",
		}}
	default:
		return Issues{{Field: "case_scope", Kind: IssueUnsupported, Detail: "unknown case scope"}}
	}
}

// AreaLowerBound intersects the smallest wing area each applicable case
// demands, and reports which case controls. A stall ceiling bounds the area
// from below; it does not choose one.
func (d Design) AreaLowerBound(scope CaseScope) (SizingBound, error) {
	ceilings, err := d.stallCeilings(scope)
	if err != nil {
		return SizingBound{}, err
	}
	bound := SizingBound{Subject: SubjectWingArea, Direction: BoundLower}
	// The mass is read through the design's mode rather than off the entered
	// field, so a component inventory sizes the wing the same way an entered
	// figure does. An unestablished mass leaves the zero Quantity, which each
	// contribution then reports as missing in its own right.
	mass := d.massReading()
	for n := range ceilings {
		c := &ceilings[n]
		result, evalErr := MinimumWingArea(c.designCase.Case, mass.Value, c.requirement.effectiveMaximum())
		bound.Contributions = append(bound.Contributions,
			contribute(c.designCase.Case.Name, result, evalErr))
	}
	bound.intersect()
	if len(ceilings) == 0 {
		bound.Detail = "no stall-speed ceiling applies in " + scope.String() +
			", so nothing bounds the wing area from below"
	}
	return bound, nil
}

// MassUpperBound intersects the largest all-up mass each applicable case allows
// at the design's solved wing area, and reports which case controls. The
// ceiling is aerodynamic for those cases and is not a structural rating.
func (d Design) MassUpperBound(scope CaseScope) (SizingBound, error) {
	ceilings, err := d.stallCeilings(scope)
	if err != nil {
		return SizingBound{}, err
	}
	bound := SizingBound{Subject: SubjectMass, Direction: BoundUpper}
	solved := d.solveOnce()
	if failure := solved.failure(); failure != "" {
		bound.Detail = "the wing geometry did not solve, so no area is available to bound the mass at: " +
			failure
		bound.Partial = len(ceilings) > 0
		return bound, nil
	}
	area := solved.wing.Projected.Area
	for n := range ceilings {
		c := &ceilings[n]
		result, evalErr := MaximumMass(c.designCase.Case, area, c.requirement.effectiveMaximum())
		bound.Contributions = append(bound.Contributions,
			contribute(c.designCase.Case.Name, result, evalErr))
	}
	bound.intersect()
	if len(ceilings) == 0 {
		bound.Detail = "no stall-speed ceiling applies in " + scope.String() +
			", so nothing bounds the all-up mass from above"
	}
	return bound, nil
}

// contribute converts one evaluation into a contribution, keeping the reason a
// failed source could not contribute rather than dropping it.
func contribute(source string, r Result, err error) BoundContribution {
	if err != nil {
		return BoundContribution{Source: source, Detail: err.Error()}
	}
	return BoundContribution{Source: source, Value: r.Value, Trace: r.Trace, Known: true}
}

// AreaUpperBound intersects the required upper bounds on the plan-view
// reference area. A wing-area maximum contributes directly; a maximum span
// together with a minimum aspect ratio contributes S <= b^2/A, which is how a
// span limit and an aspect-ratio target become an area limit between them.
//
// Preferred requirements are excluded: they are assessed, but they never narrow
// the required feasible set.
func (d Design) AreaUpperBound() SizingBound {
	bound := SizingBound{Subject: SubjectWingArea, Direction: BoundUpper}
	span := d.requiredBound(SubjectSpan, BoundUpper)
	aspect := d.requiredBound(SubjectAspectRatio, BoundLower)
	for n := range d.Requirements {
		r := d.Requirements[n]
		if r.Priority != PriorityRequired || r.Subject != SubjectWingArea {
			continue
		}
		if ceiling := r.effectiveMaximum(); ceiling.supplied() {
			bound.Contributions = append(bound.Contributions,
				BoundContribution{Source: r.Name, Value: ceiling, Known: true})
		}
	}
	if span.known && aspect.known && aspect.value.si > 0 {
		bound.Contributions = append(bound.Contributions, BoundContribution{
			Source: span.from + " with " + aspect.from,
			Value:  Quantity{si: span.value.si * span.value.si / aspect.value.si, dim: DimArea},
			Known:  true,
			Detail: "S <= b^2/A with b at most " + span.value.String() +
				" and A at least " + aspect.value.String(),
		})
	}
	bound.intersect()
	if !bound.Known {
		bound.Detail = "no required requirement bounds the wing area from above"
	}
	return bound
}

// namedBound is one required bound found on a subject, with the requirement it
// came from.
type namedBound struct {
	from  string
	value Quantity
	known bool
}

// requiredBound returns the first required bound of the given direction on the
// subject. First rather than tightest on purpose: naming which requirement a
// derived bound came from matters more here than shaving it, and every stated
// bound is separately checked in its own right.
func (d Design) requiredBound(subject RequirementSubject, direction BoundDirection) namedBound {
	for n := range d.Requirements {
		r := d.Requirements[n]
		if r.Priority != PriorityRequired || r.Subject != subject {
			continue
		}
		value := r.effectiveMaximum()
		if direction == BoundLower {
			value = r.effectiveMinimum()
		}
		if value.supplied() {
			return namedBound{from: r.Name, value: value, known: true}
		}
	}
	return namedBound{}
}

// MassInterval is the feasible all-up mass range implied by the required
// requirements and cases. A range needs both ends: a stall ceiling supplies an
// upper bound and nothing else, so an interval with no lower bound is reported
// as one rather than as a range starting at zero.
type MassInterval struct {
	// Detail explains an incomplete or empty interval.
	Detail string
	// Lower is the intersected lower bound.
	Lower SizingBound
	// Upper is the intersected upper bound.
	Upper SizingBound
	// Complete reports whether both ends are known.
	Complete bool
	// Empty reports that the lower bound exceeds the upper one, so no mass
	// satisfies every required constraint.
	Empty bool
}

// MassInterval intersects the required mass bounds. The upper end combines the
// stall ceilings in scope with any required maximum mass; the lower end comes
// from a required minimum mass or a required minimum mass wing loading, because
// no aerodynamic ceiling implies a nonzero lower bound.
func (d Design) MassInterval(scope CaseScope) (MassInterval, error) {
	upper, err := d.MassUpperBound(scope)
	if err != nil {
		return MassInterval{}, err
	}
	solved := d.solveOnce()
	lower := SizingBound{Subject: SubjectMass, Direction: BoundLower}
	for n := range d.Requirements {
		r := d.Requirements[n]
		if r.Priority != PriorityRequired {
			continue
		}
		addMassLowerContribution(&lower, r, solved)
		if ceiling := r.effectiveMaximum(); r.Subject == SubjectMass && ceiling.supplied() {
			upper.Contributions = append(upper.Contributions,
				BoundContribution{Source: r.Name, Value: ceiling, Known: true})
		}
	}
	upper.Controlling = nil
	upper.intersect()
	lower.intersect()
	return newMassInterval(lower, upper), nil
}

// addMassLowerContribution records the lower-bound contribution of one required
// requirement, if it makes one.
func addMassLowerContribution(lower *SizingBound, r Requirement, solved solvedWing) {
	floor := r.effectiveMinimum()
	if !floor.supplied() {
		return
	}
	switch r.Subject {
	case SubjectMass:
		lower.Contributions = append(lower.Contributions,
			BoundContribution{Source: r.Name, Value: floor, Known: true})
	case SubjectWingLoadingMass:
		if failure := solved.failure(); failure != "" {
			lower.Contributions = append(lower.Contributions, BoundContribution{
				Source: r.Name,
				Detail: "the wing geometry did not solve, so a loading bound implies no mass: " + failure,
			})
			return
		}
		lower.Contributions = append(lower.Contributions, BoundContribution{
			Source: r.Name,
			Value:  Quantity{si: floor.si * solved.wing.Projected.Area.si, dim: DimMass},
			Known:  true,
			Detail: "m >= (m/S) * S with the solved reference area",
		})
	default:
	}
}

func newMassInterval(lower, upper SizingBound) MassInterval {
	interval := MassInterval{Lower: lower, Upper: upper}
	switch {
	case lower.Known && upper.Known:
		interval.Complete = true
		interval.Empty = lower.Value.si > upper.Value.si*(1+requirementTolerance)
		if interval.Empty {
			interval.Detail = "no mass satisfies every required constraint: the lower bound " +
				lower.Value.String() + " exceeds the upper bound " + upper.Value.String()
		}
	case upper.Known:
		interval.Detail = "an upper mass bound alone is not a mass range; a stall ceiling justifies " +
			"no nonzero lower bound. Supply a component minimum mass or a minimum wing loading"
	case lower.Known:
		interval.Detail = "a lower mass bound alone is not a mass range; supply a stall-speed " +
			"ceiling or a maximum mass"
	default:
		interval.Detail = "no required requirement bounds the all-up mass in either direction"
	}
	return interval
}

// Alternative is a change that would resolve a conflict. It carries the command
// that would make it, unapplied: resolving a conflict is a design decision, and
// this package never takes it on the builder's behalf.
type Alternative struct {
	// Command is the edit that would make the change.
	Command Command
	// Name identifies the alternative.
	Name string
	// Description says what the change is and what it costs, including any
	// requirement it would leave unmet.
	Description string
}

// Conflict is a group of required constraints that cannot all hold at once,
// with the alternatives that would resolve it.
//
// The group is a known conflicting set, not a minimal one. Identifying the
// smallest set of constraints whose removal restores feasibility is a solver
// result, and no solver runs here, so claiming minimality would overstate what
// was computed.
type Conflict struct {
	// Summary states the conflict in one line.
	Summary string
	// Detail explains how the two sides were derived.
	Detail string
	// Group names the participating cases and requirements.
	Group []string
	// Alternatives are changes that would resolve it, offered and not applied.
	Alternatives []Alternative
}

// Conflicts reports the known conflicting groups among the required
// constraints. Today one group is detected: a required lower bound on wing area
// that exceeds a required upper bound on it.
func (d Design) Conflicts() []Conflict {
	lower, err := d.AreaLowerBound(AllRequiredCases())
	if err != nil {
		return nil
	}
	upper := d.AreaUpperBound()
	if !lower.Known || !upper.Known || lower.Value.si <= upper.Value.si*(1+requirementTolerance) {
		return nil
	}
	group := append(append([]string(nil), lower.Controlling...), upper.Controlling...)
	sort.Strings(group)
	return []Conflict{{
		Summary: "the wing area must be at least " + lower.Value.String() + " and at most " +
			upper.Value.String(),
		Detail: "the lower bound is controlled by " + joinNames(lower.Controlling) +
			" and the upper bound by " + joinNames(upper.Controlling) +
			". This is a known conflicting group, not a minimal one: no solver ran, so no claim " +
			"is made that removing fewer constraints would restore feasibility",
		Group:        group,
		Alternatives: d.areaAlternatives(lower),
	}}
}

// areaAlternatives builds the changes that would resolve an area conflict:
// grow the span so the required area fits the required aspect ratio, or reduce
// the mass so the required area falls.
func (d Design) areaAlternatives(lower SizingBound) []Alternative {
	var alternatives []Alternative
	if aspect, name, ok := d.requiredAspectMinimum(); ok {
		span := math.Sqrt(aspect.si * lower.Value.si)
		alternatives = append(alternatives, Alternative{
			Name: "span at " + formatFloat(span) + " m",
			Description: "hold the aspect ratio at " + aspect.String() + " (" + name + ") and grow the span to " +
				formatFloat(span) + " m, which carries the required area " + lower.Value.String() +
				". Any maximum-span requirement would then be unmet and has to be revised deliberately",
			Command: SetDriver{Key: ParamSpanProjected, Value: Quantity{si: span, dim: DimLength}},
		})
	}
	if ceiling, err := d.MassUpperBound(AllRequiredCases()); err == nil && ceiling.Known {
		alternatives = append(alternatives, Alternative{
			Name: "mass at " + formatFloat(ceiling.Value.si) + " kg",
			Description: "keep the geometry and reduce the all-up mass to " + ceiling.Value.String() +
				", the ceiling " + joinNames(ceiling.Controlling) + " allows at the current area",
			Command: SetMass{Mass: ceiling.Value, Basis: "reduced to the stall-limited ceiling at the current geometry"},
		})
	}
	return alternatives
}

// requiredAspectMinimum returns the required minimum aspect ratio, if one is set.
func (d Design) requiredAspectMinimum() (Quantity, string, bool) {
	aspect := d.requiredBound(SubjectAspectRatio, BoundLower)
	return aspect.value, aspect.from, aspect.known
}
