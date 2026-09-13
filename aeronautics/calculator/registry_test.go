package calculator_test

import (
	"testing"

	"yalb.aero/calculator"
)

// evaluatedEquations lists every equation this package can evaluate. The
// evaluation harness looks equations up by ID without panicking, so this test
// is what guarantees an ID typo cannot silently produce an empty trace.
var evaluatedEquations = []string{
	calculator.EqWeightFromMass,
	calculator.EqDynamicPressure,
	calculator.EqRequiredLift,
	calculator.EqRequiredCL,
	calculator.EqWingLoadingForce,
	calculator.EqWingLoadingMass,
	calculator.EqStallSpeed,
	calculator.EqMinimumWingArea,
	calculator.EqMaximumMass,
	calculator.EqMaximumWingLoadingForce,

	calculator.EqAspectRatio,
	calculator.EqSpanFromAreaAndAspect,
	calculator.EqAreaFromSpanAndAspect,
	calculator.EqSpanFromAreaAndRootChord,
	calculator.EqAreaFromSpanAndRootChord,
	calculator.EqSpanFromAspectAndRootChord,
	calculator.EqRootChord,
	calculator.EqTipChord,
	calculator.EqMeanChord,
	calculator.EqMeanAerodynamicChord,
	calculator.EqMACStation,
	calculator.EqChordAtStation,
	calculator.EqSweepTransform,
	calculator.EqTipLeadingEdgeOffset,
	calculator.EqTipRise,
	calculator.EqMACLeadingEdgeStation,
	calculator.EqExposedArea,
	calculator.EqProjectedSpan,
	calculator.EqProjectedArea,
	calculator.EqPanelSpan,
	calculator.EqPanelArea,
	calculator.EqSemiSpan,
	calculator.EqReynolds,

	calculator.EqTotalMass,
	calculator.EqComponentMoment,
	calculator.EqMomentSum,
	calculator.EqCenterOfGravity,
	calculator.EqStationFractionOfMAC,

	calculator.EqInducedDragFactor,
	calculator.EqDragCoefficient,
	calculator.EqDragForce,
	calculator.EqLiftToDrag,
	calculator.EqOswaldStraightWing,
	calculator.EqMinimumPowerCL,

	calculator.EqElectricalPower,
	calculator.EqPropulsivePower,
	calculator.EqElectricalRequired,
	calculator.EqThrustToWeight,
	calculator.EqPropellerClearance,
	calculator.EqAuxiliaryPackSide,
	calculator.EqAuxiliarySum,
	calculator.EqSegmentLift,
	calculator.EqSegmentLiftCoefficient,
	calculator.EqRequiredThrust,
	calculator.EqBatteryEnergy,
	calculator.EqBatteryUsableEnergy,
	calculator.EqEnergyBudget,
	calculator.EqSegmentEnergy,
	calculator.EqEnergySum,
	calculator.EqEnduranceConstantDraw,
	calculator.EqGroundSpeed,
	calculator.EqSegmentDistance,
	calculator.EqSegmentDuration,
	calculator.EqDistanceSum,
	calculator.EqDurationSum,
}

func TestRegistryDefinesEveryEvaluatedEquation(t *testing.T) {
	for _, id := range evaluatedEquations {
		if _, err := calculator.Lookup(id); err != nil {
			t.Errorf("equation %q is evaluated but not registered: %v", id, err)
		}
	}
	if got, want := len(calculator.EquationIDs()), len(evaluatedEquations); got != want {
		t.Errorf("registry holds %d equations, evaluators cover %d", got, want)
	}
}

// Every registered equation must carry the provenance the task requires, so a
// result can always be traced back to a source and its adaptation.
func TestEveryEquationCarriesItsProvenance(t *testing.T) {
	for _, id := range calculator.EquationIDs() {
		eq, err := calculator.Lookup(id)
		if err != nil {
			t.Fatalf("Lookup(%q): %v", id, err)
		}
		t.Run(id, func(t *testing.T) {
			assertEquationShape(t, eq)
			assertSourceShape(t, eq.Source)
		})
	}
}

func assertEquationShape(t *testing.T, eq calculator.Equation) {
	t.Helper()
	if eq.Revision == "" {
		t.Error("no revision")
	}
	if eq.Expression == "" {
		t.Error("no expression")
	}
	if len(eq.Inputs) == 0 {
		t.Error("no declared inputs")
	}
	if eq.Output.Name == "" {
		t.Error("no output port")
	}
	if len(eq.Assumptions) == 0 {
		t.Error("no stated assumptions")
	}
}

func assertSourceShape(t *testing.T, src calculator.Source) {
	t.Helper()
	if src.URL == "" || src.Title == "" {
		t.Error("source is not identified")
	}
	if src.Accessed == "" && src.UpstreamRevision == "" {
		t.Error("source records neither an access date nor an upstream revision")
	}
	if src.OriginalUnits == "" {
		t.Error("source does not record its original units")
	}
	if src.Adaptation == "" {
		t.Error("source does not record how it was adapted")
	}
}

func TestUnknownEquationLookupFails(t *testing.T) {
	if _, err := calculator.Lookup("no.such.equation"); err == nil {
		t.Error("Lookup accepted an unregistered ID")
	}
}

// Inspecting metadata must not be able to change it for anyone else.
func TestMetadataInspectionCannotMutateTheRegistry(t *testing.T) {
	first, err := calculator.Lookup(calculator.EqStallSpeed)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	// Only the slice fields can alias the registry. Equation is returned by
	// value, so writing to a returned copy's scalar fields is dead by
	// construction and would prove nothing; Inputs and Assumptions are the
	// fields that would share backing arrays without an explicit clone.
	first.Inputs[0].Name = "tampered"
	first.Assumptions[0] = "tampered"

	second, err := calculator.Lookup(calculator.EqStallSpeed)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if second.Inputs[0].Name == "tampered" {
		t.Error("registry input ports were mutated through a returned Equation")
	}
	if second.Assumptions[0] == "tampered" {
		t.Error("registry assumptions were mutated through a returned Equation")
	}
}

// A trace must report the equation and revision actually used, and the values
// actually substituted, so a stored result can be matched to its model.
func TestTraceMatchesSuppliedValuesAndRevision(t *testing.T) {
	ev := eval{t}
	fc := baseCase(t)
	mass := mustQ(t, 2, calculator.Kilogram)
	area := mustQ(t, 0.24, calculator.SquareMeter)

	r := ev.ok(calculator.StallSpeed(fc, mass, area))
	eq, err := calculator.Lookup(calculator.EqStallSpeed)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if r.Trace.EquationID != eq.ID {
		t.Errorf("trace equation %q, want %q", r.Trace.EquationID, eq.ID)
	}
	if r.Trace.Revision != eq.Revision {
		t.Errorf("trace revision %q, want %q", r.Trace.Revision, eq.Revision)
	}
	if r.Trace.Expression != eq.Expression {
		t.Errorf("trace expression %q, want %q", r.Trace.Expression, eq.Expression)
	}
	if r.Trace.Result != r.Value {
		t.Error("trace result does not match the returned value")
	}

	// Every declared input appears exactly once, with the supplied value.
	if len(r.Trace.Substitutions) != len(eq.Inputs) {
		t.Fatalf("trace has %d substitutions, equation declares %d inputs",
			len(r.Trace.Substitutions), len(eq.Inputs))
	}
	for _, port := range eq.Inputs {
		q, ok := r.Trace.Substitution(port.Name)
		if !ok {
			t.Errorf("trace omits declared input %q", port.Name)
			continue
		}
		if q.Dimension() != port.Dimension {
			t.Errorf("trace records %q as %v, want %v", port.Name, q.Dimension(), port.Dimension)
		}
	}
	if got, _ := r.Trace.Substitution("mass"); got != mass {
		t.Errorf("trace mass %v, want %v", got, mass)
	}
	if got, _ := r.Trace.Substitution("wing_area"); got != area {
		t.Errorf("trace wing_area %v, want %v", got, area)
	}
	if got, _ := r.Trace.Substitution("density"); got != fc.Density {
		t.Errorf("trace density %v, want %v", got, fc.Density)
	}
}

// Evaluating must not touch the caller's inputs, and two evaluations must not
// share trace storage.
func TestEvaluationDoesNotMutateCallerData(t *testing.T) {
	ev := eval{t}
	fc := baseCase(t)
	before := fc
	mass := mustQ(t, 2, calculator.Kilogram)
	area := mustQ(t, 0.24, calculator.SquareMeter)
	massBefore, areaBefore := mass, area

	first := ev.ok(calculator.StallSpeed(fc, mass, area))
	if fc != before {
		t.Error("the flight case was mutated by evaluation")
	}
	if mass != massBefore || area != areaBefore {
		t.Error("an input quantity was mutated by evaluation")
	}

	// Tampering with one trace must not affect a later evaluation.
	first.Trace.Substitutions[0].Name = "tampered"
	second := ev.ok(calculator.StallSpeed(fc, mass, area))
	if second.Trace.Substitutions[0].Name == "tampered" {
		t.Error("two evaluations share trace storage")
	}
}

func TestEquationIDsAreSorted(t *testing.T) {
	ids := calculator.EquationIDs()
	for i := 1; i < len(ids); i++ {
		if ids[i-1] >= ids[i] {
			t.Fatalf("EquationIDs not sorted at %d: %q then %q", i, ids[i-1], ids[i])
		}
	}
}

// Only the methods from a chapter that was actually read may claim the book as
// their source, and each may cite only the chapter it came from. The Lift
// chapter was checked on the recorded access date and contains no stall-speed
// equation and no inversion of the lift identity, so the Task 02 family cites
// the standard identity and its own derivations. The Wing Planform Sizing
// chapter was checked on the same date and does contain the planform relations
// below. The Center of gravity chapter was checked for Task 07 and contains the
// weighted-mean relation, written over component weights rather than masses.
// Adding a method to this map is a deliberate act: provenance cannot drift
// upward, and it cannot drift sideways onto a chapter that does not hold it.
var bookSourcedEquations = map[string]string{
	calculator.EqAspectRatio:           wingLayoutURL,
	calculator.EqSpanFromAreaAndAspect: wingLayoutURL,
	calculator.EqRootChord:             wingLayoutURL,
	calculator.EqTipChord:              wingLayoutURL,
	calculator.EqMeanAerodynamicChord:  wingLayoutURL,
	calculator.EqMACStation:            wingLayoutURL,
	calculator.EqSweepTransform:        wingLayoutURL,

	calculator.EqTotalMass:       centerOfGravityURL,
	calculator.EqComponentMoment: centerOfGravityURL,
	calculator.EqMomentSum:       centerOfGravityURL,
	calculator.EqCenterOfGravity: centerOfGravityURL,

	// The Drag Polar chapter was read on 2026-09-07 for Task 09 and does
	// contain the parabolic polar and the Raymer straight-wing Oswald
	// correlation it quotes.
	calculator.EqInducedDragFactor:  dragPolarURL,
	calculator.EqDragCoefficient:    dragPolarURL,
	calculator.EqOswaldStraightWing: dragPolarURL,

	// The Mission analysis chapter was read on the same date. Only these two
	// relations are taken from it: its own mission method is a piston
	// fuel-fraction analysis and is not implemented.
	calculator.EqLiftToDrag:     missionAnalysisURL,
	calculator.EqMinimumPowerCL: missionAnalysisURL,
}

const (
	wingLayoutURL      = "https://computationaldesignlab.github.io/aircraft-design/wing_layout.html"
	centerOfGravityURL = "https://computationaldesignlab.github.io/aircraft-design/weight_and_balance/cg.html"
	dragPolarURL       = "https://computationaldesignlab.github.io/aircraft-design/aerodynamics/drag_polar_induced_drag.html"
	missionAnalysisURL = "https://computationaldesignlab.github.io/aircraft-design/performance/mission_analysis.html"
)

func TestBookProvenanceIsLimitedToCheckedChapterMethods(t *testing.T) {
	for _, id := range calculator.EquationIDs() {
		eq, err := calculator.Lookup(id)
		if err != nil {
			t.Fatalf("Lookup(%q): %v", id, err)
		}
		chapter, listed := bookSourcedEquations[id]
		claimsBook := eq.Source.Kind == calculator.SourceBook
		switch {
		case claimsBook && !listed:
			t.Errorf("equation %q claims book provenance but is not one of the checked "+
				"chapters' methods", id)
		case !claimsBook && listed:
			t.Errorf("equation %q is a checked chapter method but no longer claims it", id)
		case claimsBook && eq.Source.URL != chapter:
			t.Errorf("equation %q claims the book but cites %q rather than %q",
				id, eq.Source.URL, chapter)
		}
	}
}

// The Task 07 mass family carries the CG chapter's relation and its own
// derivation, and nothing else. Expressing a station as a fraction of the mean
// aerodynamic chord is a rearrangement this project made for the worksheet, not
// a method the chapter states, so it must stay derived.
func TestMACFractionIsDerivedRatherThanBookSourced(t *testing.T) {
	eq, err := calculator.Lookup(calculator.EqStationFractionOfMAC)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if eq.Source.Kind != calculator.SourceDerived {
		t.Errorf("source kind = %v, want derived; the chapter states no chord-fraction relation",
			eq.Source.Kind)
	}
}

// The Task 02 lift family specifically must never acquire book provenance.
func TestLiftSubsetDoesNotClaimBookProvenance(t *testing.T) {
	for _, id := range []string{
		calculator.EqWeightFromMass,
		calculator.EqDynamicPressure,
		calculator.EqRequiredLift,
		calculator.EqRequiredCL,
		calculator.EqWingLoadingForce,
		calculator.EqWingLoadingMass,
		calculator.EqStallSpeed,
		calculator.EqMinimumWingArea,
		calculator.EqMaximumMass,
		calculator.EqMaximumWingLoadingForce,
	} {
		eq, err := calculator.Lookup(id)
		if err != nil {
			t.Fatalf("Lookup(%q): %v", id, err)
		}
		if eq.Source.Kind == calculator.SourceBook {
			t.Errorf("equation %q claims book provenance; the Lift chapter contains no "+
				"stall-speed equation or inversion, so this attribution overstates the source", id)
		}
	}
}

// The applicability limit the book does establish must be stated by every
// equation that consumes a whole-aircraft CLmax.
func TestCLmaxConsumersStateTheirApplicabilityLimit(t *testing.T) {
	consumers := []string{
		calculator.EqStallSpeed,
		calculator.EqMinimumWingArea,
		calculator.EqMaximumMass,
		calculator.EqMaximumWingLoadingForce,
	}
	for _, id := range consumers {
		eq, err := calculator.Lookup(id)
		if err != nil {
			t.Fatalf("Lookup(%q): %v", id, err)
		}
		if !mentionsAircraftCLmax(eq.Assumptions) {
			t.Errorf("equation %q consumes CLmax without stating that it must be a "+
				"whole-aircraft coefficient", id)
		}
	}
}

func mentionsAircraftCLmax(assumptions []string) bool {
	for _, a := range assumptions {
		if a != "" && containsAll(a, "whole-aircraft", "CLmax") {
			return true
		}
	}
	return false
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !contains(s, sub) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
