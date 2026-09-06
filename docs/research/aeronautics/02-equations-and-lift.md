# Task 02 — Equation registry, units, and lift inversions

**Outcome:** independently callable Go calculations with descriptive validation,
traceable formulas, and checked numerical examples. Dependency: [01](01-project-boundaries.md).

## Work

- Define quantities/units so mass (kg), weight (N), lengths (m), areas (m²), true
  speed (m/s), angles, power and energy cannot be casually interchanged. Use SI
  internally and test supported boundary conversions; never round intermediates.
- Each implemented equation has an ID/revision, expression, input/output units,
  source and assumptions. Each evaluation returns actual substitutions and output
  for an inspectable log. Keep logging free of implicit I/O and timestamps.
- Return typed missing/invalid/unsupported results with field-specific issues.
  Validate finite inputs and output; detect zero divisors and overflow/underflow.
  Ordinary invalid user data must not become panics or successful NaN values.
- Flight cases include configuration, air density, load factor and coefficient
  source. Make assumed density/CLmax visible. A NACA identifier or 2D `clmax`
  must not silently become whole-aircraft `CLmax`.

## Initial equations

```text
g = 9.80665 m/s²                  W = m g
q = rho V² / 2                   L_required = n W
CL_required = n W / (q S)         loading_force = W/S
loading_mass = m/S
Vs = sqrt(2 n m g / (rho S CLmax))
S_min = 2 n m g / (rho Vs_limit² CLmax)
m_max = rho Vs_limit² S CLmax / (2 n g)
(W/S)_max = rho Vs_limit² CLmax / (2 n)
```

Use true airspeed with local density and an explicitly positive load factor.
This is a lumped lift model, without separately solved wing/tail trim loads.
The mass ceiling is aerodynamic for the selected case, not a structural rating.
A stall ceiling produces an area lower bound; it does not choose actual area.
[NASA lift equation](https://www1.grc.nasa.gov/beginners-guide-to-aeronautics/lift-equation/).

## Acceptance checks

Use independently calculated fixtures with `m=2 kg`, `rho=1.225 kg/m³`,
`CLmax=1.2`, `n=1`, `S=.24 m²`. CLmax is a synthetic assumption, not a default
recommendation. Expected values rounded here:

| Quantity | Expected |
|---|---|
| Weight | 19.6133 N |
| Loading | 81.7220833 N/m²; 8.3333333 kg/m² |
| Stall speed | 10.5445013 m/s |
| Area lower bound for 8 m/s | 0.4169494048 m² |
| Mass ceiling for 8 m/s, same area | 1.1512188158 kg |

- Forward/inverse round trips and mixed-unit equivalence pass.
- With other assumptions held fixed, quadrupling mass doubles stall speed;
  quadrupling area halves it. Changing load factor is reflected explicitly.
- Missing input, negative/zero physical denominators, nonfinite/extreme values,
  and incompatible units return useful issues.
- Traces match supplied values and registry revisions; evaluation does not mutate
  caller data. Metadata inspection cannot mutate shared definitions.
- Numerical tolerances are stated in physical units plus relative tolerance,
  independently of display rounding.
