# YALB Aero

Independent fixed-wing RC aircraft calculator project. The current implementation
is a transport-free Go library covering units, the initial lift calculations,
wing geometry, mass properties and a sizing workflow engine; a JSON HTTP
boundary over it; an executable that serves that boundary; and a Vite/React
worksheet that drives the running service, draws the dimensioned views, places
component masses and plots one-driver sensitivity sweeps. No handling
prediction, power model or external-tool adapter is implemented.

## What the library does

`yalb.aero/calculator` provides dimensioned quantities, an equation registry, the
first lift calculations, the wing geometry model and the sizing workflow engine:

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

## Sizing workflows

A `Design` is the authoritative parametric definition: named inputs, the two size
drivers the builder chose, the flight cases the candidate must hold, and the
requirements it is judged against. Nothing else owns a second copy of a value
that appears in it, and every edit goes through a `Command`, so history, driver
roles and evaluation identity cannot disagree about what happened. The `Command`
interface has an unexported method: the supported edits are a curated set, not a
general-purpose expression language or arbitrary caller code.

- **Drivers move atomically.** `PromoteDriver` makes a derived value a driver and
  releases a named existing one in the same edit. A planform holds exactly two
  size drivers and every pair is a supported solve path, so a promotion is
  normally ambiguous; without a named release it returns the valid swaps rather
  than picking one, and `SetDriver` on a derived value refuses and names the same
  swaps. Nothing infers precedence from edit order.
- **Requirements are not drivers.** A `Requirement` bounds a subject — stall
  speed in a named case, span, area, mass, aspect ratio or mass wing loading —
  with a minimum, a maximum, an optional relative margin and a priority. Its
  outcome (`met`, `unmet`, `unknown`) is reported separately from whether the
  model produced the number at all (`computed`, `missing`, `invalid`, `stale`)
  and from how good the evidence behind it is (`assumed`, `measured`,
  `simulated`).
- **Bounds intersect and name their controlling case.** `AreaLowerBound` takes
  the largest lower bound over the required cases and `MassUpperBound` the
  smallest upper bound, both listing every contribution and every controlling
  source, ties included. A required case that could not contribute marks the
  bound `Partial`: missing evidence permits a labeled partial bound and never a
  complete feasible interval. A preferred case or a preferred requirement is
  assessed and contributes nothing. An empty required set makes no feasibility
  claim; it is not a passing one.
- **A mass range needs both ends.** A stall ceiling bounds mass from above and
  justifies no nonzero lower bound; a component minimum or a minimum wing loading
  supplies one, and an empty interval is detected and explained.
- **Conflicts are reported with alternatives, not resolved.** A required area
  minimum above a required area maximum — including the maximum implied by a
  span limit with an aspect-ratio target — is reported as a *known* conflicting
  group, explicitly not a minimal one, since no solver runs. Each `Alternative`
  carries the `Command` that would resolve it, unapplied, and says what it would
  cost.
- **Evaluation identity is separate from history.** Every `Session.Evaluate`
  mints a fresh `RequestID`. `Do`, `Undo`, `Redo` and `Load` all retire the
  current identity, so a result computed before a branch can never be accepted
  after it — including when an undo restores the exact inputs the result came
  from. A `Session` is single-goroutine: the core reaches neither a clock nor
  `sync/atomic`, and serialization belongs to whatever serves it.
- **Curated patterns, including the one that is missing.** `Patterns()` returns
  the span-first, mass-and-performance-first, mass-and-size-first and
  existing-design workflows with their rationale, required inputs, active
  drivers, outcome and validity limits. The power-first journey is registered as
  unsupported and is never offered: power alone cannot determine a wing, and no
  sizing formula was invented for it.

The constraint set is stall-only. This is not the book's takeoff, climb and
cruise matching plot, and nothing here presents it as one; later constraints
enter through the same case engine as their models arrive.

## Mass properties and sensitivity

- **A balance, not a guess.** `Design.MassProperties` totals the listed component
  masses and divides their moment sum by it, on all three axes, from the book's
  Center of gravity relation. A component with no mass, or with any coordinate
  unstated, contributes nothing: it is never read as weightless and never placed
  at the origin, and the result says which one is incomplete. Zero is spelled
  out, exactly as the wing angles are.
- **Two mass modes, kept apart.** `MassMode` selects whether the all-up mass is
  the entered figure or the component total. Adopting the components never
  overwrites the entered one, and an incomplete inventory establishes no all-up
  mass, so every result resting on it waits rather than being computed against a
  partial aircraft.
- **Moving is not resizing.** `PlaceComponent` carries a position and nothing
  else, so it changes the balance and leaves the total, the wing loading and the
  stall speed alone. `SetComponent` carries the item, so changing its mass moves
  all of them.
- **Mechanical only.** The centre of gravity is not an aerodynamic centre, a
  neutral point or a centre of pressure. `StationFractionOfMAC` expresses a
  station as a chord fraction as a geometric reference; it is not a static
  margin, and no handling conclusion follows from any of it.
- **One driver at a time.** `Design.PlanSweep` validates a sensitivity request
  and reports what it holds fixed; `SweepPlan.Run` evaluates between 2 and 65
  candidates, each an ordinary design produced by the ordinary driver edit, so a
  plotted sample *is* the candidate. A derived value cannot be swept: moving it
  would mean a promotion, which releases another driver and is the builder's
  decision. Samples the model cannot evaluate are gaps, and `Status` never
  collapses into `Feasibility`.
- **A plot carries its driver mode.** The answer states the solve mode and what
  stayed fixed, and where the plotted output does not move it names the derived
  parameters that did — at a fixed area a longer span leaves the stall speed
  alone and changes the aspect ratio and every chord. This is a documented
  one-driver subset, not the book's wing-loading/power-loading trade study, which
  needs the propulsion models Task 09 brings.
- **Dimensions and formulas.** `Wing.Views` returns dimensioned plan, front and
  side views with the sketch origin, axes, centerline and construction geometry,
  and every dimension carries the parameter key it measures and that parameter's
  value. `Wing.Explain` returns the relationship, revision, substituted values
  and dependencies behind any parameter. Projected and panel dimensions are
  distinguished; a projected outline is not a cutting template.

## The HTTP boundary

`yalb.aero/api` is the application boundary: the wire contract and the
translation between it and the core. `yalb.aero/httpapi` serves that boundary
over HTTP. The split is the point — `api` imports no `net/http`, so a future MCP
sidecar reuses it directly instead of making loopback calls, and the calculator
knows about neither.

- **The service is stateless.** A request carries the whole design; the design
  history, undo and the evaluation request stream belong to the client that owns
  them. Session state on the server would make two clients of one design fight
  over a single history and put a second authoritative copy of the definition
  somewhere the builder cannot see.
- **Identity crosses the boundary intact.** Every call carries the client's
  `{session, sequence}`, the response echoes it in the body and in the
  `X-Aero-Request` and `X-Aero-Sequence` headers, and it carries the input
  snapshot the service computed. A client can therefore discard a late answer
  without decoding it.
- **Strict decoding.** The body must be `application/json` and hold exactly one
  value; unknown fields are refused rather than ignored, because a client's typo
  that is silently dropped looks like a call that succeeded and quietly did
  something else. Bodies are capped at 1 MiB, and the case, requirement, command
  and batch counts are capped in the `api` package so every adapter is bounded
  the same way. The limits are published in the discovery document.
- **Statuses distinguish the kinds of failure.** A malformed or invalid request
  is 400, an unregistered equation or path is 404, an input outside the
  implemented models is 422, an oversized body is 413. A design that does not
  *solve* is none of these: it comes back 200 with the core's typed issues,
  because an unfinished candidate is still editable and a worksheet has to show
  it.
- **The Go types are the one contract authority.** `go run ./cmd/aero contract`
  regenerates `frontend/src/api/contract.ts` from them, and a test fails when the
  checked-in file drifts. Those are types, not validation: they vanish at run
  time and do not check that an untrusted body has the shape they describe.

Routes live under `/api/v1`: `GET discovery`, `GET equations`,
`GET equations/{id}`, `GET patterns`, `GET units`, and `POST evaluate`,
`evaluate-batch`, `apply`, `preview` and `sweep`.

A sweep answer carries the input snapshot and a fingerprint of the settings it
answers, because a curve belongs to a question: a different range or a different
plotted output is a different one, and an answer to either is not an answer to
the other. Sampling is evaluated one candidate at a time and stops when the
caller goes away.

Run it with `go run ./cmd/aero serve` (default `127.0.0.1:8081`); `pnpm dev` in
`frontend/` proxies `/api` to it. A deployment needs this process as well as the
static assets — a static host serves the page but does not execute the core.

## Boundaries

The domain package is `yalb.aero/calculator`. The application boundary is
`yalb.aero/api` and its HTTP transport is `yalb.aero/httpapi`; application
binaries live in `cmd/`. Storage belongs outside all of them. There is no `go.work`, root
pnpm workspace, GCS package import or GCS frontend dependency. Go is pinned to
1.25.0 in `go.mod`; CI uses that version. Frontend dependencies have their own
`pnpm-lock.yaml`. `go.sum` is empty: the module uses only the standard library,
and the core's transitive closure is `math`, `sort`, `strconv` and `errors`.

The calculator uses an independent copy of the repository's strict Go lint rules,
without its generated-GCS-code exclusion. No `wrapcheck` or `funlen` relaxation is
in effect. Frontend strict TypeScript configuration is entirely module-local.

`boundary/imports_test.go` inspects production transitive imports using `go list
-deps`. The allowlist is the toolchain closure of the four packages the core may
actually reach — `math`, `errors`, `strconv` and `sort` — and the explicit `net`,
`os`, `database/sql` and `time` bans override that expansion, including
subpackages. Thus importing `fmt` into the core is disallowed because its closure
reaches `os`, which is why the core formats numbers with `strconv`. The test
runner's own I/O lives outside the core.

Task 01 names `fmt` and `testing` as well, and both are deliberately left out of
the seed: `testing` is a dependency of the tests rather than of the non-test
closure this check reads, and seeding either widened the allowlist from 38
packages to 71, quietly admitting `strings`, `bytes`, `io`, `reflect`, `context`,
`sync/atomic`, `flag` and `path/filepath`.
`TestAllowlistDoesNotAdmitTestOnlyPackages` pins that narrowing, so admitting any
of them into the core is a decision rather than an accident.

`boundary/imports_test.go` also holds the Task 05 seams: `yalb.aero/api` may not
reach `net/http`, and `yalb.aero/calculator` may not reach `api`, `httpapi`,
`encoding/json` or `context`. `TestTransportDoesNotReimplementTheCore` parses the
two boundary packages and fails on any multiplication, division, subtraction or
remainder in them, so every number a response carries came out of the core rather
than from a unit conversion copied into a transport.

Run the executable from this directory: `go run ./cmd/aero` prints the version
and the contract version, `go run ./cmd/aero contract` prints the TypeScript
contract, and `go run ./cmd/aero serve` serves the API. The frontend starts
independently with `pnpm dev` from `frontend/` at <http://127.0.0.1:5173> and
proxies `/api` to the server.

Root `make` and `bazel test //...` do **not** check this nested module. Its own
[CI workflow](../.github/workflows/aeronautics.yml) runs Go build, vet, race tests
and lint, plus frontend typecheck, lint, test and production build. The local
commands and verification record are in the
[runbook](../docs/runbooks/aeronautics.md) and
[implementation log](../docs/reference/aeronautics-implementation.md).

The [task breakdown](../docs/research/aeronautics/README.md) defines subsequent
work. [Source mappings](docs/reference/sources.md) and
[reference fixtures](calculator/testdata/README.md) have dedicated locations.
