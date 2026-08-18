# Cockpit, Second Pass — Firmware Variance, Capability Negotiation, Parameter Metadata

## Purpose

`cockpit-blueos-analysis.md` asked three questions — what a configuration layer looks
like, whether we need a telemetry store, where the video port belongs — and answered
them. It did not look at the thing this repo is actually missing.

Two protos declare a layer that does not exist. `proto/gcs/v1/vehicle.proto:38` said
`custom_mode` is "decoded into a human-readable name by the firmware adapter layer,"
and `proto/gcs/v1/track.proto:78` carries the same dependency on `MavlinkDetail.flight_mode`.
`docs/roadmap/tier-0-3-adversarial-review.md:607-611` already flagged it: *"a layer that
does not exist and is not scheduled, so that field is empty through Tier 8."* Its
suggested fix — *"a minimal ArduCopter mode table … it is a map literal"* — is repeated
verbatim at `qgc-missionplanner-analysis.md:25-31`.

**This document supersedes that suggestion.** The mechanism already exists in the
dependency we pin, and a map literal is the specific failure mode
`order-of-operations.md` already closed for vehicle types. The general problem — how a
GCS absorbs firmware variance without growing a per-firmware class hierarchy — has a
better answer than the one Cockpit and QGC both reached.

The decision this document argues for is recorded in **ADR-0007**.

> **License.** Unchanged from the first pass: Cockpit is **AGPL-3.0** (dual-licensed).
> Patterns may be derived; code may not be lifted. Nothing below is a code transcription.

> **Provenance of the Cockpit citations.** The `src/…` file:line references in §4, §5,
> §6 and §8 were read out of a Cockpit checkout during the investigation that produced
> this document. There is no Cockpit checkout in this repo and no network fetch in the
> change that wrote it, so they are reported as recorded, not re-verified here. Every
> claim about **gomavlib**, **our own protos**, and **our own docs** was verified against
> the tree and is cited with a path you can open.

---

## 1. Runtime mode discovery already exists in the pinned dependency

`go.mod` pins `github.com/bluenviron/gomavlib/v3 v3.3.5`. That version already ships the
MAVLink standard-modes microservice:

| Message | ID | File in the pinned module |
|---|---|---|
| `MessageAvailableModes` | 435 | `pkg/dialects/common/message_available_modes.go` |
| `MessageCurrentMode` | 436 | `pkg/dialects/common/message_current_mode.go` |
| `MessageAvailableModesMonitor` | 437 | `pkg/dialects/common/message_available_modes_monitor.go` |

All three are aliased into `pkg/dialects/ardupilotmega`, which is the dialect
`internal/codec` already uses.

`AVAILABLE_MODES` carries exactly the fields a mode picker needs:

```go
type MessageAvailableModes struct {
    NumberModes  uint8
    ModeIndex    uint8              // 1-based; not persistent across reboot
    StandardMode MAV_STANDARD_MODE  `mavenum:"uint8"`
    CustomMode   uint32
    Properties   MAV_MODE_PROPERTY  `mavenum:"uint32"`
    ModeName     string             `mavlen:"35"`
}
```

`MAV_MODE_PROPERTY` has three values
(`pkg/dialects/common/enum_mav_mode_property.go`): `ADVANCED = 1`,
`NOT_USER_SELECTABLE = 2`, `AUTO_MODE = 4`. The middle one is the important one — it is
the vehicle telling the GCS *do not put this in the mode dropdown*. No hand-maintained
table can produce that, because it is a property of the running firmware build and its
frame configuration, not of the firmware family.

The request path needs no contract change: `MAV_CMD_REQUEST_MESSAGE = 512` is already at
`proto/gcs/v1/types.proto:168`, and `SEND-CMD-LONG` is already `complete` in the
capability matrix.

ArduPilot implements `AVAILABLE_MODES` from **4.7.0**. The static fallback for older
firmware is *also* already generated, in the same dialect:

| Enum | File | Has `String()` |
|---|---|---|
| `COPTER_MODE` | `enum_copter_mode.go` | yes, line 151 |
| `PLANE_MODE` | `enum_plane_mode.go` | yes, line 147 |
| `ROVER_MODE` | `enum_rover_mode.go` | yes, line 99 |
| `SUB_MODE` | `enum_sub_mode.go` | yes, line 87 |
| `TRACKER_MODE` | `enum_tracker_mode.go` | yes, line 71 |

**Verdict.** There is nothing to write a table for. Layer (a) is a message we can already
decode; layer (b) is a `String()` method on a constant we already compile. The only new
code is the selection function between them.

**Why the map literal is the wrong answer specifically.** `order-of-operations.md` closed
the identical question for vehicle classification: *"Derived from the generated `MavType`
constants … never from a retyped integer list."* That decision exists because a
hand-copied pre-2019 table mapped `ROCKET(9)` and `GROUND_ROVER(10)` onto `KITE` and
`FLAPPING_WING` after upstream renumbered the VTOL family. A hand-typed mode literal is
the same artefact one enum over.

The current instance is at `docs/roadmap/tier-8-write-transactions.md:132-135`:

```
- ArduCopter GUIDED = 4
- ArduCopter STABILIZE = 0, LOITER = 5, RTL = 6, LAND = 9
- ArduPlane GUIDED = 15
```

Every one of those numbers is **correct today** — checked against
`enum_copter_mode.go:15,23,25,27,31` and `enum_plane_mode.go:43`. That is not the point.
The values are right and have no mechanism for staying right; the generated constants
have the mechanism and need no values typed at all. Importing them makes a dialect bump a
compile error instead of a silent mis-mapping.

Cockpit knows it is on the wrong side of this. `src/libs/vehicle/ardupilot/common.ts:106`
carries the comment `// TODO: Use the new MAVLink Mode microservice`.

---

## 2. `AUTOPILOT_VERSION` is the portability primitive, and we had nowhere to put it

`AUTOPILOT_VERSION` (#148, `pkg/dialects/standard/message_autopilot_version.go`) carries:

```go
Capabilities        MAV_PROTOCOL_CAPABILITY `mavenum:"uint64"`  // 21 defined flags
FlightSwVersion     uint32                  // (major)(minor)(patch)(type), MSB→LSB
MiddlewareSwVersion uint32
OsSwVersion         uint32
BoardVersion        uint32
VendorId, ProductId uint16
Uid                 uint64
Uid2                [18]uint8               // MAVLink 2 extension; supersedes Uid
```

Before this change, **none of it appeared in any of our 19 protos**, and #148 is absent
from the 18-family priority receive set in `order-of-operations.md`.

The 21 flags are enumerated with their wire values in
`pkg/dialects/standard/enum_mav_protocol_capability.go`. The ones that change our
behaviour:

| Flag | Value | Why we care |
|---|---|---|
| `MISSION_INT` | 4 | Our mission protocol is `MISSION_ITEM_INT`-only (`tier-7`, `tier-8` ch.5) |
| `COMMAND_INT` | 8 | Determines whether `COMMAND_INT` is a legal alternative to `COMMAND_LONG` |
| `PARAM_ENCODE_BYTEWISE` | 16 | See §3 — this one is a correctness bug, not a feature gate |
| `FTP` | 32 | Enables the `@PARAM/param.pck` fast path (§4, tier-7) |
| `COMPASS_CALIBRATION` | 4096 | `CalibrationService` is unreachable without it |
| `MAVLINK2` | 8192 | Signing, extension fields |
| `MISSION_FENCE` | 16384 | `MAV_MISSION_TYPE_FENCE` is already in `types.proto:225` |
| `MISSION_RALLY` | 32768 | `MAV_MISSION_TYPE_RALLY`, same |
| `PARAM_ENCODE_C_CAST` | 131072 | See §3 |

**Verdict — and this is the load-bearing claim of the document.** Both reference GCSs
discriminate on **identity**. QGC branches on a `FirmwarePlugin` selected by autopilot
type; Cockpit branches on a hardcoded `switch` over `MavAutopilot` and `MavType` in
`src/libs/vehicle/vehicle-factory.ts:30-91`. `AUTOPILOT_VERSION` lets us discriminate on
**declared capability**, which is the thing that actually varies.

The concrete test of that claim: an ArduPilot 4.5 vehicle and an ArduPilot 4.7 vehicle
differ on `AVAILABLE_MODES`, on the parameter-encoding declaration, and on roughly 330
parameter names (§4). An ArduPilot vehicle and a PX4 vehicle, on any *single* one of those
features, may not differ at all. Identity is a proxy for capability that is wrong in both
directions — it splits vehicles that behave the same and merges vehicles that do not.

Contract answer: `VehicleCapabilities` in `proto/gcs/v1/vehicle.proto`, populated from
#148 requested at discovery, hung off `VehicleSnapshot`, with `MAV_PROTOCOL_CAPABILITY`
and `MAV_MODE_PROPERTY` mirrored value-for-value into `types.proto` beside the other
MAVLink mirrors.

---

## 3. A latent correctness bug in the parameter path

`MAV_PROTOCOL_CAPABILITY_PARAM_ENCODE_BYTEWISE` (16) and
`MAV_PROTOCOL_CAPABILITY_PARAM_ENCODE_C_CAST` (131072) are **mutually exclusive
declarations of how an integer parameter is packed into the float
`PARAM_VALUE.param_value`**. The generated comment says so outright:

> *"Note that either this flag or MAV_PROTOCOL_CAPABILITY_PARAM_ENCODE_C_CAST should be
> set if the parameter protocol is supported."*

`proto/gcs/v1/parameters.proto:14` said:

> *"param_value is always float on the wire; cast per param_type for integer params."*

That sentence silently commits the codec to C_CAST, and records nothing about any vehicle
having declared it. Decode a BYTEWISE vehicle under a C_CAST assumption and integer
parameters read as garbage — a bit pattern reinterpreted as a magnitude — **with no
error**, because both readings are finite floats. A "NaN never returned" gate
(`tier-1-pure-domain-logic.md:373`) passes happily.

This is not hypothetical for us. Tier 8b's first parameter write target is `LOG_BITMASK`
(`docs/roadmap/tier-8-write-transactions.md:81-98`) — an integer bitmask, straight through
that path, chosen precisely because it was believed low-consequence. A bitmask is the
worst case for this bug: every bit is independently meaningful and nothing about a wrong
value looks wrong.

**Verdict.** The cast is a function of declared capability, and the third state is not a
default — it is an error. If neither bit is set, the vehicle has not told us how it
encodes parameters, and the only honest behaviours are *refuse to decode integer
parameters* and *say why*. Guessing C_CAST because it is the common case is how you get a
bug that only appears on the hardware you did not test with.

This is R3 in ADR-0007. `parameters.proto` now states the dependency where the wrong
sentence used to be.

---

## 4. Parameter metadata: Cockpit proves both the value and the mistake

Cockpit vendors `ArduPilot/ParameterRepository` as a git submodule. Each vehicle subclass
statically imports one hardcoded version: `arducopter.ts` → `Copter-4.3`, `ardurover.ts` →
`Rover-4.2`. **There is no runtime version negotiation at all** — a Copter 4.6 vehicle is
described by Copter 4.3 metadata, silently.

The value is also under-collected. The only runtime consumer is one joystick dropdown
reading `BTN0_FUNCTION.Values`; `Range`, `Bitmask`, `RebootRequired` and `ReadOnly` are
parsed and never used.

### Measured drift

Fetched and counted during the investigation — parameter names added or removed between
published `apm.pdef.xml` files:

| Comparison | Names added/removed |
|---|---|
| Copter 4.5.6 → 4.5.7 (patch) | 1 |
| Copter 4.5.7 → 4.6.0 (minor) | **329** |
| Copter 4.5.0 → 4.6.0 | 402 |

**Verdict.** The pin is a real defect, not a nitpick, and the numbers also settle the
granularity question. Pinning the wrong *patch* costs ~1 parameter; pinning the wrong
*minor* costs ~330. So metadata is vendored at **minor-line granularity** — latest patch
of each minor line — and selected at runtime from `AUTOPILOT_VERSION.flight_sw_version`,
nearest vendored version ≤ actual, **with the mismatch surfaced rather than hidden**. A
UI that renders Copter-4.5 metadata against a Copter-4.6 vehicle must say so.

### Source shape, verified by download

- URL pattern:
  `https://autotest.ardupilot.org/Parameters/versioned/{Copter,Plane,Rover,Sub,Tracker}/stable-X.Y.Z/apm.pdef.xml`
  — generated per git tag and immutable.
- Published versions: Copter 35, Plane 31, Rover 20, Sub 10, Tracker 9 — **105 pairs**.
- Copter-4.6.0: 2,222,828 bytes, 4820 parameters,
  sha256 `a1176c87bd2a2e332ac45443b4a78d7581879839931e8fb0f331ca3fb4728dc3`, ~174 KB gzipped.
- **The versioned directories publish XML only.** `apm.pdef.json` exists only in the
  unversioned "latest" directories (`/Parameters/ArduCopter/`). A build-time XML reader is
  required; there is no JSON passthrough to lean on.

Complete field vocabulary across those 4820 parameters, with occurrence counts:

| Field | Count |
|---|---|
| `Range` | 2388 |
| `UnitText` / `Units` | 1409 |
| `Increment` | 973 |
| `RebootRequired` | 551 |
| `Bitmask` | 222 |
| `Calibration` | 192 |
| `ReadOnly` | 39 |
| `Volatile` | 8 |

Plus `<values><value code=…>`, `<bitmask><bit code=…>`, and the attributes `humanName`,
`documentation`, and `user` (`Standard` / `Advanced`).

**Parsing gotcha, and it is a silent one.** Names are inconsistently scoped: vehicle
parameters are prefixed (`ArduCopter:SYSID_THISMAV`), library parameters are bare
(`ARMING_CHECK`). The prefix must be stripped before matching against
`PARAM_VALUE.param_id`, which is 16 unprefixed ASCII characters. Skip the strip and every
vehicle-specific parameter silently loses its metadata while every library parameter keeps
it — a failure that looks like patchy upstream coverage rather than a bug.

That rule is now written into `parameters.proto` on `ParameterMetadata.param_id`, because
a convention that lives only in a research doc is a convention that gets re-derived wrong.

### Hermeticity

Immutable URLs plus sha256 make this a clean `http_file` in `MODULE.bazel` — fetched into
the Bazel cache, never committed to git. That is the answer ADR-0001 asks for and the exact
opposite of Cockpit's `postinstall`. At minor-line granularity that is ~23 of the 105
published pairs, ~46 MB fetched, ~4 MB gzipped, **zero bytes in git**.

Not done here: ADR-0006 §3 requires checking `bazel_compatibility` before adopting any new
ruleset, and that check belongs with the implementation, not with the contract.

---

## 5. Cockpit's extensibility model, and the half of it to copy

Three user-authored action types share one registry. Two of them execute user-supplied
code with full renderer privileges and no sandbox:

| File | Mechanism |
|---|---|
| `src/libs/actions/free-javascript.ts` | `new Function(code)()` |
| `src/libs/actions/data-lake-transformations.ts` | `eval(...)` |

Two further holes on the same surface:

- `src/components/widgets/IFrame.vue` — the widget bridge never validates `event.origin`
  and posts with target origin `'*'`. Any frame that can reach the page can drive it.
- HTTP-request actions accept any URL with no allowlist.

The third action type is the counter-example. `src/libs/actions/mavlink-message-actions.ts`
is **schema-driven**: fields are structurally validated against generated message
definitions and nothing is evaluated.

**Verdict.** The schema-driven one is the pattern, and for us it already exists.
`CommandService.SendCommand` (`proto/gcs/v1/services.proto:73`) behind the explicit
command allowlist and role gate that `order-of-operations.md` already specifies *is* the
same design, expressed in a typed contract instead of a JSON schema. There is no
outstanding work here — only an outstanding prohibition, now recorded in `tier-10`, so
that "add a custom script widget" is a decision someone has to argue against a written
position rather than a gap they fill.

Note that this is not a hypothetical risk profile for a GCS. An `eval` surface in a page
that also holds an authenticated session to a service that can arm an aircraft is not the
usual XSS calculus.

---

## 6. Portability: our backend deletes most of Cockpit's problem

Cockpit has **130 `window.electronAPI?.` call sites** and no port between them and the
application. `isElectron()` sniffs the user-agent string. Capabilities gated on Electron:
serial/TCP/UDP links, LAN vehicle discovery, recording to disk, native TTS, filesystem
storage.

Nearly all of those are *backend* concerns for us. Serial transport and LAN discovery are
`FleetService.ConnectLink`; recording is `VideoService.StartRecording`. The browser never
learns whether a serial port exists, because it was never the thing holding one.

**Verdict.** The structural point survives the fact that we do not have the problem: where
environment genuinely varies, the frontend reads a **capability descriptor served by the
backend**, never an environment sniff. That is §2's discipline one layer up — the same
argument about declared capability versus inferred identity, applied to our own process
boundary instead of the vehicle's.

Two Cockpit mechanisms are worth taking regardless of the above.

**One configurable base address, services as paths under it.** `globalAddress` defaults to
`window.location.hostname`, falls back to mDNS, and every service is a path under it
(`/mavlink2rest/…`, `/bag/…`) behind a reverse proxy. We already terminate TLS at nginx
(`docker/nginx/nginx.conf`), so this is the shape we have — the discipline to add is that
it stays *one* setting rather than N endpoint URLs that can disagree.

**A liveness watchdog distinct from reconnect backoff.** Cockpit recycles sockets that are
`OPEN` but silent — 4 s timeout, 1 s tick — *in addition to* capped exponential backoff.
The two are not the same mechanism: backoff answers "the connection failed", the watchdog
answers "the connection is fine and nothing is coming through it."

We have the second concept for the MAVLink link and not for the client link.
`internal/codec/frame.go:45` sets `HeartbeatTTL = 60 * time.Second` and line 53 derives
`LinkIdleTimeout = 3 * HeartbeatTTL` (180 s), and `TestLinkIdleTimeoutOutlivesHeartbeatTTL`
(`internal/vehicle/state_test.go:334`) pins the ordering. Nothing covers a **Connect
server-stream that is open and delivering nothing** — which is precisely what an operator
sees as "the display froze" and what a reconnect-on-error policy will never notice.
Recorded in `tier-5`.

---

## 7. Non-hermetic build inputs, sharpened

The first pass said "do not copy the postinstall." Specifically, what it fetches:

| Artefact | Source | Pinned |
|---|---|---|
| ffmpeg | `BtbN/FFmpeg-Builds`, `osxexperts.net` | no |
| go2rtc | `AlexxIT/go2rtc` releases | no |
| Piper | `rhasspy/piper` releases | no |
| Piper voice model | HuggingFace | no |

All four are baked into shipped artifacts. Additionally the version string comes from
`git describe --tags` via `execSync`, silently falling back to `'unknown'` when git is
absent or the tree is a tarball — so a released build can ship claiming no version at all.

**Verdict.** Nothing to adopt; one thing to remember. If we ever surface a build version,
it is Bazel-stamped, not shelled. A version obtained by running a subprocess is a version
that is wrong in exactly the environment where it matters most: the shipped container,
where there is no `.git`.

---

## 8. What Cockpit's abstraction actually costs

This is the avoid-this evidence for ADR-0007, and it is worth stating as measurement
rather than opinion.

| Artefact | Size |
|---|---|
| `src/libs/vehicle/mavlink/vehicle.ts` | 1628 lines |
| `src/views/MissionPlanningView.vue` | 5032 lines |
| Unit test files in the repo | 7 total, **none** under `libs/vehicle/` |

Three concrete consequences of discriminating on firmware identity:

1. The four ArduPilot subclasses each carry an **identical** copy-pasted
   `onMAVLinkPackage` block decoding `_mode` from HEARTBEAT. Four copies, one behaviour.
2. `PX4` has no such override at all, so `PX4.mode()` **always returns `MANUAL`**. Not an
   error, not empty — a plausible wrong answer, which is the worst of the three.
3. Generic base-class methods (`startMission`, `pauseMission`, `resetMode`, `returnHome`)
   hardcode ArduPilot mode-name strings and throw for PX4. The base class, the one place
   in the hierarchy that is supposed to be firmware-neutral, is the least neutral part of
   it.

**Verdict.** (2) is the argument in one line. An inheritance hierarchy keyed on firmware
identity makes "we never implemented this for PX4" indistinguishable from "PX4 is in
MANUAL", because the type system is satisfied either way. A capability model cannot
produce that failure: an undeclared capability is a missing bit, and a missing bit is
checkable. R1 layer (c) — *leave `flight_mode_name` empty and render the raw integer,
never fabricate a name* — is the same principle at field granularity.

The absence of tests under `libs/vehicle/` is the second-order cost. A four-way class
hierarchy with a live MAVLink dependency at its root is expensive to test, so it is not
tested, so the duplication in (1) and the gap in (2) survive.

---

## 9. Persistence — define the boundary now, defer the adapters

The first Cockpit document endorsed Cockpit's settings model — per-user *and* per-vehicle
scoping, provenance on every value, deterministic tiebreak — and observed we had nowhere
server-side to put it. Deferring "storage" wholesale would be inconsistent with how this
repo already works.

**The repo's own precedent.** `internal/vehicle/registry.go:3-9` states the pattern
outright:

> *"Forward definition. Tier 5 implements it over Redis (SADD/SREM on fleet:active); Tier 4
> defines it so the fold's consumers can be written and tested against the interface rather
> than against Redis."*

with `NopFleetRegistry` (line 20) whose zero value is usable. `tier-4-bridge-core.md:181`
says the same in prose — *"This chapter is forward-definition only."*
`internal/codec/frame.go:67-73` describes `FrameSource` as "the transport port". ADR-0003
does it for authentication, with Supabase / OIDC / Noop / Mock behind one `AuthProvider`.
Persistence gets the same treatment: **the boundary is defined; the adapters are deferred.**

**This supersedes nothing.** `ADR-0002:93` says Redis "must be treated as ephemeral (fleet
state is rebuilt from live MAVLink, not persisted across restarts as ground truth)".
`ADR-0003:85` says Redis is "a cache and fan-out bus, **not the system of record**." Neither
says there is no system of record — both say Redis is not it, and neither names what is,
because at the time nothing needed one. Naming one completes that sentence. The "ephemeral"
line was scope-bounding for the initial build, not a claim that durable state is
unnecessary.

### Three ports, not one

"Caching and storage" collapses three distinct contracts. A single storage port produces a
lowest-common-denominator interface that no backend implements well — the store that is
good at transactional durability is not the store that is good at fan-out, and an interface
spanning both asks each to pretend.

| Port | Contract | Plausible adapters | Tier where its first caller appears |
|---|---|---|---|
| **State cache** | last-known value, TTL, lossy, rebuildable from live MAVLink | in-memory, Redis (`HSET vehicle:state:<sysId>`, `params:<sysId>`) | Tier 5 |
| **Event history** | append-only, ordered, range-queryable, explicit retention policy | in-memory, Redis Streams (`XADD`), later a datalake sink | Tier 5–6 |
| **Durable record** | transactional, survives restart, provenance per value | in-memory, a durable store, later vehicle-side storage | vehicle-configuration milestone |

The discriminator between the first two and the third is one question: **is it rebuildable
from live MAVLink?** Fleet state, telemetry, and the parameter cache are — losing them costs
a reconnect. Per-user/per-vehicle settings, widget/layout profiles, audit history that must
outlive a restart, and the resolved parameter-metadata selection per vehicle (§4) are not —
losing them loses operator intent, which no vehicle can re-supply. That test is not a
judgement call, and it is why the split is worth drawing before anything is built.

### Two limits, stated as limits

Neither of these is a reason not to draw the boundary. Both are reasons the three ports
land at different times rather than all now.

**1. A port with no caller is speculative API design.** `FleetRegistry` worked as a forward
definition because the fold *already emitted* exactly the events it consumes — `AddVehicle`
on `VEHICLE_DISCOVERED` / `VEHICLE_RECOVERED`, `RemoveVehicle` on `VEHICLE_LOST` — and only
the backend was missing. The shape was derived from a caller, not imagined for one. Nothing
in the tree calls a settings store today.

The repo argues this against itself, and it is the strongest evidence available.
`internal/vehicle/event.go:26-33`, on why `Warning` is a Go type and not a proto:

> *"Adding a proto message commits the wire contract in Tier 0 terms — buf breaking then
> owns it forever — for a shape Tier 5 has not yet had to use. Promote it when a client
> needs to render it."*

That is the same argument, applied by this repo, to this repo, three tiers ago. It is why
each port waits for its first caller rather than all three landing at once — and why the
durable-record port is named here and defined at the vehicle-configuration milestone, not
today.

**2. For settings, the port is the easy half.** The first Cockpit document already found
this and it is worth quoting because it is the part that gets skipped:

> *"none of the hard parts above are about reading config. They are about reconciling it
> across nodes."*

A `Get`/`Set` port hides epoch provenance, per-user and per-vehicle scoping,
merge-versus-last-write-wins, and the tiebreak that makes a merge deterministic when two
clients are connected at once (Cockpit's rule: newer epoch wins, and **on a tie the vehicle
wins**). Those belong in the port's contract, or they leak into every adapter and each one
reconciles slightly differently.

### What this change actually delivers

The contract half only — the part where being wrong later is a breaking change.

`SettingsRecord` and `SettingsScope` enter `proto/gcs/v1/auth.proto`: value, an explicit
`epoch_last_changed_ms`, an `origin` discriminator that makes the tiebreak expressible, and
a scope carrying both the operator ID and the vehicle. No settings service, no RPC, no Go
interface. The reconciliation semantics are in the contract from the start rather than
rediscovered per adapter.

One wrinkle worth naming, because it is not obvious and this change is the moment it
became answerable: **`VehicleId` is the wrong scope key.** `(system_id, component_id)` is
transport identity — sysid is operator-assignable and reused across airframes, so settings
keyed on it follow the *slot*, not the vehicle. `AUTOPILOT_VERSION` carries `Uid` and the
`Uid2` extension, which are hardware identity, and `VehicleCapabilities` (§2) is where they
now land. So the durable scope key is the vehicle UID, and the contract says so.

**One constraint on the eventual adapter is already fixed by the tree.** `Dockerfile:22`:

```dockerfile
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/gcs ./cmd/gcs
```

The comment above it gives the reason — "CGO off so the binary runs on any base image" —
and the image is `FROM alpine:3.22`, so this is deliberate rather than incidental. **Any
durable-record adapter whose driver requires CGO would break that build.** That rules out a
class of otherwise-obvious choices, and it is a fact about our build rather than a
recommendation about any store. No technology is named here because none has been chosen.

Recording it now is the difference between an adapter decision made with the constraint in
view and one made against a container build that has already stopped working.

**No ADR for persistence.** The boundary is recorded; the adapters, the schema, and the
migration story are not, because the durable-record port has no caller. The ADR is due when
one arrives — earliest plausible triggers: a tier scheduling the settings layer (tier-10
owns layout profiles), audit history being required to survive a restart (`security.proto`
declares `AuditEvent` and `SecurityService.WatchAuditLog` with only Redis Streams under
them), or per-vehicle metadata selection needing to persist rather than be re-resolved from
`AUTOPILOT_VERSION` on each discovery.

---

## What this document changes

| Before | After |
|---|---|
| `vehicle.proto:38` names a "firmware adapter layer" that does not exist | Three-layer resolution, named, with "empty means unknown" stated |
| `qgc-missionplanner-analysis.md:33` endorses `FirmwarePlugin` | Superseded — the pattern is right, capability negotiation is the better mechanism (ADR-0007) |
| Adversarial review proposes a mode map literal | Superseded — generated dialect enums, per the `MavType` precedent |
| `parameters.proto` silently assumes C_CAST | Encoding derived from `AUTOPILOT_VERSION`; unknown is an error |
| No home for `AUTOPILOT_VERSION` | `VehicleCapabilities` on `VehicleSnapshot` |
| No parameter metadata contract | `ParameterMetadata` / `ParameterMetadataSet` + `GetParameterMetadata` |
| Persistence deferred wholesale | Three ports named with their contracts; `SettingsRecord` carries provenance in the contract (§9) |

## Cross-references

- `docs/research/cockpit-blueos-analysis.md` — first pass: configuration, data lake,
  storage, video port. Its findings stand; this document does not revise any of them.
- `docs/research/qgc-missionplanner-analysis.md` — the `FirmwarePlugin` endorsement at
  line 33 and the scheduling note at lines 25-31 are **superseded by ADR-0007**.
- `docs/roadmap/tier-0-3-adversarial-review.md:607-611` — the finding this closes; its
  proposed remedy is superseded, its diagnosis was correct.
- `docs/adr/0007-firmware-variance-via-capability-negotiation.md` — the decision.
