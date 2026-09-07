package calculator

// Mass-properties equation IDs. Like the other families these are stable: a
// trace records the ID and revision, so a stored balance result can be matched
// against the relationship that produced it.
const (
	EqTotalMass            = "mass.total"
	EqComponentMoment      = "mass.component-moment"
	EqMomentSum            = "mass.moment-sum"
	EqCenterOfGravity      = "mass.center-of-gravity"
	EqStationFractionOfMAC = "mass.station-fraction-of-mac"
)

// sourceCenterOfGravity is the second SourceBook attribution in this package.
// The Center of gravity chapter was read on the access date and does contain
// the weighted-mean relation implemented here, unlike the Lift chapter, which
// contains no stall-speed equation.
//
// The chapter writes the sum over component *weights*. This package holds
// masses, and under one uniform standard gravity the g cancels from numerator
// and denominator, so the mass form computes the same station. That is the one
// adaptation, and it is recorded here rather than left for a reader to notice.
var sourceCenterOfGravity = Source{
	Kind:     SourceBook,
	Title:    "CODE Lab Aircraft Design — Weight and Balance, Center of gravity",
	URL:      centerOfGravityURL,
	Section:  "Center of gravity",
	Accessed: accessedOn,
	OriginalUnits: "US customary in the worked example (lb, ft, lb-ft), measured from a nose " +
		"datum; the relation itself is unit-agnostic and is evaluated here in SI",
	Adaptation: "Ported to Go and evaluated in SI. The chapter writes " +
		"x_cg = sum(x_k W_k)/sum(W_k) over component weights; this package holds masses, and one " +
		"uniform standard gravity cancels between the numerator and the denominator, so the mass " +
		"form gives the same station. The chapter states that a similar equation applies to the y " +
		"and z axes and demonstrates only x; all three axes are evaluated here with the same " +
		"relation. Its datum is the aircraft nose and its example is a manned twin with a piston " +
		"propulsion group; neither the datum nor any of its component masses is a default here.",
}

// centerOfGravityURL is the checked chapter the mass equations may cite.
// TestBookProvenanceIsLimitedToCheckedChapterMethods holds the list.
const centerOfGravityURL = "https://computationaldesignlab.github.io/aircraft-design/weight_and_balance/cg.html"

var (
	portComponentMass = Port{
		Name:        "component_mass",
		Description: "one component's mass; the input is repeated once per component",
		Dimension:   DimMass,
	}
	portTotalMass = Port{
		Name:        "total_mass",
		Description: "sum of the contributing component masses",
		Dimension:   DimMass,
	}
	portComponentStation = Port{
		Name:        "component_station",
		Description: "the component's distance from the datum along one axis, signed",
		Dimension:   DimLength,
	}
	portMassMoment = Port{
		Name: "mass_moment",
		Description: "one component's mass moment about the datum on one axis; the input is " +
			"repeated once per component",
		Dimension: DimMassMoment,
	}
	portMassMomentSum = Port{
		Name:        "mass_moment_sum",
		Description: "sum of the contributing mass moments on one axis",
		Dimension:   DimMassMoment,
	}
	portCGStation = Port{
		Name:        "cg_station",
		Description: "the centre of gravity's distance from the datum along one axis, signed",
		Dimension:   DimLength,
	}
	portMACLeading = Port{
		Name:        "mac_leading_edge_station",
		Description: "how far aft of the datum the mean aerodynamic chord's leading edge sits",
		Dimension:   DimLength,
	}
	portMAC = Port{
		Name:        "mac",
		Description: "mean aerodynamic chord length",
		Dimension:   DimLength,
	}
)

const (
	assumeMechanicalCG = "This is the mechanical centre of gravity of the listed masses. It is " +
		"not an aerodynamic centre, a neutral point or a centre of pressure, and no static margin, " +
		"trim or handling conclusion follows from it."
	assumeRigidPointMasses = "Each component is treated as a point mass at its stated position. " +
		"No component's own moment of inertia or internal distribution is modelled."
	assumeCompleteInventory = "The result describes exactly the masses listed. A component with " +
		"no stated mass or no stated position contributes nothing and makes the assessment " +
		"incomplete; it is never placed at the origin."
	assumeSharedDatum = "Every position is measured in the design's declared datum, and a result " +
		"is meaningless without it."
	assumeUniformGravity = "One uniform standard gravity is assumed across the aircraft, which is " +
		"what lets the chapter's weight form be evaluated with masses."
	assumeMACReference = "Expressing a station as a fraction of the mean aerodynamic chord is a " +
		"geometric reference. It is not a static margin and carries no stability claim."
)

// massEquations holds the Task 07 mass-properties relationships. They are kept
// in their own set so that this family's provenance stays separately
// inspectable, as the lift and geometry sets are.
var massEquations = map[string]Equation{
	EqTotalMass: {
		ID:         EqTotalMass,
		Revision:   "1",
		Expression: "m = sum(m_i)",
		// One declared port, substituted once per component. The trace therefore
		// lists every mass that entered the sum rather than only its result,
		// which is what makes a total inspectable at all.
		Inputs:      []Port{portComponentMass},
		Output:      portTotalMass,
		Source:      sourceCenterOfGravity,
		Assumptions: []string{assumeCompleteInventory, assumeRigidPointMasses},
	},
	EqComponentMoment: {
		ID:          EqComponentMoment,
		Revision:    "1",
		Expression:  "M_i = m_i * r_i",
		Inputs:      []Port{portComponentMass, portComponentStation},
		Output:      portMassMoment,
		Source:      sourceCenterOfGravity,
		Assumptions: []string{assumeRigidPointMasses, assumeSharedDatum, assumeUniformGravity},
	},
	EqMomentSum: {
		ID:          EqMomentSum,
		Revision:    "1",
		Expression:  "M = sum(M_i)",
		Inputs:      []Port{portMassMoment},
		Output:      portMassMomentSum,
		Source:      sourceCenterOfGravity,
		Assumptions: []string{assumeCompleteInventory, assumeSharedDatum},
	},
	EqCenterOfGravity: {
		ID:          EqCenterOfGravity,
		Revision:    "1",
		Expression:  "r_cg = sum(m_i * r_i) / sum(m_i)",
		Inputs:      []Port{portMassMomentSum, portTotalMass},
		Output:      portCGStation,
		Source:      sourceCenterOfGravity,
		Assumptions: []string{assumeMechanicalCG, assumeCompleteInventory, assumeSharedDatum, assumeUniformGravity},
	},
	EqStationFractionOfMAC: {
		ID:         EqStationFractionOfMAC,
		Revision:   "1",
		Expression: "fraction = (x - x_le_mac) / MAC",
		Inputs:     []Port{portCGStation, portMACLeading, portMAC},
		Output: Port{
			Name:        "mac_fraction",
			Description: "the station as a fraction of the mean aerodynamic chord, 0 at its leading edge",
			Dimension:   Dimensionless,
		},
		Source:      derivedFrom(sourceCenterOfGravity, "Rearranged to express a station as a chord fraction."),
		Assumptions: []string{assumeMACReference, assumeSharedDatum},
	},
}
