# Task 07 — Show how a change affects the design

**Outcome:** informative visual feedback showing cause, consequence and constraints.
Dependencies: [04](04-workflow-engine.md), [06](06-worksheet-ui.md).

## Work

- Let the user select an active driver and a bounded range. Go evaluates each
  candidate using the same workflow; the frontend plots returned physical data.
- State what remains fixed. At fixed span, increasing AR reduces area; at fixed
  area, increasing AR increases span. A plot without its driver mode is ambiguous.
- Show a selected output curve, current candidate marker, requirement boundary,
  and feasible/unknown regions. Include before/after values for affected outputs.
  Offer a compact table alternative and keyboard selection.
- Keep requirements fixed during a sensitivity sweep unless the user explicitly
  chooses to explore a requirement. Sampling does not commit a new candidate.
- Show gaps for invalid/unsupported samples; do not join a line through them or
  interpolate an uncomputed feasible boundary as an exact result.
- Keep a snapshot/revision with each request. Cancel or discard superseded sweeps.
  Bound sample counts and work; parallelize independent evaluations only when
  useful, retaining deterministic output order and cancellation behavior.
- Treat assumption ranges separately from confidence intervals. Unknown airfoil
  quality does not become a statistically meaningful shaded band automatically.

## Acceptance checks

- Every plotted sample matches direct Go evaluation of that candidate.
- For the Task 02 model, the expected fixed-input trends hold: mass increases
  stall speed; more area lowers stall speed; higher AR at fixed span raises it.
- In mass/performance sizing, changing span can leave target stall speed constant
  while changing AR/chord; the visual explains that mode rather than suggesting
  span has no effect on aircraft design.
- Inactive/derived fields cannot be swept without a valid driver change.
- Invalid samples, empty feasible regions, request cancellation and stale-result
  suppression are tested. Sweeps do not mutate the saved candidate.
- Visuals remain useful in a narrow viewport and without color perception.
