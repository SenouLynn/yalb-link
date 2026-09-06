package calculator

import "math"

// Configuration names the airframe layout. It is part of the design definition
// from the start because the three layouts do not share a handling model: a
// missing horizontal tail is a defect on a conventional aircraft and is the
// defining feature of a flying wing.
type Configuration uint8

const (
	// ConfigurationUnknown is the zero value and is never accepted.
	ConfigurationUnknown Configuration = iota
	// ConfigurationConventionalTail has separate horizontal and vertical tails.
	ConfigurationConventionalTail
	// ConfigurationVTail has two canted panels that serve both axes.
	ConfigurationVTail
	// ConfigurationFlyingWing has no tail group at all.
	ConfigurationFlyingWing
)

var configurationNames = [...]string{
	ConfigurationUnknown:          "unknown",
	ConfigurationConventionalTail: "conventional tail",
	ConfigurationVTail:            "V-tail",
	ConfigurationFlyingWing:       "flying wing",
}

// String returns the configuration's readable name.
func (c Configuration) String() string {
	if int(c) < len(configurationNames) {
		return configurationNames[c]
	}
	return "unknown"
}

// TailSurface is one conventional tail surface's reference geometry.
type TailSurface struct {
	// Area is the surface's reference area.
	Area Quantity
	// Span is its tip-to-tip span, or its height for a vertical surface.
	Span Quantity
	// Arm is the distance from the wing MAC quarter chord to the surface's own
	// quarter chord, measured along x.
	Arm Quantity
}

func (s TailSurface) supplied() bool { return s != TailSurface{} }

// VTailPanels records a V-tail as what it is: two canted panels. It stores the
// panel area and cant angle rather than an equivalent horizontal and vertical
// tail, because resolving the panels into two imaginary surfaces is a handling
// model this package does not implement.
type VTailPanels struct {
	// PanelArea is the area of one panel.
	PanelArea Quantity
	// PanelSpan is the length of one panel, measured along its own plane.
	PanelSpan Quantity
	// Cant is the angle of a panel from the horizontal, positive up. A cant of
	// 90 degrees would be a pure vertical tail and 0 a pure horizontal one;
	// neither is a V-tail.
	Cant Quantity
	// Arm is the distance from the wing MAC quarter chord to the panels' own
	// quarter chord, measured along x.
	Arm Quantity
}

func (v VTailPanels) supplied() bool { return v != VTailPanels{} }

// TailGeometry holds whichever tail description the configuration calls for.
// Only the fields belonging to the chosen configuration may be filled in.
type TailGeometry struct {
	// Horizontal and Vertical describe a conventional tail.
	Horizontal TailSurface
	// Vertical is the fin.
	Vertical TailSurface
	// VTail describes canted V-tail panels.
	VTail VTailPanels
}

// Airframe is a wing plus the tail description its configuration requires.
type Airframe struct {
	// Name identifies the design.
	Name string
	// Wing is the wing definition.
	Wing WingDefinition
	// Tail is the tail description, which must match Configuration.
	Tail TailGeometry
	// Configuration names the layout.
	Configuration Configuration
}

// cant angles outside this band are not a V-tail; they are a horizontal or a
// vertical surface, whose handling models differ again.
const (
	minCantDegrees = 15.0
	maxCantDegrees = 75.0
)

// ValidateGeometry checks that the tail description matches the configuration.
// It is a geometry check only: a complete description is not a handling result,
// and no configuration here has a supported handling assessment.
func (a Airframe) ValidateGeometry() error {
	rs := &resultSet{}
	switch a.Configuration {
	case ConfigurationConventionalTail:
		a.validateConventional(rs)
	case ConfigurationVTail:
		a.validateVTail(rs)
	case ConfigurationFlyingWing:
		a.validateFlyingWing(rs)
	case ConfigurationUnknown:
		rs.add("configuration", IssueMissing,
			"choose the configuration: conventional, V-tail and flying wing do not share a tail "+
				"description or a handling model")
	default:
		rs.add("configuration", IssueUnsupported,
			"only conventional, V-tail and flying-wing layouts are implemented")
	}
	if len(rs.issues) > 0 {
		return rs.issues
	}
	return nil
}

func (a Airframe) validateConventional(rs *resultSet) {
	if !a.Tail.Horizontal.supplied() {
		rs.add("horizontal_tail", IssueMissing, "a conventional tail needs a horizontal surface")
	}
	if !a.Tail.Vertical.supplied() {
		rs.add("vertical_tail", IssueMissing, "a conventional tail needs a vertical surface")
	}
	if a.Tail.VTail.supplied() {
		rs.add("v_tail", IssueInvalid,
			"a conventional tail has no V-tail panels; choose the V-tail configuration instead")
	}
}

func (a Airframe) validateVTail(rs *resultSet) {
	if !a.Tail.VTail.supplied() {
		rs.add("v_tail", IssueMissing, "a V-tail needs its panel area and cant angle")
		return
	}
	if a.Tail.Horizontal.supplied() || a.Tail.Vertical.supplied() {
		rs.add("v_tail", IssueInvalid,
			"a V-tail is two canted panels, not a horizontal and a vertical tail; describe the "+
				"panels instead")
	}
	if !a.Tail.VTail.PanelArea.supplied() {
		rs.add("v_tail_panel_area", IssueMissing, "state the area of one panel")
	}
	if !a.Tail.VTail.Cant.supplied() {
		rs.add("v_tail_cant", IssueMissing, "state the cant angle of a panel from the horizontal")
		return
	}
	degrees := a.Tail.VTail.Cant.si * 180 / math.Pi
	if degrees < minCantDegrees || degrees > maxCantDegrees {
		rs.add("v_tail_cant", IssueUnsupported,
			"a cant outside "+formatFloat(minCantDegrees)+" to "+formatFloat(maxCantDegrees)+
				" degrees is a horizontal or a vertical surface rather than a V-tail; received "+
				formatFloat(degrees)+" degrees")
	}
}

func (a Airframe) validateFlyingWing(rs *resultSet) {
	// A flying wing is complete without a tail. Nothing is reported missing
	// here: treating an absent horizontal tail as a defect is exactly the error
	// this configuration exists to prevent.
	if a.Tail != (TailGeometry{}) {
		rs.add("tail", IssueInvalid,
			"a flying wing has no tail group; its pitch and yaw control come from the wing itself, "+
				"which needs a model this package does not implement")
	}
}

// HorizontalTailArea returns the conventional horizontal tail's reference area.
// It refuses to invent one for the other configurations: a V-tail's equivalent
// horizontal surface is a modelling claim, not a measurement, and a flying wing
// has no such surface at all.
func (a Airframe) HorizontalTailArea() (Quantity, error) {
	switch a.Configuration {
	case ConfigurationConventionalTail:
		if !a.Tail.Horizontal.Area.supplied() {
			return Quantity{}, Issues{{
				Field:  "horizontal_tail",
				Kind:   IssueMissing,
				Detail: "no horizontal tail area was supplied",
			}}
		}
		return a.Tail.Horizontal.Area, nil
	case ConfigurationVTail:
		return Quantity{}, Issues{{
			Field: "v_tail",
			Kind:  IssueUnsupported,
			Detail: "resolving V-tail panels into an equivalent horizontal tail requires a " +
				"validated V-tail model, which is not implemented; use the panel area and cant angle",
		}}
	case ConfigurationFlyingWing:
		return Quantity{}, Issues{{
			Field:  "horizontal_tail",
			Kind:   IssueUnsupported,
			Detail: "a flying wing has no horizontal tail; its pitch behaviour is not this quantity",
		}}
	case ConfigurationUnknown:
		return Quantity{}, Issues{{
			Field:  "configuration",
			Kind:   IssueMissing,
			Detail: "choose the configuration first",
		}}
	default:
		return Quantity{}, Issues{{
			Field:  "configuration",
			Kind:   IssueUnsupported,
			Detail: "only conventional, V-tail and flying-wing layouts are implemented",
		}}
	}
}

// Handling reports what this package can say about how an airframe flies, which
// is nothing. Task 03 delivers geometry; trim, static margin, control authority
// and dynamic behaviour are separate models with their own evidence, and none
// of them is implemented for any configuration.
func (a Airframe) Handling() error {
	return Issues{{
		Field: "handling",
		Kind:  IssueUnsupported,
		Detail: "no handling model is implemented for a " + a.Configuration.String() +
			"; geometry alone establishes no trim, stability or control result",
	}}
}
