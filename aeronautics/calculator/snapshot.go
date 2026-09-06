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
	d.Wing.canonicalize(c)
	d.Tail.canonicalize(c)
	d.canonicalizeCases(c)
	d.canonicalizeRequirements(c)
	return c.out
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
