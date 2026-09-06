package httpapi_test

import "yalb.aero/api"

// The HTTP tests use the same candidate the api tests do: Task 04's acceptance
// fixture, at span 1.2 m and aspect ratio 6, carrying 2 kg with an 8 m/s stall
// ceiling required in one level-flight case. It is written out here rather than
// imported so that a change to the api package's own fixture cannot silently
// change what the transport tests send.
const fixtureCaseName = "n=1 level"

// zeroDegrees is the explicitly stated zero every wing angle must carry: an
// unstated sweep is a missing field, not zero.
func zeroDegrees() *api.Quantity { return &api.Quantity{Value: 0, Unit: "deg"} }

func identity() api.Request { return api.Request{Session: "http-fixture", Sequence: 7} }

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
			Sweep:          zeroDegrees(),
			SweepReference: 0.25,
			Dihedral:       zeroDegrees(),
			Twist:          zeroDegrees(),
			Incidence:      zeroDegrees(),
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
				Basis:    "synthetic fixture assumption, not a recommended default",
				Evidence: "assumed",
			},
		}},
		Requirements: []api.Requirement{{
			Name:     "stall ceiling",
			Subject:  "stall-speed",
			Priority: "required",
			Basis:    "handling target for a hand launch, chosen for this fixture",
			Cases:    []string{fixtureCaseName},
			Maximum:  &api.Quantity{Value: 8, Unit: "m/s"},
		}},
	}
}

func evaluateRequest() api.EvaluateRequest {
	return api.EvaluateRequest{Request: identity(), Design: wireDesign()}
}
