# Aeronautics calculator — task handoff

Status: **implementation in progress in `aeronautics/`.** The user authorized
the original eleven tasks on 2026-09-05; Tasks 12 and 13 were added afterwards
and are listed in the task order below. Task descriptions are acceptance
targets, not claims of completed capabilities. See the [implementation log](../../reference/aeronautics-implementation.md)
for checked behavior, decisions and outstanding verification.

## Start here in a fresh session

1. Confirm the active workspace is `/Users/senoulynn/Desktop/yalb-link`.
   These planning documents belong in this checkout. The older `ligma-gcs`
   directory is a separate checkout and must not receive implementation changes
   for this work.
2. Read this file, the selected task, its CODE Lab source chapters, and applicable
   repository instructions.
3. Implement the selected task through its acceptance checks. Report what is
   implemented, what was tested, and any remaining model limitations.

Suggested continuation prompt:

> Read docs/research/aeronautics/README.md and implement Task 08 only, including
> its checks. Keep the calculator independent of the existing GCS frontend and
> services. Do not start another task automatically.

Tasks 01 to 07 are complete; the [implementation log](../../reference/aeronautics-implementation.md)
records what each one actually checks and where it stops. Task 06 finished the
first useful sizing release: the worksheet drives the span-first and both
weight-first journeys against the running Go service, and versioned drafts save
and reopen. Task 07 added the explanatory visuals: dimensioned views whose every
dimension names the parameter that drives it, a shared mass-properties model with
visual placement, and a one-driver sensitivity sweep. Tasks 08 and 09 are both
schedulable now; Task 08 is the natural next one, because Task 07 has left every
aerodynamic reference location and every handling question explicitly unknown and
Task 08 is what may answer them.

## User intent and settled direction

- **Interaction model: an OpenSCAD-like parametric design flow.** The complete
  supported design process is expressed through named parameters, explicit
  relationships and reproducible evaluation. Geometry, mass placement, flight
  cases, requirements, analysis, visuals and CAD handoff derive from the same
  authoritative design definition. Editing a field or dragging a component edits
  that definition and reevaluates its dependencies. This describes the workflow;
  it does not require OpenSCAD syntax, a new scripting language or a CSG engine.
- Base the calculator's engineering methodology on the CODE Lab Aircraft Design
  book linked below. Use its methods and worked examples throughout the tasks.
- **Target aircraft:** fixed-wing RC drones/UAVs, including autopilot-equipped
  conventional aircraft, V-tails and flying wings. Prioritize unmanned missions,
  payload, electric propulsion, avionics and control-actuator integration.
  Multirotor/VTOL sizing and transition flight require additional models.
- A compact, informative RC aircraft design worksheet, visually inspired by
  Dear ImGui. Trust the builder; provide useful explanations without modal-heavy
  flows, dramatic warnings, or a universal aircraft/handling score.
- **Go owns the work:** equations, validation, workflow decisions, dependency
  evaluation, traces, and sensitivity calculations. Isolate it from HTTP, MCP,
  storage, and the GCS. Add concurrency for measured batch/solver needs, not for
  individual algebraic formulas.
- **Separate frontend:** Vite/React is the current working choice. The user asked
  whether HTMX or a Go Dear ImGui port might be better but did not select either.
  React remains appropriate for a deployable web worksheet with interactive
  charts; the Go core must not depend on that choice.
- Include conventional tails, **V-tails and flying wings from the beginning** in
  configuration/workflow design. Their stability and control models differ.
  Never treat a missing conventional tail as an error on a flying wing or report
  ordinary tail-volume results as its handling assessment.
- “Maximum length” in the original geometry example means **maximum wingspan**.
- Log the equations and actual substitutions; keep implementations readable and
  reasonably tested. Define and test workflow behavior before implementing UI.
- Provide a visual mass-placement worksheet: move battery, motor, avionics and
  payload on aircraft views and see CG, aerodynamic reference markers and supported
  lift/trim effects update alongside loading and stall results. Task 07 established
  the view and the mechanical CG; Tasks 08 and 09 connect handling and
  component/power models. The aerodynamic reference markers other than the
  geometric quarter-MAC stay unknown until Task 08.
- **Sketch-ready geometry:** values must be usable in Fusion 360 sketches. Show
  named dimensions, datums, construction lines and the formulas linking them;
  connect each worksheet field to its visual dimension and CAD parameter. Task 03
  defines the relationships, Task 07 explains them visually, and Task 11 verifies
  the actual Fusion parameter/expression and sketch workflow.
- An optional **MCP sidecar** should expose the same calculations and curated,
  versioned (“blessed”) workflows. It must not duplicate the calculation engine.
- Eventually hand dimensions and requirements to Fusion 360, XFLR5/flow5, and a
  NASA's **OpenVSP**, selected by the user on 2026-09-05. Verify the target version
  and handoff format during Task 11; VSPAERO analysis is a separate capability.

## Primary user journeys

1. **Span first:** maximum span → explicitly choose actual span/use the maximum →
   aspect ratio or rectangular chord → area → adjust mass, or choose quantitative
   flight requirements to obtain a feasible mass range → choose battery/motor →
   revisit mass, CG, power, and performance.
2. **Weight first:** all-up mass → choose performance, available wing size, or an
   electrical power ceiling as the deciding constraint. Each needs different
   additional inputs; power alone cannot uniquely determine a wing.
3. **Existing design:** enter geometry/components → evaluate requirements → adjust
   selected drivers and compare candidates.

UI explanations and next actions depend on the entry point. All views use the
same design state, not duplicate sets of area, mass, and speed fields.
The worksheet is an interactive editor for that parametric definition. A saved
definition plus its model/evidence revisions must reproduce the supported results
without depending on hidden UI state or the sequence of edits used to create it.

## RC drone and autopilot scope

Adapt the book's methodology to the aircraft being built. Use explicit RC-scale
geometry, Reynolds conditions and component evidence; do not inherit its example
aircraft's empirical constants, passenger assumptions or piston-engine sizing.

Model the autopilot, receiver, telemetry, navigation sensors, power electronics,
wiring, servos and payload as mass/position and electrical-budget contributors.
Describe mission cases such as launch, climb, cruise, loiter, return and recovery
with named assumptions and quantitative requirements. An entered mission power
estimate does not establish launch, landing or maneuver feasibility.

Record intended manual/stabilized/autonomous operation, commanded speed/bank or
load-factor limits, control mixing, actuator travel/rate and relevant power limits.
Evaluate only cases supported by the implemented airframe and propulsion models;
autopilot presence does not establish trim, control authority or dynamic stability.
Any relaxed-static-stability design needs a separate supported closed-loop model
before a handling claim can be made.

The calculator supports airframe/mission design for autopilot use. Flight-control
firmware, gain tuning, live vehicle commands, automatic parameter uploads and
closed-loop/SITL validation are additional work. Keep platform-specific parameter
mapping in an explicit future adapter with named firmware/version and verified
units, axes and mixing semantics; the Go calculator remains independent of GCS.

## Engineering foundation — CODE Lab Aircraft Design

The [Aircraft Design book](https://computationaldesignlab.github.io/aircraft-design/intro.html)
is the primary engineering reference for this entire plan. It is a Python-based
conceptual design resource from Purdue's CODE Lab. Port applicable calculations
into the Go core and build the worksheet around their inputs, outputs and design
iteration. Python notebooks are reference material, not a runtime dependency.

Use this chapter map when implementing; the existing task numbers remain the
software delivery order, with alternate user entry points into the same methods.

| Tasks | Book basis and application |
|---|---|
| 02 | [Lift](https://computationaldesignlab.github.io/aircraft-design/aerodynamics/lift_curve.html): coefficient evidence and applicability; initial lift inversions remain the first subset |
| 03 | [Wing Planform Sizing](https://computationaldesignlab.github.io/aircraft-design/wing_layout.html): geometry, reference dimensions and airfoil context |
| 04, 06 | [Matching process](https://computationaldesignlab.github.io/aircraft-design/constraint_analysis/final.html): intersect requirements, explain active constraints and deliberately select a candidate |
| 07 | Matching-process plots and the book's Trade study chapters: visualize supported constraints and design changes |
| 08 | [Tail Sizing](https://computationaldesignlab.github.io/aircraft-design/tail_sizing.html), [Static Margin](https://computationaldesignlab.github.io/aircraft-design/long_stability/static_margin.html), [Trim Analysis](https://computationaldesignlab.github.io/aircraft-design/long_stability/trim.html), and their Weight and Balance prerequisites |
| 09 | [Drag Polar](https://computationaldesignlab.github.io/aircraft-design/aerodynamics/drag_polar_induced_drag.html), [Engine and Propeller Selection](https://computationaldesignlab.github.io/aircraft-design/powerplant/engine_propeller.html), [Mission analysis](https://computationaldesignlab.github.io/aircraft-design/performance/mission_analysis.html), and Initial Weight Estimation/Weight and Balance |
| 01, 05, 10, 11 | Package, expose and preserve these same methods and their provenance; these are application tasks, not additional aerodynamic models |
| 12, 13 | **Outside the book.** It treats airfoil selection as context and has no section-generation method, and it has no structures chapter. Sections cite Abbott & von Doenhoff and the NACA reports; spanwise load and beam bending cite Schrenk and standard Euler–Bernoulli. Carbon material properties are always supplied evidence and have no source here |

For each implemented method, record the chapter/section URL, access date and
upstream revision if available, original notation/units, assumptions, applicability,
and any adaptation. Reproduce a relevant worked example in its original units and
SI where available, alongside independently calculated fixtures. Check equations
against the displayed code and results; document discrepancies and the chosen
interpretation rather than copying a suspected error or rounded intermediate.

Treat the book as the methodological foundation, with an explicit coverage record:
implemented book method, documented RC adaptation, or deferred/unsupported method.
Its example propulsion uses piston engines and its mission analysis uses fuel
weight fractions. Electric power, battery energy, V-tail mixing and flying-wing
handling must have explicit applicability/evidence before being claimed supported.
Example empirical constants are example inputs, not universal RC defaults. Follow
the book's own cited references when more detail is needed; supplementary sources
serve documented gaps and external-tool interfaces, not competing design recipes.

The book's full contents do not automatically expand the first release. Record
unimplemented takeoff/landing, fuselage, landing gear, cost and other analyses as
deferred. Structural coverage is split rather than wholly deferred: Task 13 implements
spar fit and stiffness, while strength, buckling, joints, fatigue and aeroelasticity
stay explicitly unsupported and stay named. A supported subset must not imply
completion of the whole book or whole-aircraft validation.

## Rules every task must preserve

- Separate **driver**, **derived value**, and **requirement**. `span <= 1.4 m` does
  not mean `span = 1.4 m` until the builder chooses that boundary.
- A rectangle has two independent values among span, area, chord, and AR. A
  trapezoid adds taper; root chord, tip chord, geometric mean chord, and MAC have
  distinct meanings. Do not implement “any three fields editable” globally.
- A computable candidate can violate requirements. Report `met`, `unmet`, or
  `unknown` separately from numeric validity and evidence quality.
- Combine all applicable required flight cases as specified in Task 04; show
  controlling cases and partial bounds when evidence is missing.
- Never silently relax constraints, overwrite drivers, invent missing polar
  data, or show stale outputs as current. Preserve saveable draft designs.
- Evaluation identities are never restored by undo/redo. Version saved drafts
  from Task 06 onward and validate compatibility before adopting loaded state.
- Distinguish alternate algebraic entry points from physical design iteration.
  The former needs explicit solve modes; the latter needs user revision or a
  bounded solver with explicit convergence/failure results.
- All aerodynamic results carry their flight condition, configuration, reference
  conventions, and assumptions. Correct arithmetic is not validated handling.

## Task order

| Task | Outcome | Dependencies |
|---|---|---|
| [01 — Project boundaries](01-project-boundaries.md) | Independent Go core, API/frontend seams, reproducible local and CI checks | None |
| [02 — Equations and lift](02-equations-and-lift.md) | Unit-safe, logged forward/inverse lift calculations | 01 |
| [03 — Geometry and configurations](03-geometry-and-configurations.md) | Planform, MAC, dihedral, Reynolds and configuration contracts | 02 |
| [04 — Workflow engine](04-workflow-engine.md) | Tested driver selection, requirements, inversions and recovery | 02, 03 |
| [05 — HTTP boundary](05-http-boundary.md) | Thin, validated API over the same Go core | 04 |
| [06 — Worksheet UI](06-worksheet-ui.md) | Standalone sizing workflows and versioned draft persistence | 05 |
| [07 — Tradeoff visuals](07-tradeoff-visuals.md) | Explain the effects of changing one driver | 04, 06 |
| [08 — Stability and controls](08-stability-and-controls.md) | Book-based conventional trim/static-margin assessment in the UI; explicit configuration coverage | 03–06 |
| [09 — Power and mission](09-power-and-mission.md) | Tested power-first UI, battery/motor feedback, energy budget | 02–06 |
| [10 — MCP sidecar](10-mcp-sidecar.md) | Calculations and curated patterns available through MCP | 04; expose later models as they exist |
| [11 — External handoff](11-external-handoff.md) | Reproducible export and analysis feedback | 04–06; relevant model tasks |
| [12 — Airfoil sections](12-airfoil-sections.md) | NACA 4/5-digit generation, coordinate ingest, spanwise lofting | 03 |
| [13 — Spar fit and stiffness](13-spar-fit-and-stiffness.md) | Carbon spar fit under taper/twist, bending and deflection | 03, 12 |

Tasks 08 and 09 are independently schedulable. Task 10 is a nice-to-have and may
move earlier if agent access becomes a driver. This table is a dependency map,
not authorization to spawn parallel agents.

Tasks 12 and 13 were added after the original eleven. They depend only on 03 and
on each other, so they are schedulable independently of 04–11, but their worksheet
surface arrives with Tasks 06 and 07 and their export surface with Task 11.

The first useful sizing release ends at Task 06 and has been delivered, and
Task 07's explanatory visuals have been delivered on top of it: dimension and
formula views, visual mass placement with a mechanical centre of gravity, and a
one-driver sensitivity sweep over the implemented outputs. Configuration choices
already exist, but handling remains explicitly unknown until Task 08's supported
assessment, and Task 07 names every aerodynamic reference location it does not
have rather than drawing one. V-tail/flying-wing handling remains unknown until
its extension is validated. A complete power-first workflow arrives in Task 09,
and the book's wing-loading/power-loading trade study waits for the same models.

## Open decisions, resolved when they become relevant

- An RC comparison dataset beyond the book's worked examples and synthetic fixtures.
- Additional quantitative handling targets beyond Task 08's minimum: roll rate at
  a specified speed, remaining pitch moment, yaw/sideslip control, or other outcomes.
  “Gentle/sport/aerobatic” can name transparent presets, not universal constants.
- OpenVSP target version and desired handoff format.
- Target autopilot platform/firmware and mission/control limits if a platform
  adapter is added; do not assume a platform from the existing GCS checkout.
- Minimal snapshot schema/version handling in Task 06; external export extensions
  in Task 11. Specific API/MCP transport is decided in its adapter task.
- If the UI choice is revisited, do so before Task 06 without moving physics out
  of Go. Compare a representative driver swap and sensitivity chart, not a demo form.

Prior work produced temporary experiments outside this checkout. Implementation
and verification claims now come from this checkout and its implementation log;
do not infer them from those earlier experiments.
