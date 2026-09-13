package api

import "yalb.aero/calculator"

// The Task 09 encoders. They are in their own file for the same reason the
// power families are in their own equation sets: the drag, propulsion, energy
// and mission surface is large enough that mixing it into the geometry encoder
// would make neither readable.
//
// Every array field is built as an empty slice rather than left nil, because
// the contract declares them as arrays and a nil slice marshals to null.
// TestArrayFieldsAreNeverNull holds that across the whole contract.

func encodeDragPolar(p calculator.DragPolar) *DragPolar {
	if p == (calculator.DragPolar{}) {
		return nil
	}
	return &DragPolar{
		CD0:              p.CD0,
		CD0Basis:         p.CD0Basis,
		OswaldEfficiency: p.OswaldEfficiency,
		EfficiencyBasis:  p.EfficiencyBasis,
		Configuration:    p.Configuration,
		CLValidMin:       p.CLValidMin,
		CLValidMax:       p.CLValidMax,
		Scope:            coefficientScopes.format(p.Scope),
		Evidence:         evidenceGrades.format(p.Evidence),
	}
}

func encodeCapability(c calculator.PropulsionCapability) Capability {
	return Capability{
		Name:            c.Name,
		Basis:           c.Basis,
		Note:            c.Note,
		Kind:            capabilityKinds.format(c.Kind),
		Speed:           encodeQuantity(c.Speed),
		Density:         encodeQuantity(c.Density),
		DensityBasis:    c.DensityBasis,
		Voltage:         encodeQuantity(c.Voltage),
		RPM:             encodeQuantity(c.RPM),
		Throttle:        c.Throttle,
		Thrust:          encodeQuantity(c.Thrust),
		ElectricalPower: encodeQuantity(c.ElectricalPower),
		Current:         encodeQuantity(c.Current),
		Evidence:        evidenceGrades.format(c.Evidence),
	}
}

func encodePropulsionLimits(l calculator.PropulsionLimits) *PropulsionLimits {
	if l == (calculator.PropulsionLimits{}) {
		return nil
	}
	return &PropulsionLimits{
		Basis:              l.Basis,
		MaxContinuousPower: encodeQuantity(l.MaxContinuousElectricalPower),
		MaxPeakPower:       encodeQuantity(l.MaxPeakElectricalPower),
		MaxRPM:             encodeQuantity(l.MaxRPM),
		MaxVoltage:         encodeQuantity(l.MaxVoltage),
		PropellerDiameter:  encodeQuantity(l.PropellerDiameter),
		PropellerHubHeight: encodeQuantity(l.PropellerHubHeight),
	}
}

func encodePropulsion(p calculator.Propulsion) *Propulsion {
	if p.Efficiency == (calculator.PropulsionEfficiency{}) &&
		len(p.Capabilities) == 0 && len(p.Targets) == 0 &&
		p.Limits == (calculator.PropulsionLimits{}) {
		return nil
	}
	out := &Propulsion{
		Efficiency:         p.Efficiency.Total,
		EfficiencyBasis:    p.Efficiency.Basis,
		EfficiencyEvidence: evidenceGrades.format(p.Efficiency.Evidence),
		Limits:             encodePropulsionLimits(p.Limits),
	}
	for n := range p.Capabilities {
		out.Capabilities = append(out.Capabilities, encodeCapability(p.Capabilities[n]))
	}
	for _, target := range p.Targets {
		out.Targets = append(out.Targets, ThrustTarget{
			Name:       target.Name,
			Basis:      target.Basis,
			Capability: target.Capability,
			Ratio:      target.Ratio,
			Priority:   priorities.format(target.Priority),
		})
	}
	return out
}

func encodeBattery(b calculator.Battery) *Battery {
	if b == (calculator.Battery{}) {
		return nil
	}
	return &Battery{
		Basis:                  b.Basis,
		Component:              b.Component,
		Mode:                   batteryEnergyModes.format(b.Mode),
		Capacity:               encodeQuantity(b.Capacity),
		NominalVoltage:         encodeQuantity(b.NominalVoltage),
		Energy:                 encodeQuantity(b.Energy),
		UsableFraction:         b.UsableFraction,
		ContinuousCurrentLimit: encodeQuantity(b.ContinuousCurrentLimit),
		PeakCurrentLimit:       encodeQuantity(b.PeakCurrentLimit),
		Evidence:               evidenceGrades.format(b.Evidence),
	}
}

func encodeAuxiliaryLoad(a calculator.AuxiliaryLoad) AuxiliaryLoad {
	return AuxiliaryLoad{
		Name:                a.Name,
		Basis:               a.Basis,
		Component:           a.Component,
		Continuous:          encodeQuantity(a.Continuous),
		Peak:                encodeQuantity(a.Peak),
		RegulatorEfficiency: a.RegulatorEfficiency,
		Side:                regulatorSides.format(a.Side),
		Evidence:            evidenceGrades.format(a.Evidence),
	}
}

func encodeMissionSegment(s calculator.MissionSegment) MissionSegment {
	return MissionSegment{
		Name:            s.Name,
		Case:            s.Case,
		Notes:           s.Notes,
		Kind:            segmentKinds.format(s.Kind),
		Model:           segmentModels.format(s.Model),
		Timing:          segmentTimings.format(s.Timing),
		Speed:           encodeQuantity(s.Speed),
		ClimbAngle:      encodeQuantity(s.ClimbAngle),
		Duration:        encodeQuantity(s.Duration),
		Distance:        encodeQuantity(s.Distance),
		WindAlongTrack:  encodeQuantity(s.WindAlongTrack),
		EnteredPower:    encodeQuantity(s.EnteredPower),
		EnteredBasis:    s.EnteredBasis,
		EnteredEvidence: evidenceGrades.format(s.EnteredEvidence),
		Efficiency:      s.Efficiency,
		EfficiencyBasis: s.EfficiencyBasis,
		Capability:      s.Capability,
	}
}

func encodeMission(m calculator.Mission) *Mission {
	if m.Name == "" && len(m.Segments) == 0 {
		return nil
	}
	out := &Mission{
		Name:            m.Name,
		Basis:           m.Basis,
		ReserveFraction: m.ReserveFraction,
		ReserveBasis:    m.ReserveBasis,
	}
	for n := range m.Segments {
		out.Segments = append(out.Segments, encodeMissionSegment(m.Segments[n]))
	}
	return out
}

func encodeElectricalBudget(b calculator.ElectricalBudget) ElectricalBudget {
	out := ElectricalBudget{
		Status:     resultStatuses.format(b.Status),
		Detail:     b.Detail,
		Evidence:   evidenceGrades.format(b.Evidence),
		Complete:   b.Complete,
		Continuous: encodeQuantity(b.Continuous),
		Peak:       encodeQuantity(b.Peak),
		Loads:      []AuxiliaryContribution{},
	}
	for n := range b.Loads {
		load := &b.Loads[n]
		out.Loads = append(out.Loads, AuxiliaryContribution{
			Name:       load.Name,
			Component:  load.Component,
			Detail:     load.Detail,
			Continuous: encodeQuantity(load.Continuous),
			Peak:       encodeQuantity(load.Peak),
			Side:       regulatorSides.format(load.Side),
			Evidence:   evidenceGrades.format(load.Evidence),
			Known:      load.Known,
		})
	}
	return out
}

func encodeSegmentPower(p calculator.SegmentPower) SegmentPower {
	return SegmentPower{
		Model:                segmentModels.format(p.Model),
		Status:               resultStatuses.format(p.Status),
		Detail:               p.Detail,
		Evidence:             evidenceGrades.format(p.Evidence),
		Traces:               encodeTraces(p.Traces),
		DynamicPressure:      encodeQuantity(p.DynamicPressure),
		Lift:                 encodeQuantity(p.Lift),
		Drag:                 encodeQuantity(p.Drag),
		Thrust:               encodeQuantity(p.Thrust),
		Propulsive:           encodeQuantity(p.Propulsive),
		PropulsiveElectrical: encodeQuantity(p.PropulsiveElectrical),
		Electrical:           encodeQuantity(p.Electrical),
		LiftCoefficient:      p.LiftCoefficient,
		DragCoefficient:      p.DragCoefficient,
		LiftToDrag:           p.LiftToDrag,
		ChainEfficiency:      p.ChainEfficiency,
	}
}

func encodeSegmentAvailability(a calculator.SegmentAvailability) SegmentAvailability {
	return SegmentAvailability{
		Capability:      a.Capability,
		Detail:          a.Detail,
		Status:          limitStatuses.format(a.Status),
		Evidence:        evidenceGrades.format(a.Evidence),
		Margin:          a.Margin,
		AvailableThrust: encodeQuantity(a.AvailableThrust),
		Trace:           encodeTrace(a.Trace),
	}
}

func encodeMissionResult(m calculator.MissionResult) MissionResult {
	out := MissionResult{
		Status:              resultStatuses.format(m.Status),
		EnergyStatus:        limitStatuses.format(m.EnergyStatus),
		Detail:              m.Detail,
		ReserveFraction:     m.ReserveFraction,
		Complete:            m.Complete,
		RequiredEnergy:      encodeQuantity(m.RequiredEnergy),
		UsableEnergy:        encodeQuantity(m.UsableEnergy),
		Budget:              encodeQuantity(m.Budget),
		TotalDuration:       encodeQuantity(m.TotalDuration),
		TotalDistance:       encodeQuantity(m.TotalDistance),
		PeakContinuousPower: encodeQuantity(m.PeakContinuousPower),
		Traces:              encodeTraces(m.Traces),
		Segments:            []SegmentResult{},
	}
	for n := range m.Segments {
		s := &m.Segments[n]
		out.Segments = append(out.Segments, SegmentResult{
			Name:         s.Name,
			Case:         s.Case,
			Kind:         segmentKinds.format(s.Kind),
			Status:       resultStatuses.format(s.Status),
			Detail:       s.Detail,
			Traces:       encodeTraces(s.Traces),
			GroundSpeed:  encodeQuantity(s.GroundSpeed),
			Duration:     encodeQuantity(s.Duration),
			Distance:     encodeQuantity(s.Distance),
			Energy:       encodeQuantity(s.Energy),
			Power:        encodeSegmentPower(s.Power),
			Availability: encodeSegmentAvailability(s.Availability),
		})
	}
	return out
}

func encodePowerFeasibility(f calculator.PowerFeasibility) PowerFeasibility {
	out := PowerFeasibility{
		Status:           resultStatuses.format(f.Status),
		Detail:           f.Detail,
		PeakDetail:       f.PeakDetail,
		ContinuousDemand: encodeQuantity(f.ContinuousDemand),
		PeakDemand:       encodeQuantity(f.PeakDemand),
		Checks:           []SupplyCheck{},
	}
	for n := range f.Checks {
		check := &f.Checks[n]
		out.Checks = append(out.Checks, SupplyCheck{
			Name:   check.Name,
			Detail: check.Detail,
			Status: limitStatuses.format(check.Status),
			Margin: check.Margin,
			Limit:  encodeQuantity(check.Limit),
			Actual: encodeQuantity(check.Actual),
		})
	}
	return out
}

func encodeThrustChecks(checks []calculator.ThrustCheck) []ThrustCheck {
	out := make([]ThrustCheck, 0, len(checks))
	for n := range checks {
		c := &checks[n]
		out = append(out, ThrustCheck{
			Name:       c.Name,
			Capability: c.Capability,
			Condition:  c.Condition,
			Detail:     c.Detail,
			Priority:   priorities.format(c.Priority),
			Evidence:   evidenceGrades.format(c.Evidence),
			Status:     limitStatuses.format(c.Status),
			Available:  c.Available,
			Target:     c.Target,
			Margin:     c.Margin,
			Trace:      encodeTrace(c.Trace),
		})
	}
	return out
}

// encodeTraces renders a trace slice, dropping the zero traces a step that ran
// no evaluation leaves behind.
func encodeTraces(traces []calculator.Trace) []Trace {
	out := make([]Trace, 0, len(traces))
	for n := range traces {
		if encoded := encodeTrace(traces[n]); encoded != nil {
			out = append(out, *encoded)
		}
	}
	return out
}

func encodePowerSearch(result calculator.PowerSizingResult, request Request) PowerSearchResponse {
	out := PowerSearchResponse{
		Request:             request,
		Settings:            encodePowerSearchSettings(result.Settings),
		Snapshot:            result.Snapshot,
		SettingsFingerprint: result.SettingsFingerprint,
		SolveMode:           solveModes.format(result.Mode),
		Detail:              result.Detail,
		HeldFixed:           encodeKeys(result.HeldFixed),
		Candidates:          []PowerSearchCandidate{},
		Intervals:           []PowerSearchInterval{},
		Unique:              result.Unique,
		Found:               result.Found,
	}
	for n := range result.Candidates {
		c := &result.Candidates[n]
		out.Candidates = append(out.Candidates, PowerSearchCandidate{
			Driver:        requireQuantity(c.Driver),
			Demand:        encodeQuantity(c.Demand),
			Status:        resultStatuses.format(c.Status),
			Feasibility:   limitStatuses.format(c.Feasibility),
			Detail:        c.Detail,
			Margin:        c.Margin,
			HasRequired:   c.HasRequired,
			WithinCeiling: c.WithinCeiling,
			Feasible:      c.Feasible,
		})
	}
	for n := range result.Intervals {
		i := &result.Intervals[n]
		out.Intervals = append(out.Intervals, PowerSearchInterval{
			First:      requireQuantity(i.First),
			Last:       requireQuantity(i.Last),
			BelowFirst: encodeQuantity(i.BelowFirst),
			AboveLast:  encodeQuantity(i.AboveLast),
			Detail:     i.Detail,
			OpenLow:    i.OpenLow,
			OpenHigh:   i.OpenHigh,
		})
	}
	return out
}

func encodePowerSearchSettings(s calculator.PowerSizingSettings) PowerSearchSettings {
	return PowerSearchSettings{
		Driver:  string(s.Driver),
		From:    requireQuantity(s.From),
		To:      requireQuantity(s.To),
		Ceiling: requireQuantity(s.Ceiling),
		Samples: s.Samples,
	}
}
