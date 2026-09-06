package calculator_test

import (
	"math"
	"testing"

	"yalb.aero/calculator"
)

// The Task 04 acceptance fixtures. Every value below was computed independently
// at 50 significant digits from the same closed forms the package evaluates,
// not read back from this package's output:
//
//	S      = b^2/A                      = 1.44/6                 = 0.24 m^2
//	Vs     = sqrt(2 n m g/(rho S CLmax))
//	         n=1                        = 10.544501312841112 m/s
//	         n=2                        = 14.912176765080807 m/s
//	S_min  = 2 n m g/(rho Vs^2 CLmax)
//	         n=1                        = 0.41694940476190476 m^2
//	         n=2                        = 0.83389880952380952 m^2
//	m_max  = rho Vs^2 S CLmax/(2 n g) at S = 0.24 m^2
//	         n=1                        = 1.1512188158035619 kg
//	         n=2                        = 0.57560940790178093 kg
//	A at b = 1.2 m and S = S_min(n=1)   = 3.4536564474106856
//	c at b = 1.2 m and S = S_min(n=1)   = 0.34745783730158730 m
//	b at A = 6   and S = S_min(n=1)     = 1.5816751969261668 m
//
// with g = 9.80665 m/s^2, rho = 1.225 kg/m^3, CLmax = 1.2 and m = 2 kg. The
// task quotes several of these to fewer places (0.4169494048, 0.8338988095,
// 1.1512188, 0.5756094079, 1.5816752); the tolerances below are stated as an
// absolute term in the value's own SI unit plus a relative term, taken from the
// precision the task quotes rather than from display rounding.
const (
	fixtureArea         = 0.24
	fixtureStallN1      = 10.544501312841112
	fixtureStallN2      = 14.912176765080807
	fixtureMinAreaN1    = 0.41694940476190476
	fixtureMinAreaN2    = 0.83389880952380952
	fixtureMaxMassN1    = 1.1512188158035619
	fixtureMaxMassN2    = 0.57560940790178093
	fixtureSizedAspect  = 3.4536564474106856
	fixtureSizedChord   = 0.34745783730158730
	fixtureSpanAtAspect = 1.5816751969261668
)

// designTol is the acceptance tolerance for the fixtures above.
var designTol = tol{abs: 1e-10, rel: 1e-12}

// caseNameN1 and caseNameN2 are the two required cases the task's checks use.
const (
	caseNameN1 = "n=1 level"
	caseNameN2 = "n=2 manoeuvre"
	reqStall   = "stall ceiling"
	reqSpan    = "maximum span"
	reqAspect  = "minimum aspect ratio"
	reqArea    = "maximum wing area"
)

// designCaseAt builds a required design case at the given load factor, on
// Task 02's air and CLmax fixture.
func designCaseAt(t *testing.T, name string, loadFactor float64) calculator.DesignCase {
	t.Helper()
	fc := baseCase(t)
	fc.Name = name
	fc.LoadFactor = loadFactor
	return calculator.DesignCase{
		Case:          fc,
		Priority:      calculator.PriorityRequired,
		CLmaxEvidence: calculator.EvidenceAssumed,
	}
}

// stallCeilingAt builds the required 8 m/s stall ceiling for the named cases.
func stallCeilingAt(t *testing.T, cases ...string) calculator.Requirement {
	t.Helper()
	return calculator.Requirement{
		Name:     reqStall,
		Subject:  calculator.SubjectStallSpeed,
		Maximum:  mustQ(t, 8, calculator.MeterPerSecond),
		Cases:    cases,
		Priority: calculator.PriorityRequired,
		Basis:    "handling target for a hand launch, chosen for this fixture",
	}
}

// baseWing is the fixture wing: a rectangle at span 1.2 m and aspect ratio 6,
// with every angle stated explicitly as the geometry model requires.
func baseWing(t *testing.T) calculator.WingDefinition {
	t.Helper()
	zero := mustQ(t, 0, calculator.Degree)
	return calculator.WingDefinition{
		Name: "main wing",
		Drivers: calculator.PlanformDrivers{
			Shape:       calculator.ShapeRectangle,
			Span:        mustQ(t, 1.2, calculator.Meter),
			AspectRatio: 6,
		},
		Sweep:          zero,
		SweepReference: 0.25,
		Dihedral:       zero,
		Twist:          zero,
		Incidence:      zero,
		AreaBasis:      calculator.AreaBasisReferenceTrapezoid,
	}
}

// baseDesign is acceptance check 1's starting point: span 1.2 m, aspect ratio
// 6, all-up mass 2 kg, Task 02's air and CLmax case, and an 8 m/s stall
// ceiling required in it.
//
// The configuration is a flying wing so that the airframe description is
// complete without a tail. TestIncompleteTailDoesNotBlockLiftSizing covers the
// conventional layout, where it is deliberately incomplete.
func baseDesign(t *testing.T) calculator.Design {
	t.Helper()
	return calculator.Design{
		Name:          "acceptance fixture",
		Configuration: calculator.ConfigurationFlyingWing,
		Wing:          baseWing(t),
		Mass:          mustQ(t, 2, calculator.Kilogram),
		MassBasis:     "target all-up mass for the fixture",
		Cases:         []calculator.DesignCase{designCaseAt(t, caseNameN1, 1)},
		Requirements:  []calculator.Requirement{stallCeilingAt(t, caseNameN1)},
	}
}

// bothCasesDesign is acceptance check 7's starting point: the same candidate
// with the n=1 and n=2 cases both required and the ceiling applying to both.
func bothCasesDesign(t *testing.T) calculator.Design {
	t.Helper()
	d := baseDesign(t)
	d.Cases = []calculator.DesignCase{
		designCaseAt(t, caseNameN1, 1),
		designCaseAt(t, caseNameN2, 2),
	}
	d.Requirements = []calculator.Requirement{stallCeilingAt(t, caseNameN1, caseNameN2)}
	return d
}

// requireCheck returns the single check for a requirement, bound side and case,
// failing when the set does not hold exactly one.
func requireCheck(t *testing.T, checks calculator.RequirementChecks, name, caseName string,
	direction calculator.BoundDirection,
) calculator.RequirementCheck {
	t.Helper()
	var found []calculator.RequirementCheck
	for _, c := range checks.Named(name).ForCase(caseName) {
		if c.Direction == direction {
			found = append(found, c)
		}
	}
	if len(found) != 1 {
		t.Fatalf("expected exactly one %v check on %q in case %q, got %d", direction, name, caseName, len(found))
	}
	return found[0]
}

// wantStatus asserts a check's requirement status and model result together,
// because the two are separate answers and a test that reads only one of them
// would pass while the other drifted.
func wantStatus(t *testing.T, check calculator.RequirementCheck,
	status calculator.LimitStatus, result calculator.ResultStatus,
) {
	t.Helper()
	if check.Status != status {
		t.Errorf("%s in case %q: status = %v, want %v (detail: %s)",
			check.Name, check.Case, check.Status, status, check.Detail)
	}
	if check.Result != result {
		t.Errorf("%s in case %q: result = %v, want %v (detail: %s)",
			check.Name, check.Case, check.Result, result, check.Detail)
	}
}

// wantSI asserts a quantity's SI value against a fixture.
func wantSI(t *testing.T, label string, q calculator.Quantity, want float64) {
	t.Helper()
	if !designTol.ok(q.SI(), want) {
		t.Errorf("%s = %v, want %v", label, q.SI(), want)
	}
}

// wantControlling asserts the sources controlling a bound, in order.
func wantControlling(t *testing.T, bound calculator.SizingBound, want ...string) {
	t.Helper()
	if len(bound.Controlling) != len(want) {
		t.Fatalf("controlling sources = %v, want %v", bound.Controlling, want)
	}
	for n := range want {
		if bound.Controlling[n] != want[n] {
			t.Fatalf("controlling sources = %v, want %v", bound.Controlling, want)
		}
	}
}

// mustDo applies a command and fails the test if it is rejected.
func mustDo(t *testing.T, s *calculator.Session, cmd calculator.Command) {
	t.Helper()
	if err := s.Do(cmd); err != nil {
		t.Fatalf("%s: %v", cmd.Label(), err)
	}
}

// sameWithin reports whether two float64 values agree to the given relative
// tolerance, treating an exact zero as requiring an exact match.
func sameWithin(got, want, rel float64) bool {
	if want == 0 {
		return got == 0
	}
	return math.Abs(got-want) <= rel*math.Abs(want)
}
