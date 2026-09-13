package calculator

// Journey names an entry point into the same design definition. The journeys
// differ in which values the builder starts from, not in what a wing is: all of
// them edit one definition, and none of them owns a second set of area, mass or
// speed fields.
type Journey uint8

const (
	// JourneyUnknown is the zero value.
	JourneyUnknown Journey = iota
	// JourneySpanFirst starts from a span the builder can accommodate.
	JourneySpanFirst
	// JourneyMassAndPerformanceFirst starts from an all-up mass and a required
	// stall speed, and sizes the wing from them.
	JourneyMassAndPerformanceFirst
	// JourneyMassAndSizeFirst starts from an all-up mass and an available wing
	// size, and reports the performance that follows.
	JourneyMassAndSizeFirst
	// JourneyExistingDesign starts from a wing that already exists and evaluates
	// it against requirements.
	JourneyExistingDesign
	// JourneyPowerFirst starts from an all-up mass and an electrical power
	// ceiling. Power alone still cannot determine a wing: the journey checks a
	// stated candidate against the ceiling, or searches a bounded range of
	// candidates and reports the feasible interval it finds.
	JourneyPowerFirst
)

var journeyNames = [...]string{
	JourneyUnknown:                 "unknown",
	JourneySpanFirst:               "span first",
	JourneyMassAndPerformanceFirst: "mass and performance first",
	JourneyMassAndSizeFirst:        "mass and available size first",
	JourneyExistingDesign:          "existing design",
	JourneyPowerFirst:              "power first",
}

// String returns the journey's readable name.
func (j Journey) String() string {
	if int(j) < len(journeyNames) {
		return journeyNames[j]
	}
	return "unknown"
}

// Pattern is a curated workflow: a named way into the design definition, with
// the rationale for using it, the inputs it needs, the drivers it leaves
// active, what it produces, and the limits within which that holds.
//
// Patterns are metadata, not code paths. Following one means issuing the
// ordinary commands it describes; nothing here evaluates differently because a
// pattern was named, which is what keeps every entry point on the same results.
type Pattern struct {
	// ID is the stable identifier.
	ID string
	// Name is the readable title.
	Name string
	// Rationale says when this way in is the right one.
	Rationale string
	// Outcome says what the builder ends up holding.
	Outcome string
	// RequiredInputs names what must be supplied before the pattern applies.
	RequiredInputs []string
	// ActiveDrivers are the size drivers the pattern leaves in place.
	ActiveDrivers []ParameterKey
	// ValidityLimits state where the pattern stops being applicable.
	ValidityLimits []string
	// Journey names the entry point.
	Journey Journey
	// Supported reports whether this package implements the pattern. An
	// unsupported pattern is listed so that its absence is a stated gap rather
	// than a silence.
	Supported bool
}

func (p Pattern) clone() Pattern {
	c := p
	c.RequiredInputs = append([]string(nil), p.RequiredInputs...)
	c.ActiveDrivers = append([]ParameterKey(nil), p.ActiveDrivers...)
	c.ValidityLimits = append([]string(nil), p.ValidityLimits...)
	return c
}

// Pattern IDs. Like the equation IDs these are stable, so a saved worksheet can
// record which curated workflow produced it.
const (
	PatternSpanFirst       = "workflow.span-first"
	PatternMassPerformance = "workflow.mass-and-performance-first"
	PatternMassAndSize     = "workflow.mass-and-size-first"
	PatternExistingDesign  = "workflow.existing-design"
	PatternPowerFirst      = "workflow.power-first"
	PatternMissionEnergy   = "workflow.mission-and-energy"
)

// limitLumpedLift and the other shared limits are stated once, so that a
// pattern cannot quietly claim more coverage than the models behind it have.
const (
	limitLumpedLift = "The lift model is lumped: the wing carries the whole load, with no separate " +
		"tail trim load, so a sized area is an aerodynamic bound and not a balanced aircraft."
	limitNoHandling = "No handling, stability, control or structural result follows. A wing that " +
		"meets every implemented requirement has an unknown handling assessment until Task 08."
	limitCLmaxEvidence = "CLmax is always supplied evidence. Nothing here estimates it, so a stall " +
		"result is only as good as the coefficient behind it."
	limitTwoDrivers = "A planform holds exactly two size drivers, plus the taper ratio for a " +
		"trapezoid. Kinked, cranked and elliptical planforms and pointed tips are unsupported."
	limitPolarEvidence = "CD0 and the span efficiency are always supplied evidence at " +
		"aircraft level. Nothing here estimates a zero-lift drag coefficient, and the offered " +
		"Oswald correlation is an unswept-wing statistical fit with no RC validation behind it."
	limitStaticNotCruise = "Static thrust is not cruise thrust. A capability point answers only " +
		"questions at its own condition, and no propeller operating-point model interpolates " +
		"between points."
	limitEnergyNotFeasibility = "Energy sufficiency and flight feasibility are separate answers. " +
		"A mission whose energy fits is not thereby flyable, and a segment with a thrust margin " +
		"is not thereby within the pack's energy."
	limitBoundedSearch = "The power search is a bounded scan of evaluated candidates, not a " +
		"solver. Its interval ends are brackets between two evaluated candidates, it says " +
		"nothing outside its own range, and no iterate exists that could be reported as a " +
		"converged design."
)

var patterns = []Pattern{
	{
		ID:      PatternSpanFirst,
		Name:    "Span first",
		Journey: JourneySpanFirst,
		Rationale: "The span is decided by something outside aerodynamics — a doorway, a car boot, " +
			"a contest rule — so it is entered as a driver and the rest of the wing follows.",
		RequiredInputs: []string{
			"all-up mass and its basis",
			"a flight case with its density basis and a whole-aircraft CLmax with its basis",
			"the span the builder has chosen to use",
			"a second size driver: the aspect ratio, the wing area or the root chord",
		},
		ActiveDrivers: []ParameterKey{ParamSpanProjected, ParamAspectRatio},
		Outcome: "A solved planform at the chosen span, with the stall speed in every case and " +
			"the wing area each required case demands.",
		ValidityLimits: []string{
			"A maximum span is a requirement, not a span. Using the whole allowance is a decision " +
				"the builder makes by entering that span as a driver.",
			limitTwoDrivers, limitCLmaxEvidence, limitLumpedLift, limitNoHandling,
		},
		Supported: true,
	},
	{
		ID:      PatternMassPerformance,
		Name:    "Mass and performance first",
		Journey: JourneyMassAndPerformanceFirst,
		Rationale: "The payload and the required stall speed are known and the wing is sized from " +
			"them. This is the matching-process subset the implemented constraints support.",
		RequiredInputs: []string{
			"all-up mass and its basis",
			"one or more required flight cases, each with its density and CLmax evidence",
			"a required stall-speed ceiling in each of those cases",
			"one size driver to hold while the area is sized",
		},
		ActiveDrivers: []ParameterKey{ParamSpanProjected, ParamAreaReference},
		Outcome: "The smallest wing area every required case allows, the case that controls it, " +
			"and the geometry that follows from holding the chosen driver.",
		ValidityLimits: []string{
			"Only the stall constraint is implemented. This is not the book's full takeoff, climb " +
				"and cruise matching plot, and a stall-only subset does not deliver one.",
			"A stall ceiling bounds the area from below and the mass from above. It justifies no " +
				"lower bound on mass at all.",
			limitCLmaxEvidence, limitLumpedLift, limitNoHandling,
		},
		Supported: true,
	},
	{
		ID:      PatternMassAndSize,
		Name:    "Mass and available size first",
		Journey: JourneyMassAndSizeFirst,
		Rationale: "The wing that can be built or bought is fixed, and the question is what mass " +
			"it will carry and how slowly it will fly.",
		RequiredInputs: []string{
			"all-up mass and its basis",
			"two size drivers describing the available wing",
			"a flight case with its density and CLmax evidence",
		},
		ActiveDrivers: []ParameterKey{ParamSpanProjected, ParamAreaReference},
		Outcome: "The stall speed in every case and the aerodynamic mass ceiling each required " +
			"case allows at that wing.",
		ValidityLimits: []string{
			"The mass ceiling is aerodynamic for the selected cases and is not a structural rating.",
			limitCLmaxEvidence, limitLumpedLift, limitNoHandling,
		},
		Supported: true,
	},
	{
		ID:      PatternExistingDesign,
		Name:    "Existing design",
		Journey: JourneyExistingDesign,
		Rationale: "A wing already exists on paper or in the air, and the question is which " +
			"requirements it meets and what changing one driver would do.",
		RequiredInputs: []string{
			"the complete wing definition, including every angle stated explicitly",
			"all-up mass and its basis",
			"the requirements to judge it against, each with its priority and basis",
		},
		ActiveDrivers: []ParameterKey{ParamSpanProjected, ParamChordRoot},
		Outcome: "Every requirement's status, the aggregate required status, and the known " +
			"conflicting groups with the alternatives that would resolve them.",
		ValidityLimits: []string{
			"A conflicting group is a known one, not a minimal one: no solver runs, so no claim " +
				"is made that removing fewer constraints would restore feasibility.",
			limitTwoDrivers, limitCLmaxEvidence, limitNoHandling,
		},
		Supported: true,
	},
	{
		ID:      PatternPowerFirst,
		Name:    "Power first",
		Journey: JourneyPowerFirst,
		Rationale: "The all-up mass and an electrical power ceiling are what the builder holds, " +
			"and the question is which wings can fly the mission inside that ceiling.",
		RequiredInputs: []string{
			"all-up mass and its basis, entered or from the component inventory",
			"an aircraft drag polar with CD0, the span efficiency, their bases and the " +
				"lift-coefficient range it is claimed over",
			"a propeller, motor and speed-controller chain efficiency with its basis",
			"the avionics, servo, sensor and payload electrical demand, on a stated side of " +
				"each regulator",
			"a flight case per mission segment, with its density and CLmax evidence",
			"mission segments with their speeds, flight-path angles, wind and timing",
			"a required maximum on electrical power, or a ceiling supplied to the search",
		},
		ActiveDrivers: []ParameterKey{ParamAreaReference, ParamAspectRatio},
		Outcome: "The electrical power and energy the mission demands, the segment that " +
			"controls the ceiling, and either a candidate checked against that ceiling or the " +
			"interval of candidates a bounded search found inside it.",
		ValidityLimits: []string{
			"A mass and a power ceiling do not determine a wing. The demand is not monotonic in " +
				"wing area at a fixed speed, so the feasible set is an interval and can be " +
				"several; this journey reports them and selects none.",
			limitPolarEvidence, limitStaticNotCruise, limitEnergyNotFeasibility,
			limitBoundedSearch, limitCLmaxEvidence, limitLumpedLift, limitNoHandling,
		},
		Supported: true,
	},
	{
		ID:      PatternMissionEnergy,
		Name:    "Mission and energy",
		Journey: JourneyPowerFirst,
		Rationale: "The aircraft exists and the question is what it can actually do on one " +
			"pack: how long, how far, and whether the return leg makes it home.",
		RequiredInputs: []string{
			"the complete wing definition and the all-up mass",
			"an aircraft drag polar with its bases and validity range",
			"a flight pack whose mass is a listed component, with its usable fraction",
			"the auxiliary electrical demand, continuous and peak",
			"mission segments including the return leg and the wind along its track",
			"a mission reserve and where it came from",
		},
		ActiveDrivers: []ParameterKey{ParamSpanProjected, ParamAreaReference},
		Outcome: "A segment-by-segment energy budget against the pack, the mission duration and " +
			"ground distance, and the component ratings the demand is checked against.",
		ValidityLimits: []string{
			"A return leg is a segment the builder states. An autopilot's own return setting " +
				"establishes no wind-aware range, and nothing here adds a leg that was not asked for.",
			"The reserve is applied once, to the usable energy. A peak draw with no stated duty " +
				"cycle carries no energy and is checked against supply ratings instead.",
			limitPolarEvidence, limitStaticNotCruise, limitEnergyNotFeasibility,
			limitCLmaxEvidence, limitNoHandling,
		},
		Supported: true,
	},
}

// Patterns returns every curated workflow, supported or not, in a stable order.
// The returned patterns are deep copies, so inspecting one cannot change it for
// anyone else.
func Patterns() []Pattern {
	out := make([]Pattern, 0, len(patterns))
	for n := range patterns {
		out = append(out, patterns[n].clone())
	}
	return out
}

// LookupPattern returns the curated workflow with the given ID.
func LookupPattern(id string) (Pattern, error) {
	for n := range patterns {
		if patterns[n].ID == id {
			return patterns[n].clone(), nil
		}
	}
	return Pattern{}, Issues{{
		Field:  "pattern",
		Kind:   IssueMissing,
		Detail: "no curated workflow is registered under " + id,
	}}
}

// Matches reports whether a design currently holds exactly the pattern's active
// drivers. It answers "which curated workflow am I in", which is a different
// question from whether the design is valid.
func (p Pattern) Matches(d Design) bool {
	held := d.DriverKeys()
	if len(held) != len(p.ActiveDrivers) {
		return false
	}
	for _, want := range p.ActiveDrivers {
		if !containsKey(held, d.planeSizeKey(want)) {
			return false
		}
	}
	return true
}

// PatternsFor returns the supported curated workflows whose active drivers the
// design currently holds. More than one can match: the same driver pair serves
// several ways of arriving at it, and which one the builder is following is not
// recoverable from the numbers.
func PatternsFor(d Design) []Pattern {
	var out []Pattern
	for n := range patterns {
		if patterns[n].Supported && patterns[n].Matches(d) {
			out = append(out, patterns[n].clone())
		}
	}
	return out
}
