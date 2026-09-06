# Task 03 — Geometry, configurations, and aerodynamic context

**Outcome:** independent geometry contracts that support conventional aircraft,
V-tails and flying wings without conflating their handling models.
Dependency: [02](02-equations-and-lift.md).

## Work

- Use the book's [Wing Planform Sizing](https://computationaldesignlab.github.io/aircraft-design/wing_layout.html)
  under the [handoff's source rules](README.md). Reproduce its geometry example;
  keep example taper, twist, incidence and airfoil selections as example inputs.
  Local root/tip Reynolds coverage is an explicit extension of its MAC-based check.
- Represent aircraft configuration explicitly from the start. All three use wing
  sizing; conventional-tail, V-tail and flying-wing handling paths differ.
- Support a rectangle and symmetric trapezoid initially. Give each supported
  driver combination a named solve path. Keep wing shape, airfoil shape, sweep,
  twist and dihedral as distinct choices.
- Define projected span/reference area and the treatment of the centerline
  planform through the fuselage. Distinguish physical panel construction lengths
  from projected dimensions; store reference conventions for imported data.
- Define root/tip chord, geometric mean chord and MAC separately. Store enough
  geometry/datum information to locate MAC and eventually CG relative to it.
- Add Reynolds calculations for local chords at selected speeds. Record airfoil
  identity and evidence metadata; an actual polar importer may follow later.
- Define stable, transport-neutral parameter keys and an explicit dependency graph
  for sketch geometry: full/half span, root/tip chord, taper, leading-edge offsets,
  sweep reference, dihedral and component stations as supported. Record units,
  datums, driver/derived roles and the equation revision behind each relationship.
  Task 11 maps these to Fusion names/expressions; CAD syntax stays outside the core.
- Supply enough coordinates and construction relationships to reconstruct the
  supported planform from its active drivers. Distinguish plan-view projected
  dimensions from physical panel/sketch-plane dimensions. A reference MAC or CG
  marker is not a manufacturing edge, and a planform does not define an airfoil
  section or complete 3D wing loft.

## Equation contracts

```text
Rectangle: S=b c; AR=b²/S        # two independent size values
Trapezoid: lambda=c_tip/c_root
b=sqrt(AR S)
c_root=2 S/[b(1+lambda)]; c_tip=lambda c_root
c_mean=S/b
MAC=(2/3)c_root(1+lambda+lambda²)/(1+lambda)
y_MAC=(b/6)(1+2 lambda)/(1+lambda)
Re(y)=rho V c(y)/mu
```

MAC longitudinal placement needs a sweep/datum convention. For a simple uniform
dihedral wing, `b_projected=b_flat cos(Gamma)` and
`S_projected=S_flat cos(Gamma)`. Explicitly choose which dimensions stay fixed as
dihedral changes. Do not model dihedral as a universal lift penalty or self-leveling
guarantee. [Geometry equations](https://computationaldesignlab.github.io/aircraft-design/wing_layout.html),
[supplementary Reynolds context](https://www1.grc.nasa.gov/beginners-guide-to-aeronautics/similarity-parameters/).

## Acceptance checks

- The book's `AR=8`, `S=134 ft²`, `lambda=.4` wing example reproduces its
  span/chord/MAC geometry to stated display precision; original-unit and SI
  evaluations agree without using the book's rounded outputs as intermediates.
- The rectangle limit of the trapezoid agrees; MAC differs from `S/b` under taper.
- Redundant vs conflicting driver combinations are distinguishable; limits are
  not silently consumed as equality inputs.
- Reconstruct a rectangle and tapered wing from the named parameters and datum;
  coordinates reproduce the solved span, area and chords. Changing solve mode
  gives an acyclic dependency graph with the new driver roles. Projected and
  physical dimensions remain distinguishable under nonzero dihedral.
- At `rho=1.225`, `V=15 m/s`, `c=.2 m`, `mu=1.7894e-5 Pa·s`,
  Reynolds number is approximately `205376.1037`.
- Root/tip Reynolds checks cannot be replaced by MAC-only coverage.
- Dihedral changes give correct projected/physical behavior for the chosen mode.
- Flying wings never require a horizontal-tail input. V-tail geometry records
  panel area/cant, not imaginary independent horizontal and vertical tails.
- Unsupported geometry is reported as unsupported rather than physically
  impossible. Do not reject valid negative incidence/twist or reverse taper
  merely because they are unusual.

**Later evidence integration:** preserve Reynolds/Mach/configuration, transition
settings, source and convergence information. No silent polar extrapolation or
interpolation across failed samples. See [XFOIL](https://web.mit.edu/drela/Public/web/xfoil/)
and [XFLR5 limitations](https://flow5.tech/xflr5/docs/Part%20IV:%20Limitations.pdf).
