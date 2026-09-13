package calculator

// The Task 09 edits. They follow the same rules as every other command: one
// edit is one revision, a refused command changes nothing, and a structural
// problem that the design's own validation reports better is not re-refused at
// the keystroke. What is refused here is what would make the definition
// unreadable rather than merely incomplete: a value of the wrong dimension, a
// name that identifies nothing, and a link to something the design does not have.

// SetDragPolar records the aircraft drag polar, or withdraws it when passed the
// zero DragPolar. Withdrawing takes every polar-based power result back rather
// than leaving the previous coefficients standing under new geometry.
type SetDragPolar struct {
	// Polar is the polar to record.
	Polar DragPolar
}

// Label names the edit.
func (c SetDragPolar) Label() string {
	if !c.Polar.supplied() {
		return "withdraw the drag polar"
	}
	return "set the drag polar"
}

func (c SetDragPolar) apply(d Design) (Design, error) {
	if c.Polar.supplied() && c.Polar.Scope == ScopeAirfoilSection {
		return Design{}, commandIssue("polar", IssueUnsupported,
			"a 2D airfoil section polar is not an aircraft drag polar: it carries no induced, "+
				"interference or trim drag. Supply aircraft-level coefficients")
	}
	out := d.clone()
	out.Polar = c.Polar
	return out, nil
}

// SetPropulsionEfficiency records the combined propeller, motor and
// speed-controller efficiency.
type SetPropulsionEfficiency struct {
	// Efficiency is the chain efficiency and its evidence.
	Efficiency PropulsionEfficiency
}

// Label names the edit.
func (c SetPropulsionEfficiency) Label() string {
	return "set the propulsion chain efficiency to " + formatFloat(c.Efficiency.Total)
}

func (c SetPropulsionEfficiency) apply(d Design) (Design, error) {
	if c.Efficiency.Total > 1 {
		return Design{}, commandIssue("propulsion.efficiency", IssueInvalid,
			"a chain efficiency above one would take more work out of the pack than went in, "+
				"received "+formatFloat(c.Efficiency.Total))
	}
	out := d.clone()
	out.Propulsion.Efficiency = c.Efficiency
	return out, nil
}

// SetPropulsionLimits records the component ratings a feasibility claim is made
// against. They travel together because they describe one installation, and a
// rating left behind by a partial edit would be checked against a motor that is
// no longer fitted.
type SetPropulsionLimits struct {
	// Limits are the ratings to record.
	Limits PropulsionLimits
}

// Label names the edit.
func (c SetPropulsionLimits) Label() string { return "set the propulsion ratings" }

func (c SetPropulsionLimits) apply(d Design) (Design, error) {
	for _, rating := range []struct {
		field string
		value Quantity
		want  Dimension
	}{
		{"max_continuous_power", c.Limits.MaxContinuousElectricalPower, DimPower},
		{"max_peak_power", c.Limits.MaxPeakElectricalPower, DimPower},
		{"max_rpm", c.Limits.MaxRPM, DimRotationRate},
		{"max_voltage", c.Limits.MaxVoltage, DimVoltage},
		{"propeller_diameter", c.Limits.PropellerDiameter, DimLength},
		{"propeller_hub_height", c.Limits.PropellerHubHeight, DimLength},
	} {
		if rating.value.supplied() && rating.value.dim != rating.want {
			return Design{}, commandIssue("propulsion.limits."+rating.field, IssueInvalid,
				"expected a "+rating.want.String()+" value but received "+rating.value.dim.String())
		}
	}
	out := d.clone()
	out.Propulsion.Limits = c.Limits
	return out, nil
}

// SetCapability adds a propulsion capability point or replaces the one with the
// same name.
type SetCapability struct {
	// Capability is the operating point to record.
	Capability PropulsionCapability
}

// Label names the edit.
func (c SetCapability) Label() string { return "set capability point " + c.Capability.Name }

func (c SetCapability) apply(d Design) (Design, error) {
	if c.Capability.Name == "" {
		return Design{}, commandIssue("capability", IssueMissing,
			"name the capability point so a segment and a target can refer to it")
	}
	if c.Capability.Kind == CapabilityKindUnknown {
		return Design{}, commandIssue("capability."+c.Capability.Name, IssueMissing,
			"say whether the point is static or in flight; static thrust is not cruise thrust "+
				"and nothing here converts between them")
	}
	for _, field := range []struct {
		name  string
		value Quantity
		want  Dimension
	}{
		{"speed", c.Capability.Speed, DimSpeed},
		{"density", c.Capability.Density, DimDensity},
		{"voltage", c.Capability.Voltage, DimVoltage},
		{"rpm", c.Capability.RPM, DimRotationRate},
		{"thrust", c.Capability.Thrust, DimForce},
		{"electrical_power", c.Capability.ElectricalPower, DimPower},
		{"current", c.Capability.Current, DimCurrent},
	} {
		if field.value.supplied() && field.value.dim != field.want {
			return Design{}, commandIssue("capability."+c.Capability.Name+"."+field.name, IssueInvalid,
				"expected a "+field.want.String()+" value but received "+field.value.dim.String())
		}
	}
	out := d.clone()
	for n := range out.Propulsion.Capabilities {
		if out.Propulsion.Capabilities[n].Name == c.Capability.Name {
			out.Propulsion.Capabilities[n] = c.Capability
			return out, nil
		}
	}
	out.Propulsion.Capabilities = append(out.Propulsion.Capabilities, c.Capability)
	return out, nil
}

// RemoveCapability drops a capability point. It refuses while a segment or a
// target still names it, because a check whose measurement has vanished is
// neither met, unmet nor meaningfully unknown.
type RemoveCapability struct {
	// Name identifies the point.
	Name string
}

// Label names the edit.
func (c RemoveCapability) Label() string { return "remove capability point " + c.Name }

func (c RemoveCapability) apply(d Design) (Design, error) {
	for _, target := range d.Propulsion.Targets {
		if target.Capability == c.Name {
			return Design{}, commandIssue("capability."+c.Name, IssueInvalid,
				"thrust target "+target.Name+" is stated at this point; revise or remove it first")
		}
	}
	for n := range d.Mission.Segments {
		if d.Mission.Segments[n].Capability == c.Name {
			return Design{}, commandIssue("capability."+c.Name, IssueInvalid,
				"mission segment "+d.Mission.Segments[n].Name+" is checked against this point; "+
					"revise or remove it first")
		}
	}
	out := d.clone()
	for n := range out.Propulsion.Capabilities {
		if out.Propulsion.Capabilities[n].Name != c.Name {
			continue
		}
		out.Propulsion.Capabilities = append(out.Propulsion.Capabilities[:n],
			out.Propulsion.Capabilities[n+1:]...)
		return out, nil
	}
	return Design{}, commandIssue("capability", IssueMissing,
		"the design defines no capability point named "+c.Name+"; it defines "+
			joinNames(d.Propulsion.capabilityNames()))
}

// SetThrustTarget adds a thrust-to-weight target or replaces the one with the
// same name.
type SetThrustTarget struct {
	// Target is the target to record.
	Target ThrustTarget
}

// Label names the edit.
func (c SetThrustTarget) Label() string { return "set thrust target " + c.Target.Name }

func (c SetThrustTarget) apply(d Design) (Design, error) {
	if c.Target.Name == "" {
		return Design{}, commandIssue("thrust_target", IssueMissing, "name the thrust target")
	}
	out := d.clone()
	for n := range out.Propulsion.Targets {
		if out.Propulsion.Targets[n].Name == c.Target.Name {
			out.Propulsion.Targets[n] = c.Target
			return out, nil
		}
	}
	out.Propulsion.Targets = append(out.Propulsion.Targets, c.Target)
	return out, nil
}

// RemoveThrustTarget drops the named thrust target.
type RemoveThrustTarget struct {
	// Name identifies the target.
	Name string
}

// Label names the edit.
func (c RemoveThrustTarget) Label() string { return "remove thrust target " + c.Name }

func (c RemoveThrustTarget) apply(d Design) (Design, error) {
	out := d.clone()
	for n := range out.Propulsion.Targets {
		if out.Propulsion.Targets[n].Name != c.Name {
			continue
		}
		out.Propulsion.Targets = append(out.Propulsion.Targets[:n], out.Propulsion.Targets[n+1:]...)
		return out, nil
	}
	return Design{}, commandIssue("thrust_target", IssueMissing,
		"the design states no thrust target named "+c.Name)
}

// SetBattery records the flight pack, or withdraws it when passed the zero
// Battery. It carries no mass: the pack's mass is a component, and Component
// names it.
type SetBattery struct {
	// Battery is the pack to record.
	Battery Battery
}

// Label names the edit.
func (c SetBattery) Label() string {
	if !c.Battery.supplied() {
		return "withdraw the flight pack"
	}
	return "set the flight pack"
}

func (c SetBattery) apply(d Design) (Design, error) {
	for _, field := range []struct {
		name  string
		value Quantity
		want  Dimension
	}{
		{"capacity", c.Battery.Capacity, DimCharge},
		{"nominal_voltage", c.Battery.NominalVoltage, DimVoltage},
		{"energy", c.Battery.Energy, DimEnergy},
		{"continuous_current_limit", c.Battery.ContinuousCurrentLimit, DimCurrent},
		{"peak_current_limit", c.Battery.PeakCurrentLimit, DimCurrent},
	} {
		if field.value.supplied() && field.value.dim != field.want {
			return Design{}, commandIssue("battery."+field.name, IssueInvalid,
				"expected a "+field.want.String()+" value but received "+field.value.dim.String())
		}
	}
	out := d.clone()
	out.Battery = c.Battery
	return out, nil
}

// SetAuxiliaryLoad adds an electrical load or replaces the one with the same
// name.
type SetAuxiliaryLoad struct {
	// Load is the draw to record.
	Load AuxiliaryLoad
}

// Label names the edit.
func (c SetAuxiliaryLoad) Label() string { return "set auxiliary load " + c.Load.Name }

func (c SetAuxiliaryLoad) apply(d Design) (Design, error) {
	if c.Load.Name == "" {
		return Design{}, commandIssue("auxiliary", IssueMissing,
			"name the load so its draw can be reported against it")
	}
	for _, field := range []struct {
		name  string
		value Quantity
	}{{"continuous", c.Load.Continuous}, {"peak", c.Load.Peak}} {
		if field.value.supplied() && field.value.dim != DimPower {
			return Design{}, commandIssue("auxiliary."+c.Load.Name+"."+field.name, IssueInvalid,
				"expected a power but received "+field.value.dim.String())
		}
	}
	out := d.clone()
	for n := range out.Auxiliary {
		if out.Auxiliary[n].Name == c.Load.Name {
			out.Auxiliary[n] = c.Load
			return out, nil
		}
	}
	out.Auxiliary = append(out.Auxiliary, c.Load)
	return out, nil
}

// RemoveAuxiliaryLoad drops the named electrical load.
type RemoveAuxiliaryLoad struct {
	// Name identifies the load.
	Name string
}

// Label names the edit.
func (c RemoveAuxiliaryLoad) Label() string { return "remove auxiliary load " + c.Name }

func (c RemoveAuxiliaryLoad) apply(d Design) (Design, error) {
	out := d.clone()
	for n := range out.Auxiliary {
		if out.Auxiliary[n].Name != c.Name {
			continue
		}
		out.Auxiliary = append(out.Auxiliary[:n], out.Auxiliary[n+1:]...)
		return out, nil
	}
	return Design{}, commandIssue("auxiliary", IssueMissing,
		"the design lists no auxiliary load named "+c.Name)
}

// SetMissionProfile records the mission's name, its provenance and its energy
// reserve, leaving the segments alone. The reserve lives here rather than on a
// segment because it is applied exactly once, to the whole mission.
type SetMissionProfile struct {
	// Name identifies the mission.
	Name string
	// Basis states where the mission came from.
	Basis string
	// ReserveBasis states where the reserve came from.
	ReserveBasis string
	// ReserveFraction is the fraction of the pack's usable energy held back.
	ReserveFraction float64
}

// Label names the edit.
func (c SetMissionProfile) Label() string { return "set the mission profile " + c.Name }

func (c SetMissionProfile) apply(d Design) (Design, error) {
	if c.ReserveFraction < 0 || c.ReserveFraction >= 1 || !isFinite(c.ReserveFraction) {
		return Design{}, commandIssue("mission.reserve", IssueInvalid,
			"the reserve is a fraction of the usable energy from 0 up to but not including 1, "+
				"received "+formatFloat(c.ReserveFraction))
	}
	out := d.clone()
	out.Mission.Name = c.Name
	out.Mission.Basis = c.Basis
	out.Mission.ReserveBasis = c.ReserveBasis
	out.Mission.ReserveFraction = c.ReserveFraction
	return out, nil
}

// SetMissionSegment adds a segment at the end of the mission, or replaces the
// one with the same name in place. Replacing in place keeps the mission's order,
// because a mission is a sequence and editing a leg is not reordering it.
type SetMissionSegment struct {
	// Segment is the leg to record.
	Segment MissionSegment
}

// Label names the edit.
func (c SetMissionSegment) Label() string { return "set mission segment " + c.Segment.Name }

func (c SetMissionSegment) apply(d Design) (Design, error) {
	if c.Segment.Name == "" {
		return Design{}, commandIssue("segment", IssueMissing,
			"name the segment so its result can be reported against it")
	}
	for _, field := range []struct {
		name  string
		value Quantity
		want  Dimension
	}{
		{"speed", c.Segment.Speed, DimSpeed},
		{"climb_angle", c.Segment.ClimbAngle, DimAngle},
		{"duration", c.Segment.Duration, DimTime},
		{"distance", c.Segment.Distance, DimLength},
		{"wind_along_track", c.Segment.WindAlongTrack, DimSpeed},
		{"entered_power", c.Segment.EnteredPower, DimPower},
	} {
		if field.value.supplied() && field.value.dim != field.want {
			return Design{}, commandIssue("segment."+c.Segment.Name+"."+field.name, IssueInvalid,
				"expected a "+field.want.String()+" value but received "+field.value.dim.String())
		}
	}
	out := d.clone()
	for n := range out.Mission.Segments {
		if out.Mission.Segments[n].Name == c.Segment.Name {
			out.Mission.Segments[n] = c.Segment
			return out, nil
		}
	}
	out.Mission.Segments = append(out.Mission.Segments, c.Segment)
	return out, nil
}

// RemoveMissionSegment drops the named segment.
type RemoveMissionSegment struct {
	// Name identifies the segment.
	Name string
}

// Label names the edit.
func (c RemoveMissionSegment) Label() string { return "remove mission segment " + c.Name }

func (c RemoveMissionSegment) apply(d Design) (Design, error) {
	out := d.clone()
	for n := range out.Mission.Segments {
		if out.Mission.Segments[n].Name != c.Name {
			continue
		}
		out.Mission.Segments = append(out.Mission.Segments[:n], out.Mission.Segments[n+1:]...)
		return out, nil
	}
	return Design{}, commandIssue("segment", IssueMissing,
		"the mission lists no segment named "+c.Name+"; it lists "+
			joinNames(d.Mission.segmentNames()))
}

// MoveMissionSegment moves a segment to a new position in the mission. It is a
// separate edit from setting one because reordering the legs is a different act
// from editing them: the same segments flown in a different order are a
// different mission, and the snapshot records the order for that reason.
type MoveMissionSegment struct {
	// Name identifies the segment.
	Name string
	// Index is the position to move it to, counting from zero.
	Index int
}

// Label names the edit.
func (c MoveMissionSegment) Label() string {
	return "move mission segment " + c.Name + " to position " + formatFloat(float64(c.Index))
}

func (c MoveMissionSegment) apply(d Design) (Design, error) {
	from := -1
	for n := range d.Mission.Segments {
		if d.Mission.Segments[n].Name == c.Name {
			from = n
			break
		}
	}
	if from < 0 {
		return Design{}, commandIssue("segment", IssueMissing,
			"the mission lists no segment named "+c.Name+"; it lists "+
				joinNames(d.Mission.segmentNames()))
	}
	if c.Index < 0 || c.Index >= len(d.Mission.Segments) {
		return Design{}, commandIssue("segment."+c.Name, IssueInvalid,
			"the mission has "+formatFloat(float64(len(d.Mission.Segments)))+
				" segments, so there is no position "+formatFloat(float64(c.Index)))
	}
	out := d.clone()
	segment := out.Mission.Segments[from]
	out.Mission.Segments = append(out.Mission.Segments[:from], out.Mission.Segments[from+1:]...)
	rest := append([]MissionSegment(nil), out.Mission.Segments[c.Index:]...)
	out.Mission.Segments = append(append(out.Mission.Segments[:c.Index], segment), rest...)
	return out, nil
}
