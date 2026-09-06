# Task 11 — Reproducible export and analysis feedback

**Outcome:** the builder can carry a candidate into external engineering tools
and deliberately bring compatible analysis results back.
Dependencies: [04](04-workflow-engine.md) through [06](06-worksheet-ui.md), plus
the geometry/control/power models used by each adapter.

## Work

- Export the supported parametric definition and its relationships as well as
  evaluated values. The Fusion sketch is a target-specific representation of that
  definition. Identify relationships that remain calculator-only, such as flight
  requirements or aerodynamic evaluations, rather than implying Fusion executes
  the whole workflow. CAD edits return through explicit adoption and reevaluation;
  bidirectional live synchronization is not required by this task.
- Extend Task 06's versioned snapshot contract with a readable parameter table
  and adapter-specific data. Preserve compatibility or provide tested explicit
  migrations; do not introduce a competing authoritative design snapshot.
- Carry the book chapter references, source revisions/access dates and documented
  adaptations with exported methods and results under the [handoff](README.md).
  External formats use their target tool's documentation as supplementary evidence.
- Preserve RC mission cases, avionics/payload budgets, intended control modes,
  command limits, mixing and actuator constraints when available. A design snapshot
  is not a validated autopilot parameter file. Any future firmware-specific export
  requires an explicit adapter and target-version verification; this task does not
  authorize live parameter uploads or flight-controller tuning.
- Include authoritative drivers, requirements, units, geometry/datum/axes,
  reference area/span/MAC, mass/CG/inertia as available, flight cases, airfoil IDs,
  assumptions, model revisions, sources and unmet/unknown checks.
- Save invalid/incomplete drafts without representing missing or stale numeric
  outputs as valid. Reevaluate snapshots if the active model revision differs.
- Select a Fusion 360 parameter workflow and verify it; do not claim a generic
  CSV is a directly importable format without checking the target setup.
- Deliver a concrete sketch handoff for the supported rectangle and trapezoid:
  named user parameters, units, values/expressions, origin/axes, construction lines,
  symmetry and a diagram showing which sketch dimension uses each parameter.
  Start with a documented manual parameter-entry/sketch procedure if necessary;
  automation is optional, but reconstruction in Fusion must actually be exercised.
- Map Task 03's stable keys to valid Fusion parameter names and translate the
  active dependency graph into verified expressions in dependency order. Check
  unit/angle semantics and supported functions against the target Fusion version.
  Export independent drivers as editable parameters and appropriate derived
  relationships as expressions/reference dimensions, avoiding cycles and redundant
  driving constraints. Offer an explicitly labeled frozen-value snapshot separately
  when relationships cannot be represented; do not silently flatten parametric data.
- Connect the verified parameter table and expressions to Task 07's dimension and
  formula views if available; otherwise include a static annotated sketch guide.
  Identify planform versus physical panel sketches, reference-only MAC/CG markers,
  and any missing airfoil/loft/manufacturing detail. Component CG station markers
  do not by themselves create a mass-accurate CAD assembly.
- Verify XFLR5/flow5 versions and supported geometry/data formats before writing
  adapters. Check station, airfoil, control, mass, axes and coefficient references.
- Confirm the NASA tool first. OpenVSP is a geometry environment and VSPAERO a
  potential-flow analysis tool; FUN3D is a distinct CFD toolchain. They are not
  interchangeable assumptions about “dynamic flow testing.”
  [NASA OpenVSP](https://www.nasa.gov/software/openvsp-ground-school/),
  [VSPAERO references](https://www.nasa.gov/reference/openvsp-vspaero-basics/),
  [FUN3D](https://fun3d.larc.nasa.gov/chapter-1.html).
- Attach returned analysis to its candidate revision and flight condition. Preview
  changes before the builder adopts new aerodynamic assumptions. Never silently
  overwrite newer geometry or combine incompatible coefficient normalizations.

## Acceptance checks

- Snapshot round trip preserves physical values, driver roles, requirements and
  provenance; model-version differences are handled explicitly.
- Malformed imports, incompatible units/references, unsupported versions and stale
  analysis produce helpful errors without losing the current candidate.
- Each claimed external adapter has representative files verified against its
  named tool/version. A unit test against self-generated output is insufficient.
- In the named Fusion version, build representative rectangle/trapezoid sketches
  from the handoff, including a supported sweep/dihedral case with explicit sketch
  planes. Change an independent span or taper parameter and verify derived chords,
  geometry and area against Go evaluation. Confirm the intended constraint state
  without redundant driving dimensions, and record the procedure and checked values.
  A driver-mode change must produce a consistent replacement parameter graph.
- Draft vs complete exports remain distinguishable. Adoption of imported results
  triggers the appropriate requirement checks and evidence invalidation.

Structural load envelope, wing stiffness/bending, aeroelasticity and manufacturing
detail remain additional engineering work unless separately implemented. Export
must preserve these unknowns rather than imply whole-aircraft validation.
