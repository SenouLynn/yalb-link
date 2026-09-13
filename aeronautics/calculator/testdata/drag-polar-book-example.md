# Drag Polar — CODE Lab worked example, reproduced

Source: [Drag Polar and Induced Drag](https://computationaldesignlab.github.io/aircraft-design/aerodynamics/drag_polar_induced_drag.html),
read 2026-09-07. No upstream commit revision is verified. The chapter's example
aircraft is a manned twin in US customary units; none of its coefficients is a
default in this package.

## What the chapter gives

Relations:

```
CD = CD0 + CL^2 / (pi * A * e)
e  = 1.78 * (1 - 0.045 * A^0.68) - 0.64          (quoted from Raymer eq. 12.48)
```

Example inputs, taken from the chapter's own code cell:

| Symbol | Value | Note |
|---|---|---|
| `A` | 8 | assumed, carried over from its initial weight estimation |
| `CD0` | 0.03363 | clean (cruise) configuration |
| `CL_alpha` | 5.0 per radian | used only to sweep the polar over alpha |
| `alpha_CL=0` | −1.0° | same |
| alpha sweep | −5° to 13°, 100 points | same |

Displayed results: **max L/D = 12.31**, with a scatter marker annotated at
`(CD, CL) = (0.065, 0.8)`.

## Independently computed values

Computed at 50 significant digits from the relations above, not read back from
this package. Quoted here to 25 significant digits.

| Quantity | Value |
|---|---|
| `8^0.68` | 4.112455306624266037385380 |
| `e` | 0.8105923299393962904054311 |
| `K = 1/(pi A e)` | 0.04908600082108922456882981 |
| `CL` at max L/D, `sqrt(CD0/K)` | 0.8277222097430363354943594 |
| max `L/D`, `1/(2 sqrt(K CD0))` | 12.30630701372340671267261 |
| `CD` at `CL = 0.8` | 0.06504504052549710372405108 |
| `L/D` at `CL = 0.8` | 12.29916982965683280854472 |
| `CL` at minimum power, `sqrt(3 CD0/K)` | 1.433656921828121717994504 |

## Agreement and discrepancies

**Max L/D agrees.** 12.30630701… displays as 12.31, which is the chapter's
figure. Display rounding, not a discrepancy.

**The chapter's own alpha grid attains it.** Evaluating the polar on its stated
100-point sweep from −5° to 13° gives a maximum `L/D` of 12.30624338… at
`alpha = 8.4545°`, which also displays as 12.31. The grid maximum and the
analytic optimum agree to five significant figures, so the displayed figure does
not distinguish between them and this package's analytic form is consistent with
either reading.

**The annotated marker is the polar at `CL = 0.8`, not the maximum.** The
chapter's scatter point `(0.065, 0.8)` reproduces exactly as `CD = 0.06504504…`
at `CL = 0.8`, so the marker is the polar evaluated at a round lift coefficient.
`L/D` there is 12.2992, slightly below the true maximum of 12.3063. The
annotation reads "L/D max"; taken literally it names a point that is not the
maximum. This package implements the relations, not the annotation, and reports
`L/D` at whatever condition it is asked about — `LiftToDrag` says in its own
documentation that it is a ratio at a condition and not a maximum.

**The Mission analysis chapter rounds `e` to 0.81.** Its code cell sets
`e = 0.81` and derives `K = 1/pi/A/e` from that, while the Drag Polar chapter
computes `e = 0.810592…` from the Raymer correlation for the same aircraft. The
two give `K = 0.04912…` and `K = 0.049086…`, a 0.07% difference. This package
does not choose between them: `e` is a supplied input with its own basis, and a
builder taking the correlation gets the unrounded value from
`OswaldEfficiencyStraightWing`.

## Dimensional extension — original units and SI

The chapter's polar is dimensionless, so reproducing it exercises no unit
conversion. To check the drag force in both unit systems the same example
aircraft is evaluated at the cruise condition its Mission analysis chapter
states: `S = 134 ft²`, `rho = 0.00186850 slug/ft³` (8000 ft), `V = 200 knots`,
at the annotated `CL = 0.8` and therefore `CD = 0.06504504052549710372405108`.

This is **not** a reproduction of a displayed book number — the chapter displays
an average power over ten sub-segments of a fuel-burning cruise, which needs the
piston weight fractions this package does not implement. It is an independent
fixture evaluated twice, once in each unit system, to check that the conversions
agree.

| Quantity | US customary | SI |
|---|---|---|
| `V` | 337.5619714202391367745698 ft/s | 102.8888888888888888888889 m/s |
| `rho` | 0.00186850 slug/ft³ | 0.9629853221676871061295551 kg/m³ |
| `S` | 134 ft² | 12.44900736 m² |
| `q = rho V²/2` | 106.4559979900138126750876 lbf/ft² | 5097.140753771973265528064 Pa |
| `D = q S CD` | 927.8742502613200129504386 lbf | 4127.390296256034322231295 N |

The two routes agree exactly at 25 significant digits: `927.8742502613200129504386 lbf`
converts to `4127.390296256034322231295 N`. The slug is carried through as
`1 lbf·s²/ft`, so the density factor is written from the pound force and the
foot rather than as a rounded decimal: `slug/ft³ = 515.3788183931962034410249 kg/m³`.

## What is not implemented from this chapter

- **The parasite-drag buildup.** The chapter states `CD0 = 0.03363` and refers to
  a previous section for how it was obtained; no component buildup, wetted-area
  or equivalent-skin-friction method is implemented here, so `CD0` is always
  supplied evidence with an aircraft-level basis.
- **A validity range.** The chapter states none. This package requires one,
  because a parabolic polar is symmetric in `CL` and will report a drag
  coefficient at lift coefficients the aircraft cannot reach.
