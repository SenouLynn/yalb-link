package calculator_test

import (
	"math"
	"testing"

	"yalb.aero/calculator"
)

// bookWingDefinition is the chapter's example wing: its planform plus the
// sweep, dihedral, twist, incidence and sections it selects. The 5 ft body
// width comes from the same aircraft in the book's Lift chapter, which quotes
// S_exposed = 106 ft^2 for it.
func bookWingDefinition(t *testing.T) calculator.WingDefinition {
	t.Helper()
	return calculator.WingDefinition{
		Name:           "CODE Lab example wing",
		Drivers:        bookWing(t),
		Sweep:          mustQ(t, 0, calculator.Degree),
		SweepReference: 0.25,
		Dihedral:       mustQ(t, 5, calculator.Degree),
		DihedralMode:   calculator.DihedralHoldProjected,
		Twist:          mustQ(t, -3, calculator.Degree),
		Incidence:      mustQ(t, 2, calculator.Degree),
		BodyWidth:      mustQ(t, 5, calculator.Foot),
		AreaBasis:      calculator.AreaBasisReferenceTrapezoid,
		RootAirfoil: calculator.Airfoil{
			Designation:    "NACA 23018",
			Evidence:       "the chapter's example selection; no polar imported",
			ThicknessRatio: 0.18,
		},
		TipAirfoil: calculator.Airfoil{
			Designation:    "NACA 23009",
			Evidence:       "the chapter's example selection; no polar imported",
			ThicknessRatio: 0.09,
		},
	}
}

// The chapter displays a 3.1 degree leading-edge sweep for a wing whose quarter
// chord is unswept, and the Lift chapter quotes 106 ft^2 exposed for the same
// wing behind a 5 ft body. Both are reproduced here from the drivers, not from
// the chapter's rounded outputs.
func TestBookWingEdgesAndExposedArea(t *testing.T) {
	w, err := calculator.SolveWing(bookWingDefinition(t))
	if err != nil {
		t.Fatalf("SolveWing: %v", err)
	}
	sweepDegrees := inUnit(t, w.LeadingEdgeSweep, calculator.Degree)
	if !(tol{abs: 1e-12, rel: 1e-12}).ok(sweepDegrees, 3.066485501125893377539803) {
		t.Errorf("leading edge sweep = %.15g deg, independently calculated 3.06648550112589 deg",
			sweepDegrees)
	}
	if !(tol{abs: 0.05}).ok(sweepDegrees, 3.1) {
		t.Errorf("leading edge sweep = %.6g deg, chapter displays 3.1 deg", sweepDegrees)
	}
	exposed := inUnit(t, w.ExposedArea, calculator.SquareFoot)
	if !(tol{abs: 1e-10, rel: 1e-12}).ok(exposed, 106.1058829575983929644511) {
		t.Errorf("exposed area = %.15g ft^2, independently calculated 106.105882957598 ft^2", exposed)
	}
	if !(tol{abs: 0.5}).ok(exposed, 106) {
		t.Errorf("exposed area = %.6g ft^2, the book's Lift chapter quotes 106 ft^2", exposed)
	}
	if w.ExposedArea.SI() >= w.Projected.Area.SI() {
		t.Error("the exposed area must be smaller than the reference area it is carved out of")
	}
}

// Without a body width there is no exposed area to report, and the zero
// Quantity means "not reported" rather than zero area.
func TestExposedAreaIsNotReportedWithoutABody(t *testing.T) {
	def := bookWingDefinition(t)
	def.BodyWidth = calculator.Quantity{}
	w, err := calculator.SolveWing(def)
	if err != nil {
		t.Fatalf("SolveWing: %v", err)
	}
	if w.ExposedArea != (calculator.Quantity{}) {
		t.Errorf("exposed area = %v, want the unset Quantity", w.ExposedArea)
	}
	if _, ok := w.Parameters().Get(calculator.ParamAreaExposed); ok {
		t.Error("an unreported exposed area must not appear as a parameter")
	}
}

// The two dihedral modes describe different aircraft, and the choice is
// required as soon as the dihedral is not zero.
func TestDihedralModesHoldDifferentDimensionsFixed(t *testing.T) {
	drivers := calculator.PlanformDrivers{
		Shape:      calculator.ShapeRectangle,
		Span:       mustQ(t, 1.2, calculator.Meter),
		RootChord:  mustQ(t, 0.2, calculator.Meter),
		TaperRatio: 1,
	}
	base := calculator.WingDefinition{
		Drivers:        drivers,
		Sweep:          mustQ(t, 0, calculator.Degree),
		SweepReference: 0.25,
		Dihedral:       mustQ(t, 10, calculator.Degree),
		Twist:          mustQ(t, 0, calculator.Degree),
		Incidence:      mustQ(t, 0, calculator.Degree),
		AreaBasis:      calculator.AreaBasisReferenceTrapezoid,
	}
	cos10 := math.Cos(10 * math.Pi / 180)
	exact := tol{rel: floatNoise}

	missing := base
	err := errorFrom(calculator.SolveWing(missing))
	if err == nil {
		t.Fatal("a dihedral change with no mode must not be solved")
	}
	wantIssue(t, err, "dihedral_mode", calculator.IssueMissing)

	holdPanel := base
	holdPanel.DihedralMode = calculator.DihedralHoldPanel
	built, err := calculator.SolveWing(holdPanel)
	if err != nil {
		t.Fatalf("hold panel: %v", err)
	}
	if !exact.ok(built.Panel.Span.SI(), 1.2) {
		t.Errorf("holding the panel fixed must keep the built span at 1.2 m, got %v", built.Panel.Span)
	}
	if !exact.ok(built.Projected.Span.SI(), 1.2*cos10) {
		t.Errorf("projected span = %v, want 1.2 m * cos(10 deg)", built.Projected.Span)
	}
	if !exact.ok(built.Projected.Area.SI(), 0.24*cos10) {
		t.Errorf("projected area = %v, want 0.24 m^2 * cos(10 deg)", built.Projected.Area)
	}

	holdProjected := base
	holdProjected.DihedralMode = calculator.DihedralHoldProjected
	reference, err := calculator.SolveWing(holdProjected)
	if err != nil {
		t.Fatalf("hold projected: %v", err)
	}
	if !exact.ok(reference.Projected.Span.SI(), 1.2) {
		t.Errorf("holding the projection fixed must keep the plan-view span at 1.2 m, got %v",
			reference.Projected.Span)
	}
	if !exact.ok(reference.Panel.Span.SI(), 1.2/cos10) {
		t.Errorf("panel span = %v, want 1.2 m / cos(10 deg)", reference.Panel.Span)
	}
	if reference.Panel.Span.SI() <= reference.Projected.Span.SI() {
		t.Error("the built panel must be longer than its own plan-view projection")
	}
	// The two modes are genuinely different wings, not two spellings of one.
	if exact.ok(built.Projected.Span.SI(), reference.Projected.Span.SI()) {
		t.Error("the two dihedral modes must not produce the same plan view")
	}
	// The aspect ratio an aerodynamic model would use follows the plan view.
	if !exact.ok(built.ProjectedAspectRatio, 6*cos10) {
		t.Errorf("projected aspect ratio = %v, want 6 * cos(10 deg)", built.ProjectedAspectRatio)
	}
}

// At zero dihedral the two planes coincide, so no mode has to be chosen and
// neither plane can drift from the other.
func TestZeroDihedralNeedsNoModeAndKeepsBothPlanesEqual(t *testing.T) {
	def := bookWingDefinition(t)
	def.Dihedral = mustQ(t, 0, calculator.Degree)
	def.DihedralMode = calculator.DihedralModeUnknown
	w, err := calculator.SolveWing(def)
	if err != nil {
		t.Fatalf("SolveWing: %v", err)
	}
	exact := tol{rel: floatNoise}
	if !exact.ok(w.Panel.Span.SI(), w.Projected.Span.SI()) ||
		!exact.ok(w.Panel.Area.SI(), w.Projected.Area.SI()) {
		t.Errorf("at zero dihedral the panel %v/%v and the projection %v/%v are the same wing",
			w.Panel.Span, w.Panel.Area, w.Projected.Span, w.Projected.Area)
	}
	if w.TipRise.SI() != 0 {
		t.Errorf("a flat wing has no tip rise, got %v", w.TipRise)
	}
}

// Every angle is a stated choice. An unrecorded sweep is not an unswept wing.
func TestWingAnglesAndAreaBasisMustBeStated(t *testing.T) {
	for _, c := range []struct {
		mutate func(*calculator.WingDefinition)
		field  string
	}{
		{func(d *calculator.WingDefinition) { d.Sweep = calculator.Quantity{} }, "sweep"},
		{func(d *calculator.WingDefinition) { d.Dihedral = calculator.Quantity{} }, "dihedral"},
		{func(d *calculator.WingDefinition) { d.Twist = calculator.Quantity{} }, "twist"},
		{func(d *calculator.WingDefinition) { d.Incidence = calculator.Quantity{} }, "incidence"},
		{func(d *calculator.WingDefinition) { d.AreaBasis = calculator.AreaBasisUnknown }, "area_basis"},
	} {
		t.Run(c.field, func(t *testing.T) {
			def := bookWingDefinition(t)
			c.mutate(&def)
			err := errorFrom(calculator.SolveWing(def))
			if err == nil {
				t.Fatalf("an unstated %s must not be read as zero", c.field)
			}
			wantIssue(t, err, c.field, calculator.IssueMissing)
		})
	}

	// An imported exposed area is a different measurement, and converting it is
	// not implemented rather than impossible.
	def := bookWingDefinition(t)
	def.AreaBasis = calculator.AreaBasisExposedPanels
	detail := wantIssue(t, errorFrom(calculator.SolveWing(def)), "area_basis",
		calculator.IssueUnsupported)
	if !containsText(detail, "not implemented") {
		t.Errorf("an unsupported area basis must say so, got %q", detail)
	}

	def = bookWingDefinition(t)
	def.SweepReference = 1.5
	wantIssue(t, errorFrom(calculator.SolveWing(def)), "sweep_reference", calculator.IssueInvalid)

	def = bookWingDefinition(t)
	def.Dihedral = mustQ(t, 75, calculator.Degree)
	wantIssue(t, errorFrom(calculator.SolveWing(def)), "dihedral", calculator.IssueUnsupported)

	def = bookWingDefinition(t)
	def.RootAirfoil.Evidence = ""
	wantIssue(t, errorFrom(calculator.SolveWing(def)), "root_airfoil", calculator.IssueMissing)
}

// Washout, negative incidence, anhedral and forward sweep are ordinary design
// choices. Rejecting them because they are unusual would be wrong.
func TestUnusualButValidAnglesAreAccepted(t *testing.T) {
	def := bookWingDefinition(t)
	def.Twist = mustQ(t, -6, calculator.Degree)
	def.Incidence = mustQ(t, -1.5, calculator.Degree)
	def.Dihedral = mustQ(t, -4, calculator.Degree)
	def.Sweep = mustQ(t, -8, calculator.Degree)
	w, err := calculator.SolveWing(def)
	if err != nil {
		t.Fatalf("SolveWing: %v", err)
	}
	if w.TipRise.SI() >= 0 {
		t.Errorf("anhedral must put the tip below the root chord line, got %v", w.TipRise)
	}
	if w.LeadingEdgeSweep.SI() >= 0 {
		t.Errorf("a forward-swept quarter chord this far forward must leave the leading edge "+
			"forward too, got %v", w.LeadingEdgeSweep)
	}
	if w.TipLeadingEdgeOffset.SI() >= 0 {
		t.Errorf("forward sweep must put the tip leading edge ahead of the root, got %v",
			w.TipLeadingEdgeOffset)
	}
}

// The outline must reconstruct the wing it came from, and a projected outline
// must stay distinguishable from the surface the panel is built on.
func TestOutlineReconstructsTheSolvedPlanform(t *testing.T) {
	w, err := calculator.SolveWing(bookWingDefinition(t))
	if err != nil {
		t.Fatalf("SolveWing: %v", err)
	}
	plan, err := w.Outline(calculator.OutlinePlanView)
	if err != nil {
		t.Fatalf("plan outline: %v", err)
	}
	if plan.Datum != calculator.DatumWingRoot {
		t.Error("an outline without its datum is not a coordinate")
	}
	if len(plan.Points) != 4 {
		t.Fatalf("a trapezoid panel has 4 corners, got %d", len(plan.Points))
	}
	exact := tol{rel: 1e-12}

	// Span and chords read straight back off the corners.
	semiSpan := plan.Points[1].Y.SI()
	if !exact.ok(2*semiSpan, w.Projected.Span.SI()) {
		t.Errorf("outline span = %v m, solved %v", 2*semiSpan, w.Projected.Span)
	}
	rootChord := plan.Points[3].X.SI() - plan.Points[0].X.SI()
	if !exact.ok(rootChord, w.Planform.RootChord.SI()) {
		t.Errorf("outline root chord = %v m, solved %v", rootChord, w.Planform.RootChord)
	}
	tipChord := plan.Points[2].X.SI() - plan.Points[1].X.SI()
	if !exact.ok(tipChord, w.Planform.TipChord.SI()) {
		t.Errorf("outline tip chord = %v m, solved %v", tipChord, w.Planform.TipChord)
	}
	// Area by the shoelace formula over the right panel, doubled for both sides.
	if area := 2 * shoelace(plan.Points); !exact.ok(area, w.Projected.Area.SI()) {
		t.Errorf("outline area = %v m^2, solved %v", area, w.Projected.Area)
	}

	panel, err := w.Outline(calculator.OutlinePanelSurface)
	if err != nil {
		t.Fatalf("panel outline: %v", err)
	}
	if panel.Points[1].Z.SI() <= 0 {
		t.Error("with dihedral the built tip must sit above the plan view")
	}
	// The panel's own length is the flat span, not the projected one.
	built := math.Hypot(panel.Points[1].Y.SI(), panel.Points[1].Z.SI())
	if !exact.ok(2*built, w.Panel.Span.SI()) {
		t.Errorf("panel length %v m does not match the panel span %v", 2*built, w.Panel.Span)
	}
	if exact.ok(2*built, w.Projected.Span.SI()) {
		t.Error("the built panel and its projection must stay distinguishable under dihedral")
	}
	if _, err := w.Outline(calculator.OutlinePlaneUnknown); err == nil {
		t.Error("an outline must say which plane it is in")
	}
}

// A MAC is a length until it is placed. Locating it fore and aft is what a
// later centre-of-gravity position is measured against, and it needs the datum
// and the leading-edge sweep to mean anything.
func TestMACIsLocatedAgainstTheDatum(t *testing.T) {
	w, err := calculator.SolveWing(bookWingDefinition(t))
	if err != nil {
		t.Fatalf("SolveWing: %v", err)
	}
	station := inUnit(t, w.MACLeadingEdgeStation, calculator.Foot)
	// Independently calculated: y_MAC 7.016016661604957 ft times tan(3.0665 deg),
	// where the tangent is exactly 0.05357142857142857 for this wing.
	if !(tol{abs: 1e-12, rel: 1e-12}).ok(station, 0.3758580354431227027019558) {
		t.Errorf("MAC leading edge = %.15g ft aft of the root, independently calculated "+
			"0.375858035443123 ft", station)
	}

	// The placement has to agree with the outline it is placed in: at the MAC's
	// own spanwise station the panel's leading edge is exactly there, and the
	// chord there is the MAC.
	ev := eval{t}
	chord := ev.ok(w.Planform.ChordAt(w.Projected.YMAC))
	if !(tol{rel: floatNoise}).ok(chord.Value.SI(), w.Planform.MAC.SI()) {
		t.Errorf("chord at y_MAC %v does not match the MAC %v", chord.Value, w.Planform.MAC)
	}
	plan, err := w.Outline(calculator.OutlinePlanView)
	if err != nil {
		t.Fatalf("plan outline: %v", err)
	}
	// Interpolating the outline's leading edge to y_MAC must land on the same x.
	fraction := w.Projected.YMAC.SI() / plan.Points[1].Y.SI()
	interpolated := plan.Points[0].X.SI() + fraction*(plan.Points[1].X.SI()-plan.Points[0].X.SI())
	if !(tol{rel: 1e-12}).ok(interpolated, w.MACLeadingEdgeStation.SI()) {
		t.Errorf("the outline puts the leading edge at %v m at y_MAC, the placement says %v",
			interpolated, w.MACLeadingEdgeStation)
	}
	if w.MACLeadingEdgeStation.SI() <= 0 {
		t.Error("an aft-swept leading edge must put the MAC aft of the root leading edge")
	}
}

// shoelace returns the area of the polygon the points enclose.
func shoelace(points []calculator.Point) float64 {
	sum := 0.0
	for i, p := range points {
		q := points[(i+1)%len(points)]
		sum += p.X.SI()*q.Y.SI() - q.X.SI()*p.Y.SI()
	}
	return math.Abs(sum) / 2
}
