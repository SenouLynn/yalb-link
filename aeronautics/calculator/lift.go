package calculator

import "math"

// Weight converts an all-up mass to weight using standard gravity.
func Weight(mass Quantity) (Result, error) {
	e := newEvaluation(EqWeightFromMass)
	m := e.quantity("mass", mass)
	return e.finish(m * StandardGravity)
}

// DynamicPressure returns the free-stream dynamic pressure for the case at the
// given true airspeed.
func DynamicPressure(fc FlightCase, trueAirspeed Quantity) (Result, error) {
	e := newEvaluation(EqDynamicPressure)
	rho := e.density(fc)
	v := e.quantity("true_airspeed", trueAirspeed)
	return e.finish(rho * v * v / 2)
}

// RequiredLift returns the lift the case demands, n times weight.
func RequiredLift(fc FlightCase, mass Quantity) (Result, error) {
	e := newEvaluation(EqRequiredLift)
	m := e.quantity("mass", mass)
	n := e.loadFactor(fc)
	return e.finish(n * m * StandardGravity)
}

// RequiredLiftCoefficient returns the lift coefficient needed to hold the case
// at the given true airspeed. It does not check that coefficient against any
// CLmax: whether the aircraft can reach it is a separate, evidence-bearing
// question.
func RequiredLiftCoefficient(fc FlightCase, mass, wingArea, trueAirspeed Quantity) (Result, error) {
	e := newEvaluation(EqRequiredCL)
	m := e.quantity("mass", mass)
	s := e.quantity("wing_area", wingArea)
	rho := e.density(fc)
	v := e.quantity("true_airspeed", trueAirspeed)
	n := e.loadFactor(fc)
	q := rho * v * v / 2
	return e.finish(n * m * StandardGravity / (q * s))
}

// WingLoadingForce returns force-based wing loading, W/S.
func WingLoadingForce(mass, wingArea Quantity) (Result, error) {
	e := newEvaluation(EqWingLoadingForce)
	m := e.quantity("mass", mass)
	s := e.quantity("wing_area", wingArea)
	return e.finish(m * StandardGravity / s)
}

// WingLoadingMass returns mass-based wing loading, m/S, the form usually quoted
// for RC models. It is a convention rather than a force and carries no load
// factor.
func WingLoadingMass(mass, wingArea Quantity) (Result, error) {
	e := newEvaluation(EqWingLoadingMass)
	m := e.quantity("mass", mass)
	s := e.quantity("wing_area", wingArea)
	return e.finish(m / s)
}

// StallSpeed returns the true airspeed at which the case reaches CLmax.
func StallSpeed(fc FlightCase, mass, wingArea Quantity) (Result, error) {
	e := newEvaluation(EqStallSpeed)
	m := e.quantity("mass", mass)
	s := e.quantity("wing_area", wingArea)
	rho := e.density(fc)
	n := e.loadFactor(fc)
	cl := e.clmax(fc)
	return e.finish(math.Sqrt(2 * n * m * StandardGravity / (rho * s * cl)))
}

// MinimumWingArea returns the smallest wing area whose stall speed meets the
// supplied limit. It is a lower bound on area, not a chosen area.
func MinimumWingArea(fc FlightCase, mass, stallSpeedLimit Quantity) (Result, error) {
	e := newEvaluation(EqMinimumWingArea)
	m := e.quantity("mass", mass)
	vs := e.quantity("stall_speed_limit", stallSpeedLimit)
	rho := e.density(fc)
	n := e.loadFactor(fc)
	cl := e.clmax(fc)
	return e.finish(2 * n * m * StandardGravity / (rho * vs * vs * cl))
}

// MaximumMass returns the largest all-up mass whose stall speed meets the
// supplied limit at the given area. The ceiling is aerodynamic for the selected
// case and is not a structural rating.
func MaximumMass(fc FlightCase, wingArea, stallSpeedLimit Quantity) (Result, error) {
	e := newEvaluation(EqMaximumMass)
	s := e.quantity("wing_area", wingArea)
	vs := e.quantity("stall_speed_limit", stallSpeedLimit)
	rho := e.density(fc)
	n := e.loadFactor(fc)
	cl := e.clmax(fc)
	return e.finish(rho * vs * vs * s * cl / (2 * n * StandardGravity))
}

// MaximumWingLoadingForce returns the largest force-based wing loading whose
// stall speed meets the supplied limit.
func MaximumWingLoadingForce(fc FlightCase, stallSpeedLimit Quantity) (Result, error) {
	e := newEvaluation(EqMaximumWingLoadingForce)
	vs := e.quantity("stall_speed_limit", stallSpeedLimit)
	rho := e.density(fc)
	n := e.loadFactor(fc)
	cl := e.clmax(fc)
	return e.finish(rho * vs * vs * cl / (2 * n))
}
