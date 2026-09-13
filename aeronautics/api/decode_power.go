package api

import (
	"strconv"

	"yalb.aero/calculator"
)

// Payload limits for the Task 09 lists. They live beside the others so every
// adapter is bounded the same way, and they are published in the discovery
// document so a client can respect them instead of finding them by refusal.
const (
	// MaxCapabilities is the number of propulsion capability points one design
	// may carry.
	MaxCapabilities = 32
	// MaxThrustTargets is the number of thrust-to-weight targets.
	MaxThrustTargets = 16
	// MaxAuxiliaryLoads is the number of auxiliary electrical loads.
	MaxAuxiliaryLoads = 64
	// MaxMissionSegments is the number of legs one mission may carry.
	MaxMissionSegments = 32
)

// The Task 09 decoders. Structural completeness is not checked here: a polar
// with no basis yet, a segment with no speed yet and a pack with no usable
// fraction yet are all designs in progress, and the core's own validation
// reports exactly which field is missing. What this layer reads is shape.

func (d *decoder) dragPolar(field string, p *DragPolar) calculator.DragPolar {
	if p == nil {
		return calculator.DragPolar{}
	}
	return calculator.DragPolar{
		CD0:              d.number(field+".cd0", p.CD0),
		CD0Basis:         p.CD0Basis,
		OswaldEfficiency: d.number(field+".oswaldEfficiency", p.OswaldEfficiency),
		EfficiencyBasis:  p.EfficiencyBasis,
		Configuration:    p.Configuration,
		CLValidMin:       d.number(field+".clValidMin", p.CLValidMin),
		CLValidMax:       d.number(field+".clValidMax", p.CLValidMax),
		Scope:            enumOrEmpty(d, coefficientScopes, field+".scope", p.Scope),
		Evidence:         enumOrEmpty(d, evidenceGrades, field+".evidence", p.Evidence),
	}
}

func (d *decoder) capability(field string, c Capability) calculator.PropulsionCapability {
	return calculator.PropulsionCapability{
		Name:            c.Name,
		Basis:           c.Basis,
		Note:            c.Note,
		Kind:            enumOrEmpty(d, capabilityKinds, field+".kind", c.Kind),
		Speed:           d.quantity(field+".speed", c.Speed),
		Density:         d.quantity(field+".density", c.Density),
		DensityBasis:    c.DensityBasis,
		Voltage:         d.quantity(field+".voltage", c.Voltage),
		RPM:             d.quantity(field+".rpm", c.RPM),
		Throttle:        d.number(field+".throttle", c.Throttle),
		Thrust:          d.quantity(field+".thrust", c.Thrust),
		ElectricalPower: d.quantity(field+".electricalPower", c.ElectricalPower),
		Current:         d.quantity(field+".current", c.Current),
		Evidence:        enumOrEmpty(d, evidenceGrades, field+".evidence", c.Evidence),
	}
}

func (d *decoder) propulsionLimits(field string, l *PropulsionLimits) calculator.PropulsionLimits {
	if l == nil {
		return calculator.PropulsionLimits{}
	}
	return calculator.PropulsionLimits{
		Basis:                        l.Basis,
		MaxContinuousElectricalPower: d.quantity(field+".maxContinuousPower", l.MaxContinuousPower),
		MaxPeakElectricalPower:       d.quantity(field+".maxPeakPower", l.MaxPeakPower),
		MaxRPM:                       d.quantity(field+".maxRpm", l.MaxRPM),
		MaxVoltage:                   d.quantity(field+".maxVoltage", l.MaxVoltage),
		PropellerDiameter:            d.quantity(field+".propellerDiameter", l.PropellerDiameter),
		PropellerHubHeight:           d.quantity(field+".propellerHubHeight", l.PropellerHubHeight),
	}
}

func (d *decoder) propulsion(field string, p *Propulsion) calculator.Propulsion {
	if p == nil {
		return calculator.Propulsion{}
	}
	out := calculator.Propulsion{
		Efficiency: calculator.PropulsionEfficiency{
			Total:    d.number(field+".efficiency", p.Efficiency),
			Basis:    p.EfficiencyBasis,
			Evidence: enumOrEmpty(d, evidenceGrades, field+".efficiencyEvidence", p.EfficiencyEvidence),
		},
		Limits: d.propulsionLimits(field+".limits", p.Limits),
	}
	if len(p.Capabilities) > MaxCapabilities {
		d.add(field+".capabilities", "unsupported",
			"a design may carry at most "+strconv.Itoa(MaxCapabilities)+" capability points, "+
				"received "+strconv.Itoa(len(p.Capabilities)))
		return out
	}
	if len(p.Targets) > MaxThrustTargets {
		d.add(field+".targets", "unsupported",
			"a design may carry at most "+strconv.Itoa(MaxThrustTargets)+" thrust targets, "+
				"received "+strconv.Itoa(len(p.Targets)))
		return out
	}
	for n := range p.Capabilities {
		out.Capabilities = append(out.Capabilities,
			d.capability(field+".capabilities["+strconv.Itoa(n)+"]", p.Capabilities[n]))
	}
	for n := range p.Targets {
		target := p.Targets[n]
		out.Targets = append(out.Targets, calculator.ThrustTarget{
			Name:       target.Name,
			Basis:      target.Basis,
			Capability: target.Capability,
			Ratio:      d.number(field+".targets["+strconv.Itoa(n)+"].ratio", target.Ratio),
			Priority:   enumOrEmpty(d, priorities, field+".targets["+strconv.Itoa(n)+"].priority", target.Priority),
		})
	}
	return out
}

func (d *decoder) battery(field string, b *Battery) calculator.Battery {
	if b == nil {
		return calculator.Battery{}
	}
	return calculator.Battery{
		Basis:                  b.Basis,
		Component:              b.Component,
		Mode:                   enumOrEmpty(d, batteryEnergyModes, field+".mode", b.Mode),
		Capacity:               d.quantity(field+".capacity", b.Capacity),
		NominalVoltage:         d.quantity(field+".nominalVoltage", b.NominalVoltage),
		Energy:                 d.quantity(field+".energy", b.Energy),
		UsableFraction:         d.number(field+".usableFraction", b.UsableFraction),
		ContinuousCurrentLimit: d.quantity(field+".continuousCurrentLimit", b.ContinuousCurrentLimit),
		PeakCurrentLimit:       d.quantity(field+".peakCurrentLimit", b.PeakCurrentLimit),
		Evidence:               enumOrEmpty(d, evidenceGrades, field+".evidence", b.Evidence),
	}
}

func (d *decoder) auxiliaryLoad(field string, a AuxiliaryLoad) calculator.AuxiliaryLoad {
	return calculator.AuxiliaryLoad{
		Name:                a.Name,
		Basis:               a.Basis,
		Component:           a.Component,
		Continuous:          d.quantity(field+".continuous", a.Continuous),
		Peak:                d.quantity(field+".peak", a.Peak),
		RegulatorEfficiency: d.number(field+".regulatorEfficiency", a.RegulatorEfficiency),
		Side:                enumOrEmpty(d, regulatorSides, field+".side", a.Side),
		Evidence:            enumOrEmpty(d, evidenceGrades, field+".evidence", a.Evidence),
	}
}

func (d *decoder) missionSegment(field string, s MissionSegment) calculator.MissionSegment {
	return calculator.MissionSegment{
		Name:            s.Name,
		Case:            s.Case,
		Notes:           s.Notes,
		Kind:            enumOrEmpty(d, segmentKinds, field+".kind", s.Kind),
		Model:           enumOrEmpty(d, segmentModels, field+".model", s.Model),
		Timing:          enumOrEmpty(d, segmentTimings, field+".timing", s.Timing),
		Speed:           d.quantity(field+".speed", s.Speed),
		ClimbAngle:      d.quantity(field+".climbAngle", s.ClimbAngle),
		Duration:        d.quantity(field+".duration", s.Duration),
		Distance:        d.quantity(field+".distance", s.Distance),
		WindAlongTrack:  d.quantity(field+".windAlongTrack", s.WindAlongTrack),
		EnteredPower:    d.quantity(field+".enteredPower", s.EnteredPower),
		EnteredBasis:    s.EnteredBasis,
		EnteredEvidence: enumOrEmpty(d, evidenceGrades, field+".enteredEvidence", s.EnteredEvidence),
		Efficiency:      d.number(field+".efficiency", s.Efficiency),
		EfficiencyBasis: s.EfficiencyBasis,
		Capability:      s.Capability,
	}
}

func (d *decoder) mission(field string, m *Mission) calculator.Mission {
	if m == nil {
		return calculator.Mission{}
	}
	out := calculator.Mission{
		Name:            m.Name,
		Basis:           m.Basis,
		ReserveFraction: d.number(field+".reserveFraction", m.ReserveFraction),
		ReserveBasis:    m.ReserveBasis,
	}
	if len(m.Segments) > MaxMissionSegments {
		d.add(field+".segments", "unsupported",
			"a mission may carry at most "+strconv.Itoa(MaxMissionSegments)+" segments, received "+
				strconv.Itoa(len(m.Segments)))
		return out
	}
	for n := range m.Segments {
		out.Segments = append(out.Segments,
			d.missionSegment(field+".segments["+strconv.Itoa(n)+"]", m.Segments[n]))
	}
	return out
}

// powerDefinition reads the Task 09 half of a design.
func (d *decoder) powerDefinition(design Design, out *calculator.Design) {
	out.Polar = d.dragPolar("design.polar", design.Polar)
	out.Propulsion = d.propulsion("design.propulsion", design.Propulsion)
	out.Battery = d.battery("design.battery", design.Battery)
	out.Mission = d.mission("design.mission", design.Mission)
	if len(design.Auxiliary) > MaxAuxiliaryLoads {
		d.add("design.auxiliary", "unsupported",
			"a design may carry at most "+strconv.Itoa(MaxAuxiliaryLoads)+" auxiliary loads, "+
				"received "+strconv.Itoa(len(design.Auxiliary)))
		return
	}
	for n := range design.Auxiliary {
		out.Auxiliary = append(out.Auxiliary,
			d.auxiliaryLoad("design.auxiliary["+strconv.Itoa(n)+"]", design.Auxiliary[n]))
	}
}

// powerSearchSettings converts a search request's settings. Whether the design
// holds the named driver, and whether the range and sample count are usable,
// are the core's answers: this reads the shape and hands the rest over.
func (d *decoder) powerSearchSettings(field string, s PowerSearchSettings) calculator.PowerSizingSettings {
	driver, issue := parseSweepDriverKey(field+".driver", s.Driver)
	d.take(issue)
	return calculator.PowerSizingSettings{
		Driver:  driver,
		From:    d.quantity(field+".from", &s.From),
		To:      d.quantity(field+".to", &s.To),
		Ceiling: d.quantity(field+".ceiling", &s.Ceiling),
		Samples: s.Samples,
	}
}
