package calculator

import "math"

// ElectricalPower returns P = V*I.
func ElectricalPower(voltage, current Quantity) (Result, error) {
	e := newEvaluation(EqElectricalPower)
	v := e.quantity(portVoltage.Name, voltage)
	i := e.quantity(portCurrent.Name, current)
	return e.finish(v * i)
}

// PropulsivePower returns the useful propulsive power, thrust times true
// airspeed.
//
// It is refused at zero airspeed, and that refusal is the point. Useful
// propulsive power is genuinely zero when the aircraft is not moving, so
// dividing it by a propulsive efficiency to obtain a static electrical draw
// gives zero rather than the substantial current a standing motor actually
// pulls. Static draw is entered as a capability point instead.
func PropulsivePower(thrust, trueAirspeed Quantity) (Result, error) {
	e := newEvaluation(EqPropulsivePower)
	t := e.quantity(portThrust.Name, thrust)
	v := e.quantity(portSpeed.Name, trueAirspeed)
	if v == 0 {
		e.add(portSpeed.Name, IssueUnsupported,
			"useful propulsive power is thrust times airspeed, which is zero when the aircraft "+
				"is not moving. Dividing that by a propulsive efficiency does not give the "+
				"electrical power a standing aircraft draws; enter a static capability point")
	}
	return e.finish(t * v)
}

// RequiredElectricalPower returns P_elec = P_useful/eta_total + P_aux.
func RequiredElectricalPower(propulsive Quantity, chainEfficiency float64, auxiliary Quantity) (Result, error) {
	e := newEvaluation(EqElectricalRequired)
	useful := e.quantity(portPropulsivePower.Name, propulsive)
	eta := e.scalar(portChainEfficiency.Name, chainEfficiency)
	aux := e.signedQuantity(portAuxiliaryPower.Name, auxiliary)
	if eta == 0 {
		return Result{}, e.issues
	}
	if eta > 1 {
		e.add(portChainEfficiency.Name, IssueInvalid,
			"a chain efficiency above one would take more work out of the pack than went in, "+
				"received "+formatFloat(eta))
	}
	if aux < 0 {
		e.add(portAuxiliaryPower.Name, IssueInvalid,
			"the auxiliary draw cannot be negative, received "+formatFloat(aux))
	}
	return e.finish(useful/eta + aux)
}

// ThrustToWeight returns T/(m*g) at the condition the thrust belongs to.
func ThrustToWeight(thrust, mass Quantity) (Result, error) {
	e := newEvaluation(EqThrustToWeight)
	t := e.quantity(portThrust.Name, thrust)
	m := e.quantity(portMass.Name, mass)
	if m == 0 {
		return Result{}, e.issues
	}
	return e.finish(t / (m * StandardGravity))
}

// PropellerTipClearance returns the hub height less the propeller's radius. The
// result is signed: a propeller whose tips reach below the ground line reports a
// negative clearance rather than being refused, because that is a real and
// checkable answer about an aircraft that cannot be taxied.
func PropellerTipClearance(hubHeight, diameter Quantity) (Result, error) {
	e := newEvaluation(EqPropellerClearance)
	h := e.quantity(portHubHeight.Name, hubHeight)
	d := e.quantity(portPropellerDiameter.Name, diameter)
	return e.finishSigned(h - d/2)
}

// AuxiliaryPackSideDraw converts one load-side figure into the draw the pack
// supplies.
func AuxiliaryPackSideDraw(loadSide Quantity, regulatorEfficiency float64) (Result, error) {
	e := newEvaluation(EqAuxiliaryPackSide)
	p := e.quantity(portLoadPower.Name, loadSide)
	eta := e.scalar(portRegulatorEfficiency.Name, regulatorEfficiency)
	if eta == 0 {
		return Result{}, e.issues
	}
	if eta > 1 {
		e.add(portRegulatorEfficiency.Name, IssueInvalid,
			"a regulator efficiency above one would deliver more than it draws, received "+
				formatFloat(eta))
	}
	return e.finish(p / eta)
}

// BatteryEnergyFromChargeAndVoltage returns E = Q*V_nominal.
func BatteryEnergyFromChargeAndVoltage(capacity, nominalVoltage Quantity) (Result, error) {
	e := newEvaluation(EqBatteryEnergy)
	q := e.quantity(portCharge.Name, capacity)
	v := e.quantity(portVoltage.Name, nominalVoltage)
	return e.finish(q * v)
}

// BatteryUsableEnergy returns E_usable = E * f_usable.
func BatteryUsableEnergy(nominal Quantity, usableFraction float64) (Result, error) {
	e := newEvaluation(EqBatteryUsableEnergy)
	energy := e.quantity(portNominalEnergy.Name, nominal)
	f := e.scalar(portUsableFraction.Name, usableFraction)
	if f > 1 {
		e.add(portUsableFraction.Name, IssueInvalid,
			"more than the pack's nominal energy cannot be usable, received "+formatFloat(f))
	}
	return e.finish(energy * f)
}

// EnergyBudget returns E_budget = E_usable*(1 - reserve). The reserve is applied
// here and nowhere else, which is what makes "reserve exactly once" a property
// of the code rather than a convention a caller has to remember.
func EnergyBudget(usable Quantity, reserveFraction float64) (Result, error) {
	e := newEvaluation(EqEnergyBudget)
	energy := e.quantity(portUsableEnergy.Name, usable)
	reserve := e.scalarSigned(portReserveFraction.Name, reserveFraction)
	if reserve < 0 || reserve >= 1 {
		e.add(portReserveFraction.Name, IssueInvalid,
			"the reserve is a fraction of the usable energy from 0 up to but not including 1, "+
				"received "+formatFloat(reserve))
	}
	return e.finish(energy * (1 - reserve))
}

// SegmentEnergy returns E = P*dt for one segment held at constant draw.
func SegmentEnergy(power, duration Quantity) (Result, error) {
	e := newEvaluation(EqSegmentEnergy)
	p := e.quantity(portSegmentPower.Name, power)
	t := e.quantity(portDuration.Name, duration)
	return e.finish(p * t)
}

// EnduranceAtConstantDraw returns t = E_usable/P_elec. It is a constant-draw
// estimate and says so: a mission whose power varies between segments is
// answered by the segment sum instead.
func EnduranceAtConstantDraw(usable, electrical Quantity) (Result, error) {
	e := newEvaluation(EqEnduranceConstantDraw)
	energy := e.quantity(portUsableEnergy.Name, usable)
	power := e.quantity(portElectricalPower.Name, electrical)
	if power == 0 {
		return Result{}, e.issues
	}
	return e.finish(energy / power)
}

// GroundSpeed returns the speed over the ground along the track: the airspeed's
// horizontal component plus the wind along that track, signed so that a
// headwind stronger than the airspeed reports the backwards progress it is
// rather than an absolute value that would read as forward travel.
func GroundSpeed(trueAirspeed, climbAngle, windAlongTrack Quantity) (Result, error) {
	e := newEvaluation(EqGroundSpeed)
	v := e.quantity(portSpeed.Name, trueAirspeed)
	gamma := e.signedQuantity(portClimbAngle.Name, climbAngle)
	wind := e.signedQuantity(portWindAlongTrack.Name, windAlongTrack)
	return e.finishSigned(v*math.Cos(gamma) + wind)
}

// SegmentDistance returns d = V_ground*t.
func SegmentDistance(groundSpeed, duration Quantity) (Result, error) {
	e := newEvaluation(EqSegmentDistance)
	v := e.signedQuantity(portGroundSpeed.Name, groundSpeed)
	t := e.quantity(portDuration.Name, duration)
	if v <= 0 {
		e.add(portGroundSpeed.Name, IssueUnsupported,
			"the ground speed along this track is "+formatFloat(v)+" m/s, so the aircraft makes "+
				"no forward progress; the headwind is at least as strong as the airspeed's "+
				"horizontal component")
	}
	return e.finish(v * t)
}

// SegmentDuration returns t = d/V_ground.
func SegmentDuration(distance, groundSpeed Quantity) (Result, error) {
	e := newEvaluation(EqSegmentDuration)
	d := e.quantity(portDistance.Name, distance)
	v := e.signedQuantity(portGroundSpeed.Name, groundSpeed)
	if v <= 0 {
		e.add(portGroundSpeed.Name, IssueUnsupported,
			"the ground speed along this track is "+formatFloat(v)+" m/s, so this distance is "+
				"never covered; the headwind is at least as strong as the airspeed's horizontal "+
				"component")
		return Result{}, e.issues
	}
	return e.finish(d / v)
}

// SegmentLift returns the lift a steady condition demands, n*m*g*cos(gamma).
func SegmentLift(mass Quantity, loadFactor float64, climbAngle Quantity) (Result, error) {
	e := newEvaluation(EqSegmentLift)
	m := e.quantity(portMass.Name, mass)
	n := e.scalar(portLoadFactor.Name, loadFactor)
	gamma := e.signedQuantity(portClimbAngle.Name, climbAngle)
	if math.Cos(gamma) <= 0 {
		e.add(portClimbAngle.Name, IssueUnsupported,
			"a flight-path angle at or beyond the vertical is outside the steady force balance "+
				"this model resolves")
	}
	return e.finish(n * m * StandardGravity * math.Cos(gamma))
}

// SegmentLiftCoefficient returns CL = L/(q*S).
func SegmentLiftCoefficient(lift, dynamicPressure, wingArea Quantity) (Result, error) {
	e := newEvaluation(EqSegmentLiftCoefficient)
	l := e.quantity(portLift.Name, lift)
	q := e.quantity(portDynamicPressure.Name, dynamicPressure)
	s := e.quantity(portArea.Name, wingArea)
	if q == 0 || s == 0 {
		return Result{}, e.issues
	}
	return e.finish(l / (q * s))
}

// RequiredThrust returns T = D + m*g*sin(gamma). A descent makes the second term
// negative, so the result may be zero or negative, which means the aircraft
// needs no thrust to hold that flight path; that is reported rather than
// clamped to zero.
func RequiredThrust(drag, mass, climbAngle Quantity) (Result, error) {
	e := newEvaluation(EqRequiredThrust)
	d := e.quantity(portDrag.Name, drag)
	m := e.quantity(portMass.Name, mass)
	gamma := e.signedQuantity(portClimbAngle.Name, climbAngle)
	return e.finishSigned(d + m*StandardGravity*math.Sin(gamma))
}
