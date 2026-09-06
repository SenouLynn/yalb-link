# Task 08 — Configuration-specific stability and control sizing

**Outcome:** distinguish geometry seeds, static stability, trim, available control
moment and dynamic response for conventional tails, V-tails and flying wings.
Dependencies: [03](03-geometry-and-configurations.md) through [06](06-worksheet-ui.md).

## Work

- Establish mass/CG cases, datum/axes, inertia when required, and the quantitative
  handling target before selecting model fidelity. An example target might be
  trim at forward CG during approach, remaining nose-up/nose-down moment, or roll
  rate at a named speed. A category name alone cannot size a control surface.
- Implement each supported model as its own documented, tested calculation. When
  suitable aerodynamic evidence is absent, provide geometry or a labeled sizing
  estimate and leave unsupported handling outputs unknown.
- Conventional-tail seeds use `VH=SH*lH/(S*MAC)` and `VV=SV*lV/(S*b)`, inverted
  for area or arm. State the volume convention; initially use wing-to-tail
  aerodynamic-center separation. Actual moments about CG need CG-to-force arms.
  Target volumes require a traceable comparator or explicit assumption.
  [Tail-volume basis](https://computationaldesignlab.github.io/aircraft-design/tail_sizing.html).
- Record actual tail span/height, chord, taper, incidence and movable portions;
  tail area includes its control surface. Longer arms change area needs but also
  mass distribution, damping, structure and inertia.
- V-tails need panel geometry/cant and ruddervator mixing. Pitch and yaw commands
  share deflection/actuator travel; simultaneous requests can saturate a panel.
  Select and validate any equivalent-area approximation before implementing it;
  projected area alone is not proof of equivalent control authority.
- Flying wings need wing pitching moment, sweep/twist/reflex, CG/neutral point,
  elevon layout/mixing and a defined yaw mechanism (fins, drag rudders, differential
  thrust or another modeled arrangement). Do not apply ordinary tail-volume rules.
  [Flying-wing design context](https://www.mh-aerotools.de/airfoils/flywing1.htm).
- Ailerons/flaps require span stations, local chord fractions, hinge line, travel,
  mixing and aerodynamic evidence. Account for aileron spanwise moment arm,
  adverse yaw and damping; flaps change lift, drag and pitching moment, consuming
  trim reserve. Include servo/linkage capability before claiming delivered authority.

## Mathematical bridge to handling

`SM=(xNP-xCG)/MAC`, with stations increasing aft, is a longitudinal static-margin
definition. Neutral point is an aircraft property, not automatically wing quarter
chord. Downwash, tail dynamic pressure, slopes and fuselage effects matter.

With compatible derivatives at a trim condition:

```text
Delta M = q S MAC Cm_delta_e Delta delta_e
Delta N = q S b   Cn_delta_r Delta delta_r
Delta R = q S b   Cl_delta_a Delta delta_a  # R denotes rolling moment
```

Deflections/slopes need explicit radian/sign conventions. Moment divided by the
corresponding inertia estimates initial angular acceleration under an uncoupled
approximation; it does not determine steady rate or settling time. Remaining
travel is measured from trim in each direction.
[Derivative basis](https://www.aircraftflightmechanics.com/Linearisation/AerodynamicDerivatives.html).

Coupled dynamic modes (short period, phugoid, Dutch roll, spiral, roll subsidence)
require a separate model and suitable data. Implement them only if the selected
handling target needs them; otherwise report that limit.

## Acceptance checks

- Doubling a conventional volume-method arm halves the seeded area with references
  fixed; CG changes affect stability/control separately from that geometric volume.
- Signed moments, unit conversion, trim reserve and inertia sensitivity have
  independent numerical fixtures. Zero derivative or missing data is explicit.
- V-tail mixed-command saturation and flying-wing elevon pitch/roll competition
  are covered; geometry alone cannot produce a passing handling assessment.
- Flap configurations use distinct evidence and update trim/remaining authority.
- Geometry/CG/configuration changes invalidate incompatible imported derivatives.
- No universal trainer/sport/aerobatic constants ship without cited evidence and
  visible assumptions. Each reported handling outcome names its flight case.
