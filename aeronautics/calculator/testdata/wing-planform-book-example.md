# Fixture — CODE Lab Wing Planform Sizing worked example

Source: [Wing Planform Sizing](https://computationaldesignlab.github.io/aircraft-design/wing_layout.html),
CODE Lab Aircraft Design, read 2026-09-05. No upstream commit revision is
published on the page. Its exposed-area figure comes from the
[Lift chapter](https://computationaldesignlab.github.io/aircraft-design/aerodynamics/lift_curve.html),
read the same day, which describes the same aircraft.

The example is a manned light aircraft in US customary units. Its numbers are
used here to check the ported relations. **They are not RC defaults**, and no
value below is a default anywhere in the calculator.

## Inputs, in the chapter's own units

| Symbol | Value | Note |
|---|---|---|
| `A` | 8 | aspect ratio |
| `S` | 134 ft² | reference area, trapezoid through the centerline |
| `lambda` | 0.4 | taper ratio |
| `Lambda_c/4` | 0° | quarter-chord sweep |
| `Gamma` | 5° | dihedral, chosen from historical comparison |
| twist | −3° | washout |
| incidence | 2° | root |
| sections | NACA 23018 root, NACA 23009 tip | `t/c` 0.18 and 0.09 |
| `d` | 5 ft | body width at the wing, from the Lift chapter |

## Expected values

Computed independently at 45 significant digits with mpmath from the chapter's
relations, **not** read back from its displayed output. The Go test compares
against the full-precision column; the displayed column is a separate, looser
check that the port lands where the chapter says it does.

| Quantity | Independently calculated | Chapter displays |
|---|---|---|
| span `b` | 32.74141108748979987981481 ft | 33 ft |
| root chord | 5.846680551337464264252646 ft | 5.8 ft |
| tip chord | 2.338672220534985705701058 ft | 2.3 ft |
| MAC | 4.343248409564973453444822 ft | 4.3 ft |
| `y_MAC` | 7.016016661604957117103175 ft | 7.0 ft |
| leading-edge sweep | 3.066485501125893377539803° | 3.1° |
| geometric mean chord `S/b` | 4.092676385936224984976852 ft | not reported |
| MAC leading-edge station | 0.3758580354431227027019558 ft | not reported |
| exposed area | 106.1058829575983929644511 ft² | 106 ft² (Lift chapter) |

Tolerances in the tests are an absolute term in the quantity's own unit plus a
relative term. The absolute terms for the displayed column come from the number
of digits the chapter shows — the span is displayed as a whole number, so that
check is deliberately loose and the full-precision check is the tight one.

## Discrepancies and interpretation

- **Span display.** The chapter displays 33 ft for a value of 32.741 ft. This is
  display rounding, not a different relation. Every derived value here is
  computed from the unrounded span; using 33 ft as an intermediate would move the
  root chord by about 0.05 ft.
- **Exposed area.** The Wing Planform Sizing chapter carries the reference
  trapezoid through the centerline and does not report an exposed area. The Lift
  chapter quotes `S_exposed = 106 ft²` for the same wing behind a 5 ft body.
  Integrating the linear chord distribution across the body exactly gives
  106.106 ft², which agrees at the precision quoted. The integration is this
  project's extension, recorded as derived rather than as a book method.
- **Not implemented.** The chapter's Torenbeek wing fuel-volume relation is
  deliberately absent: an electric RC wing carries no fuel. Its airfoil, twist,
  incidence and dihedral selections are example choices, and this package stores
  them as stated inputs rather than deriving or recommending them.

## SI equivalence

The Go test solves the same wing a second time with the area converted to m² by
the unit table, and requires the two solutions to agree to 1e-12 relative. The
conversion goes through `0.3048²` in the unit table rather than through any
rounded intermediate, so the check would catch a mistyped area factor.

## Independent synthetic fixture

The Reynolds check is not from the book: at `rho = 1.225 kg/m³`, `V = 15 m/s`,
`c = 0.2 m` and `mu = 1.7894e-5 Pa·s`, `Re = 205376.1037219179613278194`,
computed at the same precision. The viscosity is the ISA sea-level value, which
Sutherland's relation at 288.15 K gives as 1.78919e-5 Pa·s.
