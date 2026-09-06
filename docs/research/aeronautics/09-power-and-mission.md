# Task 09 — Power-first design and component feedback

**Outcome:** the weight/power-ceiling journey plus motor, battery, duration and
distance tradeoffs. Dependencies: [02](02-equations-and-lift.md) through [06](06-worksheet-ui.md).

## Work

- Start from the book's [Drag Polar](https://computationaldesignlab.github.io/aircraft-design/aerodynamics/drag_polar_induced_drag.html),
  [Engine and Propeller Selection](https://computationaldesignlab.github.io/aircraft-design/powerplant/engine_propeller.html)
  and [Mission analysis](https://computationaldesignlab.github.io/aircraft-design/performance/mission_analysis.html).
  Use its weight/balance and constraint-analysis organization under the
  [handoff's source rules](README.md). Document electric propulsion and battery
  accounting as adaptations; piston-engine correlations and fuel-burn fractions
  do not become electric RC defaults.
- Separate a thrust-to-weight target from available thrust, both at named speed,
  density, throttle/RPM and voltage conditions. Static thrust is not cruise thrust.
- Support explicit measured/estimated capability first. A later motor/propeller
  operating-point model needs appropriate data; Kv and prop dimensions alone
  cannot establish thrust across the flight envelope.
- Weight plus electrical power maximum is underdetermined without geometry,
  speed, drag and efficiency assumptions. Offer a clearly specified candidate
  check or bounded feasible design search, never an unexplained unique wing size.
- Use a documented preliminary polar where appropriate:
  `CD=CD0+CL²/(pi*e*AR)` and `D=q*S*CD`. CD0/e need aircraft-level provenance.
  In steady level flight `Trequired=D`; useful power is `Trequired*V`.
  A total propeller/motor/ESC efficiency plus auxiliary draw yields an electrical
  estimate at that condition. Do not divide by propulsive efficiency at zero
  speed to obtain static power. [Drag basis](https://www1.grc.nasa.gov/beginners-guide-to-aeronautics/induced-drag-coefficient/),
  [propeller data](https://m-selig.ae.illinois.edu/props/propDB.html).
- Keep climb/load-factor assumptions consistent with force balance. Reject an
  attached-flow power prediction outside the model's lift/polar envelope.
- Model component mass and position or an entered all-up total as explicit modes.
  Reuse Task 07's mass/CG model if present; if this task lands first, establish
  that shared contract here for the later placement view.
  Do not add battery/motor mass twice. A list under a fixed total is an allocation
  check; a component-derived total sums the list and explicit growth allowance.
- `t=Eusable/Pelec` is a constant-draw estimate. Missions use
  `Erequired=sum(P_i*dt_i)` and reserve exactly once. Distance uses velocity along
  the ground track and time; wind and the return leg matter for return range.
- Battery selection revisits mass, CG, wing loading, required power and energy.
  Initially expose this iteration to the builder; a later solver needs finite
  bounds, convergence criteria and explicit no-solution/nonconvergence outcomes.
- Include relevant voltage/current, electrical/thermal, RPM and prop-clearance
  limits when making component feasibility claims.
- Deliver the power-first entry point through the API and worksheet, including
  capability/evidence entry, component mass modes, mission segments, requirements
  and explicit design iteration. Extend Task 06's versioned snapshots compatibly.
  This task cannot finish with only callable core calculations.
- Include autopilot, receiver, telemetry, sensors, regulators, servos, wiring and
  payload in component mass/position and auxiliary electrical budgets. Distinguish
  continuous mission draw from peak actuator/electronics demand; document which
  side of each regulator an entered power value uses to avoid duplicate losses.
  Missing avionics/servo demand prevents a complete electrical feasibility claim.
- Provide editable RC mission segments for launch/climb, cruise/loiter, return and
  recovery, using entered estimates when a segment model is unavailable. Label
  that evidence and keep energy sufficiency separate from flight feasibility.
  Autopilot return settings do not themselves establish a wind-aware return range.
- Connect component mass/position edits to Task 07's placement view when present.
  Moving an unchanged battery affects CG and supported trim checks; changing its
  mass/capacity also revisits loading, stall, power and energy. Preserve the explicit
  fixed-total versus component-derived distinction in both the table and the view.

## Acceptance checks

- Reproduce a relevant book drag/power example with original-unit/SI equivalence;
  use separate independent electric fixtures to verify the documented adaptations.
- Browser tests complete a power-ceiling candidate check with entered capability
  and polar evidence, then change battery/motor entries and observe mass, CG,
  loading, required power and mission energy updates. Exercise both mass modes,
  a violated ceiling, and removing/restoring required evidence. Outputs agree
  with Go calls and the design survives save/reopen under the extended schema.
- Power-case arithmetic has independent fixtures, units and documented assumptions.
- Static and cruise data are never substituted automatically; flight below the
  supported lift/polar envelope cannot appear power-feasible.
- Required/preferred power ceilings and missing power-model inputs behave distinctly.
- Battery/motor additions update component-derived mass and downstream results;
  fixed-total allocation mode avoids double counting.
- Energy-unit conversions, reserve accounting, segment totals, wind/return cases
  and empty mass budgets are tested.
- Avionics/servo/payload additions affect mass and electrical budgets exactly
  once. Test a mission-energy pass with a peak supply-limit failure, plus missing
  auxiliary demand. Browser coverage includes an autopilot-equipped RC mission
  with loiter and return, explicit reserve and visible segment-model limitations.
- Any implemented search returns alternatives/intervals when nonunique and cannot
  present a nonconverged iterate as a successful design.
