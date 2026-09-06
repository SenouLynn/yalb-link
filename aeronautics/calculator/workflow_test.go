package calculator_test

import (
	"testing"

	"yalb.aero/calculator"
)

// TestSpanFirstCandidateIsSolvedAndUnmet is acceptance check 1: span 1.2 m,
// aspect ratio 6, mass 2 kg and Task 02's case give an area of 0.24 m^2, and
// the 8 m/s stall ceiling is unmet.
func TestSpanFirstCandidateIsSolvedAndUnmet(t *testing.T) {
	d := baseDesign(t)
	if err := d.Validate(); err != nil {
		t.Fatalf("fixture design is not valid: %v", err)
	}
	e := d.Evaluate()
	if e.Geometry != calculator.ResultComputed {
		t.Fatalf("geometry = %v, want computed: %v", e.Geometry, e.GeometryIssues)
	}
	wantSI(t, "reference area", e.Wing.Projected.Area, fixtureArea)

	check := requireCheck(t, e.Checks, reqStall, caseNameN1, calculator.BoundUpper)
	wantStatus(t, check, calculator.LimitUnmet, calculator.ResultComputed)
	wantSI(t, "stall speed", check.Actual, fixtureStallN1)
	if check.Evidence != calculator.EvidenceAssumed {
		t.Errorf("evidence = %v, want assumed", check.Evidence)
	}
	if status, has := e.Checks.Aggregate(); !has || status != calculator.LimitUnmet {
		t.Errorf("aggregate = %v (has required %v), want unmet", status, has)
	}
	wantSI(t, "required minimum area", e.AreaLower.Value, fixtureMinAreaN1)
	wantControlling(t, e.AreaLower, caseNameN1)
}

// TestSizeAtStallLimitHoldingSpan is acceptance check 2: the action produces
// 0.4169494048 m^2, the derived chord and aspect ratio, and a stall speed
// exactly at the boundary. The preview reports the requirement change before
// anything is applied.
func TestSizeAtStallLimitHoldingSpan(t *testing.T) {
	s := calculator.NewSession("check-2", baseDesign(t))
	cmd := calculator.SizeAtStallLimit{
		Hold:  calculator.ParamSpanProjected,
		Scope: calculator.SingleCase(caseNameN1),
	}

	preview, err := s.Preview(cmd)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if len(preview.Changes) != 1 {
		t.Fatalf("preview changes = %+v, want exactly one", preview.Changes)
	}
	change := preview.Changes[0]
	if change.Name != reqStall || change.From != calculator.LimitUnmet || change.To != calculator.LimitMet {
		t.Errorf("preview change = %+v, want the stall ceiling going unmet to met", change)
	}
	if before := s.Design(); before.Wing.Drivers.AspectRatio != 6 {
		t.Errorf("preview applied the command: aspect ratio = %v", before.Wing.Drivers.AspectRatio)
	}

	mustDo(t, s, cmd)
	e := s.Evaluate()
	wantSI(t, "sized area", e.Wing.Projected.Area, fixtureMinAreaN1)
	wantSI(t, "span", e.Wing.Projected.Span, 1.2)
	wantSI(t, "root chord", e.Wing.Planform.RootChord, fixtureSizedChord)
	if !sameWithin(e.Wing.ProjectedAspectRatio, fixtureSizedAspect) {
		t.Errorf("aspect ratio = %v, want %v", e.Wing.ProjectedAspectRatio, fixtureSizedAspect)
	}

	check := requireCheck(t, e.Checks, reqStall, caseNameN1, calculator.BoundUpper)
	wantStatus(t, check, calculator.LimitMet, calculator.ResultComputed)
	wantSI(t, "boundary stall speed", check.Actual, 8)

	drivers := s.Design().DriverKeys()
	if len(drivers) != 2 || drivers[0] != calculator.ParamSpanProjected ||
		drivers[1] != calculator.ParamAreaReference {
		t.Errorf("drivers after sizing = %v, want span and reference area", drivers)
	}
}

// TestUndoRestoresDriverRoles is acceptance check 3: undo restores the complete
// former state, driver roles included.
func TestUndoRestoresDriverRoles(t *testing.T) {
	s := calculator.NewSession("check-3", baseDesign(t))
	before := s.Snapshot()
	beforeDrivers := s.Design().DriverKeys()

	mustDo(t, s, calculator.SizeAtStallLimit{
		Hold:  calculator.ParamSpanProjected,
		Scope: calculator.SingleCase(caseNameN1),
	})
	if s.Snapshot() == before {
		t.Fatal("the command did not change the design")
	}
	if err := s.Undo(); err != nil {
		t.Fatalf("undo: %v", err)
	}
	if s.Snapshot() != before {
		t.Error("undo did not restore the former inputs")
	}
	after := s.Design().DriverKeys()
	if len(after) != len(beforeDrivers) {
		t.Fatalf("driver roles after undo = %v, want %v", after, beforeDrivers)
	}
	for n := range beforeDrivers {
		if after[n] != beforeDrivers[n] {
			t.Fatalf("driver roles after undo = %v, want %v", after, beforeDrivers)
		}
	}
	if !s.CanRedo() {
		t.Error("undo should leave the sized revision available to redo")
	}
	e := s.Evaluate()
	wantSI(t, "area after undo", e.Wing.Projected.Area, fixtureArea)
}

// TestSpanAndAspectRequirementsConflictWithTheStallMinimum is acceptance
// check 4: a required span of at most 1.2 m with a required aspect ratio of at
// least 6 bounds the area at 0.24 m^2, which conflicts with the 0.4169494048
// m^2 minimum. Both conditional alternatives are offered and neither is applied.
func TestSpanAndAspectRequirementsConflictWithTheStallMinimum(t *testing.T) {
	d := baseDesign(t)
	d.Requirements = append(d.Requirements,
		calculator.Requirement{
			Name: reqSpan, Subject: calculator.SubjectSpan,
			Maximum: mustQ(t, 1.2, calculator.Meter), Priority: calculator.PriorityRequired,
			Basis: "fits the car boot",
		},
		calculator.Requirement{
			Name: reqAspect, Subject: calculator.SubjectAspectRatio,
			Minimum: mustQ(t, 6, calculator.One), Priority: calculator.PriorityRequired,
			Basis: "glide performance target for the fixture",
		})
	e := d.Evaluate()

	wantSI(t, "implied area ceiling", e.AreaUpper.Value, fixtureArea)
	wantControlling(t, e.AreaUpper, reqSpan+" with "+reqAspect)
	wantSI(t, "required minimum area", e.AreaLower.Value, fixtureMinAreaN1)

	if len(e.Conflicts) != 1 {
		t.Fatalf("conflicts = %+v, want exactly one", e.Conflicts)
	}
	conflict := e.Conflicts[0]
	if len(conflict.Group) != 2 {
		t.Errorf("conflict group = %v, want the controlling case and the derived ceiling", conflict.Group)
	}
	if len(conflict.Alternatives) != 2 {
		t.Fatalf("alternatives = %+v, want two", conflict.Alternatives)
	}

	span, ok := conflict.Alternatives[0].Command.(calculator.SetDriver)
	if !ok {
		t.Fatalf("first alternative = %T, want a span driver edit", conflict.Alternatives[0].Command)
	}
	wantSI(t, "alternative span", span.Value, fixtureSpanAtAspect)
	mass, ok := conflict.Alternatives[1].Command.(calculator.SetMass)
	if !ok {
		t.Fatalf("second alternative = %T, want a mass edit", conflict.Alternatives[1].Command)
	}
	wantSI(t, "alternative mass", mass.Mass, fixtureMaxMassN1)

	// Neither alternative is applied: the design still holds its own numbers.
	wantSI(t, "span after reporting", e.Wing.Projected.Span, 1.2)
	wantSI(t, "mass after reporting", e.Design.Mass, 2)
}

// TestConflictAlternativesAreOfferedNotApplied checks that an offered
// alternative is an ordinary command, so accepting one is the builder's edit
// and resolves the conflict it was offered for.
func TestConflictAlternativesAreOfferedNotApplied(t *testing.T) {
	d := baseDesign(t)
	d.Requirements = append(d.Requirements, calculator.Requirement{
		Name: reqArea, Subject: calculator.SubjectWingArea,
		Maximum: mustQ(t, 0.3, calculator.SquareMeter), Priority: calculator.PriorityRequired,
		Basis: "the sheet stock available",
	})
	e := d.Evaluate()
	if len(e.Conflicts) != 1 || len(e.Conflicts[0].Alternatives) == 0 {
		t.Fatalf("expected a conflict with alternatives, got %+v", e.Conflicts)
	}
	s := calculator.NewSession("alternatives", d)
	mustDo(t, s, e.Conflicts[0].Alternatives[0].Command)
	if after := s.Evaluate(); len(after.Conflicts) != 0 {
		t.Errorf("the offered alternative did not resolve the conflict: %+v", after.Conflicts)
	}
}

// TestEntryPointsAgreeOnTheSameCandidate is acceptance check 5: every algebraic
// entry point onto the same physical wing produces the same outputs, and
// reconstructing the definition through direct Go calls reproduces them without
// any session state.
func TestEntryPointsAgreeOnTheSameCandidate(t *testing.T) {
	span := 1.2
	area := fixtureArea
	aspect := span * span / area
	chord := area / span

	entries := map[string]calculator.PlanformDrivers{
		"span and area":  {Span: mustQ(t, span, calculator.Meter), Area: mustQ(t, area, calculator.SquareMeter)},
		"span and ratio": {Span: mustQ(t, span, calculator.Meter), AspectRatio: aspect},
		"area and ratio": {Area: mustQ(t, area, calculator.SquareMeter), AspectRatio: aspect},
		"span and chord": {Span: mustQ(t, span, calculator.Meter), RootChord: mustQ(t, chord, calculator.Meter)},
		"area and chord": {Area: mustQ(t, area, calculator.SquareMeter), RootChord: mustQ(t, chord, calculator.Meter)},
		"ratio and chord": {
			AspectRatio: aspect,
			RootChord:   mustQ(t, chord, calculator.Meter),
		},
	}
	for name, drivers := range entries {
		t.Run(name, func(t *testing.T) {
			d := baseDesign(t)
			drivers.Shape = calculator.ShapeRectangle
			d.Wing.Drivers = drivers
			e := d.Evaluate()
			if e.Geometry != calculator.ResultComputed {
				t.Fatalf("geometry = %v: %v", e.Geometry, e.GeometryIssues)
			}
			wantSI(t, "span", e.Wing.Projected.Span, span)
			wantSI(t, "area", e.Wing.Projected.Area, area)
			wantSI(t, "root chord", e.Wing.Planform.RootChord, chord)
			if !sameWithin(e.Wing.ProjectedAspectRatio, aspect) {
				t.Errorf("aspect ratio = %v, want %v", e.Wing.ProjectedAspectRatio, aspect)
			}
			check := requireCheck(t, e.Checks, reqStall, caseNameN1, calculator.BoundUpper)
			if !sameWithin(check.Actual.SI(), fixtureStallN1) {
				t.Errorf("stall speed = %v, want %v", check.Actual.SI(), fixtureStallN1)
			}
			wantSI(t, "required minimum area", e.AreaLower.Value, fixtureMinAreaN1)
		})
	}
}

// TestInputOrderWithinAModeDoesNotAlterTheSolution is the second half of
// acceptance check 5: two orders of the same edits reach the same definition,
// and therefore the same snapshot and the same results.
func TestInputOrderWithinAModeDoesNotAlterTheSolution(t *testing.T) {
	mass := calculator.SetMass{Mass: mustQ(t, 1.6, calculator.Kilogram), Basis: "revised payload"}
	taper := calculator.SetTaperRatio{Value: 1}
	area := calculator.SizeAtStallLimit{
		Hold: calculator.ParamSpanProjected, Scope: calculator.SingleCase(caseNameN1),
	}

	forward := calculator.NewSession("order-a", baseDesign(t))
	for _, cmd := range []calculator.Command{mass, taper, area} {
		mustDo(t, forward, cmd)
	}
	// Sizing last in both orders: it reads the mass, so moving it ahead of the
	// mass edit would be a different sequence of intentions, not a reordering
	// of the same one.
	reverse := calculator.NewSession("order-b", baseDesign(t))
	for _, cmd := range []calculator.Command{taper, mass, area} {
		mustDo(t, reverse, cmd)
	}
	if forward.Snapshot() != reverse.Snapshot() {
		t.Fatalf("input order changed the definition:\n a: %s\n b: %s",
			forward.Snapshot(), reverse.Snapshot())
	}
	a, b := forward.Evaluate(), reverse.Evaluate()
	if a.Wing.Projected.Area.SI() != b.Wing.Projected.Area.SI() {
		t.Errorf("areas differ: %v and %v", a.Wing.Projected.Area, b.Wing.Projected.Area)
	}
	if a.Request == b.Request {
		t.Error("two sessions must not mint the same request identity")
	}
}

// TestDirectGoCallsReproduceSessionResults is the third part of acceptance
// check 5: a design reconstructed and evaluated directly gives the session's
// results, so nothing depends on session state.
func TestDirectGoCallsReproduceSessionResults(t *testing.T) {
	s := calculator.NewSession("session", baseDesign(t))
	mustDo(t, s, calculator.SizeAtStallLimit{
		Hold: calculator.ParamSpanProjected, Scope: calculator.SingleCase(caseNameN1),
	})
	fromSession := s.Evaluate()

	direct := s.Design().Evaluate()
	if direct.Snapshot != fromSession.Snapshot {
		t.Fatal("a directly evaluated design has a different input snapshot")
	}
	if !direct.Request.IsZero() {
		t.Errorf("a direct evaluation minted an identity: %v", direct.Request)
	}
	if len(direct.Checks) != len(fromSession.Checks) {
		t.Fatalf("check counts differ: %d and %d", len(direct.Checks), len(fromSession.Checks))
	}
	for n := range direct.Checks {
		if direct.Checks[n].Status != fromSession.Checks[n].Status ||
			direct.Checks[n].Actual.SI() != fromSession.Checks[n].Actual.SI() {
			t.Errorf("check %d differs: %+v and %+v", n, direct.Checks[n], fromSession.Checks[n])
		}
	}
	if direct.AreaLower.Value.SI() != fromSession.AreaLower.Value.SI() {
		t.Error("the area bound differs between a session and a direct evaluation")
	}
}

// TestSizeAtStallLimitRefusesWithoutACaseScope holds the rule that the action
// must name its scope rather than defaulting to one.
func TestSizeAtStallLimitRefusesWithoutACaseScope(t *testing.T) {
	s := calculator.NewSession("scope", baseDesign(t))
	err := s.Do(calculator.SizeAtStallLimit{Hold: calculator.ParamSpanProjected})
	detail := wantIssue(t, err, "case_scope", calculator.IssueMissing)
	if detail == "" {
		t.Error("the refusal should explain why a scope is required")
	}
	if s.Revisions() != 1 {
		t.Error("a rejected command must not record a revision")
	}
}

// TestIncompleteTailDoesNotBlockLiftSizing holds the rule that an unrelated
// incomplete panel does not withhold a valid wing sizing.
func TestIncompleteTailDoesNotBlockLiftSizing(t *testing.T) {
	d := baseDesign(t)
	d.Configuration = calculator.ConfigurationConventionalTail
	e := d.Evaluate()
	if e.ConfigurationIssues == nil {
		t.Fatal("a conventional layout with no tail should report an incomplete airframe")
	}
	if e.Geometry != calculator.ResultComputed {
		t.Fatalf("the wing should still solve: %v", e.GeometryIssues)
	}
	wantSI(t, "reference area", e.Wing.Projected.Area, fixtureArea)
	wantSI(t, "required minimum area", e.AreaLower.Value, fixtureMinAreaN1)
	check := requireCheck(t, e.Checks, reqStall, caseNameN1, calculator.BoundUpper)
	wantStatus(t, check, calculator.LimitUnmet, calculator.ResultComputed)
}

// TestInvalidGeometryBlocksDependentResults holds the rule that numeric
// invalidity blocks dependent calculations rather than approximating them, and
// that the mass, which no geometry feeds, still reads.
func TestInvalidGeometryBlocksDependentResults(t *testing.T) {
	d := baseDesign(t)
	d.Wing.Drivers.Shape = calculator.ShapeTrapezoid
	d.Wing.Drivers.TaperRatio = 0
	d.Requirements = append(d.Requirements, calculator.Requirement{
		Name: reqSpan, Subject: calculator.SubjectSpan,
		Maximum: mustQ(t, 1.4, calculator.Meter), Priority: calculator.PriorityRequired,
		Basis: "doorway",
	})
	e := d.Evaluate()
	if e.Geometry == calculator.ResultComputed {
		t.Fatal("a trapezoid with no taper ratio must not solve")
	}
	wantStatus(t, requireCheck(t, e.Checks, reqSpan, "", calculator.BoundUpper),
		calculator.LimitUnknown, calculator.ResultMissing)

	d.Requirements = append(d.Requirements, calculator.Requirement{
		Name: "minimum mass", Subject: calculator.SubjectMass,
		Minimum: mustQ(t, 1, calculator.Kilogram), Priority: calculator.PriorityRequired,
		Basis: "the components already chosen",
	})
	withMass := d.Evaluate()
	wantStatus(t, requireCheck(t, withMass.Checks, "minimum mass", "", calculator.BoundLower),
		calculator.LimitMet, calculator.ResultComputed)
}

// TestSingleCaseSizingStillReassessesEveryRequirement holds the rule that a
// single-case action names only the bound it sizes from, and does not narrow
// what is then assessed: sizing for n=1 leaves the n=2 requirement unmet and
// says so.
func TestSingleCaseSizingStillReassessesEveryRequirement(t *testing.T) {
	s := calculator.NewSession("single-case", bothCasesDesign(t))
	mustDo(t, s, calculator.SizeAtStallLimit{
		Hold: calculator.ParamSpanProjected, Scope: calculator.SingleCase(caseNameN1),
	})
	e := s.Evaluate()
	wantSI(t, "area sized for n=1", e.Wing.Projected.Area, fixtureMinAreaN1)
	wantStatus(t, requireCheck(t, e.Checks, reqStall, caseNameN1, calculator.BoundUpper),
		calculator.LimitMet, calculator.ResultComputed)
	wantStatus(t, requireCheck(t, e.Checks, reqStall, caseNameN2, calculator.BoundUpper),
		calculator.LimitUnmet, calculator.ResultComputed)
	if status, has := e.Checks.Aggregate(); !has || status != calculator.LimitUnmet {
		t.Errorf("aggregate = %v, want unmet: the n=2 case was never sized for", status)
	}
	wantSI(t, "all-case minimum area", e.AreaLower.Value, fixtureMinAreaN2)
	wantControlling(t, e.AreaLower, caseNameN2)
}

// TestChecksAndBoundsCarryTheirProvenance holds that a status is traceable back
// to the equation and revision that produced the number behind it, so a stored
// result can be matched against the model that produced it.
func TestChecksAndBoundsCarryTheirProvenance(t *testing.T) {
	e := baseDesign(t).Evaluate()
	check := requireCheck(t, e.Checks, reqStall, caseNameN1, calculator.BoundUpper)
	if check.Trace.EquationID != calculator.EqStallSpeed {
		t.Errorf("check trace equation = %q, want %q", check.Trace.EquationID, calculator.EqStallSpeed)
	}
	eq, err := calculator.Lookup(check.Trace.EquationID)
	if err != nil {
		t.Fatalf("lookup %s: %v", check.Trace.EquationID, err)
	}
	if check.Trace.Revision != eq.Revision {
		t.Errorf("trace revision = %q, want %q", check.Trace.Revision, eq.Revision)
	}
	if _, ok := check.Trace.Substitution("clmax"); !ok {
		t.Error("the trace should record the CLmax the status rests on")
	}

	if len(e.AreaLower.Contributions) != 1 {
		t.Fatalf("contributions = %d, want one", len(e.AreaLower.Contributions))
	}
	if id := e.AreaLower.Contributions[0].Trace.EquationID; id != calculator.EqMinimumWingArea {
		t.Errorf("bound trace equation = %q, want %q", id, calculator.EqMinimumWingArea)
	}
}

// TestAnUnmetCandidateStaysEditableAndSaveable holds that failing a requirement
// is not a reason to refuse an edit or a draft: the candidate round-trips
// through a save and load with its status intact.
func TestAnUnmetCandidateStaysEditableAndSaveable(t *testing.T) {
	s := calculator.NewSession("draft", baseDesign(t))
	unmet := s.Evaluate()
	if unmet.Aggregate != calculator.LimitUnmet {
		t.Fatalf("the fixture candidate should be unmet, got %v", unmet.Aggregate)
	}
	draft := s.Design()

	fresh := calculator.NewSession("reloaded", baseDesign(t))
	mustDo(t, fresh, calculator.SetMass{Mass: mustQ(t, 5, calculator.Kilogram), Basis: "scratch"})
	fresh.Load(draft)
	if fresh.Snapshot() != unmet.Snapshot {
		t.Error("loading the draft did not restore its inputs")
	}
	reloaded := fresh.Evaluate()
	if reloaded.Aggregate != calculator.LimitUnmet {
		t.Errorf("reloaded aggregate = %v, want unmet", reloaded.Aggregate)
	}
	mustDo(t, fresh, calculator.SetDriver{
		Key: calculator.ParamSpanProjected, Value: mustQ(t, 1.6, calculator.Meter),
	})
	if !fresh.CanUndo() {
		t.Error("an unmet candidate stays editable, with its edit undoable")
	}
}
