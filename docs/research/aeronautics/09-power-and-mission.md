# Task 09 — Power-first design and component feedback

**Outcome:** the weight/power-ceiling journey plus motor, battery, duration and
distance tradeoffs. Dependencies: [02](02-equations-and-lift.md) through [06](06-worksheet-ui.md).

## Work

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

## Acceptance checks

- Power-case arithmetic has independent fixtures, units and documented assumptions.
- Static and cruise data are never substituted automatically; flight below the
  supported lift/polar envelope cannot appear power-feasible.
- Required/preferred power ceilings and missing power-model inputs behave distinctly.
- Battery/motor additions update component-derived mass and downstream results;
  fixed-total allocation mode avoids double counting.
- Energy-unit conversions, reserve accounting, segment totals, wind/return cases
  and empty mass budgets are tested.
- Any implemented search returns alternatives/intervals when nonunique and cannot
  present a nonconverged iterate as a successful design.
