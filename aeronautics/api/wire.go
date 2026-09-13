// Package api is the transport-neutral application boundary over
// yalb.aero/calculator. It owns the wire contract — field names, enum tokens,
// units as symbols, request identity and payload limits — and the translation
// between that contract and the core's types.
//
// It contains no equation and no solver decision. Every number in a response
// came out of the calculator package, and every rejection here is about the
// shape of a request rather than about physics; a physically impossible input
// is refused by the core and reported through its own typed issues.
//
// The package deliberately does not import net/http. Status codes, routing,
// body limits and serialization live in yalb.aero/httpapi, and a future MCP
// sidecar reuses this package directly rather than making loopback HTTP calls.
// TestApplicationBoundaryDoesNotDependOnHTTP holds that separation.
package api

// ContractVersion identifies the wire contract. It changes when a field is
// removed, renamed or given a new meaning; adding an optional field does not
// change it.
const ContractVersion = "v1"

// Quantity is a value with the symbol of the unit it is expressed in. Symbols
// are the calculator's own unit table, read through calculator.ParseUnit, so
// the wire never carries a second copy of a conversion factor.
type Quantity struct {
	// Unit is the unit symbol, for example "m", "dm^2" or "oz/ft^2".
	Unit string `json:"unit"`
	// Value is the number in that unit.
	Value float64 `json:"value"`
}

// Issue is one field-specific reason a request could not produce a result. It
// mirrors calculator.Issue, including the distinction between a missing input, an
// impossible one and one outside the implemented model.
type Issue struct {
	// Field names the input, using the core's port or field name.
	Field string `json:"field"`
	// Kind is "missing", "invalid" or "unsupported".
	Kind string `json:"kind"`
	// Detail explains the problem in terms a worksheet can show.
	Detail string `json:"detail"`
}

// Request is the evaluation identity a client attaches to a call.
//
// The service is stateless: it holds no session and mints no identity. Whoever
// owns the design history — a browser worksheet, an MCP client — owns the
// identity too, and the boundary carries it verbatim so that a late response can
// be matched against the request that is still outstanding. The response also
// carries the input snapshot the service computed, so a client can tell a stale
// answer from a current one even if it loses track of its own requests.
type Request struct {
	// Session names the client's session.
	Session string `json:"session"`
	// Sequence is that session's request number.
	Sequence uint64 `json:"sequence"`
}

// Airfoil is a section identity and the evidence behind it. It carries no
// polar: nothing in this system derives lift or drag from a designation.
type Airfoil struct {
	Designation string `json:"designation"`
	Evidence    string `json:"evidence"`
	// ThicknessRatio is t/c where known, and zero where it is not stated.
	ThicknessRatio float64 `json:"thicknessRatio"`
}

// Wing is a wing definition. An absent quantity means "not supplied", which is
// a different answer from a supplied zero: an unstated sweep is a missing field
// and a stated zero is an unswept wing.
type Wing struct {
	// Span, Area and RootChord are size drivers. Exactly two of these three and
	// AspectRatio may be supplied.
	Span      *Quantity `json:"span,omitempty"`
	Area      *Quantity `json:"area,omitempty"`
	RootChord *Quantity `json:"rootChord,omitempty"`
	// Sweep, Dihedral, Twist and Incidence must be stated, with zero spelled out.
	Sweep     *Quantity `json:"sweep,omitempty"`
	Dihedral  *Quantity `json:"dihedral,omitempty"`
	Twist     *Quantity `json:"twist,omitempty"`
	Incidence *Quantity `json:"incidence,omitempty"`
	// BodyWidth is optional; without it no exposed area is reported.
	BodyWidth   *Quantity `json:"bodyWidth,omitempty"`
	RootAirfoil *Airfoil  `json:"rootAirfoil,omitempty"`
	TipAirfoil  *Airfoil  `json:"tipAirfoil,omitempty"`
	Name        string    `json:"name"`
	// Shape is "rectangle" or "trapezoid".
	Shape string `json:"shape"`
	// AreaBasis states what a supplied area measures.
	AreaBasis string `json:"areaBasis"`
	// DihedralMode states which dimensions stay fixed as dihedral changes. It
	// may be empty only at zero dihedral, where the two planes coincide.
	DihedralMode string `json:"dihedralMode,omitempty"`
	// AspectRatio is a size driver; zero means it is not one.
	AspectRatio float64 `json:"aspectRatio"`
	// TaperRatio is lambda = c_tip/c_root, required for a trapezoid.
	TaperRatio float64 `json:"taperRatio"`
	// SweepReference is the chord fraction Sweep is measured at.
	SweepReference float64 `json:"sweepReference"`
}

// Surface is one conventional tail surface.
type Surface struct {
	Area *Quantity `json:"area,omitempty"`
	Span *Quantity `json:"span,omitempty"`
	Arm  *Quantity `json:"arm,omitempty"`
}

// VTail records a V-tail as two canted panels rather than as an equivalent
// horizontal and vertical tail, which would be a modelling claim.
type VTail struct {
	PanelArea *Quantity `json:"panelArea,omitempty"`
	PanelSpan *Quantity `json:"panelSpan,omitempty"`
	Cant      *Quantity `json:"cant,omitempty"`
	Arm       *Quantity `json:"arm,omitempty"`
}

// Tail is whichever tail description the configuration calls for.
type Tail struct {
	Horizontal *Surface `json:"horizontal,omitempty"`
	Vertical   *Surface `json:"vertical,omitempty"`
	VTail      *VTail   `json:"vtail,omitempty"`
}

// CLmax is a maximum lift coefficient with its provenance and the grade of the
// evidence behind it.
type CLmax struct {
	// Scope is "aircraft" or "airfoil-section". A section value is refused
	// where a whole-aircraft value is required, rather than reinterpreted.
	Scope string `json:"scope"`
	// Basis states where the value came from.
	Basis string `json:"basis"`
	// Evidence grades that basis: "assumed", "measured" or "simulated".
	Evidence string `json:"evidence,omitempty"`
	// Max is the coefficient itself.
	Max float64 `json:"max"`
}

// Case is a flight condition and whether the candidate must hold it.
type Case struct {
	Name           string    `json:"name"`
	Configuration  string    `json:"configuration"`
	DensityBasis   string    `json:"densityBasis"`
	ViscosityBasis string    `json:"viscosityBasis,omitempty"`
	Priority       string    `json:"priority"`
	Density        *Quantity `json:"density,omitempty"`
	Viscosity      *Quantity `json:"viscosity,omitempty"`
	CLmax          CLmax     `json:"clmax"`
	LoadFactor     float64   `json:"loadFactor"`
}

// Requirement is a bound the candidate is judged against. It is not a driver:
// a maximum span does not set the span.
type Requirement struct {
	Minimum *Quantity `json:"minimum,omitempty"`
	Maximum *Quantity `json:"maximum,omitempty"`
	Name    string    `json:"name"`
	// Subject names the bounded quantity.
	Subject string `json:"subject"`
	// Priority is "required" or "preferred".
	Priority string `json:"priority"`
	Basis    string `json:"basis"`
	// Cases names the flight cases a per-case requirement applies to.
	Cases []string `json:"cases,omitempty"`
	// Margin is a relative safety margin that tightens both bounds.
	Margin float64 `json:"margin"`
}

// Position is a component's centre of mass in the design's datum. Each
// coordinate is optional on the wire and absent means "not stated": a component
// may be listed before it has been placed, and an unstated y is a missing field
// rather than a claim that the component sits on the centerline.
type Position struct {
	X *Quantity `json:"x,omitempty"`
	Y *Quantity `json:"y,omitempty"`
	Z *Quantity `json:"z,omitempty"`
}

// Component is one mass and where it sits. It carries no physics: a role groups
// and labels the item and implies no mass, no power draw and no attachment.
type Component struct {
	Mass     *Quantity `json:"mass,omitempty"`
	Position Position  `json:"position"`
	Name     string    `json:"name"`
	// Role is "airframe", "battery", "motor", "avionics", "payload" or "other".
	Role string `json:"role"`
	// Basis states where the mass came from.
	Basis string `json:"basis"`
}

// DragPolar is the aircraft drag polar and the evidence behind it. Both
// coefficients are supplied: nothing in this system estimates a zero-lift drag
// coefficient, and the Oswald correlation is offered rather than applied.
type DragPolar struct {
	// CD0Basis states where CD0 came from. It must describe the whole aircraft.
	CD0Basis string `json:"cd0Basis"`
	// EfficiencyBasis states where the Oswald factor came from.
	EfficiencyBasis string `json:"efficiencyBasis"`
	// Configuration names the airframe configuration the polar describes.
	Configuration string `json:"configuration"`
	// Scope is "aircraft" or "airfoil-section". A section polar is refused
	// where an aircraft polar is required, rather than reinterpreted.
	Scope string `json:"scope"`
	// Evidence grades the coefficients: "assumed", "measured" or "simulated".
	Evidence string `json:"evidence,omitempty"`
	// CD0 is the zero-lift drag coefficient.
	CD0 float64 `json:"cd0"`
	// OswaldEfficiency is the span efficiency factor e.
	OswaldEfficiency float64 `json:"oswaldEfficiency"`
	// CLValidMin and CLValidMax bound the lift coefficients the polar is
	// claimed over. They are required: a parabolic polar is symmetric in CL and
	// returns a drag coefficient at lift coefficients the aircraft cannot reach.
	CLValidMin float64 `json:"clValidMin"`
	// CLValidMax is the upper end of that range.
	CLValidMax float64 `json:"clValidMax"`
}

// Capability is what the propulsion chain was measured or estimated to deliver
// at one explicitly named operating condition. Every condition field is part of
// the claim: the same motor and propeller on a different pack is a different
// point, and static thrust is never cruise thrust.
type Capability struct {
	Speed           *Quantity `json:"speed,omitempty"`
	Density         *Quantity `json:"density,omitempty"`
	Voltage         *Quantity `json:"voltage,omitempty"`
	RPM             *Quantity `json:"rpm,omitempty"`
	Thrust          *Quantity `json:"thrust,omitempty"`
	ElectricalPower *Quantity `json:"electricalPower,omitempty"`
	Current         *Quantity `json:"current,omitempty"`
	Name            string    `json:"name"`
	Basis           string    `json:"basis"`
	DensityBasis    string    `json:"densityBasis,omitempty"`
	Note            string    `json:"note,omitempty"`
	// Kind is "static" or "in-flight".
	Kind     string `json:"kind"`
	Evidence string `json:"evidence,omitempty"`
	// Throttle is the setting as a fraction of full, from 0 to 1.
	Throttle float64 `json:"throttle"`
}

// ThrustTarget is a required or preferred thrust-to-weight ratio, stated at the
// condition of one named capability point.
type ThrustTarget struct {
	Name  string `json:"name"`
	Basis string `json:"basis"`
	// Capability names the point whose condition the target is stated at.
	Capability string `json:"capability"`
	// Priority is "required" or "preferred".
	Priority string  `json:"priority"`
	Ratio    float64 `json:"ratio"`
}

// PropulsionLimits are the component ratings a feasibility claim is made
// against. An absent rating is reported as an unchecked limit, never as a
// satisfied one.
type PropulsionLimits struct {
	MaxContinuousPower *Quantity `json:"maxContinuousPower,omitempty"`
	MaxPeakPower       *Quantity `json:"maxPeakPower,omitempty"`
	MaxRPM             *Quantity `json:"maxRpm,omitempty"`
	MaxVoltage         *Quantity `json:"maxVoltage,omitempty"`
	PropellerDiameter  *Quantity `json:"propellerDiameter,omitempty"`
	PropellerHubHeight *Quantity `json:"propellerHubHeight,omitempty"`
	Basis              string    `json:"basis,omitempty"`
}

// Propulsion is the design's propulsion definition.
type Propulsion struct {
	Limits *PropulsionLimits `json:"limits,omitempty"`
	// EfficiencyBasis states where the chain efficiency came from and at what
	// condition.
	EfficiencyBasis    string         `json:"efficiencyBasis,omitempty"`
	EfficiencyEvidence string         `json:"efficiencyEvidence,omitempty"`
	Capabilities       []Capability   `json:"capabilities,omitempty"`
	Targets            []ThrustTarget `json:"targets,omitempty"`
	// Efficiency is the combined propeller, motor and speed-controller
	// efficiency. Zero means it is not stated.
	Efficiency float64 `json:"efficiency"`
}

// Battery is the flight pack. It carries no mass: Component names the mass item
// that does, which is what stops a pack from being weighed twice.
type Battery struct {
	Capacity               *Quantity `json:"capacity,omitempty"`
	NominalVoltage         *Quantity `json:"nominalVoltage,omitempty"`
	Energy                 *Quantity `json:"energy,omitempty"`
	ContinuousCurrentLimit *Quantity `json:"continuousCurrentLimit,omitempty"`
	PeakCurrentLimit       *Quantity `json:"peakCurrentLimit,omitempty"`
	Basis                  string    `json:"basis"`
	Component              string    `json:"component"`
	// Mode is "capacity-and-voltage" or "entered-energy".
	Mode     string `json:"mode"`
	Evidence string `json:"evidence,omitempty"`
	// UsableFraction is how much of the nominal energy may be drawn. It is not
	// a mission reserve.
	UsableFraction float64 `json:"usableFraction"`
}

// AuxiliaryLoad is one non-propulsive electrical draw. Continuous and peak
// answer different questions and are never merged.
type AuxiliaryLoad struct {
	Continuous *Quantity `json:"continuous,omitempty"`
	Peak       *Quantity `json:"peak,omitempty"`
	Name       string    `json:"name"`
	Basis      string    `json:"basis"`
	// Component optionally names the mass item this draw belongs to.
	Component string `json:"component,omitempty"`
	// Side is "pack-side" or "load-side": which side of the regulator the
	// figures were measured on. Without it the regulator loss is either counted
	// twice or not at all.
	Side     string `json:"side"`
	Evidence string `json:"evidence,omitempty"`
	// RegulatorEfficiency converts a load-side figure to a pack-side one. It is
	// required on the load side and refused on the pack side.
	RegulatorEfficiency float64 `json:"regulatorEfficiency"`
}

// MissionSegment is one editable leg of a mission.
type MissionSegment struct {
	Speed          *Quantity `json:"speed,omitempty"`
	ClimbAngle     *Quantity `json:"climbAngle,omitempty"`
	Duration       *Quantity `json:"duration,omitempty"`
	Distance       *Quantity `json:"distance,omitempty"`
	WindAlongTrack *Quantity `json:"windAlongTrack,omitempty"`
	EnteredPower   *Quantity `json:"enteredPower,omitempty"`
	Name           string    `json:"name"`
	// Case names the flight case supplying the density, CLmax and load factor.
	Case  string `json:"case"`
	Notes string `json:"notes,omitempty"`
	// Kind is "launch", "climb", "cruise", "loiter", "return", "recovery" or
	// "other". It carries no physics.
	Kind string `json:"kind"`
	// Model is "drag-polar" or "entered-estimate".
	Model string `json:"model"`
	// Timing is "duration" or "ground-distance": exactly one is stated and the
	// other follows from the ground speed.
	Timing          string `json:"timing"`
	EnteredBasis    string `json:"enteredBasis,omitempty"`
	EnteredEvidence string `json:"enteredEvidence,omitempty"`
	EfficiencyBasis string `json:"efficiencyBasis,omitempty"`
	// Capability optionally names the point the required thrust is checked
	// against. Without it the segment's flight feasibility is unknown and its
	// energy still counts.
	Capability string `json:"capability,omitempty"`
	// Efficiency optionally overrides the design chain efficiency here.
	Efficiency float64 `json:"efficiency"`
}

// Mission is the flight the energy budget is formed over. Segments keep their
// stated order: a mission is a sequence.
type Mission struct {
	Name         string           `json:"name"`
	Basis        string           `json:"basis"`
	ReserveBasis string           `json:"reserveBasis,omitempty"`
	Segments     []MissionSegment `json:"segments,omitempty"`
	// ReserveFraction is applied exactly once, to the pack's usable energy.
	ReserveFraction float64 `json:"reserveFraction"`
}

// Design is the authoritative parametric definition on the wire. It is the
// whole request state: the service holds none of it between calls.
type Design struct {
	Mass          *Quantity `json:"mass,omitempty"`
	Tail          *Tail     `json:"tail,omitempty"`
	Name          string    `json:"name"`
	Configuration string    `json:"configuration"`
	MassBasis     string    `json:"massBasis"`
	// MassMode is "entered" or "components": whether the all-up mass is the
	// entered figure or the sum of the component inventory.
	MassMode     string        `json:"massMode,omitempty"`
	Components   []Component   `json:"components,omitempty"`
	Cases        []Case        `json:"cases,omitempty"`
	Requirements []Requirement `json:"requirements,omitempty"`
	// Polar is the aircraft drag polar the power model reads.
	Polar *DragPolar `json:"polar,omitempty"`
	// Propulsion is what the chain delivers, its ratings and its targets.
	Propulsion *Propulsion `json:"propulsion,omitempty"`
	// Battery is the flight pack, whose mass is a component.
	Battery *Battery `json:"battery,omitempty"`
	// Auxiliary are the non-propulsive electrical loads.
	Auxiliary []AuxiliaryLoad `json:"auxiliary,omitempty"`
	// Mission is the flight the energy budget covers.
	Mission *Mission `json:"mission,omitempty"`
	Wing    Wing     `json:"wing"`
}

// Scope names the cases an action covers. It is required rather than defaulted:
// sizing against one case and against every required case are different
// actions with different answers.
type Scope struct {
	// Kind is "all-required" or "single".
	Kind string `json:"kind"`
	// Case names the case, for "single" only.
	Case string `json:"case,omitempty"`
}

// Command is one curated edit against a design. The supported edits are a
// closed set: there is no expression language and no arbitrary code.
//
// It is a flat tagged union rather than a nested one so that a single strict
// decode pass can reject an unknown field, and so that the per-kind field rules
// can name exactly which field does not belong. commandSpecs holds those rules.
type Command struct {
	Mass        *Quantity    `json:"mass,omitempty"`
	Value       *Quantity    `json:"value,omitempty"`
	Requirement *Requirement `json:"requirement,omitempty"`
	Case        *Case        `json:"case,omitempty"`
	CLmax       *CLmax       `json:"clmax,omitempty"`
	Scope       *Scope       `json:"scope,omitempty"`
	Angles      *Angles      `json:"angles,omitempty"`
	Tail        *Tail        `json:"tail,omitempty"`
	Component   *Component   `json:"component,omitempty"`
	Position    *Position    `json:"position,omitempty"`

	// The Task 09 union members: the drag polar, the propulsion chain, the pack,
	// one auxiliary load, the mission header and one mission segment.
	Polar      *DragPolar        `json:"polar,omitempty"`
	Efficiency *Efficiency       `json:"efficiency,omitempty"`
	Limits     *PropulsionLimits `json:"limits,omitempty"`
	Capability *Capability       `json:"capability,omitempty"`
	Target     *ThrustTarget     `json:"target,omitempty"`
	Battery    *Battery          `json:"battery,omitempty"`
	Load       *AuxiliaryLoad    `json:"load,omitempty"`
	Profile    *MissionProfile   `json:"profile,omitempty"`
	Segment    *MissionSegment   `json:"segment,omitempty"`

	// Kind selects the edit.
	Kind string `json:"kind"`

	Basis    string `json:"basis,omitempty"`
	Key      string `json:"key,omitempty"`
	Promote  string `json:"promote,omitempty"`
	Release  string `json:"release,omitempty"`
	Name     string `json:"name,omitempty"`
	Priority string `json:"priority,omitempty"`
	Hold     string `json:"hold,omitempty"`
	// Shape is the planform shape, for the shape edit.
	Shape string `json:"shape,omitempty"`
	// Configuration is the airframe layout, for the configuration edit.
	Configuration string `json:"configuration,omitempty"`
	// Mode is the mass mode, for the mass-mode edit.
	Mode string `json:"mode,omitempty"`

	// Ratio is the taper ratio, for the shape and taper-ratio edits.
	Ratio float64 `json:"ratio,omitempty"`
	// Index is the position a mission segment moves to, counting from zero.
	Index int `json:"index,omitempty"`
}

// Angles is every stated wing angle. They travel together because the geometry
// model requires all of them: an unstated sweep is a missing field rather than
// zero, so setting one and leaving another unset would produce a wing that
// cannot solve for a reason the builder did not choose.
type Angles struct {
	Sweep     *Quantity `json:"sweep"`
	Dihedral  *Quantity `json:"dihedral"`
	Twist     *Quantity `json:"twist"`
	Incidence *Quantity `json:"incidence"`
	// DihedralMode may be empty only at zero dihedral, where the planes coincide.
	DihedralMode string `json:"dihedralMode,omitempty"`
	// SweepReference is the chord fraction Sweep is measured at.
	SweepReference float64 `json:"sweepReference"`
}

// Substitution is one input value an evaluation actually used.
type Substitution struct {
	Name  string   `json:"name"`
	Value Quantity `json:"value"`
}

// Trace is the inspectable record of one evaluation: which equation at which
// revision, its expression, the values substituted and the result. It carries
// no timestamp, because the core reaches no clock.
type Trace struct {
	EquationID    string         `json:"equationId"`
	Revision      string         `json:"revision"`
	Expression    string         `json:"expression"`
	Substitutions []Substitution `json:"substitutions"`
	Result        Quantity       `json:"result"`
}

// Parameter is one named value in a solved design, with its role, unit, datum
// and, for a derived value, the equation revision behind it.
type Parameter struct {
	Key        string   `json:"key"`
	Role       string   `json:"role"`
	Datum      string   `json:"datum,omitempty"`
	EquationID string   `json:"equationId,omitempty"`
	Revision   string   `json:"revision,omitempty"`
	DependsOn  []string `json:"dependsOn,omitempty"`
	Value      Quantity `json:"value"`
}

// Point is a coordinate in the wing datum.
type Point struct {
	X Quantity `json:"x"`
	Y Quantity `json:"y"`
	Z Quantity `json:"z"`
}

// SolvedWing is the geometry a successful evaluation produced, expressed as the
// parameter set rather than as a second set of named fields, so the transport
// carries no vocabulary the core does not already own.
type SolvedWing struct {
	// Datum names the coordinate convention every coordinate here uses.
	Datum string `json:"datum"`
	// SolveMode names the driver pair the planform was solved from.
	SolveMode string `json:"solveMode"`
	// Drivers are the size drivers currently held, named in their own plane.
	Drivers    []string    `json:"drivers"`
	Parameters []Parameter `json:"parameters"`
	// Outline is the right panel's plan-view corners.
	Outline []Point `json:"outline"`
	// Views are the dimensioned plan, front and side views. Every dimension
	// names the parameter it measures, which is what lets a worksheet map a
	// click on a drawing onto a field and back.
	Views []SketchView `json:"views"`
	// Explanations are the relationship behind each parameter: the expression,
	// the revision and the values actually substituted.
	Explanations []Explanation `json:"explanations"`
}

// SketchCurve is one drawn line in a view.
type SketchCurve struct {
	Label string `json:"label"`
	// Role is "outline", "centerline", "axis", "construction" or "reference".
	Role   string  `json:"role"`
	Points []Point `json:"points"`
	// Mirrored reports that the left panel is this curve's mirror in y. It is
	// stated rather than drawn twice, so a consumer cannot mistake the mirror
	// for a second, independently solved panel.
	Mirrored bool `json:"mirrored"`
	// Closed reports that the last point joins the first.
	Closed bool `json:"closed"`
}

// SketchDimension is one dimension on a view, tied to the parameter it measures.
type SketchDimension struct {
	// Key names the parameter this dimension measures, using the core's own
	// stable key. Selecting the dimension selects that field, and selecting the
	// field highlights this dimension.
	Key    string `json:"key"`
	Label  string `json:"label"`
	Detail string `json:"detail"`
	// Kind is "linear" or "angular".
	Kind string `json:"kind"`
	// Plane is "plan-view" or "panel-surface": whether the dimension is a
	// projection or a dimension of the panel as built.
	Plane string   `json:"plane"`
	From  Point    `json:"from"`
	To    Point    `json:"to"`
	Value Quantity `json:"value"`
}

// SketchView is one orthographic view: what to draw and what to dimension.
type SketchView struct {
	// View is "plan-view", "front-view" or "side-view".
	View string `json:"view"`
	// Datum names the coordinate convention every point here uses.
	Datum string `json:"datum"`
	// Across and Up name the datum axes that run left to right and up the page.
	Across     string            `json:"across"`
	Up         string            `json:"up"`
	Curves     []SketchCurve     `json:"curves"`
	Dimensions []SketchDimension `json:"dimensions"`
}

// Explanation is why one parameter holds the value it does.
type Explanation struct {
	Key string `json:"key"`
	// Role is "driver" or "derived".
	Role       string `json:"role"`
	EquationID string `json:"equationId,omitempty"`
	Revision   string `json:"revision,omitempty"`
	// Expression is the symbolic relationship, for example c_root = 2S/(b(1+lambda)).
	Expression string `json:"expression,omitempty"`
	Detail     string `json:"detail"`
	// Substitutions are the values actually used, in the order the equation
	// consumed them. It is empty for a driver: nothing was substituted.
	Substitutions []Substitution `json:"substitutions"`
	DependsOn     []string       `json:"dependsOn"`
	Value         Quantity       `json:"value"`
}

// CaseLoad is the lift one flight case demands of the whole aircraft.
//
// It is a magnitude and nothing else. The lumped model puts the entire load on
// the wing and solves no line of action, no spanwise distribution and no tail
// balancing load, so nothing here says where that force acts.
type CaseLoad struct {
	RequiredLift *Quantity `json:"requiredLift,omitempty"`
	Case         string    `json:"case"`
	// Priority is "required" or "preferred".
	Priority string `json:"priority"`
	// Status is "computed", "missing", "invalid" or "stale".
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// MassContribution is one component's contribution to the balance.
type MassContribution struct {
	Mass     *Quantity `json:"mass,omitempty"`
	Position Position  `json:"position"`
	Name     string    `json:"name"`
	Role     string    `json:"role"`
	Detail   string    `json:"detail,omitempty"`
	// Moments are the mass moments about the datum on x, y and z, in that order.
	Moments []Quantity `json:"moments"`
	// Known reports whether the component contributed. A component with no mass
	// or no complete position contributes nothing and is never placed at the
	// origin.
	Known bool `json:"known"`
}

// MassProperties is the mechanical balance of the listed components.
//
// It is mechanical only. The centre of gravity is not an aerodynamic centre, a
// neutral point or a centre of pressure, and no trim, static-margin or handling
// conclusion follows from it.
type MassProperties struct {
	Total *Quantity `json:"total,omitempty"`
	CG    *Point    `json:"cg,omitempty"`
	// Datum names the coordinate convention the centre of gravity and every
	// position use.
	Datum string `json:"datum"`
	// Status is "computed", "missing", "invalid" or "stale".
	Status string `json:"status"`
	// Detail explains an incomplete or unavailable result.
	Detail        string             `json:"detail,omitempty"`
	Contributions []MassContribution `json:"contributions"`
	// Complete reports that every listed component contributed. A false value
	// with a computed status means the result describes a subset of the design.
	Complete bool `json:"complete"`
}

// Check is one requirement bound's outcome.
type Check struct {
	Name string `json:"name"`
	// Case names the flight case, for a per-case subject only.
	Case      string `json:"case,omitempty"`
	Subject   string `json:"subject"`
	Direction string `json:"direction"`
	Priority  string `json:"priority"`
	// Status is "met", "unmet" or "unknown": whether the requirement holds.
	Status string `json:"status"`
	// Result is "computed", "missing", "invalid" or "stale": whether the model
	// produced the number at all. It is a separate answer from Status.
	Result string `json:"result"`
	// Evidence grades the case evidence behind Actual.
	Evidence string    `json:"evidence,omitempty"`
	Detail   string    `json:"detail,omitempty"`
	Actual   *Quantity `json:"actual,omitempty"`
	Trace    *Trace    `json:"trace,omitempty"`
	Bound    Quantity  `json:"bound"`
	// Margin is the achieved relative room inside the bound.
	Margin float64 `json:"margin"`
}

// Contribution is one case's or requirement's contribution to an intersected
// bound.
type Contribution struct {
	Value  *Quantity `json:"value,omitempty"`
	Trace  *Trace    `json:"trace,omitempty"`
	Source string    `json:"source"`
	Detail string    `json:"detail,omitempty"`
	// Known reports whether the source contributed a value.
	Known bool `json:"known"`
}

// Bound is an intersected sizing bound: the largest lower bound or the smallest
// upper bound over the applicable sources.
type Bound struct {
	Value     *Quantity `json:"value,omitempty"`
	Subject   string    `json:"subject"`
	Direction string    `json:"direction"`
	Detail    string    `json:"detail,omitempty"`
	// Controlling names the sources that set Value. More than one means a tie.
	Controlling   []string       `json:"controlling,omitempty"`
	Contributions []Contribution `json:"contributions,omitempty"`
	Known         bool           `json:"known"`
	// Partial reports that an applicable source could not contribute, so the
	// bound does not establish a complete feasible interval.
	Partial bool `json:"partial"`
}

// MassRange is the feasible all-up mass interval. A stall ceiling supplies an
// upper bound and justifies no nonzero lower one, so an interval with only one
// end is reported as one rather than as a range starting at zero.
type MassRange struct {
	Detail   string `json:"detail,omitempty"`
	Lower    Bound  `json:"lower"`
	Upper    Bound  `json:"upper"`
	Complete bool   `json:"complete"`
	Empty    bool   `json:"empty"`
}

// Alternative is a change that would resolve a conflict, carried as the command
// that would make it. It is offered, never applied.
type Alternative struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Command     Command `json:"command"`
}

// Conflict is a group of required constraints that cannot all hold. The group
// is a known conflicting set, not a minimal one: no solver runs.
type Conflict struct {
	Summary      string        `json:"summary"`
	Detail       string        `json:"detail"`
	Group        []string      `json:"group"`
	Alternatives []Alternative `json:"alternatives"`
}

// Change is one requirement outcome a previewed command would alter.
type Change struct {
	Name      string `json:"name"`
	Case      string `json:"case,omitempty"`
	Subject   string `json:"subject"`
	Direction string `json:"direction"`
	From      string `json:"from"`
	To        string `json:"to"`
	Detail    string `json:"detail"`
	Appeared  bool   `json:"appeared"`
	Vanished  bool   `json:"vanished"`
}

// AuxiliaryContribution is one auxiliary load's draw as the pack supplies it.
type AuxiliaryContribution struct {
	Continuous *Quantity `json:"continuous,omitempty"`
	Peak       *Quantity `json:"peak,omitempty"`
	Name       string    `json:"name"`
	Component  string    `json:"component,omitempty"`
	Side       string    `json:"side"`
	Evidence   string    `json:"evidence,omitempty"`
	Detail     string    `json:"detail,omitempty"`
	// Known reports whether the load contributed. A load with no stated
	// continuous draw contributes nothing and is never treated as drawing zero.
	Known bool `json:"known"`
}

// ElectricalBudget is the non-propulsive electrical demand. The continuous
// total is what the energy budget pays for; the peak is what the pack and the
// regulators have to survive, and the two are never merged.
type ElectricalBudget struct {
	Continuous *Quantity `json:"continuous,omitempty"`
	Peak       *Quantity `json:"peak,omitempty"`
	// Status is "computed", "missing", "invalid" or "stale". An empty load list
	// is missing, not zero: silence is not a statement that nothing draws power.
	Status   string                  `json:"status"`
	Detail   string                  `json:"detail,omitempty"`
	Evidence string                  `json:"evidence,omitempty"`
	Loads    []AuxiliaryContribution `json:"loads"`
	Complete bool                    `json:"complete"`
}

// SegmentAvailability is whether the propulsion system was measured to deliver
// the thrust a segment needs, at that segment's own condition.
type SegmentAvailability struct {
	AvailableThrust *Quantity `json:"availableThrust,omitempty"`
	Trace           *Trace    `json:"trace,omitempty"`
	Capability      string    `json:"capability,omitempty"`
	Detail          string    `json:"detail,omitempty"`
	Evidence        string    `json:"evidence,omitempty"`
	// Status is "met", "unmet" or "unknown".
	Status string  `json:"status"`
	Margin float64 `json:"margin"`
}

// SegmentPower is the power one segment requires and the chain behind it.
type SegmentPower struct {
	DynamicPressure *Quantity `json:"dynamicPressure,omitempty"`
	Lift            *Quantity `json:"lift,omitempty"`
	Drag            *Quantity `json:"drag,omitempty"`
	Thrust          *Quantity `json:"thrust,omitempty"`
	Propulsive      *Quantity `json:"propulsive,omitempty"`
	// PropulsiveElectrical is the chain draw before any auxiliary load, held
	// separately so a peak demand adds the peak auxiliary draw to it rather
	// than to a figure that already contains the continuous one.
	PropulsiveElectrical *Quantity `json:"propulsiveElectrical,omitempty"`
	Electrical           *Quantity `json:"electrical,omitempty"`
	// Model is "drag-polar" or "entered-estimate".
	Model string `json:"model"`
	// Status is "computed", "missing", "invalid" or "stale".
	Status          string  `json:"status"`
	Detail          string  `json:"detail,omitempty"`
	Evidence        string  `json:"evidence,omitempty"`
	Traces          []Trace `json:"traces"`
	LiftCoefficient float64 `json:"liftCoefficient"`
	DragCoefficient float64 `json:"dragCoefficient"`
	LiftToDrag      float64 `json:"liftToDrag"`
	ChainEfficiency float64 `json:"chainEfficiency"`
}

// SegmentResult is one mission segment fully evaluated.
type SegmentResult struct {
	GroundSpeed  *Quantity           `json:"groundSpeed,omitempty"`
	Duration     *Quantity           `json:"duration,omitempty"`
	Distance     *Quantity           `json:"distance,omitempty"`
	Energy       *Quantity           `json:"energy,omitempty"`
	Name         string              `json:"name"`
	Case         string              `json:"case,omitempty"`
	Kind         string              `json:"kind"`
	Status       string              `json:"status"`
	Detail       string              `json:"detail,omitempty"`
	Traces       []Trace             `json:"traces"`
	Availability SegmentAvailability `json:"availability"`
	Power        SegmentPower        `json:"power"`
}

// MissionResult is the mission's energy budget. Energy sufficiency and flight
// feasibility are separate answers and neither implies the other.
type MissionResult struct {
	RequiredEnergy      *Quantity `json:"requiredEnergy,omitempty"`
	UsableEnergy        *Quantity `json:"usableEnergy,omitempty"`
	Budget              *Quantity `json:"budget,omitempty"`
	TotalDuration       *Quantity `json:"totalDuration,omitempty"`
	TotalDistance       *Quantity `json:"totalDistance,omitempty"`
	PeakContinuousPower *Quantity `json:"peakContinuousPower,omitempty"`
	// Status is "computed", "missing", "invalid" or "stale".
	Status string `json:"status"`
	// EnergyStatus is "met", "unmet" or "unknown": whether the required energy
	// fits the budget.
	EnergyStatus string          `json:"energyStatus"`
	Detail       string          `json:"detail,omitempty"`
	Segments     []SegmentResult `json:"segments"`
	Traces       []Trace         `json:"traces"`
	// ReserveFraction echoes the reserve, applied exactly once.
	ReserveFraction float64 `json:"reserveFraction"`
	// Complete reports that every listed segment contributed.
	Complete bool `json:"complete"`
}

// SupplyCheck is one component rating against what the design demands of it. An
// unstated rating is an unknown check rather than an absent row.
type SupplyCheck struct {
	Limit  *Quantity `json:"limit,omitempty"`
	Actual *Quantity `json:"actual,omitempty"`
	Name   string    `json:"name"`
	Status string    `json:"status"`
	Detail string    `json:"detail,omitempty"`
	Margin float64   `json:"margin"`
}

// PowerFeasibility compares the electrical demand with the component ratings.
// It is a separate answer from the energy budget.
type PowerFeasibility struct {
	ContinuousDemand *Quantity `json:"continuousDemand,omitempty"`
	PeakDemand       *Quantity `json:"peakDemand,omitempty"`
	Status           string    `json:"status"`
	Detail           string    `json:"detail,omitempty"`
	// PeakDetail says how the worst-moment demand was constructed.
	PeakDetail string        `json:"peakDetail,omitempty"`
	Checks     []SupplyCheck `json:"checks"`
}

// ThrustCheck is one thrust-to-weight target against the capability point it
// names. The target and the measurement stay separate throughout.
type ThrustCheck struct {
	Trace      *Trace `json:"trace,omitempty"`
	Name       string `json:"name"`
	Capability string `json:"capability,omitempty"`
	// Condition describes the point's condition, so a ratio never appears
	// without the condition that gives it meaning.
	Condition string  `json:"condition,omitempty"`
	Detail    string  `json:"detail,omitempty"`
	Priority  string  `json:"priority"`
	Evidence  string  `json:"evidence,omitempty"`
	Status    string  `json:"status"`
	Available float64 `json:"available"`
	Target    float64 `json:"target"`
	Margin    float64 `json:"margin"`
}

// Evaluation is one complete assessment, tied to the request identity it
// answers and the input snapshot it was computed from.
type Evaluation struct {
	Wing     *SolvedWing `json:"wing,omitempty"`
	Snapshot string      `json:"snapshot"`
	// Geometry is "computed", "missing", "invalid" or "stale".
	Geometry string `json:"geometry"`
	// Aggregate is the combined status of the required checks. It makes no
	// feasibility claim when HasRequired is false: an empty required set is not
	// a passing one.
	Aggregate string `json:"aggregate"`
	// PowerFeasibility compares the electrical demand with the component ratings.
	PowerFeasibility PowerFeasibility `json:"powerFeasibility"`
	Request          Request          `json:"request"`
	// Loads are the lumped lift each case demands: a magnitude with no line of
	// action, from the Task 02 model.
	Loads               []CaseLoad `json:"loads"`
	DefinitionIssues    []Issue    `json:"definitionIssues"`
	GeometryIssues      []Issue    `json:"geometryIssues"`
	ConfigurationIssues []Issue    `json:"configurationIssues"`
	Conflicts           []Conflict `json:"conflicts"`
	// ThrustChecks are the thrust-to-weight targets against their points.
	ThrustChecks []ThrustCheck `json:"thrustChecks"`
	Patterns     []string      `json:"patterns"`
	Checks       []Check       `json:"checks"`
	AreaUpper    Bound         `json:"areaUpper"`
	AreaLower    Bound         `json:"areaLower"`
	// Electrical is the auxiliary demand on the pack side of every regulator.
	Electrical ElectricalBudget `json:"electrical"`
	// MassProperties is the mechanical balance of the listed components. It is
	// reported in either mass mode.
	MassProperties MassProperties `json:"massProperties"`
	Mass           MassRange      `json:"mass"`
	// Mission is the energy budget over the stated segments.
	Mission     MissionResult `json:"mission"`
	HasRequired bool          `json:"hasRequired"`
}

// Port names one input or output of an equation and fixes its dimension.
type Port struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Dimension   string `json:"dimension"`
	// Unit is the SI unit the dimension is held in.
	Unit string `json:"unit"`
}

// Source records where an equation comes from and how it was adapted.
type Source struct {
	// Kind is "book", "supplementary" or "derived". A supported subset of the
	// primary reference must never read as the whole of it.
	Kind             string `json:"kind"`
	Title            string `json:"title"`
	URL              string `json:"url"`
	Section          string `json:"section"`
	Accessed         string `json:"accessed"`
	UpstreamRevision string `json:"upstreamRevision,omitempty"`
	OriginalUnits    string `json:"originalUnits"`
	Adaptation       string `json:"adaptation"`
}

// Equation is one implemented calculation's published definition.
type Equation struct {
	ID         string `json:"id"`
	Revision   string `json:"revision"`
	Expression string `json:"expression"`
	Source     Source `json:"source"`
	Output     Port   `json:"output"`
	Inputs     []Port `json:"inputs"`
	// Assumptions are the stated conditions the equation holds under. They
	// travel with it, because a relation without them is an unqualified claim.
	Assumptions []string `json:"assumptions"`
}

// Pattern is a curated workflow with its rationale, inputs, drivers, outcome
// and validity limits.
type Pattern struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Journey        string   `json:"journey"`
	Rationale      string   `json:"rationale"`
	Outcome        string   `json:"outcome"`
	RequiredInputs []string `json:"requiredInputs"`
	ActiveDrivers  []string `json:"activeDrivers"`
	ValidityLimits []string `json:"validityLimits"`
	// Supported reports whether the system implements the workflow. An
	// unsupported pattern is listed so its absence is stated rather than silent.
	Supported bool `json:"supported"`
}

// UnitInfo is one supported unit and the dimension it belongs to.
type UnitInfo struct {
	Symbol    string `json:"symbol"`
	Dimension string `json:"dimension"`
	// FactorToSI converts a value in this unit to the dimension's SI unit by a
	// single multiplication. It is published so a display layer can render a
	// stored SI value in a chosen unit without keeping its own conversion table;
	// converting an *input* is the service's job, which is why a request carries
	// the unit symbol rather than a converted number.
	FactorToSI float64 `json:"factorToSi"`
	// SI reports whether the symbol is the dimension's own SI unit.
	SI bool `json:"si"`
}

// Vocabulary is the enum tokens the contract accepts, published so that a
// client does not have to hard-code them from documentation.
type Vocabulary struct {
	Name   string   `json:"name"`
	Tokens []string `json:"tokens"`
}

// Discovery is everything a client needs to build a request without reading
// this package's source: the contract version, the equations with their
// provenance, the curated workflows, the units and the enum vocabularies.
type Discovery struct {
	ContractVersion string       `json:"contractVersion"`
	Version         string       `json:"version"`
	Equations       []Equation   `json:"equations"`
	Patterns        []Pattern    `json:"patterns"`
	Units           []UnitInfo   `json:"units"`
	Vocabularies    []Vocabulary `json:"vocabularies"`
	Limits          Limits       `json:"limits"`
}

// Limits are the payload bounds the service enforces, published so that a
// client can respect them rather than discover them by being refused.
type Limits struct {
	MaxCases        int `json:"maxCases"`
	MaxRequirements int `json:"maxRequirements"`
	MaxCommands     int `json:"maxCommands"`
	MaxBatch        int `json:"maxBatch"`
	MaxComponents   int `json:"maxComponents"`
	// MaxSweepSamples is the largest sensitivity sweep the core evaluates in one
	// request. A sweep evaluates the whole design once per sample.
	MaxSweepSamples int `json:"maxSweepSamples"`
	// MaxCapabilities is the number of propulsion capability points one design
	// may carry.
	MaxCapabilities int `json:"maxCapabilities"`
	// MaxThrustTargets is the number of thrust-to-weight targets.
	MaxThrustTargets int `json:"maxThrustTargets"`
	// MaxAuxiliaryLoads is the number of auxiliary electrical loads.
	MaxAuxiliaryLoads int `json:"maxAuxiliaryLoads"`
	// MaxMissionSegments is the number of legs one mission may carry.
	MaxMissionSegments int `json:"maxMissionSegments"`
	MaxRequestBytes    int `json:"maxRequestBytes"`
}

// EvaluateRequest asks for one design to be assessed under one identity.
type EvaluateRequest struct {
	Request Request `json:"request"`
	Design  Design  `json:"design"`
}

// BatchEvaluateRequest asks for several designs to be assessed in one call.
// Each entry carries its own identity: a batch is a convenience, not a way to
// let one identity cover several candidates.
type BatchEvaluateRequest struct {
	Evaluations []EvaluateRequest `json:"evaluations"`
}

// BatchEvaluateResponse holds one evaluation per request, in request order.
type BatchEvaluateResponse struct {
	Evaluations []Evaluation `json:"evaluations"`
}

// ApplyRequest applies commands to a design and evaluates the result. The
// service holds no history: undo belongs to the client that owns the design.
type ApplyRequest struct {
	Commands []Command `json:"commands"`
	Request  Request   `json:"request"`
	Design   Design    `json:"design"`
}

// ApplyResponse returns the edited design and its evaluation. A command that
// fails leaves the whole call failed and nothing applied.
type ApplyResponse struct {
	Applied    []string   `json:"applied"`
	Design     Design     `json:"design"`
	Evaluation Evaluation `json:"evaluation"`
}

// PreviewRequest asks what a command would do without applying it.
type PreviewRequest struct {
	Command Command `json:"command"`
	Request Request `json:"request"`
	Design  Design  `json:"design"`
}

// PreviewResponse is the before-and-after of an unapplied command. Neither
// evaluation describes a design the client currently holds.
type PreviewResponse struct {
	Command string     `json:"command"`
	Changes []Change   `json:"changes"`
	Before  Evaluation `json:"before"`
	After   Evaluation `json:"after"`
}

// SweepOutput names the quantity a sweep plots. It reuses the requirement
// subjects rather than inventing a second vocabulary of plottable values: a
// curve is only worth drawing next to the bounds it is judged against.
type SweepOutput struct {
	// Subject names the plotted quantity, using the requirement-subject tokens.
	Subject string `json:"subject"`
	// Case names the flight case, for a per-case subject only.
	Case string `json:"case,omitempty"`
}

// SweepSettings is one sensitivity request: which driver moves, between which
// values, at how many samples, and what is read off each candidate.
type SweepSettings struct {
	// Driver names the value to move, using the core's own parameter keys. It
	// must be one the design currently holds: a derived value cannot move
	// without a promotion the builder makes explicitly.
	Driver string      `json:"driver"`
	Output SweepOutput `json:"output"`
	From   Quantity    `json:"from"`
	To     Quantity    `json:"to"`
	// Samples is how many candidates to evaluate, endpoints included.
	Samples int `json:"samples"`
}

// SweepSample is one evaluated candidate. Status and Feasibility are separate
// answers: a candidate whose output could not be computed is a gap in the
// curve, and one that computed and violates a requirement is a point outside
// the feasible region.
type SweepSample struct {
	Value *Quantity `json:"value,omitempty"`
	Trace *Trace    `json:"trace,omitempty"`
	// Status is "computed", "missing", "invalid" or "stale".
	Status string `json:"status"`
	// Feasibility is "met", "unmet" or "unknown".
	Feasibility string   `json:"feasibility"`
	Detail      string   `json:"detail,omitempty"`
	Driver      Quantity `json:"driver"`
	// HasRequired reports whether any required check existed. When it is false,
	// Feasibility makes no claim.
	HasRequired bool `json:"hasRequired"`
}

// SweepBound is a requirement boundary drawn across the plotted output.
type SweepBound struct {
	Name      string   `json:"name"`
	Direction string   `json:"direction"`
	Priority  string   `json:"priority"`
	Value     Quantity `json:"value"`
}

// SweepRequest asks for one sensitivity sweep of one design.
type SweepRequest struct {
	Request  Request       `json:"request"`
	Settings SweepSettings `json:"settings"`
	Design   Design        `json:"design"`
}

// SweepResponse is one complete sweep, tied to the request identity, the input
// snapshot and the settings it answers.
//
// It commits nothing. Sampling a range asks about candidates the design does
// not hold, and selecting one of them is a separate, explicit edit.
type SweepResponse struct {
	// SettingsFingerprint is the canonical form of Settings, so a client can
	// match an answer against the question without comparing fields.
	SettingsFingerprint string `json:"settingsFingerprint"`
	// Snapshot is the fingerprint of the design the sweep was computed from.
	Snapshot string `json:"snapshot"`
	// SolveMode names the driver pair the planform solves from. A plot without
	// it is ambiguous: at fixed span a higher aspect ratio is a smaller area,
	// and at fixed area it is a longer span.
	SolveMode string `json:"solveMode"`
	// Detail states what moved and what stayed fixed, in words.
	Detail string `json:"detail"`
	// HeldFixed names the parameters that did not move.
	HeldFixed []string `json:"heldFixed"`
	// AlsoChanged names the derived parameters whose values differ across the
	// range. It is what answers "this driver has no effect" when the plotted
	// output happens not to move.
	AlsoChanged []string      `json:"alsoChanged"`
	Bounds      []SweepBound  `json:"bounds"`
	Samples     []SweepSample `json:"samples"`
	Request     Request       `json:"request"`
	Settings    SweepSettings `json:"settings"`
	// Current is the design's own candidate, evaluated at the driver value it
	// actually holds. It is the marker on the curve and is not one of Samples.
	Current SweepSample `json:"current"`
	// Invariant reports that every computed sample produced the same output. It
	// is a fact about this output at this solve mode, not about the driver.
	Invariant bool `json:"invariant"`
}

// PowerSearchSettings is one bounded search for the candidates whose electrical
// demand stays inside a power ceiling.
//
// It exists because an all-up mass and a power maximum together do not
// determine a wing. The demand is not monotonic in wing area at a fixed speed,
// so the feasible set is generally an interval and can be several.
type PowerSearchSettings struct {
	// Driver names the value to move, using the core's own parameter keys. It
	// must be one the design currently holds.
	Driver string   `json:"driver"`
	From   Quantity `json:"from"`
	To     Quantity `json:"to"`
	// Ceiling is the largest electrical power any single segment may hold.
	Ceiling Quantity `json:"ceiling"`
	// Samples is how many candidates to evaluate, endpoints included.
	Samples int `json:"samples"`
}

// PowerSearchCandidate is one evaluated candidate.
type PowerSearchCandidate struct {
	Demand *Quantity `json:"demand,omitempty"`
	// Status is "computed", "missing", "invalid" or "stale".
	Status string `json:"status"`
	// Feasibility is the aggregate of this candidate's own required checks,
	// reported separately from the ceiling.
	Feasibility string   `json:"feasibility"`
	Detail      string   `json:"detail,omitempty"`
	Driver      Quantity `json:"driver"`
	Margin      float64  `json:"margin"`
	HasRequired bool     `json:"hasRequired"`
	// WithinCeiling reports whether the demand fits the ceiling.
	WithinCeiling bool `json:"withinCeiling"`
	// Feasible is WithinCeiling together with the required checks.
	Feasible bool `json:"feasible"`
}

// PowerSearchInterval is one run of consecutive feasible candidates, with the
// evaluated candidates that bracket its ends. The ends are brackets rather than
// boundaries: no iterate converges anywhere, so naming a single crossing value
// would present a number nothing computed.
type PowerSearchInterval struct {
	BelowFirst *Quantity `json:"belowFirst,omitempty"`
	AboveLast  *Quantity `json:"aboveLast,omitempty"`
	Detail     string    `json:"detail"`
	First      Quantity  `json:"first"`
	Last       Quantity  `json:"last"`
	// OpenLow and OpenHigh report that the run reaches a searched bound, beyond
	// which this search says nothing.
	OpenLow  bool `json:"openLow"`
	OpenHigh bool `json:"openHigh"`
}

// PowerSearchRequest asks for one bounded power search of one design.
type PowerSearchRequest struct {
	Request  Request             `json:"request"`
	Settings PowerSearchSettings `json:"settings"`
	Design   Design              `json:"design"`
}

// PowerSearchResponse is one complete bounded search. It commits nothing and
// selects nothing: choosing a candidate from an interval is a separate edit.
type PowerSearchResponse struct {
	// SettingsFingerprint is the canonical form of Settings.
	SettingsFingerprint string `json:"settingsFingerprint"`
	// Snapshot is the fingerprint of the design searched.
	Snapshot string `json:"snapshot"`
	// SolveMode names the driver pair the planform solves from, without which
	// the result is ambiguous.
	SolveMode string `json:"solveMode"`
	Detail    string `json:"detail"`
	// HeldFixed names the parameters that did not move.
	HeldFixed  []string               `json:"heldFixed"`
	Candidates []PowerSearchCandidate `json:"candidates"`
	Intervals  []PowerSearchInterval  `json:"intervals"`
	Request    Request                `json:"request"`
	Settings   PowerSearchSettings    `json:"settings"`
	// Unique reports that exactly one candidate in the range was feasible. Even
	// then it is a sampled candidate rather than a solved answer.
	Unique bool `json:"unique"`
	// Found reports whether any candidate was feasible at all.
	Found bool `json:"found"`
}
