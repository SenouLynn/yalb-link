# Changelog

All notable changes follow [Keep a Changelog](https://keepachangelog.com) conventions.
Format: `## [version] - YYYY-MM-DD`. Unreleased changes accumulate at the top.

---

## [Unreleased]

### Changed — Cockpit survey reconciliation (2026-08-18)

Cockpit was surveyed twice, independently. The second survey
(`docs/wip/cockpit-reference-analysis.md`, written on an unmerged branch against `27a9ab5`)
is reconciled against the first and closed out. Full comparison:
`docs/research/cockpit-reference-reconciliation.md`; the WIP file is deleted.

**Outcome:** the committed pass went further on everything the two shared and shipped
proto for it. It also found the parameter-encoding bug the other missed entirely. The other
pass covered four areas the committed one never touched, and those are what this change
acts on.

**ADR-0007 amended — the decision stands, one stated reason was wrong.** §5's accepted cost
says "the versioned ArduPilot directories publish XML only … there is no JSON passthrough to
lean on." True of `autotest.ardupilot.org`, false of `ArduPilot/ParameterRepository`, whose
60 directories are named per **minor line** — exactly the granularity §5 chose — and each
carry `apm.pdef.json` *and* `apm.pdef.xml` *and* `MAVLinkMessages.rst`. Verified against the
GitHub API, not recalled. The source choice is left with whoever writes the converter, with
the trade-off tabled: immutable per-tag URLs with sha256 fetch less, one commit pin fetches
all 60 sets and removes the "which patch is latest?" step. `MAVLinkMessages.rst` — a
per-firmware-version list of the messages that firmware handles — is a second capability
source neither pass noticed.

**Tier 7 had three defects, all now fixed:**
- **Its `.plan` exit gate could not be met.** It required a "field-by-field comparison with
  QGC `.plan` format" and nothing in the repo defined that mapping — zero hits for
  `fileType`, `SimpleItem`, `QGC WPL`. Now written as
  `docs/reference/mission-interchange-formats.md`, every constant read from QGC `master`
  source rather than recalled: `fileType:"Plan"` v1, `mission`/`geoFence`/`rallyPoints` all
  v2, `SimpleItem` with a **7-element** `params` array (5/6/7 are lat/lon/alt), `doJumpId`
  carrying the sequence number. It is a *written* gate, not a `make` target, so nothing was
  red in CI — it was simply unmeetable.
- **`ParametersPanel` was still spec'd as `param_id | value | type | index / count`** —
  the exact narrow shape ADR-0007 §5 says `ParameterMetadata` entered the contract early to
  prevent. The contract moved at `306a429`; the tier plan had not followed. Now consumes
  `GetParameterMetadata`, with the traps written into the spec: read presence before value
  on the `optional` numerics (0 is legal for all three, and only ~half of ArduPilot's
  parameters declare a range), render bitmask keys as `1 << key` because they are bit
  indices not masks, and surface a staleness banner on `is_exact_match == false`.
- **It called `GetCachedParameters`, an RPC that does not exist.** `ParameterService` has
  `ListParameters`, `GetParameter`, `SetParameter`, `GetParameterMetadata`. Flagged in place
  with the two ways out; serving the cached read as a mode of `ListParameters` is preferred,
  since adding the RPC is now a `buf breaking` event rather than a free Tier 0 edit.

**Manual control recorded as deferred rather than absent.** A repo-wide grep for
`manual_control|joystick|gamepad|rc_channel|rc_override` returned one incidental hit, and
the out-of-scope table did not mention it — so it read as oversight. It is now a row there,
carrying the constraints so the eventual ADR inherits them: a third write class,
unacknowledged and continuous, incompatible with the command registry and per-action audit
Tier 8 is built on; needs rate limiting, a deadman, take-control handoff, and bandwidth
arithmetic against `RADIO_STATUS.txbuf`. Gate it on
`MAV_PROTOCOL_CAPABILITY_COMPONENT_ACCEPTS_GCS_CONTROL` (524288), which landed in
`types.proto` with no consumer. Open tension recorded, not papered over: the browser Gamepad
API only reads while the tab is focused, which is the strongest Electron reopen-trigger on
file.

**The unit-normalisation rule now documents its own exception.** `order-of-operations.md`
read as an unqualified "protos carry SI units" while `telemetry.proto` deliberately keeps
`hdg_cdeg`, `cog_cdeg`, `vel_cm_s` and the battery integers in wire units. The exception was
recorded in the capability matrix, in per-field proto comments and in this changelog — but
not in the canonical decision row, which is the line a contributor would cite. A future
reader would either "fix" those fields and destroy the sentinels, or cite them as precedent
to skip normalisation elsewhere. The reason is now stated: the sentinel
(`65535` / `INT16_MAX` / `-1`) is defined in wire units and a divide turns it into an
unrecognisable magnitude. A field with no sentinel gets normalised like everything else.

**Left open, deliberately, and listed in one place** so they are findable rather than
rediscovered: `Waypoint { position, repeated Command }` (now a proto break, no longer the
free Tier 0 edit it would have been before `25afbf6`); multi-instance telemetry addressing,
where `BatteryStatus.id` exists but the Tier 1 fold flattens to scalars and two batteries
silently last-writer-wins; the WebRTC trickle-ICE disagreement between the two passes, plus
the missing `VIDEO_STREAM_TYPE_ANALOG` and the unset-vs-zero convention for
`packet_loss_pct`/`latency_ms`; `ComplexItem` survey definitions, which have no proto
equivalent and make a survey round-trip lossy by design; tlog; `.ass` telemetry subtitles;
replay-as-a-link at the transport port; and `protoreflect` as a runtime schema, still
unscheduled by either pass.

### Added — Firmware capability negotiation + parameter metadata (2026-08-17)

Three commits that had no changelog entry: `5617567` (cockpit research), `306a429`
(parameters pivot), `c536f16` (capability matrix).

**ADR-0007 — Firmware Variance via Capability Negotiation (Accepted).** Two protos declared
a firmware adapter layer that nothing built, so `flight_mode_name` and
`MavlinkDetail.flight_mode` were empty through Tier 8. Both proposed remedies discriminated
on firmware *identity* — QGC's `FirmwarePlugin`, and a hand-typed ArduCopter mode table —
and both were rejected. Variance is absorbed by **declared capability** instead: every
optional protocol path is gated on a bit in `VehicleCapabilities`, populated from
`AUTOPILOT_VERSION` (#148) at discovery. Identity is a proxy that is wrong in both
directions: an ArduPilot 4.5 and an ArduPilot 4.7 vehicle differ on `AVAILABLE_MODES`, on
the parameter-encoding declaration and on ~330 parameter names, while an ArduPilot and a
PX4 vehicle may not differ at all on any one feature.

**The bug this closed, which is the reason it was worth doing.**
`MAV_PROTOCOL_CAPABILITY_PARAM_ENCODE_BYTEWISE` (16) and `PARAM_ENCODE_C_CAST` (131072) are
mutually exclusive declarations of how an integer parameter is packed into the float
`PARAM_VALUE.param_value`. `parameters.proto` had silently committed to C_CAST. Decode a
BYTEWISE vehicle under that assumption and integer parameters read as garbage **with no
error**, because a bit pattern reinterpreted as a magnitude is still a finite float — so
even the "NaN never returned" gate passes. Tier 8b's first parameter write target is
`LOG_BITMASK`, an integer bitmask straight through that path, chosen because it was thought
low-consequence. Unknown encoding is now an error, not a default.

**Mode names resolve in three ordered layers** (`tier-1-pure-domain-logic.md` ch.6a,
`internal/codec/firmware.go`): `AVAILABLE_MODES.mode_name` (#435, ArduPilot ≥ 4.7.0) →
generated gomavlib dialect enum `String()` selected by `(MavAutopilot, MavType)` → empty.
No table is written: `COPTER_MODE`, `PLANE_MODE`, `ROVER_MODE`, `SUB_MODE` and
`TRACKER_MODE` already ship in `gomavlib v3.3.5` with `String()` methods. Layer (c)
returning empty is a designed outcome — Cockpit's `PX4.mode()` returns `MANUAL`
unconditionally because the subclass never overrode it, which is a plausible wrong answer
the type system cannot catch. An empty string is checkable; a fabricated name is not.

**Contracts (`306a429`, +379 lines of proto, ~1,800 regenerated):**
- `vehicle.proto` — `VehicleCapabilities` on `VehicleSnapshot`: raw `capability_flags`
  uint64 **and** a decomposed `repeated MavProtocolCapability`, so unknown bits are never
  silently dropped. Carries `flight_sw_version` (the join key into metadata) and
  `uid`/`uid2`, the durable per-airframe identity
- `types.proto` — `MavProtocolCapability` (22 values) and `MavModeProperty` mirrored
  value-for-value
- `parameters.proto` — `ParameterMetadata`, `ParameterMetadataSet`,
  `GetParameterMetadataRequest`. Metadata does **not** ride on `ParameterValue`; it is a
  separate set joined client-side. Sets are vendored per **minor line** and selected at
  runtime as nearest vendored ≤ actual, with `is_exact_match` forcing the UI to admit the
  gap. Granularity set by measurement, not taste: Copter 4.5.6→4.5.7 changes 1 parameter
  name, 4.5.7→4.6.0 changes 329
- `services.proto` — `GetParameterMetadata` at `OPERATOR_ROLE_OBSERVER`; an observer who
  cannot read units is reading raw floats
- `auth.proto` — `SettingsRecord`/`SettingsScope`/`SettingsOrigin`. Last-write-wins on an
  explicit `epoch_last_changed_ms` set by the *writer*, with an origin tiebreak
  (`VEHICLE > SERVER > CLIENT`). Scoped on **vehicle UID, not `VehicleId`**: sysid is
  operator-assignable and reused across airframes, so settings keyed on it follow the slot
  rather than the vehicle. **Contract only** — no service, no RPC, no Go interface

**Persistence boundary defined, adapters deferred (ADR-0007 §8).** Three ports rather than
one, because a single storage port yields a lowest-common-denominator interface no backend
implements well: state cache (lossy, rebuildable), event history (append-only, ordered),
durable record (transactional, survives restart). The discriminator is one question — is it
rebuildable from live MAVLink? This supersedes nothing: ADR-0002 and ADR-0003 said Redis is
not the system of record, and neither named what is. Each port lands in the tier that first
has a caller; nothing calls a durable record today, and an interface with no implementations
**and** no callers is speculative API design.

**Standing prohibitions recorded** so adding one is a decision argued against a written
position rather than a gap someone fills: no `eval` path for user-authored content (Cockpit
runs `new Function(code)()` and `eval(...)` unsandboxed, and its iframe bridge never
validates `event.origin`); and no environment sniffing — the backend declares capability and
the frontend reads the declaration, rather than Cockpit's `isElectron()` user-agent check
across 130 call sites.

**Capability matrix (`c536f16`)** gains a *Planned — Capability Negotiation* section:
`AUTOPILOT_VERSION` #148, `AVAILABLE_MODES` #435, `CURRENT_MODE` #436,
`MAV_CMD_REQUEST_MESSAGE` 512, all `blocked`. Those rows deliberately write the ID column as
`#148` / `cmd 512` so `TestMatrixCoverage`'s regex does not match them — they document
intent without claiming a decoder exists.

**Accepted costs, recorded rather than glossed:** discovery gains a round trip, so there is
a window after first HEARTBEAT where capabilities are unknown — a state the UI must render,
not one it can skip. A vehicle that never answers #148 stays unknown forever and integer
parameter decode stays refused for it. That is a real functional regression against "assume
C_CAST", and it is accepted: a refusal is debuggable and a silent misread is not.

### Added — Tier 4 bridge core (2026-08-17)

**Vehicle fold (`internal/vehicle`):**
- `state.go` — `State` accumulates one vehicle: proto snapshot, per-family freshness
  map, liveness timestamps, EKF flags, source address. Copied by value; the two
  reference fields are cloned by the fold, so a `State` handed in is never modified
- `fold.go` — `Fold(state, Inbound, nowMs) (State, []Event)`. Discovery, loss,
  recovery, heartbeat-change suppression, source-conflict detection, snapshot
  aggregation, telemetry pass-through. No goroutines, no Redis, no sockets
- `event.go` — `Event` carries exactly one of a `FleetEvent`, a `TelemetryEvent` or a
  `Warning`; `occurred_at` is stamped from the injected clock
- `registry.go` — `FleetRegistry` (SADD/SREM on `fleet:active`, implemented in Tier 5)
  plus a `NopFleetRegistry` whose zero value is usable
- 18 tests, `-race`, `goleak`. `TestNoWallClock` parses the package's own AST and
  fails on `time.Now` or `time.Since` — a grep would have failed on the prose
  explaining the rule

**Route table (`internal/routes`, 11 tests):** `Upsert` / `Lookup` / `Resolve` / `Evict` /
`ForgetLink`, `sync.RWMutex`, no background goroutine, lazy eviction on the key being
looked at. `Resolve` returns `ErrNoRoute` rather than a zero value, because the
alternative gomavlib offers is `WriteMessageAll`.

**Containers:** `docker-compose.yml` (SITL ×3 behind a `multi-sitl` profile, Redis
with AOF, backend, Vite dev server, nginx), multi-stage `Dockerfile` for the backend
and `docker/sitl/Dockerfile` for ArduPilot SITL pinned to `ArduCopter-4.6.0`,
`docker/nginx/nginx.conf` with streaming-safe proxy settings.

**Gates:** `make gate-tier-4` (fold + routes under `-race`, `scripts/check-tier-4.sh`,
`docker compose config`, backend image build, Redis actually answering `PING`), plus
`gate-tier-4-sitl` as the slow gate. CI gains a `containers` job and a `sitl-image`
job restricted to pushes and manual dispatch.

**Nine places the plan did not survive contact:**
- **`codec.DecodedMessage` does not exist and should not.** The codec decodes bytes
  and knows nothing about source addresses. `vehicle.Inbound` carries `codec.Decoded`
  plus the transport facts (`SrcAddr`, `MsgID`) and the separately-decoded
  `HeartbeatState`, at the layer that has all three
- **The fold could never declare a silent vehicle lost.** `Fold` only runs when a
  message arrives, so a vehicle that stops transmitting stops being evaluated. Added
  `Expire(state, nowMs)` for a sweeper, emitting `VEHICLE_LOST` at most once per
  outage. `Fold` also emits `VEHICLE_RECOVERED`, which the proto has and the plan's
  event table omitted
- **The two liveness timeouts were an exact tie.** gomavlib's `IdleTimeout` defaults
  to 60s and the plan's fold TTL is 60s, so "vehicle lost" and "channel closed"
  race. `codec.HeartbeatTTL` is now authoritative and `codec.LinkIdleTimeout` is
  derived as 3× it, set explicitly on the node
- **`VehicleState.BaseMode` has no source.** The codec expands `base_mode` into
  discrete bools by design; storing the raw byte would mean re-deriving it and
  keeping a second answer to "is it armed"
- **The route table stores a `codec.LinkID`, not a `*gomavlib.Channel`.** The codec
  already owns the channel and exposes `WriteTo(LinkID, msg)`. The distinction the
  plan cared about survives — the address is the link, never the IP — with one owner
  of the socket. Component ID is part of the key: a gimbal and an autopilot can be
  reachable on different links
- **`WarningEvent` is a Go type, not a proto.** Nothing streams it to a client yet,
  and adding a message commits the wire contract to `buf breaking` forever for a
  shape Tier 5 has not had to use
- **`--out=udp:gcs-backend:14550` is `sim_vehicle.py` syntax, not a flag the SITL
  binary has.** The image runs the autopilot directly, so the equivalent is
  `--serial0=udpclient:...`; waf emits `arducopter` lowercase; `SYSID_THISMAV` is set
  through a parameter overlay, which works on every release
- **The compose table published UDP 14550 on a SITL container.** SITL dials out and
  binds nothing there; the backend is the listener, so that is where the publish
  belongs. nginx moved behind a `tls` profile — it cannot start without certificates,
  and a service that fails on a fresh clone teaches everyone to ignore failures
- **The backend image restates the Go version**, which ADR-0006 says is how versions
  disagree. `scripts/check-tier-4.sh` fails if `Dockerfile`, `go.mod` and
  `MODULE.bazel` do not agree, and if the compose file and the SITL image disagree
  about the ArduPilot tag

**Transport threat model, now executable:** `codec.PostureWarning` names the bind
address and the missing signing key, and `cmd/gcs` prints it at startup. Verified in
the running container. `codec.ResolveBind` keeps unset (default `0.0.0.0:14550`)
distinct from explicitly empty (socket disabled) — `os.Getenv` alone collapses the
two and silently disables the bridge for everyone who never set the variable.

**Not verified locally:** the SITL image has not been built on this machine (10-20
minute cold clone and compile); it is wired as a CI job and remains unproven until
that job runs. Everything else in the gate was executed: the backend image builds,
Redis answers `PING`, and the backend container reports healthy on `/healthz` with
the posture warning in its log.


### Added — Tier 1 codec + Tier 2 parity apparatus (2026-08-17)

**Go codec (`internal/codec`) — Tier 1 chapters 1–3:**
- `frame.go` — `FrameSource` port over gomavlib. Non-deprecated struct config +
  `Initialize()` (SA1019 is on), `HeartbeatPeriod` 1s over gomavlib's own heartbeat
  rather than a hand-rolled ticker, `StreamRequestEnable` left false with the
  hardware-bringup caveat recorded. `WriteTo(LinkID, message.Message)` with a routing
  table built only from inbound frames; an unaddressable target returns
  `ErrUnknownLink` and is never broadcast
- `message.go` — dispatch tables, not a type switch. 13 telemetry families →
  `TelemetryEvent`, 5 transaction families → `ProtocolEvent`. All unit normalisation
  happens here, exactly once
- `encode.go` — the 11 priority send encoders. Pure (`message.Message`, never bytes),
  and none of them chooses a destination
- 44 tests, `-race`, `goleak` in `TestMain`

**Tier 2 evidentiary layer:**
- `scripts/gen_mavlink_fixtures.py` + 33 golden fixtures in `contracts/mavlink/`.
  Committed, deterministic (verified byte-identical across regeneration), generated
  from this repo rather than sourced from an unpinned sibling checkout. Each `.json`
  carries wire-unit `fields` plus an `expected_proto` block, so a fixture is evidence
  for a NORM-* row and not just a decode test
- `docs/wip/codec-capability-matrix.md` — 13 + 5 receive, 11 send, 6 normalisation,
  7 framing rows
- `TestMatrixCoverage` — asserts matrix rows equal
  `telemetryDecoders ∪ protocolDecoders ∪ SendFamilies`. Canaried both directions: a
  removed row with a live decoder fails, and a removed decoder with a live row fails

**Three defects found by execution, not by reading:**
- **`pump` could wedge the node permanently.** Events were forwarded with a bare send
  on an unbuffered channel, so a consumer that stopped reading blocked the goroutine —
  and `Close` waits on it, making shutdown impossible. Found via a 600s hang whose
  goroutine dump showed seven pumps blocked in `chan send`. The send is now guarded by
  a `select` on a `closing` channel and `TestCloseReturnsWithNoEventConsumer` pins it
- **The `bad_crc` fixture was not testing a bad CRC.** It corrupted byte 8, which is
  inside the v2 msgid field (the header is 10 bytes), producing a self-consistent frame
  with an *unknown message ID* that gomavlib surfaces as `MessageRaw`. Now corrupts
  index 10, the first payload byte
- **`FRAME-TRUNCATED` does not emit a parse error**, contrary to the Tier 2 plan.
  gomavlib's parser is stream-oriented: a frame cut mid-payload leaves it waiting for
  the remainder. The observable contract is only "no frame surfaces, no panic"

**Two contract corrections against the generated protos:**
- HEARTBEAT has no `TelemetryEvent` oneof variant — it decodes to `HeartbeatState` and
  drives the fleet fold. Its dispatch-table entry is nil so the ID is still claimed and
  still appears in the matrix
- `hdg_cdeg`, `cog_cdeg` and `vel_cm_s` deliberately keep wire units *and* wire-unit
  names. The 65535 unknown sentinel is defined in those units and a divide turns it
  into 655.35, which nothing downstream can recognise

**Frontend logic (`frontend/src/logic`) — Tier 1 chapters 4–5:**
- `sample.ts` shim + `attitude`, `heading`, `flightPath`, `position`, `trajectory`,
  `track`, `freshness` resolvers; 60 tests over named fixture tables
- Stall gate keys on flight regime, not vehicle type — a hovering VTOL keeps its
  predicted track
- `finite.ts` `isNum` type predicate removes the `as number` casts that
  `Number.isFinite` forces

**Build:**
- `go.mod` gains `gomavlib v3.3.5` and `goleak v1.3.0`
- `contracts/mavlink/BUILD.bazel` filegroup + `data` on the codec test with `# keep`.
  Bazel sandboxes tests, so the fixtures were absent on the first hermetic run while
  plain `go test` passed — a reminder that `bazel test //...` is the signal
- `bazel test //...` green over 3 targets, both languages

**Gate status:** `gate-tier-1`'s Go half previously passed while reporting
`[no test files]` — a gate checking nothing, in the ADR-0006 sense. It now runs 44
tests. `gate-tier-2` is fully wired.

### Changed — adversarial review of Tiers 0–3 (2026-08-16)

Findings and full reasoning: `docs/roadmap/tier-0-3-adversarial-review.md`.

**Contracts (Tier 0 decisions taken before codegen, where they are free):**
- `ProtocolEvent` added (`protocol.proto`) — PARAM_VALUE, MISSION_COUNT, MISSION_ITEM_INT,
  MISSION_ACK and COMMAND_ACK are transaction responses and no longer forced into
  `TelemetryEvent`. `MissionCount` message added; it did not exist
- Vehicle identity moved to the envelope only — `vehicle_id` removed from all 15
  telemetry payload messages (fields renumbered from 1; safe pre-codegen)
- `MissionService.UploadMission` converted from client-streaming to unary over
  `UploadMissionRequest` — connect-web cannot call client-streaming RPCs from a browser
- `StreamTelemetryRequest.payload_types` typed as `TelemetryPayloadType` (was
  `repeated string`); `OperatorContext.roles` typed as `OperatorRole`
- `options.proto` adds `required_role` method option; all 37 RPCs annotated. Absent
  option = deny, so a new unannotated RPC is unreachable rather than public
- `go_package` corrected to `yalb.gcs/internal/gen/gcs/v1;gcsv1`; project name settled on
  `yalb-gcs` across go.mod, MODULE.bazel, buf module and BUILD files
- `MavType` replaced with the complete current MAV_TYPE list (0–49). The roadmap's
  hand-copied fixed-wing table was pre-2019 and mapped ROCKET(9) and GROUND_ROVER(10)
  onto KITE and FLAPPING_WING
- `TrackAffiliation` gains an explicit `_UNSPECIFIED = 0`; "unknown affiliation" is a
  classification and must stay distinguishable from an unpopulated field
- Force-arm control stated on `CommandLong`: removing `SetArmedRequest.force` does not
  close force-arm, since `SendCommand` accepts cmd 400 with param2=21196 and a
  `raw_command` passthrough. Enforcement is the Tier 8 allowlist

**Gates that could not fail, now do:**
- `Makefile` with `gate-tier-0` … `gate-tier-3`; every exit gate is a command with an
  exit code rather than prose
- `.github/workflows/ci.yml` — first CI in the repo. `fetch-depth: 0`, and
  `buf breaking proto --against '.git#branch=origin/main,subdir=proto'` run from the
  repo root. Both parts verified: without `subdir=` imports do not resolve, and run
  from `proto/` buf looks for `proto/.git` and fails
- `scripts/check-matrix.sh` replaces the Tier 2 one-liner, whose trailing `|| true`
  made it print ERROR and exit 0
- `scripts/check-tier-0.sh` — five contract checks buf lint cannot express
- `.golangci.yml` migrated to the v2 schema; the previous file used the v1
  `linters-settings` key and was rejected outright, so none of its settings applied

**Bazel (ADR-0001's hermetic signal, now actually running):**
- `bazel build //...` green over 5 targets, including the generated proto library and
  Connect stubs; wired into CI with a BUILD-files-are-current check
- `rules_go` 0.52.0 → **0.62.0**, `gazelle` 0.40.0 → **0.52.2**. The pinned versions
  predated the host toolchain: rules_go 0.52.0 passes `GOEXPERIMENT=coverageredesign`,
  which Go 1.25 removed, so the stdlib build failed outright
- `go_sdk.host()` → `go_sdk.download(version = "1.25.0")`. A host SDK makes the build
  depend on ambient environment state, which is the thing ADR-0001 exists to avoid
- `# gazelle:proto disable_global` — gazelle was emitting `proto_library` +
  `go_proto_library` claiming the same importpath as the committed buf output (two
  generators for one package), with deps resolving to `//gcs/v1:*` labels that do not
  exist. buf is the only protobuf generator; Bazel compiles its output as ordinary Go
- `MODULE.bazel.lock` committed; `make bazel-tidy` runs go mod tidy → gazelle →
  bazel mod tidy in that order
- Generated stubs committed (`internal/gen`, `frontend/src/gen`) — Tier 3 executed
- **Frontend under Bazel** — `aspect_rules_js` 3.4.0 + `aspect_rules_ts` 3.10.0.
  `npm_translate_lock` reads `frontend/pnpm-lock.yaml`, so the lockfile stays the source
  of truth for JS versions the way go.mod is for Go. `ts_project` typechecks src
  including the generated Connect types; `vitest` runs as a Bazel test.
  `bazel test //...` is now one signal across both languages
- Bazel 7.4.1 → **8.7.0** — forced by `aspect_rules_js`'s
  `bazel_compatibility = [">=7.6.0"]`
- `tsBuildInfoFile` moved out of `node_modules/.tmp/` — Bazel materialises that tree
  from the lockfile and cannot also own outputs in it
- Opted out of `aspect_tools_telemetry` in `.bazelrc`; it arrived transitively with
  `aspect_rules_js` and announced it would begin collecting usage data
- Added `frontend/src/codegen.smoke.test.ts` — round-trips a generated `TelemetryEvent`
  through binary. Typechecking proves the generated code compiles; this proves it loads
  and works at runtime, and gives the vitest target something real to run
- **ADR-0006 — Toolchain Version Coupling and Dependency Risk.** Records the five-step
  forced-upgrade chain, the coupling table (rules_go ↔ Go SDK ↔ gazelle,
  aspect_rules_js ↔ Bazel, npm_translate_lock ↔ pnpm lockfile format, protobuf-es major
  ↔ buf plugin set), and the rule that a gate is not trusted until it has been made to
  fail on purpose — two gates here passed while checking nothing

**Build:**
- `proto/buf.gen.yaml` added — v2 `remote:` plugins, no `connectrpc/es` (protobuf-es v2
  emits service descriptors itself), `--template` required
- `buf.yaml` lint exceptions scoped per file rather than module-wide
- `MODULE.bazel` wires gazelle's `go_deps` from `go.mod`; root `BUILD.bazel` adds a
  gazelle target
- vitest, `@bufbuild/protobuf`, `@connectrpc/connect{,-web}` added to the frontend;
  `@/` alias configured in both `vite.config.ts` and `tsconfig.app.json`
- 7.9 MB compiled `gcs` binary untracked and gitignored
- `docs/runbooks/dev-setup.md` — toolchain install, gate commands, and the gotchas

**Docs:**
- Build sequence changed to **0 → 3 → (1 ∥ 2)** — codegen depends on nothing in Tiers 1–2,
  and running it late means hand-maintaining a proto type stub for two tiers
- Tier 0, 1 and 3 rewritten; Tier 2 and 4 substantially revised. Every gomavlib symbol
  verified against v3.3.5: `EndpointCustomConn` does not exist, `NodeConf`/`NewNode` are
  deprecated, sequence is `GetSequenceNumber()`, and there is no `WriteMessage`
- Heartbeat decision corrected — gomavlib already sends one (5s, MAV_TYPE_GCS) unless
  disabled; the planned hand-rolled per-vehicle ticker would have double-emitted
- Stall gate rekeyed on flight regime rather than vehicle type, so a hovering VTOL keeps
  its predicted track
- Unit normalisation fixed to happen exactly once, at the proto boundary; the planned
  `TelemetrySample` used raw wire units against already-normalised protos
- `port-plan.md` scoped to algorithms and file mapping; superseded tooling statements
  marked inline and all five Open Questions closed

### Added
- Go module (`yalb.gcs`) with `cmd/gcs` entrypoint — minimal HTTP server on `:8080` with `/healthz`
- React frontend scaffold — Vite 6, React 19, TypeScript 5.7, TanStack Query 5 wired at root
- `pnpm` as package manager (pinned `10.30.1`); all frontend deps pinned to exact versions with 1-week stability buffer policy
- TypeScript strict compiler suite: `verbatimModuleSyntax`, `noImplicitReturns`, `noImplicitOverride`, `noUncheckedIndexedAccess`, `noPropertyAccessFromIndexSignature`, `exactOptionalPropertyTypes`, `allowUnreachableCode: false`
- `typescript-eslint` `strictTypeChecked` + `stylisticTypeChecked` — type-aware lint via Node 25 native TS execution
- `@types/node` scoped to tooling tsconfig only; browser bundle has no Node globals
- Go strict lint via `golangci-lint v2` — `errorlint`, `wrapcheck`, `exhaustive`, `govet --enable-all`, `gocritic`, `cyclop ≤15`, `funlen ≤80`
- Bazel `MODULE.bazel` wired with `rules_go 0.52.0`, `gazelle 0.40.0`, `go_sdk.host()`
- Protobuf schema (`proto/gcs/v1`) — fleet, vehicle, telemetry, commands, missions, parameters, track, mesh, video, chat, auth, security, calibration
- ADRs 0001–0005: Bazel build system, Go/React/Connect/Docker stack, frontend adapter pattern, multi-protocol track layer, security and auth model
- QGC/Mission Planner research doc

---

## What's next

Infrastructure still needed before feature work starts:

1. **Docker compose** — `ardupilot-sitl`, `redis`, `gcs-backend`, `gcs-frontend` services; hermetic SITL from day one per ADR-0002
2. **Connect (Buf) codegen** — `rules_buf` in Bazel, `buf.gen.yaml`, Go server stubs and TypeScript client from the proto schema
3. **Go hexagonal skeleton** — typed channel boundaries per the pipeline (`Transport → Frame codec → Message router → Service protocols → Vehicle model → Outbound adapters`); no implementation, just the interfaces and empty adapters
4. **WebSocket SITL adapter** — backend `:8081` endpoint for the test harness (ADR-0003)
5. **Redis adapter stub** — pub/sub and last-known-state scaffolding
6. **Frontend adapter context** — `ConnectAdapter` / `WebSocketAdapter` / `MockAdapter` provider shell; no component logic yet (ADR-0003)
7. ~~**CI pipeline**~~ — done: `.github/workflows/ci.yml` runs buf lint, the Tier 0
   contract gate, buf breaking on PRs, Go build/vet/test/lint, and frontend
   typecheck/test. `bazel test //...` becomes the canonical signal once BUILD files are
   generated (`make bazel-tidy`)
