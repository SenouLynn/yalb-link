# Adversarial Review: Tiers 0–3

Review of `tier-0-proto-contracts.md`, `tier-1-pure-domain-logic.md`, `tier-2-parity-apparatus.md`,
`tier-3-codegen.md`, read against the ADRs, `order-of-operations.md`, `port-plan.md`, and the
actual contents of this repo.

**Method:** claims were checked against sources, not recalled. gomavlib API claims are verified
against `github.com/bluenviron/gomavlib/v3@v3.3.5` in the module cache. MAVLink enum values and
field types are verified against that module's generated `common`/`minimal` dialects. Proto claims
are verified against `proto/gcs/v1/`. The Tier 2 CI gate was executed. `buf` is not installed on
this machine, so buf-lint findings are derived from documented rule categories and are marked
**verify** — they are the only findings not executed.

**Verdict:** the tiers are well-sequenced and the exit-gate discipline is right. The defects are
not structural, they are factual: several prescriptions encode API and enum values that do not
match this repo or the current libraries, and three of the gates cannot fail. As written, an agent
following Tiers 0–3 literally produces code that does not compile, does not lint, and enshrines
two numerically wrong safety behaviours in "known-answer" fixtures.

---

## Resolution status

Every finding below has been actioned. Where a fix was verifiable on this machine it
was executed, not asserted — `buf lint` and `buf breaking` were run against the real
protos, codegen was run end to end and the generated Go compiled, the `golangci-lint`
config was checked in both its broken and fixed forms, and the Tier 2 gate's shell
semantics were reproduced.

| Status | Findings |
|---|---|
| **Fixed and verified by execution** | B4, B5, B6, BE2, BE4, BE5, F1, F2, F6, P2, P5, P6 |
| **Fixed in contract or plan** (enforcement lands in its own tier) | B1, B2, B3, B7, B8, N1, N2, N4, N5, BE3, F3, F4, F5, P3, P4 |
| **Fixed and verified** | BE1 — `bazel build //...` passes over 5 targets including the generated proto library and Connect stubs. Required bumping rules_go 0.52.0 → 0.62.0 and gazelle 0.40.0 → 0.52.2 (0.52.0 passes `GOEXPERIMENT=coverageredesign`, removed in Go 1.25), pinning the Go SDK instead of `go_sdk.host()`, and setting `# gazelle:proto disable_global` so gazelle stops emitting a second, broken codegen path. The frontend is now under Bazel too — `aspect_rules_js` + `ts_project` + a vitest target, `bazel test //...` green across both languages. The forced-upgrade chain this exposed is recorded in **ADR-0006**. |
| **Stated now, enforced in a later tier** | P1 — the force-arm control is stated on `CommandLong` and in the resolved decisions; the SendCommand allowlist is implemented with the Tier 8 command registry. N3 — the threat model is written into Tier 4; `InKey` and the bind-address change land with the live transport in Tier 5. |

Decisions taken where the review left a choice open:

- **buf lint posture** — kept `STANDARD` with per-file `ignore_only` exceptions rather
  than renaming ~40 messages. Scoping per file rather than module-wide is what
  surfaced `TrackAffiliation`, a GCS-native enum genuinely missing its `_UNSPECIFIED`
  zero value.
- **Bazel** — wired rather than deferred; ADR-0001 is left intact and now actually holds. `bazel test //...` is green across Go and TypeScript and wired into CI. ADR-0006 records the version-coupling risk this surfaced.
- **Tier numbering** — numbers kept as stable identifiers, sequence stated explicitly
  (0 → 3 → 1 ∥ 2). Renaming four files and every cross-reference was judged worse
  churn than one explicit sequence line.

The findings below are preserved as written, including the original severity
assessments, so the reasoning stays auditable against what was changed.

---

## Recommended structural change: run codegen at Tier 0.5, not Tier 3

Tier 1's TS shim needs the generated `TelemetryEvent` types. The plan's answer is a hand-written
`_proto_stubs.ts` kept manually in sync for two tiers, then deleted. That is a drift generator with
no upside — `buf generate` is an hour of work with no dependency on Tiers 1 or 2.

Reorder to **0 → 3 → 1 ∥ 2**. Tier 1 then starts with real types, the stub file never exists, and
Tier 3's "verify the generated field names match what the shim expected" step disappears because
there is nothing to reconcile. Nothing in Tier 3 depends on Tier 1 or 2; the current order buys
nothing and costs a synchronisation obligation.

---

## Findings

| ID | Tier | Severity | Finding |
|---|---|---|---|
| B1 | 1, 2 | blocker | `isFixedWing` MAV_TYPE values are wrong; contradict this repo's `types.proto` |
| B2 | 1, 2 | blocker | `TelemetrySample` units contradict the protos — every position/altitude/climb value off by 1e7/1000/100 |
| B3 | 0, 1 | blocker | 5 of the "18 receive families" have no `TelemetryEvent` variant and should not have one |
| B4 | 0 | blocker | `buf lint` will not pass under `STANDARD`; Tier 0 treats it as a formality and prescribes an invalid category name |
| B5 | 0, 3 | blocker | `buf breaking --against '.git#branch=main'` is wrong twice; the gate passes vacuously |
| B6 | 2 | blocker | The capability-matrix CI gate exits 0 on failure. Verified by execution |
| B7 | 1 | blocker | Four of the prescribed gomavlib symbols do not exist or are deprecated |
| B8 | 1, 5 | blocker | gomavlib already sends the GCS heartbeat; the plan adds a second one |
| N1 | 5 | high | "1 Hz per discovered vehicle" is the wrong heartbeat cardinality for a shared link |
| N2 | 1, 4 | high | Outbound targeting is undefined; the only API satisfying the Tier 1 port broadcasts to every link |
| N3 | 4, 5 | high | Bare `0.0.0.0:14550` with no source validation; unauthenticated is the default posture |
| N4 | 5, 8 | medium | ArduPilot stream rates: works in SITL, may deliver nothing on real hardware until Tier 8a |
| N5 | 4 | medium | Two independent liveness clocks (gomavlib `IdleTimeout`, fold TTL) that will disagree |
| BE1 | 0–3 | high | ADR-0001 says Bazel is the trusted signal; Tiers 0–3 build a parallel Make/go/pnpm toolchain and never touch it |
| BE2 | 0 | high | `go_package` matches neither the Go module nor the Tier 3 output path; five different project names in-tree |
| BE3 | 1 | high | The prescribed 18-case `Decode` switch fails this repo's own `funlen`/`cyclop`/`gocognit` limits |
| BE4 | 1 | medium | `.golangci.yml` is a v1/v2 hybrid; none of the settings currently apply |
| BE5 | 0 | medium | Vehicle identity is duplicated in the envelope and in every payload; Tier 0 proposes a third copy |
| F1 | 0, 8 | high | `UploadMission` is client-streaming, which connect-web cannot call from a browser |
| F2 | 0 | medium | `payload_types` is `repeated string` — an untyped contract in a typed-contract repo |
| F3 | 1, 2 | high | The 65535 sentinel check is on the wrong message; the real normalisation gap is unhandled |
| F4 | 1 | high | `resolvePosition`'s fallback silently changes altitude datum from AGL to MSL |
| F5 | 1 | medium | Stall gate keyed on vehicle type zeroes the predicted track for a hovering VTOL |
| F6 | 1, 2 | low | vitest, the proto packages, and the `@/` alias are all assumed and none exist |
| P1 | 0 | high | Tier 0's safety claim does not hold — force-arm is reachable via `SendCommand` |
| P2 | 0 | medium | Authorization has no place in the contract; Tier 0 is the last cheap moment to add one |
| P3 | 2 | high | The evidentiary layer depends on a repo that is not present and not pinned |
| P4 | all | high | Three documents disagree about closed decisions; one of them is indexed as canonical |
| P5 | all | high | No exit gate is executable, and there is no CI at all |
| P6 | — | low | A 7.9 MB compiled binary is committed at the repo root |

---

## Blockers

### B1 — `isFixedWing` MAV_TYPE values are wrong

**Claim.** Tier 1 enumerates the fixed-wing set as KITE(9), FLAPPING_WING(10), VTOL_DUOROTOR(19),
VTOL_QUADROTOR(20), VTOL_TILTROTOR(21), VTOL_TAILSITTER_DUOROTOR(22), "VTOL reserved types 23–25",
and lists COAXIAL(8) among the values that must return false.

**Evidence.** From `gomavlib/v3@v3.3.5/pkg/dialects/minimal/enum_mav_type.go`, matching this repo's
own `proto/gcs/v1/types.proto`:

| Value | Actual | Roadmap says |
|---|---|---|
| 8 | FREE_BALLOON | COAXIAL |
| 9 | ROCKET | KITE |
| 10 | GROUND_ROVER | FLAPPING_WING |
| 16 | FLAPPING_WING | — |
| 17 | KITE | — |
| 3 | COAXIAL | — |
| 22 | VTOL_FIXEDROTOR | VTOL_TAILSITTER_DUOROTOR |
| 23, 24 | VTOL_TAILSITTER, VTOL_TILTWING | "reserved" |

The roadmap's table is pre-2019 MAVLink, before the VTOL renames.

**Consequence.** A ground rover (10) and a rocket (9) get a 14 m/s stall gate and therefore never
show a predicted track — a rover never exceeds it. Real KITE, FLAPPING_WING and COAXIAL are never
exercised. Tier 2 mandates testing "each MAV_TYPE in the fixed-wing list" individually, so the
test suite ratifies the error rather than catching it.

**Fix.** Do not retype the enum. Import the generated `MavType` constants and write the predicate
as a `switch` over named values, so a proto edit produces a compile error rather than silent drift.
Separately, `types.proto` is missing values 31–46 and VTOL_GYRODYNE(47); if the track layer is
going to classify ADS-B and mesh entities, close that gap in Tier 0.

### B2 — `TelemetrySample` units contradict the protos

**Claim.** Tier 1 Chapter 4 defines `TelemetrySample` with `latDegE7`, `altMslMm`, `relativeAltMm`,
`vxCms`, `vzCms`. Chapter 5 then divides by 1e7, 1000 and 100, and flips climb as
`-(sample.vzCms / 100)`.

**Evidence.** This repo's protos normalise at the proto boundary. `telemetry.proto`:
`GlobalPosition.lat_deg` is a `double` in degrees, `alt_msl_m` / `alt_relative_m` are `float`
metres, `vx_m_s` / `vz_m_s` are `float` m/s. `GpsRaw.lat_deg` likewise, with the comment
"normalised from degE7".

**Consequence.** The shim reads degrees and hands them to a resolver that divides by 1e7. Position
lands 1e7 too small, altitude 1000× too small, climb 100× too small. Every value is finite, so the
Tier 1 exit gate ("NaN never returned") passes. Tier 2 then writes fixtures from the same document
and locks the wrong numbers in as known answers — which is the one failure mode a parity apparatus
exists to prevent.

**Root cause.** `TelemetrySample` was lifted from flight-path-hud's raw-wire shape. That repo had
no normalising proto boundary; this one does.

**Fix.** State the invariant once, in Tier 1: *normalisation happens exactly once, in the Go codec,
at the proto boundary; every consumer downstream sees SI units and degrees.* Rewrite the
`TelemetrySample` field names accordingly (`latDeg`, `altMslM`, `vzMs`), delete every division from
the resolvers, and make the climb flip `-vzMs`. Add one fixture whose expected values are
physically plausible (`latDeg: 47.6` — a reviewer spots `476000000` instantly).

### B3 — Five "receive families" have no `TelemetryEvent` variant

**Claim.** Tier 0 Chapter 1 says to verify "`TelemetryEvent` oneof covers all 18 receive families."
Tier 1 Chapter 2 maps all 18 to oneof fields including `param_value`, `mission_count`,
`mission_ack`, `mission_item_int`, `command_ack`.

**Evidence.** `telemetry.proto`'s oneof has 15 variants. It does not contain any of those five.
It *does* contain three not on the 18-family list: `pid_tuning`, `named_value_float`,
`named_value_int`. And MISSION_COUNT has no proto message anywhere — `missions.proto` defines only
`MissionItem` and `MissionAck`.

**Consequence.** The check in Tier 0 can never pass, and the mapping table in Tier 1 cannot be
implemented. More importantly the proto is right and the roadmap is wrong: those five are
transaction responses, not telemetry, and they are precisely the messages Tiers 7 and 8 must
correlate against an in-flight registry. `Decode(evt) (*TelemetryEvent, error)` has no way to
express them. Discovering that at Tier 7 means re-cutting the codec's public signature after three
tiers of tests have been written against it.

**Fix.** Decide the codec's output type in Tier 0, while it costs nothing:

```go
type Decoded struct {
    SysID, CompID, Seq uint8
    Telemetry   *gcsv1.TelemetryEvent  // 13 streaming families
    Transaction *gcsv1.ProtocolEvent   // PARAM_VALUE, MISSION_*, COMMAND_ACK
}
```

Then correct Tier 0's check to "13 streaming families in the oneof; 5 transaction families route to
the protocol-event path", and add the missing `MissionCount` message.

### B4 — `buf lint` will not pass under `STANDARD`

**Claim.** Tier 0 frames itself as "a surgical pre-flight" whose only substantive change is one
field removal, with lint as a confirmation step. It instructs setting `lint.use: DEFAULT`.

**Evidence (verify — buf is not installed here).** `proto/buf.yaml` is `version: v2` with
`use: [STANDARD]`. Under `version: v2`, `DEFAULT` is not a valid category — it was renamed to
`STANDARD` in the v2 config. Following Tier 0 literally breaks the config file.

`STANDARD` includes rules the current protos violate broadly:

- `ENUM_ZERO_VALUE_SUFFIX` — zero values must end `_UNSPECIFIED`. `MAV_TYPE_GENERIC = 0`,
  `MAV_AUTOPILOT_GENERIC = 0`, `MAV_STATE_UNINIT = 0`, `MAV_RESULT_ACCEPTED = 0` and every other
  MAVLink mirror enum violates this.
- `RPC_RESPONSE_STANDARD_NAME` — `WatchFleet` returns `FleetEvent`, `SendCommand` returns
  `CommandResult`, `ListParameters` returns `ParameterValue`.
- `RPC_REQUEST_STANDARD_NAME` — `SendCommand(CommandLong)`, `UploadMission(MissionItem)`,
  `ExchangeSdp(SdpOffer)`.
- `RPC_REQUEST_RESPONSE_UNIQUE` — `WatchFleetRequest` serves two RPCs, `CommandResult` three,
  `MissionAck` three.

**Consequence.** This is not a lint run, it is a naming decision worth ~40 message renames, and
those names land in generated Go and TS. It has to be made before codegen, which is exactly what
Tier 0 is for — but Tier 0 doesn't know the decision exists.

**Fix.** Make the call explicitly in Tier 0. The honest option, given the repo already excepts
`FIELD_LOWER_SNAKE_CASE` on the grounds that MAVLink names are canonical, is to keep the MAVLink
mirroring and except the four rules with that same rationale recorded inline. Note that Tier 0's
instruction "do not silence lint rules — fix the underlying issue" already contradicts the
committed `buf.yaml`; resolve that contradiction rather than leaving an agent to guess.

Also correct Tier 0 Chapter 3: `google.protobuf.*` well-known types are built into buf and need no
`deps` entry. `buf.build/googleapis/googleapis` is for `google.api.*` / `google.rpc.*`. Adding it
buys nothing and adds a BSR dependency and a `buf.lock` that work against the air-gap story Tier 3
is otherwise careful about.

### B5 — The breaking-change gate passes vacuously

**Claim.** Tier 0 Chapter 5 and Tier 3 Chapter 4/5 both prescribe
`cd proto && buf breaking --against '.git#branch=main'`. `order-of-operations.md` prescribes
`buf breaking --against origin/main`.

**Evidence and consequence.** Two independent faults:

1. The buf module lives in `proto/`. A `.git#...` input is resolved from the repository root, so
   buf looks for a module at the root of that ref and does not find one. The invocation needs
   `subdir=proto`.
2. In GitHub Actions there is no local `main` branch even at `fetch-depth: 0` — the checkout is a
   detached HEAD with remote-tracking refs. `branch=main` does not resolve; `branch=origin/main`
   does.

Tier 3 correctly identifies the shallow-clone trap and then prescribes the form that still fails.
The failure mode is the one Tier 3 names: a gate that reports success on every PR.

**Fix.** One spelling, in one place, verified by running it:
`buf breaking --against '.git#branch=origin/main,subdir=proto'`. Delete the other two spellings.

### B6 — The capability-matrix gate cannot fail

**Claim.** Tier 2 Chapter 2 adds a CI target to hard-fail on `unstarted` cells:

```makefile
check-matrix:
	@grep -q 'unstarted' docs/wip/codec-capability-matrix.md && \
	  (echo "ERROR: unstarted cells in capability matrix" && exit 1) || true
```

**Evidence.** Executed with a file containing `unstarted`:

```
ERROR: unstarted cells
MAKE EXIT: 0
```

The trailing `|| true` swallows the subshell's `exit 1`. Separately, the matrix's own legend line
(`Status values: unstarted | pending | ...`) matches the grep, so the check fires unconditionally —
it is simultaneously always-triggered and never-failing.

**Fix.** `@! grep -q '| unstarted |' docs/wip/codec-capability-matrix.md`, anchored to table cells.
Better, see BE3: derive the matrix from the codec's dispatch table and assert equality in a Go test,
so the evidence is a test rather than a grep over prose.

### B7 — gomavlib symbols that do not exist or are deprecated

Verified against `gomavlib/v3@v3.3.5`:

| Tier 1 says | Actual |
|---|---|
| `gomavlib.EndpointCustomConn` | Does not exist. It is `EndpointCustom{ReadWriteCloser: io.ReadWriteCloser}` — and that type is itself marked `Deprecated: replaced by EndpointCustomClient` |
| `gomavlib.NodeConf{...}` / `NewNode` | Both marked `Deprecated: configuration has been moved inside Node` / `replaced by Node.Initialize()` |
| `e.Frame.GetSequence()` | `frame.Frame` exposes `GetSequenceNumber()` |
| `WriteMessage(msg interface{}) error` | No such method. The API is `WriteMessageTo(*Channel, message.Message)`, `WriteMessageAll`, `WriteMessageExcept` |

`e.SystemID()` / `e.ComponentID()` / `e.Message()` on `*EventFrame` are correct.

**Consequence.** The Chapter 1 snippet does not compile as written, and the deprecated-API path
trips `staticcheck` SA1019, which `.golangci.yml` enables — so the prescribed code fails this
repo's own lint gate. Tier 1 does carry a hedge ("verify these API names against the actual
gomavlib v3 tagged release"), but it hedges on the three names it got right and asserts the four it
got wrong.

**Fix.** Replace the snippet with verified symbols. `WriteMessage(msg interface{})` should become
`WriteTo(link LinkID, msg message.Message)` — see N2, this is not just a naming fix.

### B8 — gomavlib already sends the GCS heartbeat

**Claim.** A resolved decision specifies a hand-rolled heartbeat: "dedicated goroutine in vehicle
model … ticks at 1 Hz per discovered vehicle … sync.Once guard", with fields type=MAV_TYPE_GCS(6),
autopilot=GENERIC(0), sysId=255, compId=190.

**Evidence.** `node.go`: `HeartbeatDisable` defaults to false, so heartbeats are on unless disabled.
Defaults are `HeartbeatPeriod = 5 * time.Second`, `HeartbeatSystemType = 6 // MAV_TYPE_GCS`,
`HeartbeatAutopilotType = 0 // MAV_AUTOPILOT_GENERIC`. `Node.Initialize()` starts
`go n.nodeHeartbeat.run()`, `go n.nodeStreamRequest.run()`, and `go n.run()`.

**Consequence.** Two heartbeat sources at different rates from the same node. Tier 1's NodeConf
snippet does not set `HeartbeatDisable`. And the Tier 1 exit gate — "`go test -race
./internal/codec/...` passes — no sockets, no goroutines" — is false by construction: instantiating
a node starts three goroutines and writes unsolicited HEARTBEATs into the test's `io.Pipe`, so a
"pure decode" test has traffic in it and leaked nodes will flake under `-race -count=N`.

**Fix.** Set `HeartbeatDisable: true` explicitly and pick one owner. The library's version is
already correct on type, autopilot and cardinality; the simplest resolution is to delete the
hand-rolled decision and configure `HeartbeatPeriod: time.Second`. Whichever you choose, restate
the Tier 1 gate honestly: "no UDP socket is opened; every node created in a test is closed" — and
add `goleak` so that is checked rather than asserted.

---

## Network engineering

### N1 — Wrong heartbeat cardinality

HEARTBEAT is a node-level broadcast on a link, not a per-peer message. "1 Hz per discovered
vehicle" over the single shared `0.0.0.0:14550` socket means a 20-vehicle fleet emits 20 identical
frames per second from (255,190). On a 57.6 kbps SiK link that is roughly 400 B/s of pure
duplication in the direction that is already scarce, and it inflates `RADIO_STATUS.txbuf` — the
exact back-pressure signal `fleet.proto` documents as "the most actionable field". Correct
cardinality is one heartbeat per link per second, which is what gomavlib already does per channel.

### N2 — Outbound targeting is undefined, and the default is broadcast

The Tier 1 port is `WriteMessage(msg) error` — no destination. The only gomavlib call that
satisfies that signature is `WriteMessageAll`, which writes to every channel: every SITL instance,
every radio link. Three consequences: uplink bandwidth multiplied by fleet size on the constrained
direction; a COMMAND_LONG addressed to sysid 2 is physically transmitted over vehicle 1's radio;
and two vehicles sharing a sysid on different links both act on it. The Tier 4 route table covers
inbound dispatch only — outbound targeting is specified nowhere in Tiers 0–8.

This is a Tier 1 interface decision, which is why it belongs in this review. Make the port
`WriteTo(link LinkID, msg message.Message)`, populate sysid→channel from inbound frames, and decide
the unknown-target behaviour explicitly. Recommend rejecting rather than broadcasting: a command
that cannot be addressed should fail loudly.

### N3 — Unauthenticated socket is the default posture

The bridge binds `0.0.0.0:14550` and accepts frames from any source that can reach it. A spoofed
HEARTBEAT creates a phantom vehicle in `fleet:active`; spoofed STATUSTEXT or EKF_STATUS_REPORT
drives operator decisions. ADR-0005 scopes MAVLink signing to "radio links" and treats the local
socket as trusted, which holds inside Docker Compose and stops holding the first time this runs on
a field box on shared WiFi.

The resolved decision — signing key from `GCS_MAVLINK_SIGNING_KEY`, empty means disabled — is a
reasonable dev default, but silence is the wrong way to express it. Cheap Tier 4/5 additions:
gomavlib's `InKey` (it drops unsigned v2 frames outright), a source-address allowlist or a bind
address that is not `0.0.0.0` by default, and a startup log line that states the posture out loud
when signing is off. Write the threat model down in Tier 4 — one paragraph, what is trusted and
why — rather than leaving it implicit.

### N4 — ArduPilot stream rates

gomavlib's `StreamRequestEnable` (default false) is what makes ArduPilot emit telemetry on links
that require an explicit request. SITL over UDP streams by default, so Tier 5 and Tier 6 will look
healthy; a real vehicle may deliver almost nothing until SET_MESSAGE_INTERVAL is sent, which the
roadmap schedules at Tier 8a — three tiers after live telemetry is declared proven. Decide now and
put the decision in Tier 5's gate, so first hardware bring-up is not a surprise.

### N5 — Two liveness clocks

gomavlib closes idle channels after `IdleTimeout` (default 60s). The Tier 4 fold has its own
heartbeat TTL driving `VEHICLE_LOST`. They will disagree, and the disagreement window is where
"vehicle lost but link still open" bugs live. Name one as authoritative in Tier 4 and derive the
other from it.

---

## Backend

### BE1 — ADR-0001 goes dormant across all four tiers

ADR-0001 makes Bazel the single build system and `bazel test //...` the trusted hermetic signal,
justified by agentic maintenance and determinism at checkpoints. Tiers 0–3 build an entirely
separate toolchain — `make test-go`, `go test -race`, `pnpm vitest`, `buf generate` — and never
mention Bazel. The repo state matches: `MODULE.bazel` has `rules_go` and `gazelle` only. There is
no `go_deps` extension, so gomavlib cannot resolve under Bazel; no `rules_js`/`rules_ts`, so the
frontend cannot build; no BUILD files under `internal/` or `frontend/`.

By the end of Tier 3 the ADR-0001 signal is dead and every gate is a Makefile target. "ADRs are law"
plus a plan that quietly ignores one is the worst of both worlds — an agent reading the ADR and an
agent reading the tier docs will build different things.

Pick one. Either amend ADR-0001 (Make is the developer surface; Bazel is adopted at Tier N, here is
the trigger) or schedule the minimum inside Tiers 2–3: `go_deps.from_file(go_mod = "//:go.mod")`,
a gazelle target, generated BUILD files, and `bazel test //internal/codec:codec_test` in the Tier 1
gate. The second is maybe half a day and keeps the ADR honest.

### BE2 — `go_package` matches nothing, and the repo has five names

Every proto declares `option go_package = "ligma-gcs/proto/gcs/v1;gcsv1"`. The Go module is
`ligma.gcs`. Tier 3 generates into `internal/gen` with `paths=source_relative`. So the files land
in the right place while the import path baked into the generated code — and any Bazel `importpath`
— points at a module that does not exist.

Fix it in Tier 0, specifically **before** the breaking baseline: `buf.yaml` sets
`breaking.use: [FILE]`, the strictest category, which flags `go_package` changes. Change it after
the baseline and the gate fires forever.

While there: this repo calls itself `ligma.gcs` (go.mod), `ligma_gcs` (MODULE.bazel), `ligma-gcs`
(directory, go_package), `yalb-gcs` (buf module, docs) and `yalb-gcs-frontend` (package.json). Five
names for one project. For a repo whose stated goal is legibility to humans and agents, this is the
cheapest cleanup available and it has to happen before codegen anyway.

### BE3 — The prescribed `Decode` fails this repo's lint config

An 18-case type switch with a `TelemetryEvent` construction per case runs 90–120 lines and carries
cyclomatic complexity near 20. `.golangci.yml` sets `funlen` to 80 lines / 50 statements,
`cyclop.max-complexity: 15`, `gocognit.min-complexity: 20`. The plan's central function fails all
three.

The fix is better than the original anyway: a dispatch table.

```go
var decoders = map[uint32]func(message.Message) *gcsv1.TelemetryEvent{
    30: decodeAttitude,   // ATTITUDE
    33: decodeGlobalPos,  // GLOBAL_POSITION_INT
    ...
}
```

One small converter per family, each independently testable, each under every limit. And the Tier 2
capability matrix becomes derivable: a Go test asserting `keys(decoders) == matrix rows` turns the
matrix from hand-maintained prose into something CI genuinely enforces — which is what B6 was
reaching for.

### BE4 — `.golangci.yml` is a v1/v2 hybrid

The file declares `version: "2"` but uses the v1 top-level keys `linters-settings:` and `issues:`.
In v2 that block moved under `linters.settings:`. As written, the funlen limits, revive rule list
and wrapcheck ignores in BE3 either do not apply or the config is rejected outright. Nothing has
caught it because no CI runs it. Fix before Tier 1 writes the first Go file — otherwise the first
real lint run lands hundreds of issues at the worst possible moment.

### BE5 — Identity is stored twice, and Tier 0 proposes a third

`TelemetryEvent` carries `vehicle_id`, and every payload message carries its own `vehicle_id` /
`id`. Two sources of truth per event, a wire cost on every message at telemetry rates, and no
stated rule for what happens when they disagree.

Tier 1's architecture note has the right instinct — "sysId/compId travel with the event … they are
NOT part of `TelemetrySample`" — but the proto contradicts it, and Tier 0 Chapter 1 asks you to
*add* top-level `system_id`/`component_id` to `TelemetryEvent`, which would make three copies.

Tier 0 is the only cheap moment: drop `vehicle_id` from the payload messages, keep it on the
envelope, and delete the Tier 0 instruction to add the scalar pair.

---

## Frontend

### F1 — `UploadMission` cannot be called from a browser

`services.proto` declares `rpc UploadMission(stream MissionItem) returns (MissionAck)` — client
streaming. `@connectrpc/connect-web` supports unary and server streaming only; client and bidi
streaming are not available to browser clients. The single RPC that Tier 8c and port-plan Phase 7b
depend on is uncallable from the frontend.

Fix in Tier 0: `UploadMissionRequest { VehicleId target = 1; repeated MissionItem items = 2; }`,
unary. Missions are hundreds of items at most; streaming buys nothing here. Free to change now,
expensive at Tier 8 (proto break, regen, service rewrite). Worth adding a Tier 0 audit step:
*no RPC uses client or bidi streaming* — it is one grep and it is exactly the class of thing a
pre-codegen contract review should catch.

### F2 — `payload_types` is a stringly-typed filter

`StreamTelemetryRequest.payload_types` is `repeated string` with example values in a comment. In a
repo whose premise is typed contracts, this one field has no lint coverage, no breaking-change
coverage, no autocomplete, and a typo subscribes the client to nothing without an error. Make it a
`repeated TelemetryPayloadType` enum. Tier 0, before codegen.

### F3 — The 65535 check is on the wrong message

Tier 1's heading chain rejects `VFR_HUD.heading == 65535`. `VFR_HUD.heading` is `int16_t` — verified
in `gomavlib .../common/message_vfr_hud.go`: `Heading int16`, "Current heading in compass units
(0-360, 0=north)". It cannot hold 65535. The UINT16_MAX-as-unknown sentinel belongs to
`GLOBAL_POSITION_INT.hdg` (uint16 centidegrees) and `GPS_RAW_INT.cog`, and this repo's own protos
say so: `hdg_cdeg ... UINT16_MAX = unknown` versus `VfrHud.heading_deg` as `int32` documented 0–359.

So the rule rejects a value that never occurs and accepts the sentinel where it does. Tier 2 then
mandates a fixture `{ headingDeg: 65535 }` — a test for an impossible input.

What is actually missing: ArduPilot can report VFR_HUD heading as a negative int16. Normalise with
`((h % 360) + 360) % 360`. Fix the resolver rule, the fixture and the proto comment together.

### F4 — The position fallback silently changes altitude datum

`resolvePosition` uses `GLOBAL_POSITION_INT.alt_relative_m` (above home) as primary and falls back
to `GPS_RAW_INT`, whose only altitude is `alt_msl_m` (above mean sea level). Both land in `altM`.
The difference is field elevation — routinely hundreds of metres — and nothing in the type warns
the consumer. The `source` field names the message, but the *meaning of the number* changed, not
just its provenance. An `AltitudeTape` reading 1400 instead of 30 during a GPS glitch is a bad
moment.

Fix at the type level in Tier 1: `{ altM, altRef: 'RELATIVE' | 'MSL' }`, and let the instrument
refuse a datum it was not configured for. Or drop the altitude fallback entirely and fall back for
lat/lon only. Either is fine; silence is not.

### F5 — The stall gate zeroes a hovering VTOL

`stallSpeed = isFixedWing(vehicleType) ? 14 : 0` treats the whole VTOL family as fixed-wing. A VTOL
in multicopter mode hovering, or translating at 10 m/s, gets `max(0, 10 - 14) = 0` and shows no
predicted track — during the phase of flight where an operator most wants one. Vehicle type is the
wrong discriminator; flight regime is the right one.

Cheapest correct version: gate on airspeed when `airspeed_m_s` is present and below stall,
otherwise use groundspeed with no floor. Also, 14 m/s is hardcoded for an entire vehicle class
while ArduPilot exposes it as `ARSPD_FBW_MIN`. Have `resolvePredictiveTrajectory` take a stall
speed argument from day one so Tier 7's parameter read can supply it, rather than retrofitting a
signature change later.

### F6 — Assumed dependencies that do not exist

`vitest` is not in `frontend/package.json`; `vite.config.ts` is three lines with no `test` block;
`@bufbuild/protobuf` and `@connectrpc/connect-web` are absent; there is no `@/` alias, which Tier 3
assumes. Also `tsconfig.app.json` sets `exactOptionalPropertyTypes` and `noUncheckedIndexedAccess`,
which the resolver return types (`source?`) and the `track.ts` ring buffer both have to be written
around — port-plan flags this, Tier 1 does not. Individually trivial; collectively the kind of
thing that stalls an agent for an hour at the start of Tier 1.

---

## Product and process

### P1 — Tier 0's safety claim does not hold

Tier 0 removes `SetArmedRequest.force` and justifies it well: "a field that exists can be set by
any caller who bypasses service-layer documentation … removing it is the only safe choice."

In the same file, `CommandService.SendCommand(CommandLong)` accepts any `MavCmd` with free
`param1`–`param7`, plus a `raw_command` escape hatch documented as "the backend passes the raw
uint32 through without interpretation." Force-arm is `MAV_CMD_COMPONENT_ARM_DISARM (400)` with
`param2 = 21196`. Any client that can call `SendCommand` can force-arm — and can send anything else
— regardless of the removed field.

The removal is still right; it removes the obvious footgun and states intent. But Tier 0's exit
gate currently certifies a safety property the system does not have, which is worse than not
claiming it. Say plainly in Tier 0 what the real control is — `SendCommand` validates against an
allowlist with per-command role requirements, and rejects 400 with param2=21196 — write it into the
`CommandLong` proto comment now, and schedule enforcement at Tier 8's command registry. Note also
that port-plan calls the dual-gate safety model "non-negotiable" while nothing in the protos or in
Tiers 0–3 represents it.

### P2 — Authorization has no place in the contract

ADR-0005 requires middleware role checks before every RPC handler. There is no link between the
proto and those roles, so the mapping is a hand-maintained table keyed by method name, with nothing
failing when a new RPC is added and forgotten.

Tier 0 is the last cheap moment to add a custom method option:

```proto
rpc SetArmed(SetArmedRequest) returns (CommandResult) {
  option (gcs.v1.required_role) = ROLE_OPERATOR;
}
```

Middleware then reads the role off the method descriptor and an unannotated RPC fails closed.
Adding this later means a new options file, a full regen, and touching all eight services. For an
agent-maintained repo it converts a drift-prone convention into a schema fact — which is the same
argument Tier 0 already makes for removing `force`.

### P3 — The evidentiary layer depends on a repo that is not here

Tier 2's dependencies list "golden byte fixtures available from `flight-path-hud/contracts/mavlink/`"
and "`flight-path-hud` accessible locally or its contracts directory copied." There is no
flight-path-hud in this repo: no path, no URL, no commit SHA, no submodule, no vendored copy. Every
later tier's evidence rests on it, and an agent reaching Tier 2 simply stops.

Fix in Tier 0 — it is small and it is the foundation of everything downstream. Promote Tier 2's
`scripts/gen_mavlink_fixtures.py` from fallback to primary: pymavlink-generated fixtures are
reproducible from a committed script, which is what ADR-0001's hermeticity argument actually wants,
where copied-from-a-sibling-checkout is not. If flight-path-hud fixtures are still wanted, vendor
them into `contracts/mavlink/` with the upstream commit SHA recorded in a README.

### P4 — Three documents disagree about closed decisions

`docs/roadmap/README.md` indexes `port-plan.md` as "Canonical reference for existing logic". That
document:

- leaves gomavlib-vs-hand-roll open (Open Question 1) and describes porting hand-rolled STX
  detection, CRC-16/MCRF4XX and CRC_EXTRA tables — a decision `order-of-operations.md` closed in
  favour of gomavlib;
- prescribes `out: gen/go` where Tier 3 prescribes `../internal/gen`;
- says "wire `rules_buf` into `MODULE.bazel`" where order-of-operations defers rules_buf;
- still lists five Open Questions that order-of-operations resolved.

`docs/research/qgc-missionplanner-analysis.md` adds a fifth position: "Bazel `genrule` reading
`common.xml`, emitting Go structs."

Any of these is a plausible thing for an agent to implement, and two of them are labelled
authoritative. One owner per decision: port-plan keeps algorithms and the file-to-file mapping,
and every build/tooling claim in it is either deleted or marked superseded, with the same treatment
for the research doc's codegen paragraph.

Related: that research doc identifies QGC's `FirmwarePlugin` and `FactSystem` as patterns "we want",
and neither appears in any tier or proto. `HeartbeatState.flight_mode_name` is documented as
"decoded by the firmware adapter layer" — a layer that does not exist and is not scheduled, so that
field is empty through Tier 8. Either schedule a minimal ArduCopter mode table in Tier 1 (it is a
map literal) or mark the field reserved so nobody builds a UI on it.

### P5 — No exit gate is executable, and there is no CI

"`buf lint` passes", "NaN never returned from any resolver", "matrix has all case IDs populated",
"goroutine count stable over 60s" — a human can adjudicate these; an agent cannot, and neither can
CI, because there is no CI. No `.github/`, no `Makefile`, no test target anywhere in the tree.
Tiers 2 and 3 both write CI YAML into prose without creating the workflow.

This is the highest-leverage change in the first four tiers, and the cheapest. Make each gate a
command that exits non-zero:

```makefile
gate-tier-0: ; cd proto && buf lint && buf breaking --against '.git#branch=origin/main,subdir=proto'
gate-tier-1: ; go test -race ./internal/codec/... && cd frontend && pnpm vitest run src/logic
gate-tier-2: ; ./scripts/check-matrix.sh && go test ./internal/codec/... -run TestMatrixCoverage
gate-tier-3: ; go build ./internal/gen/... && cd frontend && pnpm tsc --noEmit
```

Then "done" is an exit code rather than a judgement call — which is precisely what ADR-0001's
rationale asked for, and the thing that makes the roadmap safe to hand to an agent.

### P6 — Housekeeping

A 7.9 MB compiled `gcs` binary is committed at the repo root. `git ls-files` lists it; `.gitignore`
covers `/bin/` only. Every clone pays for it and any tree-walking agent hits it. `git rm --cached
gcs` and add it to `.gitignore`.

---

## Summary of Tier 0 additions

Tier 0 is described as surgical. The evidence says it is the only cheap moment for eight decisions,
most of which are currently discovered at Tier 7 or later:

1. Settle the project name (five in use today) and fix `go_package` — before the breaking baseline.
2. Decide the buf lint naming posture (B4) — ~40 renames or four documented exceptions.
3. Decide the codec's output type: telemetry vs transaction events (B3).
4. Convert `UploadMission` to unary (F1); audit that no RPC uses client/bidi streaming.
5. Make `payload_types` an enum (F2).
6. Drop duplicate `vehicle_id` from payload messages (BE5).
7. Add the `required_role` method option (P2).
8. State the real force-arm control in the `CommandLong` proto comment (P1).

Every one of these is a proto edit that costs minutes before codegen and a breaking change plus a
regeneration cycle afterwards. That is what Tier 0 is for.
