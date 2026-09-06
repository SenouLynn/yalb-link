package api_test

import (
	"testing"

	"yalb.aero/api"
	"yalb.aero/calculator"
)

// The Task 05 fixture is Task 04's acceptance candidate, expressed twice: once
// as a wire design and once through direct Go calls. Every check that compares
// them relies on the two being the same aircraft, not merely similar, so both
// are written out in full rather than one derived from the other.
//
//	span 1.2 m, aspect ratio 6, mass 2 kg, rho 1.225 kg/m^3, CLmax 1.2, n = 1
//	reference area 0.24 m^2, stall speed 10.544501312841112 m/s
//	minimum area at an 8 m/s ceiling 0.41694940476190476 m^2
const (
	fixtureArea      = 0.24
	fixtureStall     = 10.544501312841112
	fixtureMinArea   = 0.41694940476190476
	fixtureCaseName  = "n=1 level"
	fixtureStallReq  = "stall ceiling"
	fixtureClmaxBasis = "synthetic fixture assumption, not a recommended default"
)

func degrees(v float64) *api.Quantity { return &api.Quantity{Value: v, Unit: "deg"} }

func wireDesign() api.Design {
	return api.Design{
		Name:          "acceptance fixture",
		Configuration: "flying-wing",
		MassBasis:     "target all-up mass for the fixture",
		Mass:          &api.Quantity{Value: 2, Unit: "kg"},
		Wing: api.Wing{
			Name:           "main wing",
			Shape:          "rectangle",
			Span:           &api.Quantity{Value: 1.2, Unit: "m"},
			AspectRatio:    6,
			Sweep:          degrees(0),
			SweepReference: 0.25,
			Dihedral:       degrees(0),
			Twist:          degrees(0),
			Incidence:      degrees(0),
			AreaBasis:      "reference-trapezoid",
		},
		Cases: []api.Case{{
			Name:          fixtureCaseName,
			Configuration: "clean",
			DensityBasis:  "ISA sea level",
			Density:       &api.Quantity{Value: 1.225, Unit: "kg/m^3"},
			LoadFactor:    1,
			Priority:      "required",
			CLmax: api.CLmax{
				Max:      1.2,
				Scope:    "aircraft",
				Basis:    fixtureClmaxBasis,
				Evidence: "assumed",
			},
		}},
		Requirements: []api.Requirement{{
			Name:     fixtureStallReq,
			Subject:  "stall-speed",
			Priority: "required",
			Basis:    "handling target for a hand launch, chosen for this fixture",
			Cases:    []string{fixtureCaseName},
			Maximum:  &api.Quantity{Value: 8, Unit: "m/s"},
		}},
	}
}

// coreDesign builds the same candidate through direct Go calls, so that the
// equivalence check compares two independently written definitions rather than
// one round-tripped through itself.
func coreDesign(t *testing.T) calculator.Design {
	t.Helper()
	zero := mustQ(t, 0, calculator.Degree)
	return calculator.Design{
		Name:          "acceptance fixture",
		Configuration: calculator.ConfigurationFlyingWing,
		MassBasis:     "target all-up mass for the fixture",
		Mass:          mustQ(t, 2, calculator.Kilogram),
		Wing: calculator.WingDefinition{
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
		},
		Cases: []calculator.DesignCase{{
			Case: calculator.FlightCase{
				Name:          fixtureCaseName,
				Configuration: "clean",
				DensityBasis:  "ISA sea level",
				Density:       calculator.SeaLevelISADensity(),
				LoadFactor:    1,
				CLmax: calculator.LiftCoefficient{
					Max:   1.2,
					Scope: calculator.ScopeAircraft,
					Basis: fixtureClmaxBasis,
				},
			},
			Priority:      calculator.PriorityRequired,
			CLmaxEvidence: calculator.EvidenceAssumed,
		}},
		Requirements: []calculator.Requirement{{
			Name:     fixtureStallReq,
			Subject:  calculator.SubjectStallSpeed,
			Priority: calculator.PriorityRequired,
			Basis:    "handling target for a hand launch, chosen for this fixture",
			Cases:    []string{fixtureCaseName},
			Maximum:  mustQ(t, 8, calculator.MeterPerSecond),
		}},
	}
}

func mustQ(t *testing.T, v float64, u calculator.Unit) calculator.Quantity {
	t.Helper()
	q, err := calculator.NewQuantity(v, u)
	if err != nil {
		t.Fatalf("NewQuantity(%v, %v): %v", v, u.Symbol(), err)
	}
	return q
}

func identity() api.Request { return api.Request{Session: "fixture", Sequence: 7} }

func evaluateRequest() api.EvaluateRequest {
	return api.EvaluateRequest{Request: identity(), Design: wireDesign()}
}

// wantFailure asserts a call failed with the given kind and returns the failure.
func wantFailure(t *testing.T, err error, kind api.FailureKind) *api.Failure {
	t.Helper()
	if err == nil {
		t.Fatalf("expected a %v failure, got no error", kind)
	}
	failure, ok := api.AsFailure(err)
	if !ok {
		t.Fatalf("expected an *api.Failure, got %T: %v", err, err)
	}
	if failure.Kind != kind {
		t.Fatalf("failure kind = %v, want %v (%v)", failure.Kind, kind, failure)
	}
	return failure
}

// wantIssueOn asserts the failure carries an issue on the named field.
func wantIssueOn(t *testing.T, failure *api.Failure, field string) api.Issue {
	t.Helper()
	for _, issue := range failure.Issues {
		if issue.Field == field {
			if issue.Detail == "" {
				t.Errorf("issue on %q has no detail", field)
			}
			return issue
		}
	}
	fields := make([]string, 0, len(failure.Issues))
	for _, issue := range failure.Issues {
		fields = append(fields, issue.Field)
	}
	t.Fatalf("expected an issue on %q, got issues on %v", field, fields)
	return api.Issue{}
}
