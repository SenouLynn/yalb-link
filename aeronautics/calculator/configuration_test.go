package calculator_test

import (
	"testing"

	"yalb.aero/calculator"
)

func rcWingDefinition(t *testing.T) calculator.WingDefinition {
	t.Helper()
	return calculator.WingDefinition{
		Name: "RC wing",
		Drivers: calculator.PlanformDrivers{
			Shape:       calculator.ShapeRectangle,
			Area:        mustQ(t, 24, calculator.SquareDecimeter),
			AspectRatio: 6,
		},
		Sweep:          mustQ(t, 0, calculator.Degree),
		SweepReference: 0.25,
		Dihedral:       mustQ(t, 3, calculator.Degree),
		DihedralMode:   calculator.DihedralHoldProjected,
		Twist:          mustQ(t, 0, calculator.Degree),
		Incidence:      mustQ(t, 1, calculator.Degree),
		AreaBasis:      calculator.AreaBasisReferenceTrapezoid,
	}
}

func conventionalTail(t *testing.T) calculator.TailGeometry {
	t.Helper()
	return calculator.TailGeometry{
		Horizontal: calculator.TailSurface{
			Area: mustQ(t, 4, calculator.SquareDecimeter),
			Span: mustQ(t, 0.4, calculator.Meter),
			Arm:  mustQ(t, 0.55, calculator.Meter),
		},
		Vertical: calculator.TailSurface{
			Area: mustQ(t, 2, calculator.SquareDecimeter),
			Span: mustQ(t, 0.18, calculator.Meter),
			Arm:  mustQ(t, 0.55, calculator.Meter),
		},
	}
}

// A flying wing is complete without a tail. Reporting a missing horizontal tail
// on one would be the exact error the configuration exists to prevent.
func TestFlyingWingNeverRequiresAHorizontalTail(t *testing.T) {
	wing := calculator.Airframe{
		Name:          "flying wing",
		Configuration: calculator.ConfigurationFlyingWing,
		Wing:          rcWingDefinition(t),
	}
	if err := wing.ValidateGeometry(); err != nil {
		t.Fatalf("a flying wing needs no tail: %v", err)
	}

	withTail := wing
	withTail.Tail = conventionalTail(t)
	err := withTail.ValidateGeometry()
	if err == nil {
		t.Fatal("a flying wing with a tail group is a different configuration")
	}
	wantIssue(t, err, "tail", calculator.IssueInvalid)

	// Nor does it have a horizontal tail area to report.
	_, err = wing.HorizontalTailArea()
	detail := wantIssue(t, err, "horizontal_tail", calculator.IssueUnsupported)
	if !containsText(detail, "no horizontal tail") {
		t.Errorf("the refusal should say why, got %q", detail)
	}
}

func TestConventionalTailNeedsBothSurfaces(t *testing.T) {
	conventional := calculator.Airframe{
		Name:          "conventional",
		Configuration: calculator.ConfigurationConventionalTail,
		Wing:          rcWingDefinition(t),
		Tail:          conventionalTail(t),
	}
	if err := conventional.ValidateGeometry(); err != nil {
		t.Fatalf("a complete conventional tail: %v", err)
	}
	area, err := conventional.HorizontalTailArea()
	if err != nil {
		t.Fatalf("HorizontalTailArea: %v", err)
	}
	if !(tol{rel: floatNoise}).ok(area.SI(), 0.04) {
		t.Errorf("horizontal tail area = %v, want 4 dm^2", area)
	}

	missing := conventional
	missing.Tail.Horizontal = calculator.TailSurface{}
	wantIssue(t, missing.ValidateGeometry(), "horizontal_tail", calculator.IssueMissing)

	missing = conventional
	missing.Tail.Vertical = calculator.TailSurface{}
	wantIssue(t, missing.ValidateGeometry(), "vertical_tail", calculator.IssueMissing)

	mixed := conventional
	mixed.Tail.VTail = calculator.VTailPanels{
		PanelArea: mustQ(t, 3, calculator.SquareDecimeter),
		Cant:      mustQ(t, 35, calculator.Degree),
	}
	wantIssue(t, mixed.ValidateGeometry(), "v_tail", calculator.IssueInvalid)
}

// A V-tail is two canted panels. It is not two imaginary surfaces, and this
// package will not invent them.
func TestVTailRecordsPanelsNotImaginarySurfaces(t *testing.T) {
	vtail := calculator.Airframe{
		Name:          "V-tail",
		Configuration: calculator.ConfigurationVTail,
		Wing:          rcWingDefinition(t),
		Tail: calculator.TailGeometry{
			VTail: calculator.VTailPanels{
				PanelArea: mustQ(t, 3, calculator.SquareDecimeter),
				PanelSpan: mustQ(t, 0.22, calculator.Meter),
				Cant:      mustQ(t, 35, calculator.Degree),
				Arm:       mustQ(t, 0.55, calculator.Meter),
			},
		},
	}
	if err := vtail.ValidateGeometry(); err != nil {
		t.Fatalf("a described V-tail: %v", err)
	}

	_, err := vtail.HorizontalTailArea()
	detail := wantIssue(t, err, "v_tail", calculator.IssueUnsupported)
	if !containsText(detail, "not implemented") {
		t.Errorf("the refusal must name the missing model, got %q", detail)
	}

	both := vtail
	both.Tail.Horizontal = conventionalTail(t).Horizontal
	wantIssue(t, both.ValidateGeometry(), "v_tail", calculator.IssueInvalid)

	// A cant of 90 degrees is a fin, not a V-tail, and its handling model is a
	// different one again.
	upright := vtail
	upright.Tail.VTail.Cant = mustQ(t, 88, calculator.Degree)
	wantIssue(t, upright.ValidateGeometry(), "v_tail_cant", calculator.IssueUnsupported)

	empty := vtail
	empty.Tail.VTail = calculator.VTailPanels{}
	wantIssue(t, empty.ValidateGeometry(), "v_tail", calculator.IssueMissing)
}

func TestConfigurationMustBeChosen(t *testing.T) {
	unset := calculator.Airframe{Name: "unset", Wing: rcWingDefinition(t)}
	wantIssue(t, unset.ValidateGeometry(), "configuration", calculator.IssueMissing)
}

// Geometry is not handling. Every configuration, including the conventional one
// this task's geometry covers best, reports handling as unsupported until a
// model with its own evidence exists.
func TestNoConfigurationHasASupportedHandlingAssessment(t *testing.T) {
	for _, configuration := range []calculator.Configuration{
		calculator.ConfigurationConventionalTail,
		calculator.ConfigurationVTail,
		calculator.ConfigurationFlyingWing,
	} {
		t.Run(configuration.String(), func(t *testing.T) {
			airframe := calculator.Airframe{Configuration: configuration, Wing: rcWingDefinition(t)}
			err := airframe.Handling()
			if err == nil {
				t.Fatal("geometry alone must not report a handling result")
			}
			detail := wantIssue(t, err, "handling", calculator.IssueUnsupported)
			if !containsText(detail, configuration.String()) {
				t.Errorf("the refusal should name the configuration, got %q", detail)
			}
		})
	}
}
