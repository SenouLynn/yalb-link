package calculator_test

import (
	"testing"

	"yalb.aero/calculator"
)

// Each load is counted exactly once, on the pack side. The avionics figure is
// already a pack-side measurement and is not converted; the servo figure is a
// load-side one and is divided by its regulator's efficiency exactly once.
func TestAuxiliaryLoadsAreCountedOnceOnThePackSide(t *testing.T) {
	budget := rcDesign(t).ElectricalBudget()
	if budget.Status != calculator.ResultComputed {
		t.Fatalf("budget status = %v: %s", budget.Status, budget.Detail)
	}
	if !budget.Complete {
		t.Errorf("budget is incomplete: %s", budget.Detail)
	}
	// 5 W pack side, plus 2.4 W at the load through an 80% regulator = 3 W.
	if want := 8.0; !powerTol.ok(budget.Continuous.SI(), want) {
		t.Errorf("continuous draw = %v W, want %v", budget.Continuous.SI(), want)
	}
	// 7 W pack side, plus 24 W at the load through the same regulator = 30 W.
	if want := 37.0; !powerTol.ok(budget.Peak.SI(), want) {
		t.Errorf("peak draw = %v W, want %v", budget.Peak.SI(), want)
	}
	if len(budget.Loads) != 2 {
		t.Fatalf("the budget lists %d loads, want 2", len(budget.Loads))
	}
	for _, load := range budget.Loads {
		if !load.Known {
			t.Errorf("%s did not contribute: %s", load.Name, load.Detail)
		}
	}
}

// A load-side figure with no regulator efficiency cannot be converted, and
// counting it as if it were pack-side would silently drop the regulator's loss.
func TestLoadSideDrawWithoutARegulatorEfficiencyIsRefused(t *testing.T) {
	d := rcDesign(t)
	d.Auxiliary[1].RegulatorEfficiency = 0
	err := d.Validate()
	if err == nil {
		t.Fatal("a load-side draw with no regulator efficiency was accepted")
	}
	wantIssue(t, err, "auxiliary."+rcServos, calculator.IssueMissing)
}

// Applying a regulator efficiency to a figure already measured at the pack
// would count that loss twice.
func TestPackSideDrawRefusesARegulatorEfficiency(t *testing.T) {
	d := rcDesign(t)
	d.Auxiliary[0].RegulatorEfficiency = 0.8
	err := d.Validate()
	if err == nil {
		t.Fatal("a pack-side draw accepted a regulator efficiency")
	}
	detail := wantIssue(t, err, "auxiliary."+rcAvionics, calculator.IssueInvalid)
	if detail == "" {
		t.Error("the refusal carries no explanation")
	}
}

// An aircraft with no listed avionics demand has an unknown electrical budget,
// not a zero one, and that missing answer propagates: no complete electrical
// figure follows for any segment, so no feasibility claim can be made.
func TestMissingAuxiliaryDemandBlocksTheElectricalClaim(t *testing.T) {
	d := rcDesign(t)
	d.Auxiliary = nil

	budget := d.ElectricalBudget()
	if budget.Status != calculator.ResultMissing {
		t.Errorf("budget status = %v, want missing: silence is not a zero draw", budget.Status)
	}
	if budget.Detail == "" {
		t.Error("the missing budget carries no explanation")
	}

	mission := d.MissionAnalysis()
	if mission.Status == calculator.ResultComputed {
		t.Error("a mission energy was produced with no auxiliary demand established")
	}
	feasibility := d.PowerFeasibility(mission, budget)
	if feasibility.Status == calculator.ResultComputed {
		t.Error("a feasibility claim was made with no auxiliary demand established")
	}
}

// A design that genuinely draws nothing says so, and then the budget is a real
// zero rather than a silence.
func TestAStatedZeroDrawIsAnAnswer(t *testing.T) {
	d := rcDesign(t)
	d.Auxiliary = []calculator.AuxiliaryLoad{{
		Name:       "no avionics fitted",
		Basis:      "free-flight airframe with no receiver, autopilot or servos",
		Continuous: mustQ(t, 0, calculator.Watt),
		Side:       calculator.RegulatorPackSide,
		Evidence:   calculator.EvidenceMeasured,
	}}
	budget := d.ElectricalBudget()
	if budget.Status != calculator.ResultComputed {
		t.Fatalf("budget status = %v: %s", budget.Status, budget.Detail)
	}
	if budget.Continuous.SI() != 0 {
		t.Errorf("continuous draw = %v, want 0", budget.Continuous.SI())
	}
}

// Adding a payload's mass and its draw affects the mass and the electrical
// budget once each, through two separate lists, and neither leaks into the
// other.
func TestPayloadAffectsMassAndElectricalBudgetExactlyOnce(t *testing.T) {
	before := rcDesign(t)
	beforeMass := before.AllUpMass().SI()
	beforeDraw := before.ElectricalBudget().Continuous.SI()

	session := calculator.NewSession("payload", rcDesign(t))
	mustDo(t, session, calculator.SetComponent{
		Item: placedMass(t, "gimbal", calculator.ComponentPayload, 0.25, 0.02),
	})
	mustDo(t, session, calculator.SetAuxiliaryLoad{Load: calculator.AuxiliaryLoad{
		Name:       "gimbal",
		Basis:      "synthetic fixture assumption",
		Component:  "gimbal",
		Continuous: mustQ(t, 4, calculator.Watt),
		Peak:       mustQ(t, 12, calculator.Watt),
		Side:       calculator.RegulatorPackSide,
		Evidence:   calculator.EvidenceAssumed,
	}})
	after := session.Design()

	if want := beforeMass + 0.25; !powerTol.ok(after.AllUpMass().SI(), want) {
		t.Errorf("all-up mass = %v, want %v", after.AllUpMass().SI(), want)
	}
	afterBudget := after.ElectricalBudget()
	if want := beforeDraw + 4; !powerTol.ok(afterBudget.Continuous.SI(), want) {
		t.Errorf("continuous draw = %v, want %v", afterBudget.Continuous.SI(), want)
	}
	if want := 37.0 + 12; !powerTol.ok(afterBudget.Peak.SI(), want) {
		t.Errorf("peak draw = %v, want %v", afterBudget.Peak.SI(), want)
	}
}

// The pack is weighed in exactly one place. The Battery holds no mass, so the
// only route to the all-up mass is the component it names, and that component
// must actually be the battery.
func TestBatteryMassIsCarriedByExactlyOneComponent(t *testing.T) {
	d := rcDesign(t)
	d.Battery.Component = "camera"
	err := d.Validate()
	if err == nil {
		t.Fatal("the pack was allowed to name a component that is not the battery")
	}
	wantIssue(t, err, "battery.component", calculator.IssueInvalid)

	missing := rcDesign(t)
	missing.Battery.Component = ""
	err = missing.Validate()
	if err == nil {
		t.Fatal("the pack was allowed to name no component at all")
	}
	detail := wantIssue(t, err, "battery.component", calculator.IssueMissing)
	if detail == "" {
		t.Error("the refusal carries no explanation")
	}
}

// A mission whose energy fits can still exceed what the pack may deliver at its
// worst moment. The two answers are produced separately and reported separately.
func TestEnergyPassesWhilePeakSupplyFails(t *testing.T) {
	d := rcDesign(t)
	// A servo bank that is trivial on average and severe when it all moves.
	d.Auxiliary[1].Peak = mustQ(t, 700, calculator.Watt)

	budget := d.ElectricalBudget()
	mission := d.MissionAnalysis()
	if mission.EnergyStatus != calculator.LimitMet {
		t.Fatalf("energy status = %v (%s), want met", mission.EnergyStatus, mission.Detail)
	}
	feasibility := d.PowerFeasibility(mission, budget)
	peak := supplyCheckNamed(t, feasibility, "motor and controller peak power")
	if peak.Status != calculator.LimitUnmet {
		t.Errorf("peak power check = %v (limit %v, actual %v), want unmet",
			peak.Status, peak.Limit, peak.Actual)
	}
	if feasibility.PeakDetail == "" {
		t.Error("the constructed worst case does not say how it was formed")
	}
	current := supplyCheckNamed(t, feasibility, "pack peak current")
	if current.Status != calculator.LimitUnmet {
		t.Errorf("peak current check = %v, want unmet", current.Status)
	}
}

// A rating that was never entered is an unchecked limit, reported as unknown
// rather than omitted, so a builder can see what has not been checked.
func TestUnstatedRatingsAreReportedAsUnchecked(t *testing.T) {
	d := rcDesign(t)
	d.Propulsion.Limits.MaxRPM = calculator.Quantity{}
	budget := d.ElectricalBudget()
	feasibility := d.PowerFeasibility(d.MissionAnalysis(), budget)
	check := supplyCheckNamed(t, feasibility, "propeller and motor rotation rate")
	if check.Status != calculator.LimitUnknown {
		t.Errorf("rotation-rate check = %v, want unknown", check.Status)
	}
	if check.Detail == "" {
		t.Error("an unchecked limit does not say that it is unchecked")
	}
}

// The rotation-rate and voltage checks read the highest condition any
// capability point states, so a bench point taken beyond a rating is caught.
func TestRotationRateCheckReadsTheCapabilityPoints(t *testing.T) {
	d := rcDesign(t)
	d.Propulsion.Limits.MaxRPM = mustQ(t, 10000, calculator.RevolutionPerMinute)
	feasibility := d.PowerFeasibility(d.MissionAnalysis(), d.ElectricalBudget())
	check := supplyCheckNamed(t, feasibility, "propeller and motor rotation rate")
	if check.Status != calculator.LimitUnmet {
		t.Errorf("rotation-rate check = %v, want unmet: the static point runs at 10500 rpm",
			check.Status)
	}
}

// A propeller's tip clearance is an ordinary requirement subject, so it is met,
// unmet or unknown like any other bound.
func TestPropellerClearanceIsCheckedLikeAnyOtherRequirement(t *testing.T) {
	d := rcDesign(t)
	d.Requirements = append(d.Requirements, calculator.Requirement{
		Name:     "propeller clearance",
		Subject:  calculator.SubjectPropellerClearance,
		Minimum:  mustQ(t, 75, calculator.Millimeter),
		Priority: calculator.PriorityRequired,
		Basis:    "synthetic fixture assumption for a grass strip",
	})
	// 0.18 m hub height less half of a 0.254 m propeller = 0.053 m.
	check := requireCheck(t, d.Assess(), "propeller clearance", "", calculator.BoundLower)
	wantStatus(t, check, calculator.LimitUnmet, calculator.ResultComputed)
	if !powerTol.ok(check.Actual.SI(), 0.053) {
		t.Errorf("clearance = %v m, want 0.053", check.Actual.SI())
	}

	taller := rcDesign(t)
	taller.Propulsion.Limits.PropellerHubHeight = mustQ(t, 0.25, calculator.Meter)
	taller.Requirements = append(taller.Requirements, d.Requirements[len(d.Requirements)-1])
	wantStatus(t, requireCheck(t, taller.Assess(), "propeller clearance", "", calculator.BoundLower),
		calculator.LimitMet, calculator.ResultComputed)
}

// A thrust-to-weight target names the capability point it is stated at, so a
// launch target is never answered with a cruise measurement.
func TestThrustTargetIsCheckedAtItsOwnCapabilityPoint(t *testing.T) {
	d := rcDesign(t)
	d.Propulsion.Targets = []calculator.ThrustTarget{
		{
			Name: "hand launch", Basis: "synthetic fixture target",
			Capability: rcStaticPt, Ratio: 0.8, Priority: calculator.PriorityRequired,
		},
		{
			Name: "cruise margin", Basis: "synthetic fixture target",
			Capability: rcCruisePt, Ratio: 0.8, Priority: calculator.PriorityPreferred,
		},
	}
	checks := d.ThrustChecks()
	if len(checks) != 2 {
		t.Fatalf("got %d thrust checks, want 2", len(checks))
	}
	// 22 N static against 2.5 kg is T/W 0.897; 6 N in flight is 0.245.
	if checks[0].Status != calculator.LimitMet {
		t.Errorf("static target = %v (%s), want met", checks[0].Status, checks[0].Detail)
	}
	if checks[1].Status != calculator.LimitUnmet {
		t.Errorf("cruise target = %v, want unmet: the same target at a different condition is a "+
			"different question", checks[1].Status)
	}
	for _, check := range checks {
		if check.Condition == "" {
			t.Errorf("%s reports a ratio with no condition", check.Name)
		}
	}
}

// A target and the available thrust stay separate: a required target that no
// point can answer is unknown, not met.
func TestThrustTargetWithoutItsCapabilityPointIsUnknown(t *testing.T) {
	d := rcDesign(t)
	d.Propulsion.Targets = []calculator.ThrustTarget{{
		Name: "launch", Basis: "synthetic fixture target",
		Capability: "a point nobody measured", Ratio: 0.8, Priority: calculator.PriorityRequired,
	}}
	checks := d.ThrustChecks()
	if len(checks) != 1 || checks[0].Status != calculator.LimitUnknown {
		t.Fatalf("checks = %+v, want one unknown", checks)
	}
	if err := d.Validate(); err == nil {
		t.Error("a target naming a point the design does not define was accepted")
	}
}

// Energy conversions are checked in the units a builder actually enters, so a
// mistyped mAh or Wh factor is caught rather than trusted.
func TestEnergyUnitConversionsRoundTrip(t *testing.T) {
	ev := eval{t}
	fromCapacity := ev.ok(calculator.BatteryEnergyFromChargeAndVoltage(
		mustQ(t, 5000, calculator.MilliampereHour), mustQ(t, 14.8, calculator.Volt)))
	if got := inUnit(t, fromCapacity.Value, calculator.WattHour); !powerTol.ok(got, 74) {
		t.Errorf("5000 mAh at 14.8 V = %v Wh, want 74", got)
	}
	inAmpHours := ev.ok(calculator.BatteryEnergyFromChargeAndVoltage(
		mustQ(t, 5, calculator.AmpereHour), mustQ(t, 14.8, calculator.Volt)))
	if !sameWithin(inAmpHours.Value.SI(), fromCapacity.Value.SI()) {
		t.Errorf("5 Ah gives %v J and 5000 mAh gives %v J",
			inAmpHours.Value.SI(), fromCapacity.Value.SI())
	}
	entered := calculator.Battery{
		Mode: calculator.BatteryEnergyEntered, Energy: mustQ(t, 74, calculator.WattHour),
		UsableFraction: 0.8,
	}
	usable := ev.ok(entered.UsableEnergy())
	if !powerTol.ok(usable.Value.SI(), rcUsableEnergy) {
		t.Errorf("entered-energy pack gives %v J usable, want %v", usable.Value.SI(), rcUsableEnergy)
	}
}

// A pack cannot answer the same question twice. Stating an energy alongside a
// capacity and a voltage is refused rather than silently resolved.
func TestPackRefusesTwoAnswersForItsEnergy(t *testing.T) {
	d := rcDesign(t)
	d.Battery.Energy = mustQ(t, 60, calculator.WattHour)
	err := d.Validate()
	if err == nil {
		t.Fatal("a pack with both a capacity and an entered energy was accepted")
	}
	wantIssue(t, err, "battery.energy", calculator.IssueInvalid)
}

// Voltage times current and a stated electrical power must agree, or the point
// is refused rather than being read either way.
func TestCapabilityPowerAndCurrentMustAgree(t *testing.T) {
	capability := rcCapability(t)
	capability.Current = mustQ(t, 25, calculator.Ampere)
	if _, err := capability.ElectricalDraw(); err == nil {
		t.Fatal("a point whose stated power disagrees with V*I was accepted")
	}
	capability.Current = mustQ(t, 150.0/14.8, calculator.Ampere)
	if _, err := capability.ElectricalDraw(); err != nil {
		t.Errorf("a consistent point was refused: %v", err)
	}
}
