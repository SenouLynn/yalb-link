package calculator

// AuxiliaryContribution is one auxiliary load's draw as the pack supplies it,
// with the conversion that produced it when one was needed.
type AuxiliaryContribution struct {
	// Name identifies the load.
	Name string
	// Component echoes the mass item the load belongs to, when one is named.
	Component string
	// Detail explains a contribution that could not be made.
	Detail string
	// ContinuousTrace records the pack-side conversion of the continuous draw.
	// It is the zero Trace for a figure already measured on the pack side, where
	// nothing was converted.
	ContinuousTrace Trace
	// PeakTrace records the same conversion for the peak draw.
	PeakTrace Trace
	// Continuous is the continuous draw as the pack supplies it.
	Continuous Quantity
	// Peak is the peak draw as the pack supplies it.
	Peak Quantity
	// Side echoes which side of the regulator the entered figures were on.
	Side RegulatorSide
	// Evidence grades the basis behind the figures.
	Evidence EvidenceQuality
	// Known reports whether the load contributed.
	Known bool
}

// SupplyCheck is one component rating compared against what the design demands.
// A rating that was not stated produces an unknown check rather than none, so
// an unchecked limit is visible instead of silently absent.
type SupplyCheck struct {
	// Name identifies the rating.
	Name string
	// Detail explains an unknown or unmet outcome.
	Detail string
	// Limit is the rating.
	Limit Quantity
	// Actual is what the design demands of it.
	Actual Quantity
	// Margin is the relative room inside the rating, negative when exceeded.
	Margin float64
	// Status is met, unmet or unknown.
	Status LimitStatus
}

// ElectricalBudget is the non-propulsive electrical demand: what the avionics,
// servos, sensors and payload draw continuously, and what they draw at once.
//
// The two totals answer different questions and are never merged. The
// continuous total is what the mission's energy budget pays for; the peak total
// is what the pack and the regulators have to survive. An aircraft whose energy
// fits comfortably can still brown out its receiver when every servo moves.
type ElectricalBudget struct {
	// Detail explains an incomplete or unavailable result.
	Detail string
	// Traces record the pack-side conversions and the two sums.
	Traces []Trace
	// Loads are every listed load, in design order.
	Loads []AuxiliaryContribution
	// Continuous is the total continuous draw on the pack side.
	Continuous Quantity
	// Peak is the total peak draw on the pack side.
	Peak Quantity
	// Evidence grades the weakest evidence any contributing load rests on.
	Evidence EvidenceQuality
	// Status reports whether a total was established at all.
	Status ResultStatus
	// Complete reports that every listed load contributed.
	Complete bool
}

// ElectricalBudget sums the auxiliary demand on the pack side of every
// regulator.
//
// An empty list is a missing answer, not a zero one. An aircraft with an
// autopilot, a receiver and servos draws power whether or not anyone wrote it
// down, and treating silence as zero is how a design passes an electrical check
// it has never actually been subjected to. A builder with genuinely nothing to
// add says so by listing a load of zero watts with that as its basis, which is
// the same "spell the zero out" rule the wing angles follow.
func (d Design) ElectricalBudget() ElectricalBudget {
	budget := ElectricalBudget{Complete: true, Evidence: EvidenceMeasured}
	if len(d.Auxiliary) == 0 {
		budget.Status = ResultMissing
		budget.Complete = false
		budget.Detail = "no avionics, servo, sensor or payload demand is listed, so no complete " +
			"electrical figure follows and no feasibility claim can be made. An aircraft with " +
			"none of these says so by listing a load of zero watts with that as its basis"
		return budget
	}
	continuous := newEvaluation(EqAuxiliarySum)
	peak := newEvaluation(EqAuxiliarySum)
	var continuousTotal, peakTotal float64
	contributing := 0
	for _, load := range d.Auxiliary {
		contribution := auxiliaryContribution(load)
		budget.Loads = append(budget.Loads, contribution)
		if !contribution.Known {
			budget.Complete = false
			continue
		}
		contributing++
		budget.Evidence = weakerEvidence(budget.Evidence, load.Evidence)
		budget.Traces = appendTrace(budget.Traces, contribution.ContinuousTrace)
		budget.Traces = appendTrace(budget.Traces, contribution.PeakTrace)
		continuousTotal += continuous.signedQuantity(portPackSidePower.Name, contribution.Continuous)
		if contribution.Peak.supplied() {
			peakTotal += peak.signedQuantity(portPackSidePower.Name, contribution.Peak)
		}
	}
	if contributing == 0 {
		budget.Status = ResultMissing
		budget.Detail = "no listed load produced a pack-side draw, so there is no auxiliary total"
		return budget
	}
	budget.finish(continuous, peak, continuousTotal, peakTotal)
	return budget
}

// finish completes the two sums. The continuous total may legitimately be zero,
// for a design that has stated it draws nothing, so it is finished as a signed
// result; the peak total is only reported when a peak was stated somewhere.
func (b *ElectricalBudget) finish(continuous, peak *evaluation, continuousTotal, peakTotal float64) {
	result, err := continuous.finishSigned(continuousTotal)
	if err != nil {
		b.Status = statusForError(err)
		b.Detail = "the continuous draws do not sum to a usable total: " + err.Error()
		return
	}
	b.Continuous = result.Value
	b.Traces = append(b.Traces, result.Trace)
	if peakTotal > 0 {
		peakResult, peakErr := peak.finishSigned(peakTotal)
		if peakErr == nil {
			b.Peak = peakResult.Value
			b.Traces = append(b.Traces, peakResult.Trace)
		} else {
			b.Detail = "the peak draws do not sum to a usable total: " + peakErr.Error()
		}
	}
	b.Status = ResultComputed
	if !b.Complete {
		b.Detail = appendDetail(b.Detail,
			"this covers only the loads that produced a pack-side draw, so it understates the "+
				"aircraft's demand rather than describing it")
	}
}

// appendTrace adds a trace unless it is the zero Trace, which is what a
// pack-side figure that needed no conversion leaves behind.
func appendTrace(traces []Trace, trace Trace) []Trace {
	if trace.EquationID == "" {
		return traces
	}
	return append(traces, trace)
}

// auxiliaryContribution converts one load's figures onto the pack side.
func auxiliaryContribution(load AuxiliaryLoad) AuxiliaryContribution {
	contribution := AuxiliaryContribution{
		Name:      load.Name,
		Component: load.Component,
		Side:      load.Side,
		Evidence:  load.Evidence,
	}
	if !load.Continuous.supplied() {
		contribution.Detail = "no continuous draw is stated for " + load.Name + ", so it " +
			"contributes nothing; it is not treated as drawing zero"
		return contribution
	}
	if load.Side == RegulatorSideUnknown {
		contribution.Detail = "the draw for " + load.Name + " does not say which side of its " +
			"regulator it was measured on, so it cannot be converted onto the pack side without " +
			"counting the regulator's loss twice or not at all"
		return contribution
	}
	continuous, err := load.packSide(load.Continuous)
	if err != nil {
		contribution.Detail = "the pack-side continuous draw for " + load.Name +
			" could not be evaluated: " + err.Error()
		return contribution
	}
	contribution.Continuous = continuous.Value
	contribution.ContinuousTrace = continuous.Trace
	if load.Peak.supplied() {
		peak, peakErr := load.packSide(load.Peak)
		if peakErr != nil {
			contribution.Detail = "the pack-side peak draw for " + load.Name +
				" could not be evaluated: " + peakErr.Error()
			return contribution
		}
		contribution.Peak = peak.Value
		contribution.PeakTrace = peak.Trace
	}
	contribution.Known = true
	return contribution
}

// PowerFeasibility is the component-level electrical and mechanical check: what
// the design demands of the propulsion chain and the pack, against the ratings
// the builder entered for them.
//
// It is separate from the mission's energy budget on purpose. Whether the
// energy fits and whether the hardware survives delivering it are different
// questions, and the acceptance case this separation exists for is the mission
// that passes on energy and fails on peak supply.
type PowerFeasibility struct {
	// Detail explains an incomplete result.
	Detail string
	// PeakDetail says how the peak demand was formed, because a worst case built
	// from two separate segments' figures needs its construction stated.
	PeakDetail string
	// Checks are the ratings considered, in a stable order.
	Checks []SupplyCheck
	// ContinuousDemand is the largest electrical power any single segment holds
	// continuously, auxiliary draw included.
	ContinuousDemand Quantity
	// PeakDemand is the largest propulsion draw of any segment plus the peak
	// auxiliary draw: the worst moment the pack is asked to survive.
	PeakDemand Quantity
	// Status reports whether a demand was established at all.
	Status ResultStatus
}

// PowerFeasibility compares the design's electrical demand with its component
// ratings. It takes the mission and the auxiliary budget rather than recomputing
// them, so that the numbers a worksheet shows in the two places cannot differ.
func (d Design) PowerFeasibility(mission MissionResult, auxiliary ElectricalBudget) PowerFeasibility {
	feasibility := PowerFeasibility{}
	if mission.Status != ResultComputed {
		feasibility.Status = mission.Status
		feasibility.Detail = "no mission demand is established, so nothing is compared with the " +
			"component ratings: " + mission.Detail
		return feasibility
	}
	feasibility.ContinuousDemand = mission.PeakContinuousPower
	feasibility.resolvePeak(mission, auxiliary)
	feasibility.Status = ResultComputed
	limits := d.Propulsion.Limits
	feasibility.add("motor and controller continuous power",
		limits.MaxContinuousElectricalPower, feasibility.ContinuousDemand, "")
	feasibility.add("motor and controller peak power",
		limits.MaxPeakElectricalPower, feasibility.PeakDemand, feasibility.PeakDetail)
	d.addCurrentChecks(&feasibility)
	d.addRotationAndVoltageChecks(&feasibility)
	return feasibility
}

// resolvePeak forms the worst moment: the segment that draws the most from the
// propulsion chain, at the same time as every auxiliary load peaks.
func (f *PowerFeasibility) resolvePeak(mission MissionResult, auxiliary ElectricalBudget) {
	var propulsion float64
	var from string
	for n := range mission.Segments {
		segment := &mission.Segments[n]
		if segment.Status != ResultComputed {
			continue
		}
		if segment.Power.PropulsiveElectrical.si > propulsion {
			propulsion = segment.Power.PropulsiveElectrical.si
			from = segment.Name
		}
	}
	if !auxiliary.Peak.supplied() {
		f.PeakDetail = "no auxiliary load states a peak draw, so no worst-moment demand is formed"
		return
	}
	f.PeakDemand = Quantity{si: propulsion + auxiliary.Peak.si, dim: DimPower}
	f.PeakDetail = "the propulsion draw of segment " + from + " together with every auxiliary " +
		"load at its peak. It is a constructed worst case: no segment was evaluated with the " +
		"servos moving, and nothing here says the two coincide in flight"
}

// addCurrentChecks compares the pack's current ratings with the current the
// demand implies at its nominal voltage.
func (d Design) addCurrentChecks(f *PowerFeasibility) {
	voltage := d.Battery.NominalVoltage
	if !voltage.supplied() {
		f.Checks = append(f.Checks, SupplyCheck{
			Name: "pack continuous current",
			Detail: "the pack states no nominal voltage, so a power demand implies no current " +
				"and the pack's current ratings cannot be checked",
		}, SupplyCheck{
			Name:   "pack peak current",
			Detail: "the pack states no nominal voltage, so its current ratings cannot be checked",
		})
		return
	}
	f.addCurrent("pack continuous current", d.Battery.ContinuousCurrentLimit, f.ContinuousDemand, voltage, "")
	f.addCurrent("pack peak current", d.Battery.PeakCurrentLimit, f.PeakDemand, voltage, f.PeakDetail)
}

func (f *PowerFeasibility) addCurrent(name string, limit, demand, voltage Quantity, note string) {
	if !demand.supplied() {
		f.Checks = append(f.Checks, SupplyCheck{
			Name: name, Limit: limit, Detail: "no demand is established to compare",
		})
		return
	}
	current := Quantity{si: demand.si / voltage.si, dim: DimCurrent}
	detail := "the current is taken at the pack's nominal voltage " + voltage.String() +
		"; under load the voltage sags and the real current is higher"
	if note != "" {
		detail = note + ". " + detail
	}
	f.add(name, limit, current, detail)
}

// addRotationAndVoltageChecks covers the ratings that are about the hardware
// rather than about the mission: the rotation rate the capability points were
// taken at, the pack voltage the controller sees, and the propeller's clearance.
func (d Design) addRotationAndVoltageChecks(f *PowerFeasibility) {
	limits := d.Propulsion.Limits
	var highestRPM Quantity
	var highestVoltage Quantity
	for n := range d.Propulsion.Capabilities {
		capability := &d.Propulsion.Capabilities[n]
		if capability.RPM.supplied() && capability.RPM.si > highestRPM.si {
			highestRPM = capability.RPM
		}
		if capability.Voltage.supplied() && capability.Voltage.si > highestVoltage.si {
			highestVoltage = capability.Voltage
		}
	}
	f.add("propeller and motor rotation rate", limits.MaxRPM, highestRPM,
		"the highest rotation rate any capability point states. A point that states none is not "+
			"covered by this check")
	f.add("controller and motor voltage", limits.MaxVoltage, highestVoltage,
		"the highest pack voltage any capability point states")
}

// add records one rating check, reporting an unstated rating or an unavailable
// demand as unknown rather than omitting the row.
func (f *PowerFeasibility) add(name string, limit, actual Quantity, note string) {
	check := SupplyCheck{Name: name, Limit: limit, Actual: actual, Detail: note}
	switch {
	case !limit.supplied():
		check.Status = LimitUnknown
		check.Detail = appendDetail(note, "no rating is stated for this, so it is unchecked")
	case !actual.supplied():
		check.Status = LimitUnknown
		check.Detail = appendDetail(note, "no demand is established to compare with the rating")
	default:
		slack := requirementTolerance * limit.si
		check.Status = statusFor(actual.si <= limit.si+slack)
		check.Margin = relativeMargin(limit.si-actual.si, limit.si)
		if check.Status == LimitUnmet {
			check.Detail = appendDetail(note,
				actual.String()+" exceeds the rating of "+limit.String())
		}
	}
	f.Checks = append(f.Checks, check)
}

// ThrustCheck is one thrust-to-weight target compared with what a named
// capability point delivers at its own condition.
type ThrustCheck struct {
	// Name identifies the target.
	Name string
	// Capability names the point the target is stated at.
	Capability string
	// Condition describes that point's condition, so a reader never has to look
	// it up to know what the ratio means.
	Condition string
	// Detail explains an unknown or unmet outcome.
	Detail string
	// Trace records the thrust-to-weight evaluation.
	Trace Trace
	// Available is the thrust-to-weight the capability point delivers.
	Available float64
	// Target is the ratio the builder requires.
	Target float64
	// Margin is the relative room above the target, negative when short.
	Margin float64
	// Priority states whether the target must hold.
	Priority Priority
	// Evidence grades the capability point.
	Evidence EvidenceQuality
	// Status is met, unmet or unknown.
	Status LimitStatus
}

// ThrustChecks compares every thrust-to-weight target with the capability point
// it names.
//
// The target and the available thrust stay separate all the way through. The
// target is what the builder wants; the point is what somebody measured. Naming
// the point in the target is what stops a launch requirement from being answered
// with a cruise measurement, and a static point is only ever compared with a
// target stated at a static condition.
func (d Design) ThrustChecks() []ThrustCheck {
	mass := d.massReading()
	checks := make([]ThrustCheck, 0, len(d.Propulsion.Targets))
	for _, target := range d.Propulsion.Targets {
		check := ThrustCheck{
			Name:       target.Name,
			Capability: target.Capability,
			Target:     target.Ratio,
			Priority:   target.Priority,
		}
		capability, ok := d.Propulsion.Capability(target.Capability)
		if !ok {
			check.Detail = "the design defines no capability point named " + target.Capability
			checks = append(checks, check)
			continue
		}
		check.Evidence = capability.Evidence
		check.Condition = describeCondition(capability)
		if mass.Status != ResultComputed {
			check.Detail = "no all-up mass is established, so no thrust-to-weight follows: " + mass.Detail
			checks = append(checks, check)
			continue
		}
		result, err := ThrustToWeight(capability.Thrust, mass.Value)
		if err != nil {
			check.Detail = "the available thrust-to-weight could not be evaluated: " + err.Error()
			checks = append(checks, check)
			continue
		}
		check.Available = result.Value.si
		check.Trace = result.Trace
		slack := requirementTolerance * target.Ratio
		check.Status = statusFor(check.Available >= target.Ratio-slack)
		check.Margin = relativeMargin(check.Available-target.Ratio, target.Ratio)
		if check.Status == LimitUnmet {
			check.Detail = "capability point " + target.Capability + " gives a thrust-to-weight of " +
				formatFloat(check.Available) + " at " + check.Condition + ", below the target of " +
				formatFloat(target.Ratio)
		}
		checks = append(checks, check)
	}
	return checks
}

// describeCondition renders a capability point's condition in one line, so a
// ratio is never shown without the condition that gives it meaning.
func describeCondition(c PropulsionCapability) string {
	condition := c.Kind.String()
	if c.Speed.supplied() {
		condition += " at " + c.Speed.String()
	}
	if c.Voltage.supplied() {
		condition += ", " + c.Voltage.String()
	}
	if c.RPM.supplied() {
		condition += ", " + c.RPM.String()
	}
	if c.Throttle > 0 {
		condition += ", throttle " + formatFloat(c.Throttle)
	}
	return condition
}
