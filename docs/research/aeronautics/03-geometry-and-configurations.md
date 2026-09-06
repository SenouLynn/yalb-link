# Task 03 — Geometry, configurations, and aerodynamic context

**Outcome:** independent geometry contracts that support conventional aircraft,
V-tails and flying wings without conflating their handling models.
Dependency: [02](02-equations-and-lift.md).

## Work

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
guarantee. [Geometry equations](https://computationaldesignlab.github.io/aircraft-design/tail_sizing.html),
[Reynolds context](https://www1.grc.nasa.gov/beginners-guide-to-aeronautics/similarity-parameters/).

## Acceptance checks

- The rectangle limit of the trapezoid agrees; MAC differs from `S/b` under taper.
- Redundant vs conflicting driver combinations are distinguishable; limits are
  not silently consumed as equality inputs.
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
and [XFLR5 limitations](https://www.xflr5.tech/docs/Part%20IV:%20Limitations.pdf).
