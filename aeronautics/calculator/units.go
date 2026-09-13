// Package calculator implements transport-free fixed-wing aircraft sizing
// calculations for the aeronautics worksheet.
//
// The package has no transport, storage, filesystem or clock dependency. The
// boundary test in yalb.aero/boundary enforces that, which is also why numbers
// are formatted with strconv rather than fmt: fmt's import closure reaches os.
package calculator

import (
	"errors"
	"math"
	"strconv"
)

// StandardGravity is the standard acceleration of free fall, in m/s². It is the
// defined CGPM value, not a site-specific local gravity.
const StandardGravity = 9.80665

// Dimension is the physical dimension of a Quantity. Quantities of different
// dimensions are never interchangeable, even when their numbers agree.
type Dimension uint8

// The dimensions this package supports. Every Quantity is stored in the SI unit
// named in the dimension's symbol.
const (
	// Dimensionless covers pure ratios such as load factor and lift coefficient.
	Dimensionless Dimension = iota
	// DimMass is stored in kilograms.
	DimMass
	// DimForce is stored in newtons.
	DimForce
	// DimLength is stored in metres.
	DimLength
	// DimArea is stored in square metres.
	DimArea
	// DimSpeed is stored in metres per second and always means true airspeed.
	DimSpeed
	// DimDensity is stored in kilograms per cubic metre.
	DimDensity
	// DimForcePerArea is stored in newtons per square metre.
	DimForcePerArea
	// DimMassPerArea is stored in kilograms per square metre.
	DimMassPerArea
	// DimAngle is stored in radians.
	DimAngle
	// DimPower is stored in watts.
	DimPower
	// DimEnergy is stored in joules.
	DimEnergy
	// DimDynamicViscosity is stored in pascal seconds.
	DimDynamicViscosity
	// DimMassMoment is stored in kilogram metres. It is the product of a mass
	// and a distance from a datum, which is what a centre-of-gravity sum is
	// formed of; it is not a torque, and nothing here treats it as one.
	DimMassMoment
	// DimTime is stored in seconds. It measures a mission segment's duration
	// and the endurance an energy budget supports; it is never a clock reading,
	// which this package has no way of taking.
	DimTime
	// DimCurrent is stored in amperes.
	DimCurrent
	// DimVoltage is stored in volts.
	DimVoltage
	// DimCharge is stored in coulombs. It is the dimension a battery capacity
	// in ampere hours carries, which is not an energy until it is multiplied by
	// a voltage.
	DimCharge
	// DimRotationRate is stored in revolutions per second. A revolution is a
	// counted event rather than an SI unit, so this dimension is deliberately
	// not the same as an angular rate in radians per second and nothing here
	// converts between the two.
	DimRotationRate
)

var dimensionSymbols = [...]string{
	Dimensionless:   "1",
	DimMass:         "kg",
	DimForce:        "N",
	DimLength:       "m",
	DimArea:         "m^2",
	DimSpeed:        "m/s",
	DimDensity:      "kg/m^3",
	DimForcePerArea: "N/m^2",
	DimMassPerArea:  "kg/m^2",
	DimAngle:        "rad",
	DimPower:        "W",
	DimEnergy:       "J",

	DimDynamicViscosity: "Pa*s",
	DimMassMoment:       "kg*m",
	DimTime:             "s",
	DimCurrent:          "A",
	DimVoltage:          "V",
	DimCharge:           "C",
	DimRotationRate:     "rev/s",
}

// String returns the dimension's SI symbol.
func (d Dimension) String() string {
	if int(d) < len(dimensionSymbols) {
		return dimensionSymbols[d]
	}
	return "unknown-dimension"
}

// SIUnit returns the unit a Quantity of this dimension is stored in. Exported
// parameters carry an explicit unit rather than an implied one, so a consumer
// such as a CAD export never has to infer it from the dimension name.
func (d Dimension) SIUnit() Unit {
	if int(d) < len(dimensionSIUnits) {
		return dimensionSIUnits[d]
	}
	return UnitInvalid
}

// Unit is a supported measurement unit. Units are typed constants, so a unit
// table cannot be reassigned by a caller inspecting metadata.
type Unit uint8

// The supported units. UnitInvalid is the zero value and every constructor
// rejects it, so a forgotten unit cannot be read as a silent default.
const (
	UnitInvalid Unit = iota
	// One is the unit of a dimensionless ratio.
	One
	// Kilogram is the SI mass unit.
	Kilogram
	// Gram is 1e-3 kg.
	Gram
	// PoundMass is the international avoirdupois pound, 0.45359237 kg.
	PoundMass
	// Ounce is one sixteenth of a PoundMass.
	Ounce
	// Newton is the SI force unit.
	Newton
	// PoundForce is 4.4482216152605 N.
	PoundForce
	// Meter is the SI length unit.
	Meter
	// Millimeter is 1e-3 m.
	Millimeter
	// Centimeter is 1e-2 m.
	Centimeter
	// Inch is 0.0254 m.
	Inch
	// Foot is 0.3048 m.
	Foot
	// Kilometer is 1e3 m.
	Kilometer
	// NauticalMile is 1852 m.
	NauticalMile
	// SquareMeter is the SI area unit.
	SquareMeter
	// SquareCentimeter is 1e-4 m².
	SquareCentimeter
	// SquareDecimeter is 1e-2 m², the metric RC wing-area unit.
	SquareDecimeter
	// SquareInch is 0.00064516 m².
	SquareInch
	// SquareFoot is 0.09290304 m².
	SquareFoot
	// MeterPerSecond is the SI speed unit.
	MeterPerSecond
	// KilometerPerHour is 1/3.6 m/s.
	KilometerPerHour
	// Knot is 1852 m per hour.
	Knot
	// MilePerHour is 1609.344 m per hour.
	MilePerHour
	// FootPerSecond is 0.3048 m/s.
	FootPerSecond
	// KilogramPerCubicMeter is the SI density unit.
	KilogramPerCubicMeter
	// SlugPerCubicFoot is the customary density unit the primary reference's
	// worked examples are quoted in.
	SlugPerCubicFoot
	// NewtonPerSquareMeter is the SI unit of force-based wing loading.
	NewtonPerSquareMeter
	// PoundForcePerSquareFoot is the customary force-based wing loading unit.
	PoundForcePerSquareFoot
	// KilogramPerSquareMeter is the SI unit of mass-based wing loading.
	KilogramPerSquareMeter
	// GramPerSquareDecimeter is the metric RC mass-based wing loading unit.
	GramPerSquareDecimeter
	// OuncePerSquareFoot is the customary RC mass-based wing loading unit.
	OuncePerSquareFoot
	// Radian is the SI angle unit.
	Radian
	// Degree is pi/180 radians.
	Degree
	// Watt is the SI power unit.
	Watt
	// Kilowatt is 1e3 W.
	Kilowatt
	// Horsepower is the mechanical horsepower, 745.6998715822702 W. It is here
	// so a book example quoted in horsepower can be checked in its original
	// units; nothing in this package produces one.
	Horsepower
	// Joule is the SI energy unit.
	Joule
	// WattHour is 3600 J.
	WattHour
	// KilowattHour is 3.6e6 J.
	KilowattHour
	// PascalSecond is the SI unit of dynamic viscosity.
	PascalSecond
	// MicropascalSecond is 1e-6 Pa*s, the magnitude air viscosity is usually
	// tabulated in.
	MicropascalSecond
	// KilogramMeter is the SI unit of a mass moment about a datum.
	KilogramMeter
	// GramMillimeter is 1e-6 kg*m, the magnitude a small RC component's moment
	// falls in when its mass is quoted in grams and its arm in millimetres.
	GramMillimeter
	// Second is the SI time unit.
	Second
	// Minute is 60 s.
	Minute
	// Hour is 3600 s.
	Hour
	// Ampere is the SI current unit.
	Ampere
	// Milliampere is 1e-3 A.
	Milliampere
	// Volt is the SI electric potential unit.
	Volt
	// Millivolt is 1e-3 V.
	Millivolt
	// Coulomb is the SI charge unit.
	Coulomb
	// AmpereHour is 3600 C.
	AmpereHour
	// MilliampereHour is 3.6 C, the unit an RC battery pack is labelled in.
	MilliampereHour
	// RevolutionPerSecond is the SI-consistent rotation rate this package holds.
	RevolutionPerSecond
	// RevolutionPerMinute is 1/60 rev/s, the unit a motor and propeller are
	// quoted in.
	RevolutionPerMinute
)

// unitDef carries a unit's symbol, dimension, and the exact factor that
// converts a value in the unit to SI by a single multiplication.
type unitDef struct {
	symbol string
	dim    Dimension
	factor float64
}

var unitTable = [...]unitDef{
	UnitInvalid: {symbol: "", dim: Dimensionless, factor: 0},

	One: {symbol: "1", dim: Dimensionless, factor: 1},

	Kilogram:  {symbol: "kg", dim: DimMass, factor: 1},
	Gram:      {symbol: "g", dim: DimMass, factor: 1e-3},
	PoundMass: {symbol: "lb", dim: DimMass, factor: 0.45359237},
	Ounce:     {symbol: "oz", dim: DimMass, factor: 0.45359237 / 16},

	Newton:     {symbol: "N", dim: DimForce, factor: 1},
	PoundForce: {symbol: "lbf", dim: DimForce, factor: 4.4482216152605},

	Meter:        {symbol: "m", dim: DimLength, factor: 1},
	Millimeter:   {symbol: "mm", dim: DimLength, factor: 1e-3},
	Centimeter:   {symbol: "cm", dim: DimLength, factor: 1e-2},
	Inch:         {symbol: "in", dim: DimLength, factor: 0.0254},
	Foot:         {symbol: "ft", dim: DimLength, factor: 0.3048},
	Kilometer:    {symbol: "km", dim: DimLength, factor: 1e3},
	NauticalMile: {symbol: "NM", dim: DimLength, factor: 1852},

	SquareMeter:      {symbol: "m^2", dim: DimArea, factor: 1},
	SquareCentimeter: {symbol: "cm^2", dim: DimArea, factor: 1e-4},
	SquareDecimeter:  {symbol: "dm^2", dim: DimArea, factor: 1e-2},
	SquareInch:       {symbol: "in^2", dim: DimArea, factor: 0.0254 * 0.0254},
	SquareFoot:       {symbol: "ft^2", dim: DimArea, factor: 0.3048 * 0.3048},

	MeterPerSecond:   {symbol: "m/s", dim: DimSpeed, factor: 1},
	KilometerPerHour: {symbol: "km/h", dim: DimSpeed, factor: 1000.0 / 3600.0},
	Knot:             {symbol: "kn", dim: DimSpeed, factor: 1852.0 / 3600.0},
	MilePerHour:      {symbol: "mph", dim: DimSpeed, factor: 1609.344 / 3600.0},
	FootPerSecond:    {symbol: "ft/s", dim: DimSpeed, factor: 0.3048},

	KilogramPerCubicMeter: {symbol: "kg/m^3", dim: DimDensity, factor: 1},
	// One slug is 1 lbf*s^2/ft, so the factor is written from the pound force
	// and the foot rather than as a rounded decimal.
	SlugPerCubicFoot: {
		symbol: "slug/ft^3",
		dim:    DimDensity,
		factor: (4.4482216152605 / 0.3048) / (0.3048 * 0.3048 * 0.3048),
	},

	NewtonPerSquareMeter:    {symbol: "N/m^2", dim: DimForcePerArea, factor: 1},
	PoundForcePerSquareFoot: {symbol: "lbf/ft^2", dim: DimForcePerArea, factor: 4.4482216152605 / (0.3048 * 0.3048)},

	KilogramPerSquareMeter: {symbol: "kg/m^2", dim: DimMassPerArea, factor: 1},
	GramPerSquareDecimeter: {symbol: "g/dm^2", dim: DimMassPerArea, factor: 1e-3 / 1e-2},
	OuncePerSquareFoot:     {symbol: "oz/ft^2", dim: DimMassPerArea, factor: (0.45359237 / 16) / (0.3048 * 0.3048)},

	Radian: {symbol: "rad", dim: DimAngle, factor: 1},
	Degree: {symbol: "deg", dim: DimAngle, factor: math.Pi / 180},

	Watt:       {symbol: "W", dim: DimPower, factor: 1},
	Kilowatt:   {symbol: "kW", dim: DimPower, factor: 1e3},
	Horsepower: {symbol: "hp", dim: DimPower, factor: 745.6998715822702},

	Joule:        {symbol: "J", dim: DimEnergy, factor: 1},
	WattHour:     {symbol: "Wh", dim: DimEnergy, factor: 3600},
	KilowattHour: {symbol: "kWh", dim: DimEnergy, factor: 3.6e6},

	Second: {symbol: "s", dim: DimTime, factor: 1},
	Minute: {symbol: "min", dim: DimTime, factor: 60},
	Hour:   {symbol: "h", dim: DimTime, factor: 3600},

	Ampere:      {symbol: "A", dim: DimCurrent, factor: 1},
	Milliampere: {symbol: "mA", dim: DimCurrent, factor: 1e-3},

	Volt:      {symbol: "V", dim: DimVoltage, factor: 1},
	Millivolt: {symbol: "mV", dim: DimVoltage, factor: 1e-3},

	Coulomb:         {symbol: "C", dim: DimCharge, factor: 1},
	AmpereHour:      {symbol: "Ah", dim: DimCharge, factor: 3600},
	MilliampereHour: {symbol: "mAh", dim: DimCharge, factor: 3.6},

	RevolutionPerSecond: {symbol: "rev/s", dim: DimRotationRate, factor: 1},
	RevolutionPerMinute: {symbol: "rpm", dim: DimRotationRate, factor: 1.0 / 60.0},

	PascalSecond:      {symbol: "Pa*s", dim: DimDynamicViscosity, factor: 1},
	MicropascalSecond: {symbol: "uPa*s", dim: DimDynamicViscosity, factor: 1e-6},

	KilogramMeter:  {symbol: "kg*m", dim: DimMassMoment, factor: 1},
	GramMillimeter: {symbol: "g*mm", dim: DimMassMoment, factor: 1e-3 * 1e-3},
}

// dimensionSIUnits names the unit each dimension is stored in. It is the
// inverse of the unitTable entries whose factor is exactly 1.
var dimensionSIUnits = [...]Unit{
	Dimensionless:       One,
	DimMass:             Kilogram,
	DimForce:            Newton,
	DimLength:           Meter,
	DimArea:             SquareMeter,
	DimSpeed:            MeterPerSecond,
	DimDensity:          KilogramPerCubicMeter,
	DimForcePerArea:     NewtonPerSquareMeter,
	DimMassPerArea:      KilogramPerSquareMeter,
	DimAngle:            Radian,
	DimPower:            Watt,
	DimEnergy:           Joule,
	DimDynamicViscosity: PascalSecond,
	DimMassMoment:       KilogramMeter,
	DimTime:             Second,
	DimCurrent:          Ampere,
	DimVoltage:          Volt,
	DimCharge:           Coulomb,
	DimRotationRate:     RevolutionPerSecond,
}

// ErrUnknownUnit reports a Unit outside the supported table, including the zero
// Unit.
var ErrUnknownUnit = errors.New("unknown unit")

// Units returns every supported unit, in table order. It exists so that a
// transport or a CAD adapter can list what a builder may enter without keeping
// a second copy of the unit table, which is exactly how a mistyped conversion
// factor gets into a system twice.
func Units() []Unit {
	units := make([]Unit, 0, len(unitTable)-1)
	for u := UnitInvalid + 1; int(u) < len(unitTable); u++ {
		units = append(units, u)
	}
	return units
}

// ParseUnit returns the unit with the given symbol. Symbols are the printed
// forms in the unit table, so "m/s", "dm^2" and "oz/ft^2" all resolve, and an
// unrecognised symbol is refused rather than defaulting to an SI unit.
func ParseUnit(symbol string) (Unit, error) {
	if symbol == "" {
		return UnitInvalid, ErrUnknownUnit
	}
	for u := UnitInvalid + 1; int(u) < len(unitTable); u++ {
		if unitTable[u].symbol == symbol {
			return u, nil
		}
	}
	return UnitInvalid, ErrUnknownUnit
}

func lookupUnit(u Unit) (unitDef, error) {
	if u == UnitInvalid || int(u) >= len(unitTable) {
		return unitDef{}, ErrUnknownUnit
	}
	return unitTable[u], nil
}

// Symbol returns the unit's printed symbol, or the empty string if the unit is
// not supported.
func (u Unit) Symbol() string {
	def, err := lookupUnit(u)
	if err != nil {
		return ""
	}
	return def.symbol
}

// FactorToSI returns the exact factor that converts a value in this unit to the
// dimension's SI unit by a single multiplication, or zero for an unsupported
// unit.
//
// It exists so that a presentation layer outside this package can render a
// stored SI value in the unit a builder chose without keeping its own copy of
// the conversion table. Converting for display is formatting; converting an
// input is this package's job, through NewQuantity.
func (u Unit) FactorToSI() float64 {
	def, err := lookupUnit(u)
	if err != nil {
		return 0
	}
	return def.factor
}

// Dimension returns the unit's physical dimension. An unsupported unit reports
// Dimensionless, so callers must check support with a constructor.
func (u Unit) Dimension() Dimension {
	def, err := lookupUnit(u)
	if err != nil {
		return Dimensionless
	}
	return def.dim
}

// Quantity is a finite physical value held in SI units. Quantity is an
// immutable value type: every operation returns a new Quantity and no method
// mutates its receiver or any shared definition.
type Quantity struct {
	si  float64
	dim Dimension
}

// NewQuantity converts value, expressed in unit u, into a Quantity. It rejects
// unsupported units, non-finite input, and conversions that overflow float64.
func NewQuantity(value float64, u Unit) (Quantity, error) {
	def, err := lookupUnit(u)
	if err != nil {
		return Quantity{}, err
	}
	if !isFinite(value) {
		return Quantity{}, errors.New("value " + formatFloat(value) + " " + def.symbol + " is not finite")
	}
	si := value * def.factor
	if !isFinite(si) {
		return Quantity{}, errors.New("value " + formatFloat(value) + " " + def.symbol + " overflows the SI range")
	}
	if si == 0 && value != 0 {
		return Quantity{}, errors.New("value " + formatFloat(value) + " " + def.symbol + " underflows to zero in SI")
	}
	return Quantity{si: si, dim: def.dim}, nil
}

// In returns the quantity expressed in unit u. It fails when u belongs to a
// different dimension, which is what stops mass from being read as a weight.
func (q Quantity) In(u Unit) (float64, error) {
	def, err := lookupUnit(u)
	if err != nil {
		return 0, err
	}
	if def.dim != q.dim {
		return 0, errors.New("cannot express a " + q.dim.String() + " quantity in " + def.symbol)
	}
	return q.si / def.factor, nil
}

// Dimension returns the quantity's dimension.
func (q Quantity) Dimension() Dimension { return q.dim }

// SI returns the underlying value in the dimension's SI unit.
func (q Quantity) SI() float64 { return q.si }

// IsZero reports whether the quantity is exactly zero.
func (q Quantity) IsZero() bool { return q.si == 0 }

// supplied reports whether a field holding this Quantity was filled in at all.
// The zero Quantity is dimensionless zero, which no constructor can produce for
// a dimensional field, so it is unambiguously "not entered yet". A supplied zero
// carries its dimension and is therefore a value the caller must justify.
func (q Quantity) supplied() bool { return q != Quantity{} }

// String renders the quantity in its SI unit at full float64 precision. It is
// a debugging aid, not a display format: presentation rounding belongs to the
// caller so that tolerances stay independent of display.
func (q Quantity) String() string {
	if q.dim == Dimensionless {
		// A dimensionless value would otherwise read as "1.2 1", the trailing
		// symbol looking like part of the number.
		return formatFloat(q.si)
	}
	return formatFloat(q.si) + " " + q.dim.String()
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}
