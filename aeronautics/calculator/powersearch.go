package calculator

// PowerSizingSettings is a bounded search for the candidates whose electrical
// demand stays inside a power ceiling.
//
// It exists because an all-up mass and an electrical power maximum together do
// not determine a wing. Given a speed, a drag polar, a chain efficiency and an
// auxiliary draw, a range of wings meets any given ceiling, and the demand is
// not even monotonic in wing area: a small wing pays induced drag it cannot
// afford at that speed and a large one pays parasite drag, so the feasible set
// is generally an interval and can be several. Reporting one wing here would be
// inventing an answer the inputs do not contain.
type PowerSizingSettings struct {
	// Driver names the value to move. It must be one the design currently
	// holds, for the same reason a sweep's driver must: moving a derived value
	// would mean promoting it, which releases another driver and is a decision
	// the builder makes.
	Driver ParameterKey
	// From is the low end of the searched range.
	From Quantity
	// To is the high end.
	To Quantity
	// Ceiling is the largest electrical power any single mission segment may
	// hold continuously.
	Ceiling Quantity
	// Samples is how many candidates to evaluate, endpoints included.
	Samples int
}

// fingerprint renders the settings canonically, so a caller can check an answer
// against the question it asked.
func (s PowerSizingSettings) fingerprint() string {
	c := &canonical{}
	c.text("search.driver", string(s.Driver))
	c.qty("search.from", s.From)
	c.qty("search.to", s.To)
	c.qty("search.ceiling", s.Ceiling)
	c.code("search.samples", s.Samples)
	return c.out
}

// PowerSizingCandidate is one evaluated candidate in the search.
type PowerSizingCandidate struct {
	// Detail explains a candidate that did not evaluate.
	Detail string
	// Driver is the value the driver was set to.
	Driver Quantity
	// Demand is the largest electrical power any segment holds continuously.
	Demand Quantity
	// Margin is the relative room inside the ceiling, negative when over it.
	Margin float64
	// Status reports whether the demand was established.
	Status ResultStatus
	// Feasibility is the aggregate of the candidate's own required checks,
	// reported separately from the ceiling so a candidate that fits the ceiling
	// and misses a stall requirement is visibly that rather than simply "no".
	Feasibility LimitStatus
	// HasRequired reports whether any required check existed.
	HasRequired bool
	// WithinCeiling reports whether the demand fits.
	WithinCeiling bool
	// Feasible is WithinCeiling together with the required checks: what the
	// intervals are formed on.
	Feasible bool
}

// PowerSizingInterval is one run of consecutive feasible candidates, with the
// sampled candidates that bracket its ends.
//
// The ends are brackets rather than boundaries on purpose. This search
// evaluates a finite number of candidates and no iterate converges anywhere, so
// the honest statement about where feasibility begins is "between these two
// candidates, which were both evaluated". Naming a single crossing value would
// present a number nothing computed.
type PowerSizingInterval struct {
	// Detail describes the interval and its brackets in words.
	Detail string
	// First and Last are the outermost feasible candidates in the run.
	First Quantity
	// Last is the highest feasible candidate in the run.
	Last Quantity
	// BelowFirst is the nearest infeasible candidate below First, so the
	// boundary lies between the two. It is the zero Quantity when the run
	// reaches the bottom of the searched range.
	BelowFirst Quantity
	// AboveLast is the nearest infeasible candidate above Last.
	AboveLast Quantity
	// OpenLow reports that the run reaches the bottom of the searched range, so
	// the feasible set may continue below it and this search does not say.
	OpenLow bool
	// OpenHigh reports the same at the top.
	OpenHigh bool
}

// PowerSizingResult is one complete bounded search.
//
// It commits nothing and it selects nothing. Choosing a candidate from an
// interval is a separate, explicit edit, exactly as selecting a sweep sample is.
type PowerSizingResult struct {
	// Detail states what was searched and what the outcome means.
	Detail string
	// Snapshot is the fingerprint of the design searched.
	Snapshot string
	// SettingsFingerprint is the canonical form of Settings.
	SettingsFingerprint string
	// HeldFixed names the parameters that did not move.
	HeldFixed []ParameterKey
	// Candidates are every evaluated candidate, in ascending driver order.
	Candidates []PowerSizingCandidate
	// Intervals are the runs of feasible candidates, in ascending driver order.
	// More than one means the feasible set this search found is not connected.
	Intervals []PowerSizingInterval
	// Settings echoes the request.
	Settings PowerSizingSettings
	// Mode names the driver pair the planform solves from, without which a
	// result is ambiguous: at fixed span a higher aspect ratio is a smaller area.
	Mode SolveMode
	// Unique reports that exactly one candidate in the whole searched range was
	// feasible. Even then it is one sampled candidate rather than a solved
	// answer, and Detail says so.
	Unique bool
	// Found reports whether any candidate was feasible at all.
	Found bool
}

// PowerSizingSearch evaluates a bounded range of candidates and reports which
// of them keep the design's electrical demand inside a ceiling.
//
// Every candidate is an ordinary design produced by the ordinary driver edit
// and assessed by the ordinary evaluation, so a candidate reported here matches
// a direct evaluation of the same design rather than approximating one. No
// iteration runs, nothing converges, and there is therefore no iterate that
// could be mistaken for a solved design.
func (d Design) PowerSizingSearch(settings PowerSizingSettings) (PowerSizingResult, error) {
	plan, err := d.planPowerSearch(settings)
	if err != nil {
		return PowerSizingResult{}, err
	}
	result := PowerSizingResult{
		Settings:            settings,
		Snapshot:            plan.snapshot,
		SettingsFingerprint: settings.fingerprint(),
		HeldFixed:           plan.HeldFixed(),
		Mode:                plan.mode,
	}
	for n := 0; n < settings.Samples; n++ {
		result.Candidates = append(result.Candidates, plan.powerCandidate(n, settings.Ceiling))
	}
	result.collectIntervals()
	result.describe(settings)
	return result, nil
}

// planPowerSearch validates the search against the design. It reuses the sweep
// plan so that "which drivers may move, and what stays fixed" has one answer in
// this package rather than two that could disagree.
func (d Design) planPowerSearch(settings PowerSizingSettings) (SweepPlan, error) {
	rs := &resultSet{}
	if !settings.Ceiling.supplied() {
		rs.add("search.ceiling", IssueMissing,
			"state the electrical power ceiling the candidates are judged against")
	} else if settings.Ceiling.dim != DimPower {
		rs.add("search.ceiling", IssueInvalid,
			"the ceiling is a power, but a "+settings.Ceiling.dim.String()+" value was supplied")
	}
	if len(d.Mission.Segments) == 0 {
		rs.add("search", IssueMissing,
			"a power search needs a mission to demand power; the design states no segments")
	}
	if len(rs.issues) > 0 {
		return SweepPlan{}, rs.issues
	}
	// The output subject is the electrical power the search is about, so the
	// sweep plan's own validation covers the driver, the range and the sample
	// count with exactly the rules a sweep uses.
	return d.PlanSweep(SweepSettings{
		Driver:  settings.Driver,
		From:    settings.From,
		To:      settings.To,
		Samples: settings.Samples,
		Output:  SweepOutput{Subject: SubjectElectricalPower},
	})
}

// powerCandidate evaluates one candidate and judges it against the ceiling.
func (p SweepPlan) powerCandidate(n int, ceiling Quantity) PowerSizingCandidate {
	driver := p.DriverValue(n)
	design, err := p.Candidate(n)
	if err != nil {
		return PowerSizingCandidate{
			Driver: driver,
			Status: statusForError(err),
			Detail: "this candidate is not a design the core accepts: " + err.Error(),
		}
	}
	evaluation := design.Evaluate()
	candidate := PowerSizingCandidate{
		Driver:      driver,
		Feasibility: evaluation.Aggregate,
		HasRequired: evaluation.HasRequired,
	}
	demand := design.readPowerSubject(SubjectElectricalPower)
	candidate.Status = demand.Status
	candidate.Detail = demand.Detail
	if demand.Status != ResultComputed {
		return candidate
	}
	candidate.Demand = demand.Value
	slack := requirementTolerance * ceiling.si
	candidate.WithinCeiling = demand.Value.si <= ceiling.si+slack
	candidate.Margin = relativeMargin(ceiling.si-demand.Value.si, ceiling.si)
	// A candidate is only offered when it also satisfies the design's own
	// required requirements. A wing that fits the power ceiling and stalls above
	// its limit is not a smaller answer, it is the wrong one.
	candidate.Feasible = candidate.WithinCeiling &&
		(!candidate.HasRequired || candidate.Feasibility == LimitMet)
	if !candidate.WithinCeiling {
		candidate.Detail = "this candidate demands " + demand.Value.String() +
			", above the ceiling of " + ceiling.String()
	} else if !candidate.Feasible {
		candidate.Detail = "this candidate fits the power ceiling but its required requirements " +
			"are " + candidate.Feasibility.String()
	}
	return candidate
}

// collectIntervals groups the feasible candidates into runs and records the
// candidates that bracket each run's ends.
func (r *PowerSizingResult) collectIntervals() {
	feasibleCount := 0
	start := -1
	for n := range r.Candidates {
		if r.Candidates[n].Feasible {
			feasibleCount++
			if start < 0 {
				start = n
			}
			continue
		}
		if start >= 0 {
			r.Intervals = append(r.Intervals, r.interval(start, n-1))
			start = -1
		}
	}
	if start >= 0 {
		r.Intervals = append(r.Intervals, r.interval(start, len(r.Candidates)-1))
	}
	r.Found = feasibleCount > 0
	r.Unique = feasibleCount == 1
}

// interval builds one run's description, including the bracketing candidates.
func (r *PowerSizingResult) interval(first, last int) PowerSizingInterval {
	out := PowerSizingInterval{
		First:    r.Candidates[first].Driver,
		Last:     r.Candidates[last].Driver,
		OpenLow:  first == 0,
		OpenHigh: last == len(r.Candidates)-1,
	}
	if !out.OpenLow {
		out.BelowFirst = r.Candidates[first-1].Driver
	}
	if !out.OpenHigh {
		out.AboveLast = r.Candidates[last+1].Driver
	}
	out.Detail = "every evaluated candidate from " + out.First.String() + " to " + out.Last.String() +
		" fits the ceiling and meets the design's required requirements. "
	switch {
	case out.OpenLow && out.OpenHigh:
		out.Detail += "The run reaches both ends of the searched range, so the feasible set may " +
			"continue outside it; this search says nothing beyond its own bounds"
	case out.OpenLow:
		out.Detail += "The run reaches the bottom of the searched range, so the set may continue " +
			"below it. Above, feasibility ends between " + out.Last.String() + " and " +
			out.AboveLast.String() + ", both of which were evaluated"
	case out.OpenHigh:
		out.Detail += "Below, feasibility begins between " + out.BelowFirst.String() + " and " +
			out.First.String() + ", both of which were evaluated. The run reaches the top of the " +
			"searched range, so the set may continue above it"
	default:
		out.Detail += "Feasibility begins between " + out.BelowFirst.String() + " and " +
			out.First.String() + ", and ends between " + out.Last.String() + " and " +
			out.AboveLast.String() + ". All four were evaluated; no boundary was solved for"
	}
	return out
}

// describe states the outcome in the terms a worksheet can show.
func (r *PowerSizingResult) describe(settings PowerSizingSettings) {
	prefix := "searched " + string(settings.Driver) + " from " + settings.From.String() + " to " +
		settings.To.String() + " at " + formatFloat(float64(settings.Samples)) +
		" candidates, against a ceiling of " + settings.Ceiling.String() + ". "
	switch {
	case !r.Found:
		r.Detail = prefix + "No evaluated candidate both fits the ceiling and meets the design's " +
			"required requirements. That is a result about this range at this resolution: a " +
			"feasible candidate could lie between two that were evaluated, or outside the range " +
			"entirely"
	case r.Unique:
		r.Detail = prefix + "Exactly one evaluated candidate is feasible. It is a sampled " +
			"candidate rather than a solved design, and a finer search would very likely find " +
			"neighbours of it that are also feasible"
	case len(r.Intervals) > 1:
		r.Detail = prefix + "The feasible candidates fall into " +
			formatFloat(float64(len(r.Intervals))) + " separate runs, so the answer is not one " +
			"design. Each run is reported with the evaluated candidates that bracket it, and " +
			"choosing between them is the builder's decision"
	default:
		r.Detail = prefix + "The feasible candidates form one run. Every candidate in it is an " +
			"answer, so this is an interval rather than a unique wing; selecting one from it is " +
			"a separate, explicit edit"
	}
}
