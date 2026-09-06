package calculator_test

import (
	"testing"

	"yalb.aero/calculator"
)

// TestPromotionReleasesOneDriverAtomically holds the rule that promoting a
// derived value to a driver releases a named existing driver in the same edit,
// so the planform never momentarily holds one driver or three.
func TestPromotionReleasesOneDriverAtomically(t *testing.T) {
	s := calculator.NewSession("promote", baseDesign(t))
	if mode, err := s.Design().SolveMode(); err != nil || mode != calculator.SolveFromSpanAndAspectRatio {
		t.Fatalf("initial solve mode = %v (%v), want span and aspect ratio", mode, err)
	}
	mustDo(t, s, calculator.PromoteDriver{
		Promote: calculator.ParamAreaReference,
		Release: calculator.ParamAspectRatio,
		Value:   mustQ(t, 0.3, calculator.SquareMeter),
	})

	d := s.Design()
	drivers := d.DriverKeys()
	if len(drivers) != 2 || drivers[0] != calculator.ParamSpanProjected ||
		drivers[1] != calculator.ParamAreaReference {
		t.Fatalf("drivers = %v, want span and reference area", drivers)
	}
	if mode, err := s.Design().SolveMode(); err != nil || mode != calculator.SolveFromSpanAndArea {
		t.Fatalf("solve mode = %v (%v), want span and area", mode, err)
	}
	e := d.Evaluate()
	wantSI(t, "promoted area", e.Wing.Projected.Area, 0.3)
	if !sameWithin(e.Wing.ProjectedAspectRatio, 1.44/0.3) {
		t.Errorf("aspect ratio = %v, want it derived as %v", e.Wing.ProjectedAspectRatio, 1.44/0.3)
	}

	// The roles moved with the values: the aspect ratio is now derived, and the
	// parameter set says so rather than the caller having to remember.
	params := e.Wing.Parameters()
	area, ok := params.Get(calculator.ParamAreaReference)
	if !ok || area.Role != calculator.RoleDriver {
		t.Errorf("reference area parameter = %+v, want a driver", area)
	}
	aspect, ok := params.Get(calculator.ParamAspectRatio)
	if !ok || aspect.Role != calculator.RoleDerived {
		t.Errorf("aspect ratio parameter = %+v, want derived", aspect)
	}
}

// TestPromotionWithoutAReleaseOffersTheValidSwaps holds that an ambiguous
// promotion is answered with the choices rather than resolved by guessing.
func TestPromotionWithoutAReleaseOffersTheValidSwaps(t *testing.T) {
	d := baseDesign(t)
	swaps, err := d.ValidSwaps(calculator.ParamAreaReference)
	if err != nil {
		t.Fatalf("valid swaps: %v", err)
	}
	if len(swaps) != 2 {
		t.Fatalf("valid swaps = %v, want both current drivers", swaps)
	}

	s := calculator.NewSession("ambiguous", d)
	err = s.Do(calculator.PromoteDriver{
		Promote: calculator.ParamAreaReference,
		Value:   mustQ(t, 0.3, calculator.SquareMeter),
	})
	detail := wantIssue(t, err, string(calculator.ParamAreaReference), calculator.IssueMissing)
	for _, swap := range swaps {
		if !containsSubstring(detail, string(swap)) {
			t.Errorf("the offer %q does not name the valid swap %q", detail, swap)
		}
	}
	if s.Revisions() != 1 {
		t.Error("an unanswered promotion must not change the design")
	}
}

// TestPromotionRefusesToReleaseSomethingThatIsNotADriver holds that the named
// release must be one of the swaps actually on offer. Releasing a value the
// design does not hold would leave the promoted driver added and nothing given
// up, which is the three-driver state the atomic swap exists to prevent.
func TestPromotionRefusesToReleaseSomethingThatIsNotADriver(t *testing.T) {
	s := calculator.NewSession("bad-release", baseDesign(t))
	before := s.Snapshot()
	err := s.Do(calculator.PromoteDriver{
		Promote: calculator.ParamAreaReference,
		Release: calculator.ParamChordRoot,
		Value:   mustQ(t, 0.3, calculator.SquareMeter),
	})
	detail := wantIssue(t, err, string(calculator.ParamChordRoot), calculator.IssueInvalid)
	if !containsSubstring(detail, string(calculator.ParamAspectRatio)) {
		t.Errorf("the refusal %q should name the swaps that are on offer", detail)
	}
	if s.Snapshot() != before {
		t.Fatal("a refused promotion changed the design")
	}
	if drivers := s.Design().DriverKeys(); len(drivers) != 2 {
		t.Errorf("drivers after a refused promotion = %v, want the original two", drivers)
	}
}

// TestPromotingAnExistingDriverIsRefused holds that editing a driver and
// promoting a derived value are different edits, so neither can be reached by
// accident through the other.
func TestPromotingAnExistingDriverIsRefused(t *testing.T) {
	d := baseDesign(t)
	if _, err := d.ValidSwaps(calculator.ParamSpanProjected); err == nil {
		t.Error("the span is already a driver; promoting it is not a swap")
	}
	if _, err := d.ValidSwaps(calculator.ParamChordMAC); err == nil {
		t.Error("the mean aerodynamic chord is not a size driver")
	}
}

// TestSetDriverRefusesADerivedValueAndExplainsTheSwap holds that a plain field
// edit never silently promotes: it names the alternative instead.
func TestSetDriverRefusesADerivedValueAndExplainsTheSwap(t *testing.T) {
	s := calculator.NewSession("set-derived", baseDesign(t))
	err := s.Do(calculator.SetDriver{
		Key:   calculator.ParamAreaReference,
		Value: mustQ(t, 0.3, calculator.SquareMeter),
	})
	detail := wantIssue(t, err, string(calculator.ParamAreaReference), calculator.IssueInvalid)
	if !containsSubstring(detail, "derived") || !containsSubstring(detail, "swap") {
		t.Errorf("the refusal %q should say the value is derived and name the swaps", detail)
	}

	mustDo(t, s, calculator.SetDriver{
		Key:   calculator.ParamSpanProjected,
		Value: mustQ(t, 1.4, calculator.Meter),
	})
	e := s.Evaluate()
	wantSI(t, "edited span", e.Wing.Projected.Span, 1.4)
	wantSI(t, "area from the unchanged aspect ratio", e.Wing.Projected.Area, 1.4*1.4/6)
}

// TestDriverEditsAreDimensionChecked holds that the command layer rejects a
// wrong-dimension driver before the solver sees it.
func TestDriverEditsAreDimensionChecked(t *testing.T) {
	s := calculator.NewSession("dimensions", baseDesign(t))
	err := s.Do(calculator.SetDriver{
		Key:   calculator.ParamSpanProjected,
		Value: mustQ(t, 1.4, calculator.SquareMeter),
	})
	wantIssue(t, err, string(calculator.ParamSpanProjected), calculator.IssueInvalid)
	err = s.Do(calculator.SetDriver{
		Key:   calculator.ParamSpanProjected,
		Value: mustQ(t, 0, calculator.Meter),
	})
	wantIssue(t, err, string(calculator.ParamSpanProjected), calculator.IssueInvalid)
}

// TestPanelPlaneDriversStayInTheirPlane holds that sizing at the stall limit
// writes a panel-plane area driver when the drivers are panel dimensions, so
// the wing that gets built projects the required plan-view area rather than
// carrying it as a construction length.
func TestPanelPlaneDriversStayInTheirPlane(t *testing.T) {
	d := baseDesign(t)
	d.Wing.Dihedral = mustQ(t, 20, calculator.Degree)
	d.Wing.DihedralMode = calculator.DihedralHoldPanel

	drivers := d.DriverKeys()
	if len(drivers) != 2 || drivers[0] != calculator.ParamSpanPanel {
		t.Fatalf("drivers = %v, want them named in the panel plane", drivers)
	}

	s := calculator.NewSession("panel-plane", d)
	mustDo(t, s, calculator.SizeAtStallLimit{
		Hold: calculator.ParamSpanPanel, Scope: calculator.SingleCase(caseNameN1),
	})
	e := s.Evaluate()
	wantSI(t, "projected area", e.Wing.Projected.Area, fixtureMinAreaN1)
	if e.Wing.Panel.Area.SI() <= e.Wing.Projected.Area.SI() {
		t.Errorf("the panel area %v should exceed its projection %v under dihedral",
			e.Wing.Panel.Area, e.Wing.Projected.Area)
	}
	check := requireCheck(t, e.Checks, reqStall, caseNameN1, calculator.BoundUpper)
	wantSI(t, "boundary stall speed", check.Actual, 8)
}

// TestDesignValidationReportsEveryProblemAtOnce holds that a definition is
// checked as a whole rather than one field at a time, and that every stated
// assumption is a demanded field.
func TestDesignValidationReportsEveryProblemAtOnce(t *testing.T) {
	d := calculator.Design{
		Wing: baseWing(t),
		Cases: []calculator.DesignCase{{
			Case: baseCase(t),
		}},
		Requirements: []calculator.Requirement{{
			Name:    "unnamed basis",
			Subject: calculator.SubjectSpan,
			Maximum: mustQ(t, 1.4, calculator.Meter),
		}},
	}
	err := d.Validate()
	if err == nil {
		t.Fatal("an empty design should not validate")
	}
	issues, ok := calculator.AsIssues(err)
	if !ok {
		t.Fatalf("expected calculator.Issues, got %T", err)
	}
	for _, field := range []string{"design", "mass", "case." + baseCase(t).Name, "requirement.unnamed basis"} {
		if !containsString(issues.Fields(), field) {
			t.Errorf("issues %v do not report %q", issues.Fields(), field)
		}
	}
}

// TestPerCaseRequirementsMustNameTheirCases holds that "applies to everything"
// is an explicit choice rather than the meaning of an empty list, and that a
// design-level subject takes no case list at all.
func TestPerCaseRequirementsMustNameTheirCases(t *testing.T) {
	d := baseDesign(t)
	d.Requirements[0].Cases = nil
	wantIssue(t, d.Validate(), "requirement."+reqStall, calculator.IssueMissing)

	d = baseDesign(t)
	d.Requirements[0].Cases = []string{"a case that does not exist"}
	wantIssue(t, d.Validate(), "requirement."+reqStall, calculator.IssueMissing)

	d = baseDesign(t)
	d.Requirements = append(d.Requirements, calculator.Requirement{
		Name: reqSpan, Subject: calculator.SubjectSpan,
		Maximum: mustQ(t, 1.4, calculator.Meter), Priority: calculator.PriorityRequired,
		Basis: "doorway", Cases: []string{caseNameN1},
	})
	wantIssue(t, d.Validate(), "requirement."+reqSpan, calculator.IssueInvalid)
}

// TestRequirementBoundsAreDimensionChecked holds that a bound must carry the
// dimension of the quantity it bounds, and that an empty interval within one
// requirement is reported.
func TestRequirementBoundsAreDimensionChecked(t *testing.T) {
	d := baseDesign(t)
	d.Requirements[0].Maximum = mustQ(t, 8, calculator.Kilogram)
	wantIssue(t, d.Validate(), "requirement."+reqStall, calculator.IssueInvalid)

	d = baseDesign(t)
	d.Requirements = append(d.Requirements, calculator.Requirement{
		Name: "impossible mass", Subject: calculator.SubjectMass,
		Minimum: mustQ(t, 3, calculator.Kilogram), Maximum: mustQ(t, 2, calculator.Kilogram),
		Priority: calculator.PriorityRequired, Basis: "contradictory fixture",
	})
	detail := wantIssue(t, d.Validate(), "requirement.impossible mass", calculator.IssueInvalid)
	if !containsSubstring(detail, "empty") {
		t.Errorf("the refusal %q should say the requirement is empty", detail)
	}
}

// TestSnapshotCoversInputsAndNotHistory holds that two designs reached by
// different routes fingerprint identically, and that changing any input changes
// the fingerprint.
func TestSnapshotCoversInputsAndNotHistory(t *testing.T) {
	direct := baseDesign(t)
	s := calculator.NewSession("route", baseDesign(t))
	mustDo(t, s, calculator.SetMass{Mass: mustQ(t, 3, calculator.Kilogram), Basis: "wrong"})
	mustDo(t, s, calculator.SetMass{
		Mass:  direct.Mass,
		Basis: "target all-up mass for the fixture",
	})
	if s.Snapshot() != direct.Snapshot() {
		t.Errorf("the route to a definition changed its fingerprint:\n direct: %s\n edited: %s",
			direct.Snapshot(), s.Snapshot())
	}

	for name, edit := range map[string]func(*calculator.Design){
		"mass":        func(d *calculator.Design) { d.Mass = mustQ(t, 2.5, calculator.Kilogram) },
		"span":        func(d *calculator.Design) { d.Wing.Drivers.Span = mustQ(t, 1.3, calculator.Meter) },
		"dihedral":    func(d *calculator.Design) { d.Wing.Dihedral = mustQ(t, 4, calculator.Degree) },
		"case CLmax":  func(d *calculator.Design) { d.Cases[0].Case.CLmax.Max = 1.3 },
		"case weight": func(d *calculator.Design) { d.Cases[0].Priority = calculator.PriorityPreferred },
		"bound":       func(d *calculator.Design) { d.Requirements[0].Maximum = mustQ(t, 9, calculator.MeterPerSecond) },
		"evidence":    func(d *calculator.Design) { d.Cases[0].CLmaxEvidence = calculator.EvidenceMeasured },
	} {
		t.Run(name, func(t *testing.T) {
			edited := baseDesign(t)
			edit(&edited)
			if edited.Snapshot() == direct.Snapshot() {
				t.Errorf("changing the %s did not change the fingerprint", name)
			}
		})
	}
}

func containsSubstring(haystack, needle string) bool {
	if len(needle) > len(haystack) {
		return false
	}
	for n := 0; n+len(needle) <= len(haystack); n++ {
		if haystack[n:n+len(needle)] == needle {
			return true
		}
	}
	return false
}
