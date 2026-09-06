package calculator_test

import (
	"math"
	"testing"

	"yalb.aero/calculator"
)

// Conversions must be reversible without drift: a value written in one unit and
// read back in it is the same number, because nothing is rounded in between.
func TestConversionRoundTrips(t *testing.T) {
	units := []calculator.Unit{
		calculator.Kilogram, calculator.Gram, calculator.PoundMass, calculator.Ounce,
		calculator.Newton, calculator.PoundForce,
		calculator.Meter, calculator.Millimeter, calculator.Centimeter,
		calculator.Inch, calculator.Foot,
		calculator.SquareMeter, calculator.SquareCentimeter, calculator.SquareDecimeter,
		calculator.SquareInch, calculator.SquareFoot,
		calculator.MeterPerSecond, calculator.KilometerPerHour, calculator.Knot,
		calculator.MilePerHour, calculator.FootPerSecond,
		calculator.KilogramPerCubicMeter,
		calculator.NewtonPerSquareMeter, calculator.PoundForcePerSquareFoot,
		calculator.KilogramPerSquareMeter, calculator.GramPerSquareDecimeter,
		calculator.OuncePerSquareFoot,
		calculator.Radian, calculator.Degree,
		calculator.Watt, calculator.Joule, calculator.WattHour,
		calculator.One,
	}
	for _, u := range units {
		if u.Symbol() == "" {
			t.Errorf("unit %d has no symbol", u)
			continue
		}
		t.Run(u.Symbol(), func(t *testing.T) {
			for _, v := range []float64{1, 0.24, 1234.5678, 1e-6, 1e6} {
				q := mustQ(t, v, u)
				back := inUnit(t, q, u)
				if !(tol{abs: 0, rel: 1e-15}).ok(back, v) {
					t.Errorf("%v %s round-tripped to %v", v, u.Symbol(), back)
				}
			}
		})
	}
}

// Exact defined conversions must be exact, not approximated.
func TestExactDefinedConversions(t *testing.T) {
	cases := []struct {
		name string
		from calculator.Unit
		to   calculator.Unit
		in   float64
		want float64
	}{
		{"pound to kilogram", calculator.PoundMass, calculator.Kilogram, 1, 0.45359237},
		{"foot to metre", calculator.Foot, calculator.Meter, 1, 0.3048},
		{"inch to metre", calculator.Inch, calculator.Meter, 1, 0.0254},
		{"gram to kilogram", calculator.Gram, calculator.Kilogram, 2000, 2},
		{"square decimetre to square metre", calculator.SquareDecimeter, calculator.SquareMeter, 24, 0.24},
		{"metre per second to km/h", calculator.MeterPerSecond, calculator.KilometerPerHour, 1, 3.6},
		{"knot to km/h", calculator.Knot, calculator.KilometerPerHour, 1, 1.852},
		{"mph to m/s", calculator.MilePerHour, calculator.MeterPerSecond, 1, 0.44704},
		{"watt hour to joule", calculator.WattHour, calculator.Joule, 1, 3600},
		{"degree to radian", calculator.Degree, calculator.Radian, 180, math.Pi},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := inUnit(t, mustQ(t, c.in, c.from), c.to)
			if !(tol{abs: 0, rel: 1e-15}).ok(got, c.want) {
				t.Errorf("%v %s = %v %s, want %v", c.in, c.from.Symbol(), got, c.to.Symbol(), c.want)
			}
		})
	}
}

// A composite unit must agree with the base units it is built from. This is
// what catches a mistyped factor in a derived unit such as oz/ft^2.
func TestCompositeUnitsAgreeWithTheirBases(t *testing.T) {
	oneFootInM := inUnit(t, mustQ(t, 1, calculator.Foot), calculator.Meter)
	oneInchInM := inUnit(t, mustQ(t, 1, calculator.Inch), calculator.Meter)
	exact := tol{abs: 0, rel: 1e-15}

	t.Run("square foot is a foot squared", func(t *testing.T) {
		got := inUnit(t, mustQ(t, 1, calculator.SquareFoot), calculator.SquareMeter)
		if !exact.ok(got, oneFootInM*oneFootInM) {
			t.Errorf("1 ft^2 = %v m^2, want %v", got, oneFootInM*oneFootInM)
		}
	})

	t.Run("square inch is an inch squared", func(t *testing.T) {
		got := inUnit(t, mustQ(t, 1, calculator.SquareInch), calculator.SquareMeter)
		if !exact.ok(got, oneInchInM*oneInchInM) {
			t.Errorf("1 in^2 = %v m^2, want %v", got, oneInchInM*oneInchInM)
		}
	})

	t.Run("ounce per square foot", func(t *testing.T) {
		ozInKg := inUnit(t, mustQ(t, 1, calculator.Ounce), calculator.Kilogram)
		ftSqInM := inUnit(t, mustQ(t, 1, calculator.SquareFoot), calculator.SquareMeter)
		got := inUnit(t, mustQ(t, 1, calculator.OuncePerSquareFoot), calculator.KilogramPerSquareMeter)
		if !exact.ok(got, ozInKg/ftSqInM) {
			t.Errorf("1 oz/ft^2 = %v kg/m^2, want %v", got, ozInKg/ftSqInM)
		}
	})

	t.Run("gram per square decimetre", func(t *testing.T) {
		got := inUnit(t, mustQ(t, 1, calculator.GramPerSquareDecimeter), calculator.KilogramPerSquareMeter)
		if !exact.ok(got, 0.1) {
			t.Errorf("1 g/dm^2 = %v kg/m^2, want 0.1", got)
		}
	})

	t.Run("pound-force per square foot", func(t *testing.T) {
		lbfInN := inUnit(t, mustQ(t, 1, calculator.PoundForce), calculator.Newton)
		ftSqInM := inUnit(t, mustQ(t, 1, calculator.SquareFoot), calculator.SquareMeter)
		got := inUnit(t, mustQ(t, 1, calculator.PoundForcePerSquareFoot), calculator.NewtonPerSquareMeter)
		if !exact.ok(got, lbfInN/ftSqInM) {
			t.Errorf("1 lbf/ft^2 = %v N/m^2, want %v", got, lbfInN/ftSqInM)
		}
	})
}

// Mass and force stay distinct even though a pound and a pound-force share a
// name, which is exactly the confusion the dimension check exists to stop.
func TestPoundMassIsNotPoundForce(t *testing.T) {
	lb := mustQ(t, 1, calculator.PoundMass)
	if _, err := lb.In(calculator.PoundForce); err == nil {
		t.Error("a pound mass was readable as a pound-force")
	}
	if lb.Dimension() != calculator.DimMass {
		t.Errorf("PoundMass has dimension %v", lb.Dimension())
	}
	if calculator.PoundForce.Dimension() != calculator.DimForce {
		t.Errorf("PoundForce has dimension %v", calculator.PoundForce.Dimension())
	}
}

// A conversion that would silently collapse a real value to zero is refused.
func TestUnderflowingConversionIsRefused(t *testing.T) {
	// Scaling the smallest representable float by a factor below one is
	// guaranteed to reach zero; a subnormal such as 1e-320 is not, because it
	// stays representable.
	if _, err := calculator.NewQuantity(math.SmallestNonzeroFloat64, calculator.SquareInch); err == nil {
		t.Error("a conversion underflowing to zero was accepted")
	}
	if _, err := calculator.NewQuantity(1e-320, calculator.SquareInch); err != nil {
		t.Errorf("a subnormal that stays representable must be accepted: %v", err)
	}
	if _, err := calculator.NewQuantity(0, calculator.SquareInch); err != nil {
		t.Errorf("an exact zero must remain convertible: %v", err)
	}
}

func TestUnsupportedUnitIsRejected(t *testing.T) {
	if _, err := calculator.NewQuantity(1, calculator.UnitInvalid); err == nil {
		t.Error("the zero Unit was accepted")
	}
	if _, err := calculator.NewQuantity(1, calculator.Unit(200)); err == nil {
		t.Error("an out-of-range Unit was accepted")
	}
	if calculator.Unit(200).Symbol() != "" {
		t.Error("an out-of-range Unit reported a symbol")
	}
}

// The zero Quantity is dimensionless zero, which is how an unsupplied field is
// distinguished from a supplied zero.
func TestZeroQuantityIsDimensionlessZero(t *testing.T) {
	var q calculator.Quantity
	if !q.IsZero() {
		t.Error("the zero Quantity is not zero")
	}
	if q.Dimension() != calculator.Dimensionless {
		t.Errorf("the zero Quantity has dimension %v", q.Dimension())
	}
	suppliedZero := mustQ(t, 0, calculator.SquareMeter)
	if suppliedZero.Dimension() != calculator.DimArea {
		t.Error("a supplied zero lost its dimension")
	}
	if suppliedZero == q {
		t.Error("a supplied zero area is indistinguishable from an unsupplied field")
	}
}

// A dimensionless value must not render its "1" symbol, which would read as
// part of the number: "1.2 1" rather than "1.2".
func TestDimensionlessValueRendersWithoutASymbol(t *testing.T) {
	ratio := mustQ(t, 1.2, calculator.One)
	if got := ratio.String(); got != "1.2" {
		t.Errorf("dimensionless String() = %q, want %q", got, "1.2")
	}
	area := mustQ(t, 0.24, calculator.SquareMeter)
	if got := area.String(); got != "0.24 m^2" {
		t.Errorf("area String() = %q, want %q", got, "0.24 m^2")
	}
}
