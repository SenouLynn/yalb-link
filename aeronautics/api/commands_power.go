package api

import "yalb.aero/calculator"

// The Task 09 command kinds. Like the others they are stable identifiers, so a
// saved worksheet or an MCP transcript can record which edit was made.
const (
	CmdSetDragPolar         = "set-drag-polar"
	CmdSetPropulsionEff     = "set-propulsion-efficiency"
	CmdSetPropulsionLimits  = "set-propulsion-limits"
	CmdSetCapability        = "set-capability"
	CmdRemoveCapability     = "remove-capability"
	CmdSetThrustTarget      = "set-thrust-target"
	CmdRemoveThrustTarget   = "remove-thrust-target"
	CmdSetBattery           = "set-battery"
	CmdSetAuxiliaryLoad     = "set-auxiliary-load"
	CmdRemoveAuxiliaryLoad  = "remove-auxiliary-load"
	CmdSetMissionProfile    = "set-mission-profile"
	CmdSetMissionSegment    = "set-mission-segment"
	CmdRemoveMissionSegment = "remove-mission-segment"
	CmdMoveMissionSegment   = "move-mission-segment"
)

// The union fields the Task 09 commands use.
const (
	fieldPolar      commandField = "polar"
	fieldEfficiency commandField = "efficiency"
	fieldLimits     commandField = "limits"
	fieldCapability commandField = "capability"
	fieldTarget     commandField = "target"
	fieldBattery    commandField = "battery"
	fieldLoad       commandField = "load"
	fieldProfile    commandField = "profile"
	fieldSegment    commandField = "segment"
	fieldIndex      commandField = "index"
)

// Efficiency is the chain efficiency and its evidence, carried as one value so
// the number and the basis behind it cannot be set separately.
type Efficiency struct {
	Basis    string  `json:"basis"`
	Evidence string  `json:"evidence,omitempty"`
	Total    float64 `json:"total"`
}

// MissionProfile is the mission's header: its name, its provenance and its
// energy reserve, without its segments. The reserve lives here because it is
// applied exactly once, to the whole mission.
type MissionProfile struct {
	Name            string  `json:"name"`
	Basis           string  `json:"basis"`
	ReserveBasis    string  `json:"reserveBasis,omitempty"`
	ReserveFraction float64 `json:"reserveFraction"`
}

// powerCommandSpecs are the field rules for the Task 09 edits, merged into
// commandSpecs at init so the two sets share one validation path.
var powerCommandSpecs = map[string]commandSpec{
	// An absent polar withdraws it, which takes every polar-based power result
	// back rather than leaving the previous coefficients under new geometry.
	CmdSetDragPolar:        {Fields: []commandField{fieldPolar}, Optional: []commandField{fieldPolar}},
	CmdSetPropulsionEff:    {Fields: []commandField{fieldEfficiency}},
	CmdSetPropulsionLimits: {Fields: []commandField{fieldLimits}},
	CmdSetCapability:       {Fields: []commandField{fieldCapability}},
	CmdRemoveCapability:    {Fields: []commandField{fieldName}},
	CmdSetThrustTarget:     {Fields: []commandField{fieldTarget}},
	CmdRemoveThrustTarget:  {Fields: []commandField{fieldName}},
	// An absent pack withdraws it.
	CmdSetBattery:           {Fields: []commandField{fieldBattery}, Optional: []commandField{fieldBattery}},
	CmdSetAuxiliaryLoad:     {Fields: []commandField{fieldLoad}},
	CmdRemoveAuxiliaryLoad:  {Fields: []commandField{fieldName}},
	CmdSetMissionProfile:    {Fields: []commandField{fieldProfile}},
	CmdSetMissionSegment:    {Fields: []commandField{fieldSegment}},
	CmdRemoveMissionSegment: {Fields: []commandField{fieldName}},
	// Index 0 is a legitimate position, so it is optional in the shape check and
	// the core reports an out-of-range one.
	CmdMoveMissionSegment: {Fields: []commandField{fieldName, fieldIndex}, Optional: []commandField{fieldIndex}},
}

func init() {
	for kind, spec := range powerCommandSpecs {
		commandSpecs[kind] = spec
	}
}

// powerCommandFields reports which Task 09 union fields a command carries. It
// is merged into Command.present so one shape check covers the whole union.
func (c Command) powerCommandFields() map[commandField]bool {
	return map[commandField]bool{
		fieldPolar:      c.Polar != nil,
		fieldEfficiency: c.Efficiency != nil,
		fieldLimits:     c.Limits != nil,
		fieldCapability: c.Capability != nil,
		fieldTarget:     c.Target != nil,
		fieldBattery:    c.Battery != nil,
		fieldLoad:       c.Load != nil,
		fieldProfile:    c.Profile != nil,
		fieldSegment:    c.Segment != nil,
		fieldIndex:      c.Index != 0,
	}
}

// buildPowerCommand maps one checked Task 09 command onto its core value.
func (d *decoder) buildPowerCommand(field string, c Command) calculator.Command {
	switch c.Kind {
	case CmdSetDragPolar:
		return calculator.SetDragPolar{Polar: d.dragPolar(field+".polar", c.Polar)}
	case CmdSetPropulsionEff:
		if c.Efficiency == nil {
			return nil
		}
		return calculator.SetPropulsionEfficiency{Efficiency: calculator.PropulsionEfficiency{
			Total:    d.number(field+".efficiency.total", c.Efficiency.Total),
			Basis:    c.Efficiency.Basis,
			Evidence: enumOrEmpty(d, evidenceGrades, field+".efficiency.evidence", c.Efficiency.Evidence),
		}}
	case CmdSetPropulsionLimits:
		return calculator.SetPropulsionLimits{Limits: d.propulsionLimits(field+".limits", c.Limits)}
	case CmdSetCapability:
		if c.Capability == nil {
			return nil
		}
		return calculator.SetCapability{Capability: d.capability(field+".capability", *c.Capability)}
	case CmdRemoveCapability:
		return calculator.RemoveCapability{Name: c.Name}
	case CmdSetThrustTarget:
		if c.Target == nil {
			return nil
		}
		return calculator.SetThrustTarget{Target: calculator.ThrustTarget{
			Name:       c.Target.Name,
			Basis:      c.Target.Basis,
			Capability: c.Target.Capability,
			Ratio:      d.number(field+".target.ratio", c.Target.Ratio),
			Priority:   enumOrEmpty(d, priorities, field+".target.priority", c.Target.Priority),
		}}
	case CmdRemoveThrustTarget:
		return calculator.RemoveThrustTarget{Name: c.Name}
	default:
		return d.buildMissionCommand(field, c)
	}
}

// buildMissionCommand maps the pack, load and mission edits, keeping the switch
// above within a readable size.
func (d *decoder) buildMissionCommand(field string, c Command) calculator.Command {
	switch c.Kind {
	case CmdSetBattery:
		return calculator.SetBattery{Battery: d.battery(field+".battery", c.Battery)}
	case CmdSetAuxiliaryLoad:
		if c.Load == nil {
			return nil
		}
		return calculator.SetAuxiliaryLoad{Load: d.auxiliaryLoad(field+".load", *c.Load)}
	case CmdRemoveAuxiliaryLoad:
		return calculator.RemoveAuxiliaryLoad{Name: c.Name}
	case CmdSetMissionProfile:
		if c.Profile == nil {
			return nil
		}
		return calculator.SetMissionProfile{
			Name:            c.Profile.Name,
			Basis:           c.Profile.Basis,
			ReserveBasis:    c.Profile.ReserveBasis,
			ReserveFraction: d.number(field+".profile.reserveFraction", c.Profile.ReserveFraction),
		}
	case CmdSetMissionSegment:
		if c.Segment == nil {
			return nil
		}
		return calculator.SetMissionSegment{Segment: d.missionSegment(field+".segment", *c.Segment)}
	case CmdRemoveMissionSegment:
		return calculator.RemoveMissionSegment{Name: c.Name}
	case CmdMoveMissionSegment:
		return calculator.MoveMissionSegment{Name: c.Name, Index: c.Index}
	default:
		return nil
	}
}
