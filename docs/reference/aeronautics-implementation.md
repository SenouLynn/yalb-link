# Aeronautics implementation record

Updated 2026-09-05. This is the review record for implementation in `aeronautics/`.
The task plan defines acceptance targets; this file records actual checks and
decisions. All thirteen tasks are authorized: the original eleven on
2026-09-05, with 12 and 13 added afterwards. OpenVSP is the selected NASA tool.

| Task | State | Evidence / remaining work |
|---|---|---|
| 01 — Boundaries | Complete | Module checks and frontend checks run and pass; runbook recorded. One acceptance check is not satisfiable, see "GCS gates" below |
| 02 — Lift | Complete | Units, registry, traces, typed issues and the lift inversions implemented; fixtures match to the quoted precision |
| 03 — Geometry | Complete | Planform, MAC, dihedral planes, coordinates, parameters, Reynolds coverage and configuration contracts implemented; 69 tests pass, 185 including subtests; the book's worked example reproduces. A plane-blind limits check was found and fixed post-review, see below |
| 04 — Workflows | Pending | No workflow engine implemented |
| 05 — HTTP | Pending | No API implemented |
| 06 — Worksheet | Pending | Static shell only |
| 07 — Visuals | Pending | No mass placement or sensitivity view |
| 08 — Handling | Pending | Conventional minimum defined in task; no model implemented |
| 09 — Power | Pending | No electric/mission model implemented |
| 10 — MCP | Pending | No sidecar implemented |
| 11 — Handoff | Pending | OpenVSP selected; target application/version checks remain |
| 12 — Airfoil sections | Pending | Task defined; no section generation, coordinate ingest or lofting implemented |
| 13 — Spar fit and stiffness | Pending | Task defined; no material, spar, load or deflection model implemented |

## Task 01 verification

Run on 2026-09-05 from `aeronautics/`, with no root Make target involved:
`go build ./...`, `go vet ./...`, `go test -race -timeout=2m ./...` and
`golangci-lint run` all pass. From `aeronautics/frontend/`: `pnpm typecheck`,
`pnpm lint`, `pnpm test` and `pnpm build` all pass. Commands are recorded in the
[runbook](../runbooks/aeronautics.md), which the module README now links to.

Fixed during verification: `boundary/imports_test.go` failed `govet`
fieldalignment, so `golangci-lint run` did not pass as the task requires. The
struct's pointer field was moved first.

Isolation confirmed: `go list ./...` from the repository root resolves no
`yalb.aero` package, so root `go build`, `go vet` and `go test` genuinely skip
the nested module. Diffs stay within the permitted set: `BUILD.bazel`,
`.bazelignore` and `.dockerignore` carry only their exclusion lines, and
`go.mod`, `go.sum`, `internal/**`, `cmd/**`, `proto/**` and `frontend/**` are
untouched.

**GCS gates.** The acceptance check "the GCS gates still pass" cannot be met, and
not because of this work. At `HEAD` in a clean worktree with no aeronautics
changes present, `make lint-go` reports the same 88 issues and
`internal/recording` fails `TestDurationLimitDoesNotRequireAnotherEvent` on
three consecutive runs. Both failures are in `internal/**`, which this work does
not touch. `make build` passes. The gates were already red; the calculator does
not change them either way.

**Boundary-test weakness — found, measured and closed.** The import allowlist
expanded six named stdlib packages to their toolchain implementation
dependencies. Because `testing` and `fmt` were among them, their closures
admitted 71 packages in total, including `strings`, `bytes`, `io`, `reflect`,
`context`, `sync/atomic`, `flag` and `path/filepath`, into what the production
core was permitted to import. The explicit `net`/`os`/`database/sql`/`time` bans
still held, so no hard guarantee was ever broken, but anything else could have
entered the core silently.

The seed is now the four packages the core may actually reach — `math`,
`errors`, `strconv`, `sort` — which narrows the allowlist from 71 packages to
38 and leaves the core's real closure a clean subset. `testing` is a dependency
of the tests, not of the non-test closure this check reads; `fmt` reaches `os`,
which the same task bans, which is why the core formats with `strconv`.
`TestAllowlistDoesNotAdmitTestOnlyPackages` pins the narrowing and was verified
by mutation: re-seeding `testing` fails it on `strings`, `bytes`, `io`,
`reflect`, `context`, `sync` and `sync/atomic`.

This mattered ahead of Task 04 specifically: request identity and ordering are
exactly where `sync/atomic` and `context` get reached for, and admitting them
is now a decision rather than an accident.

## Task 02 verification

Implemented in `aeronautics/calculator`: a dimensioned `Quantity`/`Unit` layer,
typed `Issue`s, an equation registry with provenance, per-evaluation traces, and
the ten lift equations and inversions. `go list -deps yalb.aero/calculator`
resolves to the module plus `math`, `sort`, `strconv` and `errors`; the core does
not reach the clock, and it formats numbers with `strconv` because `fmt`'s
closure reaches `os`.

Fixture values were computed independently at 40 significant digits and match the
task's expected values exactly at the precision those are quoted to: weight
19.6133 N, loading 81.7220833 N/m² and 8.3333333 kg/m², stall speed 10.5445013
m/s, area bound 0.4169494048 m², mass ceiling 1.1512188158 kg. Tolerances are
stated as an absolute term in each quantity's SI unit plus a relative term, taken
from the quoted precision rather than from display rounding.

Also checked: forward/inverse round trips; mixed-unit equivalence (2000 g and
24 dm² give the same stall speed as 2 kg and 0.24 m²); composite units agreeing
with their bases, which is what would catch a mistyped `oz/ft²` factor;
quadrupling mass doubling stall speed and quadrupling area halving it; load
factor changing the result and appearing in the trace; missing versus supplied
zero versus negative versus dimensionally wrong input; overflow and underflow in
the result; and a sweep of adversarial inputs producing no panic and no
successful NaN.

**Source attribution corrected during implementation.** The Lift chapter was read
on 2026-09-05 rather than assumed. It contains the lift-curve slope and a CLmax
estimation method but **no stall-speed equation and no inversion of the lift
identity**, so the initial attribution of the stall-speed family to the book
overstated its provenance. Those equations now cite the standard lift identity
and their own derivation, and `TestLiftSubsetDoesNotClaimBookProvenance` keeps
the attribution from drifting back up. The chapter's real contribution is
applicability: it distinguishes section `c_lmax` from aircraft `C_Lmax`, which is
why a section value is refused rather than reinterpreted. Its worked example
checks out independently (1.88 × 0.9 − 0.25 = 1.442, displayed 1.44 — display
rounding, not a discrepancy). Details in
[sources.md](../../aeronautics/docs/reference/sources.md).

**Assumptions are enforced, not just documented.** A flight case must state its
density basis and its CLmax basis and scope; an unstated assumption is a missing
field, so a result cannot rest on an invisible default. No CLmax default exists.

**Tests verified by mutation.** Removing the registry's defensive clone makes
`TestMetadataInspectionCannotMutateTheRegistry` fail, so that test is real. The
same check on the trace's substitution slice did *not* fail when the copy was
removed, because each evaluation already allocates its own slice; the redundant
copy was removed rather than left as untested code implying a guarantee it was
not providing. The test remains as a regression guard if evaluations are ever
pooled.

**Lint relaxation.** `gocritic`'s `hugeParam` is disabled for this module, with
the reason recorded in `.golangci.yml`: `FlightCase`, `Source`, `Equation` and
`Trace` are passed by value so that evaluation cannot mutate caller data and
metadata inspection cannot mutate the registry, which is a Task 02 requirement.
No `wrapcheck` or `funlen` relaxation is in effect, and no other `gocritic` check
is disabled.

## Task 03 verification

Run on 2026-09-05 from `aeronautics/`: `go build ./...`, `go vet ./...`,
`go test -race -timeout=2m ./...` and `golangci-lint run` all pass, with no lint
relaxation added. The calculator package now holds 68 top-level tests, 184 counting subtests, up
from 107 at the end of Task 02. The import closure is unchanged:
`go list -deps yalb.aero/calculator` still resolves to the module plus `math`,
`sort`, `strconv` and `errors`. No frontend file was touched.

**The book's worked example reproduces.** The Wing Planform Sizing chapter was
read on 2026-09-05 — the reading is recorded in
[sources.md](../../aeronautics/docs/reference/sources.md) — and its A=8,
S=134 ft², lambda=0.4 example was recomputed independently at 45 significant
digits rather than read back from the chapter's display: span 32.74141108748980 ft
(chapter shows 33), root chord 5.846680551337464 (5.8), tip chord
2.338672220534986 (2.3), MAC 4.343248409564973 (4.3), y_MAC 7.016016661604957
(7.0), leading-edge sweep 3.066485501125893° (3.1). Every derived value is
computed from the unrounded span; the fixture record in
[calculator/testdata/wing-planform-book-example.md](../../aeronautics/calculator/testdata/wing-planform-book-example.md)
states each tolerance and notes that the span's whole-number display is rounding
rather than a discrepancy. Solving the same wing in SI agrees with the customary
solve to 1e-12 relative.

**A second, independent cross-check.** The chapter itself never reports an
exposed area. The book's Lift chapter quotes S_exposed = 106 ft² for the same
aircraft behind a 5 ft body, and integrating the linear chord distribution across
the body exactly gives 106.1058829575984 ft². That agreement is checked in the
tests, and it is a real check: replacing the average chord across the body with
the root chord — the obvious simplification — gives 104.77 ft² and fails.

**Tests verified by mutation.** Three were checked by breaking the code they
cover. Dividing instead of multiplying by cos(Gamma) in the panel-span projection
fails the dihedral and outline tests; removing the spanwise-station range check
fails the chord-distribution test; the exposed-area simplification above fails
the book cross-check. The tests were not accepted as passing without this.

**Book provenance is now claimed, and bounded.** Seven equations carry
`SourceBook`: the aspect-ratio definition, span from area and aspect ratio, root
and tip chord, MAC, y_MAC and the sweep transform.
`TestBookProvenanceIsLimitedToCheckedChapterMethods` holds that list and fails in
both directions, so the attribution can neither spread nor quietly disappear, and
`TestLiftSubsetDoesNotClaimBookProvenance` keeps the Task 02 family off it. Every
other geometry relation is recorded as derived from those, and the Reynolds
definition is supplementary: the NASA page gives Re = rho V L / mu with L only as
"some characteristic length", so choosing the local chord — and covering root,
MAC and tip rather than the MAC alone — is recorded as this project's adaptation.

**Design decisions taken during implementation.**

- *Drivers are exactly two.* A planform takes two of span, area, aspect ratio and
  root chord, plus the taper ratio for a trapezoid. Each pair is a named solve
  path, and all six paths are checked against each other on the same wing.
- *Over-determined is not one thing.* Extra drivers that agree are reported as
  redundant and unsupported; extra drivers that disagree are reported as invalid.
  A conflict is reported once against the whole driver set, naming the reference
  pair it was measured against, because with three mutually inconsistent values
  nothing in the numbers identifies which one the builder meant. Blaming a single
  field would have been an arbitrary choice presented as a finding.
- *Limits stay out of the solver.* `PlanformLimits` is a separate type that
  `SolvePlanform` never receives, so a maximum span cannot be consumed as a span.
  Its checks report met, unmet or unknown separately from numeric validity.
- *Two planes, and the builder chooses.* Drivers are given in one plane and the
  other is derived. A non-zero dihedral requires the mode; at zero dihedral the
  planes coincide, so the choice is not demanded. The aspect ratio an aerodynamic
  model would use follows the plan view and is a separate parameter from the
  planform's own.
- *Angles are stated, not defaulted.* Sweep, dihedral, twist and incidence must
  be supplied explicitly, with zero spelled out, because a later handling model
  cannot tell an unswept wing from an unrecorded one. Negative twist, negative
  incidence, anhedral, forward sweep and reverse taper are all accepted.
- *Unsupported is said, not implied.* A pointed tip (lambda = 0), an imported
  exposed-area basis, a dihedral beyond ±60°, a V-tail cant outside 15°–75° and a
  station beyond the semi-span are all reported as unsupported rather than as
  impossible or silently approximated.
- *A V-tail is panels.* `HorizontalTailArea` refuses to synthesise an equivalent
  horizontal tail for a V-tail and refuses to invent one for a flying wing.
  `Airframe.Handling` reports unsupported for every configuration, including the
  conventional one, so a complete geometry cannot read as a handling result.
- *Parameters carry their own provenance.* Each exported parameter states its
  unit, its datum where it is a coordinate, its driver or derived role, and for a
  derived value the equation revision it came from and what it depends on.
  `EvaluationOrder` fails on a cycle or a dangling edge, and the cycle detection
  is checked against a hand-built cyclic set rather than assumed.

## Post-review corrections (2026-09-05)

Findings from an adversarial review of the Task 01–03 commits, fixed in place.
All fixture values in this record were independently recomputed at 45
significant digits during that review and match.

**Plan-view limits were checked against panel dimensions.** `PlanformLimits`
documents `MaximumSpan` as a projected span — a doorway or a contest rule — and
`Check` compared it against `Planform.Span`. But a `Planform` is solved in the
plane its drivers were given in, and under `DihedralHoldPanel` those are
panel-plane construction lengths, larger than their projections by
`1/cos(Gamma)`. A wing with 20 degrees of dihedral and a 1.4 m panel span
projects 1.3156 m and fits a 1.35 m doorway; the check reported it unmet.
Reachable through the public API as `limits.Check(wing.Planform)`, and the one
existing limits test never used dihedral.

`Planform` now records its own plane in a `Plane` field, reusing the existing
`OutlinePlane` type rather than adding a second plane vocabulary. `SolveWing`
stamps `OutlinePanelSurface` only when panel dimensions are held under a nonzero
dihedral — at zero dihedral the planes coincide, following the precedent already
set in `resolvePlanes`. `Check` returns `unknown` with a `Detail` naming
`Wing.CheckLimits` rather than answering from the wrong plane, and
`Wing.CheckLimits` evaluates against `Wing.Projected`.
`TestPlanViewLimitsAreNotCheckedAgainstPanelDimensions` covers both, and was
verified by mutation in both directions: inverting the stamp and pointing
`CheckLimits` at the driver plane each fail it, the latter reproducing the
original wrong answer.

**Three doc comments disagreed about what a `Planform` is** — `geometry.go` said
every value was a plan-view projection, `wing.go` said the plane the drivers
were given in, and `PlanformDrivers.Span` said projected while
`WingDefinition.Drivers` said mode-dependent. They now agree, and the `Plane`
field makes the answer checkable rather than a matter of which comment is read.

Gates after these changes, from `aeronautics/`: `go build ./...`, `go vet ./...`,
`go test -race -timeout=2m ./...` and `golangci-lint run` (0 issues) all pass,
with no lint relaxation added. The calculator package holds 69 top-level tests,
185 counting subtests. The core's import closure is unchanged: the module plus
`math`, `sort`, `strconv` and `errors`. No frontend file was touched.

## Decisions and verification

- **Project boundary:** `aeronautics/`, module `yalb.aero`, Go 1.25.0, independent
  Vite/React package. These follow Task 01's proposed layout. Root build inputs
  change only for the three exclusions, plus its explicitly requested README link.
  A separate CI workflow owns calculator checks.
- **Import policy:** the allowlist expands the six named stdlib packages to their
  Go-toolchain implementation dependencies, then rejects transitive network,
  filesystem, SQL and clock imports. Literal stdlib paths alone would reject Go's
  internal implementation packages; expanding without bans would allow `fmt` to
  introduce `os`. The boundary test is outside the production core.
- **Strict checks:** Go lint is copied locally without the GCS generated-code
  exception. `wrapcheck` and `funlen` remain enabled without relaxation.
- **Frontend baseline:** pinned versions start from the existing checkout's
  dependency versions; no GCS components, configuration inheritance or runtime
  dependencies are reused. The shell test checks static rendering only. The
  frontend is still the Task 01 shell: it has no calculation backend and does not
  yet display any Task 02 result.
- **External tools:** OpenVSP was confirmed by the user. No Fusion, OpenVSP or
  XFLR5/flow5 application verification has yet been performed.

## Known limitations

- Version drift: CI pins the golangci-lint action to v2.12.2; this checkout ran
  v2.2.2. Both accept the current sources.
- The lift model is lumped: no separate wing and tail trim loads. Handling,
  stability and control remain entirely unimplemented, and no configuration
  (conventional, V-tail or flying wing) has a supported handling assessment.
- No method estimates a lift coefficient; `CLmax` is always supplied evidence.
  Task 03 supplies the geometry the book's CLmax estimation needs, but no airfoil
  polar evidence exists, so nothing estimates a coefficient yet.
- Geometry covers a rectangle and a symmetric trapezoid only. Kinked, cranked,
  elliptical and multi-panel planforms are unsupported, as are pointed tips.
- Sweep is a plan-view angle throughout. The panel's own in-plane sweep under
  dihedral is not reported; the panel outline gives the exact coordinates, so the
  value is recoverable, but no equation claims it.
- Airfoils are an identity and its evidence. No polar is imported, interpolated
  or extrapolated, and no section property is derived from a designation.
  Task 12 adds section *coordinates* — generated NACA shapes and ingested
  tables — which remains geometry: it will still derive no lift, drag or
  section clmax from a designation.
- No structural model exists. Nothing reports whether a spar fits, how stiff a
  wing is, or what load it carries. Task 13 adds fit and stiffness only, and
  strength, buckling, joints, fatigue and aeroelasticity stay unsupported there.
  A mass ceiling remains aerodynamic and is not a structural rating.
- Locating the MAC is not a balance result. The datum and the MAC leading-edge
  station are what a centre-of-gravity position would be measured against; no
  mass, balance or static-margin model exists.
- Component stations, which Task 03's brief lists as supported "where
  applicable", are deferred: no component model exists to place yet. They arrive
  with the mass-placement work in Tasks 07 and 09.
