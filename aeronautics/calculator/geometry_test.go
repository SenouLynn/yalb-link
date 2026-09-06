package calculator_test

import (
	"math"
	"testing"

	"yalb.aero/calculator"
)

// bookWing is the Wing Planform Sizing chapter's worked example: A = 8,
// S = 134 ft^2, lambda = 0.4, quarter-chord sweep 0. Its aircraft is a manned
// light aircraft in customary units; the values are used here to check the
// ported relations, not as RC defaults.
func bookWing(t *testing.T) calculator.PlanformDrivers {
	t.Helper()
	return calculator.PlanformDrivers{
		Shape:       calculator.ShapeTrapezoid,
		Area:        mustQ(t, 134, calculator.SquareFoot),
		AspectRatio: 8,
		TaperRatio:  0.4,
	}
}

// Expected values were computed independently at 45 significant digits from the
// chapter's relations, not read back from its displayed output. See
// testdata/wing-planform-book-example.md.
func TestBookWingPlanformExample(t *testing.T) {
	p, err := calculator.SolvePlanform(bookWing(t))
	if err != nil {
		t.Fatalf("SolvePlanform: %v", err)
	}
	// Absolute terms are in feet, from the precision the chapter displays; the
	// relative term absorbs float64 rounding in the evaluation.
	displayed := tol{abs: 0.05, rel: 1e-12}
	independent := tol{abs: 1e-12, rel: 1e-12}
	for _, c := range []struct {
		name       string
		got        float64
		bookShows  float64
		calculated float64
	}{
		{"span", inUnit(t, p.Span, calculator.Foot), 33, 32.74141108748979987981481},
		{"root chord", inUnit(t, p.RootChord, calculator.Foot), 5.8, 5.846680551337464264252646},
		{"tip chord", inUnit(t, p.TipChord, calculator.Foot), 2.3, 2.338672220534985705701058},
		{"MAC", inUnit(t, p.MAC, calculator.Foot), 4.3, 4.343248409564973453444822},
		{"y_MAC", inUnit(t, p.YMAC, calculator.Foot), 7.0, 7.016016661604957117103175},
	} {
		t.Run(c.name, func(t *testing.T) {
			if !independent.ok(c.got, c.calculated) {
				t.Errorf("%s = %.15g ft, independently calculated %.15g ft", c.name, c.got, c.calculated)
			}
			// The chapter's own displayed value, at the precision it shows. The
			// span is displayed as 33 ft, so this check is deliberately loose;
			// the independent check above is the tight one.
			show := displayed
			if c.name == "span" {
				show = tol{abs: 0.5, rel: 0}
			}
			if !show.ok(c.got, c.bookShows) {
				t.Errorf("%s = %.15g ft, chapter displays %v ft", c.name, c.got, c.bookShows)
			}
		})
	}
	if p.Mode != calculator.SolveFromAreaAndAspectRatio {
		t.Errorf("solve mode = %v, want the area and aspect ratio path", p.Mode)
	}
}

// The chapter's example is in customary units. Solving it in SI must give the
// same wing, and must not go through the chapter's rounded outputs.
func TestOriginalUnitsAndSIAgree(t *testing.T) {
	customary, err := calculator.SolvePlanform(bookWing(t))
	if err != nil {
		t.Fatalf("customary: %v", err)
	}
	si := bookWing(t)
	// 134 ft^2 in m^2, exactly: 0.3048^2 is exact in binary? It is not, so this
	// conversion is done by the unit table itself rather than by a literal.
	areaSI := inUnit(t, customary.Area, calculator.SquareMeter)
	si.Area = mustQ(t, areaSI, calculator.SquareMeter)
	metric, err := calculator.SolvePlanform(si)
	if err != nil {
		t.Fatalf("SI: %v", err)
	}
	agree := tol{abs: 0, rel: floatNoise}
	for _, c := range []struct {
		name string
		a, b calculator.Quantity
	}{
		{"span", customary.Span, metric.Span},
		{"root chord", customary.RootChord, metric.RootChord},
		{"tip chord", customary.TipChord, metric.TipChord},
		{"MAC", customary.MAC, metric.MAC},
		{"y_MAC", customary.YMAC, metric.YMAC},
	} {
		if !agree.ok(c.a.SI(), c.b.SI()) {
			t.Errorf("%s differs between original units and SI: %v vs %v", c.name, c.a, c.b)
		}
	}
}

// The rectangle is the taper ratio 1 limit of the trapezoid, and the two must
// agree. Under taper they must not: MAC is not S/b.
func TestRectangleIsTheTaperOneLimit(t *testing.T) {
	span := mustQ(t, 1.2, calculator.Meter)
	area := mustQ(t, 0.24, calculator.SquareMeter)
	rect, err := calculator.SolvePlanform(calculator.PlanformDrivers{
		Shape: calculator.ShapeRectangle, Span: span, Area: area,
	})
	if err != nil {
		t.Fatalf("rectangle: %v", err)
	}
	limit, err := calculator.SolvePlanform(calculator.PlanformDrivers{
		Shape: calculator.ShapeTrapezoid, Span: span, Area: area, TaperRatio: 1,
	})
	if err != nil {
		t.Fatalf("trapezoid at lambda = 1: %v", err)
	}
	agree := tol{rel: floatNoise}
	for _, c := range []struct {
		name string
		a, b calculator.Quantity
	}{
		{"root chord", rect.RootChord, limit.RootChord},
		{"tip chord", rect.TipChord, limit.TipChord},
		{"MAC", rect.MAC, limit.MAC},
		{"mean chord", rect.MeanChord, limit.MeanChord},
		{"y_MAC", rect.YMAC, limit.YMAC},
	} {
		if !agree.ok(c.a.SI(), c.b.SI()) {
			t.Errorf("%s: rectangle %v, trapezoid at lambda=1 %v", c.name, c.a, c.b)
		}
	}
	// On a rectangle every chord length is the same length.
	if !agree.ok(rect.MAC.SI(), rect.MeanChord.SI()) {
		t.Errorf("rectangle MAC %v and mean chord %v must agree", rect.MAC, rect.MeanChord)
	}
}

func TestMACDiffersFromMeanChordUnderTaper(t *testing.T) {
	p, err := calculator.SolvePlanform(bookWing(t))
	if err != nil {
		t.Fatalf("SolvePlanform: %v", err)
	}
	mac := inUnit(t, p.MAC, calculator.Foot)
	mean := inUnit(t, p.MeanChord, calculator.Foot)
	if mac == mean {
		t.Fatal("MAC and the geometric mean chord must be different lengths under taper")
	}
	// Independently calculated: MAC 4.343248409564973 ft, S/b 4.092676385936225 ft.
	independent := tol{abs: 1e-12, rel: 1e-12}
	if !independent.ok(mean, 4.092676385936224984976852) {
		t.Errorf("mean chord = %.15g ft, want 4.09267638593622 ft", mean)
	}
	if mac <= mean {
		t.Errorf("under taper the MAC (%.6g ft) is the longer reference length, not %.6g ft", mac, mean)
	}
}

// Every supported driver pair must describe the same wing. Solving the book
// example, then feeding its own outputs back in as each other pair, is what
// would catch a wrong rearrangement in one path only.
func TestEverySolvePathAgrees(t *testing.T) {
	reference, err := calculator.SolvePlanform(bookWing(t))
	if err != nil {
		t.Fatalf("reference: %v", err)
	}
	base := calculator.PlanformDrivers{Shape: calculator.ShapeTrapezoid, TaperRatio: 0.4}
	paths := []struct {
		mode    calculator.SolveMode
		drivers calculator.PlanformDrivers
	}{
		{calculator.SolveFromSpanAndArea, calculator.PlanformDrivers{
			Span: reference.Span, Area: reference.Area}},
		{calculator.SolveFromSpanAndAspectRatio, calculator.PlanformDrivers{
			Span: reference.Span, AspectRatio: reference.AspectRatio}},
		{calculator.SolveFromAreaAndAspectRatio, calculator.PlanformDrivers{
			Area: reference.Area, AspectRatio: reference.AspectRatio}},
		{calculator.SolveFromSpanAndRootChord, calculator.PlanformDrivers{
			Span: reference.Span, RootChord: reference.RootChord}},
		{calculator.SolveFromAreaAndRootChord, calculator.PlanformDrivers{
			Area: reference.Area, RootChord: reference.RootChord}},
		{calculator.SolveFromAspectRatioAndRootChord, calculator.PlanformDrivers{
			AspectRatio: reference.AspectRatio, RootChord: reference.RootChord}},
	}
	agree := tol{rel: 1e-9}
	for _, path := range paths {
		t.Run(path.mode.String(), func(t *testing.T) {
			drivers := base
			drivers.Span = path.drivers.Span
			drivers.Area = path.drivers.Area
			drivers.RootChord = path.drivers.RootChord
			drivers.AspectRatio = path.drivers.AspectRatio
			got, err := calculator.SolvePlanform(drivers)
			if err != nil {
				t.Fatalf("SolvePlanform: %v", err)
			}
			if got.Mode != path.mode {
				t.Errorf("mode = %v, want %v", got.Mode, path.mode)
			}
			for _, c := range []struct {
				name string
				a, b calculator.Quantity
			}{
				{"span", got.Span, reference.Span},
				{"area", got.Area, reference.Area},
				{"root chord", got.RootChord, reference.RootChord},
				{"tip chord", got.TipChord, reference.TipChord},
				{"MAC", got.MAC, reference.MAC},
				{"y_MAC", got.YMAC, reference.YMAC},
			} {
				if !agree.ok(c.a.SI(), c.b.SI()) {
					t.Errorf("%s = %v, reference %v", c.name, c.a, c.b)
				}
			}
			if !agree.ok(got.AspectRatio, reference.AspectRatio) {
				t.Errorf("aspect ratio = %v, reference %v", got.AspectRatio, reference.AspectRatio)
			}
		})
	}
}

// An over-determined driver set whose values agree is redundant, not wrong. One
// whose values disagree is a conflict, and names the field that disagrees. The
// two must not be reported the same way.
func TestRedundantAndConflictingDriversAreDistinguishable(t *testing.T) {
	solved, err := calculator.SolvePlanform(bookWing(t))
	if err != nil {
		t.Fatalf("SolvePlanform: %v", err)
	}
	redundant := calculator.PlanformDrivers{
		Shape:       calculator.ShapeTrapezoid,
		TaperRatio:  0.4,
		Area:        solved.Area,
		AspectRatio: solved.AspectRatio,
		Span:        solved.Span,
	}
	redundantErr := errorFrom(calculator.SolvePlanform(redundant))
	if redundantErr == nil {
		t.Error("an over-determined driver set must not be solved silently")
	} else {
		redundantDetail := wantIssue(t, redundantErr, "drivers", calculator.IssueUnsupported)
		if !containsText(redundantDetail, "consistent") {
			t.Errorf("a redundant set must be reported as consistent, got %q", redundantDetail)
		}
		if issues, _ := calculator.AsIssues(redundantErr); issues.Kind(calculator.IssueInvalid) {
			t.Error("consistent values must not be reported as invalid")
		}
	}

	conflicting := redundant
	conflicting.Span = mustQ(t, 20, calculator.Foot)
	err = errorFrom(calculator.SolvePlanform(conflicting))
	if err == nil {
		t.Fatal("conflicting drivers must not be solved")
	}
	detail := wantIssue(t, err, "drivers", calculator.IssueInvalid)
	if !containsText(detail, "span") || !containsText(detail, "reference pair") {
		t.Errorf("a conflict must name the disagreement and the pair it was measured against, got %q", detail)
	}
	if issues, _ := calculator.AsIssues(err); issues.Kind(calculator.IssueUnsupported) {
		t.Error("a real conflict must not be reported as merely redundant")
	}
}

// A limit is not a driver. A design that states only "span at most 1.4 m" has
// no span until the builder chooses one, and the solver must say so rather than
// quietly building at the boundary.
func TestSpanLimitIsNotASpanDriver(t *testing.T) {
	limits := calculator.PlanformLimits{MaximumSpan: mustQ(t, 1.4, calculator.Meter)}
	err := errorFrom(calculator.SolvePlanform(calculator.PlanformDrivers{
		Shape:      calculator.ShapeRectangle,
		RootChord:  mustQ(t, 0.2, calculator.Meter),
		TaperRatio: 1,
	}))
	if err == nil {
		t.Fatal("one driver plus a limit is not two drivers")
	}
	wantIssue(t, err, "drivers", calculator.IssueMissing)

	// Choosing to use the whole allowance is a decision, and once made it is an
	// ordinary driver.
	chosen, err := calculator.SolvePlanform(calculator.PlanformDrivers{
		Shape:      calculator.ShapeRectangle,
		Span:       limits.MaximumSpan,
		RootChord:  mustQ(t, 0.2, calculator.Meter),
		TaperRatio: 1,
	})
	if err != nil {
		t.Fatalf("choosing the limit as the span: %v", err)
	}
	checks := limits.Check(chosen)
	if len(checks) != 1 || checks[0].Status != calculator.LimitMet {
		t.Errorf("span at the limit must be met, got %+v", checks)
	}
	over := limits
	over.MaximumSpan = mustQ(t, 1, calculator.Meter)
	if checks := over.Check(chosen); len(checks) != 1 || checks[0].Status != calculator.LimitUnmet {
		t.Errorf("a 1.2 m span against a 1 m limit is unmet, got %+v", checks)
	}
	if checks := over.Check(calculator.Planform{}); len(checks) != 1 ||
		checks[0].Status != calculator.LimitUnknown {
		t.Error("a limit on an unsolved planform is unknown, not unmet")
	}
}

// Unusual is not the same as unsupported, and unsupported is not the same as
// impossible.
func TestUnusualGeometryIsAcceptedAndUnsupportedGeometryIsNamed(t *testing.T) {
	reverse, err := calculator.SolvePlanform(calculator.PlanformDrivers{
		Shape:       calculator.ShapeTrapezoid,
		Area:        mustQ(t, 0.24, calculator.SquareMeter),
		AspectRatio: 6,
		TaperRatio:  1.6,
	})
	if err != nil {
		t.Fatalf("reverse taper is a valid planform: %v", err)
	}
	if reverse.TipChord.SI() <= reverse.RootChord.SI() {
		t.Error("reverse taper must give a tip chord longer than the root chord")
	}

	err = errorFrom(calculator.SolvePlanform(calculator.PlanformDrivers{
		Shape:       calculator.ShapeTrapezoid,
		Area:        mustQ(t, 0.24, calculator.SquareMeter),
		AspectRatio: 6,
	}))
	detail := wantIssue(t, err, "taper_ratio", calculator.IssueMissing)
	if !containsText(detail, "pointed tip") {
		t.Errorf("a missing taper ratio should say what a pointed tip would mean, got %q", detail)
	}

	err = errorFrom(calculator.SolvePlanform(calculator.PlanformDrivers{
		Area:        mustQ(t, 0.24, calculator.SquareMeter),
		AspectRatio: 6,
		TaperRatio:  0.5,
	}))
	wantIssue(t, err, "shape", calculator.IssueMissing)

	err = errorFrom(calculator.SolvePlanform(calculator.PlanformDrivers{
		Shape:       calculator.ShapeTrapezoid,
		Area:        mustQ(t, 0.24, calculator.SquareMeter),
		AspectRatio: 6,
		TaperRatio:  -0.5,
	}))
	wantIssue(t, err, "taper_ratio", calculator.IssueInvalid)
}

func TestChordVariesLinearlyAcrossTheSemiSpan(t *testing.T) {
	ev := eval{t}
	p, err := calculator.SolvePlanform(bookWing(t))
	if err != nil {
		t.Fatalf("SolvePlanform: %v", err)
	}
	agree := tol{rel: floatNoise}
	root := ev.ok(p.ChordAt(mustQ(t, 0, calculator.Meter)))
	if !agree.ok(root.Value.SI(), p.RootChord.SI()) {
		t.Errorf("chord at the centerline = %v, want the root chord %v", root.Value, p.RootChord)
	}
	tip := ev.ok(p.ChordAt(p.SemiSpan()))
	if !agree.ok(tip.Value.SI(), p.TipChord.SI()) {
		t.Errorf("chord at the tip = %v, want the tip chord %v", tip.Value, p.TipChord)
	}
	// The MAC is a length, not a station: the chord at y_MAC equals the MAC.
	atYMAC := ev.ok(p.ChordAt(p.YMAC))
	if !agree.ok(atYMAC.Value.SI(), p.MAC.SI()) {
		t.Errorf("chord at y_MAC = %v, want the MAC %v", atYMAC.Value, p.MAC)
	}
	beyond := mustQ(t, p.SemiSpan().SI()*1.1, calculator.Meter)
	wantIssue(t, ev.fails(p.ChordAt(beyond)), "station", calculator.IssueUnsupported)
}

// A solved planform must carry the relationships it used, so a stored result
// can be matched to the model that produced it.
func TestPlanformCarriesItsTraces(t *testing.T) {
	p, err := calculator.SolvePlanform(bookWing(t))
	if err != nil {
		t.Fatalf("SolvePlanform: %v", err)
	}
	want := map[string]bool{
		calculator.EqSpanFromAreaAndAspect: false,
		calculator.EqRootChord:             false,
		calculator.EqTipChord:              false,
		calculator.EqMeanChord:             false,
		calculator.EqMeanAerodynamicChord:  false,
		calculator.EqMACStation:            false,
	}
	for _, trace := range p.Traces {
		if _, ok := want[trace.EquationID]; ok {
			want[trace.EquationID] = true
		}
		eq, err := calculator.Lookup(trace.EquationID)
		if err != nil {
			t.Fatalf("trace names unregistered equation %q", trace.EquationID)
		}
		if trace.Revision != eq.Revision {
			t.Errorf("%s trace revision %q, registry %q", trace.EquationID, trace.Revision, eq.Revision)
		}
		if len(trace.Substitutions) != len(eq.Inputs) {
			t.Errorf("%s recorded %d substitutions for %d declared inputs",
				trace.EquationID, len(trace.Substitutions), len(eq.Inputs))
		}
	}
	for id, seen := range want {
		if !seen {
			t.Errorf("no trace recorded for %s", id)
		}
	}
}

// errorFrom discards a result that must not exist and returns the error.
func errorFrom[T any](_ T, err error) error { return err }

func containsText(s, sub string) bool { return contains(s, sub) }

// A doorway constrains what the wing projects, not the length of its panel
// skin. Under DihedralHoldPanel the solved planform carries construction
// lengths, which exceed their plan-view projections by 1/cos(Gamma), so
// checking a plan-view limit against them reports a wing as too wide when it
// fits. The panel-plane planform must refuse the check, and the wing — which
// holds both planes — must answer it from the projected one.
func TestPlanViewLimitsAreNotCheckedAgainstPanelDimensions(t *testing.T) {
	const dihedralDegrees = 20.0
	panelSpan := 1.4
	projected := panelSpan * math.Cos(dihedralDegrees*math.Pi/180) // 1.3156...

	wing, err := calculator.SolveWing(calculator.WingDefinition{
		Drivers: calculator.PlanformDrivers{
			Shape:      calculator.ShapeRectangle,
			Span:       mustQ(t, panelSpan, calculator.Meter),
			RootChord:  mustQ(t, 0.2, calculator.Meter),
			TaperRatio: 1,
		},
		Dihedral:       mustQ(t, dihedralDegrees, calculator.Degree),
		DihedralMode:   calculator.DihedralHoldPanel,
		Sweep:          mustQ(t, 0, calculator.Degree),
		SweepReference: 0.25,
		Twist:          mustQ(t, 0, calculator.Degree),
		Incidence:      mustQ(t, 0, calculator.Degree),
		AreaBasis:      calculator.AreaBasisReferenceTrapezoid,
	})
	if err != nil {
		t.Fatalf("solving a wing with dihedral: %v", err)
	}

	if got := inUnit(t, wing.Projected.Span, calculator.Meter); !(tol{abs: 1e-12}).ok(got, projected) {
		t.Fatalf("projected span %v, want %v", got, projected)
	}
	if wing.Planform.Plane != calculator.OutlinePanelSurface {
		t.Fatalf("a planform solved from panel drivers under dihedral is a panel-plane form, got %v",
			wing.Planform.Plane)
	}

	// The wing fits a 1.35 m doorway: it projects 1.3156 m. Its panel span does not.
	limits := calculator.PlanformLimits{MaximumSpan: mustQ(t, 1.35, calculator.Meter)}

	checks := wing.CheckLimits(limits)
	if len(checks) != 1 || checks[0].Status != calculator.LimitMet {
		t.Errorf("a wing projecting %.4f m meets a 1.35 m limit, got %+v", projected, checks)
	}

	// The bare planform must not answer from its construction lengths.
	refused := limits.Check(wing.Planform)
	if len(refused) != 1 || refused[0].Status != calculator.LimitUnknown {
		t.Fatalf("a panel-plane planform cannot answer a plan-view limit, got %+v", refused)
	}
	if !containsText(refused[0].Detail, "Wing.CheckLimits") {
		t.Errorf("the refusal must name what can answer it, got %q", refused[0].Detail)
	}

	// At zero dihedral the planes coincide, so the distinction is not demanded.
	flat, err := calculator.SolveWing(calculator.WingDefinition{
		Drivers: calculator.PlanformDrivers{
			Shape:      calculator.ShapeRectangle,
			Span:       mustQ(t, panelSpan, calculator.Meter),
			RootChord:  mustQ(t, 0.2, calculator.Meter),
			TaperRatio: 1,
		},
		Dihedral:       mustQ(t, 0, calculator.Degree),
		DihedralMode:   calculator.DihedralHoldPanel,
		Sweep:          mustQ(t, 0, calculator.Degree),
		SweepReference: 0.25,
		Twist:          mustQ(t, 0, calculator.Degree),
		Incidence:      mustQ(t, 0, calculator.Degree),
		AreaBasis:      calculator.AreaBasisReferenceTrapezoid,
	})
	if err != nil {
		t.Fatalf("solving a flat wing: %v", err)
	}
	if flat.Planform.Plane != calculator.OutlinePlanView {
		t.Errorf("at zero dihedral the planes coincide, got %v", flat.Planform.Plane)
	}
	if checks := limits.Check(flat.Planform); len(checks) != 1 ||
		checks[0].Status != calculator.LimitUnmet {
		t.Errorf("a flat 1.4 m span against a 1.35 m limit is unmet, got %+v", checks)
	}
}
