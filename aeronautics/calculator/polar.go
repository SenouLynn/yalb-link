package calculator

import "math"

// DragPolar is a preliminary parabolic drag polar and the evidence behind it:
// CD = CD0 + CL^2/(pi*A*e), evaluated at the design's plan-view aspect ratio.
//
// Both coefficients are supplied evidence. Nothing in this package estimates a
// zero-lift drag coefficient: that needs a component buildup or a wetted-area
// method neither the primary reference's drag-polar chapter nor this package
// implements, and inventing one would put a number with nothing behind it in
// front of every power result. The Oswald factor has an offered estimate, but
// it is an explicit action a builder takes, never a default.
//
// The validity range is required rather than optional. A parabolic polar is
// symmetric in CL and has no notion of stall, so on its own it will happily
// report a drag coefficient at a lift coefficient the aircraft cannot reach. The
// range is what stops that, and an unstated one is a missing field.
type DragPolar struct {
	// CD0Basis states where CD0 came from, for example "component buildup at
	// Re = 2e5" or "measured in a glide test". It must describe the whole
	// aircraft in the polar's configuration.
	CD0Basis string
	// EfficiencyBasis states where the Oswald factor came from.
	EfficiencyBasis string
	// Configuration names the airframe configuration the polar describes, in the
	// same vocabulary a flight case uses.
	Configuration string
	// CD0 is the zero-lift drag coefficient.
	CD0 float64
	// OswaldEfficiency is the span efficiency factor e.
	OswaldEfficiency float64
	// CLValidMin and CLValidMax bound the lift coefficients this polar is
	// claimed to describe. Outside them a drag result is refused rather than
	// extrapolated. A pair that is not stated is (0, 0), which fails the
	// max-above-min check and is reported as missing.
	CLValidMin float64
	// CLValidMax is the upper end of that range.
	CLValidMax float64
	// Scope records whether the coefficients describe the aircraft or only a
	// section. A 2D section polar is refused where an aircraft polar is
	// required, exactly as a section clmax is.
	Scope CoefficientScope
	// Evidence grades the provenance of the coefficients.
	Evidence EvidenceQuality
}

// supplied reports whether a polar has been entered at all.
func (p DragPolar) supplied() bool { return p != DragPolar{} }

// validate reports every structural problem with the polar. It is called from
// Design.Validate, so an incomplete polar is a design in progress rather than a
// refused edit.
func (p DragPolar) validate(rs *resultSet) {
	if !p.supplied() {
		return
	}
	switch p.Scope {
	case ScopeAircraft:
	case ScopeAirfoilSection:
		rs.add("polar", IssueUnsupported,
			"a 2D airfoil section polar is not an aircraft drag polar: it carries no induced drag, "+
				"no interference and no trim drag. Supply aircraft-level coefficients")
	default:
		rs.add("polar", IssueMissing,
			"set the polar scope so section data cannot be read as whole-aircraft data")
	}
	// A missing basis is reported against the basis field rather than against
	// the number, so the complaint lands on the box that is actually empty.
	if p.CD0Basis == "" {
		rs.add("polar.cd0_basis", IssueMissing,
			"state where CD0 came from; nothing here estimates it, so the result is only as good "+
				"as the evidence behind it")
	}
	if p.EfficiencyBasis == "" {
		rs.add("polar.efficiency_basis", IssueMissing,
			"state where the Oswald efficiency factor came from")
	}
	if p.Configuration == "" {
		rs.add("polar", IssueMissing,
			"name the configuration the polar describes; a clean polar is not a landing polar")
	}
	if p.CD0 <= 0 || !isFinite(p.CD0) {
		rs.add("polar.cd0", IssueInvalid,
			"CD0 must be greater than zero, received "+formatFloat(p.CD0))
	}
	if p.OswaldEfficiency <= 0 || p.OswaldEfficiency > 1 || !isFinite(p.OswaldEfficiency) {
		rs.add("polar.oswald_efficiency", IssueInvalid,
			"the Oswald efficiency factor is a fraction greater than zero and at most one, "+
				"received "+formatFloat(p.OswaldEfficiency))
	}
	p.validateRange(rs)
}

func (p DragPolar) validateRange(rs *resultSet) {
	if !isFinite(p.CLValidMin) || !isFinite(p.CLValidMax) {
		rs.add("polar.validity", IssueInvalid, "the validity range must be finite")
		return
	}
	if p.CLValidMax <= p.CLValidMin {
		rs.add("polar.validity", IssueMissing,
			"state the lift-coefficient range this polar is claimed over, with the negative end "+
				"spelled out. A parabolic polar is symmetric in CL and knows nothing about stall, "+
				"so without a range it reports a drag coefficient at a CL the aircraft cannot reach")
	}
}

// covers reports whether the polar is claimed at this lift coefficient.
func (p DragPolar) covers(cl float64) bool {
	return cl >= p.CLValidMin && cl <= p.CLValidMax
}

// InducedDragFactor returns K = 1/(pi*A*e), the coefficient of CL^2 in the
// parabolic polar. The aspect ratio is the plan-view one: the induced-drag
// relation is about the lifting surface's projection, not about the panel's
// construction length.
func InducedDragFactor(aspectRatio float64, polar DragPolar) (Result, error) {
	e := newEvaluation(EqInducedDragFactor)
	a := e.scalar(portAspectRatioProjected.Name, aspectRatio)
	oswald := e.scalar(portOswaldEfficiency.Name, polar.OswaldEfficiency)
	if a == 0 || oswald == 0 {
		return Result{}, e.issues
	}
	return e.finish(1 / (math.Pi * a * oswald))
}

// DragCoefficient returns CD = CD0 + K*CL^2 at the given lift coefficient. It
// refuses a lift coefficient outside the polar's stated validity range rather
// than extrapolating the parabola into flow the polar does not describe.
func DragCoefficient(aspectRatio float64, polar DragPolar, cl float64) (Result, error) {
	factor, err := InducedDragFactor(aspectRatio, polar)
	if err != nil {
		return Result{}, err
	}
	e := newEvaluation(EqDragCoefficient)
	cd0 := e.scalar(portCD0.Name, polar.CD0)
	k := e.scalar(portInducedFactor.Name, factor.Value.si)
	lift := e.scalarSigned(portLiftCoefficient.Name, cl)
	if !polar.covers(cl) {
		e.add(portLiftCoefficient.Name, IssueUnsupported,
			"CL "+formatFloat(cl)+" is outside the range this polar is claimed over, "+
				formatFloat(polar.CLValidMin)+" to "+formatFloat(polar.CLValidMax)+
				"; the parabolic form would still return a number there, and it would not "+
				"describe the aircraft")
	}
	return e.finish(cd0 + k*lift*lift)
}

// DragForce returns D = q*S*CD. The dynamic pressure is the caller's, so that a
// drag result and the lift result it is paired with cannot rest on two
// different conditions.
func DragForce(dynamicPressure, wingArea Quantity, cd float64) (Result, error) {
	e := newEvaluation(EqDragForce)
	q := e.quantity(portDynamicPressure.Name, dynamicPressure)
	s := e.quantity(portArea.Name, wingArea)
	c := e.scalar(portDragCoefficient.Name, cd)
	return e.finish(q * s * c)
}

// LiftToDrag returns CL/CD at one condition. It is a ratio at that condition
// and not a maximum: the aircraft's best L/D is at one particular lift
// coefficient, which this does not find.
func LiftToDrag(cl, cd float64) (Result, error) {
	e := newEvaluation(EqLiftToDrag)
	lift := e.scalar(portLiftCoefficient.Name, cl)
	drag := e.scalar(portDragCoefficient.Name, cd)
	if drag == 0 {
		return Result{}, e.issues
	}
	return e.finish(lift / drag)
}

// OswaldEfficiencyStraightWing returns the primary reference's straight-wing
// estimate of the Oswald factor, e = 1.78(1 - 0.045 A^0.68) - 0.64.
//
// It is offered, never applied. A builder who takes this number has chosen an
// estimate whose evidence grade is "assumed", and the polar records that; the
// relation is an unswept-wing correlation from a manned-aircraft design text
// and has no RC validation behind it.
func OswaldEfficiencyStraightWing(aspectRatio float64) (Result, error) {
	e := newEvaluation(EqOswaldStraightWing)
	a := e.scalar(portAspectRatioProjected.Name, aspectRatio)
	// The correlation is quoted without a stated validity range. Outside these
	// aspect ratios it returns values a span efficiency cannot take — above one
	// below A of about 1.6, and below zero above about A of 26 — so it is
	// refused there rather than reported.
	e.requireWithin(portAspectRatioProjected.Name, a, oswaldEstimateMinAspect, oswaldEstimateMaxAspect,
		"", "the straight-wing Oswald correlation is only evaluated between aspect ratios "+
			formatFloat(oswaldEstimateMinAspect)+" and "+formatFloat(oswaldEstimateMaxAspect)+
			", outside which it returns a span efficiency above one or below zero")
	return e.finish(1.78*(1-0.045*math.Pow(a, 0.68)) - 0.64)
}

// The aspect-ratio range the straight-wing Oswald correlation is evaluated
// over. The relation is quoted with no stated range; these are the values
// between which it returns a span efficiency in (0, 1], which is the widest
// interval over which its output can mean anything at all.
const (
	oswaldEstimateMinAspect = 2.0
	oswaldEstimateMaxAspect = 20.0
)

// MinimumPowerLiftCoefficient returns CL = sqrt(3*CD0/K), the lift coefficient
// at which the power required for steady level flight is least under a
// parabolic polar. It is the primary reference's maximum-endurance condition.
//
// It is a condition, not a recommendation. Flying there needs the aircraft to
// hold that lift coefficient, which is a stall-margin and trim question this
// package does not answer, and the result is refused when it falls outside the
// polar's own validity range.
func MinimumPowerLiftCoefficient(aspectRatio float64, polar DragPolar) (Result, error) {
	factor, err := InducedDragFactor(aspectRatio, polar)
	if err != nil {
		return Result{}, err
	}
	e := newEvaluation(EqMinimumPowerCL)
	cd0 := e.scalar(portCD0.Name, polar.CD0)
	k := e.scalar(portInducedFactor.Name, factor.Value.si)
	if k == 0 {
		return Result{}, e.issues
	}
	cl := math.Sqrt(3 * cd0 / k)
	if !polar.covers(cl) {
		e.add(portLiftCoefficient.Name, IssueUnsupported,
			"the minimum-power lift coefficient "+formatFloat(cl)+" falls outside the range this "+
				"polar is claimed over, "+formatFloat(polar.CLValidMin)+" to "+
				formatFloat(polar.CLValidMax))
	}
	return e.finish(cl)
}
