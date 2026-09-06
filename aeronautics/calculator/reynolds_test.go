package calculator_test

import (
	"testing"

	"yalb.aero/calculator"
)

// viscousCase is the base fixture case with the ISA sea-level viscosity added.
func viscousCase(t *testing.T) calculator.FlightCase {
	t.Helper()
	fc := baseCase(t)
	fc.Viscosity = calculator.SeaLevelISAViscosity()
	fc.ViscosityBasis = "ISA sea level"
	return fc
}

// The task's fixture: rho = 1.225 kg/m^3, V = 15 m/s, c = 0.2 m,
// mu = 1.7894e-5 Pa*s. Computed independently at 45 significant digits as
// 205376.1037219179613278194.
func TestReynoldsFixture(t *testing.T) {
	ev := eval{t}
	fc := viscousCase(t)
	if !(tol{rel: floatNoise}).ok(fc.Viscosity.SI(), 1.7894e-5) {
		t.Fatalf("ISA sea level viscosity = %v, want 1.7894e-5 Pa*s", fc.Viscosity)
	}
	r := ev.ok(calculator.Reynolds(fc,
		mustQ(t, 15, calculator.MeterPerSecond), mustQ(t, 0.2, calculator.Meter)))
	// The expected value is quoted to four decimal places, so the absolute term
	// is 1e-4; the relative term absorbs float64 rounding.
	if !(tol{abs: 1e-4, rel: 1e-12}).ok(r.Value.SI(), 205376.1037219179613278194) {
		t.Errorf("Reynolds = %.15g, want 205376.1037", r.Value.SI())
	}
	if r.Value.Dimension() != calculator.Dimensionless {
		t.Errorf("a Reynolds number is dimensionless, got %v", r.Value.Dimension())
	}
	if _, ok := r.Trace.Substitution("viscosity"); !ok {
		t.Error("the trace must record the viscosity actually used")
	}
}

// Viscosity is evidence like density: an unstated basis is a missing field, not
// a silent standard-atmosphere default.
func TestReynoldsNeedsStatedViscosity(t *testing.T) {
	ev := eval{t}
	speed := mustQ(t, 15, calculator.MeterPerSecond)
	chord := mustQ(t, 0.2, calculator.Meter)

	unset := baseCase(t)
	wantIssue(t, ev.fails(calculator.Reynolds(unset, speed, chord)), "viscosity",
		calculator.IssueMissing)

	unstated := viscousCase(t)
	unstated.ViscosityBasis = ""
	wantIssue(t, ev.fails(calculator.Reynolds(unstated, speed, chord)), "viscosity",
		calculator.IssueMissing)

	wrong := viscousCase(t)
	wrong.Viscosity = mustQ(t, 1.225, calculator.KilogramPerCubicMeter)
	wantIssue(t, ev.fails(calculator.Reynolds(wrong, speed, chord)), "viscosity",
		calculator.IssueInvalid)
}

// A tapered RC wing does not have one Reynolds number. Covering the MAC alone
// hides the tip, which is the station most likely to be in trouble.
func TestReynoldsCoverageSpansRootToTip(t *testing.T) {
	def := calculator.WingDefinition{
		Name: "tapered RC wing",
		Drivers: calculator.PlanformDrivers{
			Shape:       calculator.ShapeTrapezoid,
			Area:        mustQ(t, 24, calculator.SquareDecimeter),
			AspectRatio: 6,
			TaperRatio:  0.5,
		},
		Sweep:          mustQ(t, 0, calculator.Degree),
		SweepReference: 0.25,
		Dihedral:       mustQ(t, 0, calculator.Degree),
		Twist:          mustQ(t, 0, calculator.Degree),
		Incidence:      mustQ(t, 0, calculator.Degree),
		AreaBasis:      calculator.AreaBasisReferenceTrapezoid,
	}
	w, err := calculator.SolveWing(def)
	if err != nil {
		t.Fatalf("SolveWing: %v", err)
	}
	coverage, err := w.ReynoldsCoverage(viscousCase(t), mustQ(t, 12, calculator.MeterPerSecond))
	if err != nil {
		t.Fatalf("ReynoldsCoverage: %v", err)
	}
	if !coverage.Complete() {
		t.Fatalf("coverage is missing %v", coverage.MissingStations())
	}
	root, _ := coverage.At(calculator.StationRoot)
	mac, _ := coverage.At(calculator.StationMAC)
	tip, _ := coverage.At(calculator.StationTip)
	if !(root.Reynolds > mac.Reynolds && mac.Reynolds > tip.Reynolds) {
		t.Errorf("under taper the stations must differ: root %.0f, MAC %.0f, tip %.0f",
			root.Reynolds, mac.Reynolds, tip.Reynolds)
	}
	// Reynolds number is linear in chord, so the tip-to-root ratio is the taper
	// ratio exactly. That is what makes MAC-only coverage a real omission.
	if !(tol{rel: 1e-12}).ok(tip.Reynolds/root.Reynolds, 0.5) {
		t.Errorf("tip/root Reynolds ratio = %v, want the taper ratio 0.5",
			tip.Reynolds/root.Reynolds)
	}
	low, high, ok := coverage.Range()
	if !ok || low != tip.Reynolds || high != root.Reynolds {
		t.Errorf("range = %v..%v, want tip..root", low, high)
	}

	for _, sample := range coverage.Samples {
		if sample.Trace.EquationID != calculator.EqReynolds {
			t.Errorf("sample %v has no Reynolds trace", sample.Station)
		}
		if sample.Chord.Dimension() != calculator.DimLength {
			t.Errorf("sample %v does not record the chord it used", sample.Station)
		}
	}
}

// A coverage holding only the MAC must report the root and tip as missing
// rather than passing as coverage of the wing.
func TestMACOnlyCoverageIsIncomplete(t *testing.T) {
	macOnly := calculator.ReynoldsCoverage{
		Case:         viscousCase(t),
		TrueAirspeed: mustQ(t, 12, calculator.MeterPerSecond),
		Samples: []calculator.ReynoldsSample{{
			Station:  calculator.StationMAC,
			Chord:    mustQ(t, 0.2, calculator.Meter),
			Reynolds: 164300.88,
		}},
	}
	if macOnly.Complete() {
		t.Error("MAC-only coverage must not count as covering the wing")
	}
	missing := macOnly.MissingStations()
	if len(missing) != 2 || missing[0] != calculator.StationRoot || missing[1] != calculator.StationTip {
		t.Errorf("missing stations = %v, want root and tip", missing)
	}
}
