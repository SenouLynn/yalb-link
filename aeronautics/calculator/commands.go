package calculator

// Command is one edit against a design definition. Commands are the only way a
// Design changes: a worksheet field, a dragged handle and a curated action all
// become the same kind of value, so history, driver roles and evaluation
// identity cannot disagree about what happened.
//
// The interface has an unexported method on purpose. The supported edits are a
// curated set defined in this package, not a general-purpose expression
// language or arbitrary code a caller supplies, which is what the task calls
// for at this stage.
type Command interface {
	// Label names the edit for history and for a preview's description.
	Label() string
	apply(Design) (Design, error)
}

// commandIssue builds the single-issue error a rejected command returns.
func commandIssue(field string, kind IssueKind, detail string) error {
	return Issues{{Field: field, Kind: kind, Detail: detail}}
}

// SetMass sets the all-up mass and the basis it came from.
//
// An empty basis is accepted here and reported by Design.Validate instead. The
// requirement that a mass say where it came from is not relaxed by that: it is
// enforced where the design is judged rather than at the keystroke, so a
// builder can type the number and the provenance in either order without the
// first of the two being refused for the absence of the second.
type SetMass struct {
	// Basis states where the mass came from.
	Basis string
	// Mass is the new all-up mass. The zero Quantity withdraws it.
	Mass Quantity
}

// Label names the edit.
func (c SetMass) Label() string { return "set all-up mass to " + c.Mass.String() }

func (c SetMass) apply(d Design) (Design, error) {
	if c.Mass.supplied() && c.Mass.dim != DimMass {
		return Design{}, commandIssue("mass", IssueInvalid,
			"expected a mass but received "+c.Mass.dim.String())
	}
	out := d.clone()
	out.Mass = c.Mass
	out.MassBasis = c.Basis
	return out, nil
}

// SetDriver changes the value of a size driver the design already holds, or
// fills an empty slot when fewer than two are held. It refuses to turn a
// derived value into a driver once two are held: that is a promotion, which
// releases another driver in the same edit and is a different decision.
//
// The zero Quantity withdraws the driver. Withdrawing is how a builder takes a
// value back without asserting anything in its place, and it is a different act
// from entering zero, which the solver would refuse as an impossible span.
type SetDriver struct {
	// Key names the driver, in either plane.
	Key ParameterKey
	// Value is its new value, or the zero Quantity to withdraw it. An aspect
	// ratio is a dimensionless Quantity.
	Value Quantity
}

// Label names the edit.
func (c SetDriver) Label() string { return "set " + string(c.Key) + " to " + c.Value.String() }

func (c SetDriver) apply(d Design) (Design, error) {
	key, ok := canonicalSizeKey(c.Key)
	if !ok {
		return Design{}, commandIssue(string(c.Key), IssueUnsupported,
			"only span, wing area, aspect ratio and root chord are size drivers")
	}
	// A planform that does not yet hold two drivers has an empty slot, and
	// filling one is not a promotion: nothing is being given up, and there is no
	// choice for the builder to make. Only once two are held does setting a
	// third become a swap.
	if !d.holdsDriver(key) && len(d.DriverKeys()) >= plainformDriverCount {
		swaps, err := d.ValidSwaps(c.Key)
		if err != nil {
			return Design{}, err
		}
		return Design{}, commandIssue(string(c.Key), IssueInvalid,
			"this value is derived, not a driver. Promoting it releases one existing driver in the "+
				"same edit; the valid swaps are "+joinKeys(swaps))
	}
	if c.Value.supplied() {
		if err := checkDriverValue(key, c.Value); err != nil {
			return Design{}, err
		}
	} else if !d.holdsDriver(key) {
		return Design{}, commandIssue(string(c.Key), IssueInvalid,
			"there is no value here to withdraw")
	}
	out := d.clone()
	out.setSizeDriver(key, c.Value)
	return out, nil
}

// PromoteDriver makes a derived size value a driver and releases an existing
// one in the same edit, so the planform never momentarily holds one driver or
// three. Which driver is released is the builder's choice: every pair of the
// four size values is a supported solve path, so the swap is ambiguous by
// nature and is never inferred from edit order.
type PromoteDriver struct {
	// Promote names the value to make a driver.
	Promote ParameterKey
	// Release names the driver to give up. Leaving it empty asks for the valid
	// swaps rather than picking one.
	Release ParameterKey
	// Value is the promoted driver's value.
	Value Quantity
}

// Label names the edit.
func (c PromoteDriver) Label() string {
	return "promote " + string(c.Promote) + ", releasing " + string(c.Release)
}

func (c PromoteDriver) apply(d Design) (Design, error) {
	swaps, err := d.ValidSwaps(c.Promote)
	if err != nil {
		return Design{}, err
	}
	promote, _ := canonicalSizeKey(c.Promote)
	if c.Release == "" {
		return Design{}, commandIssue(string(c.Promote), IssueMissing,
			"promoting "+string(c.Promote)+" must release one existing driver in the same edit; "+
				"the valid swaps are "+joinKeys(swaps))
	}
	release, ok := canonicalSizeKey(c.Release)
	if !ok || !containsKey(swaps, d.planeSizeKey(release)) {
		return Design{}, commandIssue(string(c.Release), IssueInvalid,
			string(c.Release)+" is not a driver this promotion can release; the valid swaps are "+
				joinKeys(swaps))
	}
	if err := checkDriverValue(promote, c.Value); err != nil {
		return Design{}, err
	}
	out := d.clone()
	out.setSizeDriver(release, Quantity{})
	out.setSizeDriver(promote, c.Value)
	return out, nil
}

// plainformDriverCount is how many size drivers a planform holds. Below it
// there is an empty slot to fill; at it, adding another is a swap.
const plainformDriverCount = 2

// checkDriverValue rejects a driver value of the wrong dimension, and a
// non-positive one, before it reaches the solver.
func checkDriverValue(key ParameterKey, value Quantity) error {
	want := sizeDriverDimension(key)
	if value.dim != want {
		return commandIssue(string(key), IssueInvalid,
			"expected a "+want.String()+" value but received "+value.dim.String())
	}
	if value.si <= 0 {
		return commandIssue(string(key), IssueInvalid,
			"a size driver must be greater than zero, received "+formatFloat(value.si))
	}
	return nil
}

func containsKey(keys []ParameterKey, want ParameterKey) bool {
	for _, k := range keys {
		if k == want {
			return true
		}
	}
	return false
}

func joinKeys(keys []ParameterKey) string {
	names := make([]string, 0, len(keys))
	for _, k := range keys {
		names = append(names, string(k))
	}
	return joinNames(names)
}

// SetTaperRatio changes the taper ratio. It is not one of the two size drivers:
// a trapezoid needs it in addition to them, and a rectangle fixes it at 1.
type SetTaperRatio struct {
	// Value is lambda = c_tip/c_root.
	Value float64
}

// Label names the edit.
func (c SetTaperRatio) Label() string { return "set taper ratio to " + formatFloat(c.Value) }

func (c SetTaperRatio) apply(d Design) (Design, error) {
	if !isFinite(c.Value) || c.Value <= 0 {
		return Design{}, commandIssue(portTaperRatio.Name, IssueInvalid,
			"the taper ratio must be a positive finite number, received "+formatFloat(c.Value))
	}
	out := d.clone()
	out.Wing.Drivers.TaperRatio = c.Value
	return out, nil
}

// SetRequirement adds a requirement or replaces the one with the same name.
type SetRequirement struct {
	// Requirement is the bound to record.
	Requirement Requirement
}

// Label names the edit.
func (c SetRequirement) Label() string { return "set requirement " + c.Requirement.Name }

func (c SetRequirement) apply(d Design) (Design, error) {
	if c.Requirement.Name == "" {
		return Design{}, commandIssue("requirement", IssueMissing, "name the requirement")
	}
	out := d.clone()
	for n := range out.Requirements {
		if out.Requirements[n].Name == c.Requirement.Name {
			out.Requirements[n] = c.Requirement.clone()
			return out, nil
		}
	}
	out.Requirements = append(out.Requirements, c.Requirement.clone())
	return out, nil
}

// RemoveRequirement drops the named requirement.
type RemoveRequirement struct {
	// Name identifies the requirement.
	Name string
}

// Label names the edit.
func (c RemoveRequirement) Label() string { return "remove requirement " + c.Name }

func (c RemoveRequirement) apply(d Design) (Design, error) {
	out := d.clone()
	for n := range out.Requirements {
		if out.Requirements[n].Name != c.Name {
			continue
		}
		out.Requirements = append(out.Requirements[:n], out.Requirements[n+1:]...)
		return out, nil
	}
	return Design{}, commandIssue("requirement", IssueMissing,
		"the design defines no requirement named "+c.Name)
}

// SetRequirementPriority moves a requirement between required and preferred.
// The requirement keeps its own assessment either way; only whether it narrows
// the required feasible set changes.
type SetRequirementPriority struct {
	// Name identifies the requirement.
	Name string
	// Priority is the new priority.
	Priority Priority
}

// Label names the edit.
func (c SetRequirementPriority) Label() string {
	return "make requirement " + c.Name + " " + c.Priority.String()
}

func (c SetRequirementPriority) apply(d Design) (Design, error) {
	if c.Priority == PriorityUnknown {
		return Design{}, commandIssue("requirement."+c.Name, IssueMissing,
			"choose required or preferred")
	}
	out := d.clone()
	for n := range out.Requirements {
		if out.Requirements[n].Name == c.Name {
			out.Requirements[n].Priority = c.Priority
			return out, nil
		}
	}
	return Design{}, commandIssue("requirement", IssueMissing,
		"the design defines no requirement named "+c.Name)
}

// SetCase adds a flight case or replaces the one with the same name.
type SetCase struct {
	// Case is the case to record.
	Case DesignCase
}

// Label names the edit.
func (c SetCase) Label() string { return "set case " + c.Case.Case.Name }

func (c SetCase) apply(d Design) (Design, error) {
	if c.Case.Case.Name == "" {
		return Design{}, commandIssue("case", IssueMissing, "name the flight case")
	}
	out := d.clone()
	for n := range out.Cases {
		if out.Cases[n].Case.Name == c.Case.Case.Name {
			out.Cases[n] = c.Case
			return out, nil
		}
	}
	out.Cases = append(out.Cases, c.Case)
	return out, nil
}

// RemoveCase drops the named flight case. It refuses while a requirement still
// names the case, because a bound whose case has vanished is neither met, unmet
// nor meaningfully unknown.
type RemoveCase struct {
	// Name identifies the case.
	Name string
}

// Label names the edit.
func (c RemoveCase) Label() string { return "remove case " + c.Name }

func (c RemoveCase) apply(d Design) (Design, error) {
	for _, r := range d.Requirements {
		if r.Subject.PerCase() && r.appliesTo(c.Name) {
			return Design{}, commandIssue("case."+c.Name, IssueInvalid,
				"requirement "+r.Name+" still applies to this case; revise or remove it first")
		}
	}
	out := d.clone()
	for n := range out.Cases {
		if out.Cases[n].Case.Name != c.Name {
			continue
		}
		out.Cases = append(out.Cases[:n], out.Cases[n+1:]...)
		return out, nil
	}
	return Design{}, commandIssue("case", IssueMissing, "the design defines no case named "+c.Name)
}

// SetCasePriority moves a case between required and preferred. A preferred case
// keeps its own assessment; it stops contributing to the intersected required
// bounds.
type SetCasePriority struct {
	// Name identifies the case.
	Name string
	// Priority is the new priority.
	Priority Priority
}

// Label names the edit.
func (c SetCasePriority) Label() string { return "make case " + c.Name + " " + c.Priority.String() }

func (c SetCasePriority) apply(d Design) (Design, error) {
	if c.Priority == PriorityUnknown {
		return Design{}, commandIssue("case."+c.Name, IssueMissing, "choose required or preferred")
	}
	out := d.clone()
	for n := range out.Cases {
		if out.Cases[n].Case.Name == c.Name {
			out.Cases[n].Priority = c.Priority
			return out, nil
		}
	}
	return Design{}, commandIssue("case", IssueMissing, "the design defines no case named "+c.Name)
}

// SetCaseCLmax replaces a case's maximum lift coefficient and the grade of the
// evidence behind it. Passing the zero LiftCoefficient withdraws the evidence,
// which makes every result that depends on it unknown rather than leaving the
// previous number standing.
type SetCaseCLmax struct {
	// Case identifies the case.
	Case string
	// CLmax is the coefficient and its provenance.
	CLmax LiftCoefficient
	// Evidence grades that provenance.
	Evidence EvidenceQuality
}

// Label names the edit.
func (c SetCaseCLmax) Label() string { return "set CLmax evidence for case " + c.Case }

func (c SetCaseCLmax) apply(d Design) (Design, error) {
	out := d.clone()
	for n := range out.Cases {
		if out.Cases[n].Case.Name == c.Case {
			out.Cases[n].Case.CLmax = c.CLmax
			out.Cases[n].CLmaxEvidence = c.Evidence
			return out, nil
		}
	}
	return Design{}, commandIssue("case", IssueMissing, "the design defines no case named "+c.Case)
}

// SizeAtStallLimit sets the wing area to the smallest one the scoped stall
// ceilings allow, holding one named driver. It is the "size at the stall limit"
// action, and it must name its case scope: the all-required-cases form uses the
// intersected bound, while a single-case form uses that case alone and leaves
// every other applicable requirement to be reassessed afterwards.
//
// The area it produces is a lower bound that the builder has chosen to sit on.
// It is recorded as an ordinary area driver, so nothing downstream has to know
// a requirement was involved.
type SizeAtStallLimit struct {
	// Hold names the driver to keep. The other size driver is released and the
	// wing area takes its place.
	Hold ParameterKey
	// Scope names the cases the bound is taken from.
	Scope CaseScope
}

// Label names the edit.
func (c SizeAtStallLimit) Label() string {
	return "size at the stall limit over " + c.Scope.String() + ", holding " + string(c.Hold)
}

func (c SizeAtStallLimit) apply(d Design) (Design, error) {
	hold, ok := canonicalSizeKey(c.Hold)
	if !ok || hold == ParamAreaReference {
		return Design{}, commandIssue(string(c.Hold), IssueUnsupported,
			"hold the span, the aspect ratio or the root chord; the wing area is what this action sets")
	}
	if !d.holdsDriver(hold) {
		return Design{}, commandIssue(string(c.Hold), IssueInvalid,
			string(c.Hold)+" is not a driver of this design; its drivers are "+joinKeys(d.DriverKeys()))
	}
	bound, err := d.AreaLowerBound(c.Scope)
	if err != nil {
		return Design{}, err
	}
	if !bound.Known {
		return Design{}, commandIssue("wing_area", IssueMissing,
			"no wing area follows from "+c.Scope.String()+": "+bound.Detail)
	}
	area, err := d.areaDriverFor(bound.Value)
	if err != nil {
		return Design{}, err
	}
	out := d.clone()
	for _, key := range d.DriverKeys() {
		canonical, _ := canonicalSizeKey(key)
		if canonical != hold {
			out.setSizeDriver(canonical, Quantity{})
		}
	}
	out.setSizeDriver(ParamAreaReference, area)
	return out, nil
}

// areaDriverFor converts a plan-view reference area into the area driver of the
// plane this design's drivers are given in. Holding panel dimensions under a
// nonzero dihedral means the driver is a construction area, larger than its
// projection by 1/cos(Gamma); writing the projected number into it would size
// the wing to the wrong area.
func (d Design) areaDriverFor(projected Quantity) (Quantity, error) {
	if d.Wing.DihedralMode != DihedralHoldPanel || d.Wing.Dihedral.IsZero() {
		return projected, nil
	}
	result, err := panelArea(projected, d.Wing.Dihedral)
	if err != nil {
		return Quantity{}, err
	}
	return result.Value, nil
}

// SetPlanformShape changes the plan-view shape and the taper ratio together.
// They are one edit because they constrain each other: a rectangle has taper
// ratio 1, and a trapezoid needs one stated. Setting them separately would put
// the design through a state that is neither.
type SetPlanformShape struct {
	// Shape is the new shape.
	Shape PlanformShape
	// TaperRatio is lambda = c_tip/c_root. It must be 1 for a rectangle.
	TaperRatio float64
}

// Label names the edit.
func (c SetPlanformShape) Label() string {
	return "set the planform shape to " + c.Shape.String()
}

func (c SetPlanformShape) apply(d Design) (Design, error) {
	switch c.Shape {
	case ShapeRectangle:
		if c.TaperRatio != 1 {
			return Design{}, commandIssue(portTaperRatio.Name, IssueInvalid,
				"a rectangle has taper ratio 1; choose the trapezoid shape to taper the wing")
		}
	case ShapeTrapezoid:
		if !isFinite(c.TaperRatio) || c.TaperRatio <= 0 {
			return Design{}, commandIssue(portTaperRatio.Name, IssueInvalid,
				"a trapezoid needs a positive taper ratio; a pointed tip is not supported")
		}
	case ShapeUnknown:
		return Design{}, commandIssue("shape", IssueMissing,
			"choose a planform shape: a rectangle and a trapezoid do not have the same "+
				"independent values")
	default:
		return Design{}, commandIssue("shape", IssueUnsupported,
			"only a rectangle and a symmetric trapezoid are implemented")
	}
	out := d.clone()
	out.Wing.Drivers.Shape = c.Shape
	out.Wing.Drivers.TaperRatio = c.TaperRatio
	return out, nil
}

// SetWingAngles replaces every stated angle at once. They travel together
// because the geometry model requires all of them: an unstated sweep is a
// missing field rather than zero, so an edit that set one and left another
// unset would produce a wing that cannot solve for a reason the builder did not
// choose.
type SetWingAngles struct {
	// Sweep is the sweep angle at SweepReference, positive aft.
	Sweep Quantity
	// Dihedral is the uniform dihedral angle, positive tips up.
	Dihedral Quantity
	// Twist is the geometric twist from root to tip; washout is negative.
	Twist Quantity
	// Incidence is the root incidence against the fuselage reference line.
	Incidence Quantity
	// SweepReference is the chord fraction Sweep is measured at.
	SweepReference float64
	// DihedralMode states which dimensions stay fixed as dihedral changes. It
	// may be left unknown only at zero dihedral, where the planes coincide.
	DihedralMode DihedralMode
}

// Label names the edit.
func (c SetWingAngles) Label() string { return "set the wing angles" }

func (c SetWingAngles) apply(d Design) (Design, error) {
	rs := &resultSet{}
	for _, angle := range []struct {
		field string
		value Quantity
	}{
		{"sweep", c.Sweep}, {"dihedral", c.Dihedral},
		{"twist", c.Twist}, {"incidence", c.Incidence},
	} {
		if !angle.value.supplied() {
			rs.add(angle.field, IssueMissing,
				"state the "+angle.field+" angle, using 0 where there is none")
			continue
		}
		if angle.value.dim != DimAngle {
			rs.add(angle.field, IssueInvalid, "expected an angle but received "+angle.value.dim.String())
		}
	}
	if c.SweepReference < 0 || c.SweepReference > 1 {
		rs.add("sweep_reference", IssueInvalid,
			"the sweep reference is a chord fraction between 0 and 1, received "+
				formatFloat(c.SweepReference))
	}
	if len(rs.issues) > 0 {
		return Design{}, rs.issues
	}
	out := d.clone()
	out.Wing.Sweep = c.Sweep
	out.Wing.Dihedral = c.Dihedral
	out.Wing.Twist = c.Twist
	out.Wing.Incidence = c.Incidence
	out.Wing.SweepReference = c.SweepReference
	out.Wing.DihedralMode = c.DihedralMode
	return out, nil
}

// SetConfiguration changes the airframe layout and its tail description
// together. The two are one edit because a layout without its matching
// description is not a design any check can read: a flying wing with a tail and
// a conventional aircraft without one are both incomplete rather than merely
// unsaved.
type SetConfiguration struct {
	// Tail is the description the configuration calls for. A flying wing takes
	// the zero value.
	Tail TailGeometry
	// Configuration names the layout.
	Configuration Configuration
}

// Label names the edit.
func (c SetConfiguration) Label() string {
	return "set the configuration to " + c.Configuration.String()
}

func (c SetConfiguration) apply(d Design) (Design, error) {
	if c.Configuration == ConfigurationUnknown {
		return Design{}, commandIssue("configuration", IssueMissing,
			"choose the configuration: conventional, V-tail and flying wing do not share a tail "+
				"description or a handling model")
	}
	out := d.clone()
	out.Configuration = c.Configuration
	out.Tail = c.Tail
	return out, nil
}
