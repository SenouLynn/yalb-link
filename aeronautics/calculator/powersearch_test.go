package calculator_test

import (
	"testing"

	"yalb.aero/calculator"
)

// searchDesign is the power-first fixture: the rc aircraft with its stall
// ceiling dropped, so the search is judged on the power ceiling alone and the
// interval it finds is not narrowed by an unrelated requirement.
func searchDesign(t *testing.T) calculator.Design {
	t.Helper()
	d := rcDesign(t)
	d.Requirements = nil
	return d
}

// A mass and a power ceiling do not determine a wing. The search reports the
// interval of candidates that fit, with the evaluated candidates that bracket
// its ends, and selects none of them.
//
// The range and the ceiling below straddle the demand's minimum at about
// 0.181 m^2, so the feasible run is closed on both sides and both ends are
// genuinely bracketed rather than running off a search bound.
func TestPowerSearchReportsAnIntervalRatherThanAWing(t *testing.T) {
	d := searchDesign(t)
	result, err := d.PowerSizingSearch(calculator.PowerSizingSettings{
		Driver:  calculator.ParamAreaReference,
		From:    mustQ(t, 0.15, calculator.SquareMeter),
		To:      mustQ(t, 0.30, calculator.SquareMeter),
		Samples: 31,
		Ceiling: mustQ(t, 66.5, calculator.Watt),
	})
	if err != nil {
		t.Fatalf("PowerSizingSearch: %v", err)
	}
	if !result.Found {
		t.Fatalf("no feasible candidate: %s", result.Detail)
	}
	if result.Unique {
		t.Error("a single wing was reported where a range of wings meets the ceiling")
	}
	if len(result.Intervals) != 1 {
		t.Fatalf("got %d intervals, want one connected run: %s", len(result.Intervals), result.Detail)
	}
	interval := result.Intervals[0]
	if interval.First.SI() >= interval.Last.SI() {
		t.Errorf("interval collapses to a point: %v to %v", interval.First, interval.Last)
	}
	// Both ends are inside the searched range, so both are bracketed by
	// evaluated candidates rather than by a solved crossing.
	if interval.OpenLow || interval.OpenHigh {
		t.Errorf("the interval reaches a search bound; widen the fixture range: %s", interval.Detail)
	}
	if !supplied(interval.BelowFirst) || !supplied(interval.AboveLast) {
		t.Error("a closed interval reported no bracketing candidates")
	}
	for _, bracket := range []calculator.Quantity{interval.BelowFirst, interval.AboveLast} {
		if !candidateWasEvaluated(result, bracket) {
			t.Errorf("bracket %v is not one of the evaluated candidates", bracket)
		}
	}
	if result.SettingsFingerprint == "" || result.Snapshot == "" {
		t.Error("the result cannot be matched against the question it answers")
	}
	if result.Mode == calculator.SolveModeUnknown {
		t.Error("a search result without its solve mode is ambiguous")
	}
}

// The demand is not monotonic in wing area: a small wing pays induced drag and
// a large one pays parasite drag, so the curve has a minimum inside a wide
// range. That is what makes the feasible set an interval rather than a bound.
func TestPowerDemandIsNotMonotonicInWingArea(t *testing.T) {
	d := searchDesign(t)
	result, err := d.PowerSizingSearch(calculator.PowerSizingSettings{
		Driver:  calculator.ParamAreaReference,
		From:    mustQ(t, 0.15, calculator.SquareMeter),
		To:      mustQ(t, 1.2, calculator.SquareMeter),
		Samples: 33,
		Ceiling: mustQ(t, 95, calculator.Watt),
	})
	if err != nil {
		t.Fatalf("PowerSizingSearch: %v", err)
	}
	fell, rose := false, false
	previous := 0.0
	for n := range result.Candidates {
		candidate := result.Candidates[n]
		if candidate.Status != calculator.ResultComputed {
			continue
		}
		if previous > 0 {
			if candidate.Demand.SI() < previous {
				fell = true
			}
			if fell && candidate.Demand.SI() > previous {
				rose = true
			}
		}
		previous = candidate.Demand.SI()
	}
	if !fell || !rose {
		t.Error("the demand did not fall and then rise across the range, so the fixture no " +
			"longer demonstrates why the answer is an interval")
	}
}

// A ceiling nothing meets is an explicit no-solution, and it says what that
// result is a statement about: this range, at this resolution.
func TestPowerSearchReportsNoSolutionExplicitly(t *testing.T) {
	d := searchDesign(t)
	result, err := d.PowerSizingSearch(calculator.PowerSizingSettings{
		Driver:  calculator.ParamAreaReference,
		From:    mustQ(t, 0.2, calculator.SquareMeter),
		To:      mustQ(t, 0.8, calculator.SquareMeter),
		Samples: 17,
		Ceiling: mustQ(t, 40, calculator.Watt),
	})
	if err != nil {
		t.Fatalf("PowerSizingSearch: %v", err)
	}
	if result.Found || len(result.Intervals) != 0 {
		t.Fatalf("a ceiling below every candidate produced %d intervals", len(result.Intervals))
	}
	if result.Detail == "" {
		t.Fatal("the no-solution result carries no explanation")
	}
	for n := range result.Candidates {
		if result.Candidates[n].Feasible {
			t.Fatalf("candidate %v was marked feasible in a no-solution result",
				result.Candidates[n].Driver)
		}
	}
}

// A run that reaches the end of the searched range is reported as open there:
// the search says nothing outside its own bounds and must not imply that it does.
func TestPowerSearchDoesNotClaimAnythingOutsideItsRange(t *testing.T) {
	d := searchDesign(t)
	result, err := d.PowerSizingSearch(calculator.PowerSizingSettings{
		Driver:  calculator.ParamAreaReference,
		From:    mustQ(t, 0.35, calculator.SquareMeter),
		To:      mustQ(t, 0.5, calculator.SquareMeter),
		Samples: 9,
		Ceiling: mustQ(t, 200, calculator.Watt),
	})
	if err != nil {
		t.Fatalf("PowerSizingSearch: %v", err)
	}
	if len(result.Intervals) != 1 {
		t.Fatalf("got %d intervals, want one", len(result.Intervals))
	}
	interval := result.Intervals[0]
	if !interval.OpenLow || !interval.OpenHigh {
		t.Error("a run spanning the whole range was not reported as open at both ends")
	}
	if supplied(interval.BelowFirst) || supplied(interval.AboveLast) {
		t.Error("an open end reported a bracketing candidate that does not exist")
	}
}

// A candidate that fits the ceiling but violates a required requirement is not
// offered. A wing that stalls too fast is not a smaller answer, it is the wrong one.
func TestPowerSearchDoesNotOfferCandidatesThatMissRequiredRequirements(t *testing.T) {
	d := rcDesign(t)
	d.Requirements = []calculator.Requirement{{
		Name:     "stall ceiling",
		Subject:  calculator.SubjectStallSpeed,
		Maximum:  mustQ(t, 11, calculator.MeterPerSecond),
		Cases:    []string{rcCase},
		Priority: calculator.PriorityRequired,
		Basis:    "synthetic fixture assumption for a hand launch",
	}}
	result, err := d.PowerSizingSearch(calculator.PowerSizingSettings{
		Driver:  calculator.ParamAreaReference,
		From:    mustQ(t, 0.2, calculator.SquareMeter),
		To:      mustQ(t, 0.8, calculator.SquareMeter),
		Samples: 25,
		Ceiling: mustQ(t, 95, calculator.Watt),
	})
	if err != nil {
		t.Fatalf("PowerSizingSearch: %v", err)
	}
	sawCeilingOnlyPass := false
	for n := range result.Candidates {
		candidate := result.Candidates[n]
		if candidate.WithinCeiling && candidate.Feasibility == calculator.LimitUnmet {
			sawCeilingOnlyPass = true
			if candidate.Feasible {
				t.Errorf("candidate %v fits the ceiling and misses a required requirement, "+
					"but was offered anyway", candidate.Driver)
			}
			if candidate.Detail == "" {
				t.Errorf("candidate %v was excluded without saying why", candidate.Driver)
			}
		}
	}
	if !sawCeilingOnlyPass {
		t.Error("the fixture no longer produces a candidate that fits the ceiling and misses a " +
			"requirement, so this check passes vacuously")
	}
}

// Every candidate the search reports is an ordinary design assessed by the
// ordinary path, so a candidate matches a direct evaluation of the same design
// rather than approximating one.
func TestSearchCandidatesMatchADirectEvaluation(t *testing.T) {
	d := searchDesign(t)
	result, err := d.PowerSizingSearch(calculator.PowerSizingSettings{
		Driver:  calculator.ParamAreaReference,
		From:    mustQ(t, 0.3, calculator.SquareMeter),
		To:      mustQ(t, 0.6, calculator.SquareMeter),
		Samples: 4,
		Ceiling: mustQ(t, 95, calculator.Watt),
	})
	if err != nil {
		t.Fatalf("PowerSizingSearch: %v", err)
	}
	for n := range result.Candidates {
		candidate := result.Candidates[n]
		session := calculator.NewSession("direct", searchDesign(t))
		mustDo(t, session, calculator.SetDriver{
			Key: calculator.ParamAreaReference, Value: candidate.Driver,
		})
		direct := session.Design().MissionAnalysis()
		if direct.Status != calculator.ResultComputed {
			t.Fatalf("direct evaluation at %v: %s", candidate.Driver, direct.Detail)
		}
		if !sameWithin(candidate.Demand.SI(), direct.PeakContinuousPower.SI()) {
			t.Errorf("at %v the search says %v W and a direct evaluation says %v W",
				candidate.Driver, candidate.Demand.SI(), direct.PeakContinuousPower.SI())
		}
	}
}

// A search over a driver the design does not hold would mean promoting it,
// which releases another driver and is a decision the builder makes.
func TestPowerSearchRefusesADriverTheDesignDoesNotHold(t *testing.T) {
	d := searchDesign(t)
	_, err := d.PowerSizingSearch(calculator.PowerSizingSettings{
		Driver:  calculator.ParamSpanProjected,
		From:    mustQ(t, 1.5, calculator.Meter),
		To:      mustQ(t, 2.2, calculator.Meter),
		Samples: 9,
		Ceiling: mustQ(t, 95, calculator.Watt),
	})
	if err == nil {
		t.Fatal("the search moved a value the design derives rather than holds")
	}
	wantIssue(t, err, "sweep.driver", calculator.IssueInvalid)
}

// A search with no ceiling has nothing to judge candidates against.
func TestPowerSearchRefusesAMissingCeiling(t *testing.T) {
	d := searchDesign(t)
	_, err := d.PowerSizingSearch(calculator.PowerSizingSettings{
		Driver:  calculator.ParamAreaReference,
		From:    mustQ(t, 0.2, calculator.SquareMeter),
		To:      mustQ(t, 0.8, calculator.SquareMeter),
		Samples: 9,
	})
	if err == nil {
		t.Fatal("a search with no power ceiling was accepted")
	}
	wantIssue(t, err, "search.ceiling", calculator.IssueMissing)
}

// A required power ceiling and a missing power-model input behave differently:
// the first is a violated requirement on a computed value, the second leaves
// the value unknown and the requirement neither met nor unmet.
func TestViolatedCeilingAndMissingModelInputAreDistinct(t *testing.T) {
	ceiling := calculator.Requirement{
		Name:     "power ceiling",
		Subject:  calculator.SubjectElectricalPower,
		Maximum:  mustQ(t, 60, calculator.Watt),
		Priority: calculator.PriorityRequired,
		Basis:    "synthetic fixture assumption from the speed controller's rating",
	}
	violated := rcDesign(t)
	violated.Requirements = append(violated.Requirements, ceiling)
	wantStatus(t, requireCheck(t, violated.Assess(), "power ceiling", "", calculator.BoundUpper),
		calculator.LimitUnmet, calculator.ResultComputed)

	missing := rcDesign(t)
	missing.Requirements = append(missing.Requirements, ceiling)
	missing.Propulsion.Efficiency = calculator.PropulsionEfficiency{}
	check := requireCheck(t, missing.Assess(), "power ceiling", "", calculator.BoundUpper)
	wantStatus(t, check, calculator.LimitUnknown, calculator.ResultMissing)
	if check.Detail == "" {
		t.Error("a requirement left unknown by a missing input does not say which input")
	}
}

// A preferred ceiling is assessed like any other bound and never narrows
// required feasibility.
func TestPreferredPowerCeilingDoesNotNarrowRequiredFeasibility(t *testing.T) {
	d := rcDesign(t)
	d.Requirements = []calculator.Requirement{{
		Name:     "power preference",
		Subject:  calculator.SubjectElectricalPower,
		Maximum:  mustQ(t, 60, calculator.Watt),
		Priority: calculator.PriorityPreferred,
		Basis:    "synthetic fixture preference",
	}}
	checks := d.Assess()
	wantStatus(t, requireCheck(t, checks, "power preference", "", calculator.BoundUpper),
		calculator.LimitUnmet, calculator.ResultComputed)
	if _, hasRequired := checks.Aggregate(); hasRequired {
		t.Error("a preferred bound alone reported a required feasibility claim")
	}
}

// candidateWasEvaluated reports whether the value is one of the search's own
// evaluated driver values, which is what makes an interval end a bracket rather
// than an interpolated crossing.
func candidateWasEvaluated(result calculator.PowerSizingResult, value calculator.Quantity) bool {
	for n := range result.Candidates {
		if result.Candidates[n].Driver == value {
			return true
		}
	}
	return false
}
