package calculator_test

import (
	"testing"

	"yalb.aero/calculator"
)

// TestRequestIdentityIsNeverRestored is acceptance check 9: editing, undoing
// and then making a different edit while earlier evaluations are outstanding
// keeps every identity unique, and no earlier result can be accepted for the
// new branch.
func TestRequestIdentityIsNeverRestored(t *testing.T) {
	s := calculator.NewSession("branching", baseDesign(t))
	first := s.Evaluate()
	if err := s.Accept(first); err != nil {
		t.Fatalf("the first result answers the current request: %v", err)
	}

	mustDo(t, s, calculator.SetMass{Mass: mustQ(t, 1.8, calculator.Kilogram), Basis: "branch A"})
	second := s.Evaluate()
	if err := s.Undo(); err != nil {
		t.Fatalf("undo: %v", err)
	}
	if err := s.Accept(second); err == nil {
		t.Error("undo must not leave an earlier result acceptable")
	}
	mustDo(t, s, calculator.SetMass{Mass: mustQ(t, 2.4, calculator.Kilogram), Basis: "branch B"})
	third := s.Evaluate()

	ids := []calculator.RequestID{first.Request, second.Request, third.Request}
	for i := range ids {
		for j := i + 1; j < len(ids); j++ {
			if ids[i] == ids[j] {
				t.Fatalf("request identities repeat: %v", ids)
			}
		}
	}
	if err := s.Accept(first); err == nil {
		t.Error("a pre-branch result must not be accepted for the new branch")
	}
	if err := s.Accept(second); err == nil {
		t.Error("an abandoned branch's result must not be accepted")
	}
	if err := s.Accept(third); err != nil {
		t.Errorf("the current branch's result should be accepted: %v", err)
	}
}

// TestUndoingBackToAnEvaluatedStateDoesNotRestoreItsIdentity holds the rule
// that identities are never restored, in the one case where the input snapshot
// alone cannot tell: undoing back to the exact design a result was computed
// from. The inputs match again, and the result is still not acceptable, because
// the request it answered was retired the moment the design changed.
func TestUndoingBackToAnEvaluatedStateDoesNotRestoreItsIdentity(t *testing.T) {
	s := calculator.NewSession("round-trip", baseDesign(t))
	held := s.Evaluate()
	mustDo(t, s, calculator.SetMass{Mass: mustQ(t, 1.8, calculator.Kilogram), Basis: "branch"})
	if err := s.Undo(); err != nil {
		t.Fatalf("undo: %v", err)
	}
	if s.Snapshot() != held.Snapshot {
		t.Fatal("undo should have restored the exact inputs the result was computed from")
	}
	if s.Freshness(held) != calculator.ResultStale {
		t.Error("a restored design does not restore the identity the result answered")
	}
	if err := s.Accept(held); err == nil {
		t.Error("matching inputs are not enough; the request identity was retired by the edit")
	}
	fresh := s.Evaluate()
	if fresh.Request == held.Request {
		t.Error("the new request must not reuse the retired identity")
	}
	if err := s.Accept(fresh); err != nil {
		t.Errorf("the freshly requested result should be accepted: %v", err)
	}
}

// TestRedoAndDraftLoadingMintFreshIdentities repeats acceptance check 9 for the
// other two ways of restoring a design.
func TestRedoAndDraftLoadingMintFreshIdentities(t *testing.T) {
	s := calculator.NewSession("restores", baseDesign(t))
	mustDo(t, s, calculator.SetMass{Mass: mustQ(t, 1.8, calculator.Kilogram), Basis: "revised"})
	if err := s.Undo(); err != nil {
		t.Fatalf("undo: %v", err)
	}
	beforeRedo := s.Evaluate()
	if err := s.Redo(); err != nil {
		t.Fatalf("redo: %v", err)
	}
	if err := s.Accept(beforeRedo); err == nil {
		t.Error("redo must not leave the pre-redo result acceptable")
	}
	afterRedo := s.Evaluate()
	if afterRedo.Request == beforeRedo.Request {
		t.Error("redo must mint a fresh identity, not restore one")
	}

	draft := baseDesign(t)
	draft.Name = "loaded draft"
	s.Load(draft)
	if err := s.Accept(afterRedo); err == nil {
		t.Error("loading a draft must not leave an earlier result acceptable")
	}
	afterLoad := s.Evaluate()
	if afterLoad.Request.Sequence <= afterRedo.Request.Sequence {
		t.Errorf("the request stream must keep counting across a load: %v then %v",
			afterRedo.Request, afterLoad.Request)
	}
	if err := s.Accept(afterLoad); err != nil {
		t.Errorf("the loaded draft's own result should be accepted: %v", err)
	}
}

// TestASecondEvaluationSupersedesTheFirst holds that an evaluation is a request
// rather than a cache key: asking twice makes only the later answer current.
func TestASecondEvaluationSupersedesTheFirst(t *testing.T) {
	s := calculator.NewSession("supersede", baseDesign(t))
	first := s.Evaluate()
	second := s.Evaluate()
	if first.Request == second.Request {
		t.Fatal("two evaluations must not share an identity")
	}
	if first.Snapshot != second.Snapshot {
		t.Error("nothing changed, so the input snapshots should match")
	}
	if err := s.Accept(first); err == nil {
		t.Error("only the latest request is current")
	}
	if err := s.Accept(second); err != nil {
		t.Errorf("the latest result should be accepted: %v", err)
	}
}

// TestStaleResultsAreNotShownAsCurrent is acceptance check 6's stale outcome:
// an edit makes a held result stale, and marking it stale replaces every
// requirement status rather than leaving a met one standing.
func TestStaleResultsAreNotShownAsCurrent(t *testing.T) {
	s := calculator.NewSession("stale", baseDesign(t))
	held := s.Evaluate()
	if s.Freshness(held) != calculator.ResultComputed {
		t.Fatal("a just-computed result is current")
	}
	mustDo(t, s, calculator.SetMass{Mass: mustQ(t, 1, calculator.Kilogram), Basis: "revised payload"})
	if s.Freshness(held) != calculator.ResultStale {
		t.Error("an edited design makes a held result stale")
	}
	if err := s.Accept(held); err == nil {
		t.Error("a stale result must not be accepted")
	}

	stale := held.Stale()
	if len(stale.Checks) != len(held.Checks) {
		t.Fatalf("marking stale changed the check count: %d and %d", len(stale.Checks), len(held.Checks))
	}
	for _, check := range stale.Checks {
		if check.Result != calculator.ResultStale || check.Status != calculator.LimitUnknown {
			t.Errorf("stale check %+v still reports a current outcome", check)
		}
	}
	if _, has := stale.Checks.Aggregate(); !has || stale.Aggregate != calculator.LimitUnknown {
		t.Errorf("stale aggregate = %v, want unknown", stale.Aggregate)
	}
	if held.Checks[0].Result == calculator.ResultStale {
		t.Error("Stale must not mutate the evaluation it was called on")
	}
}

// TestBoundaryToleranceIsNarrow is acceptance check 6's boundary outcome: a
// value exactly on its bound is met, a value inside the tolerance is met, and a
// value outside it is unmet.
func TestBoundaryToleranceIsNarrow(t *testing.T) {
	areaFor := func(t *testing.T, scale float64) calculator.RequirementCheck {
		t.Helper()
		d := baseDesign(t)
		d.Wing.Drivers.AspectRatio = 0
		d.Wing.Drivers.Area = mustQ(t, fixtureMinAreaN1*scale, calculator.SquareMeter)
		e := d.Evaluate()
		return requireCheck(t, e.Checks, reqStall, caseNameN1, calculator.BoundUpper)
	}
	for _, tc := range []struct {
		name  string
		scale float64
		want  calculator.LimitStatus
	}{
		{"exactly on the bound", 1, calculator.LimitMet},
		{"inside the tolerance", 1 - 1e-12, calculator.LimitMet},
		{"outside the tolerance", 1 - 1e-6, calculator.LimitUnmet},
		{"comfortably inside", 1.2, calculator.LimitMet},
	} {
		t.Run(tc.name, func(t *testing.T) {
			check := areaFor(t, tc.scale)
			if check.Status != tc.want {
				t.Errorf("status at area scale %v = %v, want %v (stall %v against %v)",
					tc.scale, check.Status, tc.want, check.Actual, check.Bound)
			}
		})
	}
}

// TestMarginTightensTheBoundItIsStatedOn holds that a requirement's margin is
// applied to the bound rather than reported alongside it and forgotten.
func TestMarginTightensTheBoundItIsStatedOn(t *testing.T) {
	d := baseDesign(t)
	d.Requirements[0].Margin = 0.1
	e := d.Evaluate()
	check := requireCheck(t, e.Checks, reqStall, caseNameN1, calculator.BoundUpper)
	wantSI(t, "effective ceiling", check.Bound, 7.2)
	// S_min scales as 1/Vs^2, so a ceiling of 7.2 m/s needs (8/7.2)^2 times the
	// area the stated 8 m/s ceiling does.
	wantSI(t, "minimum area with margin", e.AreaLower.Value, fixtureMinAreaN1*(8.0/7.2)*(8.0/7.2))
}

// TestRejectedCommandsChangeNothing holds that a failed edit leaves the design,
// the history and the request stream exactly as they were.
func TestRejectedCommandsChangeNothing(t *testing.T) {
	s := calculator.NewSession("rejects", baseDesign(t))
	before := s.Snapshot()
	revisions := s.Revisions()
	for _, cmd := range []calculator.Command{
		calculator.SetMass{Mass: mustQ(t, 2, calculator.SquareMeter), Basis: "wrong dimension"},
		calculator.RemoveRequirement{Name: "no such requirement"},
		calculator.SetCasePriority{Name: caseNameN1},
		calculator.SetTaperRatio{Value: -1},
		calculator.RemoveCase{Name: caseNameN1},
	} {
		if err := s.Do(cmd); err == nil {
			t.Errorf("%s should have been rejected", cmd.Label())
		}
	}
	if s.Snapshot() != before {
		t.Error("a rejected command changed the design")
	}
	if s.Revisions() != revisions {
		t.Errorf("revisions = %d, want %d", s.Revisions(), revisions)
	}
}

// TestRemoveCaseRefusesWhileARequirementNamesIt holds that a bound whose case
// has vanished is prevented rather than reported as unknown forever.
func TestRemoveCaseRefusesWhileARequirementNamesIt(t *testing.T) {
	s := calculator.NewSession("remove-case", baseDesign(t))
	err := s.Do(calculator.RemoveCase{Name: caseNameN1})
	detail := wantIssue(t, err, "case."+caseNameN1, calculator.IssueInvalid)
	if detail == "" {
		t.Error("the refusal should name the requirement that still applies")
	}
	mustDo(t, s, calculator.RemoveRequirement{Name: reqStall})
	mustDo(t, s, calculator.RemoveCase{Name: caseNameN1})
	if len(s.Design().Cases) != 0 {
		t.Error("the case should be gone once nothing refers to it")
	}
}

// TestPreviewLeavesTheSessionAlone holds that a preview commits nothing: the
// design, the history and the current request are untouched, and neither of the
// preview's own evaluations can be accepted.
func TestPreviewLeavesTheSessionAlone(t *testing.T) {
	s := calculator.NewSession("preview", baseDesign(t))
	current := s.Evaluate()
	before := s.Snapshot()
	revisions := s.Revisions()

	preview, err := s.Preview(calculator.SetMass{
		Mass: mustQ(t, 1.1, calculator.Kilogram), Basis: "lighter payload",
	})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if s.Snapshot() != before || s.Revisions() != revisions {
		t.Error("a preview must not commit anything")
	}
	if err := s.Accept(current); err != nil {
		t.Errorf("a preview must not invalidate the current request: %v", err)
	}
	if err := s.Accept(preview.After); err == nil {
		t.Error("a preview describes a design the session does not hold")
	}
	if preview.Before.Request == preview.After.Request {
		t.Error("the two sides of a preview must carry distinct identities")
	}
	if preview.After.Snapshot == before {
		t.Error("the preview's after side should describe the edited design")
	}
	if len(preview.Changes) == 0 {
		t.Error("dropping the mass to 1.1 kg meets the stall ceiling and should show as a change")
	}
}

// TestAMassWithoutItsBasisIsRecordedAndReported holds where that requirement
// lives. Entering a mass before saying where it came from is an ordinary order
// to work in, so the edit is accepted; the design then reports the missing
// provenance every time it is validated, which is what keeps a target mass from
// being read as a measured one.
func TestAMassWithoutItsBasisIsRecordedAndReported(t *testing.T) {
	s := calculator.NewSession("basis", baseDesign(t))
	mustDo(t, s, calculator.SetMass{Mass: mustQ(t, 1.6, calculator.Kilogram)})

	d := s.Design()
	wantSI(t, "mass", d.Mass, 1.6)
	detail := wantIssue(t, d.Validate(), "mass", calculator.IssueMissing)
	if !containsSubstring(detail, "came from") {
		t.Errorf("the issue %q should ask where the mass came from", detail)
	}
	e := s.Evaluate()
	if e.DefinitionIssues == nil {
		t.Error("every evaluation of that design should carry the missing basis")
	}
	// It blocks nothing: the wing still solves and the bounds still hold.
	if e.Geometry != calculator.ResultComputed {
		t.Fatalf("the wing should still solve: %v", e.GeometryIssues)
	}

	mustDo(t, s, calculator.SetMass{Mass: mustQ(t, 1.6, calculator.Kilogram), Basis: "measured on the bench"})
	if err := s.Design().Validate(); err != nil {
		t.Errorf("with the basis stated the design validates: %v", err)
	}
}
