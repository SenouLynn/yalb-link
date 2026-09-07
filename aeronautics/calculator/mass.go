package calculator

import "sort"

// DatumAircraft names the coordinate convention component positions and the
// centre of gravity are expressed in. It is deliberately the same origin and
// the same axes as DatumWingRoot: a balance result is only useful next to the
// wing geometry it is judged against, and two datums would need a transform
// nothing in this package implements. Stating one convention for both is what
// lets a CG be compared with the mean aerodynamic chord without one.
const DatumAircraft = DatumWingRoot

// ComponentRole names what a mass is, for the worksheet to group and label it.
// It carries no physics: no role implies a mass, a position, a power draw or a
// structural attachment, and nothing in this package reads a role to decide a
// number.
type ComponentRole uint8

const (
	// ComponentRoleUnknown is the zero value and is never accepted.
	ComponentRoleUnknown ComponentRole = iota
	// ComponentAirframe is structure: wing, fuselage, tail, hardware.
	ComponentAirframe
	// ComponentBattery is the flight battery.
	ComponentBattery
	// ComponentMotor is the motor, its mount and the propeller.
	ComponentMotor
	// ComponentAvionics is the autopilot, receiver, telemetry, sensors, power
	// electronics, wiring and servos.
	ComponentAvionics
	// ComponentPayload is what the aircraft is carrying for the mission.
	ComponentPayload
	// ComponentOther is anything the roles above do not name.
	ComponentOther
)

var componentRoleNames = [...]string{
	ComponentRoleUnknown: "unknown",
	ComponentAirframe:    "airframe",
	ComponentBattery:     "battery",
	ComponentMotor:       "motor",
	ComponentAvionics:    "avionics",
	ComponentPayload:     "payload",
	ComponentOther:       "other",
}

// String returns the role's readable name.
func (r ComponentRole) String() string {
	if int(r) < len(componentRoleNames) {
		return componentRoleNames[r]
	}
	return "unknown"
}

// MassItem is one mass and where it sits. It is a component of the design
// definition, not a result: a placement is edited through a command like any
// other value, and history restores it like any other.
//
// A position is complete only when all three coordinates are supplied. Zero is
// spelled out, exactly as the wing angles are: a component on the centerline
// has y = 0, and an unstated y is a missing field rather than a claim that it
// is centred.
type MassItem struct {
	// Name identifies the item within the design.
	Name string
	// Basis states where the mass came from, for example "weighed" or
	// "manufacturer's figure for the 3S 2200 mAh pack".
	Basis string
	// Position is the item's centre of mass in DatumAircraft.
	Position Point
	// Mass is the item's mass. The zero Quantity means it is not stated.
	Mass Quantity
	// Role names what the item is.
	Role ComponentRole
}

// Placed reports whether every coordinate of the item's position is supplied.
func (m MassItem) Placed() bool {
	return m.Position.X.supplied() && m.Position.Y.supplied() && m.Position.Z.supplied()
}

// contributes reports whether the item can enter a mass-properties sum, and why
// it cannot when it cannot.
func (m MassItem) contributes() (known bool, detail string) {
	switch {
	case !m.Mass.supplied():
		return false, "no mass is stated for " + m.Name + ", so it contributes nothing; it is not " +
			"treated as weightless"
	case !m.Placed():
		return false, "no complete position is stated for " + m.Name + ", so it contributes " +
			"nothing; it is not placed at the origin"
	default:
		return true, ""
	}
}

// MassContribution is one item's contribution to the mass properties, with the
// evaluation of each axis moment. A contribution that could not be made keeps
// the reason rather than dropping out of the list.
type MassContribution struct {
	// Name identifies the item.
	Name string
	// Detail explains a contribution that could not be made.
	Detail string
	// Moments are the mass moments about the datum on x, y and z, in that order.
	// They are empty when Known is false.
	Moments []Quantity
	// Traces record the moment evaluations, in axis order.
	Traces []Trace
	// Mass is the item's mass.
	Mass Quantity
	// Position is where the item sits.
	Position Point
	// Role names what the item is.
	Role ComponentRole
	// Known reports whether the item contributed.
	Known bool
}

// MassProperties is the mechanical balance of the listed masses: their total
// and the station their combined moment puts the centre of gravity at.
//
// It is mechanical only. The centre of gravity is not an aerodynamic centre, a
// neutral point or a centre of pressure, and no trim, static-margin or handling
// conclusion follows from it. Task 08 is where any of those may be claimed.
type MassProperties struct {
	// Detail explains an incomplete or unavailable result.
	Detail string
	// Datum names the coordinate convention CG and every position use.
	Datum string
	// Contributions are every item considered, in design order.
	Contributions []MassContribution
	// Traces record the total, the moment sums and the centre-of-gravity
	// evaluations, in evaluation order.
	Traces []Trace
	// Total is the sum of the contributing masses.
	Total Quantity
	// CG is the centre of gravity in DatumAircraft.
	CG Point
	// Status reports whether the model produced a centre of gravity at all.
	Status ResultStatus
	// Complete reports that every listed item contributed. A false value with a
	// computed status means the result describes a subset of the design, which
	// is an incomplete assessment rather than a balance of the aircraft.
	Complete bool
}

// MassMode selects what the all-up mass of the design is.
//
// The two are kept apart deliberately. An entered all-up mass is a figure a
// builder holds — a target, a measurement of a finished aircraft — and it does
// not become a component inventory by being written down. A component total is
// only as complete as the inventory behind it, and reading a partial inventory
// as the aircraft's mass would understate it silently.
type MassMode uint8

const (
	// MassModeUnknown is the zero value. It is read as MassModeEntered, which is
	// the mode a design without a component inventory is already in.
	MassModeUnknown MassMode = iota
	// MassModeEntered takes the design's own Mass as the all-up mass. Components
	// may still be listed and balanced; they do not set the mass.
	MassModeEntered
	// MassModeComponents takes the sum of the component masses as the all-up
	// mass. An incomplete inventory leaves the all-up mass unknown rather than
	// producing a total that is missing a component.
	MassModeComponents
)

var massModeNames = [...]string{
	MassModeUnknown:    "unknown",
	MassModeEntered:    "entered all-up mass",
	MassModeComponents: "component total",
}

// String returns the mode's readable name.
func (m MassMode) String() string {
	if int(m) < len(massModeNames) {
		return massModeNames[m]
	}
	return "unknown"
}

// resolved reads the zero value as the entered mode, so a design written before
// components existed keeps behaving the way it did.
func (m MassMode) resolved() MassMode {
	if m == MassModeUnknown {
		return MassModeEntered
	}
	return m
}

// Component returns the mass item with the given name.
func (d Design) Component(name string) (MassItem, bool) {
	for _, item := range d.Components {
		if item.Name == name {
			return item, true
		}
	}
	return MassItem{}, false
}

// componentNames returns every component name, sorted, so a message that lists
// them does not depend on entry order.
func (d Design) componentNames() []string {
	names := make([]string, 0, len(d.Components))
	for _, item := range d.Components {
		names = append(names, item.Name)
	}
	sort.Strings(names)
	return names
}

// MassProperties balances the listed components. It is available in either mass
// mode: a builder who has entered an all-up mass may still place components and
// read where their combined centre of gravity sits, and the two answers stay
// separately labelled rather than one overwriting the other.
func (d Design) MassProperties() MassProperties {
	mp := MassProperties{Datum: DatumAircraft, Complete: true}
	if len(d.Components) == 0 {
		mp.Status = ResultMissing
		mp.Complete = false
		mp.Detail = "no component masses are listed, so there is nothing to balance"
		return mp
	}
	total := newEvaluation(EqTotalMass)
	sums := [3]*evaluation{
		newEvaluation(EqMomentSum), newEvaluation(EqMomentSum), newEvaluation(EqMomentSum),
	}
	contributing := 0
	for _, item := range d.Components {
		contribution := massContribution(item)
		mp.Contributions = append(mp.Contributions, contribution)
		if !contribution.Known {
			mp.Complete = false
			continue
		}
		contributing++
		total.quantity(portComponentMass.Name, item.Mass)
		for axis := range sums {
			sums[axis].signedQuantity(portMassMoment.Name, contribution.Moments[axis])
		}
	}
	if contributing == 0 {
		mp.Status = ResultMissing
		mp.Detail = "no listed component states both a mass and a complete position, so no total " +
			"and no centre of gravity follow from them"
		return mp
	}
	mp.finish(total, sums, d.Components)
	return mp
}

// finish completes the sums and divides them, recording every trace in
// evaluation order: the mass total first, then each axis's moment sum and the
// station it produces.
func (mp *MassProperties) finish(total *evaluation, sums [3]*evaluation, items []MassItem) {
	totalMass, err := total.finish(sumOf(items, contributingMass))
	if err != nil {
		mp.Status = statusForError(err)
		mp.Detail = "the component masses do not sum to a usable total: " + err.Error()
		return
	}
	mp.Total = totalMass.Value
	mp.Traces = append(mp.Traces, totalMass.Trace)
	var stations [3]Quantity
	for axis := range sums {
		moment, momentErr := sums[axis].finishSigned(sumOf(items, contributingMoment(axis)))
		if momentErr != nil {
			mp.fail(momentErr, "the component moments do not sum on the "+axisNames[axis]+" axis: ")
			return
		}
		mp.Traces = append(mp.Traces, moment.Trace)
		station, stationErr := centerOfGravity(moment.Value, totalMass.Value)
		if stationErr != nil {
			mp.fail(stationErr, "the "+axisNames[axis]+" centre of gravity could not be evaluated: ")
			return
		}
		mp.Traces = append(mp.Traces, station.Trace)
		stations[axis] = station.Value
	}
	mp.CG = Point{X: stations[0], Y: stations[1], Z: stations[2]}
	mp.Status = ResultComputed
	if !mp.Complete {
		mp.Detail = "this balances only the components that state both a mass and a complete " +
			"position; it is not the balance of the whole aircraft"
	}
}

// fail records why no centre of gravity follows, discarding the partial traces
// so that nothing downstream can read half an evaluation as a result.
func (mp *MassProperties) fail(err error, prefix string) {
	mp.Traces = nil
	mp.Total = Quantity{}
	mp.Status = statusForError(err)
	mp.Detail = prefix + err.Error()
}

// axisNames names the three axes, for a message that has to say which one
// failed.
var axisNames = [3]string{"x", "y", "z"}

// contributingMass reads the mass of an item that contributes, and reports
// whether it does.
func contributingMass(item MassItem) (float64, bool) {
	if known, _ := item.contributes(); !known {
		return 0, false
	}
	return item.Mass.si, true
}

// contributingMoment reads one axis of an item's mass moment.
func contributingMoment(axis int) func(MassItem) (float64, bool) {
	return func(item MassItem) (float64, bool) {
		if known, _ := item.contributes(); !known {
			return 0, false
		}
		return item.Mass.si * item.Position.axis(axis).si, true
	}
}

// sumOf adds the contributing items' values. The sum is formed here rather than
// inside the evaluation harness because the harness records substitutions; the
// two run over the same predicate, so a value that was recorded is a value that
// was added.
func sumOf(items []MassItem, read func(MassItem) (float64, bool)) float64 {
	sum := 0.0
	for _, item := range items {
		if value, ok := read(item); ok {
			sum += value
		}
	}
	return sum
}

// axis reads one coordinate of a point by index, so the three axes can be
// evaluated by the same code rather than by three copies of it.
func (p Point) axis(n int) Quantity {
	switch n {
	case 0:
		return p.X
	case 1:
		return p.Y
	case 2:
		return p.Z
	default:
		return Quantity{}
	}
}

// massContribution evaluates one item's moments about the datum.
func massContribution(item MassItem) MassContribution {
	contribution := MassContribution{
		Name:     item.Name,
		Role:     item.Role,
		Mass:     item.Mass,
		Position: item.Position,
	}
	known, detail := item.contributes()
	if !known {
		contribution.Detail = detail
		return contribution
	}
	for axis := range axisNames {
		result, err := componentMoment(item.Mass, item.Position.axis(axis))
		if err != nil {
			// A rejected coordinate leaves the whole item out rather than
			// contributing the axes that did evaluate: two thirds of a moment is
			// not a smaller contribution, it is a wrong one.
			return MassContribution{
				Name: item.Name, Role: item.Role, Mass: item.Mass, Position: item.Position,
				Detail: "the " + axisNames[axis] + " moment of " + item.Name +
					" could not be evaluated: " + err.Error(),
			}
		}
		contribution.Moments = append(contribution.Moments, result.Value)
		contribution.Traces = append(contribution.Traces, result.Trace)
	}
	contribution.Known = true
	return contribution
}

// componentMoment returns one component's mass moment about the datum on one
// axis. The station is signed: a component ahead of the datum has a negative x.
func componentMoment(mass, station Quantity) (Result, error) {
	e := newEvaluation(EqComponentMoment)
	m := e.quantity(portComponentMass.Name, mass)
	r := e.signedQuantity(portComponentStation.Name, station)
	return e.finishSigned(m * r)
}

// centerOfGravity divides a moment sum by a mass total. The result is signed:
// a centre of gravity ahead of the datum sits at a negative station.
func centerOfGravity(moment, total Quantity) (Result, error) {
	e := newEvaluation(EqCenterOfGravity)
	sum := e.signedQuantity(portMassMomentSum.Name, moment)
	mass := e.quantity(portTotalMass.Name, total)
	if mass == 0 {
		return Result{}, Issues{{
			Field:  portTotalMass.Name,
			Kind:   IssueInvalid,
			Detail: "a centre of gravity needs a nonzero total mass",
		}}
	}
	return e.finishSigned(sum / mass)
}

// StationFractionOfMAC expresses a station as a fraction of the mean
// aerodynamic chord, measured from its leading edge. It is a geometric
// reference and nothing more: a centre of gravity at 0.25 of the MAC is not a
// static margin, a trim result or a stability claim, and this package
// implements none of those.
func StationFractionOfMAC(station, macLeadingEdge, mac Quantity) (Result, error) {
	e := newEvaluation(EqStationFractionOfMAC)
	x := e.signedQuantity(portCGStation.Name, station)
	le := e.signedQuantity(portMACLeading.Name, macLeadingEdge)
	chord := e.quantity(portMAC.Name, mac)
	if chord == 0 {
		return Result{}, Issues{{
			Field:  portMAC.Name,
			Kind:   IssueInvalid,
			Detail: "a chord fraction needs a nonzero mean aerodynamic chord",
		}}
	}
	return e.finishSigned((x - le) / chord)
}

// massReading returns the all-up mass the design is judged at, in the mode it
// selected, together with why it is not available when it is not.
//
// Every sizing result that consumes an all-up mass reads it through here, so
// the two modes cannot disagree about what the aircraft weighs.
func (d Design) massReading() subjectReading {
	if d.MassMode.resolved() == MassModeComponents {
		mp := d.MassProperties()
		if mp.Status != ResultComputed {
			return subjectReading{Status: mp.Status, Detail: mp.Detail}
		}
		if !mp.Complete {
			return subjectReading{
				Status: ResultMissing,
				Detail: "the component inventory is incomplete, so it does not establish an " +
					"all-up mass: " + mp.Detail,
			}
		}
		trace := Trace{}
		if len(mp.Traces) > 0 {
			trace = mp.Traces[0]
		}
		return subjectReading{Status: ResultComputed, Value: mp.Total, Trace: trace}
	}
	if !d.Mass.supplied() {
		return subjectReading{Status: ResultMissing, Detail: "no all-up mass is supplied"}
	}
	return subjectReading{Status: ResultComputed, Value: d.Mass}
}

// AllUpMass returns the mass the design is judged at, or the zero Quantity when
// the selected mode does not establish one.
func (d Design) AllUpMass() Quantity { return d.massReading().Value }

// validateComponents checks the structure of the inventory. Completeness is not
// checked here: a component whose mass or position is still missing is a design
// in progress, and the mass properties report exactly which one it is, so
// refusing the definition would say less and block more.
func (d Design) validateComponents(rs *resultSet) {
	seen := make(map[string]bool, len(d.Components))
	for _, item := range d.Components {
		if item.Name == "" {
			rs.add("component", IssueMissing,
				"name every component so its mass and position can be reported against it")
			continue
		}
		field := "component." + item.Name
		if seen[item.Name] {
			rs.add(field, IssueInvalid, "the design lists this component twice")
		}
		seen[item.Name] = true
		if item.Role == ComponentRoleUnknown {
			rs.add(field, IssueMissing,
				"say what the component is: a role groups and labels it, and no role implies a mass "+
					"or a position")
		}
		if item.Mass.supplied() && item.Basis == "" {
			rs.add(field, IssueMissing,
				"state where the mass came from, so an estimate is not read as a measurement")
		}
	}
	if d.MassMode.resolved() == MassModeComponents && len(d.Components) == 0 {
		rs.add("mass_mode", IssueMissing,
			"the all-up mass is set to come from the components, but none are listed")
	}
}

// CaseLoad is the lift one flight case demands of the whole aircraft: n times
// weight, from Task 02's lumped model.
//
// It is a magnitude and nothing else. The lumped model puts the entire load on
// the wing and solves no line of action, no spanwise distribution and no tail
// balancing load, so nothing here says where that force acts. Task 08 is where
// a supported force location may be claimed.
type CaseLoad struct {
	// Case names the flight case.
	Case string
	// Detail explains a load that could not be evaluated.
	Detail string
	// Trace records the evaluation, when one ran.
	Trace Trace
	// RequiredLift is n * m * g for the case.
	RequiredLift Quantity
	// Priority states whether the case must hold.
	Priority Priority
	// Status reports whether the model produced the value at all.
	Status ResultStatus
}

// CaseLoads returns the lumped lift each defined case demands, in case order.
// The all-up mass is read through the design's mass mode, so a component
// inventory and an entered figure produce the load the same way.
func (d Design) CaseLoads() []CaseLoad {
	mass := d.massReading()
	loads := make([]CaseLoad, 0, len(d.Cases))
	for n := range d.Cases {
		dc := &d.Cases[n]
		load := CaseLoad{Case: dc.Case.Name, Priority: dc.Priority}
		if mass.Status != ResultComputed {
			load.Status = mass.Status
			load.Detail = mass.Detail
			loads = append(loads, load)
			continue
		}
		result, err := RequiredLift(dc.Case, mass.Value)
		if err != nil {
			load.Status = statusForError(err)
			load.Detail = err.Error()
			loads = append(loads, load)
			continue
		}
		load.Status = ResultComputed
		load.RequiredLift = result.Value
		load.Trace = result.Trace
		loads = append(loads, load)
	}
	return loads
}
