package api

import (
	"sort"

	"yalb.aero/calculator"
)

// The command kinds the contract accepts. They are stable identifiers, so a
// saved worksheet or an MCP transcript can record which edit was made.
const (
	CmdSetMass                = "set-mass"
	CmdSetDriver              = "set-driver"
	CmdPromoteDriver          = "promote-driver"
	CmdSetTaperRatio          = "set-taper-ratio"
	CmdSetRequirement         = "set-requirement"
	CmdRemoveRequirement      = "remove-requirement"
	CmdSetRequirementPriority = "set-requirement-priority"
	CmdSetCase                = "set-case"
	CmdRemoveCase             = "remove-case"
	CmdSetCasePriority        = "set-case-priority"
	CmdSetCaseCLmax           = "set-case-clmax"
	CmdSizeAtStallLimit       = "size-at-stall-limit"
	CmdSetPlanformShape       = "set-planform-shape"
	CmdSetWingAngles          = "set-wing-angles"
	CmdSetConfiguration       = "set-configuration"
)

// commandField names one field of the flat Command union. The names are the
// JSON field names, so a message about an unexpected field points at the field
// the client actually sent.
type commandField string

const (
	fieldBasis       commandField = "basis"
	fieldKey         commandField = "key"
	fieldPromote     commandField = "promote"
	fieldRelease     commandField = "release"
	fieldName        commandField = "name"
	fieldPriority    commandField = "priority"
	fieldHold        commandField = "hold"
	fieldMass        commandField = "mass"
	fieldValue       commandField = "value"
	fieldRequirement commandField = "requirement"
	fieldCase        commandField = "case"
	fieldCLmax       commandField = "clmax"
	fieldScope       commandField = "scope"
	fieldRatio       commandField = "ratio"
	fieldAngles      commandField = "angles"
	fieldTail        commandField = "tail"
	fieldShape       commandField = "shape"
	fieldConfig      commandField = "configuration"
)

// commandSpec lists the fields one kind uses. A field outside the list is
// rejected rather than ignored: a client that sends "hold" to a mass edit has
// misunderstood the contract, and silently dropping it would let that
// misunderstanding produce a plausible wrong answer.
//
// Optional records the fields that may be absent. Everything else in Fields is
// required, so a kind cannot be half-specified either.
type commandSpec struct {
	Fields   []commandField
	Optional []commandField
}

var commandSpecs = map[string]commandSpec{
	// Both are optional: the basis is reported by the design's own validation
	// rather than at the keystroke, so the number and its provenance can be
	// typed in either order, and an absent mass withdraws it.
	CmdSetMass: {Fields: []commandField{fieldMass, fieldBasis}, Optional: []commandField{fieldMass, fieldBasis}},
	// An absent value withdraws the driver, which is a different act from
	// entering zero and is how a value is taken back.
	CmdSetDriver:              {Fields: []commandField{fieldKey, fieldValue}, Optional: []commandField{fieldValue}},
	CmdPromoteDriver:          {Fields: []commandField{fieldPromote, fieldRelease, fieldValue}, Optional: []commandField{fieldRelease}},
	CmdSetTaperRatio:          {Fields: []commandField{fieldRatio}},
	CmdSetRequirement:         {Fields: []commandField{fieldRequirement}},
	CmdRemoveRequirement:      {Fields: []commandField{fieldName}},
	CmdSetRequirementPriority: {Fields: []commandField{fieldName, fieldPriority}},
	CmdSetCase:                {Fields: []commandField{fieldCase}},
	CmdRemoveCase:             {Fields: []commandField{fieldName}},
	CmdSetCasePriority:        {Fields: []commandField{fieldName, fieldPriority}},
	// The coefficient may be absent: withdrawing the evidence is a supported
	// edit, and it is what makes the results resting on it unknown.
	CmdSetCaseCLmax:     {Fields: []commandField{fieldName, fieldCLmax}, Optional: []commandField{fieldCLmax}},
	CmdSizeAtStallLimit: {Fields: []commandField{fieldHold, fieldScope}},
	CmdSetPlanformShape: {Fields: []commandField{fieldShape, fieldRatio}},
	CmdSetWingAngles:    {Fields: []commandField{fieldAngles}},
	// A flying wing carries no tail, so the description may be absent.
	CmdSetConfiguration: {Fields: []commandField{fieldConfig, fieldTail}, Optional: []commandField{fieldTail}},
}

// commandKinds lists the accepted kinds in sorted order.
func commandKinds() []string {
	kinds := make([]string, 0, len(commandSpecs))
	for kind := range commandSpecs {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	return kinds
}

// present reports which union fields a command actually carries.
func (c Command) present() map[commandField]bool {
	carried := map[commandField]bool{
		fieldBasis:       c.Basis != "",
		fieldKey:         c.Key != "",
		fieldPromote:     c.Promote != "",
		fieldRelease:     c.Release != "",
		fieldName:        c.Name != "",
		fieldPriority:    c.Priority != "",
		fieldHold:        c.Hold != "",
		fieldMass:        c.Mass != nil,
		fieldValue:       c.Value != nil,
		fieldRequirement: c.Requirement != nil,
		fieldCase:        c.Case != nil,
		fieldCLmax:       c.CLmax != nil,
		fieldScope:       c.Scope != nil,
		fieldRatio:       c.Ratio != 0,
		fieldAngles:      c.Angles != nil,
		fieldTail:        c.Tail != nil,
		fieldShape:       c.Shape != "",
		fieldConfig:      c.Configuration != "",
	}
	return carried
}

// checkShape rejects a command whose fields do not match its kind, reporting
// every mismatch rather than the first.
func (d *decoder) checkShape(field string, c Command, spec commandSpec) {
	allowed := make(map[commandField]bool, len(spec.Fields))
	for _, name := range spec.Fields {
		allowed[name] = true
	}
	optional := make(map[commandField]bool, len(spec.Optional))
	for _, name := range spec.Optional {
		optional[name] = true
	}
	carried := c.present()
	names := make([]commandField, 0, len(carried))
	for name := range carried {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return names[i] < names[j] })
	for _, name := range names {
		switch {
		case carried[name] && !allowed[name]:
			d.add(field+"."+string(name), "invalid",
				"the "+c.Kind+" command takes no "+string(name)+" field")
		case !carried[name] && allowed[name] && !optional[name]:
			d.add(field+"."+string(name), "missing",
				"the "+c.Kind+" command needs a "+string(name)+" field")
		}
	}
}

// command converts a wire command into a core command. It checks the shape
// first, so a caller learns about a misused field even when the values in it
// would also have been rejected.
func (d *decoder) command(field string, c Command) calculator.Command {
	spec, known := commandSpecs[c.Kind]
	if !known {
		kind := "invalid"
		detail := c.Kind + " is not a command; the contract accepts " + joinTokens(commandKinds())
		if c.Kind == "" {
			kind, detail = "missing", "name the command; the contract accepts "+joinTokens(commandKinds())
		}
		d.add(field+".kind", kind, detail)
		return nil
	}
	d.checkShape(field, c, spec)
	return d.buildCommand(field, c)
}

// buildCommand maps one checked command onto its core value.
func (d *decoder) buildCommand(field string, c Command) calculator.Command {
	switch c.Kind {
	case CmdSetMass:
		return calculator.SetMass{Mass: d.quantity(field+".mass", c.Mass), Basis: c.Basis}
	case CmdSetDriver:
		key, issue := parseDriverKey(field+".key", c.Key)
		d.take(issue)
		return calculator.SetDriver{Key: key, Value: d.quantity(field+".value", c.Value)}
	case CmdPromoteDriver:
		return d.promoteDriver(field, c)
	case CmdSetTaperRatio:
		return calculator.SetTaperRatio{Value: d.number(field+".ratio", c.Ratio)}
	case CmdSetRequirement:
		if c.Requirement == nil {
			return nil
		}
		return calculator.SetRequirement{Requirement: d.requirement(field+".requirement", *c.Requirement)}
	case CmdRemoveRequirement:
		return calculator.RemoveRequirement{Name: c.Name}
	case CmdSetRequirementPriority:
		return calculator.SetRequirementPriority{
			Name:     c.Name,
			Priority: enumOrEmpty(d, priorities, field+".priority", c.Priority),
		}
	default:
		return d.buildCaseCommand(field, c)
	}
}

// buildCaseCommand maps the edits that name a case, keeping buildCommand within
// a readable size rather than growing one switch until nothing can be followed.
func (d *decoder) buildCaseCommand(field string, c Command) calculator.Command {
	switch c.Kind {
	case CmdSetCase:
		if c.Case == nil {
			return nil
		}
		return calculator.SetCase{Case: d.designCase(field+".case", *c.Case)}
	case CmdRemoveCase:
		return calculator.RemoveCase{Name: c.Name}
	case CmdSetCasePriority:
		return calculator.SetCasePriority{
			Name:     c.Name,
			Priority: enumOrEmpty(d, priorities, field+".priority", c.Priority),
		}
	case CmdSetCaseCLmax:
		return d.setCaseCLmax(field, c)
	case CmdSizeAtStallLimit:
		hold, issue := parseDriverKey(field+".hold", c.Hold)
		d.take(issue)
		return calculator.SizeAtStallLimit{Hold: hold, Scope: d.scope(field+".scope", c.Scope)}
	case CmdSetPlanformShape:
		return calculator.SetPlanformShape{
			Shape:      enumOrEmpty(d, shapes, field+".shape", c.Shape),
			TaperRatio: d.number(field+".ratio", c.Ratio),
		}
	case CmdSetWingAngles:
		return d.setWingAngles(field, c)
	case CmdSetConfiguration:
		cmd := calculator.SetConfiguration{
			Configuration: enumOrEmpty(d, configurations, field+".configuration", c.Configuration),
		}
		if c.Tail != nil {
			cmd.Tail = d.tail(*c.Tail)
		}
		return cmd
	default:
		return nil
	}
}

func (d *decoder) setWingAngles(field string, c Command) calculator.Command {
	if c.Angles == nil {
		return nil
	}
	angles := *c.Angles
	return calculator.SetWingAngles{
		Sweep:          d.quantity(field+".angles.sweep", angles.Sweep),
		Dihedral:       d.quantity(field+".angles.dihedral", angles.Dihedral),
		Twist:          d.quantity(field+".angles.twist", angles.Twist),
		Incidence:      d.quantity(field+".angles.incidence", angles.Incidence),
		SweepReference: d.number(field+".angles.sweepReference", angles.SweepReference),
		DihedralMode:   enumOrEmpty(d, dihedralModes, field+".angles.dihedralMode", angles.DihedralMode),
	}
}

func (d *decoder) promoteDriver(field string, c Command) calculator.Command {
	promote, issue := parseDriverKey(field+".promote", c.Promote)
	d.take(issue)
	cmd := calculator.PromoteDriver{
		Promote: promote,
		Value:   d.quantity(field+".value", c.Value),
	}
	// An absent release is a supported request: the core answers it with the
	// valid swaps rather than choosing one, which is exactly what a worksheet
	// needs to offer.
	if c.Release != "" {
		release, releaseIssue := parseDriverKey(field+".release", c.Release)
		d.take(releaseIssue)
		cmd.Release = release
	}
	return cmd
}

func (d *decoder) setCaseCLmax(field string, c Command) calculator.Command {
	cmd := calculator.SetCaseCLmax{Case: c.Name}
	if c.CLmax != nil {
		cmd.CLmax = d.clmax(field+".clmax", *c.CLmax)
		cmd.Evidence = enumOrEmpty(d, evidenceGrades, field+".clmax.evidence", c.CLmax.Evidence)
	}
	return cmd
}

// encodeCommand renders a core command back onto the wire. It exists so that a
// conflict's alternatives cross the boundary as commands a client can send
// back, rather than as prose it would have to reconstruct an edit from.
//
// The switch covers the commands the core actually offers as alternatives. A
// command it does not know is reported rather than silently dropped, because an
// alternative whose command vanished would read as advice with no action.
func encodeCommand(cmd calculator.Command) (Command, bool) {
	switch c := cmd.(type) {
	case calculator.SetMass:
		mass := encodeQuantity(c.Mass)
		return Command{Kind: CmdSetMass, Mass: mass, Basis: c.Basis}, true
	case calculator.SetDriver:
		return Command{Kind: CmdSetDriver, Key: string(c.Key), Value: encodeQuantity(c.Value)}, true
	case calculator.SetTaperRatio:
		return Command{Kind: CmdSetTaperRatio, Ratio: c.Value}, true
	default:
		return Command{}, false
	}
}
