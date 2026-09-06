package calculator

// CoefficientScope records what a maximum lift coefficient actually describes.
// It exists so that a section value cannot quietly become a whole-aircraft
// value: a 2D airfoil clmax ignores finite-span, trim and interference effects
// and overstates what an aircraft reaches.
type CoefficientScope uint8

const (
	// ScopeUnknown is the zero value. It is never accepted for a lift result.
	ScopeUnknown CoefficientScope = iota
	// ScopeAircraft is a whole-aircraft CLmax in the stated configuration.
	ScopeAircraft
	// ScopeAirfoilSection is 2D section data, such as a NACA profile's clmax.
	ScopeAirfoilSection
)

var coefficientScopeNames = [...]string{
	ScopeUnknown:        "unknown",
	ScopeAircraft:       "whole aircraft",
	ScopeAirfoilSection: "2D airfoil section",
}

// String returns the scope's readable name.
func (s CoefficientScope) String() string {
	if int(s) < len(coefficientScopeNames) {
		return coefficientScopeNames[s]
	}
	return "unknown"
}

// LiftCoefficient is a maximum lift coefficient with the evidence behind it.
// Basis is required by every calculation that uses Max, so an assumed value is
// always visible in the result rather than hidden in a default.
type LiftCoefficient struct {
	// Basis states where the value came from, for example "assumed for initial
	// sizing" or "wind tunnel, clean configuration".
	Basis string
	// Max is the maximum lift coefficient itself.
	Max float64
	// Scope records whether Max describes the aircraft or only a section.
	Scope CoefficientScope
}

// FlightCase is the condition an aerodynamic result is reported against. No
// result from this package is meaningful without one: correct arithmetic at an
// unstated density, load factor or configuration is not a validated result.
type FlightCase struct {
	// Name identifies the case, such as "stall, clean, sea level".
	Name string
	// Configuration names the airframe configuration the coefficients apply to.
	Configuration string
	// DensityBasis states where Density came from, for example "ISA sea level".
	DensityBasis string
	// ViscosityBasis states where Viscosity came from. It is required by every
	// calculation that consumes Viscosity, and by no other.
	ViscosityBasis string
	// CLmax is the maximum lift coefficient and its provenance.
	CLmax LiftCoefficient
	// Density is the local air density for the case.
	Density Quantity
	// Viscosity is the dynamic viscosity for the case. It is only consumed by
	// Reynolds numbers, so a case that never computes one may leave it unset.
	Viscosity Quantity
	// LoadFactor is the load factor n, which must be positive. Level flight is 1.
	LoadFactor float64
}

// SeaLevelISADensity is the ISA sea-level air density, 1.225 kg/m³. It is a
// standard-atmosphere reference value, not a measurement of any given day.
func SeaLevelISADensity() Quantity {
	return Quantity{si: 1.225, dim: DimDensity}
}

// SeaLevelISAViscosity is the ISA sea-level dynamic viscosity of air,
// 1.7894e-5 Pa*s. It is the standard-atmosphere value, consistent with
// Sutherland's relation at 288.15 K (1.78919e-5 Pa*s), not a measurement of any
// given day.
func SeaLevelISAViscosity() Quantity {
	return Quantity{si: 1.7894e-5, dim: DimDynamicViscosity}
}

// density validates and records the case's air density.
func (e *evaluation) density(fc FlightCase) float64 {
	if fc.DensityBasis == "" {
		e.add("density", IssueMissing,
			"state the air density basis so the assumption is visible alongside the result")
	}
	return e.quantity("density", fc.Density)
}

// viscosity validates and records the case's dynamic viscosity.
func (e *evaluation) viscosity(fc FlightCase) float64 {
	if fc.ViscosityBasis == "" {
		e.add(portViscosity.Name, IssueMissing,
			"state the viscosity basis so the assumption is visible alongside the Reynolds number")
	}
	return e.quantity(portViscosity.Name, fc.Viscosity)
}

// loadFactor validates and records the case's load factor.
func (e *evaluation) loadFactor(fc FlightCase) float64 {
	return e.scalar("load_factor", fc.LoadFactor)
}

// clmax validates and records the case's maximum lift coefficient, rejecting a
// section value offered where a whole-aircraft value is required.
func (e *evaluation) clmax(fc FlightCase) float64 {
	switch fc.CLmax.Scope {
	case ScopeAircraft:
	case ScopeAirfoilSection:
		e.add("clmax", IssueUnsupported,
			"a 2D airfoil section clmax is not a whole-aircraft CLmax; supply an aircraft value "+
				"or an implemented method that derives one")
		return 0
	default:
		e.add("clmax", IssueMissing,
			"set the CLmax scope so a section value cannot be read as a whole-aircraft value")
		return 0
	}
	if fc.CLmax.Basis == "" {
		e.add("clmax", IssueMissing,
			"state the CLmax basis so an assumed coefficient is visible alongside the result")
	}
	return e.scalar("clmax", fc.CLmax.Max)
}
