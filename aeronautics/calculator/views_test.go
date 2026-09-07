package calculator_test

import (
	"math"
	"testing"

	"yalb.aero/calculator"
)

func solvedFixtureWing(t *testing.T) calculator.Wing {
	t.Helper()
	w, err := calculator.SolveWing(baseWing(t))
	if err != nil {
		t.Fatalf("SolveWing: %v", err)
	}
	return w
}

// The link between a drawing and a worksheet is one stable key, not a label
// match. Every dimension must name a parameter the wing actually reports, and
// must carry that parameter's value, or selecting a dimension would highlight a
// field holding something else.
func TestEveryDimensionNamesAParameterTheWingReports(t *testing.T) {
	wing := taperedWing(t)
	parameters := wing.Parameters()
	seen := map[calculator.ParameterKey]bool{}
	for _, view := range wing.Views() {
		for _, dimension := range view.Dimensions {
			parameter, ok := parameters.Get(dimension.Key)
			if !ok {
				t.Errorf("%v dimension %q names no reported parameter", view.View, dimension.Key)
				continue
			}
			seen[dimension.Key] = true
			if dimension.Value != parameter.Value {
				t.Errorf("%v dimension %q shows %v, the parameter holds %v",
					view.View, dimension.Key, dimension.Value, parameter.Value)
			}
			if dimension.Label == "" || dimension.Detail == "" {
				t.Errorf("%v dimension %q is unlabelled", view.View, dimension.Key)
			}
			if dimension.Kind == calculator.DimensionKindUnknown {
				t.Errorf("%v dimension %q does not say whether it is a length or an angle",
					view.View, dimension.Key)
			}
		}
	}
	// And the dimensions cover the values a builder is actually driving the
	// sketch from, rather than a decorative subset.
	for _, key := range []calculator.ParameterKey{
		calculator.ParamSpanProjected, calculator.ParamChordRoot, calculator.ParamChordTip,
		calculator.ParamChordMAC, calculator.ParamStationMAC, calculator.ParamDihedral,
		calculator.ParamTwist, calculator.ParamIncidence, calculator.ParamSweepLeadingEdge,
	} {
		if !seen[key] {
			t.Errorf("%s has no dimension on any view", key)
		}
	}
}

// taperedWing is a swept, tapered, dihedralled wing, so that every construction
// line and every angle is non-degenerate and a view that quietly dropped one
// would be visible.
func taperedWing(t *testing.T) calculator.Wing {
	t.Helper()
	def := baseWing(t)
	def.Drivers.Shape = calculator.ShapeTrapezoid
	def.Drivers.TaperRatio = 0.5
	def.Sweep = mustQ(t, 10, calculator.Degree)
	def.Dihedral = mustQ(t, 8, calculator.Degree)
	def.Twist = mustQ(t, -3, calculator.Degree)
	def.Incidence = mustQ(t, 2, calculator.Degree)
	def.DihedralMode = calculator.DihedralHoldProjected
	def.BodyWidth = mustQ(t, 0.12, calculator.Meter)
	w, err := calculator.SolveWing(def)
	if err != nil {
		t.Fatalf("SolveWing: %v", err)
	}
	return w
}

// The front view is where the two planes stop coinciding, and it must show both
// so a builder cutting a panel does not cut it to a projected length.
func TestTheFrontViewDistinguishesProjectedFromPanelDimensions(t *testing.T) {
	wing := taperedWing(t)
	var front calculator.SketchView
	for _, view := range wing.Views() {
		if view.View == calculator.ViewFront {
			front = view
		}
	}
	if front.View != calculator.ViewFront {
		t.Fatal("no front view")
	}
	projected := frontDimension(t, front, calculator.ParamSemiSpanProjected, calculator.OutlinePlanView)
	panel := frontDimension(t, front, calculator.ParamSemiSpanPanel, calculator.OutlinePanelSurface)
	if projected.SI() >= panel.SI() {
		t.Errorf("projected %v is not shorter than the panel length %v under dihedral",
			projected.SI(), panel.SI())
	}
	wantSI(t, "the panel half span", panel,
		projected.SI()/math.Cos(wing.Definition.Dihedral.SI()))
}

// frontDimension returns the named dimension's value and checks it is marked as
// being in the plane it belongs to.
func frontDimension(t *testing.T, view calculator.SketchView, key calculator.ParameterKey,
	plane calculator.OutlinePlane,
) calculator.Quantity {
	t.Helper()
	for n := range view.Dimensions {
		dimension := &view.Dimensions[n]
		if dimension.Key != key {
			continue
		}
		if dimension.Plane != plane {
			t.Errorf("%s is marked as %v, want %v", key, dimension.Plane, plane)
		}
		return dimension.Value
	}
	t.Fatalf("the front view does not dimension %s", key)
	return calculator.Quantity{}
}

// A sketch that does not show what its dimensions are measured from is not a
// sketch anyone can rebuild from.
func TestEveryViewStatesItsDatumItsAxesAndItsOrigin(t *testing.T) {
	for _, view := range taperedWing(t).Views() {
		if view.Datum != calculator.DatumWingRoot {
			t.Errorf("%v does not state the datum", view.View)
		}
		across, up := view.View.Axes()
		if view.Across != across || view.Up != up || across == "" {
			t.Errorf("%v names its axes as %q/%q", view.View, view.Across, view.Up)
		}
		axes := 0
		for _, curve := range view.Curves {
			if curve.Role == calculator.SketchAxis {
				axes++
			}
			if len(curve.Points) < 2 {
				t.Errorf("%v curve %q has fewer than two points", view.View, curve.Label)
			}
			if curve.Label == "" {
				t.Errorf("%v has an unlabelled curve", view.View)
			}
		}
		if axes == 0 {
			t.Errorf("%v draws no datum axis through the sketch origin", view.View)
		}
	}
}

// The plan view draws the symmetry plane and says the left panel is a mirror,
// rather than emitting a second panel that could drift from the first.
func TestThePlanViewDrawsTheCenterlineAndStatesTheMirror(t *testing.T) {
	var plan calculator.SketchView
	for _, view := range taperedWing(t).Views() {
		if view.View == calculator.ViewPlan {
			plan = view
		}
	}
	centerline, outline := false, false
	for _, curve := range plan.Curves {
		if curve.Role == calculator.SketchCenterline {
			centerline = true
		}
		if curve.Role == calculator.SketchOutline {
			outline = true
			if !curve.Mirrored {
				t.Error("the plan outline does not say the left panel is its mirror")
			}
			if !curve.Closed {
				t.Error("the plan outline is not closed")
			}
			if len(curve.Points) != 4 {
				t.Errorf("the plan outline has %d corners, want 4", len(curve.Points))
			}
		}
	}
	if !centerline || !outline {
		t.Error("the plan view is missing its centerline or its outline")
	}
}

// The mean aerodynamic chord is drawn as a geometric reference and labelled as
// one. It is not a centre of lift and nothing here says it is.
func TestTheMACIsDrawnAsAGeometricReference(t *testing.T) {
	var plan calculator.SketchView
	for _, view := range taperedWing(t).Views() {
		if view.View == calculator.ViewPlan {
			plan = view
		}
	}
	found := false
	for _, curve := range plan.Curves {
		if curve.Role == calculator.SketchReference && containsAll(curve.Label, "mean aerodynamic chord") {
			found = true
			if !containsAll(curve.Label, "geometric reference") {
				t.Errorf("the MAC curve is labelled %q; it must say it is a geometric reference", curve.Label)
			}
		}
	}
	if !found {
		t.Fatal("the plan view does not draw the mean aerodynamic chord")
	}
	for _, dimension := range plan.Dimensions {
		if dimension.Key == calculator.ParamChordMAC && !containsAll(dimension.Detail, "not a centre of lift") {
			t.Errorf("the MAC dimension detail is %q; it must refuse the centre-of-lift reading",
				dimension.Detail)
		}
	}
}

// Selecting a dimension shows the relationship, the substitutions and the
// result the core supplied. Nothing in a frontend recomputes any of it.
func TestExplainingADerivedParameterGivesItsFormulaAndSubstitutions(t *testing.T) {
	wing := taperedWing(t)
	explanation, ok := wing.Explain(calculator.ParamChordRoot)
	if !ok {
		t.Fatal("the root chord has no explanation")
	}
	if explanation.Role != calculator.RoleDerived {
		t.Fatalf("role = %v, want derived: this design drives span and aspect ratio", explanation.Role)
	}
	if explanation.EquationID != calculator.EqRootChord {
		t.Errorf("equation = %q, want %q", explanation.EquationID, calculator.EqRootChord)
	}
	if explanation.Expression == "" || explanation.Revision == "" {
		t.Error("the explanation carries no expression or no revision")
	}
	if len(explanation.Substitutions) == 0 {
		t.Fatal("no substitutions were recorded")
	}
	// The substituted values are the ones the equation actually consumed, so
	// they must reproduce the result.
	parameters := wing.Parameters()
	area, _ := parameters.Get(calculator.ParamAreaReference)
	span, _ := parameters.Get(calculator.ParamSpanProjected)
	substituted := map[string]float64{}
	for _, substitution := range explanation.Substitutions {
		substituted[substitution.Name] = substitution.Value.SI()
	}
	if !sameWithin(substituted["wing_area"], area.Value.SI()) {
		t.Errorf("substituted area %v, the parameter set holds %v",
			substituted["wing_area"], area.Value.SI())
	}
	if !sameWithin(substituted["span"], span.Value.SI()) {
		t.Errorf("substituted span %v, the parameter set holds %v",
			substituted["span"], span.Value.SI())
	}
	if explanation.Value != mustParameterValue(t, parameters, calculator.ParamChordRoot) {
		t.Error("the explanation and the parameter set disagree about the value")
	}
}

func mustParameterValue(t *testing.T, ps calculator.ParameterSet, key calculator.ParameterKey) calculator.Quantity {
	t.Helper()
	p, ok := ps.Get(key)
	if !ok {
		t.Fatalf("no parameter %s", key)
	}
	return p.Value
}

// A driver explains itself as entered. It has no formula, and inventing one for
// it would present a builder's own choice as a derivation.
func TestADriverExplainsItselfAsEntered(t *testing.T) {
	explanation, ok := taperedWing(t).Explain(calculator.ParamSpanProjected)
	if !ok {
		t.Fatal("the span has no explanation")
	}
	if explanation.Role != calculator.RoleDriver {
		t.Fatalf("role = %v, want driver", explanation.Role)
	}
	if explanation.Expression != "" || len(explanation.Substitutions) != 0 {
		t.Error("a driver was given a formula")
	}
	if !containsAll(explanation.Detail, "entered by the builder") {
		t.Errorf("detail = %q", explanation.Detail)
	}
}

// A driver swap changes which values are derived and therefore which formulas
// exist. The keys do not move; the roles and the edges do.
func TestADriverSwapChangesTheRolesAndTheFormulas(t *testing.T) {
	before := taperedWing(t)
	beforeSpan, _ := before.Explain(calculator.ParamSpanProjected)
	if beforeSpan.Role != calculator.RoleDriver {
		t.Fatal("the span is not a driver to begin with")
	}

	def := before.Definition
	def.Drivers.Span = calculator.Quantity{}
	def.Drivers.Area = mustQ(t, 0.24, calculator.SquareMeter)
	after, err := calculator.SolveWing(def)
	if err != nil {
		t.Fatalf("SolveWing: %v", err)
	}
	afterSpan, ok := after.Explain(calculator.ParamSpanProjected)
	if !ok {
		t.Fatal("the span vanished from the parameter set after the swap")
	}
	if afterSpan.Role != calculator.RoleDerived {
		t.Fatalf("role = %v, want derived after releasing the span", afterSpan.Role)
	}
	if afterSpan.EquationID != calculator.EqSpanFromAreaAndAspect {
		t.Errorf("equation = %q, want the area-and-aspect-ratio path", afterSpan.EquationID)
	}
	if len(afterSpan.DependsOn) == 0 {
		t.Error("the derived span depends on nothing")
	}
	// And the dimension on the drawing follows the swap, because the dimension
	// carries the parameter's value rather than a copy of it.
	for _, view := range after.Views() {
		for _, dimension := range view.Dimensions {
			if dimension.Key == calculator.ParamSpanProjected && dimension.Value != afterSpan.Value {
				t.Error("the drawing kept the released driver's value")
			}
		}
	}
}

// Every parameter is explicable, so a formula view can be built from one call
// without a per-key special case that could quietly omit one.
func TestEveryParameterHasAnExplanation(t *testing.T) {
	wing := taperedWing(t)
	explanations := wing.Explanations()
	parameters := wing.Parameters()
	if len(explanations) != len(parameters) {
		t.Fatalf("%d explanations for %d parameters", len(explanations), len(parameters))
	}
	for n, explanation := range explanations {
		if explanation.Key != parameters[n].Key {
			t.Fatalf("explanation %d is for %s, parameter %d is %s",
				n, explanation.Key, n, parameters[n].Key)
		}
		if explanation.Detail == "" {
			t.Errorf("%s explains nothing", explanation.Key)
		}
		if explanation.Role == calculator.RoleDerived && explanation.Expression == "" {
			t.Errorf("%s is derived but carries no expression", explanation.Key)
		}
	}
}

// The relationships the task quotes as an example are the ones the wing
// actually reports, so the worksheet's explanation is the core's.
func TestTheTaskExampleRelationshipsAreTheReportedOnes(t *testing.T) {
	wing := taperedWing(t)
	for _, tc := range []struct {
		key  calculator.ParameterKey
		want string
	}{
		{calculator.ParamSemiSpanProjected, "b_half = b / 2"},
		{calculator.ParamChordRoot, "c_root = 2 * S / (b * (1 + lambda))"},
		{calculator.ParamChordTip, "c_tip = lambda * c_root"},
	} {
		explanation, ok := wing.Explain(tc.key)
		if !ok {
			t.Fatalf("no explanation for %s", tc.key)
		}
		if explanation.Expression != tc.want {
			t.Errorf("%s reads %q, want %q", tc.key, explanation.Expression, tc.want)
		}
	}
}

// An unknown key is answered as unknown rather than as an empty explanation a
// caller could mistake for a real one.
func TestAnUnknownParameterHasNoExplanation(t *testing.T) {
	if _, ok := solvedFixtureWing(t).Explain("wing.nonsense"); ok {
		t.Error("an unknown key was explained")
	}
}
