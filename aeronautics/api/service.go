package api

import (
	"context"
	"errors"
	"sort"
	"strconv"

	"yalb.aero/calculator"
)

// Payload limits. They live here rather than in the HTTP layer so that every
// adapter — HTTP today, MCP later — is bounded the same way, and they are
// published in the discovery document so a client can respect them instead of
// finding them by being refused.
const (
	// MaxCases is the number of flight cases one design may carry.
	MaxCases = 32
	// MaxRequirements is the number of requirements one design may carry.
	MaxRequirements = 64
	// MaxCommands is the number of commands one apply call may carry.
	MaxCommands = 32
	// MaxBatch is the number of designs one batch evaluation may carry.
	MaxBatch = 16
)

// FailureKind classifies why a call did not produce a result. It is a
// transport-neutral vocabulary: this package does not know what an HTTP status
// code is, and the mapping onto one lives in the transport that has them.
type FailureKind uint8

const (
	// FailureMalformed means the payload could not be understood at all.
	FailureMalformed FailureKind = iota
	// FailureNotFound means a named equation or pattern is not registered.
	FailureNotFound
	// FailureInvalid means the request was understood and its contents are
	// wrong: a missing field, an impossible value, a bad unit symbol.
	FailureInvalid
	// FailureUnsupported means the request is well formed but outside what the
	// implemented models may claim.
	FailureUnsupported
	// FailureTooLarge means a payload exceeded a published limit.
	FailureTooLarge
	// FailureCancelled means the caller went away before the work finished.
	FailureCancelled
)

var failureKindNames = [...]string{
	FailureMalformed:   "malformed",
	FailureNotFound:    "not-found",
	FailureInvalid:     "invalid",
	FailureUnsupported: "unsupported",
	FailureTooLarge:    "too-large",
	FailureCancelled:   "cancelled",
}

// String returns the failure kind's wire token.
func (k FailureKind) String() string {
	if int(k) < len(failureKindNames) {
		return failureKindNames[k]
	}
	return "invalid"
}

// Failure is a refused call, with the field issues behind it where there are
// any. It is the only error type this package returns.
type Failure struct {
	// Message states the problem in one line.
	Message string
	// Issues are the field-specific reasons, in the order they were found.
	Issues []Issue
	// Kind classifies the failure for a transport to map onto its own status
	// vocabulary.
	Kind FailureKind
}

// Error renders the failure and its issue count.
func (f *Failure) Error() string {
	if len(f.Issues) == 0 {
		return f.Kind.String() + ": " + f.Message
	}
	return f.Kind.String() + ": " + f.Message + " (" + strconv.Itoa(len(f.Issues)) + " field issues)"
}

// AsFailure extracts the Failure carried by err, if any.
func AsFailure(err error) (*Failure, bool) {
	var failure *Failure
	if errors.As(err, &failure) {
		return failure, true
	}
	return nil, false
}

func fail(kind FailureKind, message string, issues ...Issue) *Failure {
	return &Failure{Kind: kind, Message: message, Issues: issues}
}

// worstKind picks the failure kind a set of decoding issues deserves. An
// unsupported input is a different answer from an invalid one, and collapsing
// both into "invalid" would tell a client its request was wrong when what it
// was told is that the model does not cover it.
func worstKind(issues []Issue) FailureKind {
	kind := FailureInvalid
	unsupportedOnly := true
	for _, issue := range issues {
		if issue.Kind != "unsupported" {
			unsupportedOnly = false
		}
	}
	if unsupportedOnly && len(issues) > 0 {
		kind = FailureUnsupported
	}
	return kind
}

// Service answers boundary calls against the calculator. It holds no state: a
// request carries the whole design, and the design history, undo and the
// evaluation request stream belong to the client that owns them.
//
// That is deliberate. Session state on the server would make two clients of the
// same design fight over one history, and it would put a second authoritative
// copy of the definition somewhere the builder cannot see.
type Service struct{}

// NewService returns the boundary service.
func NewService() *Service { return &Service{} }

// Discover returns everything a client needs to build a valid request: the
// contract version, every equation with its provenance, the curated workflows,
// the supported units and the enum vocabularies.
func (s *Service) Discover() Discovery {
	return Discovery{
		ContractVersion: ContractVersion,
		Version:         calculator.Version,
		Equations:       s.Equations(),
		Patterns:        s.Patterns(),
		Units:           s.Units(),
		Vocabularies:    Vocabularies(),
		Limits: Limits{
			MaxCases:        MaxCases,
			MaxRequirements: MaxRequirements,
			MaxCommands:     MaxCommands,
			MaxBatch:        MaxBatch,
		},
	}
}

// Equations returns every implemented equation, sorted by ID.
func (s *Service) Equations() []Equation {
	ids := calculator.EquationIDs()
	out := make([]Equation, 0, len(ids))
	for _, id := range ids {
		eq, err := calculator.Lookup(id)
		if err != nil {
			continue
		}
		out = append(out, encodeEquation(eq))
	}
	return out
}

// Equation returns one equation's published definition.
func (s *Service) Equation(id string) (Equation, error) {
	eq, err := calculator.Lookup(id)
	if err != nil {
		return Equation{}, fail(FailureNotFound, "no equation is registered under "+strconv.Quote(id))
	}
	return encodeEquation(eq), nil
}

// Patterns returns the curated workflows, supported and unsupported alike.
func (s *Service) Patterns() []Pattern {
	all := calculator.Patterns()
	out := make([]Pattern, 0, len(all))
	for n := range all {
		out = append(out, encodePattern(all[n]))
	}
	return out
}

// Units returns the supported unit symbols and the dimensions they belong to.
func (s *Service) Units() []UnitInfo {
	units := calculator.Units()
	out := make([]UnitInfo, 0, len(units))
	for _, u := range units {
		dimension := u.Dimension()
		out = append(out, UnitInfo{
			Symbol:     u.Symbol(),
			Dimension:  dimensions.format(dimension),
			FactorToSI: u.FactorToSI(),
			SI:         dimension.SIUnit() == u,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Dimension != out[j].Dimension {
			return out[i].Dimension < out[j].Dimension
		}
		return out[i].Symbol < out[j].Symbol
	})
	return out
}

// Evaluate assesses one design under one identity. A design that does not solve
// is not a failure: it comes back with its issues and whatever bounds could
// still be established, which is what lets an incomplete panel coexist with a
// valid wing sizing.
func (s *Service) Evaluate(ctx context.Context, req EvaluateRequest) (Evaluation, error) {
	if err := checkContext(ctx); err != nil {
		return Evaluation{}, err
	}
	d := &decoder{}
	identity := d.request(req.Request)
	design := d.design(req.Design)
	if len(d.issues) > 0 {
		return Evaluation{}, fail(worstKind(d.issues), "the request could not be read", d.issues...)
	}
	return encodeEvaluation(design.Evaluate(), identity), nil
}

// EvaluateBatch assesses several designs in one call, each under its own
// identity. A batch is a convenience and never a way to let one identity cover
// several candidates.
func (s *Service) EvaluateBatch(ctx context.Context, req BatchEvaluateRequest) (BatchEvaluateResponse, error) {
	if len(req.Evaluations) == 0 {
		return BatchEvaluateResponse{}, fail(FailureInvalid, "a batch needs at least one evaluation")
	}
	if len(req.Evaluations) > MaxBatch {
		return BatchEvaluateResponse{}, fail(FailureTooLarge,
			"a batch carries at most "+strconv.Itoa(MaxBatch)+" evaluations, received "+
				strconv.Itoa(len(req.Evaluations)))
	}
	out := BatchEvaluateResponse{Evaluations: make([]Evaluation, 0, len(req.Evaluations))}
	for n := range req.Evaluations {
		// Checked between entries as well as at the start: a caller that goes
		// away part-way through a batch should stop the remaining work.
		if err := checkContext(ctx); err != nil {
			return BatchEvaluateResponse{}, err
		}
		evaluation, err := s.Evaluate(ctx, req.Evaluations[n])
		if err != nil {
			return BatchEvaluateResponse{}, prefixFailure(err, "evaluations["+strconv.Itoa(n)+"]")
		}
		out.Evaluations = append(out.Evaluations, evaluation)
	}
	return out, nil
}

// Apply applies commands to a design in order and evaluates the result. A
// command that fails leaves the whole call failed and nothing applied: the
// client's design is unchanged, which is the same guarantee a Session gives.
func (s *Service) Apply(ctx context.Context, req ApplyRequest) (ApplyResponse, error) {
	if err := checkContext(ctx); err != nil {
		return ApplyResponse{}, err
	}
	if len(req.Commands) == 0 {
		return ApplyResponse{}, fail(FailureInvalid, "supply at least one command to apply")
	}
	if len(req.Commands) > MaxCommands {
		return ApplyResponse{}, fail(FailureTooLarge,
			"an apply call carries at most "+strconv.Itoa(MaxCommands)+" commands, received "+
				strconv.Itoa(len(req.Commands)))
	}
	d := &decoder{}
	identity := d.request(req.Request)
	design := d.design(req.Design)
	commands := make([]calculator.Command, 0, len(req.Commands))
	for n := range req.Commands {
		commands = append(commands, d.command("commands["+strconv.Itoa(n)+"]", req.Commands[n]))
	}
	if len(d.issues) > 0 {
		return ApplyResponse{}, fail(worstKind(d.issues), "the request could not be read", d.issues...)
	}

	session := calculator.NewSession(identity.Session, design)
	applied := make([]string, 0, len(commands))
	for n, command := range commands {
		if err := session.Do(command); err != nil {
			return ApplyResponse{}, fail(coreFailureKind(err),
				"commands["+strconv.Itoa(n)+"] ("+command.Label()+") was refused; nothing was applied",
				encodeIssues(err)...)
		}
		applied = append(applied, command.Label())
	}
	edited := session.Design()
	return ApplyResponse{
		Applied:    applied,
		Design:     encodeDesign(edited),
		Evaluation: encodeEvaluation(edited.Evaluate(), identity),
	}, nil
}

// Preview reports what a command would do without applying it. Neither side of
// the result describes a design the client currently holds, which is why both
// carry the same identity the request did rather than a new one: the client
// decides whether to send the command for real.
func (s *Service) Preview(ctx context.Context, req PreviewRequest) (PreviewResponse, error) {
	if err := checkContext(ctx); err != nil {
		return PreviewResponse{}, err
	}
	d := &decoder{}
	identity := d.request(req.Request)
	design := d.design(req.Design)
	command := d.command("command", req.Command)
	if len(d.issues) > 0 {
		return PreviewResponse{}, fail(worstKind(d.issues), "the request could not be read", d.issues...)
	}
	session := calculator.NewSession(identity.Session, design)
	preview, err := session.Preview(command)
	if err != nil {
		return PreviewResponse{}, fail(coreFailureKind(err),
			"the command was refused, so there is nothing to preview", encodeIssues(err)...)
	}
	return PreviewResponse{
		Command: preview.Command,
		Before:  encodeEvaluation(preview.Before, identity),
		After:   encodeEvaluation(preview.After, identity),
		Changes: encodeChanges(preview.Changes),
	}, nil
}

// coreFailureKind maps the core's own typed issues onto a boundary failure
// kind, so that "this model does not cover your design" does not reach a client
// as "your request was wrong".
func coreFailureKind(err error) FailureKind {
	issues, ok := calculator.AsIssues(err)
	if !ok {
		return FailureInvalid
	}
	if issues.Kind(calculator.IssueInvalid) || issues.Kind(calculator.IssueMissing) {
		return FailureInvalid
	}
	if issues.Kind(calculator.IssueUnsupported) {
		return FailureUnsupported
	}
	return FailureInvalid
}

// checkContext turns a cancelled caller into a boundary failure, so that a
// transport reports it as a cancellation rather than as an empty success.
func checkContext(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fail(FailureCancelled, "the caller went away before the work finished: "+err.Error())
	}
	return nil
}

// prefixFailure re-labels a nested failure with the batch entry it came from,
// keeping its kind and its field issues.
func prefixFailure(err error, prefix string) error {
	failure, ok := AsFailure(err)
	if !ok {
		return err
	}
	issues := make([]Issue, 0, len(failure.Issues))
	for _, issue := range failure.Issues {
		issue.Field = prefix + "." + issue.Field
		issues = append(issues, issue)
	}
	return fail(failure.Kind, prefix+": "+failure.Message, issues...)
}
