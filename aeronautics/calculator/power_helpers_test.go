package calculator_test

import (
	"testing"

	"yalb.aero/calculator"
)

// The Task 09 electric fixtures, computed independently at 50 significant
// digits and quoted here to 25. They are synthetic: the primary reference's
// powerplant chapter sizes a piston engine and its mission analysis burns fuel,
// so neither produces a number an electric RC aircraft can be checked against.
// testdata/electric-power-fixtures.md records the derivation in full.
const (
	rcMass           = 2.5
	rcArea           = 0.40
	rcAspect         = 8.0
	rcCD0            = 0.035
	rcOswald         = 0.85
	rcChainEff       = 0.55
	rcAuxContinuous  = 8.0
	rcCruiseSpeed    = 16.0
	rcCruiseCL       = 0.3908900669642857142857143
	rcCruiseCD       = 0.0421523784130521282030199
	rcCruiseLD       = 9.273262427423310097793621
	rcCruiseDrag     = 2.643797174066629480893408
	rcCruiseUseful   = 42.30075478506607169429453
	rcCruiseElectric = 84.91046324557467580780824

	rcClimbSpeed    = 14.0
	rcClimbAngleDeg = 10.0
	rcClimbCL       = 0.5027938854163458020298246
	rcClimbThrust   = 6.506222357951842833729171
	rcClimbElectric = 173.6129327478650903131062

	rcNominalEnergy = 266400.0
	rcUsableEnergy  = 213120.0
	rcEnergyBudget  = 170496.0
	rcEndurance     = 2007.950416038812879198584

	rcHeadwindGround   = 13.0
	rcHeadwindDistance = 7800.0
	rcCruiseSegEnergy  = 50946.27794734480548468494
)

// powerTol is the acceptance tolerance for the fixtures above.
var powerTol = tol{abs: 1e-9, rel: 1e-13}

// Names the power fixtures refer to, so a typo is a compile error rather than a
// silently unmatched lookup.
const (
	rcCase       = "cruise, clean, sea level"
	rcBatteryCmp = "flight pack"
	rcAirframe   = "airframe"
	rcMotorCmp   = "motor and propeller"
	rcCruiseSeg  = "cruise out"
	rcClimbSeg   = "climb"
	rcReturnSeg  = "return"
	rcAvionics   = "autopilot and receiver"
	rcServos     = "servos"
	rcCruisePt   = "bench, 4S, cruise speed"
	rcStaticPt   = "bench, 4S, static"
)

func rcPolar(t *testing.T) calculator.DragPolar {
	t.Helper()
	return calculator.DragPolar{
		CD0:              rcCD0,
		CD0Basis:         "synthetic fixture assumption at aircraft level, not a default",
		OswaldEfficiency: rcOswald,
		EfficiencyBasis:  "synthetic fixture assumption, not a default",
		Configuration:    "clean",
		CLValidMin:       -0.2,
		CLValidMax:       1.1,
		Scope:            calculator.ScopeAircraft,
		Evidence:         calculator.EvidenceAssumed,
	}
}

// rcAuxiliary is a complete auxiliary budget: an always-on avionics draw
// measured on the pack side, and a servo draw measured at the load with its
// regulator's efficiency stated so the loss is counted exactly once.
func rcAuxiliary(t *testing.T) []calculator.AuxiliaryLoad {
	t.Helper()
	return []calculator.AuxiliaryLoad{
		{
			Name:       rcAvionics,
			Basis:      "synthetic fixture assumption",
			Continuous: mustQ(t, 5, calculator.Watt),
			Peak:       mustQ(t, 7, calculator.Watt),
			Side:       calculator.RegulatorPackSide,
			Evidence:   calculator.EvidenceAssumed,
		},
		{
			Name:                rcServos,
			Basis:               "synthetic fixture assumption",
			Continuous:          mustQ(t, 2.4, calculator.Watt),
			Peak:                mustQ(t, 24, calculator.Watt),
			RegulatorEfficiency: 0.8,
			Side:                calculator.RegulatorLoadSide,
			Evidence:            calculator.EvidenceAssumed,
		},
	}
}

// rcBattery is the fixture pack. It holds no mass: rcBatteryCmp is the
// component that does.
func rcBattery(t *testing.T) calculator.Battery {
	t.Helper()
	return calculator.Battery{
		Basis:                  "synthetic fixture assumption, labelled 5000 mAh 4S",
		Component:              rcBatteryCmp,
		Mode:                   calculator.BatteryEnergyFromCapacity,
		Capacity:               mustQ(t, 5000, calculator.MilliampereHour),
		NominalVoltage:         mustQ(t, 14.8, calculator.Volt),
		UsableFraction:         0.8,
		ContinuousCurrentLimit: mustQ(t, 40, calculator.Ampere),
		PeakCurrentLimit:       mustQ(t, 60, calculator.Ampere),
		Evidence:               calculator.EvidenceAssumed,
	}
}

// rcCapability is an in-flight capability point at exactly the cruise speed,
// and rcStaticCapability the static point that must never answer for it.
func rcCapability(t *testing.T) calculator.PropulsionCapability {
	t.Helper()
	return calculator.PropulsionCapability{
		Name:            rcCruisePt,
		Basis:           "synthetic fixture assumption standing in for a wind-tunnel measurement",
		Kind:            calculator.CapabilityInFlight,
		Speed:           mustQ(t, rcCruiseSpeed, calculator.MeterPerSecond),
		Density:         calculator.SeaLevelISADensity(),
		DensityBasis:    "ISA sea level",
		Voltage:         mustQ(t, 14.8, calculator.Volt),
		RPM:             mustQ(t, 9000, calculator.RevolutionPerMinute),
		Throttle:        0.75,
		Thrust:          mustQ(t, 6, calculator.Newton),
		ElectricalPower: mustQ(t, 150, calculator.Watt),
		Evidence:        calculator.EvidenceMeasured,
	}
}

func rcStaticCapability(t *testing.T) calculator.PropulsionCapability {
	t.Helper()
	return calculator.PropulsionCapability{
		Name:            rcStaticPt,
		Basis:           "synthetic fixture assumption standing in for a static bench test",
		Kind:            calculator.CapabilityStatic,
		Speed:           mustQ(t, 0, calculator.MeterPerSecond),
		Density:         calculator.SeaLevelISADensity(),
		DensityBasis:    "ISA sea level",
		Voltage:         mustQ(t, 14.8, calculator.Volt),
		RPM:             mustQ(t, 10500, calculator.RevolutionPerMinute),
		Throttle:        1,
		Thrust:          mustQ(t, 22, calculator.Newton),
		ElectricalPower: mustQ(t, 340, calculator.Watt),
		Evidence:        calculator.EvidenceMeasured,
	}
}

func rcCruiseSegment(t *testing.T) calculator.MissionSegment {
	t.Helper()
	return calculator.MissionSegment{
		Name:           rcCruiseSeg,
		Case:           rcCase,
		Kind:           calculator.SegmentCruise,
		Model:          calculator.SegmentModelPolar,
		Timing:         calculator.SegmentTimingDuration,
		Speed:          mustQ(t, rcCruiseSpeed, calculator.MeterPerSecond),
		ClimbAngle:     mustQ(t, 0, calculator.Degree),
		WindAlongTrack: mustQ(t, 0, calculator.MeterPerSecond),
		Duration:       mustQ(t, 600, calculator.Second),
		Capability:     rcCruisePt,
	}
}

// rcDesign is the Task 09 fixture aircraft: a flying wing at 2.5 kg on a
// 0.40 m^2 wing of aspect ratio 8, with a complete polar, propulsion chain,
// pack, auxiliary budget and a single cruise segment.
//
// The mass mode is the component inventory, so that the pack's mass reaches the
// all-up mass through exactly one path and a battery change moves the balance,
// the loading and the stall speed as well as the energy.
func rcDesign(t *testing.T) calculator.Design {
	t.Helper()
	fc := baseCase(t)
	fc.Name = rcCase
	fc.LoadFactor = 1
	zero := mustQ(t, 0, calculator.Degree)
	return calculator.Design{
		Name:          "power fixture",
		Configuration: calculator.ConfigurationFlyingWing,
		MassMode:      calculator.MassModeComponents,
		Wing: calculator.WingDefinition{
			Name: "main wing",
			Drivers: calculator.PlanformDrivers{
				Shape:       calculator.ShapeRectangle,
				Area:        mustQ(t, rcArea, calculator.SquareMeter),
				AspectRatio: rcAspect,
			},
			Sweep:          zero,
			SweepReference: 0.25,
			Dihedral:       zero,
			Twist:          zero,
			Incidence:      zero,
			AreaBasis:      calculator.AreaBasisReferenceTrapezoid,
		},
		Cases: []calculator.DesignCase{{
			Case: fc, Priority: calculator.PriorityRequired,
			CLmaxEvidence: calculator.EvidenceAssumed,
		}},
		Components: []calculator.MassItem{
			placedMass(t, rcAirframe, calculator.ComponentAirframe, 1.4, 0.15),
			placedMass(t, rcMotorCmp, calculator.ComponentMotor, 0.3, -0.05),
			placedMass(t, rcBatteryCmp, calculator.ComponentBattery, 0.5, 0.05),
			placedMass(t, "avionics and servos", calculator.ComponentAvionics, 0.2, 0.1),
			placedMass(t, "camera", calculator.ComponentPayload, 0.1, 0.0),
		},
		Polar:     rcPolar(t),
		Auxiliary: rcAuxiliary(t),
		Battery:   rcBattery(t),
		Propulsion: calculator.Propulsion{
			Efficiency: calculator.PropulsionEfficiency{
				Total:    rcChainEff,
				Basis:    "synthetic fixture assumption at the cruise condition",
				Evidence: calculator.EvidenceAssumed,
			},
			Limits: calculator.PropulsionLimits{
				Basis:                        "synthetic fixture ratings",
				MaxContinuousElectricalPower: mustQ(t, 300, calculator.Watt),
				MaxPeakElectricalPower:       mustQ(t, 500, calculator.Watt),
				MaxRPM:                       mustQ(t, 12000, calculator.RevolutionPerMinute),
				MaxVoltage:                   mustQ(t, 16.8, calculator.Volt),
				PropellerDiameter:            mustQ(t, 0.254, calculator.Meter),
				PropellerHubHeight:           mustQ(t, 0.18, calculator.Meter),
			},
			Capabilities: []calculator.PropulsionCapability{rcCapability(t), rcStaticCapability(t)},
		},
		Mission: calculator.Mission{
			Name:            "out and back",
			Basis:           "synthetic fixture profile",
			ReserveFraction: 0.2,
			ReserveBasis:    "synthetic fixture reserve",
			Segments:        []calculator.MissionSegment{rcCruiseSegment(t)},
		},
	}
}

// placedMass builds a component on the centreline at the given x station.
func placedMass(t *testing.T, name string, role calculator.ComponentRole, mass, x float64) calculator.MassItem {
	t.Helper()
	return calculator.MassItem{
		Name:  name,
		Role:  role,
		Basis: "synthetic fixture assumption",
		Mass:  mustQ(t, mass, calculator.Kilogram),
		Position: calculator.Point{
			X: mustQ(t, x, calculator.Meter),
			Y: mustQ(t, 0, calculator.Meter),
			Z: mustQ(t, 0, calculator.Meter),
		},
	}
}

// supplied reports whether a Quantity was ever filled in. The zero Quantity is
// dimensionless zero, which no constructor can produce for a dimensional field,
// so it is unambiguously "not entered" — the same rule the package uses
// internally, restated here because tests live outside it.
func supplied(q calculator.Quantity) bool { return q != calculator.Quantity{} }

// segmentNamed returns the named segment result, failing when it is absent.
func segmentNamed(t *testing.T, m calculator.MissionResult, name string) calculator.SegmentResult {
	t.Helper()
	for n := range m.Segments {
		if m.Segments[n].Name == name {
			return m.Segments[n]
		}
	}
	t.Fatalf("no segment named %q in the mission result", name)
	return calculator.SegmentResult{}
}

// supplyCheckNamed returns the named rating check.
func supplyCheckNamed(t *testing.T, f calculator.PowerFeasibility, name string) calculator.SupplyCheck {
	t.Helper()
	for n := range f.Checks {
		if f.Checks[n].Name == name {
			return f.Checks[n]
		}
	}
	t.Fatalf("no supply check named %q; the checks are %v", name, checkNames(f))
	return calculator.SupplyCheck{}
}

func checkNames(f calculator.PowerFeasibility) []string {
	names := make([]string, 0, len(f.Checks))
	for n := range f.Checks {
		names = append(names, f.Checks[n].Name)
	}
	return names
}
