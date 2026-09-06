# YALB Aero

Independent fixed-wing RC aircraft calculator project. The current implementation
is a transport-free Go library covering units, the initial lift calculations and
wing geometry, a version-only executable, and a static Vite/React page. No HTTP
service, sizing workflow, handling prediction, power model or external-tool
adapter is implemented yet, and the frontend does not yet display any calculation.

## What the library does

`yalb.aero/calculator` provides dimensioned quantities, an equation registry, the
first lift calculations and the wing geometry model:

- **Units.** A `Quantity` holds a finite value in SI with its `Dimension`.
  Mass, weight, area, speed, density, wing loading, angle, power and energy are
  distinct: reading a mass in newtons fails, and so does a pound mass read as a
  pound-force. Conversions cover the metric and customary units RC builders use,
  including `g/dm^2` and `oz/ft^2`.
- **Equations.** Each has an ID, revision, expression, declared input and output
  ports, a source record and stated assumptions. `Lookup` returns a deep copy, so
  inspecting metadata cannot change it for anyone else.
- **Traces.** Every evaluation returns the values actually substituted, with the
  equation ID and revision, and performs no I/O and records no timestamp.
- **Issues.** Failures are typed as missing, invalid or unsupported, per field,
  and one evaluation reports every bad field at once. Ordinary bad input never
  panics and never returns a successful NaN.
- **Flight cases.** A case carries its configuration, air density, load factor
  and CLmax. The density basis and the CLmax basis and scope must be stated, so a
  result cannot rest on an invisible assumption. A 2D airfoil section `c_lmax` is
  refused where a whole-aircraft `C_Lmax` is required, rather than reinterpreted.

Implemented lift: weight, dynamic pressure, required lift, required lift
coefficient, force and mass wing loading, stall speed, and the inversions for
minimum wing area, maximum mass and maximum wing loading. This is a lumped lift
model with no separate tail trim load; correct arithmetic here is not validated
handling. See [source mappings](docs/reference/sources.md) for provenance and
coverage.

## Wing geometry

`SolvePlanform` covers a rectangle and a symmetric trapezoid. A planform has
exactly two independent size values among span, area, aspect ratio and root
chord, plus the taper ratio for a trapezoid; there is no "any three fields
editable" mode. Each supported pair is a named solve path, an under-determined
set reports what is missing, and an over-determined one is reported as redundant
when the extra values agree and as conflicting when they do not.

- **Reference dimensions.** Root, tip, geometric mean and mean aerodynamic chord
  are distinct values; the MAC is also located spanwise and fore-and-aft against
  a named datum, which is what a later centre-of-gravity position needs.
- **Two planes.** `SolveWing` reports the wing both as a plan-view projection and
  as the panel that gets built. Dihedral requires the builder to say which of the
  two stays fixed, and the coordinates for each plane are separate.
- **Coordinates.** `Outline` returns the right panel's corners in the wing datum,
  in either plane. They reconstruct the solved span, area and chords. An outline
  is not a manufacturing drawing and describes no airfoil section.
- **Limits are not drivers.** `PlanformLimits` is a separate type the solver
  never sees, and its checks report met, unmet or unknown. A maximum span becomes
  a span only when the builder enters it as one.
- **Parameters.** `Wing.Parameters` exports stable, transport-neutral keys with
  their unit, datum, driver or derived role, the equation revision behind each
  derived value and what it depends on. `EvaluationOrder` fails on a cycle or a
  dangling edge rather than assuming the graph is sound.
- **Reynolds.** `ReynoldsCoverage` evaluates the root, MAC and tip together and
  reports MAC-only coverage as incomplete, because a tapered RC tip can sit in a
  materially lower regime than its MAC. Viscosity is stated evidence, like density.
- **Configurations.** Conventional, V-tail and flying-wing layouts are explicit
  from the start. A flying wing is never asked for a horizontal tail, and a
  V-tail records panel area and cant rather than imaginary independent surfaces.
  All of this is geometry: `Handling` reports unsupported for every configuration.

## Boundaries

The domain package is `yalb.aero/calculator`. Application binaries live in `cmd/`;
transport and storage belong outside that package. There is no `go.work`, root
pnpm workspace, GCS package import or GCS frontend dependency. Go is pinned to
1.25.0 in `go.mod`; CI uses that version. Frontend dependencies have their own
`pnpm-lock.yaml`. `go.sum` is empty: the module uses only the standard library,
and the core's transitive closure is `math`, `sort`, `strconv` and `errors`.

The calculator uses an independent copy of the repository's strict Go lint rules,
without its generated-GCS-code exclusion. No `wrapcheck` or `funlen` relaxation is
in effect. Frontend strict TypeScript configuration is entirely module-local.

`boundary/imports_test.go` inspects production transitive imports using `go list
-deps`. The named stdlib allowlist (`math`, `fmt`, `errors`, `strconv`, `sort`,
`testing`) includes their toolchain implementation dependencies; the explicit
`net`, `os`, `database/sql` and `time` bans override that expansion, including
subpackages. Thus importing `fmt` into the core is currently disallowed because
its closure reaches `os`, which is why the core formats numbers with `strconv`.
The test runner's own I/O lives outside the core.

One weakness is worth knowing: because `testing` is on the allowlist, its closure
also admits pure packages such as `strings`. The core does not rely on that, but
the allowlist would not stop it today.

Run the executable from this directory with `go run ./cmd/aero`; it prints
`yalb.aero 0.0.0` and exits. The frontend starts independently with `pnpm dev` from
`frontend/` at <http://127.0.0.1:5173>. It has no calculation backend to start yet.

Root `make` and `bazel test //...` do **not** check this nested module. Its own
[CI workflow](../.github/workflows/aeronautics.yml) runs Go build, vet, race tests
and lint, plus frontend typecheck, lint, test and production build. The local
commands and verification record are in the
[runbook](../docs/runbooks/aeronautics.md) and
[implementation log](../docs/reference/aeronautics-implementation.md).

The [task breakdown](../docs/research/aeronautics/README.md) defines subsequent
work. [Source mappings](docs/reference/sources.md) and
[reference fixtures](calculator/testdata/README.md) have dedicated locations.
