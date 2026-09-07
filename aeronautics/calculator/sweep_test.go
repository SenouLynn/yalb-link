package calculator_test

import (
	"math"
	"strconv"
	"testing"

	"yalb.aero/calculator"
)

// stallOutput plots the stall speed in the fixture's required case.
var stallOutput = calculator.SweepOutput{
	Subject: calculator.SubjectStallSpeed,
	Case:    caseNameN1,
}

// sweepOver builds settings over the named driver.
func sweepOver(t *testing.T, key calculator.ParameterKey, from, to float64,
	unit calculator.Unit, samples int,
) calculator.SweepSettings {
	t.Helper()
	return calculator.SweepSettings{
		Driver:  key,
		From:    mustQ(t, from, unit),
		To:      mustQ(t, to, unit),
		Samples: samples,
		Output:  stallOutput,
	}
}

func mustPlan(t *testing.T, d calculator.Design, settings calculator.SweepSettings) calculator.SweepPlan {
	t.Helper()
	plan, err := d.PlanSweep(settings)
	if err != nil {
		t.Fatalf("PlanSweep: %v", err)
	}
	return plan
}

// The first acceptance check: a plotted sample is not an approximation of the
// candidate, it is the candidate. Each sample is rebuilt as an ordinary design
// and evaluated directly, and the two answers must be the same number.
func TestEveryPlottedSampleMatchesADirectEvaluation(t *testing.T) {
	design := baseDesign(t)
	plan := mustPlan(t, design, sweepOver(t, calculator.ParamAspectRatio, 4, 10, calculator.One, 7))
	result := plan.Run()

	if len(result.Samples) != 7 {
		t.Fatalf("got %d samples, want 7", len(result.Samples))
	}
	for n, sample := range result.Samples {
		candidate, err := plan.Candidate(n)
		if err != nil {
			t.Fatalf("candidate %d: %v", n, err)
		}
		check := requireCheck(t, candidate.Assess(), reqStall, caseNameN1, calculator.BoundUpper)
		if check.Result != sample.Status {
			t.Errorf("sample %d: status %v, direct evaluation %v", n, sample.Status, check.Result)
		}
		if sample.Value.SI() != check.Actual.SI() {
			t.Errorf("sample %d: plotted %v, direct evaluation %v", n, sample.Value, check.Actual)
		}
	}
}

// The endpoints are exact rather than accumulated, so a range never drifts off
// its own end.
func TestASweepHitsBothEndsOfItsRangeExactly(t *testing.T) {
	plan := mustPlan(t, baseDesign(t), sweepOver(t, calculator.ParamAspectRatio, 4, 10, calculator.One, 7))
	result := plan.Run()
	wantSI(t, "first driver value", result.Samples[0].Driver, 4)
	wantSI(t, "last driver value", result.Samples[len(result.Samples)-1].Driver, 10)
	wantSI(t, "middle driver value", result.Samples[3].Driver, 7)
}

// The Task 02 trends, each with what it holds fixed stated. A trend is only a
// trend against a named solve mode: the same rise in aspect ratio is a smaller
// area at fixed span and a longer span at fixed area.
func TestTheExpectedFixedInputTrendsHold(t *testing.T) {
	t.Run("higher aspect ratio at fixed span raises the stall speed", func(t *testing.T) {
		// At fixed span, S = b^2/A: a higher aspect ratio is a smaller wing.
		result := mustPlan(t, baseDesign(t),
			sweepOver(t, calculator.ParamAspectRatio, 4, 10, calculator.One, 5)).Run()
		wantRising(t, result)
		if result.Mode != calculator.SolveFromSpanAndAspectRatio {
			t.Errorf("solve mode = %v, want span and aspect ratio", result.Mode)
		}
		wantHeld(t, result, calculator.ParamSpanProjected)
	})

	t.Run("more area lowers the stall speed", func(t *testing.T) {
		design := baseDesign(t)
		design.Wing.Drivers.AspectRatio = 0
		design.Wing.Drivers.Area = mustQ(t, 0.24, calculator.SquareMeter)
		result := mustPlan(t, design,
			sweepOver(t, calculator.ParamAreaReference, 0.2, 0.6, calculator.SquareMeter, 5)).Run()
		wantFalling(t, result)
		wantHeld(t, result, calculator.ParamSpanProjected)
	})

	t.Run("more mass raises the stall speed", func(t *testing.T) {
		result := mustPlan(t, baseDesign(t),
			sweepOver(t, calculator.ParamAllUpMass, 1, 3, calculator.Kilogram, 5)).Run()
		wantRising(t, result)
		wantHeld(t, result, calculator.ParamSpanProjected, calculator.ParamAspectRatio)
	})
}

func wantRising(t *testing.T, result calculator.SweepResult) {
	t.Helper()
	wantMonotonic(t, result, 1)
}

func wantFalling(t *testing.T, result calculator.SweepResult) {
	t.Helper()
	wantMonotonic(t, result, -1)
}

func wantMonotonic(t *testing.T, result calculator.SweepResult, sign float64) {
	t.Helper()
	previous := math.NaN()
	for n := range result.Samples {
		sample := &result.Samples[n]
		if sample.Status != calculator.ResultComputed {
			t.Fatalf("sample %d did not compute: %s", n, sample.Detail)
		}
		if !math.IsNaN(previous) && sign*(sample.Value.SI()-previous) <= 0 {
			t.Fatalf("sample %d moved the wrong way: %v then %v", n, previous, sample.Value.SI())
		}
		previous = sample.Value.SI()
	}
	if result.Invariant {
		t.Error("a monotonic curve was reported as invariant")
	}
}

func wantHeld(t *testing.T, result calculator.SweepResult, want ...calculator.ParameterKey) {
	t.Helper()
	for _, key := range want {
		found := false
		for _, held := range result.HeldFixed {
			if held == key {
				found = true
			}
		}
		if !found {
			t.Errorf("%s is not reported as held fixed; held: %v", key, result.HeldFixed)
		}
	}
}

// The acceptance check the mass-and-performance journey turns on: a span change
// at a fixed area leaves the stall speed exactly where it was, and the answer
// has to explain that rather than implying the span does not matter.
func TestASpanSweepAtFixedAreaLeavesTheStallSpeedAloneAndSaysWhy(t *testing.T) {
	design := baseDesign(t)
	design.Wing.Drivers.AspectRatio = 0
	design.Wing.Drivers.Area = mustQ(t, fixtureArea, calculator.SquareMeter)

	result := mustPlan(t, design,
		sweepOver(t, calculator.ParamSpanProjected, 0.8, 2.0, calculator.Meter, 5)).Run()

	if !result.Invariant {
		t.Fatal("the stall speed moved with the span at a fixed area and mass")
	}
	for n := range result.Samples {
		wantSI(t, "sample "+strconv.Itoa(n), result.Samples[n].Value, fixtureStallN1)
	}
	// It is not that the span does nothing: it is that this output does not see
	// it. The answer names what did move.
	if len(result.AlsoChanged) == 0 {
		t.Fatal("nothing was reported as changing, but the aspect ratio and every chord did")
	}
	wantChanged(t, result, calculator.ParamAspectRatio, calculator.ParamChordRoot)
	if !containsAll(result.Detail, "does not move", "wing.aspect_ratio.planform") {
		t.Errorf("detail = %q, want it to name what changed instead", result.Detail)
	}
}

func wantChanged(t *testing.T, result calculator.SweepResult, want ...calculator.ParameterKey) {
	t.Helper()
	for _, key := range want {
		found := false
		for _, changed := range result.AlsoChanged {
			if changed == key {
				found = true
			}
		}
		if !found {
			t.Errorf("%s is not reported as changing; changed: %v", key, result.AlsoChanged)
		}
	}
}

// A derived value cannot be swept. Moving it would mean promoting it, which
// releases another driver, and a sweep never makes that choice.
func TestADerivedValueCannotBeSweptWithoutAPromotion(t *testing.T) {
	design := baseDesign(t)
	// The design drives span and aspect ratio, so the root chord is derived.
	_, err := design.PlanSweep(sweepOver(t, calculator.ParamChordRoot, 0.1, 0.4, calculator.Meter, 5))
	detail := wantIssue(t, err, "sweep.driver", calculator.IssueInvalid)
	if !containsAll(detail, "promoting", "wing.span.projected") {
		t.Errorf("detail = %q, want it to explain the promotion and list the current drivers", detail)
	}
}

// And a value that is not a driver of anything cannot be swept at all.
func TestANonDriverCannotBeSwept(t *testing.T) {
	_, err := baseDesign(t).PlanSweep(calculator.SweepSettings{
		Driver:  calculator.ParamChordMAC,
		From:    mustQ(t, 0.1, calculator.Meter),
		To:      mustQ(t, 0.4, calculator.Meter),
		Samples: 5,
		Output:  stallOutput,
	})
	wantIssue(t, err, "sweep.driver", calculator.IssueUnsupported)
}

// The all-up mass is only a driver while the design takes it from the entered
// figure. Once it comes from the components it is derived from them, and
// sweeping it would be sweeping a result.
func TestTheAllUpMassCannotBeSweptWhenItComesFromTheComponents(t *testing.T) {
	_, err := placementDesign(t).PlanSweep(
		sweepOver(t, calculator.ParamAllUpMass, 1, 3, calculator.Kilogram, 5))
	detail := wantIssue(t, err, "sweep.driver", calculator.IssueInvalid)
	if !containsAll(detail, "components") {
		t.Errorf("detail = %q, want it to say the mass follows from the inventory", detail)
	}
}

// A sample the model cannot evaluate is a gap, not a point. Nothing interpolates
// across it and nothing reports a value for it.
func TestUncomputableSamplesAreGapsRatherThanPoints(t *testing.T) {
	design := baseDesign(t)
	// A body cannot be wider than the span. Sweeping the span across the body
	// width makes the early candidates unsolvable and the later ones fine.
	design.Wing.BodyWidth = mustQ(t, 0.8, calculator.Meter)

	result := mustPlan(t, design,
		sweepOver(t, calculator.ParamSpanProjected, 0.4, 2.0, calculator.Meter, 5)).Run()

	gaps, points := 0, 0
	for _, sample := range result.Samples {
		if sample.Status == calculator.ResultComputed {
			points++
			continue
		}
		gaps++
		if sample.Value != (calculator.Quantity{}) {
			t.Error("a gap carries a value")
		}
		if sample.Detail == "" {
			t.Error("a gap gives no reason")
		}
	}
	if gaps == 0 || points == 0 {
		t.Fatalf("expected the range to straddle the body width, got %d gaps and %d points", gaps, points)
	}
}

// A feasible region can be empty, and an empty one is reported as empty rather
// than as an absent answer.
func TestAnEmptyFeasibleRegionIsReportedAsOne(t *testing.T) {
	design := baseDesign(t)
	// A 2 kg aircraft at these areas cannot reach a 3 m/s stall speed anywhere
	// in the range, so no sample is feasible.
	design.Requirements[0].Maximum = mustQ(t, 3, calculator.MeterPerSecond)

	result := mustPlan(t, design,
		sweepOver(t, calculator.ParamAspectRatio, 4, 10, calculator.One, 5)).Run()

	for n := range result.Samples {
		sample := &result.Samples[n]
		if sample.Status != calculator.ResultComputed {
			t.Fatalf("sample %d did not compute: %s", n, sample.Detail)
		}
		if !sample.HasRequired {
			t.Fatalf("sample %d reports no required check", n)
		}
		if sample.Feasibility != calculator.LimitUnmet {
			t.Errorf("sample %d is %v, want unmet across an empty feasible region", n, sample.Feasibility)
		}
	}
	if len(result.Bounds) != 1 {
		t.Fatalf("got %d requirement boundaries, want the one stall ceiling", len(result.Bounds))
	}
	wantSI(t, "the plotted boundary", result.Bounds[0].Value, 3)
	if result.Bounds[0].Direction != calculator.BoundUpper {
		t.Errorf("boundary direction = %v, want a maximum", result.Bounds[0].Direction)
	}
}

// The marker is evaluated, not interpolated: it sits where a candidate was
// actually checked, which is the design itself.
func TestTheCurrentMarkerIsTheDesignsOwnCandidate(t *testing.T) {
	design := baseDesign(t)
	result := mustPlan(t, design,
		sweepOver(t, calculator.ParamAspectRatio, 4, 10, calculator.One, 5)).Run()
	wantSI(t, "the marker's driver value", result.Current.Driver, 6)
	wantSI(t, "the marker's stall speed", result.Current.Value, fixtureStallN1)
}

// Sampling a range is a question about candidates the session does not hold.
// Nothing is committed, no revision is recorded and the design is untouched.
func TestASweepDoesNotMutateTheCandidate(t *testing.T) {
	session := calculator.NewSession("sweep", baseDesign(t))
	before := session.Design()
	revisions := session.Revisions()

	if _, err := session.Sweep(sweepOver(t, calculator.ParamAspectRatio, 4, 10, calculator.One, 9)); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if session.Revisions() != revisions {
		t.Errorf("revisions = %d, want %d; a sweep commits nothing", session.Revisions(), revisions)
	}
	if session.Design().Snapshot() != before.Snapshot() {
		t.Error("the design changed during a sweep")
	}
	if session.Design().Wing.Drivers.AspectRatio != before.Wing.Drivers.AspectRatio {
		t.Error("the swept driver was left at a sampled value")
	}
}

// A requirement stays where it is during a sweep. Sliding a bound to follow the
// curve would turn a constraint into a result.
func TestASweepLeavesTheRequirementsWhereTheyAre(t *testing.T) {
	design := baseDesign(t)
	before, _ := design.Requirement(reqStall)
	result := mustPlan(t, design,
		sweepOver(t, calculator.ParamAspectRatio, 4, 20, calculator.One, 9)).Run()

	if len(result.Bounds) != 1 {
		t.Fatalf("got %d boundaries, want 1", len(result.Bounds))
	}
	wantSI(t, "the boundary", result.Bounds[0].Value, before.Maximum.SI())
	if design.Requirements[0].Maximum.SI() != before.Maximum.SI() {
		t.Error("the sweep moved the requirement it was judged against")
	}
}

// A sweep answer is matched against both the design it was computed from and
// the settings it answers, because a different range is a different question.
func TestASweepAnswerIsMatchedAgainstItsDesignAndItsSettings(t *testing.T) {
	session := calculator.NewSession("stale", baseDesign(t))
	settings := sweepOver(t, calculator.ParamAspectRatio, 4, 10, calculator.One, 5)
	result, err := session.Sweep(settings)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if err := session.AcceptSweep(result, settings); err != nil {
		t.Fatalf("a fresh sweep was refused: %v", err)
	}

	t.Run("a changed range is a different question", func(t *testing.T) {
		wider := sweepOver(t, calculator.ParamAspectRatio, 4, 12, calculator.One, 5)
		if err := session.AcceptSweep(result, wider); err == nil {
			t.Error("an answer for one range was accepted for another")
		}
	})

	t.Run("a changed output is a different question", func(t *testing.T) {
		other := settings
		other.Output = calculator.SweepOutput{Subject: calculator.SubjectWingArea}
		if err := session.AcceptSweep(result, other); err == nil {
			t.Error("an answer about the stall speed was accepted as one about the area")
		}
	})

	t.Run("an edit retires the answer", func(t *testing.T) {
		mustDo(t, session, calculator.SetMass{
			Mass: mustQ(t, 2.5, calculator.Kilogram), Basis: "revised target",
		})
		if err := session.AcceptSweep(result, settings); err == nil {
			t.Error("a sweep computed before the edit was accepted after it")
		}
	})

	// And returning to the exact design the sweep was computed from does not
	// make it current again. The identity is retired by the edit and is never
	// restored, so a curve the builder has already left cannot reappear as the
	// current one just because the history walked back past it.
	t.Run("undoing back to the same inputs does not revive it", func(t *testing.T) {
		if err := session.Undo(); err != nil {
			t.Fatalf("Undo: %v", err)
		}
		if session.Snapshot() != result.Snapshot {
			t.Fatal("the undo did not restore the design the sweep was computed from")
		}
		if err := session.AcceptSweep(result, settings); err == nil {
			t.Error("an obsolete sweep was accepted after an undo back to its own inputs")
		}
	})
}

// A sweep from another session is never this session's answer, even over an
// identical design and identical settings.
func TestASweepFromAnotherSessionIsRefused(t *testing.T) {
	settings := sweepOver(t, calculator.ParamAspectRatio, 4, 10, calculator.One, 5)
	elsewhere, err := calculator.NewSession("elsewhere", baseDesign(t)).Sweep(settings)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	here := calculator.NewSession("here", baseDesign(t))
	if _, err := here.Sweep(settings); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if err := here.AcceptSweep(elsewhere, settings); err == nil {
		t.Error("another session's sweep was accepted")
	}
}

// A second sweep retires the first, so a slow answer cannot land after a newer
// question has been asked.
func TestOnlyTheLatestSweepIsAcceptable(t *testing.T) {
	session := calculator.NewSession("superseded", baseDesign(t))
	settings := sweepOver(t, calculator.ParamAspectRatio, 4, 10, calculator.One, 5)
	first, err := session.Sweep(settings)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	second, err := session.Sweep(settings)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if err := session.AcceptSweep(second, settings); err != nil {
		t.Errorf("the latest sweep was refused: %v", err)
	}
	if err := session.AcceptSweep(first, settings); err == nil {
		t.Error("a superseded sweep was accepted")
	}
}

// Each sweep gets a fresh identity from the same stream an evaluation uses, so
// two sweeps of the same design are still distinguishable.
func TestEachSweepMintsAFreshIdentity(t *testing.T) {
	session := calculator.NewSession("identities", baseDesign(t))
	settings := sweepOver(t, calculator.ParamAspectRatio, 4, 10, calculator.One, 5)
	first, err := session.Sweep(settings)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	second, err := session.Sweep(settings)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if first.Request == second.Request {
		t.Error("two sweeps share one identity")
	}
	if second.Request.Sequence <= first.Request.Sequence {
		t.Error("sweep identities are not increasing")
	}
}

// The work a sweep does is bounded above and below.
func TestSampleCountsAreBounded(t *testing.T) {
	for _, samples := range []int{0, 1, calculator.MaxSweepSamples + 1} {
		settings := sweepOver(t, calculator.ParamAspectRatio, 4, 10, calculator.One, samples)
		if _, err := baseDesign(t).PlanSweep(settings); err == nil {
			t.Errorf("a sweep of %d samples was accepted", samples)
		}
	}
	settings := sweepOver(t, calculator.ParamAspectRatio, 4, 10, calculator.One, calculator.MaxSweepSamples)
	if _, err := baseDesign(t).PlanSweep(settings); err != nil {
		t.Errorf("the largest allowed sweep was refused: %v", err)
	}
}

// A range with no width is a single candidate, not a sweep.
func TestARangeMustHaveWidth(t *testing.T) {
	settings := sweepOver(t, calculator.ParamAspectRatio, 6, 6, calculator.One, 5)
	if _, err := baseDesign(t).PlanSweep(settings); err == nil {
		t.Error("a zero-width range was accepted")
	}
}

// A per-case output must name its case: the stall speed has a different value
// in every one, and picking one silently would answer a question nobody asked.
func TestAPerCaseOutputMustNameItsCase(t *testing.T) {
	settings := sweepOver(t, calculator.ParamAspectRatio, 4, 10, calculator.One, 5)
	settings.Output = calculator.SweepOutput{Subject: calculator.SubjectStallSpeed}
	_, err := baseDesign(t).PlanSweep(settings)
	wantIssue(t, err, "sweep.output.case", calculator.IssueMissing)

	settings.Output = calculator.SweepOutput{Subject: calculator.SubjectStallSpeed, Case: "nowhere"}
	_, err = baseDesign(t).PlanSweep(settings)
	wantIssue(t, err, "sweep.output.case", calculator.IssueMissing)

	settings.Output = calculator.SweepOutput{Subject: calculator.SubjectWingArea, Case: caseNameN1}
	_, err = baseDesign(t).PlanSweep(settings)
	wantIssue(t, err, "sweep.output.case", calculator.IssueInvalid)
}

// The samples come back in the order they were asked for, every time.
func TestSampleOrderIsDeterministic(t *testing.T) {
	plan := mustPlan(t, baseDesign(t), sweepOver(t, calculator.ParamAspectRatio, 4, 10, calculator.One, 9))
	first := plan.Run()
	second := plan.Run()
	for n := range first.Samples {
		if first.Samples[n].Driver != second.Samples[n].Driver ||
			first.Samples[n].Value != second.Samples[n].Value {
			t.Fatalf("sample %d differs between two runs of the same plan", n)
		}
	}
}
