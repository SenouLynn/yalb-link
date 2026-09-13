package api_test

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"testing"

	"yalb.aero/api"
	"yalb.aero/calculator"
)

// TestCoreAndBoundaryAgreeOnTheSameCandidate is Task 05's equivalence check: the
// same aircraft, described once as a wire design and once through direct Go
// calls, produces the same physical results, the same provenance and the same
// requirement states.
//
// The input snapshot is compared first. Two definitions with the same
// fingerprint are the same definition, so this is the check that the wire form
// carried the aircraft across rather than something close to it; the numbers
// after it confirm the boundary reported what the core computed.
func TestCoreAndBoundaryAgreeOnTheSameCandidate(t *testing.T) {
	fromCore := coreDesign(t).Evaluate()
	fromAPI, err := api.NewService().Evaluate(t.Context(), evaluateRequest())
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	if fromAPI.Snapshot != fromCore.Snapshot {
		t.Fatalf("the wire design is not the same definition:\n core: %s\n api:  %s",
			fromCore.Snapshot, fromAPI.Snapshot)
	}
	if fromAPI.Geometry != "computed" {
		t.Fatalf("geometry = %q: %+v", fromAPI.Geometry, fromAPI.GeometryIssues)
	}
	if fromAPI.Request != identity() {
		t.Errorf("request = %+v, want the identity the client sent", fromAPI.Request)
	}

	area := parameter(t, fromAPI, "wing.area.reference")
	if area.Value.Unit != "m^2" || area.Value.Value != fromCore.Wing.Projected.Area.SI() {
		t.Errorf("reference area = %+v, want %v m^2", area.Value, fromCore.Wing.Projected.Area.SI())
	}
	wantValue(t, "reference area", &area.Value, fixtureArea, "m^2")

	wantSameChecks(t, fromAPI, fromCore)
	wantSameBounds(t, fromAPI, fromCore)
}

func wantSameChecks(t *testing.T, fromAPI api.Evaluation, fromCore calculator.Evaluation) {
	t.Helper()
	if len(fromAPI.Checks) != len(fromCore.Checks) {
		t.Fatalf("check counts differ: %d and %d", len(fromAPI.Checks), len(fromCore.Checks))
	}
	coreCheck, wireCheck := fromCore.Checks[0], fromAPI.Checks[0]
	if wireCheck.Status != "unmet" || wireCheck.Result != "computed" {
		t.Errorf("check = %+v, want an unmet computed stall ceiling", wireCheck)
	}
	if wireCheck.Actual == nil || wireCheck.Actual.Value != coreCheck.Actual.SI() {
		t.Errorf("stall speed = %+v, want %v", wireCheck.Actual, coreCheck.Actual.SI())
	}
	wantValue(t, "stall speed", wireCheck.Actual, fixtureStall, "m/s")
	if wireCheck.Trace == nil || wireCheck.Trace.EquationID != coreCheck.Trace.EquationID ||
		wireCheck.Trace.Revision != coreCheck.Trace.Revision {
		t.Errorf("trace = %+v, want %s at revision %s",
			wireCheck.Trace, coreCheck.Trace.EquationID, coreCheck.Trace.Revision)
	}
}

func wantSameBounds(t *testing.T, fromAPI api.Evaluation, fromCore calculator.Evaluation) {
	t.Helper()
	if fromAPI.AreaLower.Value == nil || fromAPI.AreaLower.Value.Value != fromCore.AreaLower.Value.SI() {
		t.Errorf("area bound = %+v, want %v", fromAPI.AreaLower.Value, fromCore.AreaLower.Value.SI())
	}
	wantValue(t, "area bound", fromAPI.AreaLower.Value, fixtureMinArea, "m^2")
	if len(fromAPI.AreaLower.Controlling) != 1 || fromAPI.AreaLower.Controlling[0] != fixtureCaseName {
		t.Errorf("controlling cases = %v, want %q", fromAPI.AreaLower.Controlling, fixtureCaseName)
	}
	if fromAPI.Aggregate != "unmet" || !fromAPI.HasRequired {
		t.Errorf("aggregate = %q (has required %v), want unmet", fromAPI.Aggregate, fromAPI.HasRequired)
	}
}

func parameter(t *testing.T, e api.Evaluation, key string) api.Parameter {
	t.Helper()
	if e.Wing == nil {
		t.Fatal("the evaluation carries no solved wing")
	}
	for n := range e.Wing.Parameters {
		if e.Wing.Parameters[n].Key == key {
			return e.Wing.Parameters[n]
		}
	}
	t.Fatalf("no parameter %q in the solved wing", key)
	return api.Parameter{}
}

// TestEveryCoreConstantHasAWireToken holds that the enum tables stay complete.
// A new core constant that reaches the boundary without a token would encode as
// the empty string, which a client would read as an absent field rather than as
// a value it does not know.
func TestEveryCoreConstantHasAWireToken(t *testing.T) {
	// Each entry walks a core enum from its zero value until String() stops
	// naming a real constant, and requires a token for every value except the
	// deliberately unnamed zero.
	for name, tokens := range map[string][]string{
		"configuration":      vocabularyTokens(t, "configuration"),
		"planformShape":      vocabularyTokens(t, "planformShape"),
		"requirementSubject": vocabularyTokens(t, "requirementSubject"),
		"solveMode":          vocabularyTokens(t, "solveMode"),
		"journey":            vocabularyTokens(t, "journey"),
		"dimension":          vocabularyTokens(t, "dimension"),
	} {
		if len(tokens) == 0 {
			t.Errorf("vocabulary %q publishes no tokens", name)
		}
	}

	// Each entry names the enum, where its first real constant sits and what
	// String() returns past its last one.
	//
	// Most of these reserve zero for "not stated", so counting starts at 1.
	// Dimension does not: its zero is Dimensionless, the dimension a ratio or a
	// coefficient is held in, so it needs a token of its own. Nor do IssueKind,
	// LimitStatus, ResultStatus and SourceKind, whose zero values are ordinary
	// members of their sets.
	for _, tc := range []struct {
		name       func(int) string
		vocabulary string
		past       string
		first      int
	}{
		{func(n int) string { return calculator.Configuration(n).String() }, "configuration", "unknown", 1},
		{func(n int) string { return calculator.PlanformShape(n).String() }, "planformShape", "unknown", 1},
		{func(n int) string { return calculator.RequirementSubject(n).String() }, "requirementSubject", "unknown", 1},
		{func(n int) string { return calculator.SolveMode(n).String() }, "solveMode", "unknown", 1},
		{func(n int) string { return calculator.Journey(n).String() }, "journey", "unknown", 1},
		{func(n int) string { return calculator.Priority(n).String() }, "priority", "unknown", 1},
		{func(n int) string { return calculator.EvidenceQuality(n).String() }, "evidence", "unstated", 1},
		{func(n int) string { return calculator.ParameterRole(n).String() }, "parameterRole", "unknown", 1},
		{func(n int) string { return calculator.AreaBasis(n).String() }, "areaBasis", "unknown", 1},
		{func(n int) string { return calculator.DihedralMode(n).String() }, "dihedralMode", "unknown", 1},
		{func(n int) string { return calculator.CoefficientScope(n).String() }, "coefficientScope", "unknown", 1},
		{func(n int) string { return calculator.CaseScopeKind(n).String() }, "caseScope", "unknown", 1},
		{func(n int) string { return calculator.BoundDirection(n).String() }, "boundDirection", "unknown", 1},
		{func(n int) string { return calculator.IssueKind(n).String() }, "issueKind", "unknown", 0},
		{func(n int) string { return calculator.SourceKind(n).String() }, "sourceKind", "unknown", 0},
		{func(n int) string { return calculator.LimitStatus(n).String() }, "requirementStatus", "unknown", 0},
		{func(n int) string { return calculator.ResultStatus(n).String() }, "resultStatus", "missing", 0},
		{func(n int) string { return calculator.Dimension(n).String() }, "dimension", "unknown-dimension", 0},
		{func(n int) string { return calculator.ComponentRole(n).String() }, "componentRole", "unknown", 1},
		{func(n int) string { return calculator.MassMode(n).String() }, "massMode", "unknown", 1},
		{func(n int) string { return calculator.ViewKind(n).String() }, "viewKind", "unknown", 1},
		{func(n int) string { return calculator.SketchRole(n).String() }, "sketchRole", "unknown", 1},
		{func(n int) string { return calculator.DimensionKind(n).String() }, "dimensionKind", "unknown", 1},
		{func(n int) string { return calculator.OutlinePlane(n).String() }, "outlinePlane", "unknown", 1},
		{func(n int) string { return calculator.CapabilityKind(n).String() }, "capabilityKind", "unknown", 1},
		{func(n int) string { return calculator.RegulatorSide(n).String() }, "regulatorSide", "unknown", 1},
		{func(n int) string { return calculator.BatteryEnergyMode(n).String() }, "batteryEnergyMode", "unknown", 1},
		{func(n int) string { return calculator.SegmentKind(n).String() }, "segmentKind", "unknown", 1},
		{func(n int) string { return calculator.SegmentModel(n).String() }, "segmentModel", "unknown", 1},
		{func(n int) string { return calculator.SegmentTiming(n).String() }, "segmentTiming", "unknown", 1},
	} {
		t.Run(tc.vocabulary, func(t *testing.T) {
			want := countNamed(tc.name, tc.first, tc.past)
			if want == 0 {
				t.Fatal("the counter found no constants, so the check would pass vacuously")
			}
			if got := len(vocabularyTokens(t, tc.vocabulary)); got != want {
				t.Errorf("the vocabulary publishes %d tokens but the core defines %d named constants",
					got, want)
			}
		})
	}
}

// countNamed counts an enum's constants from first upwards, stopping when
// String() falls back to past. The first value is always counted: several of
// these enums have a zero value whose name happens to read like the fallback,
// and stopping there would count nothing at all.
func countNamed(name func(int) string, first int, past string) int {
	found := 0
	for n := first; n < 64; n++ {
		if n > first && name(n) == past {
			return found
		}
		found++
	}
	return found
}

func vocabularyTokens(t *testing.T, name string) []string {
	t.Helper()
	for _, vocabulary := range api.Vocabularies() {
		if vocabulary.Name == name {
			return vocabulary.Tokens
		}
	}
	t.Fatalf("no vocabulary named %q is published", name)
	return nil
}

// TestDiscoveryPublishesEveryEquationWithItsProvenance holds that the boundary
// preserves the source record rather than flattening it: a book method and a
// derived inversion must stay distinguishable through the API.
func TestDiscoveryPublishesEveryEquationWithItsProvenance(t *testing.T) {
	discovery := api.NewService().Discover()
	if discovery.ContractVersion != api.ContractVersion {
		t.Errorf("contract version = %q, want %q", discovery.ContractVersion, api.ContractVersion)
	}
	ids := calculator.EquationIDs()
	if len(discovery.Equations) != len(ids) {
		t.Fatalf("discovery lists %d equations, the registry holds %d", len(discovery.Equations), len(ids))
	}
	kinds := map[string]int{}
	for n := range discovery.Equations {
		kinds[discovery.Equations[n].Source.Kind]++
		wantPublishedEquation(t, discovery.Equations[n])
	}
	for _, kind := range []string{"book", "supplementary", "derived"} {
		if kinds[kind] == 0 {
			t.Errorf("no equation is published with source kind %q; the distinction has collapsed", kind)
		}
	}

	if len(discovery.Units) != len(calculator.Units()) {
		t.Errorf("discovery lists %d units, the table holds %d", len(discovery.Units), len(calculator.Units()))
	}
	if discovery.Limits.MaxBatch != api.MaxBatch || discovery.Limits.MaxCases != api.MaxCases {
		t.Errorf("discovery limits = %+v, want the enforced ones", discovery.Limits)
	}
}

// wantPublishedEquation checks one published equation against the registry it
// came from, including its provenance and its assumptions.
func wantPublishedEquation(t *testing.T, equation api.Equation) {
	t.Helper()
	core, err := calculator.Lookup(equation.ID)
	if err != nil {
		t.Fatalf("published equation %q is not in the registry", equation.ID)
	}
	if equation.Revision != core.Revision || equation.Expression != core.Expression {
		t.Errorf("%s: published %q at %q, registry holds %q at %q",
			equation.ID, equation.Expression, equation.Revision, core.Expression, core.Revision)
	}
	if equation.Source.Kind == "" || equation.Source.Title == "" || equation.Source.Accessed == "" {
		t.Errorf("%s: source record is incomplete: %+v", equation.ID, equation.Source)
	}
	if len(equation.Assumptions) != len(core.Assumptions) {
		t.Errorf("%s: published %d assumptions, the registry holds %d",
			equation.ID, len(equation.Assumptions), len(core.Assumptions))
	}
	if equation.Output.Unit == "" {
		t.Errorf("%s: the output port publishes no unit", equation.ID)
	}
}

// TestPowerFirstJourneyIsPublishedWithItsLimits holds that the boundary
// publishes the power-first journey together with the limits that keep it
// honest. Task 09 made it supported; what must never be dropped is the
// statement that a mass and a power ceiling do not size a wing between them.
func TestPowerFirstJourneyIsPublishedWithItsLimits(t *testing.T) {
	patterns := api.NewService().Patterns()
	found := 0
	for n := range patterns {
		pattern := &patterns[n]
		if pattern.Journey != "power-first" {
			continue
		}
		found++
		if !pattern.Supported {
			t.Errorf("%s: Task 09 implements the models this journey needs", pattern.ID)
		}
		if len(pattern.ValidityLimits) == 0 {
			t.Errorf("%s: a supported pattern must still say where it stops applying", pattern.ID)
		}
		if len(pattern.RequiredInputs) == 0 {
			t.Errorf("%s: a supported pattern must say what it needs", pattern.ID)
		}
	}
	if found == 0 {
		t.Error("the power-first journey is not published at all")
	}
}

// TestUnknownEquationIsNotFound separates "no such equation" from "your request
// was wrong", because they are different problems for a client.
func TestUnknownEquationIsNotFound(t *testing.T) {
	_, err := api.NewService().Equation("lift.does-not-exist")
	_ = wantFailure(t, err, api.FailureNotFound)
}

// TestMalformedRequestsAreRefusedWithFieldIssues covers the shapes a client can
// get wrong: a missing identity, a bad unit symbol, a non-finite number and an
// unknown enum token. Each names the field it is about.
func TestMalformedRequestsAreRefusedWithFieldIssues(t *testing.T) {
	service := api.NewService()
	for name, tc := range map[string]struct {
		mutate func(*api.EvaluateRequest)
		field  string
		kind   api.FailureKind
	}{
		"missing session": {
			mutate: func(r *api.EvaluateRequest) { r.Request.Session = "" },
			field:  "request.session", kind: api.FailureInvalid,
		},
		"missing sequence": {
			mutate: func(r *api.EvaluateRequest) { r.Request.Sequence = 0 },
			field:  "request.sequence", kind: api.FailureInvalid,
		},
		"unknown unit": {
			mutate: func(r *api.EvaluateRequest) { r.Design.Mass = &api.Quantity{Value: 2, Unit: "stone"} },
			field:  "design.mass", kind: api.FailureInvalid,
		},
		"non-finite number": {
			mutate: func(r *api.EvaluateRequest) { r.Design.Wing.AspectRatio = math.Inf(1) },
			field:  "design.wing.aspectRatio", kind: api.FailureInvalid,
		},
		"unknown enum token": {
			mutate: func(r *api.EvaluateRequest) { r.Design.Configuration = "biplane" },
			field:  "design.configuration", kind: api.FailureInvalid,
		},
		"unknown requirement subject": {
			mutate: func(r *api.EvaluateRequest) { r.Design.Requirements[0].Subject = "roll-rate" },
			field:  "design.requirements[0].subject", kind: api.FailureInvalid,
		},
	} {
		t.Run(name, func(t *testing.T) {
			request := evaluateRequest()
			tc.mutate(&request)
			_, err := service.Evaluate(t.Context(), request)
			wantIssueOn(t, wantFailure(t, err, tc.kind), tc.field)
		})
	}
}

// TestOversizedPayloadsAreRefusedByCount holds the published limits. They live
// in the api package rather than in the transport so that every adapter is
// bounded the same way.
func TestOversizedPayloadsAreRefusedByCount(t *testing.T) {
	service := api.NewService()

	t.Run("too many cases", func(t *testing.T) {
		request := evaluateRequest()
		for len(request.Design.Cases) <= api.MaxCases {
			extra := request.Design.Cases[0]
			extra.Name += "+"
			request.Design.Cases = append(request.Design.Cases, extra)
		}
		_, err := service.Evaluate(t.Context(), request)
		wantIssueOn(t, wantFailure(t, err, api.FailureUnsupported), "design.cases")
	})

	t.Run("too many batch entries", func(t *testing.T) {
		batch := api.BatchEvaluateRequest{}
		for len(batch.Evaluations) <= api.MaxBatch {
			batch.Evaluations = append(batch.Evaluations, evaluateRequest())
		}
		_, err := service.EvaluateBatch(t.Context(), batch)
		_ = wantFailure(t, err, api.FailureTooLarge)
	})

	t.Run("too many commands", func(t *testing.T) {
		request := api.ApplyRequest{Request: identity(), Design: wireDesign()}
		for len(request.Commands) <= api.MaxCommands {
			request.Commands = append(request.Commands, api.Command{
				Kind:  api.CmdSetTaperRatio,
				Ratio: 1,
			})
		}
		_, err := service.Apply(t.Context(), request)
		_ = wantFailure(t, err, api.FailureTooLarge)
	})

	t.Run("an empty batch is refused", func(t *testing.T) {
		_, err := service.EvaluateBatch(t.Context(), api.BatchEvaluateRequest{})
		_ = wantFailure(t, err, api.FailureInvalid)
	})
}

// TestCancellationIsReportedAsCancellation holds that a caller who goes away
// gets a cancellation rather than an empty success.
func TestCancellationIsReportedAsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	service := api.NewService()
	_, err := service.Evaluate(ctx, evaluateRequest())
	_ = wantFailure(t, err, api.FailureCancelled)

	batch := api.BatchEvaluateRequest{Evaluations: []api.EvaluateRequest{evaluateRequest()}}
	_, err = service.EvaluateBatch(ctx, batch)
	_ = wantFailure(t, err, api.FailureCancelled)
}

// TestApplySizesAtTheStallLimitAndReturnsTheEditedDesign is the curated action
// crossing the boundary: the wire form of "size at the stall limit, keep span"
// produces the same area the core does, and the design comes back ready to be
// sent again.
func TestApplySizesAtTheStallLimitAndReturnsTheEditedDesign(t *testing.T) {
	response, err := api.NewService().Apply(t.Context(), api.ApplyRequest{
		Request: identity(),
		Design:  wireDesign(),
		Commands: []api.Command{{
			Kind:  api.CmdSizeAtStallLimit,
			Hold:  "wing.span.projected",
			Scope: &api.Scope{Kind: "single", Case: fixtureCaseName},
		}},
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(response.Applied) != 1 {
		t.Fatalf("applied = %v, want the one command", response.Applied)
	}
	if response.Design.Wing.Area == nil {
		t.Fatalf("the edited design carries no area driver: %+v", response.Design.Wing)
	}
	wantValue(t, "sized area", response.Design.Wing.Area, fixtureMinArea, "m^2")
	if response.Design.Wing.AspectRatio != 0 {
		t.Errorf("the aspect ratio should have been released, got %v", response.Design.Wing.AspectRatio)
	}
	if response.Evaluation.Checks[0].Status != "met" {
		t.Errorf("stall check = %q, want met at the boundary", response.Evaluation.Checks[0].Status)
	}

	// The returned design is a complete request body: sending it back must
	// reproduce the same evaluation, which is what makes the boundary stateless
	// rather than merely undocumented.
	again, err := api.NewService().Evaluate(t.Context(), api.EvaluateRequest{
		Request: api.Request{Session: "fixture", Sequence: 8},
		Design:  response.Design,
	})
	if err != nil {
		t.Fatalf("re-evaluate: %v", err)
	}
	if again.Snapshot != response.Evaluation.Snapshot {
		t.Error("the returned design does not round-trip to the same definition")
	}
}

// TestARefusedCommandAppliesNothing holds that an apply call is all or nothing,
// the same guarantee a Session gives.
func TestARefusedCommandAppliesNothing(t *testing.T) {
	_, err := api.NewService().Apply(t.Context(), api.ApplyRequest{
		Request: identity(),
		Design:  wireDesign(),
		Commands: []api.Command{
			{Kind: api.CmdSetTaperRatio, Ratio: 1},
			{Kind: api.CmdSetDriver, Key: "wing.area.reference", Value: &api.Quantity{Value: 0.3, Unit: "m^2"}},
		},
	})
	if len(wantFailure(t, err, api.FailureInvalid)) == 0 {
		t.Error("a refused command should carry the core's field issues")
	}
}

// TestCommandsMustMatchTheirKind holds the flat union's field rules: a field
// that does not belong to a kind is refused rather than ignored, and a missing
// one is named.
func TestCommandsMustMatchTheirKind(t *testing.T) {
	service := api.NewService()
	for name, tc := range map[string]struct {
		field   string
		command api.Command
	}{
		"extra field": {
			command: api.Command{
				Kind: api.CmdSetMass, Basis: "revised",
				Mass: &api.Quantity{Value: 1.5, Unit: "kg"},
				Hold: "wing.span.projected",
			},
			field: "commands[0].hold",
		},
		"missing field": {
			command: api.Command{Kind: api.CmdSetRequirementPriority, Name: "Stall ceiling"},
			field:   "commands[0].priority",
		},
		"unknown kind": {
			command: api.Command{Kind: "set-everything"},
			field:   "commands[0].kind",
		},
		"missing kind": {
			command: api.Command{},
			field:   "commands[0].kind",
		},
		"bad driver key": {
			command: api.Command{
				Kind: api.CmdSetDriver, Key: "wing.chord.mac",
				Value: &api.Quantity{Value: 0.3, Unit: "m"},
			},
			field: "commands[0].key",
		},
		"scope without a case": {
			command: api.Command{
				Kind: api.CmdSizeAtStallLimit, Hold: "wing.span.projected",
				Scope: &api.Scope{Kind: "single"},
			},
			field: "commands[0].scope.case",
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := service.Apply(t.Context(), api.ApplyRequest{
				Request:  identity(),
				Design:   wireDesign(),
				Commands: []api.Command{tc.command},
			})
			wantIssueOn(t, wantFailure(t, err, api.FailureInvalid), tc.field)
		})
	}
}

// TestPromotionWithoutAReleaseOffersTheSwapsThroughTheAPI holds that the
// ambiguity the core reports survives the boundary as a usable message rather
// than as a bare rejection.
func TestPromotionWithoutAReleaseOffersTheSwapsThroughTheAPI(t *testing.T) {
	_, err := api.NewService().Apply(t.Context(), api.ApplyRequest{
		Request: identity(),
		Design:  wireDesign(),
		Commands: []api.Command{{
			Kind:    api.CmdPromoteDriver,
			Promote: "wing.area.reference",
			Value:   &api.Quantity{Value: 0.3, Unit: "m^2"},
		}},
	})
	issue := wantIssueOn(t, wantFailure(t, err, api.FailureInvalid), "wing.area.reference")
	if !contains(issue.Detail, "wing.aspect_ratio.planform") {
		t.Errorf("the offer %q does not name the swaps on offer", issue.Detail)
	}
}

// TestPreviewCrossesTheBoundaryWithoutApplying holds that a preview reports the
// requirement changes and leaves the design alone.
func TestPreviewCrossesTheBoundaryWithoutApplying(t *testing.T) {
	response, err := api.NewService().Preview(t.Context(), api.PreviewRequest{
		Request: identity(),
		Design:  wireDesign(),
		Command: api.Command{
			Kind:  api.CmdSizeAtStallLimit,
			Hold:  "wing.span.projected",
			Scope: &api.Scope{Kind: "single", Case: fixtureCaseName},
		},
	})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if len(response.Changes) != 1 {
		t.Fatalf("changes = %+v, want exactly one", response.Changes)
	}
	if response.Changes[0].From != "unmet" || response.Changes[0].To != "met" {
		t.Errorf("change = %+v, want the ceiling going unmet to met", response.Changes[0])
	}
	if response.Before.Snapshot == response.After.Snapshot {
		t.Error("the two sides of a preview should describe different definitions")
	}
	if response.Before.Snapshot != coreDesign(t).Evaluate().Snapshot {
		t.Error("the before side should describe the design the client sent")
	}
}

// TestConflictAlternativesCrossTheBoundaryAsCommands holds that an offered
// alternative arrives as something a client can send back, not as prose it would
// have to turn into an edit itself.
func TestConflictAlternativesCrossTheBoundaryAsCommands(t *testing.T) {
	request := evaluateRequest()
	request.Design.Requirements = append(request.Design.Requirements, api.Requirement{
		Name: "maximum wing area", Subject: "wing-area", Priority: "required",
		Basis: "the sheet stock available", Maximum: &api.Quantity{Value: 0.3, Unit: "m^2"},
	})
	evaluation, err := api.NewService().Evaluate(t.Context(), request)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(evaluation.Conflicts) != 1 {
		t.Fatalf("conflicts = %+v, want exactly one", evaluation.Conflicts)
	}
	conflict := evaluation.Conflicts[0]
	if len(conflict.Alternatives) == 0 {
		t.Fatal("the conflict offers no alternatives")
	}
	for _, alternative := range conflict.Alternatives {
		if alternative.Command.Kind == "" {
			t.Errorf("alternative %q carries no command a client could send", alternative.Name)
		}
	}

	// Sending one back resolves the conflict, which is what makes the offer real.
	applied, err := api.NewService().Apply(t.Context(), api.ApplyRequest{
		Request:  identity(),
		Design:   request.Design,
		Commands: []api.Command{conflict.Alternatives[0].Command},
	})
	if err != nil {
		t.Fatalf("applying the offered alternative: %v", err)
	}
	if len(applied.Evaluation.Conflicts) != 0 {
		t.Errorf("the alternative did not resolve the conflict: %+v", applied.Evaluation.Conflicts)
	}
}

// TestDomainFailureIsAResultNotAnError holds that a design that does not solve
// still comes back as an evaluation, with its issues and whatever bounds could
// be established. Refusing it would make an unfinished worksheet unreadable.
func TestDomainFailureIsAResultNotAnError(t *testing.T) {
	request := evaluateRequest()
	request.Design.Wing.Shape = "trapezoid"
	evaluation, err := api.NewService().Evaluate(t.Context(), request)
	if err != nil {
		t.Fatalf("a design that does not solve is not a request failure: %v", err)
	}
	if evaluation.Geometry == "computed" {
		t.Fatal("a trapezoid with no taper ratio must not solve")
	}
	if len(evaluation.GeometryIssues) == 0 {
		t.Fatal("the evaluation should say why the geometry did not solve")
	}
	if evaluation.GeometryIssues[0].Kind != "missing" || evaluation.GeometryIssues[0].Field != "taper_ratio" {
		t.Errorf("geometry issue = %+v, want a missing taper ratio", evaluation.GeometryIssues[0])
	}
	if evaluation.Wing != nil {
		t.Error("an unsolved design must not carry a partial wing")
	}
	if !evaluation.AreaLower.Known {
		t.Fatalf("the area bound needs no geometry and should still be reported: %+v", evaluation.AreaLower)
	}
	wantValue(t, "area bound without geometry", evaluation.AreaLower.Value, fixtureMinArea, "m^2")
}

// TestGeneratedTypeScriptMatchesTheCheckedInFile holds the single contract
// authority: the Go types generate the frontend's, and a change to one without
// the other fails here rather than at run time in a browser.
func TestGeneratedTypeScriptMatchesTheCheckedInFile(t *testing.T) {
	const path = "../frontend/src/api/contract.ts"
	checked, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if string(checked) != api.TypeScript() {
		t.Errorf("%s is out of date; regenerate it with:\n\tgo run ./cmd/aero contract > %s", path, path)
	}
}

func contains(haystack, needle string) bool {
	if len(needle) > len(haystack) {
		return false
	}
	for n := range haystack {
		if len(haystack) < n+len(needle) {
			return false
		}
		if haystack[n:n+len(needle)] == needle {
			return true
		}
	}
	return false
}

// TestTheWorksheetCommandsCrossTheBoundary covers the three edits the worksheet
// needs beyond the sizing ones: the planform shape with its taper ratio, every
// wing angle at once, and the layout with its tail description. Each is one
// command because its parts constrain each other, and the boundary refuses a
// half-stated one rather than letting the design pass through a state that is
// neither thing.
func TestTheWorksheetCommandsCrossTheBoundary(t *testing.T) {
	service := api.NewService()
	angles := &api.Angles{
		Sweep:          &api.Quantity{Value: 5, Unit: "deg"},
		Dihedral:       &api.Quantity{Value: 4, Unit: "deg"},
		Twist:          &api.Quantity{Value: -2, Unit: "deg"},
		Incidence:      &api.Quantity{Value: 1, Unit: "deg"},
		SweepReference: 0.25,
		DihedralMode:   "hold-projected",
	}
	response, err := service.Apply(t.Context(), api.ApplyRequest{
		Request: identity(),
		Design:  wireDesign(),
		Commands: []api.Command{
			{Kind: api.CmdSetPlanformShape, Shape: "trapezoid", Ratio: 0.5},
			{Kind: api.CmdSetWingAngles, Angles: angles},
			{Kind: api.CmdSetConfiguration, Configuration: "flying-wing"},
		},
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(response.Applied) != 3 {
		t.Fatalf("applied = %v, want all three", response.Applied)
	}
	if response.Design.Wing.Shape != "trapezoid" || response.Design.Wing.TaperRatio != 0.5 {
		t.Errorf("shape = %+v, want a trapezoid at 0.5", response.Design.Wing)
	}
	if response.Design.Wing.Dihedral == nil || response.Design.Wing.Dihedral.Value == 0 {
		t.Errorf("dihedral = %+v, want the 4 degrees that were set", response.Design.Wing.Dihedral)
	}
	if response.Evaluation.Geometry != "computed" {
		t.Fatalf("geometry = %q: %+v", response.Evaluation.Geometry, response.Evaluation.GeometryIssues)
	}

	for name, tc := range map[string]struct {
		field   string
		command api.Command
	}{
		"a rectangle cannot taper": {
			field:   "taper_ratio",
			command: api.Command{Kind: api.CmdSetPlanformShape, Shape: "rectangle", Ratio: 0.5},
		},
		"a trapezoid needs its taper": {
			field:   "commands[0].ratio",
			command: api.Command{Kind: api.CmdSetPlanformShape, Shape: "trapezoid"},
		},
		"an unnamed layout is refused": {
			field:   "commands[0].configuration",
			command: api.Command{Kind: api.CmdSetConfiguration},
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, applyErr := service.Apply(t.Context(), api.ApplyRequest{
				Request: identity(), Design: wireDesign(), Commands: []api.Command{tc.command},
			})
			wantIssueOn(t, wantFailure(t, applyErr, api.FailureInvalid), tc.field)
		})
	}

	t.Run("a partly stated angle set is refused", func(t *testing.T) {
		_, applyErr := service.Apply(t.Context(), api.ApplyRequest{
			Request: identity(), Design: wireDesign(),
			Commands: []api.Command{{
				Kind: api.CmdSetWingAngles,
				Angles: &api.Angles{
					Sweep: &api.Quantity{Value: 0, Unit: "deg"}, SweepReference: 0.25,
				},
			}},
		})
		wantIssueOn(t, wantFailure(t, applyErr, api.FailureInvalid), "incidence")
	})
}

// wantArrays asserts that each named field marshalled as a JSON array.
func wantArrays(t *testing.T, decoded map[string]any, prefix string, fields ...string) {
	t.Helper()
	for _, field := range fields {
		value, present := decoded[field]
		if !present {
			t.Errorf("%s%s is absent, but the contract declares it as an array", prefix, field)
			continue
		}
		if _, ok := value.([]any); !ok {
			t.Errorf("%s%s marshalled as %T, want an array", prefix, field, value)
		}
	}
}

// TestArrayFieldsAreNeverNull holds a property the generated TypeScript quietly
// depends on. A Go nil slice marshals to JSON null, and the contract declares
// these fields as arrays; a client that trusted the declared type would then
// iterate over null. The worst case is the emptiest design, where every slice
// that could be nil is.
func TestArrayFieldsAreNeverNull(t *testing.T) {
	empty := api.EvaluateRequest{
		Request: identity(),
		Design:  api.Design{Name: "empty", Configuration: "flying-wing"},
	}
	for name, request := range map[string]api.EvaluateRequest{
		"an empty design": empty,
		"the fixture":     evaluateRequest(),
	} {
		t.Run(name, func(t *testing.T) {
			evaluation, err := api.NewService().Evaluate(t.Context(), request)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			encoded, err := json.Marshal(evaluation)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var decoded map[string]any
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			wantArrays(t, decoded, "", "checks", "conflicts", "patterns",
				"definitionIssues", "geometryIssues", "configurationIssues")
			if wing, ok := decoded["wing"].(map[string]any); ok {
				wantArrays(t, wing, "wing.", "drivers", "parameters", "outline")
			}
		})
	}
}
