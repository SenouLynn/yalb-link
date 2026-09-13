package calculator_test

import (
	"testing"

	"yalb.aero/calculator"
)

// The CODE Lab Drag Polar chapter's worked example, computed independently at
// 50 significant digits and quoted here to 25. testdata/drag-polar-book-example.md
// records the derivation, the agreement with the chapter's displayed 12.31 and
// the two discrepancies found while reading it.
const (
	bookPolarAspect       = 8.0
	bookPolarCD0          = 0.03363
	bookPolarOswald       = 0.8105923299393962904054311
	bookPolarInducedK     = 0.04908600082108922456882981
	bookPolarCLatBestLD   = 0.8277222097430363354943594
	bookPolarBestLD       = 12.30630701372340671267261
	bookPolarCDatCL08     = 0.06504504052549710372405108
	bookPolarLDatCL08     = 12.29916982965683280854472
	bookPolarMinimumPower = 1.433656921828121717994504
)

// The dimensional extension: the same aircraft at the Mission analysis
// chapter's cruise condition, evaluated in customary units and in SI.
const (
	bookCruiseAreaFt2      = 134.0
	bookCruiseDensitySlug  = 0.00186850
	bookCruiseSpeedKnots   = 200.0
	bookCruiseDynamicPa    = 5097.140753771973265528064
	bookCruiseDynamicLbfFt = 106.4559979900138126750876
	bookCruiseDragNewton   = 4127.390296256034322231295
	bookCruiseDragLbf      = 927.8742502613200129504386
)

// polarTol is the acceptance tolerance for the fixtures above: an absolute term
// in the quantity's own SI unit plus a relative term taken from the precision
// they are quoted to, not from display rounding.
var polarTol = tol{abs: 1e-12, rel: 1e-13}

// bookPolar is the chapter's example polar. Its validity range is this
// package's requirement rather than the chapter's, and is set wide enough to
// contain the annotated CL = 0.8, the analytic optimum and the chapter's
// minimum-power condition, so that a refusal in a test below is the test's own
// doing rather than an accident of the fixture.
func bookPolar(t *testing.T) calculator.DragPolar {
	t.Helper()
	return calculator.DragPolar{
		CD0:              bookPolarCD0,
		CD0Basis:         "CODE Lab Drag Polar chapter worked example, clean configuration",
		OswaldEfficiency: bookPolarOswald,
		EfficiencyBasis:  "Raymer 12.48 straight-wing correlation at A = 8, as the chapter quotes it",
		Configuration:    "clean",
		CLValidMin:       -0.5,
		CLValidMax:       1.5,
		Scope:            calculator.ScopeAircraft,
		Evidence:         calculator.EvidenceAssumed,
	}
}

func TestOswaldCorrelationReproducesTheChapterValue(t *testing.T) {
	ev := eval{t}
	got := ev.ok(calculator.OswaldEfficiencyStraightWing(bookPolarAspect))
	if !polarTol.ok(got.Value.SI(), bookPolarOswald) {
		t.Errorf("e = %v, want %v", got.Value.SI(), bookPolarOswald)
	}
	if got.Trace.EquationID != calculator.EqOswaldStraightWing {
		t.Errorf("trace equation = %q", got.Trace.EquationID)
	}
}

// The correlation is quoted with no validity range and returns values a span
// efficiency cannot take outside a narrow band of aspect ratios. Refusing it
// there is what stops an offered estimate from becoming a wrong number.
func TestOswaldCorrelationIsRefusedWhereItStopsMeaningAnything(t *testing.T) {
	ev := eval{t}
	for _, aspect := range []float64{1.2, 30} {
		err := ev.fails(calculator.OswaldEfficiencyStraightWing(aspect))
		detail := wantIssue(t, err, "aspect_ratio_projected", calculator.IssueUnsupported)
		if detail == "" {
			t.Errorf("aspect ratio %v: refusal carries no detail", aspect)
		}
	}
}

func TestInducedDragFactorAndPolarReproduceTheChapter(t *testing.T) {
	ev := eval{t}
	polar := bookPolar(t)

	factor := ev.ok(calculator.InducedDragFactor(bookPolarAspect, polar))
	if !polarTol.ok(factor.Value.SI(), bookPolarInducedK) {
		t.Errorf("K = %v, want %v", factor.Value.SI(), bookPolarInducedK)
	}

	cd := ev.ok(calculator.DragCoefficient(bookPolarAspect, polar, 0.8))
	if !polarTol.ok(cd.Value.SI(), bookPolarCDatCL08) {
		t.Errorf("CD at CL 0.8 = %v, want %v", cd.Value.SI(), bookPolarCDatCL08)
	}

	ratio := ev.ok(calculator.LiftToDrag(0.8, cd.Value.SI()))
	if !polarTol.ok(ratio.Value.SI(), bookPolarLDatCL08) {
		t.Errorf("L/D at CL 0.8 = %v, want %v", ratio.Value.SI(), bookPolarLDatCL08)
	}

	// The chapter's displayed maximum, 12.31, is the polar at its own optimum.
	best := ev.ok(calculator.DragCoefficient(bookPolarAspect, polar, bookPolarCLatBestLD))
	bestRatio := ev.ok(calculator.LiftToDrag(bookPolarCLatBestLD, best.Value.SI()))
	if !polarTol.ok(bestRatio.Value.SI(), bookPolarBestLD) {
		t.Errorf("max L/D = %v, want %v", bestRatio.Value.SI(), bookPolarBestLD)
	}
	if rounded := roundTo(bestRatio.Value.SI(), 2); rounded != 12.31 {
		t.Errorf("max L/D displays as %v, and the chapter shows 12.31", rounded)
	}
}

// The chapter's annotated marker sits at CL = 0.8, where L/D is 12.2992 rather
// than the maximum 12.3063. The two are different numbers and this package must
// keep them different: LiftToDrag is a ratio at a condition, not an optimum.
func TestAnnotatedPointIsNotTheMaximumLiftToDrag(t *testing.T) {
	if !(bookPolarLDatCL08 < bookPolarBestLD) {
		t.Fatal("the fixture no longer distinguishes the annotated point from the maximum")
	}
	ev := eval{t}
	polar := bookPolar(t)
	cd := ev.ok(calculator.DragCoefficient(bookPolarAspect, polar, 0.8))
	ratio := ev.ok(calculator.LiftToDrag(0.8, cd.Value.SI()))
	if ratio.Value.SI() >= bookPolarBestLD {
		t.Errorf("L/D at the annotated point = %v, which is not below the maximum %v",
			ratio.Value.SI(), bookPolarBestLD)
	}
}

func TestMinimumPowerLiftCoefficientMatchesTheChapterRelation(t *testing.T) {
	ev := eval{t}
	polar := bookPolar(t)
	got := ev.ok(calculator.MinimumPowerLiftCoefficient(bookPolarAspect, polar))
	if !polarTol.ok(got.Value.SI(), bookPolarMinimumPower) {
		t.Errorf("CL at minimum power = %v, want %v", got.Value.SI(), bookPolarMinimumPower)
	}
}

// The minimum-power condition can fall outside the polar's own validity range,
// and when it does it is refused rather than reported as a flyable condition.
func TestMinimumPowerIsRefusedOutsideThePolarRange(t *testing.T) {
	ev := eval{t}
	polar := bookPolar(t)
	polar.CLValidMax = 1.0
	err := ev.fails(calculator.MinimumPowerLiftCoefficient(bookPolarAspect, polar))
	wantIssue(t, err, "cl", calculator.IssueUnsupported)
}

// The drag force is checked in the chapter's own units and in SI. The two
// routes are the same evaluation reached through different unit conversions, so
// agreement here is what would catch a mistyped slug or knot factor.
func TestDragForceAgreesInOriginalUnitsAndSI(t *testing.T) {
	ev := eval{t}
	polar := bookPolar(t)
	cd := ev.ok(calculator.DragCoefficient(bookPolarAspect, polar, 0.8))

	density := mustQ(t, bookCruiseDensitySlug, calculator.SlugPerCubicFoot)
	speed := mustQ(t, bookCruiseSpeedKnots, calculator.Knot)
	area := mustQ(t, bookCruiseAreaFt2, calculator.SquareFoot)
	fc := baseCase(t)
	fc.Density = density
	fc.DensityBasis = "CODE Lab Mission analysis chapter, 8000 ft"

	q := ev.ok(calculator.DynamicPressure(fc, speed))
	if !tolFromQuoted.ok(q.Value.SI(), bookCruiseDynamicPa) {
		t.Errorf("q = %v Pa, want %v", q.Value.SI(), bookCruiseDynamicPa)
	}
	if got := inUnit(t, q.Value, calculator.PoundForcePerSquareFoot); !tolFromQuoted.ok(got, bookCruiseDynamicLbfFt) {
		t.Errorf("q = %v lbf/ft^2, want %v", got, bookCruiseDynamicLbfFt)
	}

	drag := ev.ok(calculator.DragForce(q.Value, area, cd.Value.SI()))
	if !tolFromQuoted.ok(drag.Value.SI(), bookCruiseDragNewton) {
		t.Errorf("D = %v N, want %v", drag.Value.SI(), bookCruiseDragNewton)
	}
	if got := inUnit(t, drag.Value, calculator.PoundForce); !tolFromQuoted.ok(got, bookCruiseDragLbf) {
		t.Errorf("D = %v lbf, want %v", got, bookCruiseDragLbf)
	}

	// The same condition entered in SI must give the same drag, which is what
	// makes the unit table's slug and knot factors checked rather than trusted.
	siCase := fc
	siCase.Density = mustQ(t, 0.9629853221676871061295551, calculator.KilogramPerCubicMeter)
	siQ := ev.ok(calculator.DynamicPressure(siCase, mustQ(t, 102.8888888888888888888889, calculator.MeterPerSecond)))
	siDrag := ev.ok(calculator.DragForce(siQ.Value, mustQ(t, 12.44900736, calculator.SquareMeter), cd.Value.SI()))
	if !sameWithin(siDrag.Value.SI(), drag.Value.SI()) {
		t.Errorf("SI route gives %v N and the customary route %v N", siDrag.Value.SI(), drag.Value.SI())
	}
}

// tolFromQuoted absorbs the 25-significant-digit truncation of the fixture
// constants above, which is looser than floatNoise and far tighter than any
// display rounding.
var tolFromQuoted = tol{abs: 1e-9, rel: 1e-14}

// A parabolic polar returns a number at any lift coefficient at all. The stated
// validity range is what stops that number from being reported as the aircraft's
// drag in flow the polar does not describe.
func TestPolarRefusesLiftCoefficientsOutsideItsStatedRange(t *testing.T) {
	ev := eval{t}
	polar := bookPolar(t)
	for _, cl := range []float64{-0.9, 2.5} {
		err := ev.fails(calculator.DragCoefficient(bookPolarAspect, polar, cl))
		detail := wantIssue(t, err, "cl", calculator.IssueUnsupported)
		if detail == "" {
			t.Errorf("CL %v: refusal carries no detail", cl)
		}
	}
}

// A polar without an aircraft-level scope is refused for the same reason a 2D
// section clmax is: section data carries no induced, interference or trim drag.
func TestSectionPolarIsRefusedWhereAnAircraftPolarIsRequired(t *testing.T) {
	polar := bookPolar(t)
	polar.Scope = calculator.ScopeAirfoilSection
	session := calculator.NewSession("polar", baseDesign(t))
	err := session.Do(calculator.SetDragPolar{Polar: polar})
	if err == nil {
		t.Fatal("a section polar was accepted as an aircraft polar")
	}
	wantIssue(t, err, "polar", calculator.IssueUnsupported)
	if session.Revisions() != 1 {
		t.Error("a refused command recorded a revision")
	}
}

// A polar with no stated validity range is a design problem the definition
// reports, not a refused keystroke: the two coefficients and the range can be
// typed in any order.
func TestPolarWithoutAValidityRangeIsReportedByValidation(t *testing.T) {
	d := baseDesign(t)
	polar := bookPolar(t)
	polar.CLValidMin, polar.CLValidMax = 0, 0
	d.Polar = polar
	err := d.Validate()
	if err == nil {
		t.Fatal("a polar with no validity range was accepted as a complete definition")
	}
	wantIssue(t, err, "polar.validity", calculator.IssueMissing)
}

func roundTo(v float64, places int) float64 {
	scale := 1.0
	for n := 0; n < places; n++ {
		scale *= 10
	}
	return float64(int64(v*scale+0.5)) / scale
}
