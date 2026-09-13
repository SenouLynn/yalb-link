package calculator_test

import (
	"testing"

	"yalb.aero/calculator"
)

// The whole polar-to-electrical chain reproduces the independently computed
// cruise fixture, and the segment carries a trace for every step so the result
// can be taken apart.
func TestCruiseSegmentReproducesTheElectricFixture(t *testing.T) {
	mission := rcDesign(t).MissionAnalysis()
	if mission.Status != calculator.ResultComputed {
		t.Fatalf("mission status = %v: %s", mission.Status, mission.Detail)
	}
	segment := segmentNamed(t, mission, rcCruiseSeg)
	power := segment.Power

	for _, check := range []struct {
		label string
		got   float64
		want  float64
	}{
		{"CL", power.LiftCoefficient, rcCruiseCL},
		{"CD", power.DragCoefficient, rcCruiseCD},
		{"L/D", power.LiftToDrag, rcCruiseLD},
		{"drag", power.Drag.SI(), rcCruiseDrag},
		{"thrust", power.Thrust.SI(), rcCruiseDrag},
		{"useful power", power.Propulsive.SI(), rcCruiseUseful},
		{"electrical power", power.Electrical.SI(), rcCruiseElectric},
		{"segment energy", segment.Energy.SI(), rcCruiseElectric * 600},
	} {
		if !powerTol.ok(check.got, check.want) {
			t.Errorf("%s = %v, want %v", check.label, check.got, check.want)
		}
	}
	if len(power.Traces) == 0 {
		t.Error("the segment produced no traces, so nothing about it can be inspected")
	}
	if power.Evidence != calculator.EvidenceAssumed {
		t.Errorf("segment evidence = %v, want the weakest of the polar and the chain efficiency",
			power.Evidence)
	}
}

// A climb is a different condition with a different force balance, and the
// model has to resolve it rather than reusing a level answer at a new speed.
func TestClimbSegmentResolvesTheSteadyForceBalance(t *testing.T) {
	d := rcDesign(t)
	climb := rcCruiseSegment(t)
	climb.Name = rcClimbSeg
	climb.Kind = calculator.SegmentClimb
	climb.Speed = mustQ(t, rcClimbSpeed, calculator.MeterPerSecond)
	climb.ClimbAngle = mustQ(t, rcClimbAngleDeg, calculator.Degree)
	climb.Duration = mustQ(t, 30, calculator.Second)
	climb.Capability = ""
	d.Mission.Segments = []calculator.MissionSegment{climb}

	segment := segmentNamed(t, d.MissionAnalysis(), rcClimbSeg)
	if segment.Status != calculator.ResultComputed {
		t.Fatalf("climb status = %v: %s", segment.Status, segment.Detail)
	}
	if !powerTol.ok(segment.Power.LiftCoefficient, rcClimbCL) {
		t.Errorf("climb CL = %v, want %v", segment.Power.LiftCoefficient, rcClimbCL)
	}
	if !powerTol.ok(segment.Power.Thrust.SI(), rcClimbThrust) {
		t.Errorf("climb thrust = %v, want %v", segment.Power.Thrust.SI(), rcClimbThrust)
	}
	if !powerTol.ok(segment.Power.Electrical.SI(), rcClimbElectric) {
		t.Errorf("climb electrical power = %v, want %v",
			segment.Power.Electrical.SI(), rcClimbElectric)
	}
	// The climb costs more than the cruise at a lower airspeed, which is the
	// whole reason the two are separate segments rather than one scaled figure.
	if segment.Power.Electrical.SI() <= rcCruiseElectric {
		t.Error("the climb draws no more than the cruise, so the flight-path angle is being ignored")
	}
}

func TestBatteryEnergyReserveAndEnduranceMatchTheFixture(t *testing.T) {
	d := rcDesign(t)
	ev := eval{t}

	nominal := ev.ok(d.Battery.NominalEnergy())
	if !powerTol.ok(nominal.Value.SI(), rcNominalEnergy) {
		t.Errorf("nominal energy = %v J, want %v", nominal.Value.SI(), rcNominalEnergy)
	}
	if got := inUnit(t, nominal.Value, calculator.WattHour); !powerTol.ok(got, 74) {
		t.Errorf("nominal energy = %v Wh, want 74", got)
	}
	usable := ev.ok(d.Battery.UsableEnergy())
	if !powerTol.ok(usable.Value.SI(), rcUsableEnergy) {
		t.Errorf("usable energy = %v J, want %v", usable.Value.SI(), rcUsableEnergy)
	}

	mission := d.MissionAnalysis()
	if !powerTol.ok(mission.Budget.SI(), rcEnergyBudget) {
		t.Errorf("budget = %v J, want %v", mission.Budget.SI(), rcEnergyBudget)
	}
	endurance := ev.ok(calculator.EnduranceAtConstantDraw(mission.Budget,
		mustQ(t, rcCruiseElectric, calculator.Watt)))
	if !powerTol.ok(endurance.Value.SI(), rcEndurance) {
		t.Errorf("endurance = %v s, want %v", endurance.Value.SI(), rcEndurance)
	}
}

// The reserve is applied once, to the usable energy, and never inside a
// segment. Doubling the segment count at half the duration each must not change
// the budget, and the budget must be exactly the usable energy times 1 - r.
func TestReserveIsAppliedExactlyOnce(t *testing.T) {
	d := rcDesign(t)
	first := rcCruiseSegment(t)
	first.Duration = mustQ(t, 300, calculator.Second)
	second := first
	second.Name = "cruise back"
	d.Mission.Segments = []calculator.MissionSegment{first, second}

	mission := d.MissionAnalysis()
	if mission.Status != calculator.ResultComputed {
		t.Fatalf("mission status = %v: %s", mission.Status, mission.Detail)
	}
	if !powerTol.ok(mission.Budget.SI(), rcEnergyBudget) {
		t.Errorf("budget over two segments = %v J, want the same %v as over one",
			mission.Budget.SI(), rcEnergyBudget)
	}
	if !powerTol.ok(mission.RequiredEnergy.SI(), rcCruiseElectric*600) {
		t.Errorf("required energy = %v J, want the same %v as the single 600 s segment",
			mission.RequiredEnergy.SI(), rcCruiseElectric*600)
	}
	if want := rcUsableEnergy * 0.8; !powerTol.ok(mission.Budget.SI(), want) {
		t.Errorf("budget = %v J, want usable energy times (1 - reserve) = %v",
			mission.Budget.SI(), want)
	}
}

// Wind changes the ground track and nothing else. The energy is identical
// because the aircraft flies the same airspeed for the same time.
func TestWindMovesTheGroundTrackAndNotTheEnergy(t *testing.T) {
	still := rcDesign(t).MissionAnalysis()
	stillSegment := segmentNamed(t, still, rcCruiseSeg)

	d := rcDesign(t)
	d.Mission.Segments[0].WindAlongTrack = mustQ(t, -3, calculator.MeterPerSecond)
	headwind := d.MissionAnalysis()
	segment := segmentNamed(t, headwind, rcCruiseSeg)

	if !powerTol.ok(segment.GroundSpeed.SI(), rcHeadwindGround) {
		t.Errorf("ground speed = %v, want %v", segment.GroundSpeed.SI(), rcHeadwindGround)
	}
	if !powerTol.ok(segment.Distance.SI(), rcHeadwindDistance) {
		t.Errorf("distance = %v, want %v", segment.Distance.SI(), rcHeadwindDistance)
	}
	if !powerTol.ok(segment.Energy.SI(), rcCruiseSegEnergy) {
		t.Errorf("energy into a headwind = %v, want %v", segment.Energy.SI(), rcCruiseSegEnergy)
	}
	if !sameWithin(segment.Energy.SI(), stillSegment.Energy.SI()) {
		t.Errorf("wind changed the segment energy: %v into a headwind and %v in still air",
			segment.Energy.SI(), stillSegment.Energy.SI())
	}
	if stillSegment.Distance.SI() <= segment.Distance.SI() {
		t.Error("a headwind did not shorten the ground distance")
	}
}

// A return leg into a headwind at least as strong as the airspeed makes no
// progress. Stated as a distance it is refused outright; stated as a duration
// it still burns energy for as long as it is flown, and only the distance is
// withheld.
func TestReturnLegIntoAnOverpoweringHeadwind(t *testing.T) {
	d := rcDesign(t)
	leg := rcCruiseSegment(t)
	leg.Name = rcReturnSeg
	leg.Kind = calculator.SegmentReturn
	leg.WindAlongTrack = mustQ(t, -18, calculator.MeterPerSecond)
	leg.Timing = calculator.SegmentTimingDistance
	leg.Duration = calculator.Quantity{}
	leg.Distance = mustQ(t, 5, calculator.Kilometer)
	d.Mission.Segments = []calculator.MissionSegment{leg}

	byDistance := segmentNamed(t, d.MissionAnalysis(), rcReturnSeg)
	if byDistance.Status == calculator.ResultComputed {
		t.Error("a return leg that never arrives was reported as a completed segment")
	}
	if byDistance.Detail == "" {
		t.Error("the refusal carries no explanation")
	}

	timedLeg := leg
	timedLeg.Timing = calculator.SegmentTimingDuration
	timedLeg.Distance = calculator.Quantity{}
	timedLeg.Duration = mustQ(t, 600, calculator.Second)
	byTime := rcDesign(t)
	byTime.Mission.Segments = []calculator.MissionSegment{timedLeg}
	timed := segmentNamed(t, byTime.MissionAnalysis(), rcReturnSeg)
	if timed.Status != calculator.ResultComputed {
		t.Fatalf("a segment flown for a stated time still costs energy: %v %s",
			timed.Status, timed.Detail)
	}
	if supplied(timed.Distance) {
		t.Error("a segment making no headway reported a ground distance")
	}
	if !powerTol.ok(timed.Energy.SI(), rcCruiseElectric*600) {
		t.Errorf("energy = %v, want the same %v the segment costs in still air",
			timed.Energy.SI(), rcCruiseElectric*600)
	}
}

// A condition the aircraft cannot hold must not be reported as affordable. Below
// the case's stall the required CL exceeds CLmax, and the segment is refused
// rather than priced.
func TestFlightBelowTheLiftEnvelopeIsNotPowerFeasible(t *testing.T) {
	d := rcDesign(t)
	d.Mission.Segments[0].Speed = mustQ(t, 8, calculator.MeterPerSecond)
	segment := segmentNamed(t, d.MissionAnalysis(), rcCruiseSeg)
	if segment.Status == calculator.ResultComputed {
		t.Fatal("a condition below the case's stall was priced as if it could be flown")
	}
	if segment.Power.Status == calculator.ResultComputed {
		t.Error("a power figure was produced for a condition the aircraft cannot hold")
	}
	if segment.Power.Detail == "" {
		t.Error("the refusal carries no explanation")
	}
}

// Withdrawing the case's CLmax leaves no lift envelope to check against, which
// is a different answer from an envelope that was checked and passed.
func TestMissingLiftEnvelopeBlocksThePowerResult(t *testing.T) {
	d := rcDesign(t)
	d.Cases[0].Case.CLmax = calculator.LiftCoefficient{}
	segment := segmentNamed(t, d.MissionAnalysis(), rcCruiseSeg)
	if segment.Power.Status == calculator.ResultComputed {
		t.Error("a power figure was produced with no lift envelope to check it against")
	}
}

// A capability point answers only questions at its own condition. The static
// point in the fixture delivers nearly four times the cruise thrust, and using
// it for a cruise segment would make an aircraft look far more capable than it is.
func TestStaticThrustNeverAnswersAnInFlightQuestion(t *testing.T) {
	d := rcDesign(t)
	d.Mission.Segments[0].Capability = rcStaticPt
	segment := segmentNamed(t, d.MissionAnalysis(), rcCruiseSeg)
	if segment.Availability.Status != calculator.LimitUnknown {
		t.Errorf("availability = %v, want unknown: a static measurement says nothing about "+
			"thrust at %v", segment.Availability.Status, d.Mission.Segments[0].Speed)
	}
	if segment.Availability.Detail == "" {
		t.Error("the refusal carries no explanation")
	}
	// The energy is unaffected: energy sufficiency and flight feasibility are
	// separate answers.
	if segment.Status != calculator.ResultComputed {
		t.Errorf("the segment's energy was withheld along with its thrust check: %s", segment.Detail)
	}
}

// An in-flight point at the segment's own speed does answer it.
func TestInFlightCapabilityAnswersAtItsOwnCondition(t *testing.T) {
	segment := segmentNamed(t, rcDesign(t).MissionAnalysis(), rcCruiseSeg)
	if segment.Availability.Status != calculator.LimitMet {
		t.Errorf("availability = %v (%s), want met: the point delivers 6 N and the segment "+
			"needs about %v", segment.Availability.Status, segment.Availability.Detail, rcCruiseDrag)
	}
	if segment.Availability.Evidence != calculator.EvidenceMeasured {
		t.Errorf("availability evidence = %v, want the capability point's grade",
			segment.Availability.Evidence)
	}
}

// A segment with no capability point contributes its energy and leaves its
// flight feasibility unknown, which is the honest state of a leg nobody has
// measured the thrust for.
func TestSegmentWithoutACapabilityPointKeepsItsEnergy(t *testing.T) {
	d := rcDesign(t)
	d.Mission.Segments[0].Capability = ""
	segment := segmentNamed(t, d.MissionAnalysis(), rcCruiseSeg)
	if segment.Status != calculator.ResultComputed {
		t.Fatalf("the segment lost its energy along with its thrust check: %s", segment.Detail)
	}
	if segment.Availability.Status != calculator.LimitUnknown {
		t.Errorf("availability = %v, want unknown", segment.Availability.Status)
	}
}

// An entered estimate stands in for a missing segment model. It is labelled as
// an estimate, it still pays for the avionics, and it produces no thrust, lift
// coefficient or envelope result that could be mistaken for a computed one.
func TestEnteredSegmentEstimateIsLabelledAndCarriesNoFlightResult(t *testing.T) {
	d := rcDesign(t)
	launch := rcCruiseSegment(t)
	launch.Name = "hand launch"
	launch.Kind = calculator.SegmentLaunch
	launch.Model = calculator.SegmentModelEntered
	launch.EnteredPower = mustQ(t, 320, calculator.Watt)
	launch.EnteredBasis = "logged from a previous aircraft's launch, standing in for a launch model"
	launch.EnteredEvidence = calculator.EvidenceMeasured
	launch.Duration = mustQ(t, 8, calculator.Second)
	launch.Capability = rcStaticPt
	d.Mission.Segments = []calculator.MissionSegment{launch}

	segment := segmentNamed(t, d.MissionAnalysis(), "hand launch")
	if segment.Status != calculator.ResultComputed {
		t.Fatalf("the entered estimate produced no energy: %s", segment.Detail)
	}
	if want := 320 + 8.0; !powerTol.ok(segment.Power.Electrical.SI(), want) {
		t.Errorf("electrical power = %v, want the estimate plus the auxiliary draw %v",
			segment.Power.Electrical.SI(), want)
	}
	if supplied(segment.Power.Thrust) {
		t.Error("an entered power estimate produced a required thrust")
	}
	if segment.Power.LiftCoefficient != 0 {
		t.Error("an entered power estimate produced a lift coefficient")
	}
	if segment.Availability.Status != calculator.LimitUnknown {
		t.Error("an entered estimate established a thrust margin it cannot establish")
	}
	if segment.Power.Detail == "" {
		t.Error("the entered estimate is not labelled as one")
	}
}
