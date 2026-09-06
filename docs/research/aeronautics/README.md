# Aeronautics calculator — task handoff

Status: **planning only; no calculator implementation is included in this plan.**
These tasks record the user's requested future work, not existing capabilities or
accepted API contracts. Updated 2026-09-05.

## Start here in a fresh session

1. Confirm the active workspace is `/Users/senoulynn/Desktop/yalb-link`.
   These planning documents belong in this checkout. The older `ligma-gcs`
   directory is a separate checkout and must not receive implementation changes
   for this work.
2. Read this file, the selected task, and applicable repository instructions.
3. Implement the selected task through its acceptance checks. Report what is
   implemented, what was tested, and any remaining model limitations.

Suggested continuation prompt:

> Read docs/research/aeronautics/README.md and implement Task 01 only, including
> its checks. Keep the calculator independent of the existing GCS frontend and
> services. Do not start another task automatically.

## User intent and settled direction

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
- An optional **MCP sidecar** should expose the same calculations and curated,
  versioned (“blessed”) workflows. It must not duplicate the calculation engine.
- Eventually hand dimensions and requirements to Fusion 360, XFLR5/flow5, and a
  NASA analysis tool. The NASA tool is **unconfirmed**; do not assume OpenVSP,
  VSPAERO, or FUN3D interchangeably.

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

## Rules every task must preserve

- Separate **driver**, **derived value**, and **requirement**. `span <= 1.4 m` does
  not mean `span = 1.4 m` until the builder chooses that boundary.
- A rectangle has two independent values among span, area, chord, and AR. A
  trapezoid adds taper; root chord, tip chord, geometric mean chord, and MAC have
  distinct meanings. Do not implement “any three fields editable” globally.
- A computable candidate can violate requirements. Report `met`, `unmet`, or
  `unknown` separately from numeric validity and evidence quality.
- Never silently relax constraints, overwrite drivers, invent missing polar
  data, or show stale outputs as current. Preserve saveable draft designs.
- Distinguish alternate algebraic entry points from physical design iteration.
  The former needs explicit solve modes; the latter needs user revision or a
  bounded solver with explicit convergence/failure results.
- All aerodynamic results carry their flight condition, configuration, reference
  conventions, and assumptions. Correct arithmetic is not validated handling.

## Task order

| Task | Outcome | Dependencies |
|---|---|---|
| [01 — Project boundaries](01-project-boundaries.md) | Independent Go core, API/frontend seams, reproducible local checks | None |
| [02 — Equations and lift](02-equations-and-lift.md) | Unit-safe, logged forward/inverse lift calculations | 01 |
| [03 — Geometry and configurations](03-geometry-and-configurations.md) | Planform, MAC, dihedral, Reynolds and configuration contracts | 02 |
| [04 — Workflow engine](04-workflow-engine.md) | Tested driver selection, requirements, inversions and recovery | 02, 03 |
| [05 — HTTP boundary](05-http-boundary.md) | Thin, validated API over the same Go core | 04 |
| [06 — Worksheet UI](06-worksheet-ui.md) | Standalone Vite/React sizing workflows | 05 |
| [07 — Tradeoff visuals](07-tradeoff-visuals.md) | Explain the effects of changing one driver | 04, 06 |
| [08 — Stability and controls](08-stability-and-controls.md) | Configuration-specific tail/control and handling checks | 03–06 |
| [09 — Power and mission](09-power-and-mission.md) | Weight/power path, battery/motor feedback, energy budget | 02–06 |
| [10 — MCP sidecar](10-mcp-sidecar.md) | Calculations and curated patterns available through MCP | 04; expose later models as they exist |
| [11 — External handoff](11-external-handoff.md) | Reproducible export and analysis feedback | 04–06; relevant model tasks |

Tasks 08 and 09 are independently schedulable. Task 10 is a nice-to-have and may
move earlier if agent access becomes a driver. This table is a dependency map,
not authorization to spawn parallel agents.

The first useful sizing release ends at Task 06. Configuration choices already
exist, but handling remains explicitly unknown until Task 08. Task 07 adds the
requested explanatory visuals. A complete power-first workflow arrives in Task 09.

## Open decisions, resolved when they become relevant

- A real aircraft/example dataset for comparison beyond synthetic arithmetic.
- Quantitative handling targets: approach trim/CG range, roll rate at a specified
  speed, remaining pitch moment, yaw/sideslip control, or other selected outcomes.
  “Gentle/sport/aerobatic” can name transparent presets, not universal constants.
- Target NASA tool, versions and desired handoff format.
- Persistence/export schema and specific API/MCP transport: decide in their tasks.
- If the UI choice is revisited, do so before Task 06 without moving physics out
  of Go. Compare a representative driver swap and sensitivity chart, not a demo form.

Prior work produced only temporary experiments outside this checkout. Treat task
implementations as unstarted; do not assume the TypeScript or Go calculator code
mentioned earlier in the conversation exists or has passed tests.
