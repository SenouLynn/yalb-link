# Engineering method coverage

Primary reference: [CODE Lab Aircraft Design](https://computationaldesignlab.github.io/aircraft-design/intro.html),
accessed 2026-09-05. The introduction identifies this as CODE Lab's Python-based
conceptual design material for Purdue's AAE 451 course. No upstream commit revision
has been verified. Python is reference material, not a runtime dependency.

## What the Lift chapter actually contains

The [Lift chapter](https://computationaldesignlab.github.io/aircraft-design/aerodynamics/lift_curve.html)
was read on 2026-09-05. Recorded here because the reading changed the source
attribution of the implemented equations:

- It gives the linear-region lift coefficient `C_L = C_Lα (α − α_{L=0})`, a
  semi-empirical lift-curve slope, the maximum lift coefficient
  `C_Lmax = c_lmax · (C_Lmax/c_lmax) + ΔC_Lmax`, and the angle at `C_Lmax`.
- It **explicitly distinguishes** section coefficients (lowercase `c_l`) from
  aircraft coefficients (uppercase `C_L`).
- It contains **no stall-speed equation** and no inversion of the lift identity.

Its worked example uses A=8, M=0.3, b=33 ft, d=5 ft, S_ref=134 ft²,
S_exposed=106 ft², η=1.0, c_lmax=1.88, ratio=0.9, ΔC_Lmax=−0.25 and displays
C_Lα=5.0 rad⁻¹, C_Lmax=1.44, α at C_Lmax=18.1°. Checking the CLmax relation
independently: 1.88 × 0.9 − 0.25 = **1.442**, displayed as 1.44. The difference is
display rounding, not a discrepancy in the relation. The example is a manned
configuration in customary units and its constants are example inputs, not RC
defaults.

**Consequence for attribution.** The stall-speed family implemented in Task 02 is
*not* from this chapter. Those equations cite the standard lift identity
(supplementary) and their own algebraic derivation. The chapter's contribution is
*applicability*: it is the basis for refusing a 2D section `c_lmax` where a
whole-aircraft `C_Lmax` is required. `TestLiftSubsetDoesNotClaimBookProvenance`
keeps that attribution from drifting upward.

## What the Wing Planform Sizing chapter actually contains

The [Wing Planform Sizing chapter](https://computationaldesignlab.github.io/aircraft-design/wing_layout.html)
was read on 2026-09-05. Unlike the Lift chapter, it **does** contain the
relations implemented from it, so those are the first equations in this package
to carry `SourceBook`:

- taper ratio as tip over root chord, `c_root = 2S/(b(1+lambda))` and
  `c_tip = lambda c_root`;
- `MAC = (2/3) c_root (1+lambda+lambda^2)/(1+lambda)` and
  `y_mac = b(1+2 lambda)/(6(1+lambda))`;
- the sweep transform `tan(Lambda_n) = tan(Lambda_m) - (4/A)[(n-m)(1-lambda)/(1+lambda)]`;
- the aspect-ratio definition, which its worked example uses to compute the span
  from `A` and `S`. The chapter shows the computed span rather than writing
  `b = sqrt(A S)` out; the relation is that definition rearranged, and it is
  attributed to the chapter because the chapter performs that calculation.
- a Torenbeek wing fuel-volume relation, **not implemented**: an electric RC wing
  carries no fuel.
- the statement that Reynolds number for airfoil analysis should be computed with
  the mean aerodynamic chord.

Its example wing is a manned light aircraft in customary units. Its dihedral,
twist, incidence and NACA 23018/23009 selections are example choices with
historical-comparison rationale, not RC defaults, and none of them is a default
here. The worked example is reproduced in
[calculator/testdata/wing-planform-book-example.md](../../calculator/testdata/wing-planform-book-example.md),
which also records the display-rounding differences.

`TestBookProvenanceIsLimitedToCheckedChapterMethods` holds an explicit list of
the equations allowed to claim the book, and fails in both directions: a new
equation cannot quietly acquire the attribution, and an existing one cannot
quietly lose it. `TestLiftSubsetDoesNotClaimBookProvenance` keeps the Task 02
family off it entirely.

## What the Matching process chapter actually contains

The [Matching process chapter](https://computationaldesignlab.github.io/aircraft-design/constraint_analysis/final.html)
was read on 2026-09-05, before Task 04 was implemented. Like the Lift chapter,
the reading changed what the implementation may claim:

- It plots **four** constraints against wing loading: takeoff distance (a
  quadratic in the takeoff parameter, `a = 0.009`, `b = 4.9`, for a 1500 ft
  requirement), landing distance, one-engine-inoperative climb gradient at
  5000 ft against a 0.015 gradient, and cruise speed at 8000 ft at an 80% power
  setting.
- Its axes are `W/S` against `W/P`, **power** loading for a piston engine, not
  the electric propulsion this project targets.
- It reduces the constraints with a logical AND to shade a feasible region, then
  selects a design point **by visual inspection** inside it, at `W/S = 40 lb/ft²`
  and `W/P = 9.25 lb/hp`.
- It contains **no independent stall-speed constraint.** Stall speed appears only
  inside the landing-distance relation, as `v_sL = (s_lgr/0.265)^0.5` in knots.

**Consequence for attribution.** Task 04 implements no equation from this
chapter, and adds no equation to the registry at all: its bounds come from the
Task 02 stall-speed inversions, which are already recorded above as derived. All
four of the chapter's constraints need a propulsion model, a drag polar and
empirical constants that do not exist in this package, and its example values are
inputs for a manned piston aircraft, not RC defaults.

What the chapter does contribute is the *shape* of the workflow, and that is
recorded as methodology rather than as a ported equation: constraints are
intersected rather than averaged, the binding one is identified, and the design
point is **chosen** by the builder inside the feasible region rather than solved
for. `SizeAtStallLimit` follows exactly that: it reports a bound, and only sits
on it when the builder asks it to. The one deliberate departure is that this
package names the controlling case numerically instead of leaving the reader to
find it by eye on a plot.

A stall-only subset is therefore **not** the chapter's matching plot, and nothing
in this package presents it as one. `TestBookProvenanceIsLimitedToCheckedChapterMethods`
still holds the same seven geometry equations and no others.

## What the Center of gravity chapter actually contains

The [Center of gravity chapter](https://computationaldesignlab.github.io/aircraft-design/weight_and_balance/cg.html),
under Weight and Balance, was read on 2026-09-06 for Task 07. Unlike the Lift
and Matching process chapters, it **does** contain the relation implemented from
it, so the Task 07 mass family is the second set in this package to carry
`SourceBook`:

- `x_CG = sum(x_CG_k W_k) / sum(W_k)`, a weighted mean over component **weights**;
- the statement that "a similar equation can be used for y and z axis", with only
  the `x` axis demonstrated;
- a worked example: eight components totalling 3114 lb and 44214.7 lb·ft about a
  nose datum at `x = 3.4 ft`, giving `x = 14.2 ft`, which against a MAC of 4.3 ft
  with its leading edge at 13.85 ft is displayed as 8% MAC.

**Adaptation.** This package holds masses rather than weights. Under one uniform
standard gravity the `g` cancels between the numerator and the denominator, so
`sum(m_k x_k)/sum(m_k)` gives the same station; that is the only adaptation, and
it is recorded on the equation's own `Source.Adaptation` as well as here. All
three axes are evaluated with the relation the chapter sanctions for them. The
chapter's datum is the aircraft nose; this package measures from the reference
planform's root leading edge, which is the datum its wing geometry already uses,
so a centre of gravity can be compared with the mean aerodynamic chord without a
transform nothing here implements.

The worked example is reproduced in
[calculator/testdata/cg-book-example.md](../../calculator/testdata/cg-book-example.md)
and checked in SI and in the chapter's own units. Its component masses are a
manned twin's and none of them is a default here.

`mass.station-fraction-of-mac` is recorded as **derived**, not book: the chapter
states no chord-fraction relation, and `TestMACFractionIsDerivedRatherThanBookSourced`
holds that. Expressing a station as a fraction of the MAC is a geometric
reference and is not a static margin.

## What the Trade study chapter actually contains

The [Aspect ratio study](https://computationaldesignlab.github.io/aircraft-design/trade_study/ar_study.html),
under Trade study, was read on 2026-09-06 before Task 07 was implemented. As
with the Matching process chapter, the reading limited what the implementation
may claim:

- It evaluates **five discrete aspect ratios** (7 to 11) and, for each, an 80×80
  grid over wing loading `W/S` from 30 to 60 lb/ft² and power loading `W/P` from
  6 to 12 lb/hp.
- Each grid is contoured with MTOW isolines and the takeoff, landing, climb-
  gradient, cruise-speed and fuel-volume constraint boundaries, and the feasible
  region is shaded.
- It selects `A = 9` as the aspect ratio giving the lowest MTOW, at 6258 lb with
  `W/S = 45.3 lb/ft²` and `W/P = 8.7 lb/hp`.

**Consequence for attribution.** Task 07 implements no equation from this chapter
and adds none to the registry from it. Its axes are power loading for a piston
engine, its constraints need a propulsion model, a drag polar and a weight-
estimation method that do not exist here, and its MTOW isolines need the fuel-
fraction mission analysis this project has explicitly deferred. A one-driver
sweep over an implemented output is **not** this chapter's trade study, and
nothing in the worksheet presents it as one.

What the chapter does contribute is methodology, recorded as such: a design
parameter is moved across a bounded range, every candidate is evaluated by the
same method, the constraints are drawn against the result rather than applied
silently, and the builder chooses. Task 07's sweep follows exactly that, over
the outputs the implemented models actually produce. The wing-loading and
power-loading plots wait for Task 09.

## Implemented methods

All are in `yalb.aero/calculator`, each with an equation ID, revision, expression,
declared input/output ports, source record and assumptions. SI is internal.

| Equation ID | Expression | Source kind |
|---|---|---|
| `weight.from-mass` | `W = m g` | supplementary |
| `aero.dynamic-pressure` | `q = rho V² / 2` | supplementary |
| `lift.required` | `L_required = n W` | supplementary |
| `lift.required-coefficient` | `CL_required = n m g / (q S)` | supplementary |
| `wing-loading.force` | `W/S = m g / S` | supplementary |
| `wing-loading.mass` | `m/S` | supplementary |
| `lift.stall-speed` | `Vs = sqrt(2 n m g / (rho S CLmax))` | supplementary |
| `lift.minimum-wing-area` | `S_min = 2 n m g / (rho Vs_limit² CLmax)` | derived |
| `lift.maximum-mass` | `m_max = rho Vs_limit² S CLmax / (2 n g)` | derived |
| `lift.maximum-wing-loading` | `(W/S)_max = rho Vs_limit² CLmax / (2 n)` | derived |
| `geometry.aspect-ratio` | `A = b²/S` | book |
| `geometry.span-from-area-aspect` | `b = sqrt(A S)` | book |
| `geometry.root-chord` | `c_root = 2S/(b(1+lambda))` | book |
| `geometry.tip-chord` | `c_tip = lambda c_root` | book |
| `geometry.mac` | `MAC = (2/3) c_root (1+lambda+lambda²)/(1+lambda)` | book |
| `geometry.y-mac` | `y_mac = b(1+2 lambda)/(6(1+lambda))` | book |
| `geometry.sweep-transform` | `tan(Lambda_n) = tan(Lambda_m) − (4/A)[(n−m)(1−lambda)/(1+lambda)]` | book |
| `geometry.area-from-span-aspect` | `S = b²/A` | derived |
| `geometry.span-from-area-root-chord` | `b = 2S/(c_root(1+lambda))` | derived |
| `geometry.area-from-span-root-chord` | `S = b c_root (1+lambda)/2` | derived |
| `geometry.span-from-aspect-root-chord` | `b = A c_root (1+lambda)/2` | derived |
| `geometry.mean-chord` | `c_mean = S/b` | derived |
| `geometry.semi-span` | `b_half = b/2` | derived |
| `geometry.chord-at-station` | `c(y) = c_root(1 − (1−lambda) 2y/b)` | derived |
| `geometry.tip-leading-edge-offset` | `x_le_tip = (b/2) tan(Lambda_le)` | derived |
| `geometry.tip-rise` | `z_tip = (b/2) tan(Gamma)` | derived |
| `geometry.mac-leading-edge-station` | `x_le_mac = y_mac tan(Lambda_le)` | derived |
| `geometry.exposed-area` | `S_exposed = S − d(c_root + c(d/2))/2` | derived |
| `geometry.projected-span` | `b_projected = b_panel cos(Gamma)` | derived |
| `geometry.projected-area` | `S_projected = S_panel cos(Gamma)` | derived |
| `geometry.panel-span` | `b_panel = b_projected / cos(Gamma)` | derived |
| `geometry.panel-area` | `S_panel = S_projected / cos(Gamma)` | derived |
| `aero.reynolds` | `Re = rho V c / mu` | supplementary |
| `mass.total` | `m = sum(m_i)` | book |
| `mass.component-moment` | `M_i = m_i r_i` | book |
| `mass.moment-sum` | `M = sum(M_i)` | book |
| `mass.center-of-gravity` | `r_cg = sum(m_i r_i) / sum(m_i)` | book |
| `mass.station-fraction-of-mac` | `fraction = (x − x_le_mac) / MAC` | derived |

The geometry entries marked derived are algebraic consequences of the chapter's
own relations — rearrangements for a different driver pair, linear interpolation
between the root and tip chords, or the plan-view projection of its dihedral
choice — and have no independent source. The chapter selects a dihedral angle but
states no projection relation, so `b_projected = b_panel cos(Gamma)` is recorded
as derived rather than as a book method. The exposed-area relation integrates the
linear chord distribution across the body exactly; it reproduces the Lift
chapter's `S_exposed = 106 ft²` for the same aircraft as 106.106 ft².

Second supplementary source: the
[NASA similarity parameters page](https://www1.grc.nasa.gov/beginners-guide-to-aeronautics/similarity-parameters/),
read 2026-09-05, which gives `Re = rho V L / mu` with `L` only as "some
characteristic length" and does not name the viscosity as dynamic. The
dimensional form requires dynamic viscosity, and choosing the local chord as the
length — and covering root, MAC and tip rather than the MAC alone — is this
project's adaptation, recorded in the equation's source.

Supplementary source: the
[NASA lift equation](https://www1.grc.nasa.gov/beginners-guide-to-aeronautics/lift-equation/),
used as `L = CL q S` with `q = rho V² / 2`, plus `W = m g` at standard gravity
9.80665 m/s². Derived entries are algebraic inversions of the stall-speed
relation and have no independent source.

This is a lumped lift model: the wing carries the whole load, with no separately
solved wing and tail trim loads. The mass ceiling is aerodynamic for the selected
case and is not a structural rating. A stall ceiling gives a lower bound on area;
it does not choose an area.

## Coverage

| Coverage | Current status |
|---|---|
| Supplied-aircraft-CLmax lift inversions | **Implemented** (Task 02) |
| Aircraft lift-curve slope and CLmax estimation | Deferred; the book method is recorded above. Task 03 supplies the geometry it needs, but no airfoil polar evidence exists yet, so nothing estimates a coefficient |
| Rectangle and symmetric-trapezoid planform, MAC and reference dimensions | **Implemented** (Task 03) |
| Plan-view versus panel-plane dimensions under uniform dihedral | **Implemented** (Task 03), with the held-fixed choice required from the builder |
| Local Reynolds conditions at root, MAC and tip | **Implemented** (Task 03) |
| Explicit configuration contracts for conventional, V-tail and flying-wing layouts | **Implemented as geometry only** (Task 03); no handling model for any of them |
| NACA 4/5-digit section generation and coordinate ingest | Deferred to Task 12; today only the section identity and its evidence are stored |
| Spanwise section lofting under taper and twist | Deferred to Task 12; requires an explicitly stated twist axis |
| Airfoil polar import, interpolation or extrapolation | Unsupported. Task 12 generates coordinates only: a generated section establishes geometry, never lift, drag, moment or a section clmax |
| Wing fuel volume (Torenbeek) | Not implemented: an electric RC wing carries no fuel |
| Kinked, cranked, elliptical or multi-panel planforms | Unsupported; reported as unsupported rather than solved approximately |
| Requirement intersection over required cases, controlling case and deliberate candidate selection | **Implemented as a stall-only subset** (Task 04). The book's four-constraint matching plot is not delivered by it and is not claimed |
| Discovery and evaluation over HTTP, with the source records preserved | **Implemented** (Task 05). An application task: it adds no equation and no solver decision, and `TestTransportDoesNotReimplementTheCore` fails on any arithmetic in the boundary packages |
| Takeoff, landing, OEI climb-gradient and cruise-speed constraints | Unsupported. Each needs a propulsion model, a drag polar and empirical constants this package does not have; they enter through the same case engine when Task 09 supplies them |
| Numerical solvers, convergence budgets and discrete component search | Unsupported. Task 04 treats feedback loops as explicit builder revisions; no iterate is produced, so none can be mislabelled converged |
| Component mass properties: total mass and mechanical centre of gravity on all three axes | **Implemented** (Task 07), against the chapter's own worked example. Missing mass or position makes the assessment incomplete rather than assuming an origin |
| Station as a fraction of the mean aerodynamic chord | **Implemented** (Task 07) as a geometric reference only. It is not a static margin and no handling conclusion follows from it |
| One-driver sensitivity sweep over an implemented output, with requirement boundaries and feasible/unknown regions | **Implemented** (Task 07) as a documented subset. The book's trade study and its matching plot need Task 09's power models and are not claimed |
| Dimensioned plan, front and side views with construction geometry, and the relationship behind each dimension | **Implemented** (Task 07). Fusion-ready expressions and units are Task 11's; until then the parameter table is a generic geometry handoff |
| Wing aerodynamic centre, aircraft neutral point, centre of pressure, static margin and trim | Unsupported. Task 07 draws none of them and names each as unknown; Task 08 owns the conventional minimum |
| Line of action, spanwise pressure distribution and separately solved wing and tail loads | Unsupported. Task 07 reports the lumped required lift as a magnitude only |
| Conventional static margin and trim | Deferred to Task 08 |
| Drag and electric propulsion/mission adaptations | Deferred to Task 09 |
| Carbon spar geometric fit under taper, twist and varying section | Deferred to Task 13 |
| Spanwise load distribution, bending moment, flexural rigidity and deflection | Deferred to Task 13, against supplied material evidence |
| Structural strength: allowable stress, margin of safety, failure, buckling, joints, bonds and fatigue | Unsupported. Task 13 reports stiffness and fit only, and refuses a strength result explicitly rather than omitting one |
| Torsional stiffness, aeroelastic twist, divergence and flutter | Unsupported |
| Takeoff/landing, fuselage, landing gear and cost | Unsupported |
| V-tail/flying-wing aerodynamic handling, dynamic and closed-loop stability | Unsupported |

No implemented method estimates a lift coefficient. `CLmax` is supplied by the
caller with an explicit scope and basis, and a section value is refused rather
than reinterpreted.

## Fixtures

Task 03's fixtures are the chapter's worked example, recorded in full precision
in [calculator/testdata/wing-planform-book-example.md](../../calculator/testdata/wing-planform-book-example.md),
plus the independent Reynolds fixture `Re = 205376.1037219179613278194` at
`rho = 1.225 kg/m³`, `V = 15 m/s`, `c = 0.2 m`, `mu = 1.7894e-5 Pa·s`. The
example's values were computed independently at 45 significant digits and are
never taken from the chapter's rounded display.

Task 07's Center of gravity fixture is the chapter's own worked example,
recorded in full precision in
[calculator/testdata/cg-book-example.md](../../calculator/testdata/cg-book-example.md)
and checked both in SI and against the `14.2 ft` the chapter displays. Its
placement fixtures are independent of the book and of this package: 1.5 kg of
airframe at `x = 0.4 m` with a 0.5 kg battery at `x = 0.2 m` totals 2 kg and
balances at `x = 0.35 m`; moving the battery to `x = 0.6 m` gives `x = 0.45 m`
with the loading unchanged; growing it to 1 kg gives 2.5 kg at `x = 0.48 m` and a
stall speed higher by `sqrt(2.5/2)`, which is Task 02's model responding to a
mass change rather than a new one.

Task 02's numerical fixtures are synthetic and independently calculated at 40
significant digits, not taken from the book: m=2 kg, rho=1.225 kg/m³, CLmax=1.2,
n=1, S=0.24 m². The CLmax is a fixture assumption and not a recommended default.
Tolerances are stated in each quantity's SI unit plus a relative term, chosen
from the precision the expected values are quoted to rather than from display
rounding. Task 02 reproduces no worked example because no book method is
implemented in that task; the book's example is reproduced by Task 03 above.

For each method added later, record the chapter/section URL, access date and
verified upstream revision when available, original notation and units,
assumptions, applicability, RC adaptations and equation/code/result
discrepancies. Pair a worked example in original units and SI with independent
numerical fixtures in `calculator/testdata/`.
