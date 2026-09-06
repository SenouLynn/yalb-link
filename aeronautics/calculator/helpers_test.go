package calculator_test

import (
	"math"
	"testing"

	"yalb.aero/calculator"
)

// tol is an acceptance tolerance stated as an absolute term in the quantity's
// own SI unit plus a relative term. Both are properties of the check, not of
// how a value is displayed: the absolute terms below come from the number of
// decimal places the task's expected values are quoted to, and the relative
// term absorbs float64 rounding in the evaluation itself.
type tol struct {
	abs float64
	rel float64
}

func (t tol) ok(got, want float64) bool {
	return math.Abs(got-want) <= t.abs+t.rel*math.Abs(want)
}

// floatNoise is the relative tolerance for values this package computes twice
// by different routes; it is far tighter than any quoted fixture.
const floatNoise = 1e-12

func mustQ(t *testing.T, v float64, u calculator.Unit) calculator.Quantity {
	t.Helper()
	q, err := calculator.NewQuantity(v, u)
	if err != nil {
		t.Fatalf("NewQuantity(%v, %v): %v", v, u.Symbol(), err)
	}
	return q
}

func inUnit(t *testing.T, q calculator.Quantity, u calculator.Unit) float64 {
	t.Helper()
	v, err := q.In(u)
	if err != nil {
		t.Fatalf("converting %v to %v: %v", q, u.Symbol(), err)
	}
	return v
}

// baseCase is the task's fixture condition: rho = 1.225 kg/m^3, CLmax = 1.2 and
// n = 1. The CLmax is a synthetic assumption chosen for the fixture, not a
// recommended default for any real RC aircraft.
func baseCase(t *testing.T) calculator.FlightCase {
	t.Helper()
	return calculator.FlightCase{
		Name:          "stall, clean, sea level",
		Configuration: "clean",
		DensityBasis:  "ISA sea level",
		Density:       calculator.SeaLevelISADensity(),
		CLmax: calculator.LiftCoefficient{
			Max:   1.2,
			Scope: calculator.ScopeAircraft,
			Basis: "synthetic fixture assumption, not a recommended default",
		},
		LoadFactor: 1,
	}
}

// eval binds a *testing.T so a (Result, error) pair can be consumed directly,
// as in ev.ok(calculator.Weight(mass)).
type eval struct{ t *testing.T }

func (e eval) ok(r calculator.Result, err error) calculator.Result {
	e.t.Helper()
	if err != nil {
		e.t.Fatalf("unexpected error: %v", err)
	}
	return r
}

// fails asserts the evaluation was rejected and returns the error, and checks
// that a rejected evaluation yields no partial result.
func (e eval) fails(r calculator.Result, err error) error {
	e.t.Helper()
	if err == nil {
		e.t.Fatalf("expected an error, got result %v", r.Value)
	}
	if r.Value != (calculator.Quantity{}) || r.Trace.EquationID != "" {
		e.t.Errorf("a rejected evaluation must not return a partial result, got %+v", r)
	}
	return err
}

// wantIssue asserts that err carries an issue of the given kind on the given
// field, and returns its detail so a test can check the message is useful.
func wantIssue(t *testing.T, err error, field string, kind calculator.IssueKind) string {
	t.Helper()
	if err == nil {
		t.Fatalf("expected a %v issue on %q, got no error", kind, field)
	}
	issues, ok := calculator.AsIssues(err)
	if !ok {
		t.Fatalf("expected calculator.Issues, got %T: %v", err, err)
	}
	for _, issue := range issues {
		if issue.Field == field && issue.Kind == kind {
			if issue.Detail == "" {
				t.Errorf("issue on %q has no detail", field)
			}
			return issue.Detail
		}
	}
	t.Fatalf("expected a %v issue on %q, got: %v", kind, field, err)
	return ""
}
