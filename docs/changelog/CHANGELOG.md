# Changelog

All notable changes follow [Keep a Changelog](https://keepachangelog.com) conventions.
Format: `## [version] - YYYY-MM-DD`. Unreleased changes accumulate at the top.

---

## [Unreleased]

### Fixed — Documentation ordering sweep; ADR-0010 closes the stream-request defect (2026-08-19)

Closes the open decision left by **"Added — Tier 5 live bridge, first SITL contact"** below,
and five further ordering defects found by sweeping every file under `docs/` for the same
shape: an exit criterion that depends on a capability scheduled later, or on one nothing
schedules at all.

**The premise that held the defect in place was false, in three places.**
`tier-1-pure-domain-logic.md`, `tier-0-3-adversarial-review.md` (finding N4) and the comment
on `StreamRequestEnable` at `internal/codec/frame.go` all asserted that SITL streams
telemetry unprompted. The reasoning built on it was that Tiers 5–6 would look healthy and
only first hardware bring-up would be surprised, which is what made deferring the fix to
Tier 8a survivable. The six-minute run falsifies it: SITL streams nothing either. Per
ADR-0008 §3 the run outranks the prose; all three are corrected. This also inverts the
objection to `SR0_*` in the SITL overlay — with neither SITL nor a field vehicle streaming
unprompted, an overlay would now *create* the divergence it was meant to avoid.

**Principle 3 was worded wrong, not crossed — ADR-0010.** "Read-only proven before any
outbound capability" described neither the plan nor the code. Tier 5 has emitted a GCS
heartbeat since it opened a socket; Tier 7 sends five of the eleven send families; and
ADR-0007 already schedules an outbound `COMMAND_LONG` cmd 512 at discovery, which
`codec-capability-matrix.md` assigns to **Tier 5** — uncontested, with no carve-out asked
for. The repo had been operating on an unwritten boundary: a frame that asks a vehicle to
send data is read-side. ADR-0010 writes it down, and Principle 3 is restated in terms of
vehicle-state mutation. The dual-gate safety model is untouched and still governs the write
column only.

**Tier 8a was misfiled, not merely late.** It sits in Tier 8 because it rides `COMMAND_LONG`,
and everything on `COMMAND_LONG` had been filed under writes — but Tier 8's own text says of
it "Reversible. Affects only telemetry rate. No vehicle state change." The operator-facing
RPC, registry, ACK correlation and audit stay at 8a. First telemetry does not, because it is
not an operator action.

**The larger finding: the cmd-512 round trip that ADR-0007 mandates was scheduled in no tier
at all.** ADR-0007 §1 requires `AUTOPILOT_VERSION` (#148) "requested at discovery" and its
Consequences accept the round-trip cost explicitly; ADR-0009 §2 keys its entire cache
predicate on the `uid` and `flight_sw_version` that only #148 carries; Tier 7 fails
`GetParameterMetadata` outright when capabilities are unknown. Nothing sent the request. The
practical consequence was that the only Tier 7 exit line reachable was the degraded one. This
is why the resolution is one chapter rather than a reordering — **a Tier 5 outbound-request
path had to be built regardless**, and once it exists cmd 511 is one more command through an
encoder (`SEND-CMD-LONG`, #76) that is already `complete`.

**New: `tier-5-transport-live.md` Chapter 7, the discovery request round trip.** Triggered by
`VEHICLE_DISCOVERED` *and* `VEHICLE_RECOVERED` — keying on discovery alone repeats the
`sync.Once` mistake the router loop was built to avoid. Sends cmd 512 for #148 and #435, then
cmd 511 for the **nine periodic** telemetry families. Three of the twelve are deliberately
excluded and the reasons are per-message, not a blanket rule: `RADIO_STATUS` (109) is injected
by the SiK radio rather than produced by the autopilot, so there is nothing to rate and it
never appears over SITL UDP at all — no gate may require it there; `STATUSTEXT` (253) and
`HOME_POSITION` (242) are event-driven. Fire-and-observe: no registry, no ACK correlation, no
audit. Re-sent on a timer, because a lost UDP request otherwise leaves a healthy vehicle
silent forever, which is indistinguishable from the bug being fixed.

**`StreamRequestEnable` stays false, on new reasoning.** The old justification is void. It is
declined now because `REQUEST_DATA_STREAM` (#66) is deprecated *and a distinct message
family* — adopting it would move `TestMatrixFamilyCounts`' `len(SendFamilies) == 11` pin to
admit a deprecated message permanently — because it gives no per-message control, so the
nine/three split becomes inexpressible, and because its confirmation event
`EventStreamRequested` is one `internal/bridge/bridge.go` already ignores, making it the only
outbound path with no observable trace. For the record, it does work: gomavlib substitutes
4 Hz when `StreamRequestFrequency` is unset.

**Tier 5's exit gate was asserting something Tier 5 cannot prove.** The heartbeat validation
procedure opened with `HeartbeatDisable: true` → send `MAV_CMD_COMPONENT_ARM_DISARM`. That is
Tier 8e, and it is the one gate in the roadmap Principle 3 *genuinely* blocks — arming changes
vehicle state, unlike the Chapter 7 requests. Split: cardinality and rate stay at Tier 5, the
failsafe assertion moves to 8e where the command already exists.

**Contradictions found against executable artifacts, per ADR-0008 §3.**

- `tier-8-write-transactions.md` described `MissionService.UploadMission` as a
  client-streaming Connect RPC. `services.proto` has been unary since Tier 0, and
  `scripts/check-tier-0.sh` enforces it. The chapter was never updated; the proto wins.
- `tier-4-bridge-core.md` still offered `ARG ARDUPILOT_TAG=ArduCopter-4.6.0` as the pinning
  example — verbatim the string `order-of-operations.md` records as having broken every image
  build for three months. Corrected to `Copter-4.7.0`, with the prefix trap named inline.
- `port-plan.md` carried "Copter 4.6.x, Plane 4.5.x", the `ardupilot-dev-jammy` base image
  ADR-0004-era work already rejected, and `internal/transport/udp.go`, which Tier 5 as built
  established does not exist.
- `port-plan.md`'s phases still ran Command Surfaces before the two read phases. Its own Open
  Questions list records that inversion as resolved to "params (read) before commands
  (write)" — but only the list was struck; the headings and the end-to-end verification ladder
  were never annotated. The ladder also named `ARMING_CHECK` as the first parameter write,
  where Tier 8b names `LOG_BITMASK` precisely because it is low-consequence.

**The process failure, which is the part worth keeping.** This was found on 2026-08-17 by
reading, as finding N4 of the Tier 0–3 adversarial review, which closed with "Decide now and
put the decision in Tier 5's gate, so first hardware bring-up is not a surprise." Nothing was
decided, nothing reached a gate, and no mechanism existed that would notice — the same shape
ADR-0009 §6 describes, an unowned question with no gate. It was eventually caught by running
the system, which is the most expensive route available. N4's severity is corrected from
medium to blocker and its tier list from "5, 8" to "5, 6, 7, 8".

**Recorded as open rather than fixed**, each in the file that owns it: `MISSION_REQUEST_LIST`
(#43) has no encoder and is in no send set, though Tier 7's mission chapter opens with it, and
adding it moves the `len(SendFamilies) == 11` pin; and the TypeScript partial-sample
accumulator is assigned to "the Tier 4 per-vehicle fold", which is Go-only — Tier 6's
TelemetryLog depends on code no tier schedules.

**One claim checked and dismissed**, so it is not re-raised: Tier 7's seven `PARAM-*` matrix
rows do not conflict with `TestMatrixCoverage`. Its regex matches only rows prefixed `RECV-`
or `SEND-` carrying a bare numeric ID.

### Added — Tier 5 live bridge, first SITL contact (2026-08-19)

The receive path now runs end to end against real firmware. `internal/bridge` joins the
codec's frame stream to the vehicle fold and forwards what the fold emits to a `Sink`;
`cmd/gcs` opens the socket, assembles it and coordinates shutdown. First light: a real
`Copter-4.7.0` SITL was discovered over UDP as
`VEHICLE_DISCOVERED sysid=1 compid=1 mav_type=MAV_TYPE_QUADROTOR armed=false`.

**Chapter 1 of the tier plan was unbuildable and is collapsed.** It specified
`internal/transport/udp.go` wrapping a raw `net.PacketConn` behind typed
`InboundPacket`/`OutboundPacket` channels. There is no seam for it: Tier 4 as built made
`codec.Node` wrap gomavlib, and gomavlib owns its socket through an `EndpointConf` — it
takes an endpoint, not a byte stream. Sliding a `PacketConn` underneath would mean
reimplementing framing, and the one destination-free alternative, `EndpointCustom`, is a
*single* stream and so cannot represent N UDP peers on one bound port — it would collapse
all three SITL instances into one channel and destroy the per-link routing Tier 4 built.
`codec.FrameSource` is the transport port that chapter was reaching for, and it already
existed. `cmd/gcs` now constructs `gomavlib.EndpointUDPServer` from `codec.ResolveBind`
and hands it to `codec.NewNode`. No new package.

**Chapter 2's per-vehicle goroutines are replaced by a single router loop.** The plan
called for one goroutine per discovered vehicle, spawned on first HEARTBEAT under a
`sync.Once` guard and cancelled on `VEHICLE_LOST`. Three reasons not to, all from reading
Tier 4 as built: the fold is pure and cheap, so a single loop over
`map[routes.Key]vehicle.State` has identical semantics with no lifetime problem and no
leak surface; `VEHICLE_RECOVERED` did not exist when the plan was written, and a vehicle
that is lost and returns needs its goroutine back — which is precisely what a `sync.Once`
keyed on system ID prevents; and fold order across vehicles becomes nondeterministic the
moment the folds run concurrently, discarding the replayability the fold was built for.
`states` is single-owner and needs no mutex. The route table keeps its own, because the
Tier 8 send path reads it from elsewhere.

**A `Sink` interface, so the loop could be finished before Redis exists.** The plan
assembled the pipeline straight into a Redis publisher, which makes the first runnable
bridge depend on a running Redis and makes every failure ambiguous between "the fold is
wrong" and "the publisher is wrong". `LogSink` is the first-light implementation; the
Redis sink lands with Chapters 4–5. A sink that refuses an event stops `Run` rather than
being logged and continued: the sink is what makes an observation durable, and a bridge
that keeps folding while nothing records the result presents as healthy while silently
losing the fleet history an operator will later be asked to trust.

**Frames carrying our own (255, 190) identity are dropped before the fold.** gomavlib does
not loop our writes back, so in a healthy Compose topology this never fires — but a
misconfigured UDP route, a mavproxy relay or a second GCS on the network all deliver them,
and folding one creates a phantom "vehicle 255" in `fleet:active` that no downstream
filter can undo. Tested against `contracts/mavlink/heartbeat_gcs_out`, which is that exact
frame.

**A sweep tick, because the fold's own TTL check could never fire.** `Fold` runs only when
a frame arrives, so a vehicle that stops transmitting produces no more folds and would
never be declared lost. `vehicle.Expire` had already been added in Tier 4 for exactly this;
Tier 5 is what finally calls it, on a 1s ticker against the injected clock.

**Source attribution is the gomavlib channel label, not a parsed IP.** `EventFrame` carries
no peer address. For a UDP server endpoint it does not need to: gomavlib opens one channel
per remote peer and labels it `udp:<host>:<port>`, so the label already identifies the
source at exactly the granularity the conflict check wants — and it stays meaningful on a
serial link, where there is no IP at all. `splitLabel` recovers host/port for diagnostics
only and degrades to zero values on anything it cannot parse, because `routes.Upsert`
treats an empty `SrcIP` as "carry forward what you had" and a half-parsed one as truth.

**The SITL image was built and run for the first time.** Tier 4 shipped it wired as a
CI job and explicitly unproven. It builds: `Copter-4.7.0`, 120 MB runtime image against
9.3 GB for the predecessor project's single-stage equivalent — the multi-stage split is
carrying its weight. It also runs, and the backend talks to it.

**Finding, from running it: a live ArduPilot link delivers no telemetry at all.** Over six
minutes of continuous contact the backend logged one `VEHICLE_DISCOVERED`, zero
`VEHICLE_LOST` (so heartbeats never stopped) and **zero telemetry events**. The pinned
`Tools/autotest/default_params/copter.parm` sets no `SR0_*`/`SR1_*` stream rates, and
`codec.NewNode` leaves `StreamRequestEnable` false. This is the failure the comment on
that field predicted verbatim — and it was left false precisely so this would surface
rather than be masked. **It is a roadmap ordering defect, not a code defect:** Tier 6's
exit gate is "browser shows live telemetry log", Tier 7's is "ARMING_CHECK readable from
live ArduCopter", and the stream-request path that makes either possible is currently
scheduled at Tier 8a — two tiers later. Resolution is an open decision; see the tier 5
file.

### Added — Mixed-vehicle SITL, doc lifecycle, parameter paradigms (2026-08-19)

Five threads, all traceable to one root cause: questions that had no owner and no gate got
reopened at every review, which made settled decisions read as churn.

**ADR-0008 — Documentation Lifecycle and Amendments (Accepted).** There was no convention
for changing an Accepted ADR; one was improvised on 2026-08-18. Now: seven document
classes each with one lifecycle, split on **frozen versus living** — research is frozen at
write time (editing it destroys the ability to ask what we knew when we decided), tier
files are living with the changelog as their history. An Accepted ADR's Context and
Decision are immutable; new facts go in a dated amendment stating *what was wrong, what
did not change, and the consequence* — the middle one is mandatory, because an amendment
that omits it reads as a reversal. **Amendment budget is three**; a fourth means supersede
instead. §3 resolves doc-versus-code conflicts: the authority order governs *intent*, but
where a document and a test-enforced artifact disagree about what the system **does**, the
artifact wins and the document is a defect.

**The HEARTBEAT divergence, which was the worked example.** `order-of-operations.md` listed
HEARTBEAT among "13 streaming families → `TelemetryEvent`". `TelemetryEvent` has no
heartbeat variant, the capability matrix says so explicitly, and the codec's dispatch entry
is nil. The implementation found the truth in Tier 1 and the canonical document was never
corrected, so a reader consulting both found a contradiction with no stated reason and
concluded the decision was still moving. It was settled once and written down once. Now
corrected to the real three-way split — **12 → `TelemetryEvent`, 5 → `ProtocolEvent`, 1
(HEARTBEAT) → `HeartbeatState`** — with the divergence reasoned inline: `flight-path-hud`
was read-only MAVLink, where HEARTBEAT is one more message to render; here it carries fleet
identity and drives the vehicle fold, a different consumer with a different lifetime.
`order-of-operations.md` now also states its own provenance, so inherited defaults are
distinguishable from decisions.

**ADR-0009 — Parameter Acquisition Paradigms (Accepted).** The parameter *model* never
moved — `parameters.proto` is unchanged since `306a429`. What reopened every review was
where the bytes come from, for two structural reasons. Parameter metadata is the only
material in the repo that cannot obey our own acquisition doctrine (`gen_mavlink_fixtures.py`:
*"a fixture regenerable from a script in this repo is the only kind that satisfies
ADR-0001's hermeticity argument"*) — it is authored upstream and unproducible by us at any
effort, and a permanent exception to a strict rule attracts re-litigation. And nothing
owned it: Tier 7 consumes metadata, no tier produced it.

The ADR separates three questions that were being conflated — metadata (build-time
vendored, **never** on a live path), values (context-dependent), writes (bench only) — and
splits acquisition by connection context. Bench/cold connect may block: full
`PARAM_REQUEST_LIST`, resolve metadata, populate cache. Field reconnect must not: serve
cached values, no network, no re-pull of ~1400 parameters. **The discriminator is a
predicate, not a mode** — *do we hold a complete set for this vehicle UID at this
`flight_sw_version`?* Both facts are already in the contract, and a firmware change
invalidates the cache for free. `MAV_PROTOCOL_CAPABILITY_FTP` (32) is recorded as the
reconnect fast path and deliberately left unbuilt — it has no caller. The converter lands
in Tier 7, and **until Tier 7 begins the source question is closed, not open**; reopening
requires new evidence, not a new preference.

**Two SITL vehicles, two firmware lines.** The topology ran three containers that were all
the same copter, differing only by SYSID — multi-*instance*, never multi-*type*, so nothing
exercised the firmware-variance machinery ADR-0007 had just built. There was no plane
anywhere. Now `Copter-4.7.0` (sysid 1) and `Plane-4.6.3` (sysid 2), with a second Copter at
sysid 3 retained because it proves a different property: same type and firmware, distinct
system ID over one socket, i.e. that routing dispatches on sysid rather than source
address.

The two firmware lines are coverage, not untidiness: Copter 4.7 is the first line
implementing `AVAILABLE_MODES` (#435) and exercises ADR-0007 mode layer (a); Plane 4.6 has
no such message and exercises layer (b). One line for both leaves a layer permanently
unexercised and makes metadata version selection a trivially exact hit, so `is_exact_match`
never gets tested against anything but a perfect match. It also bounds the vendoring scope:
**two metadata sets, ~4 MB**, against ADR-0007's original ~23 pairs and ~46 MB — the sizing
predated the vehicle-set decision, and answering the scope question made the source
question cheap enough to defer honestly.

**Three per-vehicle facts, not one interpolated variable.** The image build takes
`ARDUPILOT_TAG`, `WAF_TARGET` and `DEFAULT_PARAMS_PATH` separately, because Plane's stock
parameters live at `Tools/autotest/models/plane.parm` while Copter's are at
`Tools/autotest/default_params/copter.parm` — and `default_params/plane.parm` does not
exist at any tag. Only the binary name is uniform (`ardu<target>`). Artefacts are
normalised to fixed names inside the image, so `entrypoint.sh` has no vehicle branching at
all: which vehicle an image holds is a build-time fact, never a runtime one.

**A gate that could not fail, now canaried.** `scripts/check-tier-4.sh` asserted that the
compose pin and the Dockerfile pin *agreed*. They did — both said `ArduCopter-4.6.0`, which
does not exist upstream, so the gate passed while every image build failed. Agreement
between two copies of the same wrong value is not evidence. The check is now against the
upstream naming rule (`Copter-*`/`Plane-*`; the `Ardu` prefix is the *binary* name), plus a
supported-set assertion, plus an opt-in `git ls-remote` resolution that CI enables.
Canaried both directions: reintroducing the old tag exits 1.

**CI: two config bugs fixed, the slow job made explicitly advisory.** CI had never passed —
every run since inception failed identically, three jobs, three unrelated causes, none a
code defect.
- **golangci-lint** — `golangci-lint-action@v6` installs linter v1.x, which cannot parse
  our `version: "2"` config *and* is built with go1.24, which refuses a module targeting
  `go 1.25.0`. Now `@v9` with `v2.12.2` pinned. `version: latest` was the other half: an
  unpinned input in a repo whose entire toolchain posture is pinning, which is why both
  breaks arrived together and neither was attributable.
- **pnpm** — `defaults.run.working-directory` applies to `run:` steps only, so
  `pnpm/action-setup@v4` resolved from the repo root and looked for a `package.json` that
  is not there. Now points at `frontend/package.json`. The frontend job had never reached
  typecheck or tests.
- **sitl-image** — `continue-on-error: true` with the promotion trigger named in a comment:
  it becomes required when Tier 5 has a caller. A permanently red job with no stated reason
  is how a CI suite stops being read. It now builds both vehicle images.

**The capability matrix moved to `docs/reference/`.** ADR-0008 §1 marks `docs/wip/` as the
one class with an expiry — temporary, must graduate or be deleted — while §3 makes the
capability matrix authoritative over prose about what the codec does. It was sitting in
`docs/wip/`. The repo's most enforced statement of fact lived in the directory reserved for
things not yet decided: the location said "provisional" while the test said "binding."

Repointed in four places — `scripts/check-matrix.sh`, `internal/codec/matrix_test.go`,
`internal/codec/BUILD.bazel`'s `data` dep, and the `exports_files` package, which moved
with it. Verified under Bazel rather than `go test` alone, because the failure this could
cause is a sandboxed runfiles miss that plain `go test` cannot see — the same trap recorded
when the MAVLink fixtures were added. `bazel test //...` is 5/5 and `bazel run //:gazelle`
leaves no diff.

Two references were deliberately **not** updated. The changelog entry for 2026-08-17 names
the old path and stays as written — ADR-0008 §5 makes shipped entries append-only, and
rewriting one to match today's layout would falsify the record. Likewise
`tier-0-3-adversarial-review.md` quotes the old Makefile line while describing a defect it
found; editing the quote would make the finding incoherent.

**ADR-0006 amended — two coupling links it was missing:** `golangci-lint` binary ↔ Go SDK
version, and `golangci-lint-action` major ↔ `.golangci.yml` schema version. Its thesis
stands; this is its own predicted failure class arriving at links it had not enumerated.
The uncomfortable part is that the workflow pinned `version: latest` — the ADR contradicted
in the repository that adopted it.

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
