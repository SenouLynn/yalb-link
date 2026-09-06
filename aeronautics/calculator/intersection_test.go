package calculator_test

import (
	"testing"

	"yalb.aero/calculator"
)

// TestRequiredCasesIntersectToTheControllingBound is acceptance check 7: the
// per-case minimum areas are 0.4169494048 and 0.8338988095 m^2, the all-case
// lower bound is the larger of the two and is controlled by n=2, and at the
// original 0.24 m^2 the combined mass ceiling is 0.5756094079 kg.
func TestRequiredCasesIntersectToTheControllingBound(t *testing.T) {
	d := bothCasesDesign(t)

	single := map[string]float64{caseNameN1: fixtureMinAreaN1, caseNameN2: fixtureMinAreaN2}
	for name, want := range single {
		bound, err := d.AreaLowerBound(calculator.SingleCase(name))
		if err != nil {
			t.Fatalf("area bound for %s: %v", name, err)
		}
		wantSI(t, "minimum area in "+name, bound.Value, want)
		wantControlling(t, bound, name)
	}

	all, err := d.AreaLowerBound(calculator.AllRequiredCases())
	if err != nil {
		t.Fatalf("all-case area bound: %v", err)
	}
	wantSI(t, "all-case minimum area", all.Value, fixtureMinAreaN2)
	wantControlling(t, all, caseNameN2)
	if all.Partial {
		t.Error("both cases are fully specified, so the bound is not partial")
	}
	if len(all.Contributions) != 2 {
		t.Errorf("contributions = %d, want one per required case", len(all.Contributions))
	}

	mass, err := d.MassUpperBound(calculator.AllRequiredCases())
	if err != nil {
		t.Fatalf("mass bound: %v", err)
	}
	wantSI(t, "combined mass ceiling", mass.Value, fixtureMaxMassN2)
	wantControlling(t, mass, caseNameN2)
	perCase, err := d.MassUpperBound(calculator.SingleCase(caseNameN1))
	if err != nil {
		t.Fatalf("single-case mass bound: %v", err)
	}
	wantSI(t, "n=1 mass ceiling", perCase.Value, fixtureMaxMassN1)
}

// TestCaseOrderDoesNotChangeResults is the second part of acceptance check 7.
func TestCaseOrderDoesNotChangeResults(t *testing.T) {
	forward := bothCasesDesign(t)
	reversed := bothCasesDesign(t)
	reversed.Cases[0], reversed.Cases[1] = reversed.Cases[1], reversed.Cases[0]
	reversed.Requirements[0].Cases = []string{caseNameN2, caseNameN1}

	if forward.Snapshot() != reversed.Snapshot() {
		t.Fatal("case order changed the design fingerprint")
	}
	a, b := forward.Evaluate(), reversed.Evaluate()
	if a.AreaLower.Value.SI() != b.AreaLower.Value.SI() {
		t.Errorf("area bounds differ: %v and %v", a.AreaLower.Value, b.AreaLower.Value)
	}
	wantControlling(t, b.AreaLower, caseNameN2)
	if a.Aggregate != b.Aggregate {
		t.Errorf("aggregate statuses differ: %v and %v", a.Aggregate, b.Aggregate)
	}
}

// TestPreferredCaseLeavesRequiredBoundsAlone is the third part of acceptance
// check 7: making n=2 preferred removes it from the required bounds while
// preserving its own assessment.
func TestPreferredCaseLeavesRequiredBoundsAlone(t *testing.T) {
	s := calculator.NewSession("preferred", bothCasesDesign(t))
	mustDo(t, s, calculator.SetCasePriority{Name: caseNameN2, Priority: calculator.PriorityPreferred})
	e := s.Evaluate()

	wantSI(t, "required minimum area", e.AreaLower.Value, fixtureMinAreaN1)
	wantControlling(t, e.AreaLower, caseNameN1)
	wantSI(t, "required mass ceiling", e.Mass.Upper.Value, fixtureMaxMassN1)

	check := requireCheck(t, e.Checks, reqStall, caseNameN2, calculator.BoundUpper)
	wantStatus(t, check, calculator.LimitUnmet, calculator.ResultComputed)
	wantSI(t, "n=2 stall speed", check.Actual, fixtureStallN2)
	if check.Priority != calculator.PriorityRequired {
		t.Errorf("the requirement stays required; only the case became preferred, got %v", check.Priority)
	}
}

// TestPreferredRequirementDoesNotTightenFeasibility holds the same rule for a
// preferred requirement rather than a preferred case: it is assessed and it
// contributes nothing to the required bounds.
func TestPreferredRequirementDoesNotTightenFeasibility(t *testing.T) {
	s := calculator.NewSession("preferred-requirement", baseDesign(t))
	mustDo(t, s, calculator.SetRequirementPriority{Name: reqStall, Priority: calculator.PriorityPreferred})
	e := s.Evaluate()

	if e.AreaLower.Known {
		t.Errorf("a preferred ceiling must not bound the required area, got %v", e.AreaLower.Value)
	}
	if e.AreaLower.Detail == "" {
		t.Error("an unknown bound should say why nothing bounds it")
	}
	check := requireCheck(t, e.Checks, reqStall, caseNameN1, calculator.BoundUpper)
	wantStatus(t, check, calculator.LimitUnmet, calculator.ResultComputed)
	if _, has := e.Checks.Aggregate(); has {
		t.Error("with no required check left, the aggregate must make no feasibility claim")
	}
}

// TestWithdrawnEvidenceMakesTheBoundPartial is acceptance check 8: removing the
// CLmax evidence from one required case makes its check unknown and leaves the
// remaining bound partial, and a known failure elsewhere still makes the
// aggregate unmet.
func TestWithdrawnEvidenceMakesTheBoundPartial(t *testing.T) {
	s := calculator.NewSession("check-8", bothCasesDesign(t))
	mustDo(t, s, calculator.SetCaseCLmax{Case: caseNameN1})
	e := s.Evaluate()

	withdrawn := requireCheck(t, e.Checks, reqStall, caseNameN1, calculator.BoundUpper)
	wantStatus(t, withdrawn, calculator.LimitUnknown, calculator.ResultMissing)
	if withdrawn.Evidence != calculator.EvidenceUnstated {
		t.Errorf("evidence = %v, want unstated after it was withdrawn", withdrawn.Evidence)
	}
	remaining := requireCheck(t, e.Checks, reqStall, caseNameN2, calculator.BoundUpper)
	wantStatus(t, remaining, calculator.LimitUnmet, calculator.ResultComputed)

	if !e.AreaLower.Partial {
		t.Error("a required case that could not contribute must make the bound partial")
	}
	if !e.AreaLower.Known {
		t.Fatal("the remaining case still bounds the area")
	}
	wantSI(t, "partial area bound", e.AreaLower.Value, fixtureMinAreaN2)
	wantControlling(t, e.AreaLower, caseNameN2)
	if status, has := e.Checks.Aggregate(); !has || status != calculator.LimitUnmet {
		t.Errorf("aggregate = %v, want unmet: a known failure outweighs an unknown", status)
	}

	// With the failure resolved, the unknown is all that is left and the
	// aggregate is unknown rather than met.
	mustDo(t, s, calculator.SizeAtStallLimit{
		Hold: calculator.ParamSpanProjected, Scope: calculator.SingleCase(caseNameN2),
	})
	sized := s.Evaluate()
	wantStatus(t, requireCheck(t, sized.Checks, reqStall, caseNameN2, calculator.BoundUpper),
		calculator.LimitMet, calculator.ResultComputed)
	if status, has := sized.Checks.Aggregate(); !has || status != calculator.LimitUnknown {
		t.Errorf("aggregate = %v, want unknown while one required check is unknown", status)
	}
}

// TestAreaCeilingConflictsWithTheIntersectedMinimum is the second part of
// acceptance check 8: a 0.5 m^2 area ceiling conflicts with the two fully
// specified required cases, and the conflict names the controlling lower bound.
func TestAreaCeilingConflictsWithTheIntersectedMinimum(t *testing.T) {
	d := bothCasesDesign(t)
	d.Requirements = append(d.Requirements, calculator.Requirement{
		Name: reqArea, Subject: calculator.SubjectWingArea,
		Maximum: mustQ(t, 0.5, calculator.SquareMeter), Priority: calculator.PriorityRequired,
		Basis: "the sheet stock available",
	})
	e := d.Evaluate()
	if len(e.Conflicts) != 1 {
		t.Fatalf("conflicts = %+v, want exactly one", e.Conflicts)
	}
	conflict := e.Conflicts[0]
	if !containsString(conflict.Group, caseNameN2) || !containsString(conflict.Group, reqArea) {
		t.Errorf("conflict group = %v, want the controlling case and the area ceiling", conflict.Group)
	}
	wantSI(t, "controlling lower bound", e.AreaLower.Value, fixtureMinAreaN2)
	wantControlling(t, e.AreaLower, caseNameN2)
	if conflict.Detail == "" {
		t.Error("a conflict should explain how each side was derived")
	}
}

// TestTiedControllingCasesAreBothReported holds that an intersection reports
// every case that sets the bound, not an arbitrary winner among equals.
func TestTiedControllingCasesAreBothReported(t *testing.T) {
	d := baseDesign(t)
	twin := designCaseAt(t, "n=1 at the same air", 1)
	d.Cases = append(d.Cases, twin)
	d.Requirements = []calculator.Requirement{stallCeilingAt(t, caseNameN1, twin.Case.Name)}

	bound, err := d.AreaLowerBound(calculator.AllRequiredCases())
	if err != nil {
		t.Fatalf("area bound: %v", err)
	}
	wantSI(t, "tied minimum area", bound.Value, fixtureMinAreaN1)
	wantControlling(t, bound, caseNameN1, twin.Case.Name)
}

// TestEmptyRequiredSetMakesNoFeasibilityClaim holds that an empty required set
// is not a passing one.
func TestEmptyRequiredSetMakesNoFeasibilityClaim(t *testing.T) {
	d := baseDesign(t)
	d.Requirements = nil
	e := d.Evaluate()
	if status, has := e.Checks.Aggregate(); has || status != calculator.LimitUnknown {
		t.Errorf("aggregate = %v (has required %v), want no claim", status, has)
	}
	if e.AreaLower.Known {
		t.Error("with no requirement, nothing bounds the area")
	}
	if e.Geometry != calculator.ResultComputed {
		t.Fatalf("the wing still solves: %v", e.GeometryIssues)
	}
}

// TestAStallCeilingAloneIsNotAMassRange is acceptance check 6's mass-range
// rule: an aerodynamic ceiling bounds the mass from above and justifies no
// nonzero lower bound at all.
func TestAStallCeilingAloneIsNotAMassRange(t *testing.T) {
	interval, err := baseDesign(t).MassInterval(calculator.AllRequiredCases())
	if err != nil {
		t.Fatalf("mass interval: %v", err)
	}
	if interval.Complete {
		t.Error("a stall ceiling alone does not complete a mass range")
	}
	if !interval.Upper.Known || interval.Lower.Known {
		t.Errorf("expected an upper bound and no lower one, got %+v", interval)
	}
	wantSI(t, "mass ceiling", interval.Upper.Value, fixtureMaxMassN1)
	if interval.Detail == "" {
		t.Error("an incomplete range should say what would complete it")
	}
}

// TestAComponentMinimumCompletesTheMassRange holds that an explicit lower
// constraint is what makes a range, not the ceiling.
func TestAComponentMinimumCompletesTheMassRange(t *testing.T) {
	d := baseDesign(t)
	d.Requirements = append(d.Requirements, calculator.Requirement{
		Name: "component minimum", Subject: calculator.SubjectMass,
		Minimum: mustQ(t, 0.9, calculator.Kilogram), Priority: calculator.PriorityRequired,
		Basis: "airframe, battery and avionics already chosen",
	})
	interval, err := d.MassInterval(calculator.AllRequiredCases())
	if err != nil {
		t.Fatalf("mass interval: %v", err)
	}
	if !interval.Complete || interval.Empty {
		t.Fatalf("expected a complete non-empty range, got %+v", interval)
	}
	wantSI(t, "lower mass bound", interval.Lower.Value, 0.9)
	wantSI(t, "upper mass bound", interval.Upper.Value, fixtureMaxMassN1)
}

// TestALoadingMinimumCompletesTheMassRange holds that an explicit loading range
// supplies the lower bound through the solved reference area.
func TestALoadingMinimumCompletesTheMassRange(t *testing.T) {
	d := baseDesign(t)
	d.Requirements = append(d.Requirements, calculator.Requirement{
		Name: "minimum loading", Subject: calculator.SubjectWingLoadingMass,
		// 30 g/dm^2 is 3 kg/m^2, which at the 0.24 m^2 fixture wing is 0.72 kg.
		Minimum:  mustQ(t, 30, calculator.GramPerSquareDecimeter),
		Priority: calculator.PriorityRequired,
		Basis:    "penetration in wind, chosen for this fixture",
	})
	interval, err := d.MassInterval(calculator.AllRequiredCases())
	if err != nil {
		t.Fatalf("mass interval: %v", err)
	}
	if !interval.Complete {
		t.Fatalf("expected a complete range, got %+v", interval)
	}
	wantSI(t, "loading-derived lower bound", interval.Lower.Value, 0.72)
}

// TestAnEmptyMassRangeIsDetected is acceptance check 6's empty-interval outcome.
func TestAnEmptyMassRangeIsDetected(t *testing.T) {
	d := baseDesign(t)
	d.Requirements = append(d.Requirements, calculator.Requirement{
		Name: "component minimum", Subject: calculator.SubjectMass,
		Minimum: mustQ(t, 2, calculator.Kilogram), Priority: calculator.PriorityRequired,
		Basis: "airframe, battery and avionics already chosen",
	})
	interval, err := d.MassInterval(calculator.AllRequiredCases())
	if err != nil {
		t.Fatalf("mass interval: %v", err)
	}
	if !interval.Empty {
		t.Fatalf("a 2 kg minimum against a %v kg ceiling is empty, got %+v", fixtureMaxMassN1, interval)
	}
	if interval.Detail == "" {
		t.Error("an empty range should name both bounds")
	}
}

func containsString(all []string, want string) bool {
	for _, s := range all {
		if s == want {
			return true
		}
	}
	return false
}
