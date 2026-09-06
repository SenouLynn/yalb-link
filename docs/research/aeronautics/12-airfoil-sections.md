# Task 12 — Airfoil sections, NACA generation, and spanwise lofting

**Outcome:** real section geometry for a solved wing — generated NACA 4- and
5-digit shapes, ingested external coordinate tables, and sections lofted across
the span under taper and twist. Dependency: [03](03-geometry-and-configurations.md).

Task 03 stops at the planform and says so: a planform "does not define an airfoil
section or complete 3D wing loft", and `Airfoil` carries an identity with no shape
and no polar. This task supplies the shape. It still supplies no polar: generating
ordinates establishes coordinates, not lift, drag or stall behaviour.

## Work

- Follow the [handoff's source rules](README.md). The CODE Lab book's Wing Planform
  Sizing chapter treats airfoil selection as context and contains no section
  generation method, so **nothing in this task may claim `SourceBook`**. Sections
  cite [Abbott & von Doenhoff, *Theory of Wing Sections*](https://ntrs.nasa.gov/citations/19930091108)
  and the original NACA reports as supplementary sources, with access date,
  original notation and any adaptation recorded per equation.
- Keep `Airfoil` as it is: identity, evidence and stated thickness ratio, and
  comparable. `Airfoil.supplied()` is a struct equality test, so a coordinate slice
  added to that type would not compile. Coordinates belong to a separate `Section`.
- Generate NACA 4-digit and 5-digit sections from a designation. Report the
  modified 4/5-digit (`0012-64`), 6-series and named non-NACA sections as
  **unsupported**, not invalid: the designation is well formed and the model does
  not cover it.
- Accept externally supplied coordinate tables for the SD/AG/MH/E sections RC
  builders actually use. The core does no file I/O — the import boundary test bans
  `os`, which is why the core cannot even use `fmt` — so text parsing lives in a
  sibling package and the core receives points.
- Make every convention an explicit input, never a default: point ordering
  (Selig or Lednicer), point count, chordwise distribution, and whether the
  trailing edge is open or closed. Each changes the ordinates and each must appear
  in the section's provenance.
- Record shape-level provenance rather than a per-coordinate trace. A 200-point
  section is one shape, not 200 results. Scalar queries against a section —
  thickness at a chord fraction, the largest inscribed circle — return an ordinary
  traced `Result` like every other equation in the package.
- Loft sections across the span: chord from the solved planform, shape blended
  between root and tip where they differ, and local twist applied about an
  explicitly stated axis. A blended section is a shape that is neither parent and
  carries no section data from either.
- State the twist axis as a required chord fraction, in the same way `SweepReference`
  is already required. Rotating a section about its leading edge, its quarter chord
  or a spar line produces three different wings.

## Section conventions

A section coordinate is `x` aft from the leading edge and `y` up, both as
fractions of chord, against the wing datum Task 03 already defines.

Two point orderings are in circulation and a table is meaningless without knowing
which it uses: **Selig** runs trailing edge to upper surface to leading edge to
lower surface and back, as one continuous loop; **Lednicer** gives the upper
surface leading edge to trailing edge, then the lower surface the same way, as two
runs. They are not reliably distinguishable by inspection on a section with a
blunt trailing edge, so the order is a required input on ingest, not a guess.

Hold the upper and lower surfaces as separate runs with `x` ascending, whatever
order the input used, and re-emit either convention on request for CAD handoff.

A section also records whether it was generated from a designation, imported as a
coordinate table, or blended between two others, so a generated ordinate is never
mistaken for a measured one.

## Equation contracts

4-digit `MPTT`, with `m=M/100`, `p=P/10`, `t=TT/100`:

```text
y_c = (m/p^2)(2 p x - x^2)                     x < p
y_c = (m/(1-p)^2)((1-2p) + 2 p x - x^2)        x >= p
theta = atan(dy_c/dx)
y_t = 5t(0.2969 sqrt(x) - 0.1260 x - 0.3516 x^2 + 0.2843 x^3 - a4 x^4)
x_u = x - y_t sin(theta);   y_u = y_c + y_t cos(theta)
x_l = x + y_t sin(theta);   y_l = y_c - y_t cos(theta)
```

`a4 = 0.1015` leaves the classic open trailing edge; `a4 = 0.1036` closes it. The
choice changes the last few percent of chord and is recorded, not defaulted.
`m = 0` requires `p = 0`; `m != 0` with `p = 0` is degenerate and rejected before
it can divide by zero.

5-digit `LPSTT`: design `C_L = 0.15 L`, max camber position `P/20`, `S` selects
standard or reflexed camber, `t = TT/100`. Thickness is the 4-digit distribution.

```text
standard (S=0):
  y_c = (k1/6)(x^3 - 3 m x^2 + m^2 (3-m) x)          x < m
  y_c = (k1 m^3 / 6)(1 - x)                          x >= m

reflexed (S=1):
  y_c = (k1/6)[(x-m)^3 - (k2/k1)(1-m)^3 x - m^3 x + m^3]              x < m
  y_c = (k1/6)[(k2/k1)(x-m)^3 - (k2/k1)(1-m)^3 x - m^3 x + m^3]       x >= m
```

`m` and `k1` are tabulated per camber position, not computed. **Tabulate exactly
the standard rows and reject any other position as unsupported** — interpolating a
row between tabulated values is the same silent invention the handoff forbids for
polar data. `k1` is tabulated at design `C_L = 0.3`; other `L` scale camber as
`k1 * (0.15 L / 0.3)`, which is an adaptation and is recorded as one.

The tabulated rows below are a starting point for implementation, **not a verified
transcription**. Transcribe them from the cited primary source at implementation
time, record the access date, and check against a second independent source before
use — the same discipline that corrected the Task 02 source attribution.

| S | P | position | m | k1 | k2/k1 |
|---|---|---|---|---|---|
| 0 | 1 | 0.05 | 0.0580 | 361.4 | — |
| 0 | 2 | 0.10 | 0.1260 | 51.64 | — |
| 0 | 3 | 0.15 | 0.2025 | 15.957 | — |
| 0 | 4 | 0.20 | 0.2900 | 6.643 | — |
| 0 | 5 | 0.25 | 0.3910 | 3.230 | — |
| 1 | 2 | 0.10 | 0.1300 | 51.99 | 0.000764 |
| 1 | 3 | 0.15 | 0.2170 | 15.793 | 0.00677 |
| 1 | 4 | 0.20 | 0.3180 | 6.520 | 0.0303 |
| 1 | 5 | 0.25 | 0.4410 | 3.191 | 0.1355 |

Reflexed sections are implemented rather than deferred because flying wings are in
scope from the beginning and a reflexed section is how a tailless wing trims. It is
the one place where section choice and configuration meet.

Cosine spacing `x = (1 - cos(u))/2` for `u` over `[0, pi]` is required for a usable
leading edge. Interpolation between stored points is linear and stated as an
assumption; point count is an input, not a hidden constant.

Lofting a station `y` on a wing of span `b`:

```text
c(y)      from the solved planform's chord-at-station relation
theta(y)  = incidence + twist * (2 y / b)          linear twist only
shape     root and tip resampled to a common distribution, blended linearly
rotation  about the stated twist axis chord fraction
```

## Acceptance checks

- NACA `00tt` has `y_c` identically zero, and maximum thickness `tt/100` at the
  station the thickness polynomial puts it at.
- Published NACA 2412 and 23012 ordinates reproduce to the precision the source
  tables are quoted to. Fixtures record which trailing-edge closure the table uses;
  comparing a closed-TE generation against an open-TE table is the most likely
  false failure and must not be papered over with a loose tolerance.
- `NACA("0012-64")`, `NACA("63-012")` and `NACA("SD7037")` return **unsupported**
  with a detail naming what is implemented. None returns invalid, and none returns
  a partial section.
- A 5-digit designation with an untabulated camber position is unsupported. No
  interpolated row is produced.
- Round trip: a generated section emitted as Selig points and read back through the
  ingest path reproduces the same surfaces within tolerance.
- Ingest rejects an unstated point order, a non-monotonic surface, an
  unidentifiable leading edge, and crossed surfaces, each as its own typed issue,
  and reports every bad field in one pass.
- A coordinate table that is not unit-chord is scaled, and the scaling appears in
  the provenance as an adaptation. A caller can see that their data was changed.
- A stated `Airfoil.ThicknessRatio` that disagrees with the derived section
  thickness beyond tolerance is reported as a **conflict**, in the same way an
  over-determined planform reports conflicting drivers. Neither value silently wins.
- Blending two different designations records the result as blended, and the
  provenance states that no section data from either parent applies to it.
- Blending a section with itself returns that section's surfaces unchanged.
- The twist axis is required. A wing with nonzero twist and no stated axis reports
  a missing field. At zero twist and zero incidence the axis cannot change any
  result and is not demanded.
- Rotating about the leading edge and about the quarter chord give measurably
  different lofted coordinates at the same station and twist.
- Provenance: no equation added by this task carries `SourceBook`, asserted by a
  test in the shape of the existing book-provenance guard.
- The core still does not reach `os`, `net`, `time` or `database/sql`, and does not
  import the coordinate-file package.

**Not established by this task:** lift, drag, moment, stall behaviour, a polar, a
`clmax`, or a Reynolds-corrected section characteristic. Coordinates are geometry.
Section data remains supplied evidence, as `CLmax` already is.
