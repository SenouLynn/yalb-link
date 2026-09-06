package calculator_test

import (
	"testing"

	"yalb.aero/calculator"
)

// spanFirstWing and areaFirstWing are the same aircraft entered from two entry
// points: one builder starts from the span they can transport, the other from
// an area and aspect ratio. The parameter keys must not change; the roles and
// the dependency edges must.
func TestSolveModeChangesRolesNotKeys(t *testing.T) {
	areaFirst, err := calculator.SolveWing(bookWingDefinition(t))
	if err != nil {
		t.Fatalf("area first: %v", err)
	}
	spanFirstDef := bookWingDefinition(t)
	spanFirstDef.Drivers = calculator.PlanformDrivers{
		Shape:      calculator.ShapeTrapezoid,
		TaperRatio: 0.4,
		Span:       areaFirst.Planform.Span,
		Area:       areaFirst.Planform.Area,
	}
	spanFirst, err := calculator.SolveWing(spanFirstDef)
	if err != nil {
		t.Fatalf("span first: %v", err)
	}

	areaParams := areaFirst.Parameters()
	spanParams := spanFirst.Parameters()
	if len(areaParams) != len(spanParams) {
		t.Fatalf("the same wing exported %d and %d parameters", len(areaParams), len(spanParams))
	}
	for _, p := range areaParams {
		if _, ok := spanParams.Get(p.Key); !ok {
			t.Errorf("key %q disappeared when the solve mode changed", p.Key)
		}
	}

	// Area and aspect ratio are the drivers on one side; span and area on the
	// other. The span cannot be a driver in the first and the aspect ratio
	// cannot be a driver in the second.
	assertRole(t, areaParams, calculator.ParamAreaReference, calculator.RoleDriver)
	assertRole(t, areaParams, calculator.ParamAspectRatio, calculator.RoleDriver)
	assertRole(t, areaParams, calculator.ParamSpanProjected, calculator.RoleDerived)
	assertRole(t, spanParams, calculator.ParamSpanProjected, calculator.RoleDriver)
	assertRole(t, spanParams, calculator.ParamAreaReference, calculator.RoleDriver)
	assertRole(t, spanParams, calculator.ParamAspectRatio, calculator.RoleDerived)

	// Both graphs must be evaluable, with every dependency ahead of its user.
	assertEvaluableGraph(t, "area first", areaParams)
	assertEvaluableGraph(t, "span first", spanParams)
}

// assertEvaluableGraph checks that the set orders, covers every parameter, and
// places each dependency ahead of the value that uses it.
func assertEvaluableGraph(t *testing.T, name string, params calculator.ParameterSet) {
	t.Helper()
	order, err := params.EvaluationOrder()
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if len(order) != len(params) {
		t.Errorf("%s: ordered %d of %d parameters", name, len(order), len(params))
	}
	position := make(map[calculator.ParameterKey]int, len(order))
	for i, key := range order {
		position[key] = i
	}
	for _, p := range params {
		for _, dep := range p.DependsOn {
			if position[dep] >= position[p.Key] {
				t.Errorf("%s: %q is evaluated before its dependency %q", name, p.Key, dep)
			}
		}
	}
}

func assertRole(t *testing.T, ps calculator.ParameterSet, key calculator.ParameterKey,
	want calculator.ParameterRole,
) {
	t.Helper()
	p, ok := ps.Get(key)
	if !ok {
		t.Fatalf("parameter %q is missing", key)
	}
	if p.Role != want {
		t.Errorf("%q is %v, want %v", key, p.Role, want)
	}
}

// Every parameter must carry enough to reproduce it: a unit, a datum for a
// coordinate, and for a derived value the equation and revision behind it.
func TestParametersCarryUnitsDatumsAndEquations(t *testing.T) {
	w, err := calculator.SolveWing(bookWingDefinition(t))
	if err != nil {
		t.Fatalf("SolveWing: %v", err)
	}
	params := w.Parameters()
	for _, p := range params {
		assertParameterShape(t, p)
	}

	// A coordinate without its datum is not a coordinate.
	for _, key := range []calculator.ParameterKey{
		calculator.ParamStationMAC, calculator.ParamTipLEOffset, calculator.ParamTipRise,
	} {
		p, ok := params.Get(key)
		if !ok {
			t.Fatalf("%q is missing", key)
		}
		if p.Datum != calculator.DatumWingRoot {
			t.Errorf("%q carries no datum", key)
		}
	}
	// A length is not a position, and must not claim a datum it does not need.
	if p, _ := params.Get(calculator.ParamChordMAC); p.Datum != "" {
		t.Errorf("the MAC is a length, not a station, but carries datum %q", p.Datum)
	}

	// Driver values are the builder's own numbers, unchanged.
	area, _ := params.Get(calculator.ParamAreaReference)
	if area.Value != w.Definition.Drivers.Area {
		t.Errorf("driver area exported as %v, supplied %v", area.Value, w.Definition.Drivers.Area)
	}
}

// assertParameterShape checks one parameter's unit and its role's obligations:
// a driver is the builder's own number and claims no equation, a derived value
// names the equation revision it came from and what it came from.
func assertParameterShape(t *testing.T, p calculator.Parameter) {
	t.Helper()
	if p.Unit == calculator.UnitInvalid {
		t.Errorf("%q has no unit", p.Key)
	}
	if p.Unit != p.Value.Dimension().SIUnit() {
		t.Errorf("%q states unit %v for a %v value", p.Key, p.Unit.Symbol(), p.Value.Dimension())
	}
	switch p.Role {
	case calculator.RoleDriver:
		if p.EquationID != "" || len(p.DependsOn) != 0 {
			t.Errorf("driver %q claims to be derived from %q", p.Key, p.EquationID)
		}
	case calculator.RoleDerived:
		eq, err := calculator.Lookup(p.EquationID)
		if err != nil {
			t.Errorf("%q names unregistered equation %q", p.Key, p.EquationID)
			return
		}
		if p.Revision != eq.Revision {
			t.Errorf("%q records revision %q, the registry has %q", p.Key, p.Revision, eq.Revision)
		}
		if len(p.DependsOn) == 0 {
			t.Errorf("derived %q depends on nothing", p.Key)
		}
	case calculator.RoleUnknown:
		t.Errorf("%q has no role", p.Key)
	}
}

// The plan view and the built panel are different parameters, so a CAD or
// analysis consumer never has to guess which one a number was.
func TestProjectedAndPanelParametersStaySeparate(t *testing.T) {
	def := bookWingDefinition(t)
	def.DihedralMode = calculator.DihedralHoldPanel
	w, err := calculator.SolveWing(def)
	if err != nil {
		t.Fatalf("SolveWing: %v", err)
	}
	params := w.Parameters()
	// These drivers are an area and an aspect ratio in the panel plane, so the
	// plan-view area is what gets derived from them.
	assertRole(t, params, calculator.ParamAreaPanel, calculator.RoleDriver)
	assertRole(t, params, calculator.ParamAreaReference, calculator.RoleDerived)
	assertRole(t, params, calculator.ParamSpanPanel, calculator.RoleDerived)
	assertRole(t, params, calculator.ParamSpanProjected, calculator.RoleDerived)
	panel, _ := params.Get(calculator.ParamSpanPanel)
	projected, _ := params.Get(calculator.ParamSpanProjected)
	if panel.Value.SI() <= projected.Value.SI() {
		t.Errorf("under dihedral the panel span %v must exceed its projection %v",
			panel.Value, projected.Value)
	}
	// The aerodynamic aspect ratio follows the plan view, not the panel.
	planform, _ := params.Get(calculator.ParamAspectRatio)
	aero, _ := params.Get(calculator.ParamAspectRatioProjected)
	if planform.Value.SI() == aero.Value.SI() {
		t.Error("under dihedral the planform and projected aspect ratios must differ")
	}
	if !(tol{rel: floatNoise}).ok(aero.Value.SI(), w.ProjectedAspectRatio) {
		t.Errorf("exported projected aspect ratio %v, solved %v", aero.Value, w.ProjectedAspectRatio)
	}

	// A sketch is usually driven by the half span, and the half a builder cuts
	// is not the half that shows up in the plan view.
	halfProjected, _ := params.Get(calculator.ParamSemiSpanProjected)
	halfPanel, _ := params.Get(calculator.ParamSemiSpanPanel)
	exact := tol{rel: floatNoise}
	if !exact.ok(2*halfProjected.Value.SI(), projected.Value.SI()) {
		t.Errorf("projected half span %v is not half of %v", halfProjected.Value, projected.Value)
	}
	if !exact.ok(2*halfPanel.Value.SI(), panel.Value.SI()) {
		t.Errorf("panel half span %v is not half of %v", halfPanel.Value, panel.Value)
	}
	if halfPanel.Value.SI() <= halfProjected.Value.SI() {
		t.Error("under dihedral the built half panel must be longer than its projection")
	}
}

// The acyclic check has to be a real check. A hand-built cyclic set must be
// rejected, and so must a dependency on a key that is not in the set.
func TestDependencyGraphRejectsCyclesAndDanglingEdges(t *testing.T) {
	cyclic := calculator.ParameterSet{
		{Key: calculator.ParamSpanProjected, Role: calculator.RoleDerived,
			DependsOn: []calculator.ParameterKey{calculator.ParamAreaReference}},
		{Key: calculator.ParamAreaReference, Role: calculator.RoleDerived,
			DependsOn: []calculator.ParameterKey{calculator.ParamSpanProjected}},
	}
	err := errorFrom(cyclic.EvaluationOrder())
	if err == nil {
		t.Fatal("a cycle must not be ordered")
	}
	detail := wantIssue(t, err, "parameters", calculator.IssueInvalid)
	if !containsText(detail, string(calculator.ParamSpanProjected)) {
		t.Errorf("a cycle report should name the keys involved, got %q", detail)
	}

	dangling := calculator.ParameterSet{
		{Key: calculator.ParamChordTip, Role: calculator.RoleDerived,
			DependsOn: []calculator.ParameterKey{calculator.ParamChordRoot}},
	}
	err = errorFrom(dangling.EvaluationOrder())
	if err == nil {
		t.Fatal("a dependency outside the set must be reported")
	}
	wantIssue(t, err, string(calculator.ParamChordTip), calculator.IssueMissing)

	duplicated := calculator.ParameterSet{
		{Key: calculator.ParamChordRoot, Role: calculator.RoleDriver},
		{Key: calculator.ParamChordRoot, Role: calculator.RoleDriver},
	}
	if err := errorFrom(duplicated.EvaluationOrder()); err == nil {
		t.Error("a key defined twice must be rejected")
	}
}
