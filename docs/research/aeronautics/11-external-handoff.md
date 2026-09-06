# Task 11 — Reproducible export and analysis feedback

**Outcome:** the builder can carry a candidate into external engineering tools
and deliberately bring compatible analysis results back.
Dependencies: [04](04-workflow-engine.md) through [06](06-worksheet-ui.md), plus
the geometry/control/power models used by each adapter.

## Work

- Begin with a readable parameter table and versioned design snapshot. Design its
  concrete schema at implementation time around existing models/tests.
- Include authoritative drivers, requirements, units, geometry/datum/axes,
  reference area/span/MAC, mass/CG/inertia as available, flight cases, airfoil IDs,
  assumptions, model revisions, sources and unmet/unknown checks.
- Save invalid/incomplete drafts without representing missing or stale numeric
  outputs as valid. Reevaluate snapshots if the active model revision differs.
- Select a Fusion 360 parameter workflow and verify it; do not claim a generic
  CSV is a directly importable format without checking the target setup.
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
- Draft vs complete exports remain distinguishable. Adoption of imported results
  triggers the appropriate requirement checks and evidence invalidation.

Structural load envelope, wing stiffness/bending, aeroelasticity and manufacturing
detail remain additional engineering work unless separately implemented. Export
must preserve these unknowns rather than imply whole-aircraft validation.
