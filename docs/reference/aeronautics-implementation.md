# Aeronautics implementation record

Updated 2026-09-07. This is the review record for implementation in `aeronautics/`.
The task plan defines acceptance targets; this file records actual checks and
decisions. All thirteen tasks are authorized: the original eleven on
2026-09-05, with 12 and 13 added afterwards. OpenVSP is the selected NASA tool.

| Task | State | Evidence / remaining work |
|---|---|---|
| 01 — Boundaries | Complete | Module checks and frontend checks run and pass; runbook recorded. One acceptance check is not satisfiable, see "GCS gates" below |
| 02 — Lift | Complete | Units, registry, traces, typed issues and the lift inversions implemented; fixtures match to the quoted precision |
| 03 — Geometry | Complete | Planform, MAC, dihedral planes, coordinates, parameters, Reynolds coverage and configuration contracts implemented; 69 tests pass, 185 including subtests; the book's worked example reproduces. A plane-blind limits check was found and fixed post-review, see below |
| 04 — Workflows | Complete | Design definition, commands with undo/redo, requirement intersection, controlling cases, conflicts with offered alternatives, request identity and curated patterns implemented; 120 tests pass, 258 including subtests. The Matching process chapter was read and Task 04 claims no equation from it, see below |
| 05 — HTTP | Complete | Transport-neutral `api` boundary, `httpapi` transport, discovery, evaluate/apply/preview, generated frontend contract types and a serving executable; 157 tests pass, 359 including subtests. The seams are checked from outside, see below |
| 06 — Worksheet | Complete | Span-first and both weight-first journeys drive the running Go service; versioned drafts save and reopen; 166 Go tests and 61 frontend tests pass. The status row was left stale when the work landed and is corrected here |
| 07 — Visuals | Complete | Mass properties, a one-driver sensitivity sweep, dimensioned views and the formula behind each dimension implemented in Go and surfaced in the worksheet; 237 Go tests pass, 466 including subtests, and 101 frontend tests. The Center of gravity chapter was read and its worked example reproduces; the Trade study chapter was read and Task 07 claims no equation from it, see below |
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

## Task 04 verification

Run on 2026-09-05 from `aeronautics/`: `go build ./...`, `go vet ./...`,
`go test -race -timeout=2m ./...` and `golangci-lint run` (0 issues) all pass,
with no lint relaxation added. The calculator package now holds 120 top-level
tests, 258 counting subtests, up from 69 and 185 at the end of Task 03. The
core's import closure is unchanged: `go list -deps yalb.aero/calculator` still
resolves to the module plus `math`, `sort`, `strconv` and `errors`. No frontend
file was touched, and no equation was added to the registry.

**Every acceptance value reproduces.** The nine checks' expected numbers were
recomputed independently at 50 significant digits from the closed forms, not read
back from the package: area 0.24 m², stall speeds 10.544501312841112 and
14.912176765080807 m/s, minimum areas 0.41694940476190476 and
0.83389880952380952 m², mass ceilings 1.1512188158035619 and 0.57560940790178093
kg, and — for "size at the stall limit, keep span" — aspect ratio
3.4536564474106856, root chord 0.34745783730158730 m and the conditional
alternative span 1.5816751969261668 m. The fixtures and their derivations are
recorded in `calculator/workflow_helpers_test.go`; the task quotes several to
fewer places, and the tolerances are stated from the quoted precision rather than
from display rounding.

**The Matching process chapter was read, and Task 04 claims nothing from it.**
Reading it on 2026-09-05 changed the record. Its four constraints are takeoff
distance, landing distance, OEI climb gradient and cruise speed; its axes are
`W/S` against **power** loading `W/P` for a piston engine; its design point is
picked by visual inspection at `W/S = 40 lb/ft²`, `W/P = 9.25 lb/hp`; and it has
**no independent stall-speed constraint** — stall appears only inside its
landing-distance relation. So the stall-only subset implemented here is not one
of the chapter's constraints, no Task 04 equation carries `SourceBook`, and the
registry gained no equations at all. What the chapter contributes is the shape of
the workflow: intersect rather than average, identify the binding constraint, and
let the builder choose the point. Details in
[sources.md](../../aeronautics/docs/reference/sources.md).

**Design decisions taken during implementation.**

- *One definition, edited only by commands.* `Design` is the authoritative
  parametric definition and is a plain value; every edit is a `Command`, and the
  `Command` interface has an unexported method so the supported edits stay a
  curated set rather than arbitrary caller code. That is the task's "curated
  relationships initially" requirement expressed in the type system.
- *Promotion is atomic and normally ambiguous.* A planform holds exactly two size
  drivers and all six pairs are supported solve paths, so promoting a third value
  always has two valid releases. `PromoteDriver` without a named release returns
  the offer rather than choosing; `SetDriver` on a derived value refuses and names
  the same swaps. Nothing infers precedence from edit order.
- *Priority is carried by both a case and a requirement.* A bound contributes to
  the intersected required bounds only when the case and the requirement are both
  required. Either one being preferred removes the contribution and keeps the
  assessment, which is what acceptance check 7's final clause asks for.
- *An unknown contribution makes a bound partial, not smaller.* Withdrawing a
  case's CLmax leaves `SizingBound.Partial` set with the remaining value, so a
  bound that is missing evidence never reads as a complete feasible interval.
- *Ties are reported, not broken.* `SizingBound.Controlling` is a list. Two cases
  that produce the same bound are both named rather than one arbitrarily winning.
- *A conflicting group is known, not minimal.* `Conflict` says so in its own
  `Detail`, because identifying a minimal infeasible set is a solver result and no
  solver runs here.
- *Alternatives are commands, unapplied.* Each `Alternative` carries the `Command`
  that would resolve the conflict, so accepting one is the builder's edit. The
  span alternative also states that the maximum-span requirement would then be
  unmet, rather than presenting a change that quietly breaks another bound.
- *Identity is a counter, and the session is single-goroutine.* `RequestID` is a
  session-scoped sequence. The core reaches neither a clock nor `sync/atomic`, and
  the Task 01 boundary work made admitting `sync/atomic` and `context` a decision
  rather than an accident; that decision is not to admit them. A transport serving
  several callers owns its own serialization, outside this package.
- *Every design change retires the current request.* `Do`, `Undo`, `Redo` and
  `Load` all clear the current identity, so no restored design can accept a result
  computed before it. This matters in the one case an input snapshot cannot catch:
  undoing back to the exact design a held result was computed from.
- *Staleness replaces statuses rather than annotating them.* `Evaluation.Stale`
  turns every check unknown, because showing a previously met requirement against
  edited inputs would present a stale output as a current one.
- *Power-first is listed, not omitted.* `PatternPowerFirst` is registered with
  `Supported: false` and never offered for a design, so its absence is a stated
  gap. No sizing formula was fabricated for it.
- *The area a sizing action writes stays in its own plane.* Under
  `DihedralHoldPanel` with a nonzero dihedral the area driver is a panel-plane
  construction area, so `SizeAtStallLimit` converts the plan-view bound through
  the existing `geometry.panel-area` relation instead of writing a projected
  number into a panel driver.

**Tests verified by mutation.** Twenty-three deliberate breaks were introduced
one at a time and the suite rerun for each: inverting the bound intersection,
letting a preferred case or a preferred requirement tighten the required bounds,
dropping a failed contribution instead of marking the bound partial, making an
unknown check outrank a known failure, treating an empty required set as passing,
widening the boundary tolerance to 1e-3, recording a margin without applying it,
holding both drivers through a sizing action, writing the projected area into a
panel-plane driver, committing a preview, leaving requirement statuses standing
under `Stale`, dropping the aspect ratio from the derived area ceiling and from
the span alternative, reporting a conflict when the bounds agree, accepting a
per-case requirement with no cases, ignoring the case scope, accepting a
promotion release that is not a driver, and omitting the case priority from the
snapshot. Every one failed a test. Two initially did not and were fixed by adding
the missing tests rather than by accepting the gap:

- Making `Session.invalidateRequest` a no-op still passed, because `Accept` also
  compares the input snapshot and every existing test changed the inputs.
  `TestUndoingBackToAnEvaluatedStateDoesNotRestoreItsIdentity` closes it: the
  inputs match again after the undo and the result is still refused.
- Accepting any named driver release still passed, because no test named an
  invalid one. `TestPromotionRefusesToReleaseSomethingThatIsNotADriver` closes it
  and checks the design is unchanged afterwards, which is the three-driver state
  the atomic swap exists to prevent.

## Task 05 verification

Run on 2026-09-05 from `aeronautics/`: `go build ./...`, `go vet ./...`,
`go test -race -timeout=2m ./...` and `golangci-lint run` (0 issues) all pass,
with no lint relaxation added. From `aeronautics/frontend/`: `pnpm typecheck`,
`pnpm lint`, `pnpm test` and `pnpm build` all pass. The module now holds 157
top-level tests, 359 counting subtests, across four packages: 122 in
`calculator`, 16 in `api`, 14 in `httpapi` and 5 in `boundary`.

The executable was started and exercised: `go run ./cmd/aero serve` answered
`GET /api/v1/discovery` with the contract, the equations and their provenance,
and answered `POST /api/v1/evaluate` for the Task 04 fixture with the solved
parameter set and its per-parameter equation revisions.

**The core gained two functions and no equations.** `calculator.Units` and
`calculator.ParseUnit` exist so that a transport can list and resolve unit
symbols without keeping a second copy of the conversion table, which is exactly
how a mistyped factor gets into a system twice. The registry is unchanged, the
core's import closure is unchanged, and no Task 05 code computes anything.

**The seams are checked from outside, not asserted in prose.**

- `TestApplicationBoundaryDoesNotDependOnHTTP` reads the actual import closures:
  `yalb.aero/api` may not reach `net/http`, `net`, `os`, `database/sql` or
  `httpapi`, and `yalb.aero/calculator` may not reach `api`, `httpapi`,
  `encoding/json` or `context`. That is what makes "an MCP sidecar can reuse the
  contract without loopback HTTP" a property rather than an intention.
- `TestTransportDoesNotReimplementTheCore` parses `api` and `httpapi` with
  go/ast and fails on any multiplication, division, subtraction or remainder.
  Those four operators are how a unit conversion or a formula gets copied into a
  transport, where it is then free to disagree with the core; string
  concatenation and comparison stay allowed because that is what building a
  message is made of. One real violation was found and removed: the TypeScript
  generator was indexing with `len(tokens)-1`.
- `TestCoreAndBoundaryAgreeOnTheSameCandidate` writes the fixture aircraft twice,
  once as a wire design and once through direct Go calls, and compares the input
  snapshots before comparing any number. Two definitions with the same
  fingerprint are the same definition, so that is the check that the wire form
  carried the aircraft across rather than something close to it.

**Design decisions taken during implementation.**

- *The service is stateless.* A request carries the whole design; history, undo
  and the request stream stay with the client that owns them. Server-side
  sessions would make two clients of one design fight over a single history and
  put a second authoritative copy of the definition where the builder cannot see
  it. Identity is therefore minted by the client and carried verbatim, and the
  response returns the input snapshot the service computed so a stale answer is
  detectable either way.
- *Wire tokens are not display strings.* The enum tables in `api/enums.go` are
  stable identifiers rather than the core's `String()` output, which is wording
  meant for a person. Reusing those would make renaming a label a breaking API
  change. `TestEveryCoreConstantHasAWireToken` walks each core enum and fails
  when the table falls behind, so a new constant cannot reach the boundary
  encoded as an empty string.
- *Commands are a flat tagged union.* One strict decode pass can then reject an
  unknown field, and `commandSpecs` names exactly which field does not belong to
  which kind. A client that sends `hold` to a mass edit is told so rather than
  having it dropped.
- *A design that does not solve is a 200.* Refusing it would make an unfinished
  worksheet unreadable. Only the *shape* of a request fails the call; the
  physics comes back as the core's typed issues alongside whatever bounds could
  still be established, which is why an area bound survives a wing that does not
  solve.
- *A negative span is not a bad request.* Lengths may be negative in general —
  that is what makes anhedral and forward sweep expressible — so the boundary
  passes it through and the core refuses it as an invalid driver.
  `TestNegativeSpanIsADomainFailureNotARequestFailure` pins where that line sits.
- *One failure shape everywhere.* `net/http`'s built-in 404 and 405 are plain
  text, so the router answers both itself in the boundary's JSON error shape,
  with an `Allow` header on a method mismatch. A client that has to parse two
  failure formats will eventually parse one of them wrong.
- *Limits live in `api`, not in the transport.* The case, requirement, command
  and batch counts bind every adapter equally, and they are published in the
  discovery document so a client can respect them instead of finding them by
  being refused. Only the byte cap is HTTP's, because only HTTP has a body.
- *The Go types are the contract authority.* `go run ./cmd/aero contract`
  regenerates the frontend's types from them by reflection, and
  `TestGeneratedTypeScriptMatchesTheCheckedInFile` fails on drift. The generated
  file says in its own header that types are not validation.

**Tests verified by mutation.** Sixteen deliberate breaks were introduced one at
a time and the suite rerun for each. In `api`: dropping a requirement subject's
token, rounding a value on the way out, ignoring an unknown command field,
half-applying an apply call, dropping the request identity, ignoring the batch
limit, ignoring cancellation, and reporting an unsupported input as an invalid
one. In `httpapi`: ignoring unknown JSON fields, removing the body limit, mapping
unsupported onto 400, falling back to net/http's plain-text errors, accepting
trailing JSON after the body, dropping the identity header, turning a not-found
equation into a success, and skipping the content-type check. Every one failed a
test.

## Task 06 verification

Run on 2026-09-06 from `aeronautics/`: `go build ./...`, `go vet ./...`,
`go test -race -timeout=3m ./...` and `golangci-lint run` (0 issues) all pass,
with no lint relaxation added. From `aeronautics/frontend/`: `pnpm typecheck`,
`pnpm lint`, `pnpm test` and `pnpm build` all pass. The Go module holds 166
top-level tests, 374 counting subtests: 129 in `calculator`, 18 in `api`, 14 in
`httpapi` and 5 in `boundary`. The frontend holds 61 tests across six files: 17
journey tests, 9 request-ordering tests, 10 reducer, 10 draft, 9 response
validation and 6 field-editing tests.

**The core gained commands and one published factor, and no equation.** Nothing
in the frontend computes: it added no equation, no constant and no conversion of
its own, and every number on the page came back from the service. What the
worksheet needed from Go was the vocabulary to express a builder's edits.

- *Withdrawing a value is not entering zero.* `SetDriver` and `SetMass` accept
  the zero `Quantity` to take a value back, and `SetDriver` refuses to withdraw
  where there is nothing to withdraw. This is what stands behind the brief's
  "empty input never becomes zero": clearing a field asserts nothing, where a
  typed zero would be refused as an impossible span.
- *Filling an empty slot is not a promotion.* Below two held drivers `SetDriver`
  now fills a slot, because nothing is being given up and there is no choice for
  the builder to make. Only at two does setting a third become a swap, which
  still releases another driver in the same edit.
- *A mass may arrive before its provenance.* `SetMass` no longer refuses an
  empty basis at the keystroke; `Design.Validate` reports it where the design is
  judged. The requirement is not relaxed, it is enforced somewhere that lets a
  builder type the number and the basis in either order.
- *Three edits bundle values that constrain each other.* `SetPlanformShape`
  carries shape and taper ratio, `SetWingAngles` every stated angle, and
  `SetConfiguration` the layout together with its tail description. Each pair
  would otherwise put the design through a state that is neither of the two —
  a rectangle with a taper, a wing with one angle unset and so unable to solve
  for a reason nobody chose, a flying wing still holding a tail.
- *`Unit.FactorToSI` is published in the discovery document.* A display layer can
  then render a stored SI value in the unit a builder chose without keeping a
  second copy of the conversion table, which is exactly how a mistyped factor
  gets into a system twice. Converting an *input* remains the service's job,
  which is why a request carries the unit symbol and not a converted number.
- *Array fields are never null.* `encodeIssues`, `encodeSolvedWing`, `encodeKeys`
  and `encodeConflicts` emit `[]` rather than a nil slice, because the generated
  contract declares those fields as arrays and a client that trusted the
  declared type would iterate over `null`. `TestArrayFieldsAreNeverNull` holds it.

These are covered by seven new `calculator` tests and two in `api`, including
`TestWithdrawingADriverIsNotEnteringZero`, `TestTheFirstDriversFillEmptySlots`,
`TestSettingTheAnglesDemandsAllOfThem` and
`TestTheWorksheetCommandsCrossTheBoundary`. The generated TypeScript was
regenerated for the new `Angles` type and `TestGeneratedTypeScriptMatchesTheCheckedInFile`
passes, so the checked-in contract is not drifting from the Go types.

**The journey tests run against the real Go service, not a transcript.**
`vitest.globalSetup.ts` starts `go run ./cmd/aero serve` on a free port and
provides its base URL to the suite. A recorded stub would drift from the core
the moment an equation changed, and a journey test asserting against a stub
would prove only that the stub was consistent with itself. The consequence is
that the frontend suite needs a Go toolchain; the request-ordering tests are the
exception and use a transport whose promises the test settles by hand, because
there the question is which answer is accepted rather than what is in it.

**The design is never edited in the browser.** Every edit becomes a curated
command, the boundary answers with the edited design and its evaluation
together, and the reducer decides whether that answer is still wanted. There is
no local code that could disagree with the core about what a driver swap means.
Unit selection is the one piece of arithmetic in the UI, and it runs on the
service's own published factor table: it changes what is shown and never what is
stored or evaluated.

**Freshness is decided by request identity, not by cancellation.** Every design
change retires the outstanding identity, undo, redo and draft loading included,
so a slow answer cannot land on a branch the builder has left — including after
an undo back to the exact design that answer was computed from. Three browser
tests delay a response across edit → undo → different edit, across redo, and
across draft loading; a fourth settles two evaluations out of order. Loading a
draft never restores its cached result as current: the snapshot records the
equation revisions that produced its numbers rather than the numbers, and
reopening it recalculates.

**Four defects were found and fixed while finishing the task.** The first two
were in the worksheet and reachable by an ordinary builder.

- *Blur re-sent an edit that was already in flight.* A commit is a request, and
  the draft text has to outlive it, because a refused edit must keep what was
  typed. Between pressing Enter and the answer arriving, the field therefore
  still held text differing from the committed value — so blurring it, by
  clicking Undo or any other control, sent the same edit a second time. The
  field now remembers the text it last sent and blur will not repeat it. Enter
  stays exempt: it is a deliberate act, so pressing it again after a refusal
  retries rather than doing nothing. `QuantityField` and `TextField` both.
- *Recalculate was disabled while a request was outstanding.* Nothing in the
  transport times a request out, so one call that never returned left the
  worksheet with no way to ask again. Correctness here belongs to the identity
  check — a late answer is discarded because it is not the outstanding one — not
  to preventing a second ask, so the button now stays live and the headline says
  "Calculating…" meanwhile.

The other two were in the request-ordering tests, which had been asserting less
than they appeared to.

- *Every branch answered with the same mass.* The fake boundary's design helper
  set a distinct `name` but a fixed mass of 2 kg, while the assertions read the
  mass field. Branch A, branch B and the answer that was supposed to be
  discarded were indistinguishable on the page, so the assertions would have
  held whether or not a late answer landed. Each branch now answers with its own
  mass and a discarded answer carries one no branch ever asked for.
- *A "half-typed" entry contradicted the specified behaviour.* One test typed
  into the span and moved to the mass field expecting the span to stay
  uncommitted, but blur commits, by design and by this task's brief. That test
  now sends the span through the boundary first and covers survival of a refusal
  with the refused entry itself, which is genuinely uncommitted: the design
  never took it.

**Tests verified by mutation.** Each fix was reverted one at a time and the suite
rerun. Removing the blur guard fails four tests, including the one named for it.
Extending the guard to Enter as well fails the retry test. Restoring
`disabled={api.pending}` on Recalculate fails the out-of-order test. All three
were caught.

**What Task 06 does not establish.** Drafts live in the browser's storage,
injected so a test can hand in an in-memory one; there is no export file, no
sharing and no server-side persistence, and clearing site data clears them.
Task 11 owns external handoff. `SCHEMA_VERSION` is 1 and `MIGRATIONS` is empty,
so an unknown or older schema is refused rather than migrated — the mechanism is
in place and the refusal is tested, but no migration path has been exercised
because none exists yet. The sections a builder can see are the wing sizing,
requirements and cases of Tasks 02–04: choosing a conventional tail, a V-tail or
a flying wing works and none of them reports a handling result, which is the
honest state rather than a gap in the page.

## Task 07 verification

Run on 2026-09-07 from `aeronautics/`: `go build ./...`, `go vet ./...`,
`go test -race -timeout=3m ./...` and `golangci-lint run` (0 issues) all pass,
with no lint relaxation added. From `aeronautics/frontend/`: `pnpm typecheck`,
`pnpm lint`, `pnpm test` and `pnpm build` all pass. The Go module holds 237
top-level tests, 466 counting subtests: 181 in `calculator`, 32 in `api`, 19 in
`httpapi` and 5 in `boundary`. The frontend holds 103 tests across eleven files,
of which 42 are new: 6 dimension-and-formula, 7 placement, 8 sensitivity,
5 request-ordering, 10 sweep-state and 6 reducer tests.

**Two chapters were read, and they produced opposite answers.** The
[Center of gravity chapter](https://computationaldesignlab.github.io/aircraft-design/weight_and_balance/cg.html)
contains the relation implemented from it, so the mass family is the second set
in this package to carry `SourceBook`; its worked example — eight components,
3114 lb, 44214.7 lb·ft, displayed as `x = 14.2 ft` and 8% MAC — reproduces in SI
and in its own units, and is recorded at 50 significant digits in
`calculator/testdata/cg-book-example.md`. The chapter writes the sum over
component *weights*; this package holds masses, and one uniform standard gravity
cancels between numerator and denominator, which is the single adaptation and is
recorded on the equation itself.

The [Aspect ratio trade study](https://computationaldesignlab.github.io/aircraft-design/trade_study/ar_study.html)
does not. It contours MTOW against wing loading and *power* loading over an 80×80
grid per aspect ratio, against takeoff, landing, climb-gradient, cruise-speed and
fuel-volume constraints. Every one of those needs a propulsion model, a drag
polar and a weight-estimation method this package does not have. Task 07
therefore implements no equation from it, and the one-driver sweep is documented
as a subset rather than presented as that plot. `TestBookProvenanceIsLimitedToCheckedChapterMethods`
now holds a chapter URL per equation, so an attribution cannot drift sideways
onto a chapter that does not contain the method either.

**A balance is only as complete as its inventory, and says so.** A component
with no mass, or with any coordinate of its position unstated, contributes
nothing: it is never read as weightless and never placed at the origin. The
result carries `Complete: false` and names which component is missing what, and
in the component mass mode an incomplete inventory establishes no all-up mass at
all, so every sizing result that rests on the mass waits for it rather than
being computed against a partial aircraft. Zero is spelled out exactly as the
wing angles are — a component on the centerline has `y = 0`, and an unstated `y`
is a missing field rather than a claim.

**Moving a mass and resizing one are different edits because they are different
physics.** `PlaceComponent` carries a whole position and nothing else, so a
completed drag changes the balance and leaves the all-up mass, the wing loading
and Task 02's stall speed exactly where they were. `SetComponent` carries the
item, so changing its mass moves all of them. The worksheet sends the first when
a component is already placed and the second while it is still being filled in,
which is what lets a builder type one coordinate at a time without the core
accepting a position with a field missing.

**The mass modes are kept apart.** An entered all-up mass is a figure a builder
holds; a component total is only as good as the inventory behind it. `MassMode`
selects which the design is judged at, the entered figure is never overwritten
by adopting the components, and switching back restores it. Components are
balanced in either mode, so a builder can see where the masses sit before
adopting their total.

**Four references are named apart, and three of them are unknown.** The
mechanical centre of gravity is implemented. The quarter-MAC marker is drawn and
labelled as a geometric reference on the reference planform; the panel says in
words that it is not a wing aerodynamic centre, not an aircraft neutral point,
not a centre of pressure and not a "centre of lift". Static margin and trim are
named as unknown until Task 08. The lumped required lift crosses the boundary as
a magnitude per case with no line of action, because the Task 02 model solves
none; `TestTheLumpedCaseLoadIsAMagnitudeOnly` checks that its equation still
states the lumped assumption.

**A plot without its driver mode is ambiguous, so the mode travels with it.**
Every sweep answer carries the solve mode, what was held fixed, and — when the
plotted output does not move — which derived parameters did. That is what
answers the acceptance check that a span sweep at a fixed area leaves the target
stall speed constant: the answer says so and then names the aspect ratio and the
chords as what changed, rather than leaving "the span does nothing" on the page.

**A sweep is a question, so an answer is matched against the question.** Three
things must agree before a curve is shown: the outstanding sweep identity, the
input snapshot, and the settings. The identity clause is Task 04's history rule
applied to sweeps — every design change retires it, undo and redo included — so
an obsolete answer stays obsolete even when the history walks back to the exact
design it was computed from. The settings clause is the sweep's own: a different
range or a different plotted output is a different question, and the worksheet
says so rather than relabelling an old curve. `Session.AcceptSweep` holds all
three in the core, and the reducer holds them again in the browser.

**Gaps are gaps.** A candidate the model cannot evaluate carries no value, no
line is drawn through it, and the table says why. A candidate that computes and
violates a requirement is a point outside the feasible region, which is a
different answer and is reported as one: `Status` and `Feasibility` never
collapse into each other.

**Sampling costs bounded work.** Between 2 and 65 candidates per request, each
one an ordinary design produced by the ordinary driver edit and evaluated
through the ordinary workflow, so a plotted sample *is* the candidate rather
than an approximation of it — `TestEveryPlottedSampleMatchesADirectEvaluation`
and its boundary twin check that against direct evaluation. The samples run
sequentially: each is a handful of algebraic steps, the count is bounded, and a
deterministic order is part of the answer rather than an implementation detail.
`RunUntil` takes a stop predicate rather than a context, because the core
reaches neither a context nor a clock, and the HTTP boundary passes its own
cancellation check in so a caller that goes away stops the remaining samples.

**A dimension and a field are one thing seen twice.** Every dimension the
service emits carries the core's own parameter key and that parameter's value,
so selecting a dimension selects the field and selecting a field highlights the
dimension, with no shared code between the two components and no second copy of
a value. `TestEveryDimensionNamesAParameterTheWingReports` holds it in the core
and `TestEveryDimensionNamesAParameterInTheSameResponse` at the boundary. A
driver swap moves the roles and the formulas without moving the keys, and the
drawing follows because it never held a copy.

**Explanations are the core's, not the browser's.** Each parameter's
relationship, revision, substituted values and dependencies come from the
service; the trace is matched to the parameter by equation *and* result value,
and where no trace matches exactly the explanation says the substitutions were
not recorded rather than showing another evaluation's. The copyable parameter
table is written with `String(value)`, the shortest text that reads back as the
same float64, so copying loses nothing that the rounded display costs.

**Projected and panel dimensions are told apart in words as well as in line
style.** The front view dimensions both half spans, marks which plane each is
in, and the accessible name says "panel as built" or "plan-view projection".
Nothing on any drawing can only be read by seeing a colour: each outcome has a
shape and is written out beside it, the plot's accessible name carries the axes
and what was held fixed, and the sample table is a complete alternative to the
plot.

**Three defects were found and fixed while finishing the task.**

- *A preview took an identity without moving the reducer's counter.* The hook
  mints request identities from its own counter and the reducer keeps a matching
  one, and the pairing is what makes the two agree. Adding the drag preview
  broke it: `previewCommand` minted an identity and dispatched nothing, so from
  the first drag onwards every apply and every evaluation answered an identity
  the reducer had never issued and was discarded. The worksheet showed
  "Calculating…" forever. `preview-started` exists for no reason except to move
  the counter, `sweep-started` now moves it too, and
  `TestEveryRequestKindKeepsTheCountersInStep` fails if a fourth kind is added
  without one.
- *A delayed preview could describe a position the builder had left.* The first
  implementation matched preview answers by a sequence number and gated on one
  request in flight, which meant a fast pointer left the readout showing an
  older position's numbers under newer coordinates. An answer is now applied
  only when the marker is still exactly where it was when the question was
  asked, and the newest position waits its turn behind the outstanding request.
- *A sweep could be revived by an undo.* The first `AcceptSweep` matched on the
  input snapshot and the settings alone, on the reasoning that the same inputs
  give the same curve. That contradicts the task's acceptance check, which
  requires an obsolete response to stay obsolete even when the design returns to
  a previously visited revision. The identity clause was added and the core test
  inverted; the reducer already behaved correctly, which is why the browser test
  passed while the core one asserted the opposite.

**Selecting a candidate and adopting one are different acts.** Choosing a
sample on the plot describes it and changes nothing; "use this as the aspect
ratio" is the ordinary driver edit, going through the same command, the same
history and the same identity check as a typed one, and undoing in one step. The
two coordinated placement views share a single pixels-per-metre scale, because a
plan and a side view of one aircraft drawn at different scales are two drawings
of two aircraft. And no band is shaded anywhere: an assumed lift coefficient is
an assumption whose consequences the samples show, and shading it would present
it as a statistical interval it is not.

**Two additions the task did not name but the checks needed.** `DimMassMoment`
with `kg*m` and `g*mm`, because a moment sum is what a centre of gravity is
formed of and a trace that could not carry one would be a total with no
inspectable inputs. And a body-width field in the wing panel, which the contract
and the geometry model already supported but nothing surfaced: it is what makes
the exposed area reportable, draws the body sides on the plan view, and gives
the sweep a range that straddles a genuine model limit rather than a contrived
one.

**What Task 07 does not establish.** No aerodynamic reference location, no
static margin, no trim, no control authority and no handling result: the panel
names each as unknown rather than omitting it. The sweep plots one driver
against one implemented output; wing-loading and power-loading plots wait for
Task 09's models, and no assumption range is drawn as a confidence interval
anywhere. The parameter table is labelled a generic geometry handoff, because
Task 11 owns the verified Fusion expressions, units and sketch workflow. The
component model carries mass and position only — no electrical budget, no
volume, no attachment — and Task 09 extends this same model rather than starting
a second one. A found-but-unfixed observation: because the service answers in SI,
an angle field shows radians once the design has been through the boundary once,
so a builder entering degrees must choose the unit; that is Task 06's field
behaviour, it is visible rather than silent, and changing it was left out of
this task's scope.

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
  dependencies are reused. Task 06 replaced the Task 01 shell and its
  static-rendering test with the worksheet: the journey tests now drive it
  against a real `go run ./cmd/aero serve`, so the frontend suite needs a Go
  toolchain.
- **External tools:** OpenVSP was confirmed by the user. No Fusion, OpenVSP or
  XFLR5/flow5 application verification has yet been performed.

## Known limitations

- Version drift: CI pins the golangci-lint action to v2.12.2; this checkout ran
  v2.2.2. Both accept the current sources.
- The lift model is lumped: no separate wing and tail trim loads. Handling,
  stability and control remain entirely unimplemented, and no configuration
  (conventional, V-tail or flying wing) has a supported handling assessment.
- The workflow engine's constraint set is stall-only. Requirement subjects cover
  stall speed, span, area, mass, aspect ratio and mass wing loading — the
  quantities the implemented models produce — and nothing else. The book's
  takeoff, landing, climb-gradient and cruise-speed constraints, and any handling
  or power requirement, are absent until the models behind them exist.
- No solver runs. Feedback loops are explicit builder revisions: `SizeAtStallLimit`
  is a closed-form bound, not an iteration, so there is no iterate that could be
  mislabelled converged and no residual or budget to report. A conflicting group
  is reported as known rather than minimal for the same reason.
- A `Session` is single-goroutine. Concurrency belongs to whatever serves it;
  the HTTP boundary avoids the question entirely by holding no session.
- The API is the Task 04 workflow and nothing more. The service is stateless and
  has no persistence, no authentication, no rate limiting and no CORS handling:
  the dev server proxies same-origin, and a deployment that needs any of those
  adds them outside these packages. Task 06's drafts are held in the browser's
  own storage and never reach the service, so they are per-browser and are lost
  with site data; an export format is Task 11's.
- The generated TypeScript is types only. It disappears at run time and does not
  validate an untrusted response; a client that parses one still has to check it.
- Component placements, mass items and mission cases are not expressible in a
  `Design` yet; they arrive with Tasks 07 and 09. Task 06 added draft versioning
  and load-time compatibility checks: a draft records its contract, schema and
  equation revisions, and an unreadable or unknown-schema one is refused with the
  open design left untouched. `MIGRATIONS` is still empty, so an older schema is
  refused rather than migrated and no migration path has been exercised.
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
