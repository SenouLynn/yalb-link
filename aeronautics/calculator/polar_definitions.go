package calculator

// Drag-polar equation IDs. Like the other families these are stable: a trace
// records the ID and revision, so a stored drag or power result can be matched
// against the relationship that produced it.
const (
	EqInducedDragFactor  = "aero.induced-drag-factor"
	EqDragCoefficient    = "aero.drag-coefficient"
	EqDragForce          = "aero.drag-force"
	EqLiftToDrag         = "aero.lift-to-drag"
	EqOswaldStraightWing = "aero.oswald-straight-wing"
	EqMinimumPowerCL     = "aero.minimum-power-lift-coefficient"
)

// polarAccessedOn is the date the drag-polar, engine-and-propeller and
// mission-analysis chapters were read for Task 09. It is a literal for the same
// reason every other access date is: the core reaches no clock.
const polarAccessedOn = "2026-09-07"

// The checked chapter URLs the Task 09 families may cite.
// TestBookProvenanceIsLimitedToCheckedChapterMethods holds the list.
const (
	dragPolarURL       = "https://computationaldesignlab.github.io/aircraft-design/aerodynamics/drag_polar_induced_drag.html"
	missionAnalysisURL = "https://computationaldesignlab.github.io/aircraft-design/performance/mission_analysis.html"
)

// sourceDragPolar is the third SourceBook attribution in this package. The Drag
// Polar chapter was read on the access date and does contain the parabolic
// polar and the Oswald correlation implemented from it.
var sourceDragPolar = Source{
	Kind:     SourceBook,
	Title:    "CODE Lab Aircraft Design — Drag Polar and Induced Drag",
	URL:      dragPolarURL,
	Section:  "Drag polar",
	Accessed: polarAccessedOn,
	OriginalUnits: "dimensionless throughout; the chapter's worked example is a manned twin in " +
		"US customary units and none of its coefficients is a default here",
	Adaptation: "Ported to Go and evaluated at the design's plan-view aspect ratio. The chapter " +
		"assumes a clean cruise configuration and an aspect ratio taken from its own initial " +
		"weight estimation; here the aspect ratio comes from the solved wing and both CD0 and the " +
		"Oswald factor are supplied evidence with an aircraft-level basis. The chapter states no " +
		"validity range for the polar; this package requires one, because a parabolic polar is " +
		"symmetric in CL and returns a drag coefficient at lift coefficients the aircraft cannot " +
		"reach.",
}

// sourceMissionAnalysis covers the two relations taken from the Mission
// analysis chapter. Its own mission method is a piston fuel-fraction analysis
// and is NOT implemented; see the Adaptation note and sources.md.
var sourceMissionAnalysis = Source{
	Kind:     SourceBook,
	Title:    "CODE Lab Aircraft Design — Mission analysis",
	URL:      missionAnalysisURL,
	Section:  "Mission analysis",
	Accessed: polarAccessedOn,
	OriginalUnits: "US customary: lb, ft, slug/ft^3, knots, hp, and a brake specific fuel " +
		"consumption in lb/hr/bhp",
	Adaptation: "Only the two lift-to-drag relations are ported: (L/D) = CL/(CD0 + K CL^2) and " +
		"the maximum-endurance lift coefficient sqrt(3 CD0/K). The chapter's own mission method " +
		"is a piston-engine fuel-weight-fraction analysis — segment weight fractions, a brake " +
		"specific fuel consumption and a 6% trapped-fuel allowance — and none of it is " +
		"implemented: an electric aircraft's mass does not fall as it flies, so a weight fraction " +
		"is not a battery state of charge and reading one as the other would be wrong rather " +
		"than approximate. The electric energy budget in this package is an adaptation with its " +
		"own derivation, recorded separately.",
}

// sourceDragIdentity is the drag analogue of the lift identity the Task 02
// family cites. The primary reference assumes it rather than deriving it.
var sourceDragIdentity = Source{
	Kind:          SourceSupplementary,
	Title:         "NASA Beginner's Guide to Aeronautics — Drag equation",
	URL:           "https://www1.grc.nasa.gov/beginners-guide-to-aeronautics/drag-equation/",
	Section:       "Drag equation",
	Accessed:      polarAccessedOn,
	OriginalUnits: "SI",
	Adaptation: "Used as D = CD * q * S with q = rho * V^2 / 2 and S the plan-view reference " +
		"area, so that a drag result and the lift result it is paired with rest on the same " +
		"reference area and the same condition.",
}

var (
	portAspectRatioProjected = Port{
		Name:        "aspect_ratio_projected",
		Description: "plan-view aspect ratio b^2/S",
		Dimension:   Dimensionless,
	}
	portOswaldEfficiency = Port{
		Name:        "oswald_efficiency",
		Description: "span efficiency factor e",
		Dimension:   Dimensionless,
	}
	portInducedFactor = Port{
		Name:        "induced_drag_factor",
		Description: "K, the coefficient of CL^2 in the parabolic polar",
		Dimension:   Dimensionless,
	}
	portCD0 = Port{
		Name:        "cd0",
		Description: "whole-aircraft zero-lift drag coefficient in the polar's configuration",
		Dimension:   Dimensionless,
	}
	portLiftCoefficient = Port{
		Name:        "cl",
		Description: "whole-aircraft lift coefficient at the condition",
		Dimension:   Dimensionless,
	}
	portDragCoefficient = Port{
		Name:        "cd",
		Description: "whole-aircraft drag coefficient at the condition",
		Dimension:   Dimensionless,
	}
	portDynamicPressure = Port{
		Name:        "dynamic_pressure",
		Description: "free-stream dynamic pressure at the condition",
		Dimension:   DimForcePerArea,
	}
)

const (
	assumeAircraftPolar = "CD0 and the Oswald factor must describe the whole aircraft in the " +
		"stated configuration. A 2D section polar carries no induced, interference or trim drag " +
		"and is refused rather than reinterpreted."
	assumeParabolicPolar = "The polar is the parabolic conceptual-design form. It models neither " +
		"compressibility nor the drag rise near stall, and it is evaluated only inside the " +
		"lift-coefficient range the polar states."
	assumeCleanReference = "The polar describes one named configuration. A result carries that " +
		"configuration, and a clean polar says nothing about a deployed or damaged aircraft."
	assumeIncompressible = "Incompressible flow, appropriate at RC speeds."
	assumeOswaldEstimate = "The correlation is an unswept-wing statistical fit quoted from a " +
		"manned-aircraft design text. It has no RC validation behind it, it is offered rather " +
		"than applied, and a polar that takes it records assumed evidence."
	assumeConditionRatio = "A lift-to-drag ratio belongs to one condition. It is not the " +
		"aircraft's best L/D, which occurs at one particular lift coefficient this does not find."
	assumeMinimumPower = "The minimum-power lift coefficient is a condition, not a " +
		"recommendation. Whether the aircraft can be trimmed to hold it, and with what stall " +
		"margin, is a separate question this package does not answer."
)

// polarEquations holds the Task 09 drag relations. They are kept in their own
// set so this family's provenance stays separately inspectable.
var polarEquations = map[string]Equation{
	EqInducedDragFactor: {
		ID:          EqInducedDragFactor,
		Revision:    "1",
		Expression:  "K = 1/(pi * A * e)",
		Inputs:      []Port{portAspectRatioProjected, portOswaldEfficiency},
		Output:      portInducedFactor,
		Source:      sourceDragPolar,
		Assumptions: []string{assumeAircraftPolar, assumeParabolicPolar},
	},
	EqDragCoefficient: {
		ID:          EqDragCoefficient,
		Revision:    "1",
		Expression:  "CD = CD0 + K * CL^2",
		Inputs:      []Port{portCD0, portInducedFactor, portLiftCoefficient},
		Output:      portDragCoefficient,
		Source:      sourceDragPolar,
		Assumptions: []string{assumeAircraftPolar, assumeParabolicPolar, assumeCleanReference},
	},
	EqDragForce: {
		ID:         EqDragForce,
		Revision:   "1",
		Expression: "D = q * S * CD",
		Inputs:     []Port{portDynamicPressure, portArea, portDragCoefficient},
		Output: Port{
			Name:        "drag",
			Description: "drag force at the condition",
			Dimension:   DimForce,
		},
		Source:      sourceDragIdentity,
		Assumptions: []string{assumeTrueAirspeed, assumeIncompressible, assumeCleanReference},
	},
	EqLiftToDrag: {
		ID:          EqLiftToDrag,
		Revision:    "1",
		Expression:  "L/D = CL / CD",
		Inputs:      []Port{portLiftCoefficient, portDragCoefficient},
		Output:      Port{Name: "lift_to_drag", Description: "lift-to-drag ratio at the condition", Dimension: Dimensionless},
		Source:      sourceMissionAnalysis,
		Assumptions: []string{assumeConditionRatio, assumeParabolicPolar},
	},
	EqOswaldStraightWing: {
		ID:          EqOswaldStraightWing,
		Revision:    "1",
		Expression:  "e = 1.78 * (1 - 0.045 * A^0.68) - 0.64",
		Inputs:      []Port{portAspectRatioProjected},
		Output:      portOswaldEfficiency,
		Source:      sourceDragPolar,
		Assumptions: []string{assumeOswaldEstimate},
	},
	EqMinimumPowerCL: {
		ID:          EqMinimumPowerCL,
		Revision:    "1",
		Expression:  "CL_minimum_power = sqrt(3 * CD0 / K)",
		Inputs:      []Port{portCD0, portInducedFactor},
		Output:      portLiftCoefficient,
		Source:      sourceMissionAnalysis,
		Assumptions: []string{assumeMinimumPower, assumeParabolicPolar, assumeAircraftPolar},
	},
}
