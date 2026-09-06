package calculator_test

import (
	"math"
	"testing"

	"yalb.aero/calculator"
)

// The fixture design: m = 2 kg, rho = 1.225 kg/m^3, CLmax = 1.2, n = 1,
// S = 0.24 m^2. Expected values are independently calculated at 40 significant
// digits and quoted here at the precision the task states; each absolute
// tolerance is half a unit in that last quoted place.
func TestFixtureValues(t *testing.T) {
	ev := eval{t}
	_ = ev
	fc := baseCase(t)
	mass := mustQ(t, 2, calculator.Kilogram)
	area := mustQ(t, 0.24, calculator.SquareMeter)
	limit := mustQ(t, 8, calculator.MeterPerSecond)

	t.Run("weight", func(t *testing.T) {
		ev := eval{t}
		got := ev.ok(calculator.Weight(mass)).Value
		assertIn(t, got, calculator.Newton, 19.6133, tol{abs: 5e-5, rel: floatNoise})
	})

	t.Run("wing loading, force", func(t *testing.T) {
		ev := eval{t}
		r := ev.ok(calculator.WingLoadingForce(mass, area))
		assertIn(t, r.Value, calculator.NewtonPerSquareMeter, 81.7220833, tol{abs: 5e-8, rel: floatNoise})
	})

	t.Run("wing loading, mass", func(t *testing.T) {
		ev := eval{t}
		r := ev.ok(calculator.WingLoadingMass(mass, area))
		assertIn(t, r.Value, calculator.KilogramPerSquareMeter, 8.3333333, tol{abs: 5e-8, rel: floatNoise})
	})

	t.Run("stall speed", func(t *testing.T) {
		ev := eval{t}
		r := ev.ok(calculator.StallSpeed(fc, mass, area))
		assertIn(t, r.Value, calculator.MeterPerSecond, 10.5445013, tol{abs: 5e-8, rel: floatNoise})
	})

	t.Run("area lower bound for an 8 m/s stall limit", func(t *testing.T) {
		ev := eval{t}
		r := ev.ok(calculator.MinimumWingArea(fc, mass, limit))
		assertIn(t, r.Value, calculator.SquareMeter, 0.4169494048, tol{abs: 5e-11, rel: floatNoise})
	})

	t.Run("mass ceiling for an 8 m/s stall limit at the same area", func(t *testing.T) {
		ev := eval{t}
		r := ev.ok(calculator.MaximumMass(fc, area, limit))
		assertIn(t, r.Value, calculator.Kilogram, 1.1512188158, tol{abs: 5e-11, rel: floatNoise})
	})
}

// A candidate that is computable can still fail its requirement: at the fixture
// area the design stalls at 10.54 m/s, above the 8 m/s limit, and the same
// inputs simultaneously report the area and mass that would meet it.
func TestComputableCandidateCanViolateItsRequirement(t *testing.T) {
	ev := eval{t}
	fc := baseCase(t)
	mass := mustQ(t, 2, calculator.Kilogram)
	area := mustQ(t, 0.24, calculator.SquareMeter)
	limit := mustQ(t, 8, calculator.MeterPerSecond)

	stall := ev.ok(calculator.StallSpeed(fc, mass, area)).Value
	limitSI := inUnit(t, limit, calculator.MeterPerSecond)
	if inUnit(t, stall, calculator.MeterPerSecond) <= limitSI {
		t.Fatalf("fixture no longer exceeds its stall limit: %v", stall)
	}

	minArea := ev.ok(calculator.MinimumWingArea(fc, mass, limit)).Value
	if inUnit(t, minArea, calculator.SquareMeter) <= inUnit(t, area, calculator.SquareMeter) {
		t.Errorf("area bound %v should exceed the violating area %v", minArea, area)
	}
	maxMass := ev.ok(calculator.MaximumMass(fc, area, limit)).Value
	if inUnit(t, maxMass, calculator.Kilogram) >= inUnit(t, mass, calculator.Kilogram) {
		t.Errorf("mass ceiling %v should be below the violating mass %v", maxMass, mass)
	}
}

// Round trips: each inversion must return the value the forward equation used.
func TestForwardInverseRoundTrips(t *testing.T) {
	ev := eval{t}
	fc := baseCase(t)
	mass := mustQ(t, 2, calculator.Kilogram)
	area := mustQ(t, 0.24, calculator.SquareMeter)

	stall := ev.ok(calculator.StallSpeed(fc, mass, area)).Value
	round := tol{abs: 0, rel: 1e-12}

	t.Run("area", func(t *testing.T) {
		ev := eval{t}
		back := ev.ok(calculator.MinimumWingArea(fc, mass, stall)).Value
		assertClose(t, back, area, calculator.SquareMeter, round)
	})

	t.Run("mass", func(t *testing.T) {
		ev := eval{t}
		back := ev.ok(calculator.MaximumMass(fc, area, stall)).Value
		assertClose(t, back, mass, calculator.Kilogram, round)
	})

	t.Run("wing loading", func(t *testing.T) {
		ev := eval{t}
		forward := ev.ok(calculator.WingLoadingForce(mass, area)).Value
		back := ev.ok(calculator.MaximumWingLoadingForce(fc, stall)).Value
		assertClose(t, back, forward, calculator.NewtonPerSquareMeter, round)
	})

	// At the stall speed the required lift coefficient is CLmax itself.
	t.Run("required CL at the stall speed is CLmax", func(t *testing.T) {
		ev := eval{t}
		cl := ev.ok(calculator.RequiredLiftCoefficient(fc, mass, area, stall)).Value
		got := inUnit(t, cl, calculator.One)
		if !(tol{abs: 0, rel: 1e-12}).ok(got, fc.CLmax.Max) {
			t.Errorf("required CL at stall = %v, want CLmax %v", got, fc.CLmax.Max)
		}
	})
}

// Scaling relations hold with every other assumption fixed.
func TestStallSpeedScaling(t *testing.T) {
	ev := eval{t}
	fc := baseCase(t)
	area := mustQ(t, 0.24, calculator.SquareMeter)
	base := ev.ok(calculator.StallSpeed(fc, mustQ(t, 2, calculator.Kilogram), area)).Value
	baseV := inUnit(t, base, calculator.MeterPerSecond)
	scale := tol{abs: 0, rel: 1e-12}

	t.Run("quadrupling mass doubles stall speed", func(t *testing.T) {
		ev := eval{t}
		got := ev.ok(calculator.StallSpeed(fc, mustQ(t, 8, calculator.Kilogram), area)).Value
		if v := inUnit(t, got, calculator.MeterPerSecond); !scale.ok(v, 2*baseV) {
			t.Errorf("stall speed at 4x mass = %v, want %v", v, 2*baseV)
		}
	})

	t.Run("quadrupling area halves stall speed", func(t *testing.T) {
		ev := eval{t}
		wide := mustQ(t, 0.96, calculator.SquareMeter)
		got := ev.ok(calculator.StallSpeed(fc, mustQ(t, 2, calculator.Kilogram), wide)).Value
		if v := inUnit(t, got, calculator.MeterPerSecond); !scale.ok(v, baseV/2) {
			t.Errorf("stall speed at 4x area = %v, want %v", v, baseV/2)
		}
	})
}

// Load factor is reflected explicitly rather than folded into a default of 1.
func TestLoadFactorIsExplicit(t *testing.T) {
	ev := eval{t}
	fc := baseCase(t)
	mass := mustQ(t, 2, calculator.Kilogram)
	area := mustQ(t, 0.24, calculator.SquareMeter)
	base := inUnit(t, ev.ok(calculator.StallSpeed(fc, mass, area)).Value, calculator.MeterPerSecond)

	manoeuvre := fc
	manoeuvre.LoadFactor = 4
	r := ev.ok(calculator.StallSpeed(manoeuvre, mass, area))
	got := inUnit(t, r.Value, calculator.MeterPerSecond)
	if !(tol{abs: 0, rel: 1e-12}).ok(got, 2*base) {
		t.Errorf("stall speed at n=4 = %v, want %v", got, 2*base)
	}

	// The load factor actually used is visible in the trace.
	n, ok := r.Trace.Substitution("load_factor")
	if !ok {
		t.Fatal("trace does not record load_factor")
	}
	if v := inUnit(t, n, calculator.One); v != 4 {
		t.Errorf("trace records load factor %v, want 4", v)
	}
}

// Mixed-unit input must give the same answer as SI input.
func TestMixedUnitEquivalence(t *testing.T) {
	ev := eval{t}
	fc := baseCase(t)
	si := ev.ok(calculator.StallSpeed(fc,
		mustQ(t, 2, calculator.Kilogram),
		mustQ(t, 0.24, calculator.SquareMeter))).Value

	// 2 kg = 2000 g; 0.24 m^2 = 24 dm^2.
	mixed := ev.ok(calculator.StallSpeed(fc,
		mustQ(t, 2000, calculator.Gram),
		mustQ(t, 24, calculator.SquareDecimeter))).Value

	assertClose(t, mixed, si, calculator.MeterPerSecond, tol{abs: 0, rel: 1e-12})

	// The same result read in another speed unit must agree with the conversion.
	kmh := inUnit(t, si, calculator.KilometerPerHour)
	ms := inUnit(t, si, calculator.MeterPerSecond)
	if !(tol{abs: 0, rel: 1e-12}).ok(kmh, ms*3.6) {
		t.Errorf("%v m/s read as %v km/h", ms, kmh)
	}
}

func assertIn(t *testing.T, q calculator.Quantity, u calculator.Unit, want float64, tolerance tol) {
	t.Helper()
	got := inUnit(t, q, u)
	if !tolerance.ok(got, want) {
		t.Errorf("got %v %s, want %v %s (abs %v, rel %v)",
			got, u.Symbol(), want, u.Symbol(), tolerance.abs, tolerance.rel)
	}
}

func assertClose(t *testing.T, got, want calculator.Quantity, u calculator.Unit, tolerance tol) {
	t.Helper()
	g, w := inUnit(t, got, u), inUnit(t, want, u)
	if !tolerance.ok(g, w) {
		t.Errorf("got %v %s, want %v %s (delta %v)", g, u.Symbol(), w, u.Symbol(), math.Abs(g-w))
	}
}
