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

// powerWireDesign is the Task 09 candidate the power-search route is exercised
// against: 2.5 kg on 0.40 m^2 at aspect ratio 8, with a complete polar,
// propulsion chain, pack, auxiliary budget and a single cruise segment. Like
// wireDesign above it is written out here rather than imported, so a change to
// the api package's own fixture cannot silently change what the transport sends.
func powerWireDesign() api.Design {
	design := powerWireAirframe()
	design.Polar = powerWirePolar()
	design.Propulsion = &api.Propulsion{
		Efficiency:         0.55,
		EfficiencyBasis:    "synthetic fixture assumption at the cruise condition",
		EfficiencyEvidence: "assumed",
	}
	design.Battery = powerWireBattery()
	design.Auxiliary = powerWireAuxiliary()
	design.Mission = powerWireMission()
	return design
}

// powerWireAirframe is the fixture's mass mode, wing, case and inventory.
func powerWireAirframe() api.Design {
	metres := func(v float64) *api.Quantity { return &api.Quantity{Value: v, Unit: "m"} }
	component := func(name, role string, mass, x float64) api.Component {
		return api.Component{
			Name: name, Role: role, Basis: "synthetic fixture assumption",
			Mass:     &api.Quantity{Value: mass, Unit: "kg"},
			Position: api.Position{X: metres(x), Y: metres(0), Z: metres(0)},
		}
	}
	return api.Design{
		Name:          "power fixture",
		Configuration: "flying-wing",
		MassMode:      "components",
		Wing: api.Wing{
			Name:           "main wing",
			Shape:          "rectangle",
			Area:           &api.Quantity{Value: 0.40, Unit: "m^2"},
			AspectRatio:    8,
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
				Max: 1.2, Scope: "aircraft",
				Basis:    "synthetic fixture assumption, not a recommended default",
				Evidence: "assumed",
			},
		}},
		Components: []api.Component{
			component("airframe", "airframe", 1.4, 0.15),
			component("motor and propeller", "motor", 0.3, -0.05),
			component("flight pack", "battery", 0.5, 0.05),
			component("avionics and servos", "avionics", 0.2, 0.1),
			component("camera", "payload", 0.1, 0),
		},
	}
}

// powerWirePolar is the fixture's aircraft drag polar.
func powerWirePolar() *api.DragPolar {
	return &api.DragPolar{
		CD0:              0.035,
		CD0Basis:         "synthetic fixture assumption at aircraft level, not a default",
		OswaldEfficiency: 0.85,
		EfficiencyBasis:  "synthetic fixture assumption, not a default",
		Configuration:    "clean",
		CLValidMin:       -0.2,
		CLValidMax:       1.1,
		Scope:            "aircraft",
		Evidence:         "assumed",
	}
}

// powerWireBattery is the fixture's flight pack.
func powerWireBattery() *api.Battery {
	return &api.Battery{
		Basis:          "synthetic fixture assumption, labelled 5000 mAh 4S",
		Component:      "flight pack",
		Mode:           "capacity-and-voltage",
		Capacity:       &api.Quantity{Value: 5000, Unit: "mAh"},
		NominalVoltage: &api.Quantity{Value: 14.8, Unit: "V"},
		UsableFraction: 0.8,
		Evidence:       "assumed",
	}
}

// powerWireAuxiliary is the fixture's non-propulsive demand.
func powerWireAuxiliary() []api.AuxiliaryLoad {
	watts := func(v float64) *api.Quantity { return &api.Quantity{Value: v, Unit: "W"} }
	return []api.AuxiliaryLoad{{
		Name:       "autopilot and receiver",
		Basis:      "synthetic fixture assumption",
		Continuous: watts(8),
		Peak:       watts(20),
		Side:       "pack-side",
		Evidence:   "assumed",
	}}
}

// powerWireMission is the fixture's single cruise leg and its reserve.
func powerWireMission() *api.Mission {
	return &api.Mission{
		Name:            "out and back",
		Basis:           "synthetic fixture profile",
		ReserveFraction: 0.2,
		ReserveBasis:    "synthetic fixture reserve",
		Segments: []api.MissionSegment{{
			Name:           "cruise out",
			Case:           fixtureCaseName,
			Kind:           "cruise",
			Model:          "drag-polar",
			Timing:         "duration",
			Speed:          &api.Quantity{Value: 16, Unit: "m/s"},
			ClimbAngle:     zeroDegrees(),
			WindAlongTrack: &api.Quantity{Value: 0, Unit: "m/s"},
			Duration:       &api.Quantity{Value: 600, Unit: "s"},
		}},
	}
}
