package calculator

import "math"

// Sample-count bounds for a sensitivity sweep. A sweep is bounded work: it
// evaluates the whole design once per sample, and an unbounded count would let
// one request cost whatever a caller asked it to.
const (
	// MinSweepSamples is the smallest sweep that says anything. One sample is a
	// point, not a trend.
	MinSweepSamples = 2
	// MaxSweepSamples is the largest sweep the core evaluates in one request.
	MaxSweepSamples = 65
)

// SweepOutput names the quantity a sweep plots. It reuses the requirement
// subjects rather than inventing a second vocabulary of plottable values: a
// curve is only worth drawing next to the bounds it is judged against, and
// those bounds are stated on subjects.
type SweepOutput struct {
	// Case names the flight case, for a per-case subject only.
	Case string
	// Subject names the plotted quantity.
	Subject RequirementSubject
}

// SweepSettings is one sensitivity request: which driver moves, between which
// values, at how many samples, and what is read off each candidate.
//
// The driver must be one the design currently holds. Sweeping a derived value
// would mean promoting it, which releases another driver and is a different
// decision the builder makes explicitly; a sweep never makes it for them.
type SweepSettings struct {
	// Driver names the size driver to move, in either plane.
	Driver ParameterKey
	// Output selects the plotted quantity.
	Output SweepOutput
	// From is the first driver value.
	From Quantity
	// To is the last driver value.
	To Quantity
	// Samples is how many candidates to evaluate, endpoints included.
	Samples int
}

// fingerprint renders the settings canonically, so that a caller can check a
// response against the settings it asked for as well as against the design it
// asked about. Two sweeps of the same design with different ranges are
// different questions, and an answer to one is not an answer to the other.
func (s SweepSettings) fingerprint() string {
	c := &canonical{}
	c.text("sweep.driver", string(s.Driver))
	c.qty("sweep.from", s.From)
	c.qty("sweep.to", s.To)
	c.code("sweep.samples", s.Samples)
	c.code("sweep.subject", int(s.Output.Subject))
	c.text("sweep.case", s.Output.Case)
	return c.out
}

// SweepSample is one evaluated candidate: the driver value it was evaluated at,
// the plotted output, and whether that candidate satisfies the required
// requirements.
//
// Status and Feasibility are separate answers, as they are everywhere else in
// this package. A candidate whose output could not be computed is a gap in the
// curve; a candidate that computed and violates a requirement is a point on the
// curve outside the feasible region.
type SweepSample struct {
	// Detail explains a sample that did not compute.
	Detail string
	// Trace records the evaluation that produced Value, when one ran.
	Trace Trace
	// Driver is the value the driver was set to.
	Driver Quantity
	// Value is the plotted output.
	Value Quantity
	// Status reports whether the model produced Value at all.
	Status ResultStatus
	// Feasibility is the aggregate of the candidate's required checks.
	Feasibility LimitStatus
	// HasRequired reports whether any required check existed. When it is false,
	// Feasibility makes no claim: an empty required set is not a passing one.
	HasRequired bool
}

// SweepBound is a requirement boundary drawn across the plotted output.
type SweepBound struct {
	// Name is the requirement it came from.
	Name string
	// Value is the bound, after any margin.
	Value Quantity
	// Direction says which side of the value the bound is.
	Direction BoundDirection
	// Priority states whether the bound must hold.
	Priority Priority
}

// SweepResult is one complete sensitivity sweep, tied to the request identity,
// the input snapshot and the settings it was computed from.
//
// It commits nothing. Sampling a range is a question about candidates the
// design does not hold, and selecting one of them is a separate, explicit edit.
type SweepResult struct {
	// Detail states what moved and what stayed fixed, in words.
	Detail string
	// Snapshot is the fingerprint of the design the sweep was computed from.
	Snapshot string
	// SettingsFingerprint is the canonical form of Settings, so a caller can
	// match an answer against the question without comparing struct fields.
	SettingsFingerprint string
	// HeldFixed names the parameters that did not move: the design's other size
	// driver, and the taper ratio and angles a planform is solved with.
	HeldFixed []ParameterKey
	// AlsoChanged names the derived parameters whose values differ across the
	// range. It is what answers "this driver has no effect" when the plotted
	// output happens not to move.
	AlsoChanged []ParameterKey
	// Bounds are the requirement boundaries on the plotted output.
	Bounds []SweepBound
	// Samples are the evaluated candidates, in ascending driver order.
	Samples []SweepSample
	// Request is the identity this sweep answers.
	Request RequestID
	// Settings echoes the request.
	Settings SweepSettings
	// Current is the design's own candidate, evaluated at the driver value it
	// actually holds. It is the marker on the curve and is not part of Samples.
	Current SweepSample
	// Mode names the driver pair the planform solves from. A plot without it is
	// ambiguous: at fixed span a higher aspect ratio is a smaller area, and at
	// fixed area it is a longer span.
	Mode SolveMode
	// Invariant reports that every computed sample produced the same output. It
	// is a fact about this output at this solve mode, not about the driver.
	Invariant bool
}

// SweepPlan is a validated sweep, ready to evaluate. It exists as a separate
// step so that a transport can evaluate the samples one at a time — checking
// for a cancelled caller between them — without this package reaching for a
// context it is not allowed to import.
type SweepPlan struct {
	snapshot  string
	canonical ParameterKey
	held      []ParameterKey
	settings  SweepSettings
	design    Design
	mode      SolveMode
}

// PlanSweep validates a sweep against the design and reports what it will hold
// fixed. It refuses a driver the design does not hold: a sweep is a question
// about one value moving while the rest of the definition stays as it is, and a
// derived value cannot move without a promotion the builder has not made.
func (d Design) PlanSweep(settings SweepSettings) (SweepPlan, error) {
	rs := &resultSet{}
	canonicalKey := d.resolveSweptDriver(rs, settings.Driver)
	mode, err := d.SolveMode()
	if err != nil {
		rs.issues = append(rs.issues, asSweepIssues(err)...)
	}
	settings.validateRange(rs, sweptDriverDimension(canonicalKey))
	settings.Output.validate(rs, d)
	if len(rs.issues) > 0 {
		return SweepPlan{}, rs.issues
	}
	return SweepPlan{
		design:    d.clone(),
		settings:  settings,
		snapshot:  d.Snapshot(),
		canonical: canonicalKey,
		held:      d.heldDuringSweep(canonicalKey),
		mode:      mode,
	}, nil
}

// resolveSweptDriver names what the swept key edits, and refuses a key that is
// not something this design holds. A derived value cannot move on its own: it
// would need a promotion, which releases another driver in the same edit, and a
// sweep never makes that decision on the builder's behalf.
func (d Design) resolveSweptDriver(rs *resultSet, key ParameterKey) ParameterKey {
	if key == ParamAllUpMass {
		if d.MassMode.resolved() == MassModeComponents {
			rs.add("sweep.driver", IssueInvalid,
				"this design takes its all-up mass from its components, so the mass follows from "+
					"them rather than being a value to sweep. Sweep a component's mass, or take the "+
					"all-up mass from the entered figure")
		}
		return ParamAllUpMass
	}
	canonicalKey, ok := canonicalSizeKey(key)
	switch {
	case !ok:
		rs.add("sweep.driver", IssueUnsupported,
			"only the all-up mass, span, wing area, aspect ratio and root chord can be swept; "+
				string(key)+" is neither")
		return ""
	case !d.holdsDriver(canonicalKey):
		rs.add("sweep.driver", IssueInvalid,
			string(key)+" is derived from this design's drivers, not one of them, so sweeping it "+
				"would mean promoting it and releasing another driver. This design's drivers are "+
				joinKeys(d.DriverKeys()))
		return canonicalKey
	default:
		return canonicalKey
	}
}

// sweptDriverDimension is the dimension a swept driver's endpoints must carry.
func sweptDriverDimension(key ParameterKey) Dimension {
	if key == ParamAllUpMass {
		return DimMass
	}
	return sizeDriverDimension(key)
}

// asSweepIssues re-labels a solve failure so the reason a sweep was refused
// points at the sweep rather than at a field the caller did not send.
func asSweepIssues(err error) Issues {
	issues, ok := AsIssues(err)
	if !ok {
		return Issues{{Field: "sweep", Kind: IssueInvalid, Detail: err.Error()}}
	}
	return issues
}

// validateRange checks the endpoints and the sample count.
func (s SweepSettings) validateRange(rs *resultSet, want Dimension) {
	for _, end := range []struct {
		field string
		value Quantity
	}{{"sweep.from", s.From}, {"sweep.to", s.To}} {
		switch {
		case !end.value.supplied():
			rs.add(end.field, IssueMissing, "state both ends of the range")
		case end.value.dim != want:
			rs.add(end.field, IssueInvalid,
				"expected a "+want.String()+" value but received "+end.value.dim.String())
		case end.value.si <= 0:
			rs.add(end.field, IssueInvalid,
				"a swept driver must be greater than zero, received "+formatFloat(end.value.si))
		}
	}
	if s.From.supplied() && s.To.supplied() && s.From.si == s.To.si {
		rs.add("sweep.to", IssueInvalid,
			"the range has no width; a sweep between one value and itself is a single candidate")
	}
	if s.Samples < MinSweepSamples || s.Samples > MaxSweepSamples {
		rs.add("sweep.samples", IssueInvalid,
			"a sweep evaluates between "+formatFloat(MinSweepSamples)+" and "+
				formatFloat(MaxSweepSamples)+" candidates, received "+formatFloat(float64(s.Samples)))
	}
}

// validate checks that the plotted output is a quantity this design can read.
func (o SweepOutput) validate(rs *resultSet, d Design) {
	if o.Subject == SubjectUnknown {
		rs.add("sweep.output", IssueMissing, "choose what the sweep plots")
		return
	}
	if !o.Subject.PerCase() {
		if o.Case != "" {
			rs.add("sweep.output.case", IssueInvalid,
				o.Subject.String()+" has one value for the whole design, so it names no case")
		}
		return
	}
	if o.Case == "" {
		rs.add("sweep.output.case", IssueMissing,
			"name the flight case: "+o.Subject.String()+" has a different value in each one")
		return
	}
	if _, ok := d.Case(o.Case); !ok {
		rs.add("sweep.output.case", IssueMissing,
			"the design defines no case named "+o.Case+"; it defines "+joinNames(d.caseNames()))
	}
}

// heldDuringSweep names what does not move. It is the other size driver plus
// the shape values a planform is solved with, because a plot whose reader
// cannot see what was held fixed says nothing about the design.
func (d Design) heldDuringSweep(swept ParameterKey) []ParameterKey {
	held := make([]ParameterKey, 0, len(sizeDriverKeyOrder)+3)
	if swept != ParamAllUpMass && d.MassMode.resolved() == MassModeEntered {
		held = append(held, ParamAllUpMass)
	}
	for _, key := range sizeDriverKeyOrder {
		if key == swept || !d.holdsDriver(key) {
			continue
		}
		held = append(held, d.planeSizeKey(key))
	}
	held = append(held, ParamTaperRatio, ParamDihedral)
	return held
}

// Settings returns the validated settings.
func (p SweepPlan) Settings() SweepSettings { return p.settings }

// Count returns how many samples the plan holds.
func (p SweepPlan) Count() int { return p.settings.Samples }

// Snapshot returns the fingerprint of the design the plan was built from.
func (p SweepPlan) Snapshot() string { return p.snapshot }

// Mode returns the driver pair the planform solves from.
func (p SweepPlan) Mode() SolveMode { return p.mode }

// HeldFixed returns the parameters that do not move.
func (p SweepPlan) HeldFixed() []ParameterKey {
	return append([]ParameterKey(nil), p.held...)
}

// DriverValue returns the driver value at sample n, counting from zero. The
// endpoints are exact: the first sample is From and the last is To, rather than
// From plus n steps, so a range never drifts off its own end by accumulated
// rounding.
func (p SweepPlan) DriverValue(n int) Quantity {
	last := p.settings.Samples - 1
	switch {
	case n <= 0:
		return p.settings.From
	case n >= last:
		return p.settings.To
	default:
		fraction := float64(n) / float64(last)
		si := p.settings.From.si + (p.settings.To.si-p.settings.From.si)*fraction
		return Quantity{si: si, dim: p.settings.From.dim}
	}
}

// Candidate returns the design at sample n. It is an ordinary design produced
// by the ordinary driver edit, which is what makes a plotted sample match a
// direct evaluation of the same candidate rather than approximate one.
func (p SweepPlan) Candidate(n int) (Design, error) {
	value := p.DriverValue(n)
	if p.canonical == ParamAllUpMass {
		return SetMass{Mass: value, Basis: p.design.MassBasis}.apply(p.design)
	}
	return SetDriver{Key: p.settings.Driver, Value: value}.apply(p.design)
}

// Sample evaluates one candidate.
func (p SweepPlan) Sample(n int) SweepSample {
	candidate, err := p.Candidate(n)
	if err != nil {
		return SweepSample{
			Driver: p.DriverValue(n),
			Status: statusForError(err),
			Detail: "this candidate is not a design the core accepts: " + err.Error(),
		}
	}
	return p.read(p.DriverValue(n), candidate)
}

// Current evaluates the design as it stands, at the driver value it holds. It
// is the marker on the curve, and it is evaluated rather than interpolated from
// the samples: an interpolated marker would sit where no candidate was checked.
func (p SweepPlan) Current() SweepSample {
	return p.read(p.design.driverValue(p.canonical), p.design)
}

// driverValue reads a swept driver's current value.
func (d Design) driverValue(key ParameterKey) Quantity {
	drivers := d.Wing.Drivers
	switch key {
	case ParamAllUpMass:
		return d.Mass
	case ParamSpanProjected, ParamSpanPanel:
		return drivers.Span
	case ParamAreaReference, ParamAreaPanel:
		return drivers.Area
	case ParamAspectRatio:
		return ratio(drivers.AspectRatio)
	case ParamChordRoot:
		return drivers.RootChord
	default:
		return Quantity{}
	}
}

// read evaluates a candidate and takes the plotted output off it.
func (p SweepPlan) read(driver Quantity, candidate Design) SweepSample {
	evaluation := candidate.Evaluate()
	designCase := DesignCase{}
	if p.settings.Output.Subject.PerCase() {
		designCase, _ = candidate.Case(p.settings.Output.Case)
	}
	value := candidate.read(p.settings.Output.Subject, designCase, candidate.solveOnce())
	return SweepSample{
		Driver:      driver,
		Value:       value.Value,
		Status:      value.Status,
		Detail:      value.Detail,
		Trace:       value.Trace,
		Feasibility: evaluation.Aggregate,
		HasRequired: evaluation.HasRequired,
	}
}

// Run evaluates every sample in order and assembles the result.
func (p SweepPlan) Run() SweepResult {
	result, _ := p.RunUntil(nil)
	return result
}

// RunUntil evaluates every sample in order, asking stop between samples whether
// the caller still wants the answer. It reports false when it stopped early, in
// which case the result is incomplete and must not be shown.
//
// The samples are evaluated sequentially. Each one is a handful of algebraic
// steps, the count is bounded above, and a deterministic order is part of the
// answer rather than an implementation detail; a goroutine per sample would buy
// microseconds and cost the guarantee. Cancellation is a callback rather than a
// context because this package reaches neither a context nor a clock, and a
// transport that has one passes its own check in.
func (p SweepPlan) RunUntil(stop func() bool) (SweepResult, bool) {
	result := SweepResult{
		Settings:            p.settings,
		SettingsFingerprint: p.settings.fingerprint(),
		Snapshot:            p.snapshot,
		Mode:                p.mode,
		HeldFixed:           p.HeldFixed(),
		Samples:             make([]SweepSample, 0, p.settings.Samples),
		Current:             p.Current(),
		Bounds:              p.bounds(),
	}
	for n := range p.settings.Samples {
		if stop != nil && stop() {
			return SweepResult{}, false
		}
		result.Samples = append(result.Samples, p.Sample(n))
	}
	result.Invariant = invariantOutput(result.Samples)
	result.AlsoChanged = p.alsoChanged()
	result.Detail = p.describe(result)
	return result, true
}

// bounds collects the requirement boundaries on the plotted output. They are
// read from the design the sweep started from and are never moved by it:
// exploring a requirement is a different request, and sliding a bound to follow
// a curve would turn a constraint into a result.
func (p SweepPlan) bounds() []SweepBound {
	out := p.settings.Output
	bounds := make([]SweepBound, 0, len(p.design.Requirements))
	for _, r := range p.design.Requirements {
		if r.Subject != out.Subject {
			continue
		}
		if out.Subject.PerCase() && !r.appliesTo(out.Case) {
			continue
		}
		for _, side := range []struct {
			value     Quantity
			direction BoundDirection
		}{
			{r.effectiveMinimum(), BoundLower},
			{r.effectiveMaximum(), BoundUpper},
		} {
			if !side.value.supplied() {
				continue
			}
			bounds = append(bounds, SweepBound{
				Name:      r.Name,
				Value:     side.value,
				Direction: side.direction,
				Priority:  r.Priority,
			})
		}
	}
	return bounds
}

// invariantOutput reports whether every computed sample produced the same
// value. Fewer than two computed samples cannot establish it: one point is not
// a flat line.
func invariantOutput(samples []SweepSample) bool {
	computed := 0
	first := 0.0
	for n := range samples {
		if samples[n].Status != ResultComputed {
			continue
		}
		computed++
		if computed == 1 {
			first = samples[n].Value.si
			continue
		}
		if math.Abs(samples[n].Value.si-first) > requirementTolerance*math.Abs(first) {
			return false
		}
	}
	return computed >= 2
}

// alsoChanged names the derived parameters whose values differ between the ends
// of the range. It is what keeps an invariant output from reading as "this
// driver does not matter": at a fixed target stall speed a longer span leaves
// the speed alone and changes the aspect ratio and every chord.
func (p SweepPlan) alsoChanged() []ParameterKey {
	first, firstErr := p.solvedParameters(0)
	last, lastErr := p.solvedParameters(p.settings.Samples - 1)
	if firstErr != nil || lastErr != nil {
		return nil
	}
	changed := make([]ParameterKey, 0, len(first))
	for _, before := range first {
		after, ok := last.Get(before.Key)
		if !ok || before.Value.dim != after.Value.dim {
			continue
		}
		if math.Abs(after.Value.si-before.Value.si) >
			requirementTolerance*math.Max(math.Abs(before.Value.si), 1) {
			changed = append(changed, before.Key)
		}
	}
	return changed
}

func (p SweepPlan) solvedParameters(n int) (ParameterSet, error) {
	candidate, err := p.Candidate(n)
	if err != nil {
		return nil, err
	}
	wing, err := candidate.Solve()
	if err != nil {
		return nil, err
	}
	return wing.Parameters(), nil
}

// describe states in words what moved and what did not, because a plot without
// its driver mode is ambiguous: the same rise in aspect ratio is a smaller area
// at fixed span and a longer span at fixed area.
func (p SweepPlan) describe(result SweepResult) string {
	detail := "The planform solves from " + p.mode.String() + ". Sweeping " +
		string(p.settings.Driver) + " holds " + joinKeys(p.held) +
		" fixed; every other dimension follows from them."
	if !result.Invariant {
		return detail
	}
	changed := joinKeys(result.AlsoChanged)
	if len(result.AlsoChanged) == 0 {
		return detail + " " + p.settings.Output.Subject.String() +
			" does not move across this range."
	}
	return detail + " " + p.settings.Output.Subject.String() +
		" does not move across this range, which is a fact about this output at this solve mode " +
		"rather than about the driver: " + changed + " all change with it."
}

// Sweep evaluates a sensitivity sweep under a fresh request identity. The
// identity is minted from the same stream as an evaluation's, so a sweep answer
// can be told apart from an evaluation answer and from an earlier sweep's.
//
// Nothing is committed. The session's design, its history and its current
// request are all left exactly as they were: sampling a range asks about
// candidates the session does not hold.
func (s *Session) Sweep(settings SweepSettings) (SweepResult, error) {
	plan, err := s.history[s.cursor].PlanSweep(settings)
	if err != nil {
		return SweepResult{}, err
	}
	s.next++
	result := plan.Run()
	result.Request = RequestID{Session: s.name, Sequence: s.next}
	s.currentSweep = result.Request
	return result, nil
}

// AcceptSweep reports whether a sweep result still answers the question the
// session is asking. Three things must agree: the outstanding sweep identity,
// the inputs the design still holds, and the settings still selected.
//
// The identity is checked under Task 04's history rule, which is what makes an
// obsolete answer stay obsolete. Every design change retires it, undo and redo
// included, so a sweep requested before an edit is refused afterwards even when
// the history walks back to the exact design it was computed from. Matching on
// the inputs alone would accept it there, and the builder would be looking at a
// curve they had already left.
//
// The settings clause is the sweep's own: a different range or a different
// plotted output is a different question, and an answer to one is not an answer
// to the other even over identical inputs.
func (s *Session) AcceptSweep(result SweepResult, settings SweepSettings) error {
	if s.currentSweep.IsZero() {
		return commandIssue("sweep", IssueInvalid,
			"the design changed since this sweep was requested; sample again before accepting a result")
	}
	if result.Request != s.currentSweep {
		return commandIssue("sweep", IssueInvalid,
			"result answers sweep "+result.Request.String()+" but the outstanding sweep is "+
				s.currentSweep.String())
	}
	if result.Snapshot != s.Snapshot() {
		return commandIssue("sweep", IssueInvalid,
			"the sweep was computed from a different input snapshot than the design now holds")
	}
	if result.SettingsFingerprint != settings.fingerprint() {
		return commandIssue("sweep", IssueInvalid,
			"the sweep answers different settings than the ones now selected")
	}
	return nil
}
