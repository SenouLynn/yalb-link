# Task 04 — Driver selection, requirements, and curated workflows

**Outcome:** UI-independent Go workflows tested before their controls are built.
Dependencies: [02](02-equations-and-lift.md), [03](03-geometry-and-configurations.md).

## Work

- Implement the span-first, mass/performance-first, mass/size-first and existing
  design journeys described in the [handoff](README.md). Reserve the power-first
  journey for Task 09 instead of fabricating an incomplete sizing formula.
- Use explicit driver sets and solve modes. Promoting a derived field to a driver
  releases a named existing driver atomically; if ambiguous, offer the valid swaps.
  Do not infer precedence from edit order or UI callbacks.
- Separate requirements from drivers: bounds/ranges, required/preferred priority,
  applicable cases, and margins. Distinguish computed/missing/invalid/stale model
  results from met/unmet/unknown requirements and assumed/measured/simulated evidence.
- A mass range requires actual lower and upper constraints. A stall ceiling alone
  provides an upper mass bound, not a physically justified nonzero lower bound.
  A component minimum or explicit loading range can supply one. Detect empty ranges.
- Preserve source/provenance and revision links. Input edits invalidate affected
  evidence/results. Unrelated incomplete panels must not block valid lift sizing.
- Define deterministic state transitions for driver changes, requirements, source
  changes and undo/redo. Curated patterns include rationale, required inputs,
  active drivers, outcomes and validity limits.
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
6. Boundary tolerance, preferred vs required checks, empty mass intervals,
   missing data and stale evidence have distinct tested outcomes.

Numeric invalidity blocks dependent calculations. Unmet requirements leave the
candidate editable and saveable as a draft. Show the known conflicting group;
do not claim globally minimal constraint conflicts without a solver.
