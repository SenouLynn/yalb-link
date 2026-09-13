package calculator

import "sort"

// canonical builds a deterministic text rendering of a design's inputs. It is
// the package's own serializer rather than a reflective one: the core cannot
// reach encoding/json, and a hand-written form makes it explicit which fields
// are part of a design's identity.
type canonical struct{ out string }

func (c *canonical) field(name, value string) {
	c.out += name + "=" + value + ";"
}

func (c *canonical) text(name, value string) { c.field(name, quoteText(value)) }

func (c *canonical) num(name string, value float64) { c.field(name, formatFloat(value)) }

func (c *canonical) code(name string, value int) { c.field(name, formatFloat(float64(value))) }

// qty records a quantity as its SI value and dimension, so two quantities that
// were entered in different units but mean the same thing snapshot identically.
func (c *canonical) qty(name string, q Quantity) {
	if !q.supplied() {
		c.field(name, "-")
		return
	}
	c.field(name, formatFloat(q.si)+q.dim.String())
}

// quoteText wraps a string so that a value containing the field separator
// cannot forge a field boundary. It escapes the backslash and the quote only,
// which is all that is needed for that, and keeps the escaping rule visible
// next to the format it protects.
func quoteText(s string) string {
	out := "\""
	for _, r := range s {
		if r == '"' || r == '\\' {
			out += "\\"
		}
		out += string(r)
	}
	return out + "\""
}

// Snapshot returns a canonical fingerprint of the design's inputs. Two designs
// with the same snapshot produce the same results, and an evaluation carries
// the snapshot it was computed from so a result can never be shown against
// inputs it did not come from.
//
// It covers inputs only. Nothing derived appears in it, and neither does the
// history that produced the design: reaching the same definition by a different
// route gives the same snapshot, which is what makes the entry points
// interchangeable rather than merely similar.
func (d Design) Snapshot() string {
	c := &canonical{}
	c.text("design", d.Name)
	c.code("configuration", int(d.Configuration))
	c.qty("mass", d.Mass)
	c.text("mass_basis", d.MassBasis)
	c.code("mass_mode", int(d.MassMode))
	d.canonicalizeComponents(c)
	d.canonicalizePower(c)
	d.Wing.canonicalize(c)
	d.Tail.canonicalize(c)
	d.canonicalizeCases(c)
	d.canonicalizeRequirements(c)
	return c.out
}

// canonicalizeComponents writes the components in name order, so that adding a
// component and then reordering the slice cannot change the fingerprint.
func (d Design) canonicalizeComponents(c *canonical) {
	items := append([]MassItem(nil), d.Components...)
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	c.code("components", len(items))
	for _, item := range items {
		prefix := "component[" + item.Name + "]"
		c.text(prefix+".name", item.Name)
		c.code(prefix+".role", int(item.Role))
		c.qty(prefix+".mass", item.Mass)
		c.text(prefix+".basis", item.Basis)
		c.qty(prefix+".x", item.Position.X)
		c.qty(prefix+".y", item.Position.Y)
		c.qty(prefix+".z", item.Position.Z)
	}
}

// canonicalizePower writes the Task 09 definition: the drag polar, the
// propulsion chain, the pack, the auxiliary loads and the mission. The
// name-keyed lists are sorted for the same reason the components are, so that
// reordering a slice cannot change the fingerprint.
func (d Design) canonicalizePower(c *canonical) {
	d.Polar.canonicalize(c)
	d.Propulsion.canonicalize(c)
	d.Battery.canonicalize(c)
	d.canonicalizeAuxiliary(c)
	d.Mission.canonicalize(c)
}

func (p DragPolar) canonicalize(c *canonical) {
	c.num("polar.cd0", p.CD0)
	c.text("polar.cd0_basis", p.CD0Basis)
	c.num("polar.oswald_efficiency", p.OswaldEfficiency)
	c.text("polar.oswald_basis", p.EfficiencyBasis)
	c.text("polar.configuration", p.Configuration)
	c.num("polar.cl_valid_min", p.CLValidMin)
	c.num("polar.cl_valid_max", p.CLValidMax)
	c.code("polar.scope", int(p.Scope))
	c.code("polar.evidence", int(p.Evidence))
}

func (p Propulsion) canonicalize(c *canonical) {
	c.num("propulsion.efficiency", p.Efficiency.Total)
	c.text("propulsion.efficiency_basis", p.Efficiency.Basis)
	c.code("propulsion.efficiency_evidence", int(p.Efficiency.Evidence))
	p.Limits.canonicalize(c)
	capabilities := append([]PropulsionCapability(nil), p.Capabilities...)
	sort.Slice(capabilities, func(i, j int) bool { return capabilities[i].Name < capabilities[j].Name })
	c.code("capabilities", len(capabilities))
	for n := range capabilities {
		capabilities[n].canonicalize(c)
	}
	targets := append([]ThrustTarget(nil), p.Targets...)
	sort.Slice(targets, func(i, j int) bool { return targets[i].Name < targets[j].Name })
	c.code("thrust_targets", len(targets))
	for _, target := range targets {
		prefix := "thrust_target[" + target.Name + "]"
		c.text(prefix+".name", target.Name)
		c.text(prefix+".basis", target.Basis)
		c.text(prefix+".capability", target.Capability)
		c.num(prefix+".ratio", target.Ratio)
		c.code(prefix+".priority", int(target.Priority))
	}
}

func (l PropulsionLimits) canonicalize(c *canonical) {
	c.text("propulsion.limits_basis", l.Basis)
	c.qty("propulsion.max_continuous_power", l.MaxContinuousElectricalPower)
	c.qty("propulsion.max_peak_power", l.MaxPeakElectricalPower)
	c.qty("propulsion.max_rpm", l.MaxRPM)
	c.qty("propulsion.max_voltage", l.MaxVoltage)
	c.qty("propulsion.propeller_diameter", l.PropellerDiameter)
	c.qty("propulsion.propeller_hub_height", l.PropellerHubHeight)
}

func (p PropulsionCapability) canonicalize(c *canonical) {
	prefix := "capability[" + p.Name + "]"
	c.text(prefix+".name", p.Name)
	c.text(prefix+".basis", p.Basis)
	c.text(prefix+".note", p.Note)
	c.code(prefix+".kind", int(p.Kind))
	c.qty(prefix+".speed", p.Speed)
	c.qty(prefix+".density", p.Density)
	c.text(prefix+".density_basis", p.DensityBasis)
	c.qty(prefix+".voltage", p.Voltage)
	c.qty(prefix+".rpm", p.RPM)
	c.num(prefix+".throttle", p.Throttle)
	c.qty(prefix+".thrust", p.Thrust)
	c.qty(prefix+".electrical_power", p.ElectricalPower)
	c.qty(prefix+".current", p.Current)
	c.code(prefix+".evidence", int(p.Evidence))
}

func (b Battery) canonicalize(c *canonical) {
	c.text("battery.basis", b.Basis)
	c.text("battery.component", b.Component)
	c.code("battery.mode", int(b.Mode))
	c.qty("battery.capacity", b.Capacity)
	c.qty("battery.nominal_voltage", b.NominalVoltage)
	c.qty("battery.energy", b.Energy)
	c.num("battery.usable_fraction", b.UsableFraction)
	c.qty("battery.continuous_current_limit", b.ContinuousCurrentLimit)
	c.qty("battery.peak_current_limit", b.PeakCurrentLimit)
	c.code("battery.evidence", int(b.Evidence))
}

func (d Design) canonicalizeAuxiliary(c *canonical) {
	loads := append([]AuxiliaryLoad(nil), d.Auxiliary...)
	sort.Slice(loads, func(i, j int) bool { return loads[i].Name < loads[j].Name })
	c.code("auxiliary", len(loads))
	for _, load := range loads {
		prefix := "auxiliary[" + load.Name + "]"
		c.text(prefix+".name", load.Name)
		c.text(prefix+".basis", load.Basis)
		c.text(prefix+".component", load.Component)
		c.qty(prefix+".continuous", load.Continuous)
		c.qty(prefix+".peak", load.Peak)
		c.num(prefix+".regulator_efficiency", load.RegulatorEfficiency)
		c.code(prefix+".side", int(load.Side))
		c.code(prefix+".evidence", int(load.Evidence))
	}
}

// canonicalize writes the mission. Segments keep their stated order rather than
// being sorted: a mission is a sequence, and reordering its legs is a different
// mission even when every leg is unchanged.
func (m Mission) canonicalize(c *canonical) {
	c.text("mission.name", m.Name)
	c.text("mission.basis", m.Basis)
	c.num("mission.reserve", m.ReserveFraction)
	c.text("mission.reserve_basis", m.ReserveBasis)
	c.code("mission.segments", len(m.Segments))
	for n := range m.Segments {
		m.Segments[n].canonicalize(c, n)
	}
}

func (s MissionSegment) canonicalize(c *canonical, index int) {
	prefix := "segment[" + formatFloat(float64(index)) + "]"
	c.text(prefix+".name", s.Name)
	c.text(prefix+".case", s.Case)
	c.text(prefix+".notes", s.Notes)
	c.code(prefix+".kind", int(s.Kind))
	c.code(prefix+".model", int(s.Model))
	c.code(prefix+".timing", int(s.Timing))
	c.qty(prefix+".speed", s.Speed)
	c.qty(prefix+".climb_angle", s.ClimbAngle)
	c.qty(prefix+".duration", s.Duration)
	c.qty(prefix+".distance", s.Distance)
	c.qty(prefix+".wind_along_track", s.WindAlongTrack)
	c.qty(prefix+".entered_power", s.EnteredPower)
	c.text(prefix+".entered_basis", s.EnteredBasis)
	c.code(prefix+".entered_evidence", int(s.EnteredEvidence))
	c.num(prefix+".efficiency", s.Efficiency)
	c.text(prefix+".efficiency_basis", s.EfficiencyBasis)
	c.text(prefix+".capability", s.Capability)
}

func (def WingDefinition) canonicalize(c *canonical) {
	c.text("wing", def.Name)
	c.code("shape", int(def.Drivers.Shape))
	c.qty("wing.span", def.Drivers.Span)
	c.qty("wing.area", def.Drivers.Area)
	c.qty("wing.root_chord", def.Drivers.RootChord)
	c.num("wing.aspect_ratio", def.Drivers.AspectRatio)
	c.num("wing.taper_ratio", def.Drivers.TaperRatio)
	c.qty("wing.sweep", def.Sweep)
	c.num("wing.sweep_reference", def.SweepReference)
	c.qty("wing.dihedral", def.Dihedral)
	c.qty("wing.twist", def.Twist)
	c.qty("wing.incidence", def.Incidence)
	c.qty("wing.body_width", def.BodyWidth)
	c.code("wing.area_basis", int(def.AreaBasis))
	c.code("wing.dihedral_mode", int(def.DihedralMode))
	def.RootAirfoil.canonicalize(c, "wing.root_airfoil")
	def.TipAirfoil.canonicalize(c, "wing.tip_airfoil")
}

func (a Airfoil) canonicalize(c *canonical, prefix string) {
	c.text(prefix+".designation", a.Designation)
	c.text(prefix+".evidence", a.Evidence)
	c.num(prefix+".thickness_ratio", a.ThicknessRatio)
}

func (t TailGeometry) canonicalize(c *canonical) {
	t.Horizontal.canonicalize(c, "tail.horizontal")
	t.Vertical.canonicalize(c, "tail.vertical")
	c.qty("tail.vtail.panel_area", t.VTail.PanelArea)
	c.qty("tail.vtail.panel_span", t.VTail.PanelSpan)
	c.qty("tail.vtail.cant", t.VTail.Cant)
	c.qty("tail.vtail.arm", t.VTail.Arm)
}

func (s TailSurface) canonicalize(c *canonical, prefix string) {
	c.qty(prefix+".area", s.Area)
	c.qty(prefix+".span", s.Span)
	c.qty(prefix+".arm", s.Arm)
}

// canonicalizeCases writes the cases in name order, so that adding a case and
// then reordering the slice cannot change the fingerprint.
func (d Design) canonicalizeCases(c *canonical) {
	cases := append([]DesignCase(nil), d.Cases...)
	sort.Slice(cases, func(i, j int) bool { return cases[i].Case.Name < cases[j].Case.Name })
	c.code("cases", len(cases))
	for n := range cases {
		dc := &cases[n]
		prefix := "case[" + dc.Case.Name + "]"
		c.text(prefix+".name", dc.Case.Name)
		c.text(prefix+".configuration", dc.Case.Configuration)
		c.qty(prefix+".density", dc.Case.Density)
		c.text(prefix+".density_basis", dc.Case.DensityBasis)
		c.qty(prefix+".viscosity", dc.Case.Viscosity)
		c.text(prefix+".viscosity_basis", dc.Case.ViscosityBasis)
		c.num(prefix+".load_factor", dc.Case.LoadFactor)
		c.num(prefix+".clmax", dc.Case.CLmax.Max)
		c.code(prefix+".clmax_scope", int(dc.Case.CLmax.Scope))
		c.text(prefix+".clmax_basis", dc.Case.CLmax.Basis)
		c.code(prefix+".clmax_evidence", int(dc.CLmaxEvidence))
		c.code(prefix+".priority", int(dc.Priority))
	}
}

// canonicalizeRequirements writes the requirements in name order, for the same
// reason the cases are sorted.
func (d Design) canonicalizeRequirements(c *canonical) {
	requirements := append([]Requirement(nil), d.Requirements...)
	sort.Slice(requirements, func(i, j int) bool { return requirements[i].Name < requirements[j].Name })
	c.code("requirements", len(requirements))
	for _, r := range requirements {
		prefix := "requirement[" + r.Name + "]"
		c.text(prefix+".name", r.Name)
		c.code(prefix+".subject", int(r.Subject))
		c.code(prefix+".priority", int(r.Priority))
		c.qty(prefix+".minimum", r.Minimum)
		c.qty(prefix+".maximum", r.Maximum)
		c.num(prefix+".margin", r.Margin)
		c.text(prefix+".basis", r.Basis)
		names := append([]string(nil), r.Cases...)
		sort.Strings(names)
		c.text(prefix+".cases", joinNames(names))
	}
}
