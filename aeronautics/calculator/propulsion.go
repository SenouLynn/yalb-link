package calculator

import "sort"

// CapabilityKind separates propulsion capability measured with the aircraft
// standing still from capability measured or estimated in flight.
//
// The two are never interchanged. A static thrust figure is the propeller
// working at zero advance ratio, which is the one condition where it produces
// the most thrust and the least useful work, and treating it as cruise thrust
// overstates the aircraft by a factor that depends on the propeller. Nothing in
// this package converts one into the other: doing so needs a propeller
// operating-point model with real blade data, which is not implemented.
type CapabilityKind uint8

const (
	// CapabilityKindUnknown is the zero value and is never accepted.
	CapabilityKindUnknown CapabilityKind = iota
	// CapabilityStatic is measured with no free-stream velocity.
	CapabilityStatic
	// CapabilityInFlight is measured or estimated at a stated nonzero airspeed.
	CapabilityInFlight
)

var capabilityKindNames = [...]string{
	CapabilityKindUnknown: "unknown",
	CapabilityStatic:      "static",
	CapabilityInFlight:    "in flight",
}

// String returns the kind's readable name.
func (k CapabilityKind) String() string {
	if int(k) < len(capabilityKindNames) {
		return capabilityKindNames[k]
	}
	return "unknown"
}

// PropulsionCapability is what the propulsion system was measured or estimated
// to deliver at one explicitly named operating condition.
//
// Every condition field is part of the claim. The same motor and propeller on a
// 4S pack and on a 6S pack are different capability points, and so are the same
// pair at half throttle and at full throttle; a thrust figure that does not say
// which is not a capability this package can check anything against.
type PropulsionCapability struct {
	// Name identifies the point within the design.
	Name string
	// Basis states where the numbers came from, for example "bench test, 4S
	// 14.8 V, APC 10x5, load cell" or "manufacturer's table".
	Basis string
	// DensityBasis states where Density came from.
	DensityBasis string
	// Note records anything else about the condition a reader needs.
	Note string
	// Speed is the free-stream true airspeed the point was taken at. A static
	// point states zero explicitly; an unstated speed is a missing field.
	Speed Quantity
	// Density is the air density at the point.
	Density Quantity
	// Voltage is the pack voltage under load at the point.
	Voltage Quantity
	// RPM is the propeller rotation rate at the point. Either this or Throttle
	// must be stated, because "at full throttle" and "at 9000 rpm" are the two
	// ways a builder actually knows which point this is.
	RPM Quantity
	// Thrust is the thrust delivered at the point.
	Thrust Quantity
	// ElectricalPower is the electrical power drawn at the point.
	ElectricalPower Quantity
	// Current is the current drawn at the point.
	Current Quantity
	// Throttle is the throttle setting as a fraction of full, from 0 to 1.
	Throttle float64
	// Evidence grades the provenance: a bench measurement and a catalogue figure
	// are both usable and are not the same claim.
	Evidence EvidenceQuality
	// Kind separates a static point from an in-flight one.
	Kind CapabilityKind
}

// appliesAt reports whether this point may answer a question about flight at
// the given true airspeed, and why it may not when it may not.
//
// The tolerance is relative and deliberately tight. A capability point is a
// measurement at a condition, and stretching it across a speed range is the
// propeller operating-point model this package does not have.
func (c PropulsionCapability) appliesAt(speed Quantity) (applies bool, why string) {
	switch {
	case c.Kind == CapabilityStatic && speed.si > 0:
		return false, "capability point " + c.Name + " is a static measurement at zero airspeed. " +
			"Static thrust is not thrust at " + speed.String() + ", and nothing here converts one " +
			"into the other: that needs a propeller operating-point model with real blade data"
	case c.Kind == CapabilityInFlight && speed.si == 0:
		return false, "capability point " + c.Name + " was taken in flight at " + c.Speed.String() +
			", so it says nothing about a standing start"
	case !c.Speed.supplied():
		return false, "capability point " + c.Name + " states no airspeed, so there is no " +
			"condition to match against"
	case relativeGap(c.Speed.si, speed.si) > capabilitySpeedTolerance:
		return false, "capability point " + c.Name + " was taken at " + c.Speed.String() +
			", which is not the " + speed.String() + " this condition asks about. Interpolating " +
			"between operating points needs a propeller model this package does not implement"
	default:
		return true, ""
	}
}

// capabilitySpeedTolerance is the relative agreement between a capability
// point's airspeed and a condition's before the point is read as describing it.
// It is display slack, not a modelling allowance: 1% of 15 m/s is 0.15 m/s.
const capabilitySpeedTolerance = 0.01

// relativeGap is the absolute difference between two values relative to the
// larger magnitude, so that a comparison against zero is still meaningful.
func relativeGap(a, b float64) float64 {
	scale := absOf(a)
	if other := absOf(b); other > scale {
		scale = other
	}
	if scale == 0 {
		return 0
	}
	return absOf(a-b) / scale
}

func absOf(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// validate reports every structural problem with one capability point.
func (c PropulsionCapability) validate(rs *resultSet) {
	field := "capability." + c.Name
	if c.Name == "" {
		rs.add("capability", IssueMissing,
			"name every capability point so a mission segment can refer to it")
		return
	}
	if c.Basis == "" {
		rs.add(field, IssueMissing,
			"state where the thrust and power figures came from, so a catalogue number is not "+
				"read as a measurement")
	}
	c.validateCondition(field, rs)
	c.validateOutputs(field, rs)
}

func (c PropulsionCapability) validateCondition(field string, rs *resultSet) {
	c.validateKind(field, rs)
	c.validateAtmosphere(field, rs)
	c.validateSetting(field, rs)
}

// validateKind checks that the point's speed matches the kind it claims. A
// static point measured at a nonzero airspeed and an in-flight point measured
// at zero are both mislabelled rather than merely incomplete.
func (c PropulsionCapability) validateKind(field string, rs *resultSet) {
	switch c.Kind {
	case CapabilityStatic:
		if c.Speed.supplied() && c.Speed.si != 0 {
			rs.add(field, IssueInvalid,
				"a static point is measured at zero airspeed, but "+c.Speed.String()+" is stated")
		}
	case CapabilityInFlight:
		if c.Speed.supplied() && c.Speed.si <= 0 {
			rs.add(field, IssueInvalid,
				"an in-flight point needs a positive airspeed; a zero-speed measurement is a "+
					"static point and is labelled as one")
		}
	default:
		rs.add(field, IssueMissing,
			"say whether the point is static or in flight; static thrust is not cruise thrust "+
				"and nothing here converts between them")
	}
}

// validateAtmosphere checks the air the point was taken in and the pack that
// was driving it. Both are part of the claim: the same motor and propeller on a
// different pack, or at a different density, is a different capability point.
func (c PropulsionCapability) validateAtmosphere(field string, rs *resultSet) {
	if !c.Speed.supplied() {
		rs.add(field, IssueMissing,
			"state the airspeed the point was taken at, with zero spelled out for a static test")
	}
	if !c.Density.supplied() {
		rs.add(field, IssueMissing, "state the air density the point was taken at")
	} else if c.DensityBasis == "" {
		rs.add(field, IssueMissing, "state where the point's air density came from")
	}
	if !c.Voltage.supplied() {
		rs.add(field, IssueMissing,
			"state the pack voltage under load; the same motor and propeller on a different pack "+
				"is a different capability point")
	}
}

// validateSetting checks that the point names which operating condition it is.
func (c PropulsionCapability) validateSetting(field string, rs *resultSet) {
	if c.Throttle <= 0 && !c.RPM.supplied() {
		rs.add(field, IssueMissing,
			"state the throttle setting or the propeller rpm, so the point names which operating "+
				"condition it is")
	}
	if c.Throttle != 0 && (c.Throttle < 0 || c.Throttle > 1 || !isFinite(c.Throttle)) {
		rs.add(field, IssueInvalid,
			"the throttle setting is a fraction of full between 0 and 1, received "+
				formatFloat(c.Throttle))
	}
}

func (c PropulsionCapability) validateOutputs(field string, rs *resultSet) {
	if !c.Thrust.supplied() {
		rs.add(field, IssueMissing, "state the thrust the point delivers")
	}
	if !c.ElectricalPower.supplied() && !c.Current.supplied() {
		rs.add(field, IssueMissing,
			"state the electrical power or the current drawn, or this point supports no "+
				"electrical feasibility claim at all")
	}
	if c.Evidence == EvidenceUnstated {
		rs.add(field, IssueMissing,
			"grade the evidence: an estimated operating point and a bench measurement are not "+
				"the same claim")
	}
}

// ElectricalDraw returns the electrical power the point draws, taking it from
// the stated power or from the voltage and current, and reports a disagreement
// between the two rather than silently preferring one.
func (c PropulsionCapability) ElectricalDraw() (Result, error) {
	if c.Current.supplied() && c.Voltage.supplied() {
		result, err := ElectricalPower(c.Voltage, c.Current)
		if err != nil {
			return Result{}, err
		}
		if c.ElectricalPower.supplied() &&
			relativeGap(result.Value.si, c.ElectricalPower.si) > capabilityPowerTolerance {
			return Result{}, Issues{{
				Field: "capability." + c.Name,
				Kind:  IssueInvalid,
				Detail: "the stated electrical power " + c.ElectricalPower.String() +
					" disagrees with voltage times current, " + result.Value.String() +
					"; correct one of them rather than leaving the point to be read either way",
			}}
		}
		return result, nil
	}
	if !c.ElectricalPower.supplied() {
		return Result{}, Issues{{
			Field:  "capability." + c.Name,
			Kind:   IssueMissing,
			Detail: "the point states neither an electrical power nor a current",
		}}
	}
	return Result{Value: c.ElectricalPower}, nil
}

// capabilityPowerTolerance is the relative agreement required between a stated
// electrical power and the product of the stated voltage and current before
// they are called consistent. It is loose enough for values read back off a
// rounded display and tight enough that a real disagreement is reported.
const capabilityPowerTolerance = 0.02

// PropulsionEfficiency is the combined propeller, motor and speed-controller
// efficiency at a stated condition: the fraction of electrical power that
// leaves the aircraft as useful propulsive work.
//
// It is one number covering three separate machines on purpose. Splitting it
// would imply this package models each of them, and it does not. What it does
// insist on is that the number says where it came from, because a chain
// efficiency is the single largest lever on every electrical estimate below.
type PropulsionEfficiency struct {
	// Basis states where the value came from and at which condition.
	Basis string
	// Total is the combined efficiency, greater than zero and at most one.
	Total float64
	// Evidence grades that basis.
	Evidence EvidenceQuality
}

func (p PropulsionEfficiency) supplied() bool { return p.Total != 0 || p.Basis != "" }

func (p PropulsionEfficiency) validate(field string, rs *resultSet) {
	if !p.supplied() {
		return
	}
	if p.Total <= 0 || p.Total > 1 || !isFinite(p.Total) {
		rs.add(field, IssueInvalid,
			"the chain efficiency is a fraction greater than zero and at most one, received "+
				formatFloat(p.Total))
	}
	if p.Basis == "" {
		rs.add(field, IssueMissing,
			"state where the propeller, motor and speed-controller chain efficiency came from, "+
				"and at what condition")
	}
	if p.Evidence == EvidenceUnstated {
		rs.add(field, IssueMissing, "grade the evidence behind the chain efficiency")
	}
}

// PropulsionLimits are the ratings a component feasibility claim is made
// against. Every one of them is optional and every one of them that is absent
// is reported as an unchecked limit rather than as a satisfied one.
type PropulsionLimits struct {
	// Basis states where the ratings came from.
	Basis string
	// MaxContinuousElectricalPower is the largest electrical power the
	// motor and speed controller are rated for continuously.
	MaxContinuousElectricalPower Quantity
	// MaxPeakElectricalPower is the largest electrical power they are rated for
	// in a burst. It is a different rating from the continuous one and never
	// stands in for it.
	MaxPeakElectricalPower Quantity
	// MaxRPM is the propeller's or the motor's rotation-rate rating.
	MaxRPM Quantity
	// MaxVoltage is the highest pack voltage the speed controller and motor
	// accept.
	MaxVoltage Quantity
	// PropellerDiameter is the propeller's diameter.
	PropellerDiameter Quantity
	// PropellerHubHeight is how far the propeller hub sits above the ground line
	// the aircraft rests on. With the diameter it gives the tip clearance.
	PropellerHubHeight Quantity
}

// ThrustTarget is a required or preferred thrust-to-weight ratio, stated at the
// condition of one named capability point.
//
// A target and an available thrust are deliberately different things held in
// different places. The target is what the builder wants the aircraft to do;
// the capability point is what the propulsion system was measured to deliver.
// Naming the point is what stops a target stated for a launch from being
// checked against a cruise measurement.
type ThrustTarget struct {
	// Name identifies the target.
	Name string
	// Basis states where the ratio came from, for example "hand launch, from
	// the club's rule of thumb" or "climb gradient requirement".
	Basis string
	// Capability names the capability point whose condition the target is
	// stated at.
	Capability string
	// Ratio is the required thrust-to-weight ratio.
	Ratio float64
	// Priority states whether the target must hold.
	Priority Priority
}

// RegulatorSide records which side of a regulator an entered electrical draw
// was measured on. It is required, and it is the difference between counting a
// regulator's losses once and counting them twice or not at all.
type RegulatorSide uint8

const (
	// RegulatorSideUnknown is the zero value and is never accepted.
	RegulatorSideUnknown RegulatorSide = iota
	// RegulatorPackSide means the figure is what the battery supplies, with any
	// regulator loss already in it.
	RegulatorPackSide
	// RegulatorLoadSide means the figure is what the load consumes downstream of
	// a regulator, so the pack supplies more by the regulator's efficiency.
	RegulatorLoadSide
)

var regulatorSideNames = [...]string{
	RegulatorSideUnknown: "unknown",
	RegulatorPackSide:    "pack side",
	RegulatorLoadSide:    "load side",
}

// String returns the side's readable name.
func (s RegulatorSide) String() string {
	if int(s) < len(regulatorSideNames) {
		return regulatorSideNames[s]
	}
	return "unknown"
}

// AuxiliaryLoad is one non-propulsive electrical draw: the autopilot, the
// receiver, a telemetry radio, a sensor, a servo, the payload.
//
// Continuous and peak are separate figures because they answer separate
// questions. The continuous draw is what the mission's energy budget pays for;
// the peak is what the pack and the regulator have to survive when every servo
// moves at once, and an aircraft can pass the first and fail the second.
type AuxiliaryLoad struct {
	// Name identifies the load.
	Name string
	// Basis states where the figures came from.
	Basis string
	// Component optionally names the mass item this draw belongs to, so that a
	// device's mass and its power appear against the same thing.
	Component string
	// Continuous is the draw held throughout a mission segment.
	Continuous Quantity
	// Peak is the short-term draw. It is never added to the energy budget: an
	// unstated duty cycle makes an energy figure from a peak meaningless.
	Peak Quantity
	// RegulatorEfficiency converts a load-side figure to a pack-side one. It is
	// required when Side is RegulatorLoadSide and refused otherwise.
	RegulatorEfficiency float64
	// Side records which side of the regulator the figures were measured on.
	Side RegulatorSide
	// Evidence grades the basis.
	Evidence EvidenceQuality
}

func (a AuxiliaryLoad) validate(rs *resultSet) {
	field := "auxiliary." + a.Name
	if a.Name == "" {
		rs.add("auxiliary", IssueMissing, "name every auxiliary load so its draw can be reported")
		return
	}
	if a.Basis == "" {
		rs.add(field, IssueMissing,
			"state where the draw came from, so a datasheet maximum is not read as a measurement")
	}
	if !a.Continuous.supplied() {
		rs.add(field, IssueMissing,
			"state the continuous draw, with zero spelled out for a device that draws nothing "+
				"between actuations")
	}
	switch a.Side {
	case RegulatorPackSide:
		if a.RegulatorEfficiency != 0 {
			rs.add(field, IssueInvalid,
				"a pack-side figure already includes the regulator's loss, so it takes no "+
					"regulator efficiency; applying one would count that loss twice")
		}
	case RegulatorLoadSide:
		if a.RegulatorEfficiency <= 0 || a.RegulatorEfficiency > 1 || !isFinite(a.RegulatorEfficiency) {
			rs.add(field, IssueMissing,
				"a load-side figure needs the regulator's efficiency to become a pack-side draw; "+
					"supply a fraction greater than zero and at most one")
		}
	default:
		rs.add(field, IssueMissing,
			"say which side of the regulator the draw was measured on; without it the "+
				"regulator's loss is either counted twice or not at all")
	}
	if a.Evidence == EvidenceUnstated {
		rs.add(field, IssueMissing, "grade the evidence behind the draw")
	}
}

// packSide converts one figure on this load to the draw the battery sees.
func (a AuxiliaryLoad) packSide(value Quantity) (Result, error) {
	if a.Side == RegulatorLoadSide {
		return AuxiliaryPackSideDraw(value, a.RegulatorEfficiency)
	}
	return Result{Value: value, Trace: Trace{}}, nil
}

// BatteryEnergyMode selects where the pack's nominal energy comes from. The two
// are explicit modes for the same reason the mass modes are: a capacity and a
// nominal voltage is a different claim from a measured energy, and inferring
// which one a builder meant from which fields happen to be filled would make
// the answer depend on entry order.
type BatteryEnergyMode uint8

const (
	// BatteryEnergyModeUnknown is the zero value and is never accepted for an
	// energy result.
	BatteryEnergyModeUnknown BatteryEnergyMode = iota
	// BatteryEnergyFromCapacity multiplies the labelled capacity by the nominal
	// voltage.
	BatteryEnergyFromCapacity
	// BatteryEnergyEntered takes an energy the builder supplies directly, for
	// example from a discharge test.
	BatteryEnergyEntered
)

var batteryEnergyModeNames = [...]string{
	BatteryEnergyModeUnknown:  "unknown",
	BatteryEnergyFromCapacity: "capacity and nominal voltage",
	BatteryEnergyEntered:      "entered energy",
}

// String returns the mode's readable name.
func (m BatteryEnergyMode) String() string {
	if int(m) < len(batteryEnergyModeNames) {
		return batteryEnergyModeNames[m]
	}
	return "unknown"
}

// Battery is the flight pack: where its energy comes from, how much of that
// energy may actually be used, and what current it may deliver.
//
// It carries no mass. The pack's mass is a component in the design's inventory
// like every other mass, and Component names it, so that changing the pack
// changes the balance and the all-up mass through exactly one path. A Battery
// with its own mass field would be the second authoritative copy this package
// spends its structure avoiding, and it is how a pack gets counted twice.
type Battery struct {
	// Basis states where the pack's figures came from.
	Basis string
	// Component names the mass item that carries the pack's mass.
	Component string
	// Capacity is the labelled charge capacity, for BatteryEnergyFromCapacity.
	Capacity Quantity
	// NominalVoltage is the pack's nominal voltage, for the same mode.
	NominalVoltage Quantity
	// Energy is a directly supplied energy, for BatteryEnergyEntered.
	Energy Quantity
	// ContinuousCurrentLimit is the largest current the pack is rated to deliver
	// continuously.
	ContinuousCurrentLimit Quantity
	// PeakCurrentLimit is its short-term rating.
	PeakCurrentLimit Quantity
	// UsableFraction is how much of the nominal energy may actually be drawn,
	// after voltage sag, cell balance and the state of charge a lithium pack is
	// not taken below. It is not a reserve: a reserve is a mission decision and
	// is held on the mission.
	UsableFraction float64
	// Mode selects where the nominal energy comes from.
	Mode BatteryEnergyMode
	// Evidence grades the basis.
	Evidence EvidenceQuality
}

func (b Battery) supplied() bool { return b != Battery{} }

func (b Battery) validate(d Design, rs *resultSet) {
	if !b.supplied() {
		return
	}
	if b.Basis == "" {
		rs.add("battery", IssueMissing, "state where the pack's figures came from")
	}
	if b.Evidence == EvidenceUnstated {
		rs.add("battery", IssueMissing, "grade the evidence behind the pack's figures")
	}
	b.validateEnergyMode(rs)
	if b.UsableFraction <= 0 || b.UsableFraction > 1 || !isFinite(b.UsableFraction) {
		rs.add("battery.usable_fraction", IssueMissing,
			"state what fraction of the pack's nominal energy may actually be drawn, as a "+
				"fraction greater than zero and at most one. This is not a mission reserve")
	}
	b.validateComponentLink(d, rs)
}

func (b Battery) validateEnergyMode(rs *resultSet) {
	switch b.Mode {
	case BatteryEnergyFromCapacity:
		if !b.Capacity.supplied() {
			rs.add("battery.capacity", IssueMissing, "state the pack's labelled capacity")
		}
		if !b.NominalVoltage.supplied() {
			rs.add("battery.nominal_voltage", IssueMissing, "state the pack's nominal voltage")
		}
		if b.Energy.supplied() {
			rs.add("battery.energy", IssueInvalid,
				"this pack takes its energy from the capacity and the nominal voltage, so a "+
					"separately entered energy would be a second answer to the same question")
		}
	case BatteryEnergyEntered:
		if !b.Energy.supplied() {
			rs.add("battery.energy", IssueMissing, "state the pack's energy")
		}
	default:
		rs.add("battery.mode", IssueMissing,
			"say where the pack's energy comes from: the labelled capacity and nominal voltage, "+
				"or an energy measured directly")
	}
}

// validateComponentLink checks that the pack's mass is carried by exactly one
// component in the inventory. This is the structural half of "do not add the
// battery's mass twice": the Battery has no mass of its own to add.
func (b Battery) validateComponentLink(d Design, rs *resultSet) {
	if b.Component == "" {
		rs.add("battery.component", IssueMissing,
			"name the component that carries the pack's mass, so that the pack is weighed in "+
				"exactly one place; the battery definition holds no mass of its own")
		return
	}
	item, ok := d.Component(b.Component)
	if !ok {
		rs.add("battery.component", IssueMissing,
			"the design lists no component named "+b.Component+"; it lists "+
				joinNames(d.componentNames()))
		return
	}
	if item.Role != ComponentBattery {
		rs.add("battery.component", IssueInvalid,
			"component "+b.Component+" is listed as "+item.Role.String()+
				", not as the battery")
	}
}

// NominalEnergy returns the pack's nominal energy in the mode it selected.
func (b Battery) NominalEnergy() (Result, error) {
	switch b.Mode {
	case BatteryEnergyFromCapacity:
		return BatteryEnergyFromChargeAndVoltage(b.Capacity, b.NominalVoltage)
	case BatteryEnergyEntered:
		if !b.Energy.supplied() {
			return Result{}, Issues{{
				Field: "battery.energy", Kind: IssueMissing, Detail: "no pack energy is supplied",
			}}
		}
		return Result{Value: b.Energy}, nil
	default:
		return Result{}, Issues{{
			Field: "battery.mode", Kind: IssueMissing,
			Detail: "say where the pack's energy comes from before reading an energy off it",
		}}
	}
}

// UsableEnergy returns the energy the pack may actually deliver.
func (b Battery) UsableEnergy() (Result, error) {
	nominal, err := b.NominalEnergy()
	if err != nil {
		return Result{}, err
	}
	return BatteryUsableEnergy(nominal.Value, b.UsableFraction)
}

// Propulsion is the design's propulsion definition: what the chain is measured
// to deliver, how efficiently it converts electrical power into useful work,
// the ratings a feasibility claim is made against, and the thrust-to-weight
// targets it is judged by.
type Propulsion struct {
	// Capabilities are the measured or estimated operating points.
	Capabilities []PropulsionCapability
	// Targets are the thrust-to-weight targets, each stated at one point.
	Targets []ThrustTarget
	// Efficiency is the combined chain efficiency.
	Efficiency PropulsionEfficiency
	// Limits are the component ratings.
	Limits PropulsionLimits
}

func (p Propulsion) clone() Propulsion {
	c := p
	c.Capabilities = append([]PropulsionCapability(nil), p.Capabilities...)
	c.Targets = append([]ThrustTarget(nil), p.Targets...)
	return c
}

// Capability returns the capability point with the given name.
func (p Propulsion) Capability(name string) (PropulsionCapability, bool) {
	for n := range p.Capabilities {
		if p.Capabilities[n].Name == name {
			return p.Capabilities[n], true
		}
	}
	return PropulsionCapability{}, false
}

// capabilityNames returns every capability point's name, sorted.
func (p Propulsion) capabilityNames() []string {
	names := make([]string, 0, len(p.Capabilities))
	for n := range p.Capabilities {
		names = append(names, p.Capabilities[n].Name)
	}
	sort.Strings(names)
	return names
}

func (p Propulsion) validate(rs *resultSet) {
	p.Efficiency.validate("propulsion.efficiency", rs)
	seen := make(map[string]bool, len(p.Capabilities))
	for n := range p.Capabilities {
		c := p.Capabilities[n]
		if seen[c.Name] && c.Name != "" {
			rs.add("capability."+c.Name, IssueInvalid, "the design lists this capability point twice")
		}
		seen[c.Name] = true
		c.validate(rs)
	}
	p.validateTargets(rs)
	p.validateLimits(rs)
}

func (p Propulsion) validateTargets(rs *resultSet) {
	seen := make(map[string]bool, len(p.Targets))
	for _, target := range p.Targets {
		field := "thrust_target." + target.Name
		if target.Name == "" {
			rs.add("thrust_target", IssueMissing, "name every thrust-to-weight target")
			continue
		}
		if seen[target.Name] {
			rs.add(field, IssueInvalid, "the design states this target twice")
		}
		seen[target.Name] = true
		if target.Basis == "" {
			rs.add(field, IssueMissing, "state where the thrust-to-weight target came from")
		}
		if target.Priority == PriorityUnknown {
			rs.add(field, IssueMissing,
				"state whether the target is required or preferred")
		}
		if target.Ratio <= 0 || !isFinite(target.Ratio) {
			rs.add(field, IssueInvalid,
				"a thrust-to-weight target is greater than zero, received "+formatFloat(target.Ratio))
		}
		if target.Capability == "" {
			rs.add(field, IssueMissing,
				"name the capability point this target is stated at; a target with no condition "+
					"cannot be checked against a measurement taken at one")
			continue
		}
		if _, ok := p.Capability(target.Capability); !ok {
			rs.add(field, IssueMissing,
				"names capability point "+target.Capability+", which the design does not define; "+
					"it defines "+joinNames(p.capabilityNames()))
		}
	}
}

func (p Propulsion) validateLimits(rs *resultSet) {
	limits := p.Limits
	stated := limits.MaxContinuousElectricalPower.supplied() || limits.MaxPeakElectricalPower.supplied() ||
		limits.MaxRPM.supplied() || limits.MaxVoltage.supplied() ||
		limits.PropellerDiameter.supplied() || limits.PropellerHubHeight.supplied()
	if stated && limits.Basis == "" {
		rs.add("propulsion.limits", IssueMissing,
			"state where the component ratings came from")
	}
	if limits.PropellerHubHeight.supplied() != limits.PropellerDiameter.supplied() {
		rs.add("propulsion.limits.propeller", IssueMissing,
			"a tip clearance needs both the propeller diameter and the hub height above the "+
				"ground line; one without the other establishes nothing")
	}
}

// PropellerClearance returns the propeller's tip clearance above the ground
// line: the hub height less the propeller's radius. A negative result means the
// tips reach the ground before the aircraft does, which is reported as the
// negative number it is rather than refused.
func (p PropulsionLimits) PropellerClearance() (Result, error) {
	return PropellerTipClearance(p.PropellerHubHeight, p.PropellerDiameter)
}
