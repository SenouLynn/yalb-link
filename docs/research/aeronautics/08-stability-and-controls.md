# Task 08 — Configuration-specific stability and control sizing

**Outcome:** a usable conventional-tail static-margin and trim assessment based on
the book, with explicit coverage of geometry, control and handling for each configuration.
Dependencies: [03](03-geometry-and-configurations.md) through [06](06-worksheet-ui.md).

## Work

- Use the book's [Static Margin](https://computationaldesignlab.github.io/aircraft-design/long_stability/static_margin.html)
  and [Trim Analysis](https://computationaldesignlab.github.io/aircraft-design/long_stability/trim.html)
  as the baseline, with its lift, tail-sizing and weight/balance prerequisites.
  Follow the source/adaptation rules in the [handoff](README.md).
- Minimum delivery is conventional-tail power-off longitudinal static margin at
  a named aft-CG case, plus approach trim and remaining elevator travel/control
  moment at a named forward-CG case with user-entered quantitative requirements.
  Solve lift and pitching-moment balance together within the supported envelope;
  report residual tolerances and explicit no-solution/unsupported outcomes.
  A geometry seed or all-unknown result cannot satisfy this minimum.
- Track support by configuration and outcome. V-tail/flying-wing geometry and
  mixing checks remain required; their aerodynamic trim/handling extensions and
  dynamic-rate predictions are deferred unless separately modeled and validated.
  Do not apply the conventional baseline to them implicitly.
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
- Expose the minimum assessment through the existing API and worksheet: case/CG
  selection, evidence entry, requirements, trim results, remaining authority and
  recovery from missing data. Extend Task 06's snapshot contract compatibly.
- For autopilot-equipped RC aircraft, record commanded flight limits, mixer
  conventions and actuator travel/rate limits independently of aerodynamic
  capability. Identify which limits the current model can assess; a static moment
  calculation cannot establish tracking bandwidth or closed-loop stability.
  Do not assume stabilization compensates for missing trim or saturated controls.
- If Task 07's placement view exists, connect its shared mass/CG model to the
  supported trim loads, force locations and stability overlays. Verify a component
  move updates the assessment. If Task 08 lands first, expose these structured
  results and leave visual integration to Task 07; this is a conditional integration
  check, not a new circular dependency.

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

- Reproduce relevant book static-margin/trim examples with documented unit and
  reference conversions, resolving any equation/code discrepancies explicitly.
  Add an independent complete conventional-aircraft fixture connecting geometry,
  aerodynamic evidence, CG, simultaneous lift/moment balance, elevator geometry,
  travel and servo/linkage limits to met/unmet requirements. Include an untrimmable
  case and a missing-evidence case; state supported aerodynamic validity limits.
- Browser tests enter that aircraft and its evidence, complete the minimum
  assessment, change CG/control limits to violate a requirement, and remove/restore
  evidence to exercise unknown-state recovery. Results agree with direct Go calls.
- Doubling a conventional volume-method arm halves the seeded area with references
  fixed; CG changes affect stability/control separately from that geometric volume.
- Signed moments, unit conversion, trim reserve and inertia sensitivity have
  independent numerical fixtures. Zero derivative or missing data is explicit.
- V-tail mixed-command saturation and flying-wing elevon pitch/roll competition
  are covered; geometry alone cannot produce a passing handling assessment.
- Manual and autopilot command cases use the same physical actuator limits. An
  excessive mixed command remains saturated in either mode; unmodeled rate or
  closed-loop requirements remain unknown even when static authority is sufficient.
- Flap configurations use distinct evidence and update trim/remaining authority.
- Geometry/CG/configuration changes invalidate incompatible imported derivatives.
- No universal trainer/sport/aerobatic constants ship without cited evidence and
  visible assumptions. Each reported handling outcome names its flight case.
