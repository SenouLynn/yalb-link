# Task 04 — Driver selection, requirements, and curated workflows

**Outcome:** UI-independent Go workflows tested before their controls are built.
Dependencies: [02](02-equations-and-lift.md), [03](03-geometry-and-configurations.md).

## Work

- Treat the supported design as a declarative parametric definition: named inputs,
  active solve modes, relationships, component placements, cases, requirements and
  evidence references. Geometry and analysis share dependency evaluation. UI edits
  and visual manipulations are commands against this definition; no derived view
  owns a second authoritative value. Use curated relationships initially rather
  than requiring a general-purpose expression language or arbitrary code execution.
- Base requirement intersection and deliberate candidate selection on the book's
  [Matching process](https://computationaldesignlab.github.io/aircraft-design/constraint_analysis/final.html),
  following the [handoff](README.md). Initially expose only implemented lift and
  geometry constraints. Its full takeoff/climb/cruise matching plot is not delivered
  by a stall-only subset; later constraints enter through the same case engine.
- Implement the span-first, mass/performance-first, mass/size-first and existing
  design journeys described in the [handoff](README.md). Reserve the power-first
  journey for Task 09 instead of fabricating an incomplete sizing formula.
- Use explicit driver sets and solve modes. Promoting a derived field to a driver
  releases a named existing driver atomically; if ambiguous, offer the valid swaps.
  Do not infer precedence from edit order or UI callbacks.
- Separate requirements from drivers: bounds/ranges, required/preferred priority,
  applicable cases, and margins. Distinguish computed/missing/invalid/stale model
  results from met/unmet/unknown requirements and assumed/measured/simulated evidence.
- Intersect bounds from all applicable required cases and identify the controlling
  case(s). For area sizing use the largest required lower bound; for a mass ceiling
  use the smallest upper bound. Preferred requirements do not tighten required
  feasibility. Aggregate required status is unmet if any required check is unmet,
  otherwise unknown if any is unknown, and met only when all are met. Preserve
  individual statuses; an empty required set makes no feasibility claim.
- “Size at stall limit” must name its case scope. The all-required-cases action
  uses their intersected bounds; a single-case action still reevaluates every
  applicable requirement. Missing evidence permits a labeled partial bound but
  cannot establish a complete feasible interval or overall compliance.
- A mass range requires actual lower and upper constraints. A stall ceiling alone
  provides an upper mass bound, not a physically justified nonzero lower bound.
  A component minimum or explicit loading range can supply one. Detect empty ranges.
- Preserve source/provenance and revision links. Input edits invalidate affected
  evidence/results. Unrelated incomplete panels must not block valid lift sizing.
- Define deterministic state transitions for driver changes, requirements, source
  changes and undo/redo. Curated patterns include rationale, required inputs,
  active drivers, outcomes and validity limits.
- Keep evaluation request identity separate from undoable design history. Every
  evaluation gets a fresh identity, including after undo, redo, loading a draft,
  or branching from an earlier state. Restoring a design never restores an active
  request identity; accept results only for the current request and input snapshot.
- Treat feedback loops as explicit revisions initially. Later numerical solvers
  need bounds, residuals, budgets and failure states; discrete component choices
  may need candidate search. Never label a final nonconverged iterate successful.

## Acceptance checks

1. Start with span `1.2 m`, AR `6`, mass `2 kg` and Task 02's air/CLmax case.
   Area is `.24 m²`; an `8 m/s` stall ceiling is unmet.
2. “Size at stall limit; keep span” produces area `.4169494048 m²`, derived
   chord/AR and the boundary stall speed. Preview resulting requirement changes.
3. Undo restores the complete former state, including driver roles.
4. With required span `<=1.2 m` and AR `>=6`, area is `<=.24 m²`. Explain its
   conflict with the `.4169494048 m²` minimum. Conditional alternatives include
   span at least `1.5816752 m` at AR 6 or mass at most `1.1512188 kg` at the
   original geometry. Do not apply either silently.
5. Switching entry points for the same physical candidate gives the same outputs.
   Input order within a mode does not alter the solution.
   Reconstructing the same definition through direct Go calls reproduces results
   without UI state. As later models arrive, extend this check to component
   placement, handling, mission results and CAD parameter relationships.
6. Boundary tolerance, preferred vs required checks, empty mass intervals,
   missing data and stale evidence have distinct tested outcomes.
7. With Task 02's mass/density/CLmax and an `8 m/s` ceiling in required cases
   `n=1` and `n=2`, minimum areas are `.4169494048` and `.8338988095 m²`.
   The all-case area lower bound is `.8338988095 m²`, controlled by `n=2`.
   At the original `.24 m²`, the combined mass ceiling is `.5756094079 kg`.
   Case order does not change results. Making `n=2` preferred removes it from
   required bounds while preserving its own assessment.
8. Remove CLmax evidence from one required case: its check becomes unknown and
   the remaining bound is partial. A known failure still makes the aggregate
   unmet; otherwise it is unknown. An area upper bound of `.5 m²` conflicts with
   the two fully specified required cases above and identifies the controlling
   lower bound. Also test tied controlling cases and an empty required set.
9. Edit, undo, then make a different edit while earlier evaluations are pending.
   Request identities remain unique, and earlier results cannot be accepted for
   the new branch. Repeat for redo and draft loading.

Numeric invalidity blocks dependent calculations. Unmet requirements leave the
candidate editable and saveable as a draft. Show the known conflicting group;
do not claim globally minimal constraint conflicts without a solver.
