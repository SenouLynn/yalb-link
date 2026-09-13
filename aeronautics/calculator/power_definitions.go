package calculator

// Power, propulsion and mission equation IDs. Like the other families these are
// stable: a trace records the ID and revision, so a stored energy budget can be
// matched against the relationships that produced it.
const (
	EqElectricalPower        = "power.electrical"
	EqPropulsivePower        = "power.propulsive"
	EqElectricalRequired     = "power.electrical-required"
	EqThrustToWeight         = "propulsion.thrust-to-weight"
	EqPropellerClearance     = "propulsion.propeller-clearance"
	EqAuxiliaryPackSide      = "power.auxiliary-pack-side"
	EqAuxiliarySum           = "power.auxiliary-sum"
	EqSegmentLift            = "flight.segment-lift"
	EqSegmentLiftCoefficient = "flight.segment-lift-coefficient"
	EqRequiredThrust         = "flight.required-thrust"

	EqBatteryEnergy       = "battery.nominal-energy"
	EqBatteryUsableEnergy = "battery.usable-energy"
	EqEnergyBudget        = "mission.energy-budget"

	EqSegmentEnergy         = "mission.segment-energy"
	EqEnergySum             = "mission.energy-required"
	EqEnduranceConstantDraw = "mission.endurance-constant-draw"
	EqGroundSpeed           = "mission.ground-speed"
	EqSegmentDistance       = "mission.segment-distance"
	EqSegmentDuration       = "mission.segment-duration"
	EqDistanceSum           = "mission.distance-total"
	EqDurationSum           = "mission.duration-total"
)

// sourceElectricAccounting covers the unit-level identities the electric
// propulsion and energy accounting are built from. They are coherent SI derived
// relations rather than aerodynamic models, and the primary reference contains
// none of them: its powerplant chapter sizes a piston engine and its mission
// analysis burns fuel.
//
// The definitions this project adds on top of them — a single chain efficiency
// covering propeller, motor and speed controller, an auxiliary draw added on
// the pack side, a usable fraction of a pack's nominal energy, and a mission
// reserve applied once — are recorded in Adaptation and in sources.md, and are
// this project's electric adaptation rather than anyone's published method.
var sourceElectricAccounting = Source{
	Kind:          SourceSupplementary,
	Title:         "BIPM, The International System of Units (SI Brochure), 9th edition — coherent derived units",
	URL:           "https://www.bipm.org/en/publications/si-brochure",
	Section:       "Table 4, coherent derived units with special names",
	Accessed:      polarAccessedOn,
	OriginalUnits: "SI",
	Adaptation: "Used for the coherent relations W = V*A, J = W*s, J = C*V and W = N*(m/s), " +
		"which is what makes electrical power, energy, pack capacity and propulsive power one " +
		"consistent set. The modelling choices layered on them are this project's electric " +
		"adaptation and are not taken from any source: a single propeller/motor/ESC chain " +
		"efficiency at a stated condition, an auxiliary electrical draw added on the pack side " +
		"of every regulator, a usable fraction of a pack's nominal energy, and a mission reserve " +
		"applied exactly once to the usable energy. The primary reference's own mission method " +
		"is a piston fuel-fraction analysis and is not implemented.",
}

// sourceClimbBalance is the steady flight-path force balance the segment model
// resolves. The cited page writes the balance in ground axes with acceleration
// terms; setting those to zero reduces it to the two relations used here, and
// that reduction is this package's algebra rather than the page's.
var sourceClimbBalance = Source{
	Kind:          SourceSupplementary,
	Title:         "NASA Beginner's Guide to Aeronautics — Forces in a Climb",
	URL:           "https://www1.grc.nasa.gov/beginners-guide-to-aeronautics/forces-in-a-climb/",
	Section:       "Forces in a climb",
	Accessed:      polarAccessedOn,
	OriginalUnits: "SI",
	Adaptation: "The page gives the ground-axis balances F*sin(c) - D*sin(c) + L*cos(c) - W = m*a_v " +
		"and F*cos(c) - D*cos(c) - L*sin(c) = m*a_h. Setting both accelerations to zero reduces " +
		"them to L = W*cos(gamma) and T = D + W*sin(gamma); that reduction is this package's " +
		"algebra and is recorded on the equations that use it. A load factor multiplies the " +
		"perpendicular balance so that a steady banked or pulled-up condition can be described; " +
		"the load factor and the flight-path angle are independent inputs and neither is derived " +
		"from the other.",
}

// sourcePropellerData is cited for applicability rather than for arithmetic. It
// is a database of measured propeller performance, held separately for static
// and wind-tunnel conditions, which is the evidence that those two conditions
// are different measurements rather than one number scaled.
var sourcePropellerData = Source{
	Kind:     SourceSupplementary,
	Title:    "UIUC Propeller Data Site — wind tunnel measurements for small UAV and model aircraft propellers",
	URL:      "https://m-selig.ae.illinois.edu/props/propDB.html",
	Section:  "Propeller database",
	Accessed: polarAccessedOn,
	OriginalUnits: "the database reports non-dimensional thrust, power and efficiency coefficients " +
		"against advance ratio, together with separate static performance data",
	Adaptation: "Used for applicability only, and for no equation. The database holds static and " +
		"wind-tunnel measurements as separate data, which is why this package refuses to answer " +
		"an in-flight thrust question from a static capability point: establishing thrust across " +
		"the flight envelope needs measured propeller data of this kind, and a motor Kv with a " +
		"propeller's diameter and pitch does not substitute for it. No coefficient, curve or " +
		"interpolation from the database is implemented here.",
}

var (
	portVoltage = Port{
		Name:        "voltage",
		Description: "pack voltage under load at the condition",
		Dimension:   DimVoltage,
	}
	portCurrent = Port{
		Name:        "current",
		Description: "current drawn at the condition",
		Dimension:   DimCurrent,
	}
	portElectricalPower = Port{
		Name:        "electrical_power",
		Description: "electrical power drawn from the pack",
		Dimension:   DimPower,
	}
	portThrust = Port{
		Name:        "thrust",
		Description: "thrust along the flight path",
		Dimension:   DimForce,
	}
	portDrag = Port{
		Name:        "drag",
		Description: "drag force at the condition",
		Dimension:   DimForce,
	}
	portPropulsivePower = Port{
		Name:        "propulsive_power",
		Description: "useful propulsive power, thrust times true airspeed",
		Dimension:   DimPower,
	}
	portChainEfficiency = Port{
		Name:        "chain_efficiency",
		Description: "combined propeller, motor and speed-controller efficiency at the condition",
		Dimension:   Dimensionless,
	}
	portAuxiliaryPower = Port{
		Name:        "auxiliary_power",
		Description: "non-propulsive electrical draw on the pack side of every regulator",
		Dimension:   DimPower,
	}
	portLoadPower = Port{
		Name:        "load_power",
		Description: "one load's draw measured downstream of its regulator",
		Dimension:   DimPower,
	}
	portRegulatorEfficiency = Port{
		Name:        "regulator_efficiency",
		Description: "the regulator's efficiency, converting a load-side draw to a pack-side one",
		Dimension:   Dimensionless,
	}
	portPackSidePower = Port{
		Name: "pack_side_power",
		Description: "one load's draw as the pack supplies it; the input is repeated once per " +
			"auxiliary load",
		Dimension: DimPower,
	}
	portCharge = Port{
		Name:        "charge",
		Description: "the pack's labelled charge capacity",
		Dimension:   DimCharge,
	}
	portNominalEnergy = Port{
		Name:        "nominal_energy",
		Description: "the pack's nominal energy",
		Dimension:   DimEnergy,
	}
	portUsableFraction = Port{
		Name:        "usable_fraction",
		Description: "the fraction of the pack's nominal energy that may actually be drawn",
		Dimension:   Dimensionless,
	}
	portUsableEnergy = Port{
		Name:        "usable_energy",
		Description: "the energy the pack may actually deliver",
		Dimension:   DimEnergy,
	}
	portReserveFraction = Port{
		Name:        "reserve_fraction",
		Description: "the fraction of the usable energy the mission holds back",
		Dimension:   Dimensionless,
	}
	portEnergyBudget = Port{
		Name:        "energy_budget",
		Description: "the usable energy the mission may spend, after its reserve",
		Dimension:   DimEnergy,
	}
	portSegmentPower = Port{
		Name:        "segment_power",
		Description: "the electrical power held through one mission segment",
		Dimension:   DimPower,
	}
	portDuration = Port{
		Name:        "duration",
		Description: "elapsed time",
		Dimension:   DimTime,
	}
	portSegmentEnergy = Port{
		Name:        "segment_energy",
		Description: "one segment's energy; the input is repeated once per segment",
		Dimension:   DimEnergy,
	}
	portRequiredEnergy = Port{
		Name:        "required_energy",
		Description: "the energy every segment demands together",
		Dimension:   DimEnergy,
	}
	portGroundSpeed = Port{
		Name:        "ground_speed",
		Description: "speed over the ground along the track, signed",
		Dimension:   DimSpeed,
	}
	portWindAlongTrack = Port{
		Name:        "wind_along_track",
		Description: "the wind component along the ground track, positive as a tailwind",
		Dimension:   DimSpeed,
	}
	portClimbAngle = Port{
		Name:        "climb_angle",
		Description: "the flight-path angle, positive climbing",
		Dimension:   DimAngle,
	}
	portDistance = Port{
		Name:        "distance",
		Description: "distance over the ground along the track",
		Dimension:   DimLength,
	}
	portSegmentDistance = Port{
		Name:        "segment_distance",
		Description: "one segment's ground distance; the input is repeated once per segment",
		Dimension:   DimLength,
	}
	portSegmentDuration = Port{
		Name:        "segment_duration",
		Description: "one segment's duration; the input is repeated once per segment",
		Dimension:   DimTime,
	}
	portHubHeight = Port{
		Name:        "propeller_hub_height",
		Description: "how far the propeller hub sits above the ground line",
		Dimension:   DimLength,
	}
	portPropellerDiameter = Port{
		Name:        "propeller_diameter",
		Description: "the propeller's diameter",
		Dimension:   DimLength,
	}
	portLift = Port{
		Name:        "lift",
		Description: "lift perpendicular to the flight path",
		Dimension:   DimForce,
	}
)

const (
	assumeSteadyFlightPath = "The condition is steady: no acceleration along or perpendicular to " +
		"the flight path. A launch transient, a pull-up and a gust are none of them this."
	assumeThrustAlongPath = "Thrust acts along the flight path. A thrust line inclined to it, and " +
		"the pitching moment such a line produces, are not modelled."
	assumeIndependentLoadFactor = "The load factor and the flight-path angle are independent " +
		"inputs. Neither is derived from the other, and the combination is the builder's " +
		"description of the condition."
	assumeChainEfficiency = "One efficiency covers the propeller, the motor and the speed " +
		"controller together, at the stated condition. It is supplied evidence: nothing here " +
		"derives it from a Kv, a propeller size or an operating point."
	assumeNoStaticDivision = "Useful propulsive power is thrust times airspeed, so it is zero at " +
		"zero airspeed. Dividing zero useful power by a propulsive efficiency does not give the " +
		"electrical power a standing aircraft draws, and this relation is not evaluated there; " +
		"static draw is entered as a capability point instead."
	assumeAuxPackSide = "Auxiliary draw is summed on the pack side of every regulator. A figure " +
		"measured at the load is converted once by that regulator's efficiency, and a figure " +
		"already measured at the pack is not converted at all."
	assumeContinuousOnly = "Only continuous draw enters an energy budget. A peak draw with no " +
		"stated duty cycle carries no energy, and it is checked against supply ratings instead."
	assumeNominalVoltage = "A pack's energy from its capacity uses the nominal voltage, which is " +
		"a label rather than a discharge curve. Real delivered energy depends on the current, the " +
		"temperature and the cell chemistry, which is what the usable fraction stands in for."
	assumeUsableFraction = "The usable fraction is supplied evidence covering voltage sag, cell " +
		"balance and the state of charge the pack is not taken below. It is not a mission reserve."
	assumeReserveOnce = "The reserve is applied exactly once, to the usable energy, and never " +
		"again inside a segment."
	assumeConstantDraw = "A constant-draw endurance holds one electrical power for the whole " +
		"flight. A mission with segments does not, and its energy is summed segment by segment."
	assumeGroundTrack = "Ground speed is the airspeed's horizontal component plus the wind along " +
		"the track. A crosswind component, a varying wind and a curved track are not modelled, " +
		"and a return leg is a segment the builder states rather than one this package adds."
	assumeClearanceGeometry = "Tip clearance is the hub height above the ground line less the " +
		"propeller's radius, with the aircraft level on its gear. Suspension travel, tyre " +
		"deflection, rotation attitude and blade flexing are not modelled."
)

// powerEquations holds the Task 09 propulsion, electrical and mission
// relationships. They are kept in their own set so this family's provenance
// stays separately inspectable, as the lift, geometry, mass and polar sets are.
var powerEquations = map[string]Equation{
	EqElectricalPower: {
		ID:          EqElectricalPower,
		Revision:    "1",
		Expression:  "P = V * I",
		Inputs:      []Port{portVoltage, portCurrent},
		Output:      portElectricalPower,
		Source:      sourceElectricAccounting,
		Assumptions: []string{"Direct-current power at the stated pack voltage under load."},
	},
	EqPropulsivePower: {
		ID:          EqPropulsivePower,
		Revision:    "1",
		Expression:  "P_useful = T * V",
		Inputs:      []Port{portThrust, portSpeed},
		Output:      portPropulsivePower,
		Source:      sourceElectricAccounting,
		Assumptions: []string{assumeThrustAlongPath, assumeTrueAirspeed, assumeNoStaticDivision},
	},
	EqElectricalRequired: {
		ID:         EqElectricalRequired,
		Revision:   "1",
		Expression: "P_elec = P_useful / eta_total + P_aux",
		Inputs:     []Port{portPropulsivePower, portChainEfficiency, portAuxiliaryPower},
		Output:     portElectricalPower,
		Source:     sourceElectricAccounting,
		Assumptions: []string{
			assumeChainEfficiency, assumeNoStaticDivision, assumeAuxPackSide,
		},
	},
	EqThrustToWeight: {
		ID:         EqThrustToWeight,
		Revision:   "1",
		Expression: "T/W = T / (m * g)",
		Inputs:     []Port{portThrust, portMass},
		Output: Port{
			Name:        "thrust_to_weight",
			Description: "thrust-to-weight ratio at the condition",
			Dimension:   Dimensionless,
		},
		Source: derivedFrom(sourceLiftIdentity,
			"Ratio of a thrust to the weight the same lift identity gives."),
		Assumptions: []string{
			assumeStandardGravity,
			"A thrust-to-weight ratio belongs to the condition its thrust was measured at.",
		},
	},
	EqPropellerClearance: {
		ID:         EqPropellerClearance,
		Revision:   "1",
		Expression: "clearance = h_hub - D_prop / 2",
		Inputs:     []Port{portHubHeight, portPropellerDiameter},
		Output: Port{
			Name:        "propeller_clearance",
			Description: "the propeller tip's clearance above the ground line, signed",
			Dimension:   DimLength,
		},
		Source: derivedFrom(sourcePropellerData,
			"A geometric definition this package states; the cited database holds performance "+
				"data and no installation geometry."),
		Assumptions: []string{assumeClearanceGeometry},
	},
	EqAuxiliaryPackSide: {
		ID:          EqAuxiliaryPackSide,
		Revision:    "1",
		Expression:  "P_pack = P_load / eta_regulator",
		Inputs:      []Port{portLoadPower, portRegulatorEfficiency},
		Output:      portPackSidePower,
		Source:      sourceElectricAccounting,
		Assumptions: []string{assumeAuxPackSide},
	},
	EqAuxiliarySum: {
		ID:         EqAuxiliarySum,
		Revision:   "1",
		Expression: "P_aux = sum(P_pack_i)",
		// One declared port, substituted once per load, so the trace lists every
		// draw that entered the sum rather than only its result.
		Inputs:      []Port{portPackSidePower},
		Output:      portAuxiliaryPower,
		Source:      sourceElectricAccounting,
		Assumptions: []string{assumeAuxPackSide, assumeContinuousOnly},
	},
	EqSegmentLift: {
		ID:          EqSegmentLift,
		Revision:    "1",
		Expression:  "L = n * m * g * cos(gamma)",
		Inputs:      []Port{portMass, portLoadFactor, portClimbAngle},
		Output:      portLift,
		Source:      sourceClimbBalance,
		Assumptions: []string{assumeSteadyFlightPath, assumeStandardGravity, assumeIndependentLoadFactor, assumeLumpedLift},
	},
	EqSegmentLiftCoefficient: {
		ID:          EqSegmentLiftCoefficient,
		Revision:    "1",
		Expression:  "CL = L / (q * S)",
		Inputs:      []Port{portLift, portDynamicPressure, portArea},
		Output:      portLiftCoefficient,
		Source:      derivedFrom(sourceLiftIdentity, "The lift identity solved for its coefficient."),
		Assumptions: []string{assumeTrueAirspeed, assumeLumpedLift},
	},
	EqRequiredThrust: {
		ID:          EqRequiredThrust,
		Revision:    "1",
		Expression:  "T = D + m * g * sin(gamma)",
		Inputs:      []Port{portDrag, portMass, portClimbAngle},
		Output:      portThrust,
		Source:      sourceClimbBalance,
		Assumptions: []string{assumeSteadyFlightPath, assumeThrustAlongPath, assumeStandardGravity},
	},

	EqBatteryEnergy: {
		ID:          EqBatteryEnergy,
		Revision:    "1",
		Expression:  "E = Q * V_nominal",
		Inputs:      []Port{portCharge, portVoltage},
		Output:      portNominalEnergy,
		Source:      sourceElectricAccounting,
		Assumptions: []string{assumeNominalVoltage},
	},
	EqBatteryUsableEnergy: {
		ID:          EqBatteryUsableEnergy,
		Revision:    "1",
		Expression:  "E_usable = E * f_usable",
		Inputs:      []Port{portNominalEnergy, portUsableFraction},
		Output:      portUsableEnergy,
		Source:      derivedFrom(sourceElectricAccounting, "The usable fraction is this project's definition."),
		Assumptions: []string{assumeUsableFraction},
	},
	EqEnergyBudget: {
		ID:          EqEnergyBudget,
		Revision:    "1",
		Expression:  "E_budget = E_usable * (1 - reserve)",
		Inputs:      []Port{portUsableEnergy, portReserveFraction},
		Output:      portEnergyBudget,
		Source:      derivedFrom(sourceElectricAccounting, "The reserve convention is this project's definition."),
		Assumptions: []string{assumeReserveOnce},
	},

	EqSegmentEnergy: {
		ID:          EqSegmentEnergy,
		Revision:    "1",
		Expression:  "E_i = P_i * dt_i",
		Inputs:      []Port{portSegmentPower, portDuration},
		Output:      portSegmentEnergy,
		Source:      sourceElectricAccounting,
		Assumptions: []string{assumeContinuousOnly, "The segment's electrical power is held constant across it."},
	},
	EqEnergySum: {
		ID:          EqEnergySum,
		Revision:    "1",
		Expression:  "E_required = sum(E_i)",
		Inputs:      []Port{portSegmentEnergy},
		Output:      portRequiredEnergy,
		Source:      derivedFrom(sourceElectricAccounting, "Summation over the stated segments."),
		Assumptions: []string{assumeReserveOnce, "The sum covers exactly the segments listed."},
	},
	EqEnduranceConstantDraw: {
		ID:          EqEnduranceConstantDraw,
		Revision:    "1",
		Expression:  "t = E_usable / P_elec",
		Inputs:      []Port{portUsableEnergy, portElectricalPower},
		Output:      portDuration,
		Source:      derivedFrom(sourceElectricAccounting, "The energy relation solved for time."),
		Assumptions: []string{assumeConstantDraw},
	},
	EqGroundSpeed: {
		ID:          EqGroundSpeed,
		Revision:    "1",
		Expression:  "V_ground = V * cos(gamma) + w_track",
		Inputs:      []Port{portSpeed, portClimbAngle, portWindAlongTrack},
		Output:      portGroundSpeed,
		Source:      derivedFrom(sourceClimbBalance, "The flight-path speed resolved onto the ground track."),
		Assumptions: []string{assumeGroundTrack, assumeTrueAirspeed},
	},
	EqSegmentDistance: {
		ID:          EqSegmentDistance,
		Revision:    "1",
		Expression:  "d = V_ground * t",
		Inputs:      []Port{portGroundSpeed, portDuration},
		Output:      portDistance,
		Source:      derivedFrom(sourceElectricAccounting, "Distance from a constant ground speed."),
		Assumptions: []string{assumeGroundTrack},
	},
	EqSegmentDuration: {
		ID:          EqSegmentDuration,
		Revision:    "1",
		Expression:  "t = d / V_ground",
		Inputs:      []Port{portDistance, portGroundSpeed},
		Output:      portDuration,
		Source:      derivedFrom(sourceElectricAccounting, "The distance relation solved for time."),
		Assumptions: []string{assumeGroundTrack},
	},
	EqDistanceSum: {
		ID:          EqDistanceSum,
		Revision:    "1",
		Expression:  "d_total = sum(d_i)",
		Inputs:      []Port{portSegmentDistance},
		Output:      portDistance,
		Source:      derivedFrom(sourceElectricAccounting, "Summation over the stated segments."),
		Assumptions: []string{assumeGroundTrack, "The sum covers exactly the segments listed."},
	},
	EqDurationSum: {
		ID:          EqDurationSum,
		Revision:    "1",
		Expression:  "t_total = sum(t_i)",
		Inputs:      []Port{portSegmentDuration},
		Output:      portDuration,
		Source:      derivedFrom(sourceElectricAccounting, "Summation over the stated segments."),
		Assumptions: []string{"The sum covers exactly the segments listed."},
	},
}
