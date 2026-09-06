# Task 07 — Show how a change affects the design

**Outcome:** informative visual feedback showing cause, consequence and constraints.
Dependencies: [04](04-workflow-engine.md), [06](06-worksheet-ui.md).

## Work

- Make the visuals live views/editors of Task 04's parametric design definition.
  Dimension edits, component drags and driver controls use the same commands,
  dependency invalidation and undo history. Selecting a preview candidate commits
  its parameters explicitly; a plotted or drawn result is never independent state.
- Use the book's [Matching process](https://computationaldesignlab.github.io/aircraft-design/constraint_analysis/final.html)
  and Trade study chapters (via the [handoff](README.md)) as the visual methodology.
  Plot only implemented models. The initial one-driver sweep is a documented
  subset; wing/power-loading plots require Task 09's applicable power models.
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
  Give each sweep a fresh evaluation identity under Task 04's history rules;
  match both its input snapshot and sweep settings before accepting a response.
  Bound sample counts and work; parallelize independent evaluations only when
  useful, retaining deterministic output order and cancellation behavior.
- Treat assumption ranges separately from confidence intervals. Unknown airfoil
  quality does not become a statistically meaningful shaded band automatically.

## Dimension and formula views for Fusion sketches

- Show dimensioned plan, side and front views as appropriate, with sketch origin,
  axes, centerline, symmetry and construction geometry. Label full versus half
  span, root/tip chord, sweep reference and dihedral using Task 03's parameter keys.
  Distinguish projected plan dimensions from physical panel dimensions visually.
- Selecting a dimension highlights its worksheet field and displays the symbolic
  relationship, actual substitutions, units and result supplied by Go. Selecting a
  field highlights its dimension. Show a compact dependency view from active
  drivers to derived dimensions; a driver swap updates both the view and formulas.
- For example, a span/area/taper-driven trapezoid shows `half_span = span/2`,
  `root_chord = 2*area/(span*(1+taper))` and `tip_chord = taper*root_chord`.
  These are explanatory mathematical relationships; Task 11 verifies their
  translation to Fusion expressions and dimensional units before claiming they
  are directly pasteable. Do not export every relationship as a driving constraint.
- Provide a copyable parameter table with names, units, full-precision values,
  driver/derived roles and formulas. Link each row to the sketch feature it defines.
  Task 11 adds the verified Fusion-ready representation; until then label the
  table as a generic geometry handoff. Keep requirements such as maximum span
  separate from actual sketch dimensions.

## Visual mass placement and lift

- Add coordinated plan and side views with a consistent aircraft datum and scale.
  Show component mass markers, mechanical CG, wing outline and MAC. Make battery,
  motor, avionics and payload positions draggable, with equivalent keyboard/numeric
  editing and explicit coordinates. Show before/after values during placement.
- Establish a shared Go mass-properties calculation using the book's Weight and
  Balance/Center of gravity chapter, reached through the [handoff](README.md).
  Component-derived CG is `sum(m_i * r_i) / sum(m_i)` in the declared axes; account
  for airframe mass and position as well as removable components. Missing mass or
  position produces an incomplete assessment, not an assumed origin. Keep entered
  all-up mass/CG as a separate mode until a complete component model is adopted.
  Task 09 extends this same model for electrical budgets and component feedback;
  it must not create a second mass/CG authority.
- Use bounded preview evaluations through Go during dragging. The frontend maps
  screen coordinates to design coordinates but contains no mass-property or
  aerodynamic equations. Commit a completed drag as one undoable placement change;
  cancel restores the previous placement. Preserve placements in Task 06 snapshots.
- Distinguish mechanical CG, wing aerodynamic center, aircraft neutral point and
  center of pressure. Label each displayed reference and its evidence. A geometric
  quarter-MAC marker is only a geometric reference unless a model supports its
  aerodynamic interpretation. Do not label it a universal “center of lift.”
- Initially show the lumped required lift magnitude from Task 02 without inventing
  a line of action or spanwise pressure distribution. With Task 08, add supported
  wing/tail force locations, trim loads and CG/neutral-point/static-margin overlays
  for the selected case. Missing force-location evidence leaves those arrows or
  markers unknown. The visual is a quasi-static design preview, not flight dynamics.
- Explain distinct causes: moving a fixed component changes CG but leaves total
  mass, wing loading and Task 02's lumped stall speed unchanged. Adding/removing or
  resizing its mass changes those sizing results. CG-dependent trim can change
  usable lift limits only through a supported coupled model; report trim feasibility
  separately and leave any unmodeled stall correction unknown.

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
- Delay a sweep across edit → undo → different edit and across a change of range
  or output selection. An obsolete response cannot replace the current plot even
  when the design returns to a previously visited history revision.
- Visuals remain useful in a narrow viewport and without color perception.
- Browser tests select a dimension and its corresponding field in both directions,
  inspect its formula/substitutions, change a driver and verify updated dimensions
  and dependency links. Copying parameter values preserves physical precision.
  Formula and relationship information is also accessible without the diagram.
- Use an independent placement fixture: `1.5 kg` of airframe at `x=.4 m` and a
  `.5 kg` battery at `x=.2 m` gives `2 kg` total and `xCG=.35 m`. Moving the
  battery to `x=.6 m` gives `xCG=.45 m` with unchanged Task 02 loading/stall speed.
  Increasing that battery to `1 kg` gives `2.5 kg` and `xCG=.48 m`; with Task 02's
  other assumptions fixed, stall speed increases by `sqrt(2.5/2)`. Also test lateral
  and vertical placements, mixed units and incomplete mass/position evidence.
- Browser tests drag and numerically edit a component to the same coordinates,
  compare Go results, cancel a preview, undo/redo a committed drag and save/reopen
  the layout. Delayed preview responses cannot overwrite a later placement.
- Before Task 08, unknown aerodynamic locations/trim effects are visibly unknown.
  Once Task 08 is implemented, its complete conventional fixture exercises force
  and stability overlays, including a CG move that loses trim feasibility.
