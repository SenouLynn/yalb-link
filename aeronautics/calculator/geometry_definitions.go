package calculator

// Geometry equation IDs. Like the lift IDs these are stable: a trace records
// the ID and revision, so a stored planform can be matched to the relationships
// that produced it.
const (
	EqAspectRatio                = "geometry.aspect-ratio"
	EqSpanFromAreaAndAspect      = "geometry.span-from-area-aspect"
	EqAreaFromSpanAndAspect      = "geometry.area-from-span-aspect"
	EqSpanFromAreaAndRootChord   = "geometry.span-from-area-root-chord"
	EqAreaFromSpanAndRootChord   = "geometry.area-from-span-root-chord"
	EqSpanFromAspectAndRootChord = "geometry.span-from-aspect-root-chord"
	EqRootChord                  = "geometry.root-chord"
	EqTipChord                   = "geometry.tip-chord"
	EqMeanChord                  = "geometry.mean-chord"
	EqMeanAerodynamicChord       = "geometry.mac"
	EqMACStation                 = "geometry.y-mac"
	EqChordAtStation             = "geometry.chord-at-station"
	EqSweepTransform             = "geometry.sweep-transform"
	EqTipLeadingEdgeOffset       = "geometry.tip-leading-edge-offset"
	EqTipRise                    = "geometry.tip-rise"
	EqMACLeadingEdgeStation      = "geometry.mac-leading-edge-station"
	EqExposedArea                = "geometry.exposed-area"
	EqProjectedSpan              = "geometry.projected-span"
	EqProjectedArea              = "geometry.projected-area"
	EqPanelSpan                  = "geometry.panel-span"
	EqPanelArea                  = "geometry.panel-area"
	EqSemiSpan                   = "geometry.semi-span"
	EqReynolds                   = "aero.reynolds"
)

// sourceWingLayout is the first SourceBook attribution in this package. The
// chapter was read on the access date and does contain these planform
// relations, unlike the Lift chapter, which contains no stall-speed equation.
// TestBookProvenanceIsLimitedToCheckedChapters records which equations may
// carry it.
var sourceWingLayout = Source{
	Kind:     SourceBook,
	Title:    "CODE Lab Aircraft Design — Wing Planform Sizing",
	URL:      "https://computationaldesignlab.github.io/aircraft-design/wing_layout.html",
	Section:  "Wing Planform Sizing",
	Accessed: accessedOn,
	OriginalUnits: "US customary in the worked example (ft, ft^2, degrees); the relations " +
		"themselves are unit-agnostic and are evaluated here in SI",
	Adaptation: "Ported to Go and evaluated in SI. The chapter's example values (A=8, S=134 ft^2, " +
		"lambda=0.4, 5 deg dihedral, -3 deg twist, 2 deg incidence, NACA 23018/23009) are example " +
		"inputs for a manned light aircraft, not RC defaults, and none of them is a default here. " +
		"Its Torenbeek wing fuel-volume relation is deliberately not implemented: an electric RC " +
		"wing carries no fuel.",
}

// sourceSimilarity carries the Reynolds definition. The NASA page gives
// Re = rho V L / mu with L only as "some characteristic length"; choosing the
// local chord, and covering root and tip rather than the MAC alone, is this
// project's adaptation and is recorded as such.
var sourceSimilarity = Source{
	Kind:          SourceSupplementary,
	Title:         "NASA Beginner's Guide to Aeronautics — Similarity parameters",
	URL:           "https://www1.grc.nasa.gov/beginners-guide-to-aeronautics/similarity-parameters/",
	Section:       "Reynolds number",
	Accessed:      accessedOn,
	OriginalUnits: "SI",
	Adaptation: "Used as Re = rho V c / mu with the local chord as the characteristic length. The " +
		"page writes mu as a \"viscosity coefficient\" without naming it; the dimensional form " +
		"requires dynamic viscosity, which is what this package's DimDynamicViscosity holds. The " +
		"CODE Lab Wing Planform Sizing chapter says airfoil analysis should use the mean " +
		"aerodynamic chord; reporting root and tip stations as well is an explicit extension, " +
		"because an RC tip chord can sit in a materially lower Reynolds regime than its MAC.",
}

var (
	portSpan        = Port{Name: "span", Description: "projected wing span b, tip to tip", Dimension: DimLength}
	portAspectRatio = Port{
		Name:        "aspect_ratio",
		Description: "wing aspect ratio A = b^2/S",
		Dimension:   Dimensionless,
	}
	portTaperRatio = Port{
		Name:        "taper_ratio",
		Description: "taper ratio lambda = c_tip/c_root",
		Dimension:   Dimensionless,
	}
	portRootChord = Port{
		Name:        "root_chord",
		Description: "chord of the reference trapezoid at the centerline",
		Dimension:   DimLength,
	}
	portStation = Port{
		Name:        "station",
		Description: "spanwise distance from the centerline, plan view",
		Dimension:   DimLength,
	}
	portBodyWidth = Port{
		Name:        "body_width",
		Description: "width of the body the wing passes through, measured at the wing",
		Dimension:   DimLength,
	}
	portDihedral = Port{
		Name:        "dihedral",
		Description: "uniform dihedral angle Gamma, positive tips up",
		Dimension:   DimAngle,
	}
	portPanelSpan = Port{
		Name:        "panel_span",
		Description: "span measured in the panel plane, along the dihedral line",
		Dimension:   DimLength,
	}
	portPanelArea = Port{
		Name:        "panel_area",
		Description: "area measured in the panel plane",
		Dimension:   DimArea,
	}
	portReferenceSweep = Port{
		Name:        "reference_sweep",
		Description: "sweep angle at the reference chord fraction, positive aft",
		Dimension:   DimAngle,
	}
	portReferenceFraction = Port{
		Name:        "reference_fraction",
		Description: "chord fraction the reference sweep is measured at, 0 at the leading edge",
		Dimension:   Dimensionless,
	}
	portTargetFraction = Port{
		Name:        "target_fraction",
		Description: "chord fraction the sweep is wanted at",
		Dimension:   Dimensionless,
	}
	portLeadingEdgeSweep = Port{
		Name:        "leading_edge_sweep",
		Description: "plan-view sweep of the leading edge, positive aft",
		Dimension:   DimAngle,
	}
	portChord = Port{
		Name:        "chord",
		Description: "chord at the station the result is reported for",
		Dimension:   DimLength,
	}
	portViscosity = Port{
		Name:        "viscosity",
		Description: "dynamic viscosity mu of the case",
		Dimension:   DimDynamicViscosity,
	}
)

const (
	assumeReferenceTrapezoid = "The reference planform is the straight-tapered trapezoid carried " +
		"through the centerline, so the reference area includes the part inside any body."
	assumeLinearTaper = "Chord varies linearly from root to tip: one straight-tapered panel per " +
		"side, with no kink, crank or curved edge."
	assumeSymmetricPanels = "The left and right panels are mirror images."
	assumePlanView        = "Dimensions are plan-view projections unless the port name says panel."
	assumeStraightEdges   = "Leading and trailing edges are straight across the whole semi-span, " +
		"which is what makes one sweep angle describe the panel."
	assumeGeometryNotHandling = "Geometry is not handling: no stability, trim or control conclusion " +
		"follows from these dimensions."
	assumeUniformDihedral = "Dihedral is uniform along the panel and is applied about the root chord " +
		"line, so the plan view scales by cos(Gamma). Dihedral is not modelled as a lift penalty or " +
		"as a self-levelling guarantee."
	assumeLocalChordReynolds = "Reynolds number uses the case's free-stream density, true airspeed " +
		"and dynamic viscosity with the local chord; it describes one station, not the whole wing."
)

// geometryEquations holds the Task 03 planform, projection and Reynolds
// relationships. They are kept in their own set so that the lift subset's
// provenance rules stay separately inspectable.
var geometryEquations = map[string]Equation{
	EqAspectRatio: {
		ID:         EqAspectRatio,
		Revision:   "1",
		Expression: "A = b^2 / S",
		Inputs:     []Port{portSpan, portArea},
		Output:     portAspectRatio,
		Source:     sourceWingLayout,
		Assumptions: []string{
			assumeReferenceTrapezoid, assumePlanView, assumeGeometryNotHandling,
		},
	},
	EqSpanFromAreaAndAspect: {
		ID:         EqSpanFromAreaAndAspect,
		Revision:   "1",
		Expression: "b = sqrt(A * S)",
		Inputs:     []Port{portArea, portAspectRatio},
		Output:     portSpan,
		Source:     sourceWingLayout,
		Assumptions: []string{
			assumeReferenceTrapezoid, assumePlanView, assumeGeometryNotHandling,
		},
	},
	EqAreaFromSpanAndAspect: {
		ID:         EqAreaFromSpanAndAspect,
		Revision:   "1",
		Expression: "S = b^2 / A",
		Inputs:     []Port{portSpan, portAspectRatio},
		Output:     portArea,
		Source: derivedFrom(sourceWingLayout,
			"Rearrangement of the chapter's aspect-ratio definition for area."),
		Assumptions: []string{
			assumeReferenceTrapezoid, assumePlanView, assumeGeometryNotHandling,
		},
	},
	EqRootChord: {
		ID:         EqRootChord,
		Revision:   "1",
		Expression: "c_root = 2 * S / (b * (1 + lambda))",
		Inputs:     []Port{portArea, portSpan, portTaperRatio},
		Output:     portRootChord,
		Source:     sourceWingLayout,
		Assumptions: []string{
			assumeReferenceTrapezoid, assumeLinearTaper, assumeSymmetricPanels, assumePlanView,
		},
	},
	EqTipChord: {
		ID:         EqTipChord,
		Revision:   "1",
		Expression: "c_tip = lambda * c_root",
		Inputs:     []Port{portRootChord, portTaperRatio},
		Output:     Port{Name: "tip_chord", Description: "chord at the tip", Dimension: DimLength},
		Source:     sourceWingLayout,
		Assumptions: []string{
			assumeLinearTaper, assumePlanView,
			"Reverse taper (lambda > 1) is a valid planform, not an error.",
		},
	},
	EqMeanChord: {
		ID:         EqMeanChord,
		Revision:   "1",
		Expression: "c_mean = S / b",
		Inputs:     []Port{portArea, portSpan},
		Output: Port{
			Name:        "mean_chord",
			Description: "geometric mean chord S/b, which is not the MAC under taper",
			Dimension:   DimLength,
		},
		Source: derivedFrom(sourceWingLayout,
			"Consequence of the reference area and span definitions. The chapter reports MAC "+
				"rather than this mean; both are provided because they are different lengths under taper."),
		Assumptions: []string{
			assumeReferenceTrapezoid, assumePlanView,
			"The geometric mean chord is an area-over-span average and is not an aerodynamic " +
				"reference length; MAC is.",
		},
	},
	EqMeanAerodynamicChord: {
		ID:         EqMeanAerodynamicChord,
		Revision:   "1",
		Expression: "MAC = (2/3) * c_root * (1 + lambda + lambda^2) / (1 + lambda)",
		Inputs:     []Port{portRootChord, portTaperRatio},
		Output: Port{
			Name:        "mac",
			Description: "mean aerodynamic chord of the reference trapezoid",
			Dimension:   DimLength,
		},
		Source: sourceWingLayout,
		Assumptions: []string{
			assumeReferenceTrapezoid, assumeLinearTaper, assumeSymmetricPanels,
			"MAC is a reference length for coefficients and moment arms, not a manufacturing edge.",
		},
	},
	EqMACStation: {
		ID:         EqMACStation,
		Revision:   "1",
		Expression: "y_MAC = b * (1 + 2 * lambda) / (6 * (1 + lambda))",
		Inputs:     []Port{portSpan, portTaperRatio},
		Output: Port{
			Name:        "y_mac",
			Description: "spanwise station of the MAC, from the centerline",
			Dimension:   DimLength,
		},
		Source: sourceWingLayout,
		Assumptions: []string{
			assumeReferenceTrapezoid, assumeLinearTaper, assumeSymmetricPanels, assumePlanView,
			"The station locates the MAC spanwise only; placing it longitudinally needs the sweep " +
				"and datum conventions.",
		},
	},
	EqChordAtStation: {
		ID:         EqChordAtStation,
		Revision:   "1",
		Expression: "c(y) = c_root * (1 - (1 - lambda) * 2 * y / b)",
		Inputs:     []Port{portRootChord, portTaperRatio, portSpan, portStation},
		Output:     portChord,
		Source: derivedFrom(sourceWingLayout,
			"Linear interpolation between the chapter's root and tip chords."),
		Assumptions: []string{
			assumeLinearTaper, assumeSymmetricPanels, assumePlanView,
			"The station is measured from the centerline and must lie within the semi-span.",
		},
	},
	EqSweepTransform: {
		ID:         EqSweepTransform,
		Revision:   "1",
		Expression: "tan(Lambda_n) = tan(Lambda_m) - (4/A) * ((n - m) * (1 - lambda) / (1 + lambda))",
		Inputs: []Port{
			portReferenceSweep, portReferenceFraction, portTargetFraction,
			portAspectRatio, portTaperRatio,
		},
		Output: Port{
			Name:        "sweep",
			Description: "sweep angle at the target chord fraction, positive aft",
			Dimension:   DimAngle,
		},
		Source: sourceWingLayout,
		Assumptions: []string{
			assumeStraightEdges, assumeLinearTaper, assumePlanView,
			"Forward sweep is a negative angle and is a valid input, not an error.",
		},
	},
	EqSpanFromAreaAndRootChord: {
		ID:         EqSpanFromAreaAndRootChord,
		Revision:   "1",
		Expression: "b = 2 * S / (c_root * (1 + lambda))",
		Inputs:     []Port{portArea, portRootChord, portTaperRatio},
		Output:     portSpan,
		Source:     derivedFrom(sourceWingLayout, "Inversion of the chapter's root-chord relation for span."),
		Assumptions: []string{
			assumeReferenceTrapezoid, assumeLinearTaper, assumePlanView,
		},
	},
	EqAreaFromSpanAndRootChord: {
		ID:         EqAreaFromSpanAndRootChord,
		Revision:   "1",
		Expression: "S = b * c_root * (1 + lambda) / 2",
		Inputs:     []Port{portSpan, portRootChord, portTaperRatio},
		Output:     portArea,
		Source:     derivedFrom(sourceWingLayout, "Inversion of the chapter's root-chord relation for area."),
		Assumptions: []string{
			assumeReferenceTrapezoid, assumeLinearTaper, assumePlanView,
		},
	},
	EqSpanFromAspectAndRootChord: {
		ID:         EqSpanFromAspectAndRootChord,
		Revision:   "1",
		Expression: "b = A * c_root * (1 + lambda) / 2",
		Inputs:     []Port{portAspectRatio, portRootChord, portTaperRatio},
		Output:     portSpan,
		Source: derivedFrom(sourceWingLayout,
			"Elimination of area between the chapter's aspect-ratio and root-chord relations."),
		Assumptions: []string{
			assumeReferenceTrapezoid, assumeLinearTaper, assumePlanView,
		},
	},
	EqTipLeadingEdgeOffset: {
		ID:         EqTipLeadingEdgeOffset,
		Revision:   "1",
		Expression: "x_le_tip = (b/2) * tan(Lambda_le)",
		Inputs:     []Port{portSpan, portLeadingEdgeSweep},
		Output: Port{
			Name:        "tip_leading_edge_offset",
			Description: "plan-view distance from the root leading edge to the tip leading edge, aft positive",
			Dimension:   DimLength,
		},
		Source: derivedFrom(sourceWingLayout,
			"Construction offset implied by the chapter's straight-edge sweep definition."),
		Assumptions: []string{
			assumeStraightEdges, assumePlanView,
			"The offset is measured in the plan view from the datum at the root leading edge; " +
				"forward sweep makes it negative.",
		},
	},
	EqTipRise: {
		ID:         EqTipRise,
		Revision:   "1",
		Expression: "z_tip = (b/2) * tan(Gamma)",
		Inputs:     []Port{portSpan, portDihedral},
		Output: Port{
			Name:        "tip_rise",
			Description: "height of the tip above the root chord line",
			Dimension:   DimLength,
		},
		Source: derivedFrom(sourceWingLayout,
			"Height implied by rotating the panel about the root chord line through the chapter's "+
				"dihedral angle; b here is the projected span, so the relation holds in both modes."),
		Assumptions: []string{
			assumeUniformDihedral, assumeSymmetricPanels,
			"Anhedral makes the rise negative, which is a valid wing, not an error.",
		},
	},
	EqMACLeadingEdgeStation: {
		ID:         EqMACLeadingEdgeStation,
		Revision:   "1",
		Expression: "x_le_mac = y_mac * tan(Lambda_le)",
		Inputs:     []Port{portStation, portLeadingEdgeSweep},
		Output: Port{
			Name:        "mac_leading_edge_station",
			Description: "distance aft from the root leading edge to the MAC leading edge",
			Dimension:   DimLength,
		},
		Source: derivedFrom(sourceWingLayout,
			"Longitudinal placement of the chapter's MAC, which the chapter reports as a length "+
				"and a spanwise station without locating it fore and aft. This is what a later "+
				"centre-of-gravity position is measured against, so it needs the datum and the "+
				"leading-edge sweep to mean anything."),
		Assumptions: []string{
			assumeStraightEdges, assumePlanView, assumeLinearTaper,
			"The station is measured in the plan view from the datum at the root leading edge. " +
				"Locating the MAC is not a centre-of-gravity result: no mass, balance or static " +
				"margin model is implemented here.",
		},
	},
	EqExposedArea: {
		ID:         EqExposedArea,
		Revision:   "1",
		Expression: "S_exposed = S - d * (c_root + c(d/2)) / 2",
		Inputs:     []Port{portArea, portRootChord, portTaperRatio, portSpan, portBodyWidth},
		Output: Port{
			Name:        "exposed_area",
			Description: "reference area outside the body, both panels",
			Dimension:   DimArea,
		},
		Source: derivedFrom(sourceWingLayout,
			"Exact integration of the linear chord distribution across the body width. The chapter "+
				"itself only carries the reference trapezoid through the centerline; the book's Lift "+
				"chapter quotes S_exposed = 106 ft^2 for the same wing with a 5 ft body, which this "+
				"relation reproduces as 106.1 ft^2."),
		Assumptions: []string{
			assumeReferenceTrapezoid, assumeLinearTaper, assumeSymmetricPanels,
			"The body is treated as a constant-width plan-view band centred on the centerline.",
		},
	},
	EqProjectedSpan: {
		ID:         EqProjectedSpan,
		Revision:   "1",
		Expression: "b_projected = b_panel * cos(Gamma)",
		Inputs:     []Port{portPanelSpan, portDihedral},
		Output:     portSpan,
		Source: derivedFrom(sourceWingLayout,
			"Plan-view projection of the chapter's uniform dihedral choice; the chapter selects a "+
				"dihedral angle but states no projection relation."),
		Assumptions: []string{assumeUniformDihedral, assumeSymmetricPanels, assumePlanView},
	},
	EqProjectedArea: {
		ID:         EqProjectedArea,
		Revision:   "1",
		Expression: "S_projected = S_panel * cos(Gamma)",
		Inputs:     []Port{portPanelArea, portDihedral},
		Output:     portArea,
		Source: derivedFrom(sourceWingLayout,
			"Plan-view projection of the chapter's uniform dihedral choice."),
		Assumptions: []string{assumeUniformDihedral, assumeSymmetricPanels, assumePlanView},
	},
	EqPanelSpan: {
		ID:         EqPanelSpan,
		Revision:   "1",
		Expression: "b_panel = b_projected / cos(Gamma)",
		Inputs:     []Port{portSpan, portDihedral},
		Output:     portPanelSpan,
		Source: derivedFrom(sourceWingLayout,
			"Inversion of the plan-view projection, for the mode that holds projected dimensions fixed."),
		Assumptions: []string{
			assumeUniformDihedral, assumeSymmetricPanels,
			"A panel dimension is a construction length in the panel plane, not a plan-view dimension.",
		},
	},
	EqPanelArea: {
		ID:         EqPanelArea,
		Revision:   "1",
		Expression: "S_panel = S_projected / cos(Gamma)",
		Inputs:     []Port{portArea, portDihedral},
		Output:     portPanelArea,
		Source: derivedFrom(sourceWingLayout,
			"Inversion of the plan-view projection, for the mode that holds projected dimensions fixed."),
		Assumptions: []string{
			assumeUniformDihedral, assumeSymmetricPanels,
			"A panel dimension is a construction length in the panel plane, not a plan-view dimension.",
		},
	},
	EqSemiSpan: {
		ID:         EqSemiSpan,
		Revision:   "1",
		Expression: "b_half = b / 2",
		Inputs:     []Port{portSpan},
		Output: Port{
			Name:        "semi_span",
			Description: "half span, the length of one panel in the same plane",
			Dimension:   DimLength,
		},
		Source: derivedFrom(sourceWingLayout,
			"Half of the chapter's span. It is carried as its own parameter because a sketch is "+
				"usually driven by the half span, and because the plan-view half and the panel "+
				"half are different lengths under dihedral."),
		Assumptions: []string{assumeSymmetricPanels},
	},
	EqReynolds: {
		ID:         EqReynolds,
		Revision:   "1",
		Expression: "Re = rho * V * c / mu",
		Inputs:     []Port{portDensity, portSpeed, portChord, portViscosity},
		Output: Port{
			Name:        "reynolds",
			Description: "Reynolds number at the reported station",
			Dimension:   Dimensionless,
		},
		Source: sourceSimilarity,
		Assumptions: []string{
			assumeLocalChordReynolds, assumeTrueAirspeed,
			"A Reynolds number is a flow condition, not an airfoil result: no polar, lift " +
				"coefficient or drag value is implied or interpolated here.",
		},
	},
}
