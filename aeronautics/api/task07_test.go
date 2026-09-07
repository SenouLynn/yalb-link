package api_test

import (
	"context"
	"encoding/json"
	"math"
	"testing"

	"yalb.aero/api"
	"yalb.aero/calculator"
)

// wirePlacement is the task's placement fixture on the wire: 1.5 kg of airframe
// at x = 0.4 m and a 0.5 kg battery at x = 0.2 m, with the all-up mass taken
// from the components. Every coordinate is stated, zero included.
func wirePlacement() api.Design {
	design := wireDesign()
	design.MassMode = "components"
	design.Components = []api.Component{
		wireComponent("airframe", "airframe", 1.5, 0.4),
		wireComponent("battery", "battery", 0.5, 0.2),
	}
	return design
}

func wireComponent(name, role string, mass, x float64) api.Component {
	metres := func(v float64) *api.Quantity { return &api.Quantity{Value: v, Unit: "m"} }
	return api.Component{
		Name:     name,
		Role:     role,
		Basis:    "fixture value, independently computed",
		Mass:     &api.Quantity{Value: mass, Unit: "kg"},
		Position: api.Position{X: metres(x), Y: metres(0), Z: metres(0)},
	}
}

func mustEvaluate(t *testing.T, design api.Design) api.Evaluation {
	t.Helper()
	evaluation, err := api.NewService().Evaluate(t.Context(),
		api.EvaluateRequest{Request: identity(), Design: design})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	return evaluation
}

// The balance crosses the boundary with its datum, its total and its station,
// and it agrees with a direct core evaluation of the same design.
func TestTheBalanceCrossesTheBoundary(t *testing.T) {
	evaluation := mustEvaluate(t, wirePlacement())
	mp := evaluation.MassProperties
	if mp.Status != "computed" {
		t.Fatalf("status = %q, want computed (%s)", mp.Status, mp.Detail)
	}
	if !mp.Complete {
		t.Errorf("the inventory is complete; the boundary says %q", mp.Detail)
	}
	if mp.Datum == "" {
		t.Error("the balance crossed without its datum")
	}
	wantValue(t, "total mass", mp.Total, 2, "kg")
	if mp.CG == nil {
		t.Fatal("no centre of gravity")
	}
	wantValue(t, "x_cg", &mp.CG.X, 0.35, "m")
	wantValue(t, "y_cg", &mp.CG.Y, 0, "m")

	if len(mp.Contributions) != 2 {
		t.Fatalf("%d contributions, want one per component", len(mp.Contributions))
	}
	for _, contribution := range mp.Contributions {
		if !contribution.Known {
			t.Errorf("%s did not contribute: %s", contribution.Name, contribution.Detail)
		}
		if len(contribution.Moments) != 3 {
			t.Errorf("%s carries %d moments, want one per axis", contribution.Name, len(contribution.Moments))
		}
	}
}

// A component with no position is reported as unplaced rather than being given
// one, and the boundary says so instead of quietly returning a smaller number.
func TestAnUnplacedComponentCrossesAsUnplaced(t *testing.T) {
	design := wirePlacement()
	design.Components = append(design.Components, api.Component{
		Name: "payload", Role: "payload", Basis: "fixture",
		Mass: &api.Quantity{Value: 0.5, Unit: "kg"},
	})
	mp := mustEvaluate(t, design).MassProperties
	if mp.Complete {
		t.Fatal("an incomplete inventory crossed as a complete one")
	}
	wantValue(t, "x_cg", &mp.CG.X, 0.35, "m")
	for _, contribution := range mp.Contributions {
		if contribution.Name != "payload" {
			continue
		}
		if contribution.Known {
			t.Fatal("the unplaced component contributed")
		}
		if contribution.Position.X != nil {
			t.Error("an unplaced component crossed carrying a coordinate")
		}
		if contribution.Detail == "" {
			t.Error("no reason was given")
		}
	}
}

// The placement commands cross the boundary as commands, and a completed drag
// is one apply call that answers with the edited design and its balance.
func TestThePlacementCommandsCrossTheBoundary(t *testing.T) {
	applied, err := api.NewService().Apply(t.Context(), api.ApplyRequest{
		Request: identity(),
		Design:  wirePlacement(),
		Commands: []api.Command{{
			Kind: "place-component",
			Name: "battery",
			Position: &api.Position{
				X: &api.Quantity{Value: 0.6, Unit: "m"},
				Y: &api.Quantity{Value: 0, Unit: "m"},
				Z: &api.Quantity{Value: 0, Unit: "m"},
			},
		}},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	wantValue(t, "x_cg", &applied.Evaluation.MassProperties.CG.X, 0.45, "m")
	// The edited design comes back in a form the client can send straight back.
	if len(applied.Design.Components) != 2 {
		t.Fatalf("%d components came back, want 2", len(applied.Design.Components))
	}
	for _, component := range applied.Design.Components {
		if component.Name == "battery" {
			wantValue(t, "the battery's x", component.Position.X, 0.6, "m")
		}
	}
	// And the all-up mass is unchanged, because moving a fixed mass is not
	// resizing it.
	stall := stallSpeedFrom(t, applied.Evaluation)
	if math.Abs(stall-fixtureStall) > fixtureTolerance*fixtureStall {
		t.Errorf("stall speed = %v, want %v; a placement changes no loading", stall, fixtureStall)
	}
}

func stallSpeedFrom(t *testing.T, evaluation api.Evaluation) float64 {
	t.Helper()
	for n := range evaluation.Checks {
		check := &evaluation.Checks[n]
		if check.Name == fixtureStallReq && check.Actual != nil {
			return check.Actual.Value
		}
	}
	t.Fatal("no stall-speed check in the evaluation")
	return 0
}

// A placement missing a coordinate is refused with the coordinate named, and
// nothing is applied.
func TestAPlacementMissingACoordinateIsRefused(t *testing.T) {
	_, err := api.NewService().Apply(t.Context(), api.ApplyRequest{
		Request: identity(),
		Design:  wirePlacement(),
		Commands: []api.Command{{
			Kind:     "place-component",
			Name:     "battery",
			Position: &api.Position{X: &api.Quantity{Value: 0.6, Unit: "m"}},
		}},
	})
	issues := wantFailure(t, err, api.FailureInvalid)
	wantIssueOn(t, issues, "component.battery")
}

// A command that carries a field its kind does not use is refused rather than
// having the field ignored, which is what stops a typo producing a plausible
// wrong answer.
func TestAPlacementCarryingAMassIsRefused(t *testing.T) {
	_, err := api.NewService().Apply(t.Context(), api.ApplyRequest{
		Request: identity(),
		Design:  wirePlacement(),
		Commands: []api.Command{{
			Kind:     "place-component",
			Name:     "battery",
			Mass:     &api.Quantity{Value: 1, Unit: "kg"},
			Position: &api.Position{X: &api.Quantity{Value: 0.6, Unit: "m"}},
		}},
	})
	issues := wantFailure(t, err, api.FailureInvalid)
	wantIssueOn(t, issues, "commands[0].mass")
}

// Every dimension a view carries names a parameter the same response reports,
// and carries that parameter's value. That is the whole basis of selecting a
// dimension and having it select a field.
func TestEveryDimensionNamesAParameterInTheSameResponse(t *testing.T) {
	evaluation := mustEvaluate(t, wireDesign())
	wing := evaluation.Wing
	if wing == nil {
		t.Fatal("the fixture wing did not solve")
	}
	values := make(map[string]api.Quantity, len(wing.Parameters))
	for _, parameter := range wing.Parameters {
		values[parameter.Key] = parameter.Value
	}
	if len(wing.Views) != 3 {
		t.Fatalf("%d views, want plan, front and side", len(wing.Views))
	}
	for n := range wing.Views {
		checkView(t, &wing.Views[n], values)
	}
}

func checkView(t *testing.T, view *api.SketchView, values map[string]api.Quantity) {
	t.Helper()
	if view.View == "" || view.Datum == "" || view.Across == "" || view.Up == "" {
		t.Errorf("a view crossed without its name, datum or axes: %q", view.View)
	}
	if len(view.Curves) == 0 || len(view.Dimensions) == 0 {
		t.Errorf("%s carries nothing to draw", view.View)
	}
	for d := range view.Dimensions {
		dimension := &view.Dimensions[d]
		value, ok := values[dimension.Key]
		switch {
		case !ok:
			t.Errorf("%s dimension %q names no reported parameter", view.View, dimension.Key)
		case value != dimension.Value:
			t.Errorf("%s dimension %q shows %v, the parameter holds %v",
				view.View, dimension.Key, dimension.Value, value)
		case dimension.Kind == "" || dimension.Plane == "":
			t.Errorf("%s dimension %q does not state its kind or its plane",
				view.View, dimension.Key)
		}
	}
}

// Every parameter is explained, with the expression and the substitutions the
// core recorded, so a frontend never has to reconstruct a formula.
func TestEveryParameterIsExplainedOnTheWire(t *testing.T) {
	wing := mustEvaluate(t, wireDesign()).Wing
	if wing == nil {
		t.Fatal("the fixture wing did not solve")
	}
	if len(wing.Explanations) != len(wing.Parameters) {
		t.Fatalf("%d explanations for %d parameters", len(wing.Explanations), len(wing.Parameters))
	}
	derived := 0
	for _, explanation := range wing.Explanations {
		if explanation.Detail == "" {
			t.Errorf("%s explains nothing", explanation.Key)
		}
		if explanation.Role != "derived" {
			continue
		}
		derived++
		if explanation.Expression == "" || explanation.EquationID == "" {
			t.Errorf("%s is derived but carries no relationship", explanation.Key)
		}
		if len(explanation.Substitutions) == 0 {
			t.Errorf("%s is derived but substituted nothing", explanation.Key)
		}
	}
	if derived == 0 {
		t.Fatal("no derived parameter was explained, so the check passed vacuously")
	}
}

// A sweep crosses the boundary with what it held fixed, the boundary it is
// judged against and the marker for the design's own candidate.
func TestASweepCrossesTheBoundary(t *testing.T) {
	response, err := api.NewService().Sweep(t.Context(), api.SweepRequest{
		Request: identity(),
		Design:  wireDesign(),
		Settings: api.SweepSettings{
			Driver:  "wing.aspect_ratio.planform",
			From:    api.Quantity{Value: 4, Unit: "1"},
			To:      api.Quantity{Value: 10, Unit: "1"},
			Samples: 7,
			Output:  api.SweepOutput{Subject: "stall-speed", Case: fixtureCaseName},
		},
	})
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if response.Request != identity() {
		t.Errorf("identity = %+v, want the one asked with", response.Request)
	}
	if response.Snapshot == "" || response.SettingsFingerprint == "" {
		t.Error("a sweep crossed without its snapshot or its settings fingerprint")
	}
	if response.SolveMode != "span-and-aspect-ratio" {
		t.Errorf("solve mode = %q; a plot without it is ambiguous", response.SolveMode)
	}
	if len(response.Samples) != 7 {
		t.Fatalf("%d samples, want 7", len(response.Samples))
	}
	previous := math.NaN()
	for n, sample := range response.Samples {
		if sample.Status != "computed" {
			t.Fatalf("sample %d: %s", n, sample.Detail)
		}
		if sample.Value == nil {
			t.Fatalf("sample %d computed but carries no value", n)
		}
		if !math.IsNaN(previous) && sample.Value.Value <= previous {
			t.Errorf("sample %d did not rise with the aspect ratio at a fixed span", n)
		}
		previous = sample.Value.Value
	}
	if len(response.HeldFixed) == 0 {
		t.Error("the sweep does not say what it held fixed")
	}
	if len(response.Bounds) != 1 {
		t.Fatalf("%d boundaries, want the one stall ceiling", len(response.Bounds))
	}
	wantValue(t, "the plotted boundary", &response.Bounds[0].Value, 8, "m/s")
	wantValue(t, "the marker", response.Current.Value, fixtureStall, "m/s")
}

// Every plotted sample is the candidate itself, so an evaluation of the same
// design at the same driver value produces the same number.
func TestEverySweptSampleMatchesAnEvaluationOfTheSameCandidate(t *testing.T) {
	service := api.NewService()
	response, err := service.Sweep(t.Context(), api.SweepRequest{
		Request: identity(),
		Design:  wireDesign(),
		Settings: api.SweepSettings{
			Driver:  "wing.aspect_ratio.planform",
			From:    api.Quantity{Value: 4, Unit: "1"},
			To:      api.Quantity{Value: 10, Unit: "1"},
			Samples: 4,
			Output:  api.SweepOutput{Subject: "stall-speed", Case: fixtureCaseName},
		},
	})
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	for n, sample := range response.Samples {
		candidate := wireDesign()
		candidate.Wing.AspectRatio = sample.Driver.Value
		evaluation, evalErr := service.Evaluate(t.Context(),
			api.EvaluateRequest{Request: identity(), Design: candidate})
		if evalErr != nil {
			t.Fatalf("Evaluate candidate %d: %v", n, evalErr)
		}
		if got := stallSpeedFrom(t, evaluation); got != sample.Value.Value {
			t.Errorf("sample %d plotted %v, a direct evaluation gives %v", n, sample.Value.Value, got)
		}
	}
}

// A sweep of a value the design does not drive is refused, with the promotion
// it would need explained.
func TestSweepingADerivedValueIsRefused(t *testing.T) {
	_, err := api.NewService().Sweep(t.Context(), api.SweepRequest{
		Request: identity(),
		Design:  wireDesign(),
		Settings: api.SweepSettings{
			Driver:  "wing.chord.root",
			From:    api.Quantity{Value: 0.1, Unit: "m"},
			To:      api.Quantity{Value: 0.4, Unit: "m"},
			Samples: 5,
			Output:  api.SweepOutput{Subject: "stall-speed", Case: fixtureCaseName},
		},
	})
	issues := wantFailure(t, err, api.FailureInvalid)
	wantIssueOn(t, issues, "sweep.driver")
}

// A key that is not a swept driver at all is refused by the contract before the
// core is asked.
func TestSweepingAnUnknownKeyIsRefused(t *testing.T) {
	_, err := api.NewService().Sweep(t.Context(), api.SweepRequest{
		Request: identity(),
		Design:  wireDesign(),
		Settings: api.SweepSettings{
			Driver:  "wing.chord.mac",
			From:    api.Quantity{Value: 0.1, Unit: "m"},
			To:      api.Quantity{Value: 0.4, Unit: "m"},
			Samples: 5,
			Output:  api.SweepOutput{Subject: "stall-speed", Case: fixtureCaseName},
		},
	})
	issues := wantFailure(t, err, api.FailureInvalid)
	wantIssueOn(t, issues, "settings.driver")
}

// A caller that goes away stops the sampling rather than paying for a whole
// answer nobody is waiting for.
func TestACancelledSweepIsReportedAsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := api.NewService().Sweep(ctx, api.SweepRequest{
		Request: identity(),
		Design:  wireDesign(),
		Settings: api.SweepSettings{
			Driver:  "wing.aspect_ratio.planform",
			From:    api.Quantity{Value: 4, Unit: "1"},
			To:      api.Quantity{Value: 10, Unit: "1"},
			Samples: 9,
			Output:  api.SweepOutput{Subject: "stall-speed", Case: fixtureCaseName},
		},
	})
	wantFailure(t, err, api.FailureCancelled)
}

// The published limits say how much work one request may ask for, so a client
// can respect them rather than discover them by being refused.
func TestTheNewLimitsArePublished(t *testing.T) {
	limits := api.NewService().Discover().Limits
	if limits.MaxComponents != api.MaxComponents {
		t.Errorf("maxComponents = %d, want %d", limits.MaxComponents, api.MaxComponents)
	}
	if limits.MaxSweepSamples != calculator.MaxSweepSamples {
		t.Errorf("maxSweepSamples = %d, want %d", limits.MaxSweepSamples, calculator.MaxSweepSamples)
	}
}

// The arrays a client iterates over are arrays, never null.
func TestTask07ArrayFieldsAreNeverNull(t *testing.T) {
	evaluation := mustEvaluate(t, wireDesign())
	encoded, err := json.Marshal(evaluation)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	properties, ok := decoded["massProperties"].(map[string]any)
	if !ok {
		t.Fatal("no massProperties in the response")
	}
	if properties["contributions"] == nil {
		t.Error("massProperties.contributions marshalled as null")
	}
	wing, ok := decoded["wing"].(map[string]any)
	if !ok {
		t.Fatal("no wing in the response")
	}
	for _, field := range []string{"views", "explanations"} {
		if wing[field] == nil {
			t.Errorf("wing.%s marshalled as null", field)
		}
	}
}
