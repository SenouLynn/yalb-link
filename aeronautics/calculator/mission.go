package calculator

import "sort"

// SegmentKind names what a mission segment is for. It carries no physics: no
// kind implies a speed, a duration, a power or a climb angle, and nothing in
// this package reads a kind to decide a number. It exists so a worksheet can
// group and label the mission the way a builder describes it.
type SegmentKind uint8

const (
	// SegmentKindUnknown is the zero value and is never accepted.
	SegmentKindUnknown SegmentKind = iota
	// SegmentLaunch is the launch or takeoff run.
	SegmentLaunch
	// SegmentClimb is a climb to the working altitude.
	SegmentClimb
	// SegmentCruise is transit at a chosen speed.
	SegmentCruise
	// SegmentLoiter is time spent on station.
	SegmentLoiter
	// SegmentReturn is the leg home, which is where a headwind is paid for.
	SegmentReturn
	// SegmentRecovery is the descent, approach and landing.
	SegmentRecovery
	// SegmentOther is anything the kinds above do not name.
	SegmentOther
)

var segmentKindNames = [...]string{
	SegmentKindUnknown: "unknown",
	SegmentLaunch:      "launch",
	SegmentClimb:       "climb",
	SegmentCruise:      "cruise",
	SegmentLoiter:      "loiter",
	SegmentReturn:      "return",
	SegmentRecovery:    "recovery",
	SegmentOther:       "other",
}

// String returns the kind's readable name.
func (k SegmentKind) String() string {
	if int(k) < len(segmentKindNames) {
		return segmentKindNames[k]
	}
	return "unknown"
}

// SegmentModel selects where a segment's electrical power comes from.
type SegmentModel uint8

const (
	// SegmentModelUnknown is the zero value and is never accepted.
	SegmentModelUnknown SegmentModel = iota
	// SegmentModelPolar computes the power from the drag polar, the steady
	// force balance and the propulsion chain efficiency.
	SegmentModelPolar
	// SegmentModelEntered takes an electrical power the builder supplies. It is
	// how a launch or a recovery is budgeted for while no model covers it, and
	// the result carries the evidence grade of that estimate rather than the
	// authority of a computed one.
	SegmentModelEntered
)

var segmentModelNames = [...]string{
	SegmentModelUnknown: "unknown",
	SegmentModelPolar:   "drag polar",
	SegmentModelEntered: "entered estimate",
}

// String returns the model's readable name.
func (m SegmentModel) String() string {
	if int(m) < len(segmentModelNames) {
		return segmentModelNames[m]
	}
	return "unknown"
}

// SegmentTiming selects whether a segment is stated as a duration or as a
// ground distance. Exactly one is stated and the other follows, because stating
// both would be two answers to one question and would silently disagree the
// moment the wind changed.
type SegmentTiming uint8

const (
	// SegmentTimingUnknown is the zero value and is never accepted.
	SegmentTimingUnknown SegmentTiming = iota
	// SegmentTimingDuration states how long the segment lasts; the distance
	// follows from the ground speed.
	SegmentTimingDuration
	// SegmentTimingDistance states how far the segment covers over the ground;
	// the duration follows from the ground speed.
	SegmentTimingDistance
)

var segmentTimingNames = [...]string{
	SegmentTimingUnknown:  "unknown",
	SegmentTimingDuration: "duration",
	SegmentTimingDistance: "ground distance",
}

// String returns the timing's readable name.
func (t SegmentTiming) String() string {
	if int(t) < len(segmentTimingNames) {
		return segmentTimingNames[t]
	}
	return "unknown"
}

// MissionSegment is one editable leg of a mission: the condition it is flown
// at, how long it lasts or how far it goes, where its electrical power comes
// from, and optionally which capability point its thrust is checked against.
//
// Energy and flight feasibility are separate answers throughout. A segment can
// contribute a perfectly good energy figure and still name no capability point,
// in which case whether the aircraft can actually fly it is unknown rather than
// assumed; that is the honest state of an autopilot-flown return leg for which
// nobody has measured the thrust.
type MissionSegment struct {
	// Name identifies the segment within the mission.
	Name string
	// Case names the flight case supplying this segment's air density, its CLmax
	// and its load factor, so that a segment cannot rest on an unstated
	// atmosphere.
	Case string
	// Notes records anything about the segment a reader needs.
	Notes string
	// EnteredBasis states where an entered power estimate came from.
	EnteredBasis string
	// EfficiencyBasis states where a per-segment chain efficiency came from.
	EfficiencyBasis string
	// Capability optionally names the propulsion capability point this
	// segment's required thrust is checked against.
	Capability string
	// Speed is the true airspeed flown.
	Speed Quantity
	// ClimbAngle is the flight-path angle, positive climbing. It must be stated,
	// with zero spelled out for level flight.
	ClimbAngle Quantity
	// Duration is how long the segment lasts, for SegmentTimingDuration.
	Duration Quantity
	// Distance is the ground distance covered, for SegmentTimingDistance.
	Distance Quantity
	// WindAlongTrack is the wind component along the ground track, positive as a
	// tailwind. It must be stated, with zero spelled out for still air.
	WindAlongTrack Quantity
	// EnteredPower is the electrical power estimate, for SegmentModelEntered.
	EnteredPower Quantity
	// Efficiency optionally overrides the design's chain efficiency for this
	// segment. It is a stated value with its own basis, not a default.
	Efficiency float64
	// Kind names what the segment is for.
	Kind SegmentKind
	// Model selects where the electrical power comes from.
	Model SegmentModel
	// Timing selects whether the duration or the distance is stated.
	Timing SegmentTiming
	// EnteredEvidence grades an entered power estimate.
	EnteredEvidence EvidenceQuality
}

// Mission is the flight the design is judged against: its segments, in order,
// and the energy reserve held back from the pack.
type Mission struct {
	// Name identifies the mission.
	Name string
	// Basis states where the mission came from.
	Basis string
	// ReserveBasis states where the reserve fraction came from.
	ReserveBasis string
	// Segments are the legs, in the order they are flown.
	Segments []MissionSegment
	// ReserveFraction is the fraction of the pack's usable energy held back. It
	// is applied exactly once, to the usable energy, and never inside a segment.
	ReserveFraction float64
}

func (m Mission) clone() Mission {
	c := m
	c.Segments = append([]MissionSegment(nil), m.Segments...)
	return c
}

func (m Mission) supplied() bool { return len(m.Segments) > 0 || m.Name != "" }

// Segment returns the mission segment with the given name.
func (m Mission) Segment(name string) (MissionSegment, bool) {
	for n := range m.Segments {
		if m.Segments[n].Name == name {
			return m.Segments[n], true
		}
	}
	return MissionSegment{}, false
}

// segmentNames returns every segment name, sorted.
func (m Mission) segmentNames() []string {
	names := make([]string, 0, len(m.Segments))
	for n := range m.Segments {
		names = append(names, m.Segments[n].Name)
	}
	sort.Strings(names)
	return names
}

func (m Mission) validate(d Design, rs *resultSet) {
	if !m.supplied() {
		return
	}
	if m.Name == "" {
		rs.add("mission", IssueMissing, "name the mission")
	}
	if m.Basis == "" {
		rs.add("mission", IssueMissing,
			"state where the mission came from, so an illustrative profile is not read as a "+
				"required one")
	}
	if m.ReserveFraction < 0 || m.ReserveFraction >= 1 || !isFinite(m.ReserveFraction) {
		rs.add("mission.reserve", IssueInvalid,
			"the reserve is a fraction of the usable energy from 0 up to but not including 1, "+
				"received "+formatFloat(m.ReserveFraction))
	}
	if m.ReserveBasis == "" {
		rs.add("mission.reserve", IssueMissing,
			"state where the reserve came from, with zero spelled out for a mission that holds "+
				"nothing back")
	}
	if len(m.Segments) == 0 {
		rs.add("mission", IssueMissing, "a mission needs at least one segment")
	}
	seen := make(map[string]bool, len(m.Segments))
	for n := range m.Segments {
		s := m.Segments[n]
		if seen[s.Name] && s.Name != "" {
			rs.add("segment."+s.Name, IssueInvalid, "the mission lists this segment twice")
		}
		seen[s.Name] = true
		s.validate(d, rs)
	}
}

func (s MissionSegment) validate(d Design, rs *resultSet) {
	field := "segment." + s.Name
	if s.Name == "" {
		rs.add("segment", IssueMissing, "name every mission segment so its result can be reported")
		return
	}
	if s.Kind == SegmentKindUnknown {
		rs.add(field, IssueMissing,
			"say what the segment is for; a kind groups and labels it and implies no value")
	}
	s.validateCondition(d, field, rs)
	s.validateTiming(field, rs)
	s.validateModel(d, field, rs)
	if s.Capability != "" {
		if _, ok := d.Propulsion.Capability(s.Capability); !ok {
			rs.add(field, IssueMissing,
				"names capability point "+s.Capability+", which the design does not define; it "+
					"defines "+joinNames(d.Propulsion.capabilityNames()))
		}
	}
}

func (s MissionSegment) validateCondition(d Design, field string, rs *resultSet) {
	if s.Case == "" {
		rs.add(field, IssueMissing,
			"name the flight case this segment is flown in, so its air density, its CLmax and "+
				"its load factor are stated rather than assumed")
	} else if _, ok := d.Case(s.Case); !ok {
		rs.add(field, IssueMissing,
			"names case "+s.Case+", which the design does not define; it defines "+
				joinNames(d.caseNames()))
	}
	if !s.Speed.supplied() {
		rs.add(field, IssueMissing, "state the true airspeed the segment is flown at")
	}
	if !s.ClimbAngle.supplied() {
		rs.add(field, IssueMissing,
			"state the flight-path angle, using 0 for level flight; an unstated angle is a "+
				"missing field rather than a claim that the segment is level")
	}
	if !s.WindAlongTrack.supplied() {
		rs.add(field, IssueMissing,
			"state the wind along the ground track, using 0 for still air and a negative value "+
				"for a headwind; a return leg's range depends on it")
	}
}

func (s MissionSegment) validateTiming(field string, rs *resultSet) {
	switch s.Timing {
	case SegmentTimingDuration:
		if !s.Duration.supplied() {
			rs.add(field, IssueMissing, "state how long the segment lasts")
		}
		if s.Distance.supplied() {
			rs.add(field, IssueInvalid,
				"this segment is stated as a duration, so a ground distance would be a second "+
					"answer to the same question; the distance follows from the ground speed")
		}
	case SegmentTimingDistance:
		if !s.Distance.supplied() {
			rs.add(field, IssueMissing, "state the ground distance the segment covers")
		}
		if s.Duration.supplied() {
			rs.add(field, IssueInvalid,
				"this segment is stated as a ground distance, so a duration would be a second "+
					"answer to the same question; the duration follows from the ground speed")
		}
	default:
		rs.add(field, IssueMissing,
			"say whether the segment is stated as a duration or as a ground distance")
	}
}

func (s MissionSegment) validateModel(d Design, field string, rs *resultSet) {
	switch s.Model {
	case SegmentModelPolar:
		s.validatePolarModel(d, field, rs)
	case SegmentModelEntered:
		s.validateEnteredModel(field, rs)
	default:
		rs.add(field, IssueMissing,
			"say where the segment's electrical power comes from: the drag polar, or an entered "+
				"estimate")
	}
	s.validateEfficiencyOverride(field, rs)
}

// validatePolarModel checks that a computed segment has the polar and the chain
// efficiency it needs, and no entered power that would be a second answer.
func (s MissionSegment) validatePolarModel(d Design, field string, rs *resultSet) {
	if s.EnteredPower.supplied() {
		rs.add(field, IssueInvalid,
			"this segment's power comes from the drag polar, so an entered power would be a "+
				"second answer to the same question")
	}
	if !d.Polar.supplied() {
		rs.add(field, IssueMissing,
			"this segment's power comes from the drag polar, and the design states none")
	}
	if s.Efficiency == 0 && !d.Propulsion.Efficiency.supplied() {
		rs.add(field, IssueMissing,
			"an electrical estimate needs a propeller, motor and speed-controller chain "+
				"efficiency; state one for the design or for this segment")
	}
}

// validateEnteredModel checks that an estimate standing in for a missing model
// is visible as one.
func (s MissionSegment) validateEnteredModel(field string, rs *resultSet) {
	if !s.EnteredPower.supplied() {
		rs.add(field, IssueMissing, "state the electrical power this segment is estimated at")
	}
	if s.EnteredBasis == "" {
		rs.add(field, IssueMissing,
			"state where the entered power estimate came from; an estimate standing in for a "+
				"missing model has to be visible as one")
	}
	if s.EnteredEvidence == EvidenceUnstated {
		rs.add(field, IssueMissing, "grade the evidence behind the entered power estimate")
	}
}

// validateEfficiencyOverride checks a per-segment chain efficiency, which is a
// stated value with its own basis rather than a default.
func (s MissionSegment) validateEfficiencyOverride(field string, rs *resultSet) {
	if s.Efficiency == 0 {
		return
	}
	if s.Efficiency < 0 || s.Efficiency > 1 || !isFinite(s.Efficiency) {
		rs.add(field, IssueInvalid,
			"a chain efficiency is a fraction greater than zero and at most one, received "+
				formatFloat(s.Efficiency))
	}
	if s.EfficiencyBasis == "" {
		rs.add(field, IssueMissing,
			"state where this segment's chain efficiency came from")
	}
}

// SegmentAvailability is whether the propulsion system was measured to deliver
// the thrust a segment needs, at that segment's condition.
type SegmentAvailability struct {
	// Capability names the point the check was made against.
	Capability string
	// Detail explains an unknown or unmet outcome.
	Detail string
	// Trace records the thrust-to-weight evaluation at the capability point.
	Trace Trace
	// AvailableThrust is the thrust the point delivers.
	AvailableThrust Quantity
	// Margin is the relative room between the required and available thrust,
	// negative when the segment asks for more than the point delivers.
	Margin float64
	// Status is met, unmet or unknown.
	Status LimitStatus
	// Evidence grades the capability point behind AvailableThrust.
	Evidence EvidenceQuality
}

// SegmentPower is the power one segment requires and the chain of evaluations
// that produced it.
type SegmentPower struct {
	// Detail explains a power that could not be established.
	Detail string
	// Traces record every relationship evaluated, in evaluation order.
	Traces []Trace
	// DynamicPressure is q at the segment's condition.
	DynamicPressure Quantity
	// Lift is the lift the steady balance demands.
	Lift Quantity
	// Drag is the drag at the resulting lift coefficient.
	Drag Quantity
	// Thrust is the thrust the steady balance demands along the flight path.
	Thrust Quantity
	// Propulsive is the useful propulsive power, thrust times airspeed.
	Propulsive Quantity
	// PropulsiveElectrical is the electrical power the propulsion chain draws,
	// before any auxiliary load. It is held separately so a peak demand can add
	// the peak auxiliary draw to it rather than to a figure that already
	// contains the continuous one.
	PropulsiveElectrical Quantity
	// Electrical is the total electrical draw: the propulsion chain plus the
	// continuous auxiliary load.
	Electrical Quantity
	// LiftCoefficient is the coefficient the condition demands.
	LiftCoefficient float64
	// DragCoefficient is the polar's answer at that lift coefficient.
	DragCoefficient float64
	// LiftToDrag is CL/CD at the condition.
	LiftToDrag float64
	// ChainEfficiency is the efficiency actually used.
	ChainEfficiency float64
	// Model records where the electrical power came from.
	Model SegmentModel
	// Evidence grades the weakest evidence the result rests on.
	Evidence EvidenceQuality
	// Status reports whether the model produced the electrical power at all.
	Status ResultStatus
}

// SegmentResult is one mission segment fully evaluated: its timing, its power,
// its energy and whether the propulsion system was shown to be able to fly it.
type SegmentResult struct {
	// Name identifies the segment.
	Name string
	// Case names the flight case it was evaluated in.
	Case string
	// Detail explains a segment that did not contribute.
	Detail string
	// Traces record the timing and energy evaluations.
	Traces []Trace
	// Availability is the thrust check, when a capability point was named.
	Availability SegmentAvailability
	// Power is the power the segment requires.
	Power SegmentPower
	// GroundSpeed is the speed made good along the track.
	GroundSpeed Quantity
	// Duration is how long the segment lasts.
	Duration Quantity
	// Distance is the ground distance it covers.
	Distance Quantity
	// Energy is the energy it costs.
	Energy Quantity
	// Kind names what the segment is for.
	Kind SegmentKind
	// Status reports whether the segment contributed an energy at all.
	Status ResultStatus
}

// MissionResult is the mission's energy budget: what every segment costs, what
// the pack can supply after its reserve, and whether the two are compatible.
//
// Energy sufficiency and flight feasibility are reported separately and neither
// implies the other. A mission whose energy fits is not thereby flyable, and a
// mission every segment of which has a thrust margin is not thereby within the
// pack's energy.
type MissionResult struct {
	// Detail explains an incomplete or unavailable result.
	Detail string
	// Traces record the mission-level sums and the budget evaluation.
	Traces []Trace
	// Segments are every segment considered, in mission order.
	Segments []SegmentResult
	// RequiredEnergy is the sum of the segment energies.
	RequiredEnergy Quantity
	// UsableEnergy is what the pack may deliver, before the reserve.
	UsableEnergy Quantity
	// Budget is the usable energy less the reserve, applied exactly once.
	Budget Quantity
	// TotalDuration is the sum of the segment durations.
	TotalDuration Quantity
	// TotalDistance is the sum of the segment ground distances.
	TotalDistance Quantity
	// PeakContinuousPower is the largest electrical power any single segment
	// holds continuously.
	PeakContinuousPower Quantity
	// ReserveFraction echoes the reserve that was applied.
	ReserveFraction float64
	// EnergyStatus reports whether the required energy fits the budget. It is
	// unknown when either side could not be established.
	EnergyStatus LimitStatus
	// Status reports whether the mission produced a required energy at all.
	Status ResultStatus
	// Complete reports that every listed segment contributed. A false value with
	// a computed status means the energy figure covers a subset of the mission,
	// which is an understatement rather than a budget.
	Complete bool
}

// MissionAnalysis evaluates every segment, sums the energy the mission needs,
// and compares it with what the pack may spend.
//
// It never adds a segment the builder did not state. A return leg is a segment,
// a reserve is a fraction, and an autopilot's own return setting is neither: it
// establishes no wind-aware range on its own, and nothing here invents one.
func (d Design) MissionAnalysis() MissionResult {
	result := MissionResult{ReserveFraction: d.Mission.ReserveFraction, Complete: true}
	if len(d.Mission.Segments) == 0 {
		result.Status = ResultMissing
		result.Complete = false
		result.Detail = "the design states no mission segments, so there is nothing to budget for"
		return result
	}
	context := d.powerContext()
	auxiliary := d.ElectricalBudget()
	energySum := newEvaluation(EqEnergySum)
	durationSum := newEvaluation(EqDurationSum)
	distanceSum := newEvaluation(EqDistanceSum)
	var energy, duration, distance, peak float64
	contributing := 0
	for n := range d.Mission.Segments {
		segment := d.evaluateSegment(d.Mission.Segments[n], context, auxiliary)
		result.Segments = append(result.Segments, segment)
		if segment.Status != ResultComputed {
			result.Complete = false
			continue
		}
		contributing++
		energy += energySum.quantity(portSegmentEnergy.Name, segment.Energy)
		duration += durationSum.quantity(portSegmentDuration.Name, segment.Duration)
		distance += distanceSum.quantity(portSegmentDistance.Name, segment.Distance)
		if segment.Power.Electrical.si > peak {
			peak = segment.Power.Electrical.si
		}
	}
	if contributing == 0 {
		result.Status = ResultMissing
		result.Detail = "no listed segment produced an energy, so the mission has no budget to compare"
		return result
	}
	result.finishSums(energySum, durationSum, distanceSum, energy, duration, distance)
	result.PeakContinuousPower = Quantity{si: peak, dim: DimPower}
	d.applyEnergyBudget(&result)
	return result
}

// finishSums completes the three mission-level totals.
func (m *MissionResult) finishSums(energySum, durationSum, distanceSum *evaluation,
	energy, duration, distance float64,
) {
	for _, sum := range []struct {
		eval  *evaluation
		into  *Quantity
		what  string
		total float64
	}{
		{energySum, &m.RequiredEnergy, "energy", energy},
		{durationSum, &m.TotalDuration, "duration", duration},
		{distanceSum, &m.TotalDistance, "ground distance", distance},
	} {
		result, err := sum.eval.finish(sum.total)
		if err != nil {
			m.Status = statusForError(err)
			m.Detail = "the segment " + sum.what + " values do not sum to a usable total: " + err.Error()
			return
		}
		*sum.into = result.Value
		m.Traces = append(m.Traces, result.Trace)
	}
	m.Status = ResultComputed
	if !m.Complete {
		m.Detail = "this covers only the segments that produced a result; it is an understatement " +
			"of the mission rather than a budget for it"
	}
}

// applyEnergyBudget compares the required energy with what the pack may spend.
func (d Design) applyEnergyBudget(result *MissionResult) {
	if result.Status != ResultComputed {
		return
	}
	usable, err := d.Battery.UsableEnergy()
	if err != nil {
		result.EnergyStatus = LimitUnknown
		result.Detail = appendDetail(result.Detail,
			"no pack energy is established, so the required energy is not compared with anything: "+
				err.Error())
		return
	}
	result.UsableEnergy = usable.Value
	result.Traces = append(result.Traces, usable.Trace)
	budget, err := EnergyBudget(usable.Value, d.Mission.ReserveFraction)
	if err != nil {
		result.EnergyStatus = LimitUnknown
		result.Detail = appendDetail(result.Detail,
			"the energy budget could not be evaluated: "+err.Error())
		return
	}
	result.Budget = budget.Value
	result.Traces = append(result.Traces, budget.Trace)
	if !result.Complete {
		result.EnergyStatus = LimitUnknown
		result.Detail = appendDetail(result.Detail,
			"the required energy covers only part of the mission, so comparing it with the "+
				"budget would call an incomplete mission sufficient")
		return
	}
	slack := requirementTolerance * budget.Value.si
	result.EnergyStatus = statusFor(result.RequiredEnergy.si <= budget.Value.si+slack)
	if result.EnergyStatus == LimitUnmet {
		result.Detail = appendDetail(result.Detail,
			"the mission needs "+result.RequiredEnergy.String()+" and the budget allows "+
				budget.Value.String()+" after a reserve of "+formatFloat(d.Mission.ReserveFraction))
	}
}

func appendDetail(existing, addition string) string {
	if existing == "" {
		return addition
	}
	return existing + "; " + addition
}

// powerContext is everything a segment evaluation needs from the design that
// does not vary between segments: the mass it is judged at, and the wing the
// polar is evaluated over. Gathering it once means every segment in one mission
// rests on the same aircraft rather than on a wing solved several times.
type powerContext struct {
	detail       string
	geometryFail string
	mass         Quantity
	area         Quantity
	aspectRatio  float64
	massStatus   ResultStatus
}

func (d Design) powerContext() powerContext {
	context := powerContext{}
	mass := d.massReading()
	context.mass = mass.Value
	context.massStatus = mass.Status
	context.detail = mass.Detail
	solved := d.solveOnce()
	if failure := solved.failure(); failure != "" {
		context.geometryFail = failure
		return context
	}
	context.area = solved.wing.Projected.Area
	context.aspectRatio = solved.wing.ProjectedAspectRatio
	return context
}

// evaluateSegment evaluates one segment: its ground speed and timing, the power
// it requires, the energy that costs, and the thrust check when one applies.
func (d Design) evaluateSegment(s MissionSegment, context powerContext, aux ElectricalBudget) SegmentResult {
	result := SegmentResult{Name: s.Name, Case: s.Case, Kind: s.Kind}
	if !d.timeSegment(s, &result) {
		return result
	}
	result.Power = d.segmentPower(s, context, aux)
	// The thrust check is resolved either way. A leg whose power did not
	// compute still has something to say about its capability point — that none
	// is named, that the one named does not exist, or that it was measured at a
	// condition this leg does not fly — and reporting a bare "unknown" instead
	// would hide a static measurement being declined behind the same word as an
	// empty field.
	result.Availability = d.checkAvailability(s, result.Power, context)
	if result.Power.Status != ResultComputed {
		result.Status = result.Power.Status
		result.Detail = "no electrical power follows for this segment, so it contributes no energy"
		return result
	}
	energy, err := SegmentEnergy(result.Power.Electrical, result.Duration)
	if err != nil {
		result.Status = statusForError(err)
		result.Detail = "the segment's energy could not be evaluated: " + err.Error()
		return result
	}
	result.Energy = energy.Value
	result.Traces = append(result.Traces, energy.Trace)
	result.Status = ResultComputed
	return result
}

// timeSegment establishes the segment's ground speed, duration and distance. It
// reports whether the segment may continue.
func (d Design) timeSegment(s MissionSegment, result *SegmentResult) bool {
	ground, err := GroundSpeed(s.Speed, s.ClimbAngle, s.WindAlongTrack)
	if err != nil {
		result.Status = statusForError(err)
		result.Detail = "the ground speed could not be evaluated: " + err.Error()
		return false
	}
	result.GroundSpeed = ground.Value
	result.Traces = append(result.Traces, ground.Trace)
	switch s.Timing {
	case SegmentTimingDuration:
		result.Duration = s.Duration
		distance, distErr := SegmentDistance(ground.Value, s.Duration)
		if distErr != nil {
			// A segment that makes no headway still costs energy for as long as
			// it is flown, so the duration stands and only the distance is
			// withheld. Reporting the whole segment as uncomputable here would
			// hide the fuel a failed return leg actually burns.
			result.Detail = "no ground distance follows: " + distErr.Error()
			return true
		}
		result.Distance = distance.Value
		result.Traces = append(result.Traces, distance.Trace)
		return true
	case SegmentTimingDistance:
		result.Distance = s.Distance
		duration, durErr := SegmentDuration(s.Distance, ground.Value)
		if durErr != nil {
			result.Status = statusForError(durErr)
			result.Detail = "no duration follows from this distance: " + durErr.Error()
			return false
		}
		result.Duration = duration.Value
		result.Traces = append(result.Traces, duration.Trace)
		return true
	default:
		result.Status = ResultMissing
		result.Detail = "the segment says neither how long it lasts nor how far it goes"
		return false
	}
}

// segmentPower establishes the electrical power one segment holds.
func (d Design) segmentPower(s MissionSegment, context powerContext, aux ElectricalBudget) SegmentPower {
	power := SegmentPower{Model: s.Model}
	if aux.Status != ResultComputed {
		power.Status = aux.Status
		power.Detail = "no auxiliary electrical demand is established, so no complete electrical " +
			"figure follows for this segment: " + aux.Detail
		return power
	}
	switch s.Model {
	case SegmentModelEntered:
		return d.enteredSegmentPower(s, aux)
	case SegmentModelPolar:
		return d.polarSegmentPower(s, context, aux)
	default:
		power.Status = ResultMissing
		power.Detail = "the segment does not say where its electrical power comes from"
		return power
	}
}

// enteredSegmentPower takes the builder's own estimate. The entered figure is
// the propulsion draw: the auxiliary demand is added here as it is everywhere
// else, so that switching a segment between the two models does not silently
// change whether the avionics were counted.
func (d Design) enteredSegmentPower(s MissionSegment, aux ElectricalBudget) SegmentPower {
	power := SegmentPower{Model: SegmentModelEntered, Evidence: s.EnteredEvidence}
	if !s.EnteredPower.supplied() {
		power.Status = ResultMissing
		power.Detail = "the segment states no entered power estimate"
		return power
	}
	if s.EnteredPower.dim != DimPower {
		power.Status = ResultInvalid
		power.Detail = "the entered estimate is a " + s.EnteredPower.dim.String() + ", not a power"
		return power
	}
	power.PropulsiveElectrical = s.EnteredPower
	power.Electrical = Quantity{si: s.EnteredPower.si + aux.Continuous.si, dim: DimPower}
	power.Status = ResultComputed
	power.Detail = "this segment's propulsion draw is an entered estimate, not a computed one; " +
		"no thrust, lift coefficient or flight-envelope result follows from it"
	return power
}

// polarSegmentPower resolves the steady force balance, reads the polar and
// converts the useful power into an electrical draw.
func (d Design) polarSegmentPower(s MissionSegment, context powerContext, aux ElectricalBudget) SegmentPower {
	power := SegmentPower{Model: SegmentModelPolar, Evidence: weakerEvidence(d.Polar.Evidence, d.Propulsion.Efficiency.Evidence)}
	designCase, ok := d.Case(s.Case)
	if !ok {
		power.Status = ResultMissing
		power.Detail = "the segment names no flight case this design defines"
		return power
	}
	if !d.Polar.supplied() {
		power.Status = ResultMissing
		power.Detail = "this segment's power comes from the drag polar, and the design states none"
		return power
	}
	if context.geometryFail != "" {
		power.Status = ResultInvalid
		power.Detail = "the wing geometry did not solve, so no reference area or aspect ratio is " +
			"available: " + context.geometryFail
		return power
	}
	if context.massStatus != ResultComputed {
		power.Status = context.massStatus
		power.Detail = context.detail
		return power
	}
	efficiency, basis := d.chainEfficiency(s)
	if efficiency == 0 {
		power.Status = ResultMissing
		power.Detail = basis
		return power
	}
	power.ChainEfficiency = efficiency
	d.resolveSegmentBalance(s, designCase, context, &power)
	if power.Status != ResultComputed {
		return power
	}
	power.Electrical = Quantity{si: power.PropulsiveElectrical.si + aux.Continuous.si, dim: DimPower}
	return power
}

// resolveSegmentBalance runs the physical chain: dynamic pressure, the steady
// lift, the lift coefficient, the envelope check, the polar, the drag, the
// required thrust, the useful power and the electrical draw.
func (d Design) resolveSegmentBalance(s MissionSegment, designCase DesignCase,
	context powerContext, power *SegmentPower,
) {
	steps := &resultSet{}
	q := steps.take(DynamicPressure(designCase.Case, s.Speed))
	lift := steps.take(SegmentLift(context.mass, designCase.Case.LoadFactor, s.ClimbAngle))
	if len(steps.issues) > 0 {
		power.fail(steps.issues, "the segment's condition could not be resolved: ")
		return
	}
	power.DynamicPressure = q
	power.Lift = lift
	cl := steps.take(SegmentLiftCoefficient(lift, q, context.area))
	if len(steps.issues) > 0 {
		power.fail(steps.issues, "the lift coefficient could not be evaluated: ")
		return
	}
	power.LiftCoefficient = cl.si
	if err := envelopeCheck(designCase, d.Polar, cl.si); err != nil {
		power.fail(err, "")
		return
	}
	cd := steps.take(DragCoefficient(context.aspectRatio, d.Polar, cl.si))
	if len(steps.issues) > 0 {
		power.fail(steps.issues, "the drag coefficient could not be evaluated: ")
		return
	}
	power.DragCoefficient = cd.si
	ratioResult := steps.take(LiftToDrag(cl.si, cd.si))
	drag := steps.take(DragForce(q, context.area, cd.si))
	if len(steps.issues) > 0 {
		power.fail(steps.issues, "the drag could not be evaluated: ")
		return
	}
	power.LiftToDrag = ratioResult.si
	power.Drag = drag
	thrust := steps.take(RequiredThrust(drag, context.mass, s.ClimbAngle))
	if len(steps.issues) > 0 {
		power.fail(steps.issues, "the required thrust could not be evaluated: ")
		return
	}
	power.Thrust = thrust
	if thrust.si <= 0 {
		power.Traces = steps.traces
		power.Status = ResultInvalid
		power.Detail = "this flight path needs no thrust at all: the descent angle already " +
			"supplies the drag, which is a glide rather than a powered segment. Model it as an " +
			"entered estimate if it still costs the avionics energy"
		return
	}
	useful := steps.take(PropulsivePower(thrust, s.Speed))
	if len(steps.issues) > 0 {
		power.fail(steps.issues, "the useful propulsive power could not be evaluated: ")
		return
	}
	power.Propulsive = useful
	electrical := steps.take(RequiredElectricalPower(useful, power.ChainEfficiency, Quantity{si: 0, dim: DimPower}))
	if len(steps.issues) > 0 {
		power.fail(steps.issues, "the electrical power could not be evaluated: ")
		return
	}
	power.PropulsiveElectrical = electrical
	power.Traces = steps.traces
	power.Status = ResultComputed
}

// fail records why a segment's power could not be established, discarding the
// partial traces so nothing downstream reads half a chain as a result.
func (p *SegmentPower) fail(err error, prefix string) {
	p.Traces = nil
	p.Status = statusForError(err)
	p.Detail = prefix + err.Error()
}

// envelopeCheck refuses a condition the aircraft's own lift evidence does not
// support. A segment flown below the stall of its case is not slow flight the
// polar happens not to cover: it is a condition the aircraft cannot hold, and a
// power figure for it would make the impossible look affordable.
func envelopeCheck(designCase DesignCase, polar DragPolar, cl float64) error {
	var issues Issues
	switch designCase.Case.CLmax.Scope {
	case ScopeAircraft:
		if designCase.Case.CLmax.Max > 0 && cl > designCase.Case.CLmax.Max {
			issues = append(issues, Issue{
				Field: portLiftCoefficient.Name,
				Kind:  IssueUnsupported,
				Detail: "holding this condition needs CL " + formatFloat(cl) + ", above the case's " +
					"CLmax of " + formatFloat(designCase.Case.CLmax.Max) + ". The aircraft cannot " +
					"fly here, so no power requirement is reported for it",
			})
		}
	case ScopeAirfoilSection:
		issues = append(issues, Issue{
			Field: "clmax",
			Kind:  IssueUnsupported,
			Detail: "the case's CLmax is 2D section data, which is not a whole-aircraft lift " +
				"envelope; the segment's flight envelope cannot be checked against it",
		})
	default:
		issues = append(issues, Issue{
			Field: "clmax",
			Kind:  IssueMissing,
			Detail: "the case states no whole-aircraft CLmax, so there is no lift envelope to " +
				"check this condition against and no power result is reported",
		})
	}
	if !polar.covers(cl) {
		issues = append(issues, Issue{
			Field: portLiftCoefficient.Name,
			Kind:  IssueUnsupported,
			Detail: "CL " + formatFloat(cl) + " is outside the range the polar is claimed over, " +
				formatFloat(polar.CLValidMin) + " to " + formatFloat(polar.CLValidMax),
		})
	}
	if len(issues) > 0 {
		return issues
	}
	return nil
}

// chainEfficiency returns the efficiency a segment uses and, when there is
// none, why.
func (d Design) chainEfficiency(s MissionSegment) (efficiency float64, missing string) {
	if s.Efficiency > 0 {
		return s.Efficiency, ""
	}
	if d.Propulsion.Efficiency.Total > 0 {
		return d.Propulsion.Efficiency.Total, ""
	}
	return 0, "an electrical estimate needs a propeller, motor and speed-controller chain " +
		"efficiency, and neither the design nor this segment states one"
}

// weakerEvidence returns the lower of two evidence grades, treating unstated as
// the weakest. A result is only as good as the worst thing it rests on.
func weakerEvidence(a, b EvidenceQuality) EvidenceQuality {
	if evidenceRank(a) <= evidenceRank(b) {
		return a
	}
	return b
}

// evidenceRank orders the grades from weakest to strongest. Simulated ranks
// between assumed and measured: it is a computed answer with its own validation
// limits, which is more than a guess and less than a measurement.
func evidenceRank(q EvidenceQuality) int {
	switch q {
	case EvidenceAssumed:
		return 1
	case EvidenceSimulated:
		return 2
	case EvidenceMeasured:
		return 3
	case EvidenceUnstated:
		return 0
	default:
		return 0
	}
}

// checkAvailability compares the thrust a segment needs with what a named
// capability point was measured to deliver at that segment's condition.
func (d Design) checkAvailability(s MissionSegment, power SegmentPower, context powerContext) SegmentAvailability {
	check := SegmentAvailability{Capability: s.Capability}
	if s.Capability == "" {
		check.Detail = "this segment names no capability point, so whether the propulsion system " +
			"can fly it is unknown. Its energy still counts: energy sufficiency and flight " +
			"feasibility are separate answers"
		return check
	}
	capability, ok := d.Propulsion.Capability(s.Capability)
	if !ok {
		check.Detail = "the design defines no capability point named " + s.Capability
		return check
	}
	check.Evidence = capability.Evidence
	check.AvailableThrust = capability.Thrust
	if applies, why := capability.appliesAt(s.Speed); !applies {
		check.Detail = why
		return check
	}
	if power.Status != ResultComputed {
		check.Detail = "this segment establishes no required thrust to compare, because no " +
			"power figure followed for it at all"
		return check
	}
	if !power.Thrust.supplied() {
		check.Detail = "this segment establishes no required thrust to compare, because " +
			"its power came from an entered estimate rather than from the drag polar"
		return check
	}
	if !capability.Thrust.supplied() {
		check.Detail = "capability point " + s.Capability + " states no thrust"
		return check
	}
	if ratio, err := ThrustToWeight(capability.Thrust, context.mass); err == nil {
		check.Trace = ratio.Trace
	}
	slack := requirementTolerance * capability.Thrust.si
	check.Status = statusFor(power.Thrust.si <= capability.Thrust.si+slack)
	check.Margin = relativeMargin(capability.Thrust.si-power.Thrust.si, capability.Thrust.si)
	if check.Status == LimitUnmet {
		check.Detail = "the segment needs " + power.Thrust.String() + " and capability point " +
			s.Capability + " delivers " + capability.Thrust.String() + " at that condition"
	}
	return check
}
