package calculator

import "strconv"

// RequestID identifies one evaluation request. Identities are minted by a
// Session, are unique for its lifetime and are never reused: undo, redo and
// loading a draft all restore a design, and none of them restores an identity.
// That is what lets a caller reject a result that arrives late for a branch the
// builder has already left.
//
// The sequence is a counter rather than a clock or a random value on purpose.
// The core reaches neither, and a counter makes the ordering of two identities
// checkable rather than merely probable.
type RequestID struct {
	// Session names the session that minted the identity.
	Session string
	// Sequence is the strictly increasing request number within that session.
	Sequence uint64
}

// String renders the identity as "session#sequence".
func (r RequestID) String() string {
	return r.Session + "#" + strconv.FormatUint(r.Sequence, 10)
}

// IsZero reports whether the identity was never minted.
func (r RequestID) IsZero() bool { return r == RequestID{} }

// Evaluation is one complete assessment of a design, tied to the request
// identity and the input snapshot it was computed from. Nothing in it is
// authoritative: it is a view of the Design that produced it, and re-evaluating
// the same Design reproduces it apart from the identity.
type Evaluation struct {
	// DefinitionIssues reports structural problems in the definition itself.
	DefinitionIssues error
	// GeometryIssues reports why the wing did not solve, when it did not.
	GeometryIssues error
	// ConfigurationIssues reports an incomplete or mismatched tail description.
	// It never blocks the lift and area results: an unfinished tail panel is not
	// a reason to withhold a valid wing sizing.
	ConfigurationIssues error
	// Snapshot is the fingerprint of the inputs it was computed from.
	Snapshot string
	// Checks are the requirement outcomes, one per bound side and case.
	Checks RequirementChecks
	// Conflicts are the known conflicting groups among the required constraints.
	Conflicts []Conflict
	// Request is the identity this evaluation answers.
	Request RequestID
	// AreaLower is the intersected lower bound on wing area over the required
	// cases.
	AreaLower SizingBound
	// AreaUpper is the intersected required upper bound on wing area.
	AreaUpper SizingBound
	// Mass is the feasible all-up mass interval over the required cases.
	Mass MassInterval
	// Loads are the lumped lift each case demands: a magnitude with no line of
	// action, from Task 02's model.
	Loads []CaseLoad
	// MassProperties is the mechanical balance of the listed components. It is
	// reported in either mass mode: a builder who entered an all-up mass may
	// still place components and read where their combined centre of gravity
	// sits, and the two answers stay separately labelled.
	MassProperties MassProperties
	// Electrical is the auxiliary electrical demand: the avionics, servos,
	// sensors and payload, summed on the pack side of every regulator.
	Electrical ElectricalBudget
	// Mission is the energy budget: what each segment costs and whether the
	// pack can pay for it after its reserve.
	Mission MissionResult
	// PowerFeasibility compares the electrical demand with the component
	// ratings. It is a separate answer from the energy budget: a mission whose
	// energy fits can still exceed what the pack and the controller may deliver.
	PowerFeasibility PowerFeasibility
	// ThrustChecks are the thrust-to-weight targets against the capability
	// points they are stated at.
	ThrustChecks []ThrustCheck
	// Design is the definition evaluated.
	Design Design
	// Wing is the solved geometry. It is the zero Wing when Geometry is not
	// ResultComputed.
	Wing Wing
	// Aggregate is the combined status of the required checks.
	Aggregate LimitStatus
	// HasRequired reports whether any required check existed. When it is false,
	// Aggregate makes no feasibility claim: an empty required set is not a
	// passing one.
	HasRequired bool
	// Geometry reports whether the wing solved.
	Geometry ResultStatus
}

// Stale returns a copy of the evaluation marked as no longer describing the
// current design. Every check becomes unknown and every result stale, because
// showing a previously met requirement against edited inputs would present a
// stale output as a current one.
func (e Evaluation) Stale() Evaluation {
	out := e
	out.Checks = make(RequirementChecks, len(e.Checks))
	copy(out.Checks, e.Checks)
	for n := range out.Checks {
		out.Checks[n].Result = ResultStale
		out.Checks[n].Status = LimitUnknown
		out.Checks[n].Margin = 0
		out.Checks[n].Detail = staleDetail
	}
	out.Geometry = ResultStale
	out.Loads = make([]CaseLoad, len(e.Loads))
	copy(out.Loads, e.Loads)
	for n := range out.Loads {
		out.Loads[n].Status = ResultStale
		out.Loads[n].Detail = staleDetail
	}
	out.MassProperties.Status = ResultStale
	out.MassProperties.Detail = staleDetail
	out.Electrical.Status = ResultStale
	out.Electrical.Detail = staleDetail
	out.Mission = e.Mission.stale()
	out.PowerFeasibility.Status = ResultStale
	out.PowerFeasibility.Detail = staleDetail
	out.ThrustChecks = make([]ThrustCheck, len(e.ThrustChecks))
	copy(out.ThrustChecks, e.ThrustChecks)
	for n := range out.ThrustChecks {
		out.ThrustChecks[n].Status = LimitUnknown
		out.ThrustChecks[n].Margin = 0
		out.ThrustChecks[n].Detail = staleDetail
	}
	out.Aggregate, out.HasRequired = out.Checks.Aggregate()
	return out
}

// staleDetail is what every stale result says, so a caller matching on it does
// not have to know which subsystem produced it.
const staleDetail = "computed from an earlier revision of the design and not recomputed"

// stale returns a copy of the mission result marked as no longer describing the
// current design. Every segment goes stale with it: a segment energy shown
// against edited inputs would be a stale output presented as a current one.
func (m MissionResult) stale() MissionResult {
	out := m
	out.Status = ResultStale
	out.Detail = staleDetail
	out.EnergyStatus = LimitUnknown
	out.Segments = make([]SegmentResult, len(m.Segments))
	copy(out.Segments, m.Segments)
	for n := range out.Segments {
		out.Segments[n].Status = ResultStale
		out.Segments[n].Detail = staleDetail
		out.Segments[n].Power.Status = ResultStale
		out.Segments[n].Power.Detail = staleDetail
		out.Segments[n].Availability.Status = LimitUnknown
		out.Segments[n].Availability.Margin = 0
		out.Segments[n].Availability.Detail = staleDetail
	}
	return out
}

// Evaluate assesses a design without a session, for a caller reconstructing a
// definition through direct Go calls. It mints no identity: identity belongs to
// a session's request stream, and a bare evaluation has none to belong to.
func (d Design) Evaluate() Evaluation {
	e := Evaluation{
		Design:              d.clone(),
		Snapshot:            d.Snapshot(),
		DefinitionIssues:    d.Validate(),
		ConfigurationIssues: d.Airframe().ValidateGeometry(),
	}
	solved := d.solveOnce()
	if solved.err != nil {
		e.Geometry = solved.status
		e.GeometryIssues = solved.err
	} else {
		e.Geometry = ResultComputed
		e.Wing = solved.wing
	}
	e.Checks = d.Assess()
	e.Aggregate, e.HasRequired = e.Checks.Aggregate()
	e.AreaLower, _ = d.AreaLowerBound(AllRequiredCases())
	e.AreaUpper = d.AreaUpperBound()
	e.Mass, _ = d.MassInterval(AllRequiredCases())
	e.MassProperties = d.MassProperties()
	e.Loads = d.CaseLoads()
	e.Electrical = d.ElectricalBudget()
	e.Mission = d.MissionAnalysis()
	e.PowerFeasibility = d.PowerFeasibility(e.Mission, e.Electrical)
	e.ThrustChecks = d.ThrustChecks()
	e.Conflicts = d.Conflicts()
	return e
}

// Session holds a design, its undoable history and its evaluation request
// stream. It is the state a worksheet edits.
//
// A Session is not safe for concurrent use. Request ordering here is a counter
// guarded by the caller rather than an atomic, because the core deliberately
// does not import sync/atomic or context; a transport that serves several
// callers owns its own serialization and lives outside this package.
type Session struct {
	name string
	// current is the outstanding evaluation identity, and currentSweep the
	// outstanding sweep identity. They are separate streams of acceptance over
	// one stream of identities: an answer to one is never an answer to the
	// other, and a design change retires both.
	current      RequestID
	currentSweep RequestID
	history      []Design
	cursor       int
	next         uint64
}

// NewSession starts a session over an initial design. The design is copied, so
// later edits to the caller's value do not reach the session.
func NewSession(name string, initial Design) *Session {
	return &Session{name: name, history: []Design{initial.clone()}}
}

// Name returns the session's name, which every request identity carries.
func (s *Session) Name() string { return s.name }

// Design returns a copy of the current design.
func (s *Session) Design() Design { return s.history[s.cursor].clone() }

// Snapshot returns the fingerprint of the current design's inputs.
func (s *Session) Snapshot() string { return s.history[s.cursor].Snapshot() }

// CanUndo reports whether an earlier revision exists.
func (s *Session) CanUndo() bool { return s.cursor > 0 }

// CanRedo reports whether a later revision exists on the current branch.
func (s *Session) CanRedo() bool { return s.cursor < len(s.history)-1 }

// Revisions returns how many revisions the history holds, current branch
// included.
func (s *Session) Revisions() int { return len(s.history) }

// Do applies a command and records the result as a new revision. A command that
// fails changes nothing: the design, the history and the request stream are all
// left as they were, so a rejected edit cannot half-apply.
func (s *Session) Do(cmd Command) error {
	if cmd == nil {
		return commandIssue("command", IssueMissing, "no command was supplied")
	}
	next, err := cmd.apply(s.history[s.cursor])
	if err != nil {
		return err
	}
	s.history = append(s.history[:s.cursor+1], next)
	s.cursor++
	s.invalidateRequest()
	return nil
}

// Undo moves to the previous revision, restoring the complete former design,
// driver roles included: the drivers live in the design, so restoring one
// restores the other by construction.
func (s *Session) Undo() error {
	if !s.CanUndo() {
		return commandIssue("history", IssueMissing, "there is no earlier revision to undo to")
	}
	s.cursor--
	s.invalidateRequest()
	return nil
}

// Redo moves forward again along the current branch.
func (s *Session) Redo() error {
	if !s.CanRedo() {
		return commandIssue("history", IssueMissing, "there is no later revision to redo to")
	}
	s.cursor++
	s.invalidateRequest()
	return nil
}

// Load replaces the design with a loaded draft, recording it as a new revision
// so the previous state stays undoable. Loading restores a design and nothing
// else: the request stream keeps counting, so a result still in flight for the
// pre-load design can never be accepted for the loaded one.
//
// A draft is adopted as given. An incomplete draft stays loadable and saveable,
// and its problems are reported by the evaluation rather than by refusing it;
// versioning and compatibility checks of stored drafts arrive with Task 06.
func (s *Session) Load(loaded Design) {
	s.history = append(s.history[:s.cursor+1], loaded.clone())
	s.cursor++
	s.invalidateRequest()
}

// invalidateRequest drops the current request identity. Every design change,
// including one that restores an earlier revision, makes any outstanding result
// unacceptable until a new evaluation is requested.
func (s *Session) invalidateRequest() {
	s.current = RequestID{}
	s.currentSweep = RequestID{}
}

// Evaluate mints a fresh request identity and evaluates the current design.
// Calling it twice without an edit produces two distinct identities, and only
// the later one is current: an evaluation is a request, not a cache key.
func (s *Session) Evaluate() Evaluation {
	s.next++
	s.current = RequestID{Session: s.name, Sequence: s.next}
	e := s.history[s.cursor].Evaluate()
	e.Request = s.current
	return e
}

// Freshness reports whether a held evaluation still describes the current
// design and the current request. Anything else is stale.
func (s *Session) Freshness(e Evaluation) ResultStatus {
	if s.current.IsZero() || e.Request != s.current || e.Snapshot != s.Snapshot() {
		return ResultStale
	}
	return ResultComputed
}

// Accept records a result against the session. It refuses anything that is not
// the answer to the current request over the current inputs, which is what
// stops a result computed before an undo, a redo or a draft load from being
// shown as the current one.
func (s *Session) Accept(e Evaluation) error {
	if s.current.IsZero() {
		return commandIssue("request", IssueInvalid,
			"the design changed since the last evaluation was requested; evaluate again before "+
				"accepting a result")
	}
	if e.Request != s.current {
		return commandIssue("request", IssueInvalid,
			"result answers request "+e.Request.String()+" but the current request is "+
				s.current.String())
	}
	if e.Snapshot != s.Snapshot() {
		return commandIssue("request", IssueInvalid,
			"result was computed from a different input snapshot than the design now holds")
	}
	return nil
}

// Preview is what a command would do, without doing it. Both evaluations carry
// their own fresh identities and neither is acceptable as a current result: a
// preview describes a design the session does not hold.
type Preview struct {
	// Command is the command's label.
	Command string
	// Changes lists the requirement outcomes the command would alter.
	Changes []RequirementChange
	// Before is the current design's evaluation.
	Before Evaluation
	// After is the evaluation of the design the command would produce.
	After Evaluation
}

// RequirementChange is one requirement outcome a command would alter.
type RequirementChange struct {
	// Name is the requirement's name.
	Name string
	// Case names the flight case, for a per-case subject.
	Case string
	// Detail describes the change in the terms a worksheet can show.
	Detail string
	// Subject names the bounded quantity.
	Subject RequirementSubject
	// Direction says which bound side changed.
	Direction BoundDirection
	// From is the status before the command, To the status after.
	From LimitStatus
	// To is the status after the command.
	To LimitStatus
	// Appeared reports a check the command creates, Vanished one it removes.
	Appeared bool
	// Vanished reports a check the command removes.
	Vanished bool
}

// Preview evaluates what a command would produce without applying it, and
// reports which requirement outcomes it would change. Nothing is committed and
// the session's history is untouched.
func (s *Session) Preview(cmd Command) (Preview, error) {
	if cmd == nil {
		return Preview{}, commandIssue("command", IssueMissing, "no command was supplied")
	}
	after, err := cmd.apply(s.history[s.cursor])
	if err != nil {
		return Preview{}, err
	}
	// A preview is not an answer to the session's current request, and it is not
	// an edit either: it mints identities from the same stream so they stay
	// unique, and leaves the current request alone. Accept refuses both of these
	// because neither is the current identity, which is the point — the session
	// does not hold the design the preview describes.
	s.next++
	before := s.history[s.cursor].Evaluate()
	before.Request = RequestID{Session: s.name, Sequence: s.next}
	s.next++
	afterEval := after.Evaluate()
	afterEval.Request = RequestID{Session: s.name, Sequence: s.next}
	return Preview{
		Command: cmd.Label(),
		Before:  before,
		After:   afterEval,
		Changes: diffChecks(before.Checks, afterEval.Checks),
	}, nil
}

// checkKey identifies a check across two evaluations, so a diff pairs the same
// bound in the same case rather than matching by position.
type checkKey struct {
	name      string
	caseName  string
	direction BoundDirection
}

func keyOf(c *RequirementCheck) checkKey {
	return checkKey{name: c.Name, caseName: c.Case, direction: c.Direction}
}

// diffChecks reports the outcomes that differ between two check sets, in the
// order of the later set, then the removals in the order of the earlier one.
func diffChecks(before, after RequirementChecks) []RequirementChange {
	previous := make(map[checkKey]LimitStatus, len(before))
	for n := range before {
		previous[keyOf(&before[n])] = before[n].Status
	}
	seen := make(map[checkKey]bool, len(after))
	changes := make([]RequirementChange, 0, len(after))
	for n := range after {
		c := &after[n]
		key := keyOf(c)
		seen[key] = true
		was, existed := previous[key]
		if existed && was == c.Status {
			continue
		}
		change := RequirementChange{
			Name: c.Name, Case: c.Case, Subject: c.Subject, Direction: c.Direction,
			From: was, To: c.Status, Appeared: !existed,
		}
		change.Detail = describeChange(change, c.Bound)
		changes = append(changes, change)
	}
	for n := range before {
		c := &before[n]
		if seen[keyOf(c)] {
			continue
		}
		changes = append(changes, RequirementChange{
			Name: c.Name, Case: c.Case, Subject: c.Subject, Direction: c.Direction,
			From: c.Status, To: LimitUnknown, Vanished: true,
			Detail: "the " + c.Direction.String() + " on " + c.Subject.String() +
				" no longer applies after this edit",
		})
	}
	return changes
}

func describeChange(change RequirementChange, bound Quantity) string {
	prefix := "the " + change.Direction.String() + " on " + change.Subject.String()
	if change.Case != "" {
		prefix += " in case " + change.Case
	}
	if change.Appeared {
		return prefix + " becomes " + change.To.String() + " (" + bound.String() + ")"
	}
	return prefix + " goes from " + change.From.String() + " to " + change.To.String()
}
