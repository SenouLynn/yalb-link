package calculator

// Equation IDs. These are stable identifiers: a trace records the ID and
// revision so a stored result can be matched against the model that produced it.
const (
	EqWeightFromMass          = "weight.from-mass"
	EqDynamicPressure         = "aero.dynamic-pressure"
	EqRequiredLift            = "lift.required"
	EqRequiredCL              = "lift.required-coefficient"
	EqWingLoadingForce        = "wing-loading.force"
	EqWingLoadingMass         = "wing-loading.mass"
	EqStallSpeed              = "lift.stall-speed"
	EqMinimumWingArea         = "lift.minimum-wing-area"
	EqMaximumMass             = "lift.maximum-mass"
	EqMaximumWingLoadingForce = "lift.maximum-wing-loading"
)

const accessedOn = "2026-09-05"

// The stall-speed family below is NOT attributed to the book's Lift chapter.
// That chapter was checked on the access date and contains the lift-curve slope
// and a CLmax estimation method, but no stall-speed equation and no inversion of
// the lift identity. Attributing these to it would overstate the provenance, so
// they cite the standard identity and their own derivation instead. The book's
// role here is applicability: it is what establishes that a section c_l_max is
// not an aircraft C_L_max. See docs/reference/sources.md for the coverage record.
var sourceLiftIdentity = Source{
	Kind:          SourceSupplementary,
	Title:         "NASA Beginner's Guide to Aeronautics — Lift equation",
	URL:           "https://www1.grc.nasa.gov/beginners-guide-to-aeronautics/lift-equation/",
	Section:       "Lift equation",
	Accessed:      accessedOn,
	OriginalUnits: "SI",
	Adaptation: "Used as the identity L = CL * q * S with q = rho * V^2 / 2, together with " +
		"W = m * g. The CODE Lab book assumes this identity rather than deriving it.",
}

func derivedFrom(base Source, note string) Source {
	s := base
	s.Kind = SourceDerived
	s.Adaptation = note + " " + base.Adaptation
	return s
}

var (
	portMass       = Port{Name: "mass", Description: "all-up mass", Dimension: DimMass}
	portArea       = Port{Name: "wing_area", Description: "reference wing area", Dimension: DimArea}
	portDensity    = Port{Name: "density", Description: "local air density", Dimension: DimDensity}
	portSpeed      = Port{Name: "true_airspeed", Description: "true airspeed", Dimension: DimSpeed}
	portStallLimit = Port{
		Name:        "stall_speed_limit",
		Description: "the highest acceptable stall speed",
		Dimension:   DimSpeed,
	}
	portLoadFactor = Port{Name: "load_factor", Description: "load factor n", Dimension: Dimensionless}
	portCLmax      = Port{Name: "clmax", Description: "whole-aircraft maximum lift coefficient", Dimension: Dimensionless}
	portWeight     = Port{Name: "weight", Description: "weight", Dimension: DimForce}
)

const (
	assumeStandardGravity = "Weight uses the standard gravity constant 9.80665 m/s^2, not local gravity."
	assumeTrueAirspeed    = "Speeds are true airspeed at the case's density, not indicated airspeed."
	assumeLumpedLift      = "Lumped lift model: the wing carries the whole load, with no separately " +
		"solved wing and tail trim loads."
	assumeAeroCeiling = "The mass ceiling is aerodynamic for the selected case; it is not a " +
		"structural rating."
	assumeAreaBound     = "A stall ceiling gives a lower bound on area; it does not choose the actual area."
	assumeAircraftCLmax = "CLmax must be a whole-aircraft coefficient for the case's configuration. " +
		"The CODE Lab Lift chapter distinguishes section c_l_max from aircraft C_L_max and gives an " +
		"estimation method for the latter; that estimation is not implemented here, so the aircraft " +
		"value and its evidence are supplied by the caller."
)

// registry is every equation the package can evaluate, assembled from the
// per-subject sets. A duplicate ID between sets would shadow one definition
// silently; TestRegistryDefinesEveryEvaluatedEquation compares the registry
// count against the evaluator list, which is what catches that.
var registry = mergeEquations(liftEquations, geometryEquations, massEquations)

func mergeEquations(sets ...map[string]Equation) map[string]Equation {
	merged := make(map[string]Equation)
	for _, set := range sets {
		for id := range set {
			merged[id] = set[id]
		}
	}
	return merged
}

// liftEquations holds the Task 02 lift identity, its inversions and the wing
// loading conventions.
var liftEquations = map[string]Equation{
	EqWeightFromMass: {
		ID:          EqWeightFromMass,
		Revision:    "1",
		Expression:  "W = m * g",
		Inputs:      []Port{portMass},
		Output:      portWeight,
		Source:      sourceLiftIdentity,
		Assumptions: []string{assumeStandardGravity},
	},
	EqDynamicPressure: {
		ID:         EqDynamicPressure,
		Revision:   "1",
		Expression: "q = rho * V^2 / 2",
		Inputs:     []Port{portDensity, portSpeed},
		Output: Port{
			Name:        "dynamic_pressure",
			Description: "free-stream dynamic pressure",
			Dimension:   DimForcePerArea,
		},
		Source:      sourceLiftIdentity,
		Assumptions: []string{assumeTrueAirspeed, "Incompressible flow, appropriate at RC speeds."},
	},
	EqRequiredLift: {
		ID:          EqRequiredLift,
		Revision:    "1",
		Expression:  "L_required = n * W = n * m * g",
		Inputs:      []Port{portMass, portLoadFactor},
		Output:      Port{Name: "required_lift", Description: "lift required for the case", Dimension: DimForce},
		Source:      sourceLiftIdentity,
		Assumptions: []string{assumeStandardGravity, assumeLumpedLift},
	},
	EqRequiredCL: {
		ID:         EqRequiredCL,
		Revision:   "1",
		Expression: "CL_required = n * m * g / ((rho * V^2 / 2) * S)",
		Inputs:     []Port{portMass, portArea, portDensity, portSpeed, portLoadFactor},
		Output: Port{
			Name:        "required_cl",
			Description: "lift coefficient required to hold the case",
			Dimension:   Dimensionless,
		},
		Source:      sourceLiftIdentity,
		Assumptions: []string{assumeStandardGravity, assumeTrueAirspeed, assumeLumpedLift},
	},
	EqWingLoadingForce: {
		ID:         EqWingLoadingForce,
		Revision:   "1",
		Expression: "W/S = m * g / S",
		Inputs:     []Port{portMass, portArea},
		Output: Port{
			Name:        "wing_loading_force",
			Description: "force-based wing loading",
			Dimension:   DimForcePerArea,
		},
		Source:      sourceLiftIdentity,
		Assumptions: []string{assumeStandardGravity},
	},
	EqWingLoadingMass: {
		ID:         EqWingLoadingMass,
		Revision:   "1",
		Expression: "m/S",
		Inputs:     []Port{portMass, portArea},
		Output: Port{
			Name:        "wing_loading_mass",
			Description: "mass-based wing loading, as commonly quoted for RC models",
			Dimension:   DimMassPerArea,
		},
		Source: sourceLiftIdentity,
		Assumptions: []string{
			"Mass-based loading is a convention, not a force; it does not include load factor.",
		},
	},
	EqStallSpeed: {
		ID:         EqStallSpeed,
		Revision:   "1",
		Expression: "Vs = sqrt(2 * n * m * g / (rho * S * CLmax))",
		Inputs:     []Port{portMass, portArea, portDensity, portLoadFactor, portCLmax},
		Output: Port{
			Name:        "stall_speed",
			Description: "true airspeed at CLmax for the case",
			Dimension:   DimSpeed,
		},
		Source: sourceLiftIdentity,
		Assumptions: []string{
			assumeStandardGravity, assumeTrueAirspeed, assumeLumpedLift,
			assumeAircraftCLmax,
		},
	},
	EqMinimumWingArea: {
		ID:         EqMinimumWingArea,
		Revision:   "1",
		Expression: "S_min = 2 * n * m * g / (rho * Vs_limit^2 * CLmax)",
		Inputs:     []Port{portMass, portStallLimit, portDensity, portLoadFactor, portCLmax},
		Output: Port{
			Name:        "minimum_wing_area",
			Description: "smallest wing area meeting the stall-speed limit",
			Dimension:   DimArea,
		},
		Source: derivedFrom(sourceLiftIdentity, "Algebraic inversion of the stall-speed relation for area."),
		Assumptions: []string{
			assumeStandardGravity, assumeTrueAirspeed, assumeLumpedLift, assumeAreaBound,
			assumeAircraftCLmax,
		},
	},
	EqMaximumMass: {
		ID:         EqMaximumMass,
		Revision:   "1",
		Expression: "m_max = rho * Vs_limit^2 * S * CLmax / (2 * n * g)",
		Inputs:     []Port{portArea, portStallLimit, portDensity, portLoadFactor, portCLmax},
		Output: Port{
			Name:        "maximum_mass",
			Description: "largest all-up mass meeting the stall-speed limit",
			Dimension:   DimMass,
		},
		Source: derivedFrom(sourceLiftIdentity, "Algebraic inversion of the stall-speed relation for mass."),
		Assumptions: []string{
			assumeStandardGravity, assumeTrueAirspeed, assumeLumpedLift, assumeAeroCeiling,
			assumeAircraftCLmax,
		},
	},
	EqMaximumWingLoadingForce: {
		ID:         EqMaximumWingLoadingForce,
		Revision:   "1",
		Expression: "(W/S)_max = rho * Vs_limit^2 * CLmax / (2 * n)",
		Inputs:     []Port{portStallLimit, portDensity, portLoadFactor, portCLmax},
		Output: Port{
			Name:        "maximum_wing_loading_force",
			Description: "largest force-based wing loading meeting the stall-speed limit",
			Dimension:   DimForcePerArea,
		},
		Source: derivedFrom(sourceLiftIdentity,
			"Algebraic inversion of the stall-speed relation for wing loading."),
		Assumptions: []string{assumeTrueAirspeed, assumeLumpedLift, assumeAircraftCLmax},
	},
}
