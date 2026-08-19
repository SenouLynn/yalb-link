# 0007 — Firmware Variance via Capability Negotiation

**Status:** Accepted. Amended 2026-08-18 — see the Amendment at the end (§5's metadata
source; the decision is unchanged, one stated reason was wrong).

**Supersedes:** the `FirmwarePlugin` endorsement in
`docs/research/qgc-missionplanner-analysis.md:33` and its scheduling note at lines 25-31;
the "minimal ArduCopter mode table … it is a map literal" remedy proposed in
`docs/roadmap/tier-0-3-adversarial-review.md:607-611`.

**Evidence:** `docs/research/cockpit-blueos-analysis-ii.md` §1–§9.

---

## Context

Two protos declared a layer that does not exist. `proto/gcs/v1/vehicle.proto:38` said
`custom_mode` is "decoded into a human-readable name by the firmware adapter layer", and
`proto/gcs/v1/track.proto:78` carries the same dependency. Nothing in any tier builds such
a layer, so both fields are empty through Tier 8. The adversarial review found this and
proposed a hand-typed ArduCopter mode table; the QGC research doc proposed adopting QGC's
`FirmwarePlugin`.

Both proposals discriminate on **firmware identity**. Neither was adopted, and this ADR
records why, because the alternative is not obvious and the evidence for it is measurable.

### What firmware variance actually looks like

Three facts, all verified against the tree and the pinned dependency:

1. **Mode names are already discoverable at runtime.** `gomavlib v3.3.5` — the version in
   `go.mod` — ships `AVAILABLE_MODES` (#435), `CURRENT_MODE` (#436) and
   `AVAILABLE_MODES_MONITOR` (#437) in `pkg/dialects/common`, aliased into
   `ardupilotmega`. `AVAILABLE_MODES` carries `ModeName string` and
   `Properties MAV_MODE_PROPERTY`, where `NOT_USER_SELECTABLE = 2` is the vehicle telling
   the GCS which modes to keep out of a picker. ArduPilot implements it from 4.7.0.
   `MAV_CMD_REQUEST_MESSAGE = 512` is already at `proto/gcs/v1/types.proto:168`.

2. **The static fallback is already generated.** `COPTER_MODE`, `PLANE_MODE`,
   `ROVER_MODE`, `SUB_MODE` and `TRACKER_MODE` exist in the same dialect, each with a
   `String()` method (`enum_copter_mode.go:151`, `enum_plane_mode.go:147`,
   `enum_rover_mode.go:99`, `enum_sub_mode.go:87`, `enum_tracker_mode.go:71`).

3. **Capability is declared on the wire and we had nowhere to put it.**
   `AUTOPILOT_VERSION` (#148) carries a 21-flag `MAV_PROTOCOL_CAPABILITY` bitmap plus
   `FlightSwVersion`, board/vendor/product IDs and UID. Before this change none of it
   appeared in any of our 19 protos, and #148 was absent from the 18-family priority
   receive set.

### The cost of the identity-based alternative, measured

From `cockpit-blueos-analysis-ii.md` §8, on Cockpit's `MavAutopilot`/`MavType` switch
(`src/libs/vehicle/vehicle-factory.ts:30-91`) and the class hierarchy under it:

- `src/libs/vehicle/mavlink/vehicle.ts` is 1628 lines; `MissionPlanningView.vue` is 5032.
- The four ArduPilot subclasses each carry an **identical** copy-pasted `onMAVLinkPackage`
  block decoding `_mode` from HEARTBEAT.
- `PX4` has no such override, so **`PX4.mode()` always returns `MANUAL`** — not an error,
  not empty, a plausible wrong answer.
- Generic base-class methods (`startMission`, `pauseMission`, `resetMode`, `returnHome`)
  hardcode ArduPilot mode-name strings and throw for PX4.
- 7 unit test files in the repository; **none** under `libs/vehicle/`.

The third bullet is the argument in one line. A hierarchy keyed on firmware identity makes
"we never implemented this for PX4" indistinguishable from "PX4 is in MANUAL", because the
type system is satisfied either way.

### The bug this closes

`MAV_PROTOCOL_CAPABILITY_PARAM_ENCODE_BYTEWISE` (16) and `PARAM_ENCODE_C_CAST` (131072)
are mutually exclusive declarations of how an integer parameter is packed into the float
`PARAM_VALUE.param_value`. `parameters.proto:14` said "param_value is always float on the
wire; cast per param_type for integer params" — silently committing to C_CAST without
recording that any vehicle declared it. Decode a BYTEWISE vehicle under a C_CAST
assumption and integer parameters read as garbage, **with no error**, because both
readings are finite floats.

Tier 8b's first parameter write target is `LOG_BITMASK`
(`docs/roadmap/tier-8-write-transactions.md:81-98`) — an integer bitmask, straight through
that path, chosen because it was thought low-consequence.

---

## Decision

### 1. Firmware variance is absorbed by declared capability, not by firmware identity

There is no `FirmwarePlugin`-style per-firmware class or interface hierarchy, and no
`switch` on `(MavAutopilot, MavType)` that selects behaviour. Every optional protocol path
is gated on a bit in `VehicleCapabilities`, populated from `AUTOPILOT_VERSION` (#148)
requested at discovery.

`VehicleCapabilities` is a first-class contract object on `VehicleSnapshot`
(`proto/gcs/v1/vehicle.proto`), with `MAV_PROTOCOL_CAPABILITY` and `MAV_MODE_PROPERTY`
mirrored value-for-value into `types.proto` beside the existing MAVLink mirrors.

**Why capability beats identity, stated so it can be tested against:** an ArduPilot 4.5
vehicle and an ArduPilot 4.7 vehicle differ on `AVAILABLE_MODES`, on the parameter-encoding
declaration, and on ~330 parameter names. An ArduPilot vehicle and a PX4 vehicle, on any
single one of those features, may not differ at all. Identity is a proxy that is wrong in
both directions — it splits vehicles that behave alike and merges vehicles that do not.

`(MavAutopilot, MavType)` remains legitimate for exactly one thing: selecting *which*
generated dialect enum to consult in R1 layer (b). That is a lookup, not a behaviour
branch, and it is confined to one pure function.

### 2. Mode names resolve in three ordered layers (R1)

One pure function, table-tested, no class hierarchy:

| Layer | Source | Condition |
|---|---|---|
| (a) | `AVAILABLE_MODES.mode_name` | The vehicle supplied it. Authoritative, and carries `NOT_USER_SELECTABLE` / `ADVANCED` for UI gating. |
| (b) | Generated dialect enum `String()`, selected by `(MavAutopilot, MavType)` | No `AVAILABLE_MODES` — pre-4.7 firmware. |
| (c) | Empty `flight_mode_name`; the UI renders the raw `custom_mode` integer | Neither is available. |

**Never fabricate a name.** Layer (c) is a designed outcome, not a failure path — the same
principle as §1's `PX4.mode()` finding, at field granularity. An empty string means
unknown, and `vehicle.proto` now says so where the reference to the nonexistent adapter
layer used to be.

### 3. Static tables derive from generated constants, never hand-typed

This is not a new rule. `order-of-operations.md` already closed it for vehicle
classification — *"Derived from the generated `MavType` constants … never from a retyped
integer list"* — after a hand-copied pre-2019 table mapped `ROCKET(9)` and
`GROUND_ROVER(10)` onto `KITE` and `FLAPPING_WING`. This ADR extends the same rule to
flight modes and records that the extension is deliberate.

The mode table at `tier-8-write-transactions.md:132-135` is replaced accordingly. Note that
every value in it was **correct** when checked against `enum_copter_mode.go` and
`enum_plane_mode.go`. Correctness was never the objection: a retyped table has no mechanism
for staying correct, and the generated constants need no values typed at all. Importing
them turns a dialect bump into a compile error instead of a silent mis-mapping.

### 4. Unknown capability is an error state, not a default (R3)

The integer-parameter cast is a function of the declared encoding bit. If **neither**
`PARAM_ENCODE_BYTEWISE` nor `PARAM_ENCODE_C_CAST` is set, the codec **refuses to decode
integer parameters and reports why**. It does not fall back to the common case.

Generalised: an absent capability bit means *not declared*, which is distinct from
*declared absent*, and neither is *present*. Where the difference is observable, the
contract carries it and the UI shows it. Guessing the common case is how a bug arrives on
the one airframe nobody tested with.

### 5. Parameter metadata is versioned, vendored hermetically, and selected at runtime (R4)

`ParameterMetadata` and `ParameterMetadataSet` enter `parameters.proto` now — before Tier 7
writes tests against the narrow name/value/type shape — with `GetParameterMetadata` on
`ParameterService` at `OPERATOR_ROLE_OBSERVER`.

Metadata is vendored per (vehicle, **minor line**) — latest patch of each minor line — and
selected **at runtime** from `AUTOPILOT_VERSION.flight_sw_version`: nearest vendored
version ≤ actual, **with the mismatch surfaced rather than hidden**.

Granularity is set by measurement, not taste (`cockpit-blueos-analysis-ii.md` §4):

| Comparison | Parameter names added/removed |
|---|---|
| Copter 4.5.6 → 4.5.7 (patch) | 1 |
| Copter 4.5.7 → 4.6.0 (minor) | 329 |
| Copter 4.5.0 → 4.6.0 | 402 |

Pinning the wrong patch costs ~1 parameter; pinning the wrong minor costs ~330. This is the
specific thing Cockpit gets wrong: it statically imports one hardcoded version per vehicle
subclass (`arducopter.ts` → `Copter-4.3`, `ardurover.ts` → `Rover-4.2`) with no runtime
negotiation at all.

Sources are immutable per-tag URLs with known sha256, so vendoring is `http_file` in
`MODULE.bazel` — fetched into the Bazel cache, **zero bytes in git**. Not implemented here:
ADR-0006 §3 requires checking `bazel_compatibility` before adopting any ruleset, and that
belongs with the implementation.

### 6. Where environment varies, serve a capability descriptor — never sniff

The frontend does not infer its environment. Where a capability genuinely depends on the
deployment (serial links, LAN discovery, recording), the backend declares it and the
frontend reads the declaration. Cockpit's `isElectron()` user-agent sniff across 130
`window.electronAPI?.` call sites is the counter-example; this is §1's discipline applied
one layer up, at our own process boundary instead of the vehicle's.

### 7. No `eval` execution path for user-authored content

Extensibility goes through schema-validated typed RPCs — for us, `CommandService.SendCommand`
behind the command allowlist and role gate `order-of-operations.md` already specifies.
Cockpit runs `new Function(code)()` (`src/libs/actions/free-javascript.ts`) and `eval(...)`
(`src/libs/actions/data-lake-transformations.ts`) unsandboxed with full renderer
privileges, and its iframe widget bridge (`src/components/widgets/IFrame.vue`) never
validates `event.origin` and posts to `'*'`.

An `eval` surface in a page holding an authenticated session to a service that can arm an
aircraft is not the usual XSS calculus. Recorded in `tier-10-ui-composition.md` so that
adding one is a decision argued against a written position, not a gap someone fills.

---

### 8. Persistence: define the ports now, defer the adapters (R5)

Deferring "storage" wholesale would be inconsistent with how this repo already works.
`internal/vehicle/registry.go:3-9` states the pattern — *"Forward definition. Tier 5
implements it over Redis (SADD/SREM on fleet:active); Tier 4 defines it so the fold's
consumers can be written and tested against the interface rather than against Redis"* —
with `NopFleetRegistry` whose zero value is usable. `tier-4-bridge-core.md:181` says the
same in prose. `internal/codec/frame.go:67-73` calls `FrameSource` "the transport port".
ADR-0003 does it for authentication. Persistence gets the same treatment.

**This supersedes nothing.** `ADR-0002:93` says Redis "must be treated as ephemeral";
`ADR-0003:85` says Redis is "a cache and fan-out bus, **not the system of record**." Neither
says there is no system of record — both say Redis is not it, and neither names what is,
because at the time nothing needed one. Naming one completes that sentence. The "ephemeral"
line was scope-bounding for the initial build, not a claim that durable state is
unnecessary.

**Three ports, not one.** A single storage port yields a lowest-common-denominator interface
no backend implements well.

| Port | Contract | Plausible adapters | Tier where its first caller appears |
|---|---|---|---|
| **State cache** | last-known value, TTL, lossy, rebuildable from live MAVLink | in-memory, Redis (`HSET vehicle:state:<sysId>`, `params:<sysId>`) | Tier 5 |
| **Event history** | append-only, ordered, range-queryable, explicit retention | in-memory, Redis Streams (`XADD`), later a datalake sink | Tier 5–6 |
| **Durable record** | transactional, survives restart, provenance per value | in-memory, a durable store, later vehicle-side | vehicle-configuration milestone |

The discriminator is one question: **is it rebuildable from live MAVLink?** Fleet state,
telemetry and the parameter cache are. Per-user/per-vehicle settings, layout profiles, audit
history that must outlive a restart, and the resolved parameter-metadata selection of §5 are
not — losing them loses operator intent, which no vehicle can re-supply.

**Each port lands in the tier that first has a caller, not earlier**, and the reason is
recorded honestly rather than as an aside. `FleetRegistry` worked as a forward definition
because the fold *already emitted* exactly the events it consumes; the shape was derived
from a caller rather than imagined for one. Nothing in the tree calls a durable record
today. The repo makes this argument against itself at `internal/vehicle/event.go:26-33`,
on why `Warning` is a Go type and not a proto: *"Adding a proto message commits the wire
contract in Tier 0 terms — buf breaking then owns it forever — for a shape Tier 5 has not
yet had to use."* An interface with no implementations **and** no callers is speculative
API design.

**For settings, the port is the easy half.** `cockpit-blueos-analysis.md` found the real
problem: *"none of the hard parts above are about reading config. They are about
reconciling it across nodes."* A `Get`/`Set` port hides epoch provenance, per-user and
per-vehicle scoping, merge-versus-last-write-wins, and the tiebreak. Those go in the port's
contract or they leak into every adapter.

**So this change delivers the contract half only**, which is the half where being wrong
later is a breaking change: `SettingsRecord` and `SettingsScope` in
`proto/gcs/v1/auth.proto`, carrying value, `epoch_last_changed_ms`, an `origin`
discriminator that makes the tiebreak expressible, and a scope of (operator, vehicle). No
service, no RPC, no Go interface.

Note that the scope key is **not** `VehicleId`. `(system_id, component_id)` is transport
identity — sysid is operator-assignable and reused across airframes, so settings keyed on it
follow the slot rather than the vehicle. `AUTOPILOT_VERSION.uid`/`uid2` is hardware
identity, and §1 puts it in `VehicleCapabilities`, which is what makes the durable key
answerable now.

**One constraint on the eventual adapter is already fixed by the tree.** `Dockerfile:22`
builds `CGO_ENABLED=0` — "CGO off so the binary runs on any base image", onto
`alpine:3.22`. Any durable-record adapter whose driver requires CGO would break that build.
That is a fact about our build, not a recommendation; **no technology is named here because
none has been chosen.**

**No ADR for persistence yet**, because the durable-record port has no caller. It is due
when one arrives — earliest plausible triggers: a tier scheduling the settings layer
(tier-10 owns layout profiles); audit history being required to survive a restart, where
`security.proto`'s `AuditEvent` and `SecurityService.WatchAuditLog` have only Redis Streams
under them; or per-vehicle metadata selection needing to persist rather than be re-resolved
from `AUTOPILOT_VERSION` on each discovery. Whichever arrives supplies the concrete schema
the ADR should be written against.

---

## Consequences

**Accepted costs:**

- Discovery gains a round trip. `MAV_CMD_REQUEST_MESSAGE` for #148 must complete before
  any capability-gated path is decided, so there is a window after first HEARTBEAT in which
  capabilities are unknown. That window is a state the UI has to render, not a state it
  can skip — which is the point of §4, and it is the cost of not guessing.
- A vehicle that never answers #148 is permanently in the unknown state. Integer parameter
  decode stays refused for it. This is a real functional regression against "assume
  C_CAST", and it is accepted: a refusal is debuggable and a silent misread is not.
- Three mode-resolution layers are more code than one map literal, and the fixtures to
  test all three are more work than the fixtures to test one.
- `ParameterMetadata` enters the contract before anything consumes it, so `buf breaking`
  guards a shape that is not yet exercised. Cheaper than widening it after Tier 7 has
  tests against the narrow one.
- Metadata vendoring adds ~46 MB of Bazel-cached external fetch and a build-time XML
  reader. The versioned ArduPilot directories publish XML only — `apm.pdef.json` exists
  solely in the unversioned "latest" directories — so there is no JSON passthrough to lean
  on.

**Expected benefits:**

- `flight_mode_name` and `MavlinkDetail.flight_mode` stop being permanently empty, without
  scheduling a layer that does not exist.
- The `PX4.mode()` failure mode is structurally unavailable: an undeclared capability is a
  missing bit, and a missing bit is checkable.
- One pure function over generated constants replaces a class hierarchy. It is Tier 1's
  charter exactly, testable with no socket, no vehicle, and no Docker.
- The parameter-encoding bug is closed before the first write lands on `LOG_BITMASK`.
- New optional protocol paths (`FTP`-gated MAVFTP parameter download, `MISSION_FENCE`,
  `MISSION_RALLY`, `COMPASS_CALIBRATION`) become one capability check each, with no new
  branch on firmware identity.

**Monitoring.** The claim to watch is that no `switch` on `MavAutopilot` ever acquires a
behavioural branch. Layer (b)'s dialect-enum selection is the sole permitted use; if a
second one appears, the capability model is failing to carry something and that is the
signal to widen `VehicleCapabilities`, not to add a branch.

---

## Amendment (2026-08-18) — §5's metadata source

**The decision stands. One of its stated reasons is wrong.**

§5 vendors parameter metadata per minor line, selected at runtime from
`flight_sw_version` with the mismatch surfaced. None of that changes. What changes is the
accepted cost recorded above:

> Metadata vendoring adds ~46 MB of Bazel-cached external fetch and a build-time XML
> reader. The versioned ArduPilot directories publish XML only — `apm.pdef.json` exists
> solely in the unversioned "latest" directories — so there is no JSON passthrough to lean
> on.

That is true of `autotest.ardupilot.org/Parameters/versioned/`, which is the source §5 was
written against. It is **not** true of
[`ArduPilot/ParameterRepository`](https://github.com/ArduPilot/ParameterRepository).
Checked against the GitHub API on 2026-08-18:

- 60 top-level directories named per **minor line** — `Copter-3.5` … `Copter-4.8`,
  `Plane-*`, `Rover-*`, `Sub-*`, `Tracker-*`, `Blimp-*`, `AP_Periph-*`. That is exactly the
  granularity §5 chose, and it is chosen for us rather than derived by picking the latest
  patch of each line.
- `Copter-4.7/` holds **both** `apm.pdef.json` (2,163,839 B) **and** `apm.pdef.xml`
  (2,710,473 B), plus `MAVLinkMessages.rst` (125,026 B), `Parameters.md`, `Parameters.rst`,
  `Parameters.html`.
- Actively maintained — commits 2026-08-06, 08-11, 08-16, all "Update metadata".

### What this does and does not change

**Not a reason to switch on its own.** A Go build-time converter reads XML or JSON from the
standard library, so "a build-time XML reader" was never a real cost. The honest difference
is the fetch shape:

| | `autotest.ardupilot.org` (§5 as written) | `ArduPilot/ParameterRepository` |
|---|---|---|
| Granularity | `stable-X.Y.Z`; we pick the latest patch per minor line | Per minor line, already |
| Immutability | Per-tag URLs, immutable, known sha256 | Branch-tracking directories; pinned by **commit SHA** instead |
| Fetch shape | ~23 individually pinned `http_file` entries | One pin yields all 60 sets |
| Size | ~46 MB, only what is needed | Larger — the whole repository, including `Parameters.html`/`.rst` we do not want |
| Formats | XML | XML **and** JSON |

Both give hermeticity; they differ in mechanism, not in guarantee. Immutable-URL-plus-sha256
is the stronger primitive and fetches less. One pin for all 60 sets is simpler and removes
the "which patch is latest?" step. **The choice belongs with whoever writes the converter**,
against a real Bazel target rather than against this table — and ADR-0006 §3's
`bazel_compatibility` check (deferred with the implementation) may come out differently for
the two, since the ParameterRepository route may need no new ruleset at all.

### The part neither research pass noticed

`MAVLinkMessages.rst` is a **per-firmware-version list of the MAVLink messages that
firmware actually handles**. That is a second, orthogonal capability source alongside
`AUTOPILOT_VERSION`'s bitmap: the capability bits declare *protocol features*, this declares
*which messages are handled*. It is directly in the spirit of §1 — absorb variance by asking
what the vehicle supports rather than by branching on what it is — and nothing in this ADR
uses it. Recorded as open, not scheduled.

### Provenance

Found while reconciling a second, independent Cockpit survey against this one. Full
comparison, including what each pass missed, in
`../research/cockpit-reference-reconciliation.md`.
