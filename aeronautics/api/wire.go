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
	Name string `json:"name"`
	// Shape is "rectangle" or "trapezoid".
	Shape string `json:"shape"`
	// Span, Area and RootChord are size drivers. Exactly two of these three and
	// AspectRatio may be supplied.
	Span      *Quantity `json:"span,omitempty"`
	Area      *Quantity `json:"area,omitempty"`
	RootChord *Quantity `json:"rootChord,omitempty"`
	// AspectRatio is a size driver; zero means it is not one.
	AspectRatio float64 `json:"aspectRatio"`
	// TaperRatio is lambda = c_tip/c_root, required for a trapezoid.
	TaperRatio float64 `json:"taperRatio"`
	// Sweep, Dihedral, Twist and Incidence must be stated, with zero spelled out.
	Sweep     *Quantity `json:"sweep,omitempty"`
	Dihedral  *Quantity `json:"dihedral,omitempty"`
	Twist     *Quantity `json:"twist,omitempty"`
	Incidence *Quantity `json:"incidence,omitempty"`
	// BodyWidth is optional; without it no exposed area is reported.
	BodyWidth *Quantity `json:"bodyWidth,omitempty"`
	// SweepReference is the chord fraction Sweep is measured at.
	SweepReference float64 `json:"sweepReference"`
	// AreaBasis states what a supplied area measures.
	AreaBasis string `json:"areaBasis"`
	// DihedralMode states which dimensions stay fixed as dihedral changes. It
	// may be empty only at zero dihedral, where the two planes coincide.
	DihedralMode string   `json:"dihedralMode,omitempty"`
	RootAirfoil  *Airfoil `json:"rootAirfoil,omitempty"`
	TipAirfoil   *Airfoil `json:"tipAirfoil,omitempty"`
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
	Name string `json:"name"`
	// Subject names the bounded quantity.
	Subject string `json:"subject"`
	// Priority is "required" or "preferred".
	Priority string `json:"priority"`
	Basis    string `json:"basis"`
	// Cases names the flight cases a per-case requirement applies to.
	Cases   []string  `json:"cases,omitempty"`
	Minimum *Quantity `json:"minimum,omitempty"`
	Maximum *Quantity `json:"maximum,omitempty"`
	// Margin is a relative safety margin that tightens both bounds.
	Margin float64 `json:"margin"`
}

// Design is the authoritative parametric definition on the wire. It is the
// whole request state: the service holds none of it between calls.
type Design struct {
	Name          string        `json:"name"`
	Configuration string        `json:"configuration"`
	MassBasis     string        `json:"massBasis"`
	Mass          *Quantity     `json:"mass,omitempty"`
	Wing          Wing          `json:"wing"`
	Tail          *Tail         `json:"tail,omitempty"`
	Cases         []Case        `json:"cases,omitempty"`
	Requirements  []Requirement `json:"requirements,omitempty"`
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
	// Kind selects the edit.
	Kind string `json:"kind"`

	Basis    string `json:"basis,omitempty"`
	Key      string `json:"key,omitempty"`
	Promote  string `json:"promote,omitempty"`
	Release  string `json:"release,omitempty"`
	Name     string `json:"name,omitempty"`
	Priority string `json:"priority,omitempty"`
	Hold     string `json:"hold,omitempty"`

	Mass  *Quantity `json:"mass,omitempty"`
	Value *Quantity `json:"value,omitempty"`

	Requirement *Requirement `json:"requirement,omitempty"`
	Case        *Case        `json:"case,omitempty"`
	CLmax       *CLmax       `json:"clmax,omitempty"`
	Scope       *Scope       `json:"scope,omitempty"`

	// Ratio is the taper ratio, for the taper-ratio edit.
	Ratio float64 `json:"ratio,omitempty"`
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
	Bound    Quantity  `json:"bound"`
	Actual   *Quantity `json:"actual,omitempty"`
	Trace    *Trace    `json:"trace,omitempty"`
	// Margin is the achieved relative room inside the bound.
	Margin float64 `json:"margin"`
}

// Contribution is one case's or requirement's contribution to an intersected
// bound.
type Contribution struct {
	Source string    `json:"source"`
	Detail string    `json:"detail,omitempty"`
	Value  *Quantity `json:"value,omitempty"`
	Trace  *Trace    `json:"trace,omitempty"`
	// Known reports whether the source contributed a value.
	Known bool `json:"known"`
}

// Bound is an intersected sizing bound: the largest lower bound or the smallest
// upper bound over the applicable sources.
type Bound struct {
	Subject   string `json:"subject"`
	Direction string `json:"direction"`
	Detail    string `json:"detail,omitempty"`
	// Controlling names the sources that set Value. More than one means a tie.
	Controlling   []string       `json:"controlling,omitempty"`
	Contributions []Contribution `json:"contributions,omitempty"`
	Value         *Quantity      `json:"value,omitempty"`
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

// Evaluation is one complete assessment, tied to the request identity it
// answers and the input snapshot it was computed from.
type Evaluation struct {
	Request  Request `json:"request"`
	Snapshot string  `json:"snapshot"`
	// Geometry is "computed", "missing", "invalid" or "stale".
	Geometry string `json:"geometry"`
	// Aggregate is the combined status of the required checks. It makes no
	// feasibility claim when HasRequired is false: an empty required set is not
	// a passing one.
	Aggregate           string      `json:"aggregate"`
	Wing                *SolvedWing `json:"wing,omitempty"`
	Checks              []Check     `json:"checks"`
	AreaLower           Bound       `json:"areaLower"`
	AreaUpper           Bound       `json:"areaUpper"`
	Mass                MassRange   `json:"mass"`
	Conflicts           []Conflict  `json:"conflicts"`
	Patterns            []string    `json:"patterns"`
	DefinitionIssues    []Issue     `json:"definitionIssues"`
	GeometryIssues      []Issue     `json:"geometryIssues"`
	ConfigurationIssues []Issue     `json:"configurationIssues"`
	HasRequired         bool        `json:"hasRequired"`
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
	ID          string   `json:"id"`
	Revision    string   `json:"revision"`
	Expression  string   `json:"expression"`
	Inputs      []Port   `json:"inputs"`
	Output      Port     `json:"output"`
	Assumptions []string `json:"assumptions"`
	Source      Source   `json:"source"`
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
	MaxRequestBytes int `json:"maxRequestBytes"`
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
	Request  Request   `json:"request"`
	Design   Design    `json:"design"`
	Commands []Command `json:"commands"`
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
	Request Request `json:"request"`
	Design  Design  `json:"design"`
	Command Command `json:"command"`
}

// PreviewResponse is the before-and-after of an unapplied command. Neither
// evaluation describes a design the client currently holds.
type PreviewResponse struct {
	Command string     `json:"command"`
	Before  Evaluation `json:"before"`
	After   Evaluation `json:"after"`
	Changes []Change   `json:"changes"`
}
