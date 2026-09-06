package calculator_test

import (
	"math"
	"testing"

	"yalb.aero/calculator"
)

// A field never supplied is reported as missing, not as an invalid zero: the
// worksheet must be able to tell "not entered yet" from "entered as nonsense".
func TestUnsuppliedInputIsMissing(t *testing.T) {
	ev := eval{t}
	var unset calculator.Quantity
	err := ev.fails(calculator.Weight(unset))
	detail := wantIssue(t, err, "mass", calculator.IssueMissing)
	if detail == "" {
		t.Error("missing-mass issue carries no guidance")
	}
}

// A supplied zero is a different answer from an unsupplied field.
func TestZeroDenominatorIsInvalidNotMissing(t *testing.T) {
	ev := eval{t}
	mass := mustQ(t, 2, calculator.Kilogram)
	zeroArea := mustQ(t, 0, calculator.SquareMeter)
	err := ev.fails(calculator.WingLoadingForce(mass, zeroArea))
	wantIssue(t, err, "wing_area", calculator.IssueInvalid)

	issues, _ := calculator.AsIssues(err)
	if issues.Kind(calculator.IssueMissing) {
		t.Error("a supplied zero must not be reported as a missing field")
	}
}

func TestNegativeInputIsInvalid(t *testing.T) {
	ev := eval{t}
	err := ev.fails(calculator.Weight(mustQ(t, -2, calculator.Kilogram)))
	wantIssue(t, err, "mass", calculator.IssueInvalid)
}

// Dimensional confusion is rejected rather than silently reinterpreted.
func TestIncompatibleDimensionIsRejected(t *testing.T) {
	ev := eval{t}
	area := mustQ(t, 0.24, calculator.SquareMeter)
	// An area handed to the mass port.
	err := ev.fails(calculator.Weight(area))
	detail := wantIssue(t, err, "mass", calculator.IssueInvalid)
	if detail == "" {
		t.Error("dimension mismatch reported without detail")
	}
}

func TestUnitConversionAcrossDimensionsFails(t *testing.T) {
	mass := mustQ(t, 2, calculator.Kilogram)
	if _, err := mass.In(calculator.Newton); err == nil {
		t.Error("reading a mass in newtons must fail: mass is not weight")
	}
	if _, err := mass.In(calculator.UnitInvalid); err == nil {
		t.Error("the zero Unit must be rejected")
	}
}

// Non-finite user input is refused at construction, so it can never reach an
// equation and become a successful NaN.
func TestNonFiniteInputIsRefusedAtConstruction(t *testing.T) {
	for _, v := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if _, err := calculator.NewQuantity(v, calculator.Kilogram); err == nil {
			t.Errorf("NewQuantity(%v) was accepted", v)
		}
	}
}

// Extreme but finite inputs overflow or underflow the result rather than
// returning an infinity or a silent zero.
func TestExtremeValuesAreCaughtInTheResult(t *testing.T) {
	t.Run("overflow", func(t *testing.T) {
		ev := eval{t}
		err := ev.fails(calculator.WingLoadingForce(
			mustQ(t, 1e300, calculator.Kilogram),
			mustQ(t, 1e-300, calculator.SquareMeter)))
		wantIssue(t, err, "wing_loading_force", calculator.IssueInvalid)
	})

	t.Run("underflow", func(t *testing.T) {
		ev := eval{t}
		err := ev.fails(calculator.WingLoadingMass(
			mustQ(t, 1e-300, calculator.Kilogram),
			mustQ(t, 1e300, calculator.SquareMeter)))
		wantIssue(t, err, "wing_loading_mass", calculator.IssueInvalid)
	})
}

// A 2D section coefficient must not be usable as a whole-aircraft CLmax.
func TestSectionCoefficientIsUnsupportedAsAircraftCLmax(t *testing.T) {
	ev := eval{t}
	fc := baseCase(t)
	fc.CLmax = calculator.LiftCoefficient{
		Max:   1.6,
		Scope: calculator.ScopeAirfoilSection,
		Basis: "NACA 2412 section data",
	}
	err := ev.fails(calculator.StallSpeed(fc,
		mustQ(t, 2, calculator.Kilogram),
		mustQ(t, 0.24, calculator.SquareMeter)))
	wantIssue(t, err, "clmax", calculator.IssueUnsupported)

	issues, _ := calculator.AsIssues(err)
	if issues.Kind(calculator.IssueInvalid) {
		t.Error("a section coefficient is unsupported, not arithmetically invalid")
	}
}

func TestUnknownCoefficientScopeIsMissing(t *testing.T) {
	ev := eval{t}
	fc := baseCase(t)
	fc.CLmax = calculator.LiftCoefficient{Max: 1.2, Basis: "unstated"}
	err := ev.fails(calculator.StallSpeed(fc,
		mustQ(t, 2, calculator.Kilogram),
		mustQ(t, 0.24, calculator.SquareMeter)))
	wantIssue(t, err, "clmax", calculator.IssueMissing)
}

// Assumed density and CLmax must be stated, so a result cannot quietly rest on
// an invisible assumption.
func TestAssumptionsMustBeVisible(t *testing.T) {
	mass := mustQ(t, 2, calculator.Kilogram)
	area := mustQ(t, 0.24, calculator.SquareMeter)

	t.Run("density basis", func(t *testing.T) {
		ev := eval{t}
		fc := baseCase(t)
		fc.DensityBasis = ""
		err := ev.fails(calculator.StallSpeed(fc, mass, area))
		wantIssue(t, err, "density", calculator.IssueMissing)
	})

	t.Run("clmax basis", func(t *testing.T) {
		ev := eval{t}
		fc := baseCase(t)
		fc.CLmax.Basis = ""
		err := ev.fails(calculator.StallSpeed(fc, mass, area))
		wantIssue(t, err, "clmax", calculator.IssueMissing)
	})
}

func TestLoadFactorMustBePositive(t *testing.T) {
	ev := eval{t}
	fc := baseCase(t)
	fc.LoadFactor = 0
	err := ev.fails(calculator.StallSpeed(fc,
		mustQ(t, 2, calculator.Kilogram),
		mustQ(t, 0.24, calculator.SquareMeter)))
	wantIssue(t, err, "load_factor", calculator.IssueInvalid)
}

// One evaluation reports every bad field at once, so a worksheet can annotate
// them together instead of revealing them one at a time.
func TestAllBadFieldsAreReportedTogether(t *testing.T) {
	ev := eval{t}
	fc := baseCase(t)
	fc.LoadFactor = -1
	fc.DensityBasis = ""
	var unsetMass calculator.Quantity

	err := ev.fails(calculator.StallSpeed(fc, unsetMass, mustQ(t, 0, calculator.SquareMeter)))
	issues, ok := calculator.AsIssues(err)
	if !ok {
		t.Fatalf("expected Issues, got %T", err)
	}
	for _, field := range []string{"mass", "wing_area", "density", "load_factor"} {
		if !hasField(issues, field) {
			t.Errorf("issues do not mention %q: %v", issues.Fields(), err)
		}
	}
}

func hasField(issues calculator.Issues, field string) bool {
	for _, f := range issues.Fields() {
		if f == field {
			return true
		}
	}
	return false
}

// No input this package accepts may produce a panic or a successful NaN.
func TestNoPanicOrNaNSuccessAcrossAdversarialInputs(t *testing.T) {
	values := []float64{0, -1, 1, 1e-320, 1e308, -1e308, 0.5}
	fc := baseCase(t)
	for _, m := range values {
		for _, a := range values {
			mass, massErr := calculator.NewQuantity(m, calculator.Kilogram)
			area, areaErr := calculator.NewQuantity(a, calculator.SquareMeter)
			if massErr != nil || areaErr != nil {
				continue
			}
			for _, n := range values {
				probe := fc
				probe.LoadFactor = n
				r, err := calculator.StallSpeed(probe, mass, area)
				if err != nil {
					continue
				}
				if v := r.Value.SI(); math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
					t.Fatalf("m=%v a=%v n=%v returned a successful non-physical %v", m, a, n, v)
				}
			}
		}
	}
}
