# Cockpit Reference Analysis — Reconciliation

**Status:** Closed out. This document supersedes `docs/wip/cockpit-reference-analysis.md`,
which is deleted. Nothing here is authoritative. Order of authority remains: ADRs →
`order-of-operations.md` → tier files → this document.

**Date:** 2026-08-18
**Subject:** [bluerobotics/cockpit](https://github.com/bluerobotics/cockpit)
**Companions:** `cockpit-blueos-analysis.md`, `cockpit-blueos-analysis-ii.md`,
`qgc-missionplanner-analysis.md`, `../adr/0007-firmware-variance-via-capability-negotiation.md`

---

## Why this document exists

Cockpit was surveyed **twice, independently**, and the two passes did not overlap cleanly.

- **Pass A** — `docs/wip/cockpit-reference-analysis.md`, written 2026-08-17 against the
  tree at `27a9ab5`, on a branch that was never merged. Scope was eight areas requested in
  order of interest: canonical MAVLink reference, ArduPilot vehicle/parameter handling,
  WebRTC, framework/desktop targets, Tailwind, datalake, joystick/manual control, mission
  planning.
- **Pass B** — commit `5617567`, producing the two `cockpit-blueos-analysis*.md` documents
  and **ADR-0007 (Accepted)**, then `306a429` "wip - parameters pivot" which landed the
  contracts, then `c536f16`.

Pass B went further on everything the two share, and shipped proto. Pass A covered four
areas Pass B never touched. This document records which is which so neither gets
re-derived, and lists what is still open.

**Summary:** of Pass A's findings, roughly a third are superseded, a third were reached
independently by Pass B and landed better-specified than Pass A wrote them, and a third
are untouched by Pass B and still true — including one unmeetable exit gate, one latent
telemetry defect, and one capability area absent from the repo with no deferral entry.

---

## Part 1 — Closed by Pass B

Recorded so they are not re-proposed. These are not disputes; Pass B went further.

| Topic | Pass A said | What landed | Where |
|---|---|---|---|
| Parameter metadata model | "should exist; model on QGC `factmetadata.schema.json`. **Open question:** does it ride on `ParameterValue` or join client-side?" | `ParameterMetadata` + `ParameterMetadataSet` + `GetParameterMetadata` (unary, `OBSERVER`). Separate message, joined client-side by `param_id`. Open question answered | `parameters.proto`, `services.proto` |
| Metadata versioning | not raised | Vendored per **minor line**, selected at runtime from `flight_sw_version` (nearest ≤ actual), `is_exact_match` forcing the UI to admit staleness. Justified by measurement: 4.5.6→4.5.7 changes 1 parameter name, 4.5.7→4.6.0 changes **329** | ADR-0007 §5 |
| Metadata field set | units, range, increment, enum, bitmask, reboot-required, group | All of the above except `group`, **plus** `read_only`, `volatile_value`, `calibration`, `user_level`, `human_name`, `documentation`, and `unit_text` alongside `units` | `parameters.proto` |
| `flight_mode_name` owner | derive the table from `FLTMODE1.Values` | **Rejected in favour of something better.** ADR-0007 §2: (a) `AVAILABLE_MODES` #435 → (b) generated gomavlib dialect enum `String()` → (c) empty. No table, no artifact, no parsing | ADR-0007 §2, `tier-1-pure-domain-logic.md` ch.6a |
| Firmware capability bitmask | a design note, "from QGC rather than Cockpit" | `VehicleCapabilities` first-class on `VehicleSnapshot`; `MavProtocolCapability` and `MavModeProperty` mirrored into `types.proto` | `vehicle.proto`, `types.proto` |
| Parameter integer encoding | **missed entirely** | `PARAM_ENCODE_BYTEWISE` (16) vs `PARAM_ENCODE_C_CAST` (131072). Guessing C_CAST yields finite, plausible, wrong integers with no error raised. Tier 8b's first write target `LOG_BITMASK` runs straight through it. Unknown is now an **error** | ADR-0007 §4, `tier-1` ch.6b |
| Durable telemetry history | Pass A open question #4, "absent by omission rather than decision" | Three ports — state cache / event history / durable record — discriminated by "is it rebuildable from live MAVLink?". Adapters deferred with named triggers | ADR-0007 §8 |
| Settings / config model | not covered | `SettingsRecord`/`SettingsScope`/`SettingsOrigin`: LWW on explicit `epoch_last_changed_ms` with an origin tiebreak, scoped on **vehicle UID, not sysid** (sysid is operator-assignable and follows the slot, not the airframe). Contract only — no service, no RPC | `auth.proto` |
| `eval` / XSS surface | not covered | Standing prohibition. Cockpit runs `new Function(code)()` and `eval(...)` unsandboxed, and its iframe bridge never validates `event.origin` | ADR-0007 §7 |
| Late binding / runtime schema | "three-representation insight; `protoreflect` is unplanned" | Same conclusion — "schema reflection over protos we already generate" — while rejecting the data lake that would have imported singletons and string keys along with it | `cockpit-blueos-analysis.md` |
| Tailwind | "an unclaimed slot, no ADR conflict" | Endorsed, plus the sharper half: **take Tailwind, skip the component library.** Overlapping style systems are a cost, not a feature | `cockpit-blueos-analysis.md` |

**Why `FLTMODE1` lost, since it was Pass A's most-tested recommendation.** Pass A verified
that `Copter-4.7` `FLTMODE1.Values` holds 26 current entries, and flagged its own caveat:
that list enumerates modes assignable to a *flight-mode switch position*, which is not by
definition every value `HEARTBEAT.custom_mode` can report. ADR-0007's layer (b) has no such
boundary — it reads the generated dialect enum directly — and needs no artifact vendored,
no JSON parsed, and turns a dialect bump into a compile error instead of a silent
mis-mapping. Pass A's caveat is exactly the failure its own recommendation would have had.

Pass A's six self-corrections in its adversarial-review section all hold up on re-check.
The battery-units retraction in particular was correct: `telemetry.proto` uses mV/cA/cdeg/mAh
systematically, and each field carries a wire sentinel an SI float cannot express distinctly.

---

## Part 2 — Still open

Verified against `main` at `c536f16`, not recalled. Ordered by cost of leaving them.

### A. Manual control / joystick — absent, now recorded as deferred

Repo-wide grep for `manual_control|joystick|gamepad|rc_channel|rc_override` returns **one**
hit, an incidental remark about Cockpit's own code
(`cockpit-blueos-analysis-ii.md:213`). Pass B does not cover the topic. Not in the
18-family receive set, not in the 11-family send set, not in any proto.

It was also not in the out-of-scope table, so it read as oversight rather than decision.
**Fixed:** `order-of-operations.md` now carries a deferral row with the constraints below,
so the eventual ADR inherits them rather than rediscovering them.

Pass A's material, preserved because it is the only treatment that exists:

- **Two-stage mapping.** device→standard (per model) then standard→action (per profile,
  with a shift layer). Survives hardware swaps. Calibration as pure functions —
  `applyDeadband` **rescales** rather than clips, so usable range stays [0,1]. Versioned
  settings keys (`-v1`/`-v2`, read old as fallback) rather than migrations.
- **Route axes through a variable layer, not straight to MAVLink.** Cockpit's pipeline is
  gamepad → std mapping → calibration → variable → scale → `outputs/mavlink/axis-*` →
  `MANUAL_CONTROL`. Any producer (script, HTTP action, extension) drives the vehicle
  through the same seam. Maps onto the agentic inbound adapter in ADR-0002.
- **It is a third write class.** Unacknowledged and continuous, so incompatible with the
  command registry, the ACK correlation and the per-action audit Tier 8 is built on. Needs
  its own path: rate limiting, a deadman/heartbeat requirement, explicit take-control
  handoff, sampled audit, its own eligibility gate, and bandwidth arithmetic against the
  `RADIO_STATUS.txbuf` back-pressure signal the heartbeat-cardinality decision protects.
- **The one real tension.** The browser Gamepad API only reads while the tab is focused,
  which is why Cockpit ships an SDL fork in Electron and lists "joystick works
  unfocused/backgrounded" as desktop-only. This is the strongest Electron reopen-trigger in
  either pass and should not be papered over. `docs/wip/maybe-esc-configurator.md` set the
  precedent for browser-direct peripheral access (Web Serial, no Go backend), so Gamepad
  API / WebHID is consistent with existing thinking. The alternative if it bites: read the
  joystick in the Go backend over native HID with the browser as display only — which
  inverts Cockpit's architecture and fits ours better anyway.

**Hook Pass B handed this.** `MAV_PROTOCOL_CAPABILITY_COMPONENT_ACCEPTS_GCS_CONTROL`
(524288) landed in `types.proto` with **no consumer**. Under ADR-0007's model that is
precisely the bit this path would be gated on.

**Rejected outright**, and worth keeping rejected: Cockpit's `BTNn_FUNCTION` auto-remapper.
It is ArduSub-specific, rewrites vehicle parameters on a 1 Hz timer, and falls back to
silently un-mapping the operator's binding.

### B. Mission `.plan` interchange — gate was unmeetable

`tier-7-read-transactions.md` requires, in its exit gate:

> Round-trip: download → JSON → field-by-field comparison with QGC `.plan` format passes

Nothing in the repo defined that mapping — grep for `fileType|SimpleItem|ComplexItem|QGC WPL`
returned zero. Pass B never mentions `.plan`; `MissionPlanningView.vue` appears only as a
line-count statistic. Note this is a *written* gate, not yet a `make` target (only
`gate-tier-0`…`gate-tier-4` are mechanized), so nothing was red in CI — but it could not
pass as written.

**Fixed:** the mapping now exists at `../reference/mission-interchange-formats.md`.
`ComplexItem` remains unresolved and is listed below.

### C. `Waypoint { position, repeated Command }` — the window closed

`missions.proto` is still a flat 1:1 `MISSION_ITEM_INT` mirror. Pass A called the hierarchy
"Cockpit's one genuinely better-than-QGC idea": MAVLink attaches `DO_*` items *positionally*
to the preceding `NAV_*`, so making the hierarchy explicit means deleting a marker takes its
commands with it. Pass B never evaluated it.

**The cost changed.** Tier 0's premise is "free now, breaking after codegen", and codegen
ran at `25afbf6`. This is now a proto break plus a regeneration, not a free edit. It should
be decided deliberately rather than by default — which is why it is listed as open rather
than recommended.

### D. Multi-instance telemetry addressing — a latent defect

The contract has the fields and nothing resolves them. `telemetry.proto` carries
`BatteryStatus.id` ("battery instance, 0-indexed") under a message comment claiming
"multi-battery support"; `calibration.proto` carries `compass_id`. There is **no fold rule**
for two `BATTERY_STATUS` messages with different `id`, and `tier-1-pure-domain-logic.md`
flattens to scalar `batteryVoltagesMv` / `batteryCurrentA` / `batteryRemainingPct` with no
instance dimension — so two batteries silently last-writer-wins. No GPS, EKF-core or link
instance ids exist at all.

Pass A's fix, unused: Cockpit's `mavlinkIdentificationKeys` list (`id`, `idx`, `sensor_id`,
`camera_id`, `compass_id`, `stream_id`, …) turns two batteries into distinct paths instead
of a collision. It is the biggest real problem with flattening MAVLink, and Pass B never
raised it.

### E. WebRTC signalling — the two passes disagree

| | Position |
|---|---|
| **Pass A** | `ExchangeSdp` is unary; trickle ICE cannot round-trip in one call and client-streaming is banned, so it needs a unary offer + **server-streamed candidate channel** + unary candidate submission |
| **Pass B** | "No contract change needed — `SdpOffer`/`SdpAnswer`/`ExchangeSdp` already express Cockpit's protocol" |

`video.proto` is unchanged: unary, `repeated string ice_candidates` in both directions,
comment reads *"ICE candidates are included inline (trickle ICE not supported in this
version)."* On the merits Pass A's objection stands — non-trickle means gathering every
candidate before answering, which adds seconds to session setup. But WebRTC/WFB-NG video is
explicitly out of scope in `order-of-operations.md`, so this is recorded rather than acted
on. Whoever schedules the video tier owns the call.

Pass B's own video leftovers sit alongside it, and are also open:
`VIDEO_STREAM_TYPE_ANALOG` does not exist in the enum (`USB_WEBCAM` is adjacent but means
something else), and there is no unset-versus-zero convention for `packet_loss_pct` /
`latency_ms` — so a capture card currently renders as a perfect link.

### F. Smaller, unclaimed

- **tlog.** Byte-identical between QGC and Mission Planner: repeated
  `[8-byte big-endian µs timestamp][raw MAVLink frame]`. Zero mentions anywhere on main.
  ADR-0007 §8's **event history** port is now the obvious home, so this got cheaper than
  when Pass A wrote it — ecosystem interop for near-zero cost.
- **`.ass` telemetry subtitles.** A 3×3 configurable grid burned into a standard SubStation
  Alpha sidecar, one per recording, correlated by wall-clock epoch window; plays in
  VLC/mpv/ffmpeg. The only video↔telemetry correlation mechanism in *any* of the three
  reference GCSs (QGC's `SubtitleWriter` does the same). Weakness: wall-clock correlation
  means pipeline latency shows up as a constant offset.
- **Replay-as-a-link.** QGC's `LogReplayLink : LinkInterface` — replay reuses the entire
  live pipeline with zero duplicate code paths. This suggests our replay seam belongs at
  the *transport* port, which is what the "replay produces zero outbound bytes" gates are
  actually trying to prove.
- **`dontTouch` message-interval intent**, so we never fight another GCS or the vehicle's
  own configuration. Belongs on the `SetMessageInterval` request shape.
- **Value-change vs timestamp-change subscriptions.** "Did the number move?" and "is this
  still live?" are different questions. `FreshnessRing`/`SourceBadge` have no stated
  mechanism for the distinction.
- **Parameter name remapping across versions** (`PSC_VELXY_* → PSC_NE_VEL_*`,
  `WPNAV_SPEED → WP_SPD` in real 4.7). Pass B measured 329 name changes per minor line and
  solved the *metadata* side by version-selecting whole sets, which works. Unsolved:
  operator-saved artifacts that reference retired names.
- **`ParameterMetadata` has no `group`/category field.** Only `user_level`
  ("Standard"/"Advanced"). Grouping presumably comes from the `param_id` prefix, but nothing
  says so — and see the prefix caveat in Part 4.
- **`protoreflect` runtime schema is still unscheduled** by either pass. It is what makes a
  generic message inspector possible without hand-writing a form per message. Note it covers
  only the message families we model, not the whole MAVLink dialect — Pass A overclaimed
  that equivalence once and corrected it.

---

## Part 3 — A correction to ADR-0007 §5

ADR-0007 records this accepted cost:

> The versioned ArduPilot directories publish XML only — `apm.pdef.json` exists solely in
> the unversioned "latest" directories — so there is no JSON passthrough to lean on.

That is true of `autotest.ardupilot.org/Parameters/versioned/`. It is **not** true of
`github.com/ArduPilot/ParameterRepository`. Checked against the GitHub API on 2026-08-18:

- 60 top-level directories named per **minor line** — `Copter-3.5` … `Copter-4.8`,
  `Plane-*`, `Rover-*`, `Sub-*`, `Tracker-*`, `Blimp-*`, `AP_Periph-*`. That is exactly the
  granularity ADR-0007 §5 chose.
- `Copter-4.7/` holds **both** `apm.pdef.json` (2,163,839 B) **and** `apm.pdef.xml`
  (2,710,473 B), plus `MAVLinkMessages.rst` (125,026 B), `Parameters.md`, `Parameters.rst`,
  `Parameters.html`.
- Actively maintained — commits 2026-08-06, 08-11, 08-16, all "Update metadata".

Recorded as an amendment on the ADR itself. The decision does not change; the stated reason
does. See `../adr/0007-firmware-variance-via-capability-negotiation.md`, Amendment
(2026-08-18).

**`MAVLinkMessages.rst` is the part neither pass noticed.** It is a per-firmware-version
list of the MAVLink messages that firmware actually handles — a second, orthogonal
capability source alongside `AUTOPILOT_VERSION`'s bitmap. The bits declare *protocol
features*; this declares *which messages are handled*. Unexploited.

---

## Part 4 — Verified artifacts worth keeping

Live-checked during Pass A (2026-08-17) unless noted. The `.plan` and `.waypoints` schemas
have moved to `../reference/mission-interchange-formats.md`.

**`apm.pdef.json` shape gotchas**, re-verified firsthand by downloading and parsing
`Copter-4.7`. These apply to the JSON dialect specifically; the XML has a different shape
and its own prefix rule, already recorded on `ParameterMetadata` in `parameters.proto`.

- 387 top-level keys, and **not all are name-prefix groups**. Most are (`ADSB_`, `AFS_`,
  `ARMING_`), but there is also a **vehicle-name group** (`"Copter"`, 92 params) holding
  `FLTMODE1..6`. An iterator keyed on "ends with `_`" misses them. This is also why any
  future `ParameterMetadata.group` derived from the `param_id` prefix will be incomplete.
- A `"json"` header key exists with value `{"version": 0}` — that **is** an object, so it
  must be skipped **by name**, not by shape.
- **Every scalar is a string**: `Increment: "10"`, `Range: {"high":"1800","low":"0"}`,
  `Units: "deg/s/s"`, `RebootRequired`.
- `Values`/`Bitmask` keys are strings in **lexicographic, not numeric, order** — the raw
  order for `FLTMODE1` is `0, 1, 11, 13, 14, …` with `2` appearing later.
- `Bitmask` keys are **bit indices, not masks**. (`parameters.proto` already states this
  for the XML path: rendering `1 << code` against a value-keyed map is off by one position.)

**Distribution channels, and how the reference GCSs pick.** QGC vendors
`ParameterRepository` at **build time** via CPM, embedded as Qt resources. Mission Planner
**downloads at runtime** from `autotest.ardupilot.org/Parameters/{Vehicle}/apm.pdef.xml.gz`
with a 7-day cache, and pins to the exact connected firmware via
`.../Parameters/versioned/{Vehicle}/stable-{ver}/apm.pdef.xml`. Cockpit does neither: it
statically imports four `apm.pdef.json` files at four *different* firmware versions
(Sub-4.5, Copter-4.3, Plane-4.3, Rover-4.2) with no version negotiation at all, and its CI
notes the bundle "sits near the default ~2 GB cap on every platform". ADR-0007 §5 chose
build-time vendoring with runtime *selection*, which is neither of the three and better
than all of them.

**Other confirmations.** `Parameters.json` does not exist — the artifact is
`apm.pdef.json`. There is no official standalone mode-list artifact and no `modes.json`.
QGroundControl has no WebRTC (no module, settings entry or signalling code — strong
evidence, though GStreamer pipeline strings inside `.cc` files were not searched). QGC's
`factmetadata.schema.json` (draft-07, `additionalProperties:false`) is a battle-tested
schema, and QGC uses **one** metadata format for parameters, telemetry fact groups,
mission-item settings and app settings.

---

## Part 5 — Do not port

Kept from Pass A because the reasoning is still load-bearing.

**Blocked by decisions already made:** Vue/Pinia (ADR-0002); Electron (out of scope);
`mavlink2rest` / `mavlink2rest-wasm` (we own our codec in Go — precisely the friction
Cockpit pays for *not* owning it).

**Actively worse than what we have already specified** — the important category, because
borrowing here would cost us:

| Cockpit | Ours |
|---|---|
| Command correlation **by command type only**, 5 s / 100 ms poll; two concurrent `DO_SET_MODE` calls cross-talk | Registry keyed `(sysID, compID, commandID)` with duplicate rejection |
| Parameter completeness as `len(received) >= count`, **no gap detection**; can hang forever on a lossy link | Received-index bitmask + 500 ms quiescence re-request + duplicate suppression |
| **Fire-and-forget** parameter writes, no echo verification | `PARAM_VALUE` echo confirmation, 3 s timeout |
| Static 1.5 MB metadata bundles at four mismatched firmware versions | Vendored per minor line, runtime-selected, mismatch surfaced (ADR-0007 §5) |
| Single-connection; two `unimplemented()` apologies in `ConnectionManager`; multi-system bolted on only in the data-lake path | sysId first-class from day one |
| Entire test suite is `expect(Math.sqrt(4)).toBe(2)` | Golden byte vectors, capability matrix, a `make` gate per tier |
| `libs/` purity is convention-only and leaks (imports Vue and Pinia; `post-pinia-connections.ts` exists to paper over the cycle) | Go import cycles are a compile error |

**Specific rejects:** the `BTNn_FUNCTION` auto-remapper (see A); Cockpit's `.cmp` mission
format (use `.plan` + `.waypoints`); four simultaneous styling systems (Tailwind + Vuetify +
Flowbite + Sass); the datalake as a persistence layer (mine the concepts, skip the scope);
and string-tagged enums and `char[]` handling — their `.includes('MAV_CMD_NAV')` string
matching, `(0,0)` as a "no position" sentinel, `param_id.join('').replace(/\0/g,'')`, and
`Math.round(v*10000)/10000` for float32↔float64 are all artifacts protobuf already
eliminates. **Do not "improve" our design toward these.**

---

## Open and unowned

One list, so these are findable. None is scheduled.

1. `Waypoint { position, repeated Command }` — now a proto break, no longer free (Part 2 C).
2. Multi-instance telemetry fold — latent last-writer-wins across battery instances (D).
3. WebRTC trickle-ICE disagreement, plus `VIDEO_STREAM_TYPE_ANALOG` and the unset-vs-zero
   convention for `packet_loss_pct` / `latency_ms` (E).
4. `ComplexItem` ↔ `MissionItem`: survey/corridor/structure-scan definitions have no proto
   equivalent. Both Cockpit and QGC concluded the *definition* must be persisted to stay
   re-editable, and if it is ever a stored artifact it is a message.
5. tlog under ADR-0007 §8's event-history port (F).
6. `.ass` telemetry subtitles (F).
7. Replay-as-a-link at the transport port (F).
8. `dontTouch` message-interval intent (F).
9. Value-change vs timestamp-change subscriptions (F).
10. Parameter name remapping for operator-saved artifacts (F).
11. `ParameterMetadata.group` (F), noting the prefix caveat in Part 4.
12. `protoreflect` runtime schema for a generic inspector (F).
13. `MAVLinkMessages.rst` as a second capability source (Part 3).
