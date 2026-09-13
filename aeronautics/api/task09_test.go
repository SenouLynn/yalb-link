package api_test

import (
	"context"
	"math"
	"testing"

	"yalb.aero/api"
	"yalb.aero/calculator"
)

// The Task 09 boundary fixture is the same electric aircraft the core's own
// fixtures use, expressed on the wire. Every check below compares the boundary's
// answer against a direct Go call on the equivalent design, so the two are
// written out separately rather than one derived from the other.
//
//	2.5 kg on 0.40 m^2 at aspect ratio 8, CD0 0.035, e 0.85, chain efficiency
//	0.55, 8 W of auxiliary draw, a 5000 mAh 4S pack at 80% usable with a 20%
//	reserve, and a 600 s cruise at 16 m/s.
const (
	powerCaseName    = "cruise, clean, sea level"
	powerCruiseSeg   = "cruise out"
	powerCruisePoint = "bench, 4S, cruise speed"
	powerStaticPoint = "bench, 4S, static"
	powerBatteryPart = "flight pack"
	powerCruiseWatts = 84.91046324557467580780824
	powerBudgetJoule = 170496.0
)

func powerWireDesign() api.Design {
	design := powerWireAirframe()
	design.Polar = powerWirePolar()
	design.Propulsion = powerWirePropulsion()
	design.Battery = powerWireBattery()
	design.Auxiliary = powerWireAuxiliary()
	design.Mission = powerWireMission()
	return design
}

// powerWireAirframe is the fixture's mass mode, wing, case and inventory.
func powerWireAirframe() api.Design {
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
			Name:          powerCaseName,
			Configuration: "clean",
			DensityBasis:  "ISA sea level",
			Density:       &api.Quantity{Value: 1.225, Unit: "kg/m^3"},
			LoadFactor:    1,
			Priority:      "required",
			CLmax: api.CLmax{
				Max: 1.2, Scope: "aircraft", Basis: fixtureClmaxBasis, Evidence: "assumed",
			},
		}},
		Components: []api.Component{
			wireComponent("airframe", "airframe", 1.4, 0.15),
			wireComponent("motor and propeller", "motor", 0.3, -0.05),
			wireComponent(powerBatteryPart, "battery", 0.5, 0.05),
			wireComponent("avionics and servos", "avionics", 0.2, 0.1),
			wireComponent("camera", "payload", 0.1, 0),
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

// powerWirePropulsion is the chain efficiency, the component ratings and the
// two capability points the fixture measures.
func powerWirePropulsion() *api.Propulsion {
	return &api.Propulsion{
		Efficiency:         0.55,
		EfficiencyBasis:    "synthetic fixture assumption at the cruise condition",
		EfficiencyEvidence: "assumed",
		Limits: &api.PropulsionLimits{
			Basis:              "synthetic fixture ratings",
			MaxContinuousPower: &api.Quantity{Value: 300, Unit: "W"},
			MaxPeakPower:       &api.Quantity{Value: 500, Unit: "W"},
			MaxRPM:             &api.Quantity{Value: 12000, Unit: "rpm"},
			MaxVoltage:         &api.Quantity{Value: 16.8, Unit: "V"},
			PropellerDiameter:  &api.Quantity{Value: 0.254, Unit: "m"},
			PropellerHubHeight: &api.Quantity{Value: 0.18, Unit: "m"},
		},
		Capabilities: []api.Capability{
			{
				Name:            powerCruisePoint,
				Basis:           "synthetic fixture assumption standing in for a wind-tunnel measurement",
				Kind:            "in-flight",
				Speed:           &api.Quantity{Value: 16, Unit: "m/s"},
				Density:         &api.Quantity{Value: 1.225, Unit: "kg/m^3"},
				DensityBasis:    "ISA sea level",
				Voltage:         &api.Quantity{Value: 14.8, Unit: "V"},
				RPM:             &api.Quantity{Value: 9000, Unit: "rpm"},
				Throttle:        0.75,
				Thrust:          &api.Quantity{Value: 6, Unit: "N"},
				ElectricalPower: &api.Quantity{Value: 150, Unit: "W"},
				Evidence:        "measured",
			},
			{
				Name:            powerStaticPoint,
				Basis:           "synthetic fixture assumption standing in for a static bench test",
				Kind:            "static",
				Speed:           &api.Quantity{Value: 0, Unit: "m/s"},
				Density:         &api.Quantity{Value: 1.225, Unit: "kg/m^3"},
				DensityBasis:    "ISA sea level",
				Voltage:         &api.Quantity{Value: 14.8, Unit: "V"},
				RPM:             &api.Quantity{Value: 10500, Unit: "rpm"},
				Throttle:        1,
				Thrust:          &api.Quantity{Value: 22, Unit: "N"},
				ElectricalPower: &api.Quantity{Value: 340, Unit: "W"},
				Evidence:        "measured",
			},
		},
	}
}

// powerWireBattery is the fixture's flight pack.
func powerWireBattery() *api.Battery {
	return &api.Battery{
		Basis:                  "synthetic fixture assumption, labelled 5000 mAh 4S",
		Component:              powerBatteryPart,
		Mode:                   "capacity-and-voltage",
		Capacity:               &api.Quantity{Value: 5000, Unit: "mAh"},
		NominalVoltage:         &api.Quantity{Value: 14.8, Unit: "V"},
		UsableFraction:         0.8,
		ContinuousCurrentLimit: &api.Quantity{Value: 40, Unit: "A"},
		PeakCurrentLimit:       &api.Quantity{Value: 60, Unit: "A"},
		Evidence:               "assumed",
	}
}

// powerWireAuxiliary is the fixture's non-propulsive demand, with one figure on
// each side of a regulator.
func powerWireAuxiliary() []api.AuxiliaryLoad {
	return []api.AuxiliaryLoad{
		{
			Name:       "autopilot and receiver",
			Basis:      "synthetic fixture assumption",
			Continuous: &api.Quantity{Value: 5, Unit: "W"},
			Peak:       &api.Quantity{Value: 7, Unit: "W"},
			Side:       "pack-side",
			Evidence:   "assumed",
		},
		{
			Name:                "servos",
			Basis:               "synthetic fixture assumption",
			Continuous:          &api.Quantity{Value: 2.4, Unit: "W"},
			Peak:                &api.Quantity{Value: 24, Unit: "W"},
			RegulatorEfficiency: 0.8,
			Side:                "load-side",
			Evidence:            "assumed",
		},
	}
}

// powerWireMission is the fixture's single cruise leg and its reserve.
func powerWireMission() *api.Mission {
	return &api.Mission{
		Name:            "out and back",
		Basis:           "synthetic fixture profile",
		ReserveFraction: 0.2,
		ReserveBasis:    "synthetic fixture reserve",
		Segments: []api.MissionSegment{{
			Name:           powerCruiseSeg,
			Case:           powerCaseName,
			Kind:           "cruise",
			Model:          "drag-polar",
			Timing:         "duration",
			Speed:          &api.Quantity{Value: 16, Unit: "m/s"},
			ClimbAngle:     zeroDegrees(),
			WindAlongTrack: &api.Quantity{Value: 0, Unit: "m/s"},
			Duration:       &api.Quantity{Value: 600, Unit: "s"},
			Capability:     powerCruisePoint,
		}},
	}
}

func evaluatePower(t *testing.T, design api.Design) api.Evaluation {
	t.Helper()
	result, err := api.NewService().Evaluate(context.Background(), api.EvaluateRequest{
		Request: api.Request{Session: "task09", Sequence: 1},
		Design:  design,
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	return result
}

// The whole power definition survives the round trip and the boundary reports
// the same numbers a direct Go call does. The wire carries no arithmetic, so
// the two must agree exactly rather than approximately.
func TestPowerEvaluationMatchesTheCoreExactly(t *testing.T) {
	evaluation := evaluatePower(t, powerWireDesign())
	if evaluation.Mission.Status != "computed" {
		t.Fatalf("mission status = %q: %s", evaluation.Mission.Status, evaluation.Mission.Detail)
	}
	wantValue(t, "peak continuous power", evaluation.Mission.PeakContinuousPower, powerCruiseWatts, "W")
	wantValue(t, "energy budget", evaluation.Mission.Budget, powerBudgetJoule, "J")
	wantValue(t, "required energy", evaluation.Mission.RequiredEnergy, powerCruiseWatts*600, "J")
	if evaluation.Mission.EnergyStatus != "met" {
		t.Errorf("energy status = %q, want met", evaluation.Mission.EnergyStatus)
	}
	if len(evaluation.Mission.Segments) != 1 {
		t.Fatalf("got %d segments, want 1", len(evaluation.Mission.Segments))
	}
	segment := evaluation.Mission.Segments[0]
	if segment.Power.Status != "computed" {
		t.Fatalf("segment power status = %q: %s", segment.Power.Status, segment.Power.Detail)
	}
	if len(segment.Power.Traces) == 0 {
		t.Error("the segment crossed the boundary with no traces, so nothing can be inspected")
	}
	if segment.Availability.Status != "met" {
		t.Errorf("availability = %q (%s), want met", segment.Availability.Status, segment.Availability.Detail)
	}

	// The same design through direct Go calls.
	core := coreEquivalent(t)
	mission := core.MissionAnalysis()
	if got, want := evaluation.Mission.PeakContinuousPower.Value, mission.PeakContinuousPower.SI(); got != want {
		t.Errorf("boundary reports %v W and a direct Go call %v W", got, want)
	}
	if got, want := evaluation.Mission.RequiredEnergy.Value, mission.RequiredEnergy.SI(); got != want {
		t.Errorf("boundary reports %v J and a direct Go call %v J", got, want)
	}
}

// coreEquivalent builds the same aircraft through direct Go calls. It is
// written out rather than decoded from the wire fixture, because a comparison
// against a value the decoder produced would only prove the decoder agrees with
// itself.
func coreEquivalent(t *testing.T) calculator.Design {
	t.Helper()
	design := coreAirframe(t)
	design.Polar = corePolar()
	design.Auxiliary = coreAuxiliary(t)
	design.Battery = coreBattery(t)
	design.Propulsion = corePropulsion(t)
	design.Mission = coreMission(t)
	return design
}

// coreQuantity is the fixture's constructor for a dimensioned value. A bad one
// is the test's own mistake, so it fails the test rather than being handled.
func coreQuantity(t *testing.T, v float64, u calculator.Unit) calculator.Quantity {
	t.Helper()
	value, err := calculator.NewQuantity(v, u)
	if err != nil {
		t.Fatalf("NewQuantity(%v, %v): %v", v, u.Symbol(), err)
	}
	return value
}

// coreAirframe is the fixture's mass mode, wing, case and inventory.
func coreAirframe(t *testing.T) calculator.Design {
	t.Helper()
	q := func(v float64, u calculator.Unit) calculator.Quantity { return coreQuantity(t, v, u) }
	zero := q(0, calculator.Degree)
	component := func(name string, role calculator.ComponentRole, mass, x float64) calculator.MassItem {
		return calculator.MassItem{
			Name: name, Role: role, Basis: "synthetic fixture assumption",
			Mass: q(mass, calculator.Kilogram),
			Position: calculator.Point{
				X: q(x, calculator.Meter), Y: q(0, calculator.Meter), Z: q(0, calculator.Meter),
			},
		}
	}
	return calculator.Design{
		Name:          "power fixture",
		Configuration: calculator.ConfigurationFlyingWing,
		MassMode:      calculator.MassModeComponents,
		Wing: calculator.WingDefinition{
			Name: "main wing",
			Drivers: calculator.PlanformDrivers{
				Shape: calculator.ShapeRectangle,
				Area:  q(0.40, calculator.SquareMeter), AspectRatio: 8,
			},
			Sweep: zero, SweepReference: 0.25, Dihedral: zero, Twist: zero, Incidence: zero,
			AreaBasis: calculator.AreaBasisReferenceTrapezoid,
		},
		Cases: []calculator.DesignCase{{
			Case: calculator.FlightCase{
				Name: powerCaseName, Configuration: "clean",
				DensityBasis: "ISA sea level", Density: calculator.SeaLevelISADensity(),
				LoadFactor: 1,
				CLmax: calculator.LiftCoefficient{
					Max: 1.2, Scope: calculator.ScopeAircraft, Basis: fixtureClmaxBasis,
				},
			},
			Priority: calculator.PriorityRequired, CLmaxEvidence: calculator.EvidenceAssumed,
		}},
		Components: []calculator.MassItem{
			component("airframe", calculator.ComponentAirframe, 1.4, 0.15),
			component("motor and propeller", calculator.ComponentMotor, 0.3, -0.05),
			component(powerBatteryPart, calculator.ComponentBattery, 0.5, 0.05),
			component("avionics and servos", calculator.ComponentAvionics, 0.2, 0.1),
			component("camera", calculator.ComponentPayload, 0.1, 0),
		},
	}
}

// corePolar is the fixture's aircraft drag polar.
func corePolar() calculator.DragPolar {
	return calculator.DragPolar{
		CD0:              0.035,
		CD0Basis:         "synthetic fixture assumption at aircraft level, not a default",
		OswaldEfficiency: 0.85,
		EfficiencyBasis:  "synthetic fixture assumption, not a default",
		Configuration:    "clean",
		CLValidMin:       -0.2, CLValidMax: 1.1,
		Scope: calculator.ScopeAircraft, Evidence: calculator.EvidenceAssumed,
	}
}

// coreAuxiliary is the fixture's non-propulsive demand.
func coreAuxiliary(t *testing.T) []calculator.AuxiliaryLoad {
	t.Helper()
	q := func(v float64) calculator.Quantity { return coreQuantity(t, v, calculator.Watt) }
	return []calculator.AuxiliaryLoad{
		{
			Name: "autopilot and receiver", Basis: "synthetic fixture assumption",
			Continuous: q(5), Peak: q(7),
			Side: calculator.RegulatorPackSide, Evidence: calculator.EvidenceAssumed,
		},
		{
			Name: "servos", Basis: "synthetic fixture assumption",
			Continuous: q(2.4), Peak: q(24),
			RegulatorEfficiency: 0.8,
			Side:                calculator.RegulatorLoadSide, Evidence: calculator.EvidenceAssumed,
		},
	}
}

// coreBattery is the fixture's flight pack.
func coreBattery(t *testing.T) calculator.Battery {
	t.Helper()
	q := func(v float64, u calculator.Unit) calculator.Quantity { return coreQuantity(t, v, u) }
	return calculator.Battery{
		Basis: "synthetic fixture assumption, labelled 5000 mAh 4S", Component: powerBatteryPart,
		Mode:     calculator.BatteryEnergyFromCapacity,
		Capacity: q(5000, calculator.MilliampereHour), NominalVoltage: q(14.8, calculator.Volt),
		UsableFraction:         0.8,
		ContinuousCurrentLimit: q(40, calculator.Ampere),
		PeakCurrentLimit:       q(60, calculator.Ampere),
		Evidence:               calculator.EvidenceAssumed,
	}
}

// corePropulsion is the chain efficiency and the cruise capability point.
func corePropulsion(t *testing.T) calculator.Propulsion {
	t.Helper()
	q := func(v float64, u calculator.Unit) calculator.Quantity { return coreQuantity(t, v, u) }
	return calculator.Propulsion{
		Efficiency: calculator.PropulsionEfficiency{
			Total: 0.55, Basis: "synthetic fixture assumption at the cruise condition",
			Evidence: calculator.EvidenceAssumed,
		},
		Capabilities: []calculator.PropulsionCapability{{
			Name:  powerCruisePoint,
			Basis: "synthetic fixture assumption standing in for a wind-tunnel measurement",
			Kind:  calculator.CapabilityInFlight,
			Speed: q(16, calculator.MeterPerSecond), Density: calculator.SeaLevelISADensity(),
			DensityBasis: "ISA sea level", Voltage: q(14.8, calculator.Volt),
			RPM: q(9000, calculator.RevolutionPerMinute), Throttle: 0.75,
			Thrust: q(6, calculator.Newton), ElectricalPower: q(150, calculator.Watt),
			Evidence: calculator.EvidenceMeasured,
		}},
	}
}

// coreMission is the fixture's single cruise leg and its reserve.
func coreMission(t *testing.T) calculator.Mission {
	t.Helper()
	q := func(v float64, u calculator.Unit) calculator.Quantity { return coreQuantity(t, v, u) }
	zero := q(0, calculator.Degree)
	return calculator.Mission{
		Name: "out and back", Basis: "synthetic fixture profile",
		ReserveFraction: 0.2, ReserveBasis: "synthetic fixture reserve",
		Segments: []calculator.MissionSegment{{
			Name: powerCruiseSeg, Case: powerCaseName,
			Kind: calculator.SegmentCruise, Model: calculator.SegmentModelPolar,
			Timing: calculator.SegmentTimingDuration,
			Speed:  q(16, calculator.MeterPerSecond), ClimbAngle: zero,
			WindAlongTrack: q(0, calculator.MeterPerSecond),
			Duration:       q(600, calculator.Second),
			Capability:     powerCruisePoint,
		}},
	}
}

// The definition survives the round trip: applying an edit and reading the
// design back must not lose the polar, the pack, the loads or the mission.
func TestPowerDefinitionSurvivesTheRoundTrip(t *testing.T) {
	response, err := api.NewService().Apply(context.Background(), api.ApplyRequest{
		Request: api.Request{Session: "task09", Sequence: 2},
		Design:  powerWireDesign(),
		Commands: []api.Command{{
			Kind: "set-propulsion-efficiency",
			Efficiency: &api.Efficiency{
				Total: 0.6, Basis: "revised after a bench run", Evidence: "measured",
			},
		}},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	design := response.Design
	if design.Polar == nil || design.Polar.CD0 != 0.035 {
		t.Error("the drag polar did not survive the round trip")
	}
	if design.Battery == nil || design.Battery.Component != powerBatteryPart {
		t.Error("the flight pack did not survive the round trip")
	}
	if len(design.Auxiliary) != 2 {
		t.Errorf("got %d auxiliary loads back, want 2", len(design.Auxiliary))
	}
	if design.Mission == nil || len(design.Mission.Segments) != 1 {
		t.Error("the mission did not survive the round trip")
	}
	if design.Propulsion == nil || design.Propulsion.Efficiency != 0.6 {
		t.Error("the efficiency edit was not applied")
	}
	if len(design.Propulsion.Capabilities) != 2 {
		t.Errorf("got %d capability points back, want 2", len(design.Propulsion.Capabilities))
	}
}

// A command that misuses a union field is refused rather than having the field
// silently dropped, exactly as the Task 05 commands are.
func TestPowerCommandRejectsAFieldItDoesNotTake(t *testing.T) {
	_, err := api.NewService().Apply(context.Background(), api.ApplyRequest{
		Request:  api.Request{Session: "task09", Sequence: 3},
		Design:   powerWireDesign(),
		Commands: []api.Command{{Kind: "remove-capability", Name: powerStaticPoint, Ratio: 0.5}},
	})
	if issues := wantFailure(t, err, api.FailureInvalid); len(issues) == 0 {
		t.Fatal("the refusal names no field")
	}
}

// Removing a capability point a segment still checks against is refused, so a
// thrust result never survives the measurement behind it.
func TestRemovingACapabilityAThrustCheckNeedsIsRefused(t *testing.T) {
	_, err := api.NewService().Apply(context.Background(), api.ApplyRequest{
		Request:  api.Request{Session: "task09", Sequence: 4},
		Design:   powerWireDesign(),
		Commands: []api.Command{{Kind: "remove-capability", Name: powerCruisePoint}},
	})
	if err == nil {
		t.Fatal("a capability point a segment depends on was removed")
	}
	_ = wantFailure(t, err, api.FailureInvalid)
}

// The bounded power search crosses the boundary with its intervals, its
// brackets and the statement that it selected nothing.
func TestPowerSearchCrossesTheBoundary(t *testing.T) {
	design := powerWireDesign()
	design.Requirements = nil
	response, err := api.NewService().PowerSearch(context.Background(), api.PowerSearchRequest{
		Request: api.Request{Session: "task09", Sequence: 5},
		Design:  design,
		Settings: api.PowerSearchSettings{
			Driver:  "wing.area.reference",
			From:    api.Quantity{Value: 0.15, Unit: "m^2"},
			To:      api.Quantity{Value: 0.30, Unit: "m^2"},
			Ceiling: api.Quantity{Value: 66.5, Unit: "W"},
			Samples: 31,
		},
	})
	if err != nil {
		t.Fatalf("PowerSearch: %v", err)
	}
	if !response.Found || len(response.Intervals) != 1 {
		t.Fatalf("got %d intervals, want one: %s", len(response.Intervals), response.Detail)
	}
	interval := response.Intervals[0]
	if interval.BelowFirst == nil || interval.AboveLast == nil {
		t.Error("a closed interval crossed the boundary without its brackets")
	}
	if interval.Detail == "" || response.Detail == "" {
		t.Error("the search crossed the boundary without saying what it means")
	}
	if response.Unique {
		t.Error("a range of feasible wings was reported as a single answer")
	}
	if response.SolveMode == "" || response.Snapshot == "" || response.SettingsFingerprint == "" {
		t.Error("the answer cannot be matched against the question it was asked")
	}
	if len(response.Candidates) != 31 {
		t.Errorf("got %d candidates, want 31", len(response.Candidates))
	}
}

// A search with no ceiling is refused at the boundary with a field issue, not
// answered with an empty result.
func TestPowerSearchWithoutACeilingIsRefused(t *testing.T) {
	_, err := api.NewService().PowerSearch(context.Background(), api.PowerSearchRequest{
		Request: api.Request{Session: "task09", Sequence: 6},
		Design:  powerWireDesign(),
		Settings: api.PowerSearchSettings{
			Driver:  "wing.area.reference",
			From:    api.Quantity{Value: 0.2, Unit: "m^2"},
			To:      api.Quantity{Value: 0.8, Unit: "m^2"},
			Samples: 9,
		},
	})
	if err == nil {
		t.Fatal("a search with no ceiling was accepted")
	}
	_ = wantFailure(t, err, api.FailureInvalid)
}

// Withdrawing the auxiliary demand leaves the mission without a complete
// electrical figure, and the boundary says so rather than reporting a smaller
// number.
func TestMissingAuxiliaryDemandCrossesTheBoundaryAsMissing(t *testing.T) {
	design := powerWireDesign()
	design.Auxiliary = nil
	evaluation := evaluatePower(t, design)
	if evaluation.Electrical.Status != "missing" {
		t.Errorf("electrical status = %q, want missing", evaluation.Electrical.Status)
	}
	if evaluation.Mission.Status == "computed" {
		t.Error("a mission energy crossed the boundary with no auxiliary demand behind it")
	}
	if evaluation.PowerFeasibility.Status == "computed" {
		t.Error("a feasibility claim crossed the boundary with no auxiliary demand behind it")
	}
}

// Every array field the Task 09 contract declares is an array on the wire, not
// null, so a client that trusted the declared type can iterate it.
func TestPowerArrayFieldsAreNeverNull(t *testing.T) {
	evaluation := evaluatePower(t, powerWireDesign())
	if evaluation.Electrical.Loads == nil {
		t.Error("electrical.loads is null")
	}
	if evaluation.Mission.Segments == nil || evaluation.Mission.Traces == nil {
		t.Error("a mission array is null")
	}
	if evaluation.PowerFeasibility.Checks == nil {
		t.Error("powerFeasibility.checks is null")
	}
	if evaluation.ThrustChecks == nil {
		t.Error("thrustChecks is null")
	}
	for n := range evaluation.Mission.Segments {
		if evaluation.Mission.Segments[n].Power.Traces == nil {
			t.Errorf("segment %d power traces are null", n)
		}
	}
}

// The published limits cover the Task 09 lists, so a client can respect them
// instead of discovering them by being refused.
func TestDiscoveryPublishesThePowerLimits(t *testing.T) {
	limits := api.NewService().Discover().Limits
	for _, check := range []struct {
		name string
		got  int
	}{
		{"maxCapabilities", limits.MaxCapabilities},
		{"maxThrustTargets", limits.MaxThrustTargets},
		{"maxAuxiliaryLoads", limits.MaxAuxiliaryLoads},
		{"maxMissionSegments", limits.MaxMissionSegments},
	} {
		if check.got <= 0 {
			t.Errorf("%s is not published", check.name)
		}
	}
}

// The Task 09 equations reach the discovery document with their provenance, so
// a client can see which relations are the book's and which are this project's
// electric adaptation without reading the source.
func TestPowerEquationsArePublishedWithTheirProvenance(t *testing.T) {
	equations := api.NewService().Equations()
	want := map[string]string{
		"aero.drag-coefficient":           "book",
		"aero.oswald-straight-wing":       "book",
		"aero.lift-to-drag":               "book",
		"aero.drag-force":                 "supplementary",
		"power.electrical-required":       "supplementary",
		"flight.required-thrust":          "supplementary",
		"battery.usable-energy":           "derived",
		"mission.energy-budget":           "derived",
		"mission.endurance-constant-draw": "derived",
	}
	found := map[string]bool{}
	for n := range equations {
		equation := &equations[n]
		kind, listed := want[equation.ID]
		if !listed {
			continue
		}
		found[equation.ID] = true
		if equation.Source.Kind != kind {
			t.Errorf("%s: source kind = %q, want %q", equation.ID, equation.Source.Kind, kind)
		}
		if len(equation.Assumptions) == 0 {
			t.Errorf("%s: published with no assumptions", equation.ID)
		}
		if equation.Source.Adaptation == "" {
			t.Errorf("%s: published with no adaptation record", equation.ID)
		}
	}
	for id := range want {
		if !found[id] {
			t.Errorf("%s is not published at all", id)
		}
	}
}

// The units the power model needs are published with their conversion factors,
// so a worksheet can render an SI value in mAh or rpm without its own table.
func TestPowerUnitsArePublished(t *testing.T) {
	units := api.NewService().Units()
	want := map[string]float64{
		"mAh": 3.6, "Ah": 3600, "V": 1, "A": 1, "rpm": 1.0 / 60.0,
		"Wh": 3600, "min": 60, "h": 3600, "km": 1000,
	}
	for n := range units {
		unit := &units[n]
		factor, listed := want[unit.Symbol]
		if !listed {
			continue
		}
		if math.Abs(unit.FactorToSI-factor) > 1e-15*math.Abs(factor) {
			t.Errorf("%s: factor = %v, want %v", unit.Symbol, unit.FactorToSI, factor)
		}
		delete(want, unit.Symbol)
	}
	for symbol := range want {
		t.Errorf("%s is not published", symbol)
	}
}
